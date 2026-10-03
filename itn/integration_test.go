package itn

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

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

// Operations for harness support (MinaProtocol/mina#19616). They run only with
// MINA_ITN_HARNESS=1, because older daemons do not have them.
func harnessFromEnv(t *testing.T) (*Client, context.Context) {
	t.Helper()
	uri, key := itnFromEnv(t)
	if os.Getenv("MINA_ITN_HARNESS") != "1" {
		t.Skip("MINA_ITN_HARNESS=1 not set")
	}
	return NewClient(uri, key), context.Background()
}

func TestIntegrationCommitIDAndListing(t *testing.T) {
	c, ctx := harnessFromEnv(t)
	if id, err := c.CommitID(ctx); err != nil || len(id) < 7 {
		t.Fatalf("commit %q, err %v", id, err)
	}
	if _, err := c.ScheduledTransactions(ctx); err != nil {
		t.Fatal(err)
	}
}

// CreateAccounts sends transactions, so it also needs MINA_ITN_FEE_PAYER: the
// base58 private key of a funded account.
func TestIntegrationCreateAccounts(t *testing.T) {
	c, ctx := harnessFromEnv(t)
	feePayer := os.Getenv("MINA_ITN_FEE_PAYER")
	if feePayer == "" {
		t.Skip("MINA_ITN_FEE_PAYER not set")
	}
	handle := newUUID(t)
	d := CreateAccountsDetails{
		FeePayer: feePayer, NumAccounts: 3,
		Fee: mina.MustCurrencyFromString("0.1"), Amount: mina.MustCurrencyFromString("6"),
	}
	created, err := c.CreateAccounts(ctx, d, handle)
	if err != nil {
		t.Fatal(err)
	}
	if created.Handle != handle || len(created.Accounts) != 3 {
		t.Fatalf("created %+v", created)
	}
	again, err := c.CreateAccounts(ctx, d, handle)
	if err != nil {
		t.Fatal(err)
	}
	if again.Accounts[0].PublicKey != created.Accounts[0].PublicKey {
		t.Error("a repeat with the same handle created other accounts")
	}
	deadline := time.Now().Add(10 * time.Minute)
	for {
		handles, err := c.ScheduledTransactions(ctx)
		if err != nil {
			t.Fatal(err)
		}
		listed := false
		for _, h := range handles {
			listed = listed || h == handle
		}
		if !listed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("handle %s still listed after 10 minutes", handle)
		}
		time.Sleep(5 * time.Second)
	}
}

// newUUID returns a random (version 4) UUID.
func newUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
