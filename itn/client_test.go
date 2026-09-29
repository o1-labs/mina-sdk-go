package itn

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	mina "github.com/MinaProtocol/mina-sdk-go"
)

const testUUID = "5f0c2e6e-6a57-4bd4-9a8c-3a8f6c7e8b21"

// seen is one request the fake daemon received.
type seen struct {
	body   []byte
	auth   string
	isAuth bool
}

// fakeDaemon answers auth with sequence number startSeq and every other
// request with respond(body), after checking the Authorization header the
// way the daemon does (graphql_internal.ml).
type fakeDaemon struct {
	t        *testing.T
	key      Key
	startSeq uint16
	respond  func(body string) (int, any)

	mu   sync.Mutex
	reqs []seen
}

func (f *fakeDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	auth := r.Header.Get("Authorization")
	isAuth := strings.Contains(string(body), "auth {")
	f.mu.Lock()
	f.reqs = append(f.reqs, seen{body: body, auth: auth, isAuth: isAuth})
	f.mu.Unlock()
	f.verify(body, auth, isAuth)

	status, resp := http.StatusOK, any(nil)
	if isAuth {
		resp = map[string]any{"data": map[string]any{"auth": map[string]any{
			"serverUuid":           testUUID,
			"signerSequenceNumber": strconv.Itoa(int(f.startSeq)),
			"libp2pPort":           "8302",
			"peerId":               "12D3KooWGCh9hCWdBjXp7dvxhhPKoa264Ny3BByoCinc9gyDEWNF",
			"isBlockProducer":      true,
		}}}
	} else {
		status, resp = f.respond(string(body))
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// verify checks the signature over the message the daemon reconstructs.
func (f *fakeDaemon) verify(body []byte, auth string, isAuth bool) {
	parts := strings.Split(auth, " ")
	pk, _ := base64.StdEncoding.DecodeString(parts[1])
	sig, _ := base64.StdEncoding.DecodeString(parts[2])
	if parts[1] != f.key.PublicKeyBase64() {
		f.t.Errorf("unexpected public key %s", parts[1])
	}
	msg := body
	switch {
	case len(parts) == 3:
		if !isAuth {
			f.t.Errorf("unsequenced signature on a non-auth request: %s", body)
		}
	case len(parts) == 7 && parts[3] == ";" && parts[4] == "Sequencing":
		n, _ := strconv.ParseUint(parts[6], 10, 16)
		msg = binary.BigEndian.AppendUint16(nil, uint16(n))
		msg = append(msg, parts[5]...)
		msg = append(msg, body...)
	default:
		f.t.Fatalf("unexpected Authorization header %q", auth)
	}
	if !ed25519.Verify(ed25519.PublicKey(pk), msg, sig) {
		f.t.Errorf("signature does not verify for %q", auth)
	}
}

// seqs returns the sequence numbers of the non-auth requests.
func (f *fakeDaemon) seqs() []uint16 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []uint16
	for _, r := range f.reqs {
		if r.isAuth {
			continue
		}
		parts := strings.Split(r.auth, " ")
		n, _ := strconv.ParseUint(parts[6], 10, 16)
		out = append(out, uint16(n))
	}
	return out
}

func (f *fakeDaemon) kinds() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var b strings.Builder
	for _, r := range f.reqs {
		if r.isAuth {
			b.WriteByte('A')
		} else {
			b.WriteByte('R')
		}
	}
	return b.String()
}

func newFake(t *testing.T, startSeq uint16, respond func(string) (int, any)) (*fakeDaemon, *Client) {
	t.Helper()
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeDaemon{t: t, key: key, startSeq: startSeq, respond: respond}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL+"/graphql", key, WithRetries(2), WithRetryDelay(time.Millisecond))
	t.Cleanup(c.Close)
	return f, c
}

func data(v map[string]any) (status int, resp any) {
	return http.StatusOK, map[string]any{"data": v}
}

func TestAuthIsUnsequenced(t *testing.T) {
	f, c := newFake(t, 4, nil)
	if c.LastAuth() != nil {
		t.Error("LastAuth before any auth must be nil")
	}
	a, err := c.Auth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.ServerUUID != testUUID || a.SignerSequenceNumber != 4 || a.Libp2pPort != 8302 || !a.IsBlockProducer {
		t.Errorf("unexpected auth %+v", a)
	}
	if f.kinds() != "A" {
		t.Errorf("requests %s, want A", f.kinds())
	}
}

func TestSequencedRequestsCountUp(t *testing.T) {
	f, c := newFake(t, 41, func(string) (int, any) {
		return data(map[string]any{"internalLogs": []any{map[string]any{
			"id": 3, "timestamp": "2026-09-29T08:00:00Z", "message": "Block produced",
			"metadata": []any{map[string]any{"item": "height", "value": 12}}, "process": nil,
		}}})
	})
	ctx := context.Background()
	logs, err := c.InternalLogs(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.InternalLogs(ctx, 4); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].ID != 3 || logs[0].Process != nil ||
		logs[0].Metadata[0].Item != "height" || string(logs[0].Metadata[0].Value) != "12" {
		t.Errorf("unexpected logs %+v", logs)
	}
	if got := f.seqs(); len(got) != 2 || got[0] != 41 || got[1] != 42 {
		t.Errorf("sequence numbers %v, want [41 42]", got)
	}
	if f.kinds() != "ARR" {
		t.Errorf("requests %s, want ARR", f.kinds())
	}
}

// The daemon counts a request once it accepted the signature, even when the
// GraphQL result is an error; the client must count it too.
func TestGraphQLErrorUsesUpSequenceNumber(t *testing.T) {
	f, c := newFake(t, 0, func(string) (int, any) {
		return http.StatusOK, map[string]any{"errors": []any{map[string]any{"message": "Not a block producing node"}}}
	})
	for i := 0; i < 2; i++ {
		var gqlErr *mina.GraphQLError
		if _, err := c.SlotsWon(context.Background()); !errors.As(err, &gqlErr) {
			t.Fatalf("expected GraphQLError, got %v", err)
		}
	}
	if got := f.seqs(); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("sequence numbers %v, want [0 1]", got)
	}
}

func TestPreconditionFailedRunsAuthAgain(t *testing.T) {
	first := true
	f, c := newFake(t, 9, func(string) (int, any) {
		if first {
			first = false
			return http.StatusPreconditionFailed, "Invalid sequence number"
		}
		return data(map[string]any{"flushInternalLogs": "5"})
	})
	got, err := c.FlushInternalLogs(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != "5" || f.kinds() != "ARAR" {
		t.Errorf("result %q, requests %s; want 5, ARAR", got, f.kinds())
	}
	if a := c.LastAuth(); a == nil || a.ServerUUID != testUUID {
		t.Errorf("LastAuth after the new auth = %+v", a)
	}
}

func TestPreconditionFailedTwiceIsAnError(t *testing.T) {
	_, c := newFake(t, 0, func(string) (int, any) { return http.StatusPreconditionFailed, "" })
	var seqErr *SequencingError
	if _, err := c.StopScheduledTransactions(context.Background(), "h"); !errors.As(err, &seqErr) {
		t.Fatalf("expected SequencingError, got %v", err)
	}
}

func TestUnauthorized(t *testing.T) {
	key, _ := GenerateKey()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, key, WithRetries(3), WithRetryDelay(time.Millisecond))
	var unauth *UnauthorizedError
	if _, err := c.Auth(context.Background()); !errors.As(err, &unauth) {
		t.Fatalf("expected UnauthorizedError, got %v", err)
	}
	if n != 1 {
		t.Errorf("%d requests; a 401 must not be retried", n)
	}
}

// A sequenced mutation that fails in transport is not repeated (the daemon
// may have run it), and the next request starts a new session.
func TestFailedMutationIsNotRepeated(t *testing.T) {
	f, c := newFake(t, 0, func(string) (int, any) { return http.StatusServiceUnavailable, "" })
	d := PaymentsDetails{
		DurationMin: 1, TPS: 0.5, MemoPrefix: "t",
		MaxFee: mina.CurrencyFromNanomina(2), MinFee: mina.CurrencyFromNanomina(1),
		Amount:   mina.CurrencyFromNanomina(1000),
		Receiver: "B62qrPN5Y5yq8kGE3FbVKbGTdTAJNdtNtB5sNVpxyRwWGcDEhpMzc8g",
	}
	var connErr *mina.ConnectionError
	var httpErr *HTTPError
	_, err := c.SchedulePayments(context.Background(), d)
	if !errors.As(err, &connErr) || !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected ConnectionError wrapping HTTPError 503, got %v", err)
	}
	_, _ = c.SchedulePayments(context.Background(), d)
	if f.kinds() != "ARAR" {
		t.Errorf("requests %s, want ARAR", f.kinds())
	}
	var body struct {
		Variables struct {
			Input map[string]any `json:"input"`
		} `json:"variables"`
	}
	_ = json.Unmarshal(f.reqs[1].body, &body)
	if body.Variables.Input["amount"] != "1000" || body.Variables.Input["maxFee"] != "2" {
		t.Errorf("unexpected input %v", body.Variables.Input)
	}
	if senders, ok := body.Variables.Input["senders"].([]any); !ok || len(senders) != 0 {
		t.Errorf("nil senders must be sent as [], got %v", body.Variables.Input["senders"])
	}
}

func TestSequenceNumberWraps(t *testing.T) {
	f, c := newFake(t, 65535, func(string) (int, any) {
		return data(map[string]any{"zkAppCommandLimit": nil})
	})
	for i := 0; i < 2; i++ {
		if limit, err := c.SetZkappCommandLimit(context.Background(), nil); err != nil || limit != nil {
			t.Fatalf("limit %v, err %v", limit, err)
		}
	}
	if got := f.seqs(); len(got) != 2 || got[0] != 65535 || got[1] != 0 {
		t.Errorf("sequence numbers %v, want [65535 0]", got)
	}
}

func TestNonDefaultTokenIsSentOnlyWhenSet(t *testing.T) {
	f, c := newFake(t, 0, func(string) (int, any) {
		return data(map[string]any{"scheduleZkappCommands": "h1"})
	})
	d := ZkappCommandsDetails{AccountQueueSize: 10, MemoPrefix: "z", DurationMin: 1, TPS: 0.1}
	if h, err := c.ScheduleZkappCommands(context.Background(), d); err != nil || h != "h1" {
		t.Fatalf("handle %q, err %v", h, err)
	}
	yes := true
	d.NonDefaultToken = &yes
	if _, err := c.ScheduleZkappCommands(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	var inputs []map[string]any
	for _, r := range f.reqs {
		if r.isAuth {
			continue
		}
		var body struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		_ = json.Unmarshal(r.body, &body)
		inputs = append(inputs, body.Variables.Input)
	}
	if _, ok := inputs[0]["nonDefaultToken"]; ok {
		t.Error("nonDefaultToken sent although not set")
	}
	if inputs[1]["nonDefaultToken"] != true {
		t.Errorf("nonDefaultToken = %v, want true", inputs[1]["nonDefaultToken"])
	}
}

// A request that waits for another one gives up when its context ends.
func TestContextCancelWhileWaiting(t *testing.T) {
	release := make(chan struct{})
	_, c := newFake(t, 0, func(string) (int, any) {
		<-release
		return data(map[string]any{"slotsWon": []int{}})
	})
	defer close(release)
	go func() { _, _ = c.SlotsWon(context.Background()) }()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.SlotsWon(ctx)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a request waiting for the lock did not give up when its context ended")
	}
}

func TestKeyEncoding(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	again, err := KeyFromBase64(" " + k.Base64() + "\n")
	if err != nil || again.PublicKeyBase64() != k.PublicKeyBase64() {
		t.Fatalf("round trip: %v", err)
	}
	var invalid *InvalidKeyError
	if _, err := KeyFromBase64("AAAA"); !errors.As(err, &invalid) {
		t.Errorf("short key: %v", err)
	}
	if _, err := KeyFromBase64("not base64!"); !errors.As(err, &invalid) {
		t.Errorf("bad base64: %v", err)
	}
	if s := k.String(); !strings.Contains(s, k.PublicKeyBase64()) || strings.Contains(s, k.Base64()) {
		t.Errorf("String must show only the public key: %s", s)
	}
}
