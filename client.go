// Package mina provides a Go client for interacting with Mina Protocol daemon nodes
// via the GraphQL API.
//
// Basic usage:
//
//	client := mina.NewClient()
//	defer client.Close()
//
//	status, err := client.GetSyncStatus()
package mina

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

// DefaultGraphQLURI is the default Mina daemon GraphQL endpoint.
const DefaultGraphQLURI = "http://127.0.0.1:3085/graphql"

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithGraphQLURI sets the GraphQL endpoint URI.
func WithGraphQLURI(uri string) ClientOption {
	return func(c *Client) { c.uri = uri }
}

// WithRetries sets the number of retry attempts for failed requests.
func WithRetries(n int) ClientOption {
	return func(c *Client) { c.retries = n }
}

// WithRetryDelay sets the delay between retries.
func WithRetryDelay(d time.Duration) ClientOption {
	return func(c *Client) { c.retryDelay = d }
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// Client is a Mina daemon GraphQL client.
type Client struct {
	uri        string
	retries    int
	retryDelay time.Duration
	httpClient *http.Client
}

// NewClient creates a new Mina daemon client with the given options.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		uri:        DefaultGraphQLURI,
		retries:    3,
		retryDelay: 5 * time.Second,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Close releases resources used by the client.
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage     `json:"data"`
	Errors []GraphQLErrorEntry `json:"errors"`
}

func (c *Client) request(query string, variables map[string]any, queryName string) (json.RawMessage, error) {
	payload, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.retries; attempt++ {
		log.Printf("GraphQL %s attempt %d/%d", queryName, attempt, c.retries)

		resp, err := c.httpClient.Post(c.uri, "application/json", bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			log.Printf("GraphQL %s connection error (attempt %d/%d): %v", queryName, attempt, c.retries, err)
			if attempt < c.retries {
				time.Sleep(c.retryDelay)
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			if attempt < c.retries {
				time.Sleep(c.retryDelay)
			}
			continue
		}

		var gqlResp graphqlResponse
		if err := json.Unmarshal(body, &gqlResp); err != nil {
			lastErr = err
			if attempt < c.retries {
				time.Sleep(c.retryDelay)
			}
			continue
		}

		if len(gqlResp.Errors) > 0 {
			return nil, &GraphQLError{Errors: gqlResp.Errors, QueryName: queryName}
		}

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			log.Printf("GraphQL %s HTTP %d (attempt %d/%d)", queryName, resp.StatusCode, attempt, c.retries)
			if attempt < c.retries {
				time.Sleep(c.retryDelay)
			}
			continue
		}

		return gqlResp.Data, nil
	}

	return nil, &ConnectionError{QueryName: queryName, Retries: c.retries, LastError: lastErr}
}

// -- Queries --

// GetSyncStatus returns the node's sync status.
// Returns one of the SyncStatus constants (SyncStatusSynced, SyncStatusBootstrap, ...).
func (c *Client) GetSyncStatus() (SyncStatus, error) {
	data, err := c.request(querySyncStatus, nil, "get_sync_status")
	if err != nil {
		return "", err
	}
	var result struct {
		SyncStatus SyncStatus `json:"syncStatus"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.SyncStatus, nil
}

// GetDaemonStatus returns comprehensive daemon status.
func (c *Client) GetDaemonStatus() (*DaemonStatus, error) {
	data, err := c.request(queryDaemonStatus, nil, "get_daemon_status")
	if err != nil {
		return nil, err
	}
	var result struct {
		DaemonStatus struct {
			SyncStatus                 SyncStatus `json:"syncStatus"`
			BlockchainLength           *int       `json:"blockchainLength"`
			HighestBlockLengthReceived *int       `json:"highestBlockLengthReceived"`
			UptimeSecs                 *int       `json:"uptimeSecs"`
			StateHash                  string     `json:"stateHash"`
			CommitID                   string     `json:"commitId"`
			Peers                      []struct {
				PeerID     string `json:"peerId"`
				Host       string `json:"host"`
				Libp2pPort int    `json:"libp2pPort"`
			} `json:"peers"`
		} `json:"daemonStatus"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	ds := result.DaemonStatus
	status := &DaemonStatus{
		SyncStatus:                 ds.SyncStatus,
		BlockchainLength:           ds.BlockchainLength,
		HighestBlockLengthReceived: ds.HighestBlockLengthReceived,
		UptimeSecs:                 ds.UptimeSecs,
		StateHash:                  ds.StateHash,
		CommitID:                   ds.CommitID,
	}
	if ds.Peers != nil {
		status.Peers = make([]PeerInfo, len(ds.Peers))
		for i, p := range ds.Peers {
			status.Peers[i] = PeerInfo{PeerID: p.PeerID, Host: p.Host, Port: p.Libp2pPort}
		}
	}
	return status, nil
}

// GetNetworkID returns the network identifier.
func (c *Client) GetNetworkID() (string, error) {
	data, err := c.request(queryNetworkID, nil, "get_network_id")
	if err != nil {
		return "", err
	}
	var result struct {
		NetworkID string `json:"networkID"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.NetworkID, nil
}

// GetAccount returns account data for a public key.
// Pass an empty tokenID to use the default MINA token.
func (c *Client) GetAccount(publicKey, tokenID string) (*AccountData, error) {
	// tokenID is optional: a non-empty value scopes the query to that token,
	// while an empty value sends $token as null so the daemon resolves the
	// default MINA token. The variable is always present in the map: the Mina
	// daemon rejects a query whose declared variable is absent from a supplied
	// variables object ("Missing variable `token`"), so omitting the key is not
	// an option — null is the correct "use default" signal for a nullable arg.
	vars := map[string]any{"publicKey": publicKey, "token": nil}
	if tokenID != "" {
		vars["token"] = tokenID
	}

	data, err := c.request(queryGetAccount, vars, "get_account")
	if err != nil {
		return nil, err
	}

	var result struct {
		Account *struct {
			PublicKey string      `json:"publicKey"`
			Nonce     json.Number `json:"nonce"`
			Delegate  string      `json:"delegate"`
			TokenID   string      `json:"tokenId"`
			Balance   struct {
				Total  string  `json:"total"`
				Liquid *string `json:"liquid"`
				Locked *string `json:"locked"`
			} `json:"balance"`
		} `json:"account"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.Account == nil {
		return nil, &AccountNotFoundError{PublicKey: publicKey}
	}

	acc := result.Account
	nonce, _ := acc.Nonce.Int64()
	total, err := CurrencyFromGraphQL(acc.Balance.Total)
	if err != nil {
		return nil, fmt.Errorf("parse total balance: %w", err)
	}

	balance := AccountBalance{Total: total}
	if acc.Balance.Liquid != nil && *acc.Balance.Liquid != "" {
		liq, err := CurrencyFromGraphQL(*acc.Balance.Liquid)
		if err != nil {
			return nil, fmt.Errorf("parse liquid balance: %w", err)
		}
		balance.Liquid = &liq
	}
	if acc.Balance.Locked != nil && *acc.Balance.Locked != "" {
		locked, err := CurrencyFromGraphQL(*acc.Balance.Locked)
		if err != nil {
			return nil, fmt.Errorf("parse locked balance: %w", err)
		}
		balance.Locked = &locked
	}

	return &AccountData{
		PublicKey: acc.PublicKey,
		Nonce:     int(nonce),
		Balance:   balance,
		Delegate:  acc.Delegate,
		TokenID:   acc.TokenID,
	}, nil
}

// GetBestChain returns blocks from the best chain.
// Pass 0 for maxLength to use the daemon's default.
func (c *Client) GetBestChain(maxLength int) ([]BlockInfo, error) {
	// maxLength <= 0 sends $maxLength as null, letting the daemon apply its
	// default. The variable must always be present (see GetAccount): the daemon
	// rejects a declared-but-unsupplied variable.
	vars := map[string]any{"maxLength": nil}
	if maxLength > 0 {
		vars["maxLength"] = maxLength
	}

	data, err := c.request(queryBestChain, vars, "get_best_chain")
	if err != nil {
		return nil, err
	}

	var result struct {
		BestChain []struct {
			StateHash               string `json:"stateHash"`
			CommandTransactionCount int    `json:"commandTransactionCount"`
			CreatorAccount          struct {
				PublicKey string `json:"publicKey"`
			} `json:"creatorAccount"`
			ProtocolState struct {
				ConsensusState struct {
					BlockHeight      string `json:"blockHeight"`
					SlotSinceGenesis string `json:"slotSinceGenesis"`
					Slot             string `json:"slot"`
				} `json:"consensusState"`
			} `json:"protocolState"`
		} `json:"bestChain"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.BestChain == nil {
		return nil, nil
	}

	blocks := make([]BlockInfo, len(result.BestChain))
	for i, b := range result.BestChain {
		height, _ := strconv.Atoi(b.ProtocolState.ConsensusState.BlockHeight)
		slotGenesis, _ := strconv.Atoi(b.ProtocolState.ConsensusState.SlotSinceGenesis)
		slotFork, _ := strconv.Atoi(b.ProtocolState.ConsensusState.Slot)

		// The daemon may omit the creator public key (e.g. for the genesis
		// block); fall back to a sentinel so callers get a stable value.
		creatorPK := b.CreatorAccount.PublicKey
		if creatorPK == "" {
			creatorPK = "unknown"
		}

		blocks[i] = BlockInfo{
			StateHash:               b.StateHash,
			Height:                  height,
			GlobalSlotSinceHardFork: slotFork,
			GlobalSlotSinceGenesis:  slotGenesis,
			CreatorPK:               creatorPK,
			CommandTransactionCount: b.CommandTransactionCount,
		}
	}
	return blocks, nil
}

// GetPeers returns the list of connected peers.
func (c *Client) GetPeers() ([]PeerInfo, error) {
	data, err := c.request(queryGetPeers, nil, "get_peers")
	if err != nil {
		return nil, err
	}
	var result struct {
		GetPeers []struct {
			PeerID     string `json:"peerId"`
			Host       string `json:"host"`
			Libp2pPort int    `json:"libp2pPort"`
		} `json:"getPeers"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	peers := make([]PeerInfo, len(result.GetPeers))
	for i, p := range result.GetPeers {
		peers[i] = PeerInfo{PeerID: p.PeerID, Host: p.Host, Port: p.Libp2pPort}
	}
	return peers, nil
}

// GetPooledUserCommands returns pending user commands from the transaction pool.
// Pass an empty publicKey to get all pending commands.
func (c *Client) GetPooledUserCommands(publicKey string) ([]PooledUserCommand, error) {
	// publicKey is optional: a non-empty value filters to that sender, while
	// an empty value sends $publicKey as null so the daemon returns every
	// pending command. The variable must always be present (see GetAccount):
	// the daemon rejects a declared-but-unsupplied variable.
	vars := map[string]any{"publicKey": nil}
	if publicKey != "" {
		vars["publicKey"] = publicKey
	}

	data, err := c.request(queryPooledUserCommands, vars, "get_pooled_user_commands")
	if err != nil {
		return nil, err
	}
	var result struct {
		PooledUserCommands []pooledUserCommandRaw `json:"pooledUserCommands"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.PooledUserCommands == nil {
		return []PooledUserCommand{}, nil
	}
	cmds := make([]PooledUserCommand, len(result.PooledUserCommands))
	for i, raw := range result.PooledUserCommands {
		cmds[i] = PooledUserCommand{
			ID: raw.ID, Hash: raw.Hash, Kind: raw.Kind,
			Nonce: raw.Nonce.String(), Amount: raw.Amount,
			Fee: raw.Fee, From: raw.From, To: raw.To,
		}
	}
	return cmds, nil
}

// -- Mutations --

// SendPaymentParams are the parameters for SendPayment.
type SendPaymentParams struct {
	Sender   string
	Receiver string
	Amount   Currency
	Fee      Currency
	Memo     string // optional
	Nonce    *int   // optional explicit nonce
}

// SendPayment sends a payment transaction.
// Requires the sender's account to be unlocked on the node.
func (c *Client) SendPayment(params SendPaymentParams) (*SendPaymentResult, error) {
	input := map[string]any{
		"from":   params.Sender,
		"to":     params.Receiver,
		"amount": params.Amount.NanominaString(),
		"fee":    params.Fee.NanominaString(),
	}

	// Memo and Nonce are optional: a zero memo and a nil nonce are simply
	// omitted, letting the daemon assign the next nonce.
	if params.Memo != "" {
		input["memo"] = params.Memo
	}
	if params.Nonce != nil {
		input["nonce"] = strconv.Itoa(*params.Nonce)
	}

	data, err := c.request(mutationSendPayment, map[string]any{"input": input}, "send_payment")
	if err != nil {
		return nil, err
	}

	var result struct {
		SendPayment struct {
			Payment struct {
				ID    string      `json:"id"`
				Hash  string      `json:"hash"`
				Nonce json.Number `json:"nonce"`
			} `json:"payment"`
		} `json:"sendPayment"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	nonce, _ := result.SendPayment.Payment.Nonce.Int64()
	return &SendPaymentResult{
		ID:    result.SendPayment.Payment.ID,
		Hash:  result.SendPayment.Payment.Hash,
		Nonce: int(nonce),
	}, nil
}

// SendDelegationParams are the parameters for SendDelegation.
type SendDelegationParams struct {
	Sender     string
	DelegateTo string
	Fee        Currency
	Memo       string // optional
	Nonce      *int   // optional explicit nonce
}

// SendDelegation sends a stake delegation transaction.
// Requires the sender's account to be unlocked on the node.
func (c *Client) SendDelegation(params SendDelegationParams) (*SendDelegationResult, error) {
	input := map[string]any{
		"from": params.Sender,
		"to":   params.DelegateTo,
		"fee":  params.Fee.NanominaString(),
	}
	if params.Memo != "" {
		input["memo"] = params.Memo
	}
	if params.Nonce != nil {
		input["nonce"] = strconv.Itoa(*params.Nonce)
	}

	data, err := c.request(mutationSendDelegation, map[string]any{"input": input}, "send_delegation")
	if err != nil {
		return nil, err
	}

	var result struct {
		SendDelegation struct {
			Delegation struct {
				ID    string      `json:"id"`
				Hash  string      `json:"hash"`
				Nonce json.Number `json:"nonce"`
			} `json:"delegation"`
		} `json:"sendDelegation"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	nonce, _ := result.SendDelegation.Delegation.Nonce.Int64()
	return &SendDelegationResult{
		ID:    result.SendDelegation.Delegation.ID,
		Hash:  result.SendDelegation.Delegation.Hash,
		Nonce: int(nonce),
	}, nil
}

// SetSnarkWorker sets or unsets the SNARK worker key.
// Pass an empty string to disable the SNARK worker.
// Returns the previous snark worker public key (empty if none).
func (c *Client) SetSnarkWorker(publicKey string) (string, error) {
	// A non-empty publicKey sets the SNARK worker; an empty string sends a
	// null input, which unsets it.
	var input any
	if publicKey != "" {
		input = publicKey
	}

	data, err := c.request(mutationSetSnarkWorker, map[string]any{"input": input}, "set_snark_worker")
	if err != nil {
		return "", err
	}

	var result struct {
		SetSnarkWorker struct {
			LastSnarkWorker *string `json:"lastSnarkWorker"`
		} `json:"setSnarkWorker"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if result.SetSnarkWorker.LastSnarkWorker == nil {
		return "", nil
	}
	return *result.SetSnarkWorker.LastSnarkWorker, nil
}

// SetSnarkWorkFee sets the fee for SNARK work.
// Returns the previous fee as a string.
func (c *Client) SetSnarkWorkFee(fee Currency) (string, error) {
	data, err := c.request(mutationSetSnarkWorkFee, map[string]any{"fee": fee.NanominaString()}, "set_snark_work_fee")
	if err != nil {
		return "", err
	}

	var result struct {
		SetSnarkWorkFee struct {
			LastFee string `json:"lastFee"`
		} `json:"setSnarkWorkFee"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.SetSnarkWorkFee.LastFee, nil
}
