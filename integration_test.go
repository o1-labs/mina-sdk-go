package mina_test

import (
	"os"
	"testing"
	"time"

	mina "github.com/MinaProtocol/mina-sdk-go"
)

// Integration tests require a running Mina daemon with GraphQL enabled.
// They are skipped unless MINA_GRAPHQL_URI is set.
//
// Usage:
//
//	MINA_GRAPHQL_URI=http://127.0.0.1:3085/graphql go test -v -run Integration
//
//	# With funded accounts (for payment tests):
//	MINA_GRAPHQL_URI=http://127.0.0.1:3001/graphql \
//	MINA_TEST_SENDER_KEY=B62q... \
//	MINA_TEST_RECEIVER_KEY=B62q... \
//	go test -v -run Integration

func graphqlURI() string  { return os.Getenv("MINA_GRAPHQL_URI") }
func senderKey() string   { return os.Getenv("MINA_TEST_SENDER_KEY") }
func receiverKey() string { return os.Getenv("MINA_TEST_RECEIVER_KEY") }

func skipNoDaemon(t *testing.T) {
	t.Helper()
	if graphqlURI() == "" {
		t.Skip("MINA_GRAPHQL_URI not set — no daemon available")
	}
}

func skipNoAccounts(t *testing.T) {
	t.Helper()
	if graphqlURI() == "" || senderKey() == "" || receiverKey() == "" {
		t.Skip("MINA_GRAPHQL_URI, MINA_TEST_SENDER_KEY, and MINA_TEST_RECEIVER_KEY must all be set")
	}
}

func newIntegrationClient(t *testing.T) *mina.Client {
	t.Helper()
	return mina.NewClient(
		mina.WithGraphQLURI(graphqlURI()),
		mina.WithRetries(5),
		mina.WithRetryDelay(10*time.Second),
		mina.WithTimeout(30*time.Second),
	)
}

func waitForSync(t *testing.T, client *mina.Client) {
	t.Helper()
	maxWait := 300 * time.Second
	poll := 5 * time.Second
	start := time.Now()
	for time.Since(start) < maxWait {
		status, err := client.GetSyncStatus()
		if err == nil && status == "SYNCED" {
			return
		}
		if err != nil {
			t.Logf("Waiting for daemon... %v (%v)", err, time.Since(start).Round(time.Second))
		} else {
			t.Logf("Waiting for SYNCED, current status: %s (%v)", status, time.Since(start).Round(time.Second))
		}
		time.Sleep(poll)
	}
	t.Fatalf("Daemon did not reach SYNCED within %v", maxWait)
}

// -- Read-only queries --

func TestIntegrationSyncStatus(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	status, err := client.GetSyncStatus()
	if err != nil {
		t.Fatal(err)
	}
	validStatuses := map[mina.SyncStatus]bool{
		mina.SyncStatusConnecting: true, mina.SyncStatusListening: true, mina.SyncStatusOffline: true,
		mina.SyncStatusBootstrap: true, mina.SyncStatusSynced: true, mina.SyncStatusCatchup: true,
	}
	if !validStatuses[status] {
		t.Errorf("unexpected sync status: %s", status)
	}
}

func TestIntegrationDaemonStatus(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	status, err := client.GetDaemonStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.SyncStatus != "SYNCED" {
		t.Errorf("expected SYNCED, got %s", status.SyncStatus)
	}
	if status.BlockchainLength == nil || *status.BlockchainLength <= 0 {
		t.Error("expected blockchain length > 0")
	}
}

func TestIntegrationNetworkID(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	networkID, err := client.GetNetworkID()
	if err != nil {
		t.Fatal(err)
	}
	if networkID == "" {
		t.Error("expected non-empty network ID")
	}
}

func TestIntegrationGetPeers(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	peers, err := client.GetPeers()
	if err != nil {
		t.Fatal(err)
	}
	if peers == nil {
		t.Error("expected non-nil peers list")
	}
}

func TestIntegrationBestChain(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	blocks, err := client.GetBestChain(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) == 0 {
		t.Fatal("expected at least 1 block")
	}
	block := blocks[0]
	if block.Height <= 0 {
		t.Error("expected block height > 0")
	}
	if block.StateHash == "" {
		t.Error("expected non-empty state hash")
	}
	if block.CreatorPK == "" {
		t.Error("expected non-empty creator public key")
	}
}

func TestIntegrationBestChainOrdering(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	blocks, err := client.GetBestChain(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) >= 2 {
		ascending := true
		descending := true
		for i := 0; i < len(blocks)-1; i++ {
			if blocks[i].Height > blocks[i+1].Height {
				ascending = false
			}
			if blocks[i].Height < blocks[i+1].Height {
				descending = false
			}
		}
		if !ascending && !descending {
			t.Error("blocks are not monotonically ordered")
		}
	}
}

func TestIntegrationPooledUserCommands(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	cmds, err := client.GetPooledUserCommands("")
	if err != nil {
		t.Fatal(err)
	}
	if cmds == nil {
		t.Error("expected non-nil commands list")
	}
}

// -- Account queries --

func TestIntegrationGetAccount(t *testing.T) {
	skipNoAccounts(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	account, err := client.GetAccount(senderKey(), "")
	if err != nil {
		t.Fatal(err)
	}
	if account.PublicKey != senderKey() {
		t.Errorf("expected %s, got %s", senderKey(), account.PublicKey)
	}
	if account.Nonce < 0 {
		t.Error("expected nonce >= 0")
	}
	if account.Balance.Total.Nanomina() == 0 {
		t.Error("expected non-zero total balance for funded account")
	}
}

func TestIntegrationAccountNotFound(t *testing.T) {
	skipNoAccounts(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	_, err := client.GetAccount("B62qpRzFVjd56FiHnNfxokVbcHMQLT119My1FEdSq8ss7KomLiSZcan", "")
	if err == nil {
		t.Error("expected error for non-existent account")
	}
}

// -- Mutations --

func TestIntegrationSendPayment(t *testing.T) {
	skipNoAccounts(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	result, err := client.SendPayment(mina.SendPaymentParams{
		Sender:   senderKey(),
		Receiver: receiverKey(),
		Amount:   mina.MustCurrencyFromString("0.001"),
		Fee:      mina.MustCurrencyFromString("0.01"),
		Memo:     "mina-sdk-go integration test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Hash == "" {
		t.Error("expected non-empty transaction hash")
	}
	if result.Nonce < 0 {
		t.Error("expected nonce >= 0")
	}
	if result.ID == "" {
		t.Error("expected non-empty transaction ID")
	}
}

func TestIntegrationSendDelegation(t *testing.T) {
	skipNoAccounts(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	result, err := client.SendDelegation(mina.SendDelegationParams{
		Sender:     senderKey(),
		DelegateTo: receiverKey(),
		Fee:        mina.MustCurrencyFromString("0.01"),
		Memo:       "mina-sdk-go delegation test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Hash == "" {
		t.Error("expected non-empty transaction hash")
	}
	if result.Nonce < 0 {
		t.Error("expected nonce >= 0")
	}
}

func TestIntegrationPaymentAppearsInPool(t *testing.T) {
	skipNoAccounts(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	result, err := client.SendPayment(mina.SendPaymentParams{
		Sender:   senderKey(),
		Receiver: receiverKey(),
		Amount:   mina.MustCurrencyFromString("0.001"),
		Fee:      mina.MustCurrencyFromString("0.01"),
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	cmds, err := client.GetPooledUserCommands(senderKey())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cmd := range cmds {
		if cmd.Hash == result.Hash {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("transaction %s not found in pool", result.Hash)
	}
}
