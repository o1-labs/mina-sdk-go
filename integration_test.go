package mina_test

import (
	"errors"
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
		if err == nil && status == mina.SyncStatusSynced {
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

// waitForPaymentAcceptance polls the pending pool for txHash. The transaction is
// considered accepted by the network if it appears in the pool at least once
// (proof the daemon received it) OR if it disappears from the pool after being
// seen there, meaning it was included in a block. Only fails if the tx is never
// observed in the pool at all within the deadline.
func waitForPaymentAcceptance(t *testing.T, client *mina.Client, sender, txHash string) {
	t.Helper()
	maxWait := 10 * time.Second
	poll := 300 * time.Millisecond
	start := time.Now()

	for time.Since(start) < maxWait {
		cmds, err := client.GetPooledUserCommands(sender)
		if err != nil {
			t.Logf("Error polling pool: %v (%v)", err, time.Since(start).Round(time.Millisecond))
			time.Sleep(poll)
			continue
		}
		for _, cmd := range cmds {
			if cmd.Hash == txHash {
				return // still pending — accepted
			}
		}
		time.Sleep(poll)
	}

	t.Fatalf("transaction %s was never observed in the pool within %v (daemon may not have accepted it)", txHash, maxWait)
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
	if status.SyncStatus != mina.SyncStatusSynced {
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

	waitForPaymentAcceptance(t, client, senderKey(), result.Hash)
}

// -- Common API (spec/SPEC.md) --

func TestIntegrationDaemonMetrics(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	metrics, err := client.GetDaemonMetrics()
	if err != nil {
		t.Fatal(err)
	}
	if metrics.TransactionPoolSize < 0 {
		t.Errorf("transaction pool size %d", metrics.TransactionPoolSize)
	}
}

func TestIntegrationGenesisBlockAndBlockLookups(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	genesis, err := client.GetGenesisBlock()
	if err != nil {
		t.Fatal(err)
	}
	if genesis.StateHash == "" || genesis.StakingEpochLedgerHash == "" {
		t.Errorf("genesis block = %+v", genesis)
	}

	// The best tip is always in the transition frontier, so it can be read
	// back by state hash and by height.
	chain, err := client.GetBestChain(1)
	if err != nil || len(chain) == 0 {
		t.Fatalf("best chain = %v, %v", chain, err)
	}
	tip := chain[0]
	byHash, err := client.GetBlock(mina.BlockRef{StateHash: tip.StateHash})
	if err != nil {
		t.Fatal(err)
	}
	if byHash.Height != tip.Height || byHash.PreviousStateHash != tip.PreviousStateHash {
		t.Errorf("by hash = %+v, tip = %+v", byHash, tip)
	}
	byHeight, err := client.GetBlock(mina.BlockRef{Height: &tip.Height})
	if err != nil {
		t.Fatal(err)
	}
	if byHeight.StateHash != tip.StateHash {
		t.Errorf("by height = %s, tip = %s", byHeight.StateHash, tip.StateHash)
	}
}

func TestIntegrationGenesisConstantsAndPools(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	constants, err := client.GetGenesisConstants()
	if err != nil {
		t.Fatal(err)
	}
	if constants.GenesisTimestamp == "" {
		t.Error("empty genesis timestamp")
	}
	if _, err := client.GetSnarkPool(); err != nil {
		t.Error(err)
	}
	if _, err := client.GetPooledZkappCommands(""); err != nil {
		t.Error(err)
	}
	if _, err := client.GetTrackedAccounts(); err != nil {
		t.Error(err)
	}
}

func TestIntegrationTransactionStatusOfAnUnknownPayment(t *testing.T) {
	skipNoDaemon(t)
	client := newIntegrationClient(t)
	defer client.Close()
	waitForSync(t, client)

	// An ID that does not decode is a GraphQL error, not a transport one.
	_, err := client.GetTransactionStatus(mina.TransactionRef{Payment: "not-an-id"})
	var gqlErr *mina.GraphQLError
	if !errors.As(err, &gqlErr) {
		t.Errorf("got %v, want a GraphQL error", err)
	}
}
