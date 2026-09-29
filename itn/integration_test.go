package itn

import (
	"context"
	"errors"
	"os"
	"testing"

	mina "github.com/MinaProtocol/mina-sdk-go"
)

// Integration tests against a running daemon's ITN GraphQL server. They
// require:
//   - MINA_ITN_URI: ITN GraphQL endpoint (e.g. http://127.0.0.1:3086/graphql)
//   - MINA_ITN_KEY: base64 ed25519 seed whose public key is in --itn-keys
//
// They are skipped if the variables are not set. None of them stops the
// daemon or sends transactions.

func itnFromEnv(t *testing.T) (string, Key) {
	t.Helper()
	uri, seed := os.Getenv("MINA_ITN_URI"), os.Getenv("MINA_ITN_KEY")
	if uri == "" || seed == "" {
		t.Skip("MINA_ITN_URI and MINA_ITN_KEY not set")
	}
	key, err := KeyFromBase64(seed)
	if err != nil {
		t.Fatal(err)
	}
	return uri, key
}

func TestIntegrationAuth(t *testing.T) {
	uri, key := itnFromEnv(t)
	a, err := NewClient(uri, key).Auth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.ServerUUID == "" || a.Libp2pPort == 0 {
		t.Errorf("unexpected auth %+v", a)
	}
}

func TestIntegrationUnknownKey(t *testing.T) {
	uri, _ := itnFromEnv(t)
	other, _ := GenerateKey()
	var unauth *UnauthorizedError
	if _, err := NewClient(uri, other).Auth(context.Background()); !errors.As(err, &unauth) {
		t.Fatalf("expected UnauthorizedError, got %v", err)
	}
}

func TestIntegrationSequencedRequests(t *testing.T) {
	uri, key := itnFromEnv(t)
	c, ctx := NewClient(uri, key), context.Background()
	for i := 0; i < 3; i++ {
		if _, err := c.InternalLogs(ctx, 0); err != nil {
			t.Fatal(err)
		}
	}
	seven := 7
	if limit, err := c.SetZkappCommandLimit(ctx, &seven); err != nil || limit == nil || *limit != 7 {
		t.Fatalf("limit %v, err %v", limit, err)
	}
	if limit, err := c.SetZkappCommandLimit(ctx, nil); err != nil || limit != nil {
		t.Fatalf("limit %v, err %v", limit, err)
	}
}

// Two clients with the same key share the daemon's sequence number: after
// the second one sends, the first one's number is stale (412), and it must
// recover with a new auth.
func TestIntegrationStaleSequenceNumber(t *testing.T) {
	uri, key := itnFromEnv(t)
	a, b, ctx := NewClient(uri, key), NewClient(uri, key), context.Background()
	for _, c := range []*Client{a, b, a} {
		if _, err := c.InternalLogs(ctx, 0); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntegrationInternalLogsAndFlush(t *testing.T) {
	uri, key := itnFromEnv(t)
	c, ctx := NewClient(uri, key), context.Background()
	logs, err := c.InternalLogs(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d internal logs", len(logs))
	if len(logs) == 0 {
		return
	}
	last := logs[len(logs)-1].ID
	if _, err := c.FlushInternalLogs(ctx, last); err != nil {
		t.Fatal(err)
	}
	rest, err := c.InternalLogs(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range rest {
		if l.ID <= last {
			t.Errorf("log %d survived a flush up to %d", l.ID, last)
		}
	}
}

// A block producer answers with its slots, or with a GraphQL error while its
// VRF evaluation runs; anything else is a transport or schema problem.
func TestIntegrationSlotsWon(t *testing.T) {
	uri, key := itnFromEnv(t)
	var gqlErr *mina.GraphQLError
	if _, err := NewClient(uri, key).SlotsWon(context.Background()); err != nil && !errors.As(err, &gqlErr) {
		t.Fatal(err)
	}
}

// An empty update checks that the input matches the daemon's GatingUpdate.
func TestIntegrationUpdateGatingEmpty(t *testing.T) {
	uri, key := itnFromEnv(t)
	if _, err := NewClient(uri, key).UpdateGating(context.Background(), GatingUpdate{}); err != nil {
		t.Fatal(err)
	}
}
