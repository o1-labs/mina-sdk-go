package mina

// Tests of the common API (spec/SPEC.md) against a mock daemon: the methods
// this SDK did not have, the new result fields, and the variables sent.

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// recorder is a mock daemon that answers with data and records the last
// request's query and variables.
type recorder struct {
	data      any
	query     string
	variables map[string]any
}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	r.query, r.variables = body.Query, body.Variables
	_ = json.NewEncoder(w).Encode(map[string]any{"data": r.data})
}

func mockDaemon(t *testing.T, data any) (*Client, *recorder) {
	t.Helper()
	rec := &recorder{data: data}
	client, srv := newTestClient(rec)
	t.Cleanup(func() { client.Close(); srv.Close() })
	return client, rec
}

// decodeJSON parses a JSON text into a generic value, for comparison with
// the recorded variables.
func decodeJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func assertVariables(t *testing.T, rec *recorder, want string) {
	t.Helper()
	got := any(rec.variables)
	if !reflect.DeepEqual(got, decodeJSON(t, want)) {
		t.Errorf("variables = %v, want %s", rec.variables, want)
	}
}

func blockJSON() map[string]any {
	epoch := func(seed, hash string) map[string]any {
		return map[string]any{"epochLength": "7", "seed": seed, "ledger": map[string]any{"hash": hash}}
	}
	return map[string]any{
		"stateHash":               "3NKblock",
		"commandTransactionCount": 1,
		"creatorAccount":          map[string]any{"publicKey": "B62qcreator"},
		"protocolState": map[string]any{
			"previousStateHash": "3NKprev",
			"consensusState": map[string]any{
				"blockHeight":       "12",
				"epoch":             "1",
				"slot":              "30",
				"slotSinceGenesis":  "40",
				"blockCreator":      "B62qcreator",
				"coinbaseReceiever": "B62qcoinbase",
				"stakingEpochData":  epoch("seedS", "jxS"),
				"nextEpochData":     epoch("seedN", "jxN"),
			},
			"blockchainState": map[string]any{
				"date":              "1700000000000",
				"utcDate":           "1700000000001",
				"snarkedLedgerHash": "jxSnarked",
				"stagedLedgerHash":  "jxStaged",
			},
		},
		"transactions": map[string]any{
			"coinbase":                "720000000000",
			"coinbaseReceiverAccount": map[string]any{"publicKey": "B62qcoinbase"},
			"feeTransfer": []any{
				map[string]any{"recipient": "B62qfee", "fee": "5", "type": "Fee_transfer"},
			},
			"userCommands": []any{map[string]any{
				"id": "Cmd1", "hash": "5Jhash", "kind": "PAYMENT", "nonce": 3,
				"source":   map[string]any{"publicKey": "B62qsrc"},
				"receiver": map[string]any{"publicKey": "B62qdst"},
				"amount":   "1000", "fee": "10", "memo": "E4Y", "failureReason": nil,
			}},
		},
	}
}

func TestBestChainReadsTheCommonBlockSelection(t *testing.T) {
	client, rec := mockDaemon(t, map[string]any{"bestChain": []any{blockJSON()}})
	blocks, err := client.GetBestChain(0)
	if err != nil {
		t.Fatal(err)
	}
	assertVariables(t, rec, `{"maxLength": null}`)
	b := blocks[0]
	if b.Height != 12 || b.GlobalSlotSinceHardFork != 30 || b.GlobalSlotSinceGenesis != 40 || b.Epoch != 1 {
		t.Errorf("numbers: %+v", b)
	}
	if b.PreviousStateHash != "3NKprev" || b.BlockCreator != "B62qcreator" || b.CoinbaseReceiver != "B62qcoinbase" {
		t.Errorf("consensus fields: %+v", b)
	}
	if b.StakingEpochLength != 7 || b.StakingEpochSeed != "seedS" || b.NextEpochLedgerHash != "jxN" {
		t.Errorf("epoch fields: %+v", b)
	}
	if b.Date != "1700000000000" || b.UTCDate != "1700000000001" {
		t.Errorf("dates: %q %q", b.Date, b.UTCDate)
	}
	if b.Coinbase != "720000000000" || b.CoinbaseReceiverAccount != "B62qcoinbase" || b.FeeTransferCount != 1 {
		t.Errorf("coinbase: %+v", b)
	}
	if want := (FeeTransfer{Recipient: "B62qfee", Fee: CurrencyFromNanomina(5), Type: "Fee_transfer"}); b.FeeTransfers[0] != want {
		t.Errorf("fee transfer = %+v", b.FeeTransfers[0])
	}
	u := b.UserCommands[0]
	if u.ID != "Cmd1" || u.Nonce != 3 || u.Source != "B62qsrc" || u.Receiver != "B62qdst" ||
		u.Amount != CurrencyFromNanomina(1000) || u.Fee != CurrencyFromNanomina(10) || u.FailureReason != nil {
		t.Errorf("user command = %+v", u)
	}
}

func TestGenesisBlockAndBlockByHashOrHeight(t *testing.T) {
	client, _ := mockDaemon(t, map[string]any{"genesisBlock": blockJSON()})
	g, err := client.GetGenesisBlock()
	if err != nil || g.StateHash != "3NKblock" {
		t.Fatalf("genesis = %+v, %v", g, err)
	}

	client, rec := mockDaemon(t, map[string]any{"block": blockJSON()})
	height := 12
	if _, err := client.GetBlock(BlockRef{Height: &height}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rec.query, "query Block(") {
		t.Errorf("query = %q", rec.query)
	}
	assertVariables(t, rec, `{"stateHash": null, "height": 12}`)
	if _, err := client.GetBlock(BlockRef{StateHash: "3NKblock"}); err != nil {
		t.Fatal(err)
	}
	assertVariables(t, rec, `{"stateHash": "3NKblock", "height": null}`)

	for _, bad := range []BlockRef{{}, {StateHash: "3NK", Height: &height}} {
		if _, err := client.GetBlock(bad); err == nil {
			t.Errorf("GetBlock(%+v) did not fail", bad)
		}
	}
}

func TestDaemonStatusNewFieldsAndMetrics(t *testing.T) {
	client, _ := mockDaemon(t, map[string]any{"daemonStatus": map[string]any{
		"syncStatus":                            "SYNCED",
		"highestUnvalidatedBlockLengthReceived": 9,
		"numAccounts":                           5,
		"ledgerMerkleRoot":                      "jxRoot",
		"chainId":                               "chain",
		"catchupStatus":                         nil,
		"blockProductionKeys":                   []string{"B62qbp"},
		"coinbaseReceiver":                      "B62qcb",
		"commitId":                              "abc",
		"peers":                                 []any{},
		"addrsAndPorts": map[string]any{
			"externalIp": "1.2.3.4", "bindIp": "0.0.0.0", "clientPort": 8301, "libp2pPort": 8302,
		},
	}})
	s, err := client.GetDaemonStatus()
	if err != nil {
		t.Fatal(err)
	}
	if *s.HighestUnvalidatedBlockLengthReceived != 9 || *s.NumAccounts != 5 || s.LedgerMerkleRoot != "jxRoot" ||
		s.ChainID != "chain" || s.CatchupStatus != nil || s.BlockProductionKeys[0] != "B62qbp" || s.CoinbaseReceiver != "B62qcb" {
		t.Errorf("status = %+v", s)
	}
	if want := (AddrsAndPorts{ExternalIP: "1.2.3.4", BindIP: "0.0.0.0", ClientPort: 8301, Libp2pPort: 8302}); *s.AddrsAndPorts != want {
		t.Errorf("addrsAndPorts = %+v", s.AddrsAndPorts)
	}

	client, _ = mockDaemon(t, map[string]any{"daemonStatus": map[string]any{"metrics": map[string]any{
		"blockProductionDelay":           []int{1, 2},
		"transactionPoolDiffReceived":    3,
		"transactionPoolDiffBroadcasted": 4,
		"transactionsAddedToPool":        5,
		"transactionPoolSize":            6,
		"snarkPoolDiffReceived":          7,
		"snarkPoolDiffBroadcasted":       8,
		"pendingSnarkWork":               9,
		"snarkPoolSize":                  10,
	}}})
	m, err := client.GetDaemonMetrics()
	if err != nil {
		t.Fatal(err)
	}
	want := DaemonMetrics{[]int{1, 2}, 3, 4, 5, 6, 7, 8, 9, 10}
	if !reflect.DeepEqual(*m, want) {
		t.Errorf("metrics = %+v", m)
	}
}

func TestAccountSendsANullTokenAndReadsNewFields(t *testing.T) {
	client, rec := mockDaemon(t, map[string]any{"account": map[string]any{
		"publicKey": "B62qacc", "nonce": "4", "delegate": nil, "tokenId": "wSHV",
		"tokenSymbol": "MINA", "votingFor": "3NKvote", "receiptChainHash": "2mzReceipt",
		"balance": map[string]any{"total": "100", "liquid": "60", "locked": "40", "blockHeight": "12"},
		"timing": map[string]any{
			"initialMinimumBalance": "50", "cliffTime": "10", "cliffAmount": "5",
			"vestingPeriod": "2", "vestingIncrement": "1",
		},
		"permissions": map[string]any{
			"editState": "Signature", "send": "Signature", "receive": "None", "access": "None",
			"setDelegate": "Signature", "setPermissions": "Signature",
			"setVerificationKey": map[string]any{"auth": "Signature", "txnVersion": "3"},
			"setZkappUri":        "Signature", "editActionState": "Signature", "setTokenSymbol": "Signature",
			"incrementNonce": "Signature", "setVotingFor": "Signature", "setTiming": "Signature",
		},
		"zkappState": []string{"0", "1"}, "provedState": false, "zkappUri": "",
	}})
	a, err := client.GetAccount("B62qacc", "")
	if err != nil {
		t.Fatal(err)
	}
	assertVariables(t, rec, `{"publicKey": "B62qacc", "token": null}`)
	if a.Nonce != 4 || a.Delegate != "" || a.TokenSymbol != "MINA" || a.VotingFor != "3NKvote" || a.ReceiptChainHash != "2mzReceipt" {
		t.Errorf("account = %+v", a)
	}
	if *a.Balance.BlockHeight != 12 || *a.Balance.Liquid != CurrencyFromNanomina(60) || *a.Balance.Locked != CurrencyFromNanomina(40) {
		t.Errorf("balance = %+v", a.Balance)
	}
	if *a.Timing.InitialMinimumBalance != CurrencyFromNanomina(50) || *a.Timing.CliffTime != 10 ||
		*a.Timing.VestingPeriod != 2 || *a.Timing.VestingIncrement != CurrencyFromNanomina(1) {
		t.Errorf("timing = %+v", a.Timing)
	}
	if a.Permissions.Receive != "None" || a.Permissions.SetVerification.TxnVersion != "3" {
		t.Errorf("permissions = %+v", a.Permissions)
	}
	if len(a.ZkappState) != 2 || *a.ProvedState {
		t.Errorf("zkApp fields = %v %v", a.ZkappState, a.ProvedState)
	}

	// An untimed account: the daemon sends a timing object of nulls.
	client, _ = mockDaemon(t, map[string]any{"account": map[string]any{
		"publicKey": "B62qacc", "nonce": "0", "tokenId": "wSHV",
		"balance": map[string]any{"total": "1"},
		"timing": map[string]any{
			"initialMinimumBalance": nil, "cliffTime": nil, "cliffAmount": nil,
			"vestingPeriod": nil, "vestingIncrement": nil,
		},
	}})
	a, err = client.GetAccount("B62qacc", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Timing != nil || a.Permissions != nil || a.ZkappState != nil {
		t.Errorf("untimed account = %+v", a)
	}
}

func TestPooledCommands(t *testing.T) {
	client, rec := mockDaemon(t, map[string]any{"pooledUserCommands": []any{map[string]any{
		"id": "Cmd", "hash": "5J", "kind": "PAYMENT", "nonce": 2, "amount": "1", "fee": "2",
		"from": "B62qa", "to": "B62qb",
		"source": map[string]any{"publicKey": "B62qa"}, "receiver": map[string]any{"publicKey": "B62qb"},
		"memo": "E4Y", "failureReason": nil,
	}}})
	cmds, err := client.GetPooledUserCommands("")
	if err != nil {
		t.Fatal(err)
	}
	assertVariables(t, rec, `{"publicKey": null}`)
	if c := cmds[0]; c.Nonce != "2" || c.Source != "B62qa" || c.Receiver != "B62qb" || c.Memo != "E4Y" || c.FailureReason != nil {
		t.Errorf("command = %+v", c)
	}

	client, rec = mockDaemon(t, map[string]any{"pooledZkappCommands": []any{map[string]any{
		"id": "Zk", "hash": "5Jz",
		"zkappCommand": map[string]any{"memo": "E4Y", "feePayer": map[string]any{"body": map[string]any{
			"publicKey": "B62qpayer", "fee": "100", "nonce": "7", "validUntil": nil,
		}}},
		"failureReason": []any{map[string]any{"index": "1", "failures": []string{"Invalid_fee_excess"}}},
	}}})
	zk, err := client.GetPooledZkappCommands("B62qpayer")
	if err != nil {
		t.Fatal(err)
	}
	assertVariables(t, rec, `{"publicKey": "B62qpayer"}`)
	z := zk[0]
	if z.ID != "Zk" || z.Memo != "E4Y" || z.FeePayer.PublicKey != "B62qpayer" || z.FeePayer.Fee != CurrencyFromNanomina(100) ||
		z.FeePayer.Nonce != 7 || z.FeePayer.ValidUntil != nil {
		t.Errorf("zkApp command = %+v", z)
	}
	if *z.FailureReason[0].Index != 1 || z.FailureReason[0].Failures[0] != "Invalid_fee_excess" {
		t.Errorf("failure = %+v", z.FailureReason)
	}
}

func TestSendPaymentWithAndWithoutSignature(t *testing.T) {
	submitted := map[string]any{"sendPayment": map[string]any{"payment": map[string]any{
		"id": "Cmd", "hash": "5J", "kind": "PAYMENT", "nonce": "3",
		"source": map[string]any{"publicKey": "B62qa"}, "receiver": map[string]any{"publicKey": "B62qb"},
		"amount": "1000", "fee": "10", "memo": "E4Y",
	}}}
	client, rec := mockDaemon(t, submitted)
	params := SendPaymentParams{Sender: "B62qa", Receiver: "B62qb", Amount: CurrencyFromNanomina(1000), Fee: CurrencyFromNanomina(10)}
	r, err := client.SendPayment(params)
	if err != nil {
		t.Fatal(err)
	}
	assertVariables(t, rec, `{"input": {"from": "B62qa", "to": "B62qb", "amount": "1000", "fee": "10"}, "signature": null}`)
	if r.Nonce != 3 || r.Kind != "PAYMENT" || r.Source != "B62qa" || r.Receiver != "B62qb" ||
		*r.Amount != CurrencyFromNanomina(1000) || *r.Fee != CurrencyFromNanomina(10) || r.Memo != "E4Y" {
		t.Errorf("result = %+v", r)
	}

	params.Signature = &SignatureInput{Field: "123", Scalar: "456"}
	if _, err := client.SendPayment(params); err != nil {
		t.Fatal(err)
	}
	if got := rec.variables["signature"]; !reflect.DeepEqual(got, decodeJSON(t, `{"field": "123", "scalar": "456"}`)) {
		t.Errorf("signature = %v", got)
	}

	client, rec = mockDaemon(t, map[string]any{"sendDelegation": map[string]any{"delegation": map[string]any{
		"id": "Del", "hash": "5Jd", "kind": "STAKE_DELEGATION", "nonce": "4",
		"source": map[string]any{"publicKey": "B62qa"}, "receiver": map[string]any{"publicKey": "B62qbp"},
		"amount": nil, "fee": "10", "memo": "E4Y",
	}}})
	d, err := client.SendDelegation(SendDelegationParams{Sender: "B62qa", DelegateTo: "B62qbp", Fee: CurrencyFromNanomina(10)})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.variables["signature"]; !ok {
		t.Error("the signature variable is not sent")
	}
	if d.Amount != nil || d.Receiver != "B62qbp" || d.Nonce != 4 {
		t.Errorf("delegation = %+v", d)
	}
}

func TestSendZkapp(t *testing.T) {
	client, rec := mockDaemon(t, map[string]any{"sendZkapp": map[string]any{"zkapp": map[string]any{
		"id": "Zk", "hash": "5Jz",
		"zkappCommand": map[string]any{"memo": "E4Y", "feePayer": map[string]any{"body": map[string]any{
			"publicKey": "B62qpayer", "fee": "100", "nonce": "1", "validUntil": "99",
		}}},
		"failureReason": nil,
	}}})
	command := json.RawMessage(`{"feePayer": {}, "accountUpdates": [], "memo": ""}`)
	z, err := client.SendZkapp(command)
	if err != nil {
		t.Fatal(err)
	}
	assertVariables(t, rec, `{"input": {"zkappCommand": {"feePayer": {}, "accountUpdates": [], "memo": ""}}}`)
	if z.Hash != "5Jz" || *z.FeePayer.ValidUntil != 99 || z.FailureReason != nil {
		t.Errorf("result = %+v", z)
	}
}

func TestTransactionStatusAndSmallQueries(t *testing.T) {
	client, rec := mockDaemon(t, map[string]any{"transactionStatus": "INCLUDED"})
	s, err := client.GetTransactionStatus(TransactionRef{Zkapp: "5Jz"})
	if err != nil || s != TransactionStatusIncluded {
		t.Fatalf("status = %q, %v", s, err)
	}
	assertVariables(t, rec, `{"payment": null, "zkappTransaction": "5Jz"}`)
	if _, err := client.GetTransactionStatus(TransactionRef{Payment: "Cmd"}); err != nil {
		t.Fatal(err)
	}
	assertVariables(t, rec, `{"payment": "Cmd", "zkappTransaction": null}`)
	for _, bad := range []TransactionRef{{}, {Payment: "a", Zkapp: "b"}} {
		if _, err := client.GetTransactionStatus(bad); err == nil {
			t.Errorf("GetTransactionStatus(%+v) did not fail", bad)
		}
	}

	client, _ = mockDaemon(t, map[string]any{"genesisConstants": map[string]any{
		"genesisTimestamp": "2024-01-01T00:00:00Z", "coinbase": "720000000000", "accountCreationFee": "1000000000",
	}})
	g, err := client.GetGenesisConstants()
	if err != nil {
		t.Fatal(err)
	}
	if want := (GenesisConstants{"2024-01-01T00:00:00Z", CurrencyFromNanomina(720000000000), CurrencyFromNanomina(1000000000)}); *g != want {
		t.Errorf("constants = %+v", g)
	}

	client, _ = mockDaemon(t, map[string]any{"trackedAccounts": []any{
		map[string]any{"publicKey": "B62qt", "balance": map[string]any{"total": "42"}},
	}})
	tracked, err := client.GetTrackedAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if want := (TrackedAccount{"B62qt", CurrencyFromNanomina(42)}); tracked[0] != want {
		t.Errorf("tracked = %+v", tracked)
	}

	client, _ = mockDaemon(t, map[string]any{"snarkPool": []any{
		map[string]any{"fee": "10", "prover": "B62qp", "workIds": []any{3, "4"}},
	}})
	work, err := client.GetSnarkPool()
	if err != nil {
		t.Fatal(err)
	}
	if want := (CompletedWork{Prover: "B62qp", Fee: CurrencyFromNanomina(10), WorkIDs: []int{3, 4}}); !reflect.DeepEqual(work[0], want) {
		t.Errorf("snark pool = %+v", work)
	}
}
