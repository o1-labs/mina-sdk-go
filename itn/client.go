// Package itn is a client for the daemon's ITN GraphQL server.
//
// A daemon started with ITN_FEATURES=1, --itn-graphql-port <port> and
// --itn-keys <base64 ed25519 public keys> serves a second GraphQL API
// (Mina_graphql.schema_itn). Load testing tools use it to schedule payments
// and zkApp commands, read internal logs, change connection gating and stop
// the daemon. Every request must be signed with an ed25519 key whose public
// half is in --itn-keys.
//
// # Authentication protocol
//
// The daemon reads the Authorization header:
//
//   - "Signature <pk> <sig>": the signature covers the request body. The
//     daemon accepts this form for the auth query only; every other
//     operation answers "Missing sequence information".
//   - "Signature <pk> <sig> ; Sequencing <uuid> <n>": the signature covers
//     the big-endian uint16 n, then the server UUID, then the body. n must be
//     exactly the daemon's sequence number for this public key, which starts
//     at 0, goes up by one (wrapping) after each accepted sequenced request,
//     and is shared by all clients that sign with the same key.
//
// pk and sig are standard base64. A bad signature or an unknown key gives
// HTTP 401. A wrong UUID (the daemon restarted) or sequence number gives
// HTTP 412.
//
// Client runs auth to learn the UUID and the sequence number, sends
// sequenced requests one at a time, and on a 412 runs auth again and
// repeats the request once. It never repeats a sequenced request after a
// transport error, because the daemon may have run it.
//
// # Example
//
//	key, err := itn.KeyFromBase64(seed)
//	...
//	c := itn.NewClient("http://127.0.0.1:3086/graphql", key)
//	logs, err := c.InternalLogs(ctx, 0)
package itn

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	mina "github.com/MinaProtocol/mina-sdk-go"
)

// Option configures a Client.
type Option func(*Client)

// WithRetries sets the number of attempts of the auth handshake. Sequenced
// requests are never repeated after a transport error.
func WithRetries(n int) Option { return func(c *Client) { c.retries = n } }

// WithRetryDelay sets the delay between auth attempts.
func WithRetryDelay(d time.Duration) Option { return func(c *Client) { c.retryDelay = d } }

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.httpClient.Timeout = d } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.httpClient = hc } }

// session is the server UUID and the next sequence number of the key.
type session struct {
	uuid string
	seq  uint16
}

// Client is a client for one daemon's ITN GraphQL server. It is safe for
// concurrent use; share one Client per daemon, because it holds the
// daemon's session.
type Client struct {
	uri        string
	key        Key
	retries    int
	retryDelay time.Duration
	httpClient *http.Client

	// lock serializes requests; a channel instead of a mutex so that a
	// waiting request can give up when its context ends.
	lock    chan struct{}
	session *session
}

// NewClient creates a client for the ITN endpoint uri (for example
// http://127.0.0.1:3086/graphql).
func NewClient(uri string, key Key, opts ...Option) *Client {
	c := &Client{
		uri:        uri,
		key:        key,
		retries:    3,
		retryDelay: 5 * time.Second,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		lock:       make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.retries < 1 {
		c.retries = 1
	}
	return c
}

// URI returns the ITN endpoint.
func (c *Client) URI() string { return c.uri }

// PublicKeyBase64 returns the base64 public key of the client's key, as it
// must appear in the daemon's --itn-keys.
func (c *Client) PublicKeyBase64() string { return c.key.PublicKeyBase64() }

// Close releases idle connections.
func (c *Client) Close() { c.httpClient.CloseIdleConnections() }

func (c *Client) acquire(ctx context.Context) error {
	select {
	case c.lock <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) release() { <-c.lock }

// post sends body with the given Authorization header and returns the
// status and the response body.
func (c *Client) post(ctx context.Context, body []byte, authorization string) (status int, respBody []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.uri, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authorization)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err = io.ReadAll(resp.Body)
	return resp.StatusCode, respBody, err
}

type gqlResponse struct {
	Data   json.RawMessage          `json:"data"`
	Errors []mina.GraphQLErrorEntry `json:"errors"`
}

func decode(body []byte, queryName string) (json.RawMessage, error) {
	var r gqlResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", queryName, err)
	}
	if len(r.Errors) > 0 {
		return nil, &mina.GraphQLError{Errors: r.Errors, QueryName: queryName}
	}
	return r.Data, nil
}

// handshake runs auth with an unsequenced signature and stores the session.
// The caller holds the lock.
func (c *Client) handshake(ctx context.Context) (*Auth, error) {
	const name = "itn_auth"
	body, err := json.Marshal(map[string]any{"query": QueryAuth})
	if err != nil {
		return nil, err
	}
	authorization := "Signature " + c.key.PublicKeyBase64() + " " + c.key.signBase64(body)

	var lastErr error
	for attempt := 1; attempt <= c.retries; attempt++ {
		status, respBody, err := c.post(ctx, body, authorization)
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
		case status == http.StatusUnauthorized:
			return nil, &UnauthorizedError{QueryName: name}
		case status >= 300:
			lastErr = fmt.Errorf("HTTP %d: %s", status, respBody)
		default:
			data, err := decode(respBody, name)
			if err != nil {
				return nil, err
			}
			auth, err := parseAuth(data)
			if err != nil {
				return nil, err
			}
			c.session = &session{uuid: auth.ServerUUID, seq: auth.SignerSequenceNumber}
			return auth, nil
		}
		if attempt < c.retries {
			select {
			case <-time.After(c.retryDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, &mina.ConnectionError{QueryName: name, Retries: c.retries, LastError: lastErr}
}

// Request sends one sequenced request and returns its data field.
// Requests of one client are sent one at a time, because the daemon accepts
// only its exact next sequence number.
func (c *Client) Request(ctx context.Context, query string, variables map[string]any, queryName string) (json.RawMessage, error) {
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()

	// A second pass only follows a 412: the daemon restarted, or another
	// client with the same key used our sequence number.
	for pass := 1; pass <= 2; pass++ {
		if c.session == nil {
			if _, err := c.handshake(ctx); err != nil {
				return nil, err
			}
		}
		s := *c.session
		msg := make([]byte, 2, 2+len(s.uuid)+len(body))
		binary.BigEndian.PutUint16(msg, s.seq)
		msg = append(msg, s.uuid...)
		msg = append(msg, body...)
		authorization := fmt.Sprintf("Signature %s %s ; Sequencing %s %d",
			c.key.PublicKeyBase64(), c.key.signBase64(msg), s.uuid, s.seq)

		status, respBody, err := c.post(ctx, body, authorization)
		if err != nil {
			// Unknown whether the daemon counted it: start over next time.
			c.session = nil
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, &mina.ConnectionError{QueryName: queryName, Retries: 1, LastError: err}
		}
		switch {
		case status == http.StatusPreconditionFailed:
			c.session = nil
			continue
		case status == http.StatusUnauthorized:
			return nil, &UnauthorizedError{QueryName: queryName}
		case status >= 300:
			c.session = nil
			return nil, &mina.ConnectionError{
				QueryName: queryName, Retries: 1,
				LastError: fmt.Errorf("HTTP %d: %s", status, respBody),
			}
		}
		// The signature was accepted, so the daemon has moved to the next
		// number, whatever the GraphQL result is.
		c.session.seq++
		return decode(respBody, queryName)
	}
	return nil, &SequencingError{QueryName: queryName}
}

// Auth runs the auth handshake and returns the node's answer. Other methods
// run it when needed, so calling it first is optional.
func (c *Client) Auth(ctx context.Context) (*Auth, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()
	return c.handshake(ctx)
}

// SlotsWon returns the global slots the node's block producer keys won in
// the current epoch.
func (c *Client) SlotsWon(ctx context.Context) ([]int64, error) {
	data, err := c.Request(ctx, QuerySlotsWon, nil, "itn_slots_won")
	if err != nil {
		return nil, err
	}
	var r struct {
		SlotsWon []int64 `json:"slotsWon"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("itn_slots_won: %w", err)
	}
	return r.SlotsWon, nil
}

// InternalLogs returns the internal logs with an ID of at least startLogID.
func (c *Client) InternalLogs(ctx context.Context, startLogID int64) ([]Log, error) {
	data, err := c.Request(ctx, QueryInternalLogs, map[string]any{"startLogId": startLogID}, "itn_internal_logs")
	if err != nil {
		return nil, err
	}
	var r struct {
		InternalLogs []Log `json:"internalLogs"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("itn_internal_logs: %w", err)
	}
	return r.InternalLogs, nil
}

// FlushInternalLogs drops the internal logs up to and including endLogID and
// returns the daemon's answer.
func (c *Client) FlushInternalLogs(ctx context.Context, endLogID int64) (string, error) {
	return c.stringMutation(ctx, MutationFlushInternalLogs,
		map[string]any{"endLogId": endLogID}, "flushInternalLogs", "itn_flush_internal_logs")
}

// SchedulePayments starts sending payments and returns the handle for
// StopScheduledTransactions.
func (c *Client) SchedulePayments(ctx context.Context, d PaymentsDetails) (string, error) {
	return c.stringMutation(ctx, MutationSchedulePayments,
		map[string]any{"input": d.vars()}, "schedulePayments", "itn_schedule_payments")
}

// ScheduleZkappCommands starts sending zkApp commands and returns the handle
// for StopScheduledTransactions.
func (c *Client) ScheduleZkappCommands(ctx context.Context, d ZkappCommandsDetails) (string, error) {
	return c.stringMutation(ctx, MutationScheduleZkappCommands,
		map[string]any{"input": d.vars()}, "scheduleZkappCommands", "itn_schedule_zkapp_commands")
}

// StopScheduledTransactions stops the transactions of a schedule handle.
func (c *Client) StopScheduledTransactions(ctx context.Context, handle string) (string, error) {
	return c.stringMutation(ctx, MutationStopScheduledTransactions,
		map[string]any{"handle": handle}, "stopScheduledTransactions", "itn_stop_scheduled_transactions")
}

// UpdateGating changes the node's connection gating.
func (c *Client) UpdateGating(ctx context.Context, g GatingUpdate) (string, error) {
	return c.stringMutation(ctx, MutationUpdateGating,
		map[string]any{"input": g.vars()}, "updateGating", "itn_update_gating")
}

// StopDaemon stops the daemon after delaySeconds (the daemon's minimum is 5;
// nil for its default), deleting its configuration directory if cleanConfig
// is true.
func (c *Client) StopDaemon(ctx context.Context, delaySeconds *int, cleanConfig bool) (string, error) {
	vars := map[string]any{"delaySeconds": delaySeconds, "cleanConfig": nil}
	if cleanConfig {
		vars["cleanConfig"] = true
	}
	return c.stringMutation(ctx, MutationStopDaemon, vars, "stopDaemon", "itn_stop_daemon")
}

// SetZkappCommandLimit sets the block producer's limit of zkApp commands per
// block; nil removes the limit. It returns the limit now in force.
func (c *Client) SetZkappCommandLimit(ctx context.Context, limit *int) (*int, error) {
	data, err := c.Request(ctx, MutationZkappCommandLimit, map[string]any{"limit": limit}, "itn_set_zkapp_command_limit")
	if err != nil {
		return nil, err
	}
	var r struct {
		ZkAppCommandLimit *int `json:"zkAppCommandLimit"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("itn_set_zkapp_command_limit: %w", err)
	}
	return r.ZkAppCommandLimit, nil
}

func (c *Client) stringMutation(ctx context.Context, query string, vars map[string]any, field, queryName string) (string, error) {
	data, err := c.Request(ctx, query, vars, queryName)
	if err != nil {
		return "", err
	}
	var r map[string]*string
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("%s: %w", queryName, err)
	}
	if r[field] == nil {
		return "", fmt.Errorf("%s: missing field %q", queryName, field)
	}
	return *r[field], nil
}

// parseUint16 accepts a UInt16 as the daemon sends it (a string, "8302") or
// as a number.
func parseUint16(raw json.RawMessage) (uint16, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n, err := strconv.ParseUint(s, 10, 16)
		return uint16(n), err
	}
	var n uint16
	err := json.Unmarshal(raw, &n)
	return n, err
}

func parseAuth(data json.RawMessage) (*Auth, error) {
	var r struct {
		Auth *struct {
			ServerUUID           string          `json:"serverUuid"`
			SignerSequenceNumber json.RawMessage `json:"signerSequenceNumber"`
			Libp2pPort           json.RawMessage `json:"libp2pPort"`
			PeerID               *string         `json:"peerId"`
			IsBlockProducer      bool            `json:"isBlockProducer"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("itn_auth: %w", err)
	}
	if r.Auth == nil || r.Auth.ServerUUID == "" {
		return nil, errors.New("itn_auth: missing field auth.serverUuid")
	}
	seq, err := parseUint16(r.Auth.SignerSequenceNumber)
	if err != nil {
		return nil, fmt.Errorf("itn_auth: signerSequenceNumber: %w", err)
	}
	port, err := parseUint16(r.Auth.Libp2pPort)
	if err != nil {
		return nil, fmt.Errorf("itn_auth: libp2pPort: %w", err)
	}
	a := &Auth{
		ServerUUID:           r.Auth.ServerUUID,
		SignerSequenceNumber: seq,
		Libp2pPort:           port,
		IsBlockProducer:      r.Auth.IsBlockProducer,
	}
	if r.Auth.PeerID != nil {
		a.PeerID = *r.Auth.PeerID
	}
	return a, nil
}
