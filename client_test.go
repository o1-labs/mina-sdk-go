package mina

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func gqlHandler(data any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}
}

func gqlErrorHandler(errors []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": errors})
	}
}

func newTestClient(handler http.Handler) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	client := NewClient(
		WithGraphQLURI(srv.URL),
		WithRetries(1),
		WithRetryDelay(0),
	)
	return client, srv
}

func TestGetSyncStatusSynced(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{"syncStatus": "SYNCED"}))
	defer srv.Close()
	defer client.Close()

	status, err := client.GetSyncStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status != SyncStatusSynced {
		t.Errorf("expected SYNCED, got %s", status)
	}
}

func TestGetSyncStatusBootstrap(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{"syncStatus": "BOOTSTRAP"}))
	defer srv.Close()
	defer client.Close()

	status, err := client.GetSyncStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status != SyncStatusBootstrap {
		t.Errorf("expected BOOTSTRAP, got %s", status)
	}
}

func TestGetDaemonStatus(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{
		"daemonStatus": map[string]any{
			"syncStatus":                 "SYNCED",
			"blockchainLength":           100,
			"highestBlockLengthReceived": 100,
			"uptimeSecs":                 3600,
			"stateHash":                  "3NKtest...",
			"commitId":                   "abc123",
			"peers": []map[string]any{
				{"peerId": "peer1", "host": "1.2.3.4", "libp2pPort": 8302},
			},
		},
	}))
	defer srv.Close()
	defer client.Close()

	status, err := client.GetDaemonStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.SyncStatus != SyncStatusSynced {
		t.Errorf("expected SYNCED, got %s", status.SyncStatus)
	}
	if status.BlockchainLength == nil || *status.BlockchainLength != 100 {
		t.Error("expected blockchain length 100")
	}
	if status.UptimeSecs == nil || *status.UptimeSecs != 3600 {
		t.Error("expected uptime 3600")
	}
	if len(status.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(status.Peers))
	}
	if status.Peers[0].PeerID != "peer1" {
		t.Errorf("expected peer1, got %s", status.Peers[0].PeerID)
	}
	if status.Peers[0].Host != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", status.Peers[0].Host)
	}
	if status.Peers[0].Port != 8302 {
		t.Errorf("expected 8302, got %d", status.Peers[0].Port)
	}
}

func TestGetNetworkID(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{"networkID": "mina:testnet"}))
	defer srv.Close()
	defer client.Close()

	id, err := client.GetNetworkID()
	if err != nil {
		t.Fatal(err)
	}
	if id != "mina:testnet" {
		t.Errorf("expected mina:testnet, got %s", id)
	}
}

func TestGetAccount(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{
		"account": map[string]any{
			"publicKey": "B62qtest...",
			"nonce":     "5",
			"delegate":  "B62qdelegate...",
			"tokenId":   "1",
			"balance": map[string]any{
				"total":  "1500000000000",
				"liquid": "1000000000000",
				"locked": "500000000000",
			},
		},
	}))
	defer srv.Close()
	defer client.Close()

	account, err := client.GetAccount("B62qtest...", "")
	if err != nil {
		t.Fatal(err)
	}
	if account.PublicKey != "B62qtest..." {
		t.Errorf("expected B62qtest..., got %s", account.PublicKey)
	}
	if account.Nonce != 5 {
		t.Errorf("expected nonce 5, got %d", account.Nonce)
	}
	if account.Delegate != "B62qdelegate..." {
		t.Errorf("expected B62qdelegate..., got %s", account.Delegate)
	}
	if !account.Balance.Total.Equal(NewCurrency(1500)) {
		t.Errorf("expected total 1500, got %s", account.Balance.Total)
	}
	if account.Balance.Liquid == nil || !account.Balance.Liquid.Equal(NewCurrency(1000)) {
		t.Error("expected liquid 1000")
	}
	if account.Balance.Locked == nil || !account.Balance.Locked.Equal(NewCurrency(500)) {
		t.Error("expected locked 500")
	}
}

func TestGetAccountNotFound(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{"account": nil}))
	defer srv.Close()
	defer client.Close()

	_, err := client.GetAccount("B62qnotfound...", "")
	if err == nil {
		t.Fatal("expected error")
	}
	var nf *AccountNotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("expected AccountNotFoundError, got %T: %v", err, err)
	}
}

func TestGetBestChain(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{
		"bestChain": []map[string]any{
			{
				"stateHash":               "3NKhash1",
				"commandTransactionCount": 3,
				"creatorAccount":          map[string]any{"publicKey": "B62qcreator..."},
				"protocolState": map[string]any{
					"consensusState": map[string]any{
						"blockHeight":      "50",
						"slotSinceGenesis": "1000",
						"slot":             "500",
					},
				},
			},
		},
	}))
	defer srv.Close()
	defer client.Close()

	blocks, err := client.GetBestChain(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].StateHash != "3NKhash1" {
		t.Errorf("expected 3NKhash1, got %s", blocks[0].StateHash)
	}
	if blocks[0].Height != 50 {
		t.Errorf("expected height 50, got %d", blocks[0].Height)
	}
	if blocks[0].CreatorPK != "B62qcreator..." {
		t.Errorf("expected B62qcreator..., got %s", blocks[0].CreatorPK)
	}
	if blocks[0].CommandTransactionCount != 3 {
		t.Errorf("expected 3 txns, got %d", blocks[0].CommandTransactionCount)
	}
}

func TestGetBestChainEmpty(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{"bestChain": nil}))
	defer srv.Close()
	defer client.Close()

	blocks, err := client.GetBestChain(0)
	if err != nil {
		t.Fatal(err)
	}
	if blocks != nil {
		t.Errorf("expected nil, got %v", blocks)
	}
}

func TestSendPayment(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{
		"sendPayment": map[string]any{
			"payment": map[string]any{
				"id":    "txn-id-123",
				"hash":  "CkpHash...",
				"nonce": "6",
			},
		},
	}))
	defer srv.Close()
	defer client.Close()

	result, err := client.SendPayment(SendPaymentParams{
		Sender:   "B62qsender...",
		Receiver: "B62qreceiver...",
		Amount:   NewCurrency(10),
		Fee:      MustCurrencyFromString("0.1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "txn-id-123" {
		t.Errorf("expected txn-id-123, got %s", result.ID)
	}
	if result.Hash != "CkpHash..." {
		t.Errorf("expected CkpHash..., got %s", result.Hash)
	}
	if result.Nonce != 6 {
		t.Errorf("expected nonce 6, got %d", result.Nonce)
	}
}

func TestSendDelegation(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{
		"sendDelegation": map[string]any{
			"delegation": map[string]any{
				"id":    "del-id-456",
				"hash":  "CkpDel...",
				"nonce": "7",
			},
		},
	}))
	defer srv.Close()
	defer client.Close()

	result, err := client.SendDelegation(SendDelegationParams{
		Sender:     "B62qsender...",
		DelegateTo: "B62qdelegate...",
		Fee:        MustCurrencyFromString("0.1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "del-id-456" {
		t.Errorf("expected del-id-456, got %s", result.ID)
	}
	if result.Hash != "CkpDel..." {
		t.Errorf("expected CkpDel..., got %s", result.Hash)
	}
	if result.Nonce != 7 {
		t.Errorf("expected nonce 7, got %d", result.Nonce)
	}
}

func TestGraphQLError(t *testing.T) {
	client, srv := newTestClient(gqlErrorHandler([]map[string]any{
		{"message": "field not found"},
	}))
	defer srv.Close()
	defer client.Close()

	_, err := client.GetSyncStatus()
	if err == nil {
		t.Fatal("expected error")
	}
	var gqlErr *GraphQLError
	if !errors.As(err, &gqlErr) {
		t.Errorf("expected GraphQLError, got %T: %v", err, err)
	}
}

func TestConnectionErrorAfterRetries(t *testing.T) {
	// Point to a non-existent server
	client := NewClient(
		WithGraphQLURI("http://127.0.0.1:1/graphql"),
		WithRetries(2),
		WithRetryDelay(0),
		WithTimeout(100*time.Millisecond),
	)
	defer client.Close()

	_, err := client.GetSyncStatus()
	if err == nil {
		t.Fatal("expected error")
	}
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Errorf("expected ConnectionError, got %T: %v", err, err)
	}
}

func TestGetPeers(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{
		"getPeers": []map[string]any{
			{"peerId": "p1", "host": "10.0.0.1", "libp2pPort": 8302},
			{"peerId": "p2", "host": "10.0.0.2", "libp2pPort": 8302},
		},
	}))
	defer srv.Close()
	defer client.Close()

	peers, err := client.GetPeers()
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}
	if peers[0].PeerID != "p1" {
		t.Errorf("expected p1, got %s", peers[0].PeerID)
	}
	if peers[1].Host != "10.0.0.2" {
		t.Errorf("expected 10.0.0.2, got %s", peers[1].Host)
	}
}

func TestGetPooledUserCommands(t *testing.T) {
	client, srv := newTestClient(gqlHandler(map[string]any{
		"pooledUserCommands": []map[string]any{
			{
				"id":     "cmd1",
				"hash":   "CkpHash1",
				"kind":   "PAYMENT",
				"nonce":  "1",
				"amount": "1000000000",
				"fee":    "10000000",
				"from":   "B62qsender...",
				"to":     "B62qreceiver...",
			},
		},
	}))
	defer srv.Close()
	defer client.Close()

	cmds, err := client.GetPooledUserCommands("B62qsender...")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Kind != "PAYMENT" {
		t.Errorf("expected PAYMENT, got %s", cmds[0].Kind)
	}
}
