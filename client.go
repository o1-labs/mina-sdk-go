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

// peerNode is the GraphQL shape of a peer, in DaemonStatus and Peers.
type peerNode struct {
	PeerID     string `json:"peerId"`
	Host       string `json:"host"`
	Libp2pPort int    `json:"libp2pPort"`
}

func toPeers(nodes []peerNode) []PeerInfo {
	peers := make([]PeerInfo, len(nodes))
	for i, p := range nodes {
		peers[i] = PeerInfo{PeerID: p.PeerID, Host: p.Host, Port: p.Libp2pPort}
	}
	return peers
}

// GetDaemonStatus returns comprehensive daemon status.
func (c *Client) GetDaemonStatus() (*DaemonStatus, error) {
	data, err := c.request(queryDaemonStatus, nil, "get_daemon_status")
	if err != nil {
		return nil, err
	}
	var result struct {
		DaemonStatus struct {
			SyncStatus                            SyncStatus `json:"syncStatus"`
			BlockchainLength                      *int       `json:"blockchainLength"`
			HighestBlockLengthReceived            *int       `json:"highestBlockLengthReceived"`
			HighestUnvalidatedBlockLengthReceived *int       `json:"highestUnvalidatedBlockLengthReceived"`
			UptimeSecs                            *int       `json:"uptimeSecs"`
			StateHash                             *string    `json:"stateHash"`
			CommitID                              string     `json:"commitId"`
			NumAccounts                           *int       `json:"numAccounts"`
			LedgerMerkleRoot                      *string    `json:"ledgerMerkleRoot"`
			ChainID                               string     `json:"chainId"`
			CatchupStatus                         []string   `json:"catchupStatus"`
			BlockProductionKeys                   []string   `json:"blockProductionKeys"`
			CoinbaseReceiver                      *string    `json:"coinbaseReceiver"`
			Peers                                 []peerNode `json:"peers"`
			AddrsAndPorts                         *struct {
				ExternalIP string `json:"externalIp"`
				BindIP     string `json:"bindIp"`
				ClientPort int    `json:"clientPort"`
				Libp2pPort int    `json:"libp2pPort"`
			} `json:"addrsAndPorts"`
		} `json:"daemonStatus"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	ds := result.DaemonStatus
	status := &DaemonStatus{
		SyncStatus:                            ds.SyncStatus,
		BlockchainLength:                      ds.BlockchainLength,
		HighestBlockLengthReceived:            ds.HighestBlockLengthReceived,
		HighestUnvalidatedBlockLengthReceived: ds.HighestUnvalidatedBlockLengthReceived,
		UptimeSecs:                            ds.UptimeSecs,
		StateHash:                             strOrEmpty(ds.StateHash),
		CommitID:                              ds.CommitID,
		NumAccounts:                           ds.NumAccounts,
		LedgerMerkleRoot:                      strOrEmpty(ds.LedgerMerkleRoot),
		ChainID:                               ds.ChainID,
		CatchupStatus:                         ds.CatchupStatus,
		BlockProductionKeys:                   ds.BlockProductionKeys,
		CoinbaseReceiver:                      strOrEmpty(ds.CoinbaseReceiver),
	}
	if ds.Peers != nil {
		status.Peers = toPeers(ds.Peers)
	}
	if a := ds.AddrsAndPorts; a != nil {
		status.AddrsAndPorts = &AddrsAndPorts{
			ExternalIP: a.ExternalIP,
			BindIP:     a.BindIP,
			ClientPort: a.ClientPort,
			Libp2pPort: a.Libp2pPort,
		}
	}
	return status, nil
}

// GetDaemonMetrics returns the daemon's transaction pool, snark pool and
// block production metrics.
func (c *Client) GetDaemonMetrics() (*DaemonMetrics, error) {
	data, err := c.request(queryDaemonMetrics, nil, "get_daemon_metrics")
	if err != nil {
		return nil, err
	}
	var result struct {
		DaemonStatus struct {
			Metrics struct {
				BlockProductionDelay           []int `json:"blockProductionDelay"`
				TransactionPoolDiffReceived    int   `json:"transactionPoolDiffReceived"`
				TransactionPoolDiffBroadcasted int   `json:"transactionPoolDiffBroadcasted"`
				TransactionsAddedToPool        int   `json:"transactionsAddedToPool"`
				TransactionPoolSize            int   `json:"transactionPoolSize"`
				SnarkPoolDiffReceived          int   `json:"snarkPoolDiffReceived"`
				SnarkPoolDiffBroadcasted       int   `json:"snarkPoolDiffBroadcasted"`
				PendingSnarkWork               int   `json:"pendingSnarkWork"`
				SnarkPoolSize                  int   `json:"snarkPoolSize"`
			} `json:"metrics"`
		} `json:"daemonStatus"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	m := result.DaemonStatus.Metrics
	return &DaemonMetrics{
		BlockProductionDelay:           m.BlockProductionDelay,
		TransactionPoolDiffReceived:    m.TransactionPoolDiffReceived,
		TransactionPoolDiffBroadcasted: m.TransactionPoolDiffBroadcasted,
		TransactionsAddedToPool:        m.TransactionsAddedToPool,
		TransactionPoolSize:            m.TransactionPoolSize,
		SnarkPoolDiffReceived:          m.SnarkPoolDiffReceived,
		SnarkPoolDiffBroadcasted:       m.SnarkPoolDiffBroadcasted,
		PendingSnarkWork:               m.PendingSnarkWork,
		SnarkPoolSize:                  m.SnarkPoolSize,
	}, nil
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

// accountNode is the GraphQL shape of an account.
type accountNode struct {
	PublicKey        string  `json:"publicKey"`
	Nonce            scalar  `json:"nonce"`
	Delegate         *string `json:"delegate"`
	TokenID          string  `json:"tokenId"`
	TokenSymbol      *string `json:"tokenSymbol"`
	VotingFor        *string `json:"votingFor"`
	ReceiptChainHash *string `json:"receiptChainHash"`
	Balance          struct {
		Total       scalar `json:"total"`
		Liquid      scalar `json:"liquid"`
		Locked      scalar `json:"locked"`
		BlockHeight scalar `json:"blockHeight"`
	} `json:"balance"`
	Timing *struct {
		InitialMinimumBalance scalar `json:"initialMinimumBalance"`
		CliffTime             scalar `json:"cliffTime"`
		CliffAmount           scalar `json:"cliffAmount"`
		VestingPeriod         scalar `json:"vestingPeriod"`
		VestingIncrement      scalar `json:"vestingIncrement"`
	} `json:"timing"`
	Permissions *struct {
		EditState          string `json:"editState"`
		Send               string `json:"send"`
		Receive            string `json:"receive"`
		Access             string `json:"access"`
		SetDelegate        string `json:"setDelegate"`
		SetPermissions     string `json:"setPermissions"`
		SetVerificationKey struct {
			Auth       string `json:"auth"`
			TxnVersion scalar `json:"txnVersion"`
		} `json:"setVerificationKey"`
		SetZkappURI     string `json:"setZkappUri"`
		EditActionState string `json:"editActionState"`
		SetTokenSymbol  string `json:"setTokenSymbol"`
		IncrementNonce  string `json:"incrementNonce"`
		SetVotingFor    string `json:"setVotingFor"`
		SetTiming       string `json:"setTiming"`
	} `json:"permissions"`
	ZkappState  []string `json:"zkappState"`
	ProvedState *bool    `json:"provedState"`
	ZkappURI    *string  `json:"zkappUri"`
}

func (acc accountNode) toAccountData() (*AccountData, error) {
	total, err := CurrencyFromGraphQL(acc.Balance.Total.String())
	if err != nil {
		return nil, fmt.Errorf("parse total balance: %w", err)
	}
	balance := AccountBalance{Total: total, BlockHeight: acc.Balance.BlockHeight.OptInt()}
	if balance.Liquid, err = acc.Balance.Liquid.OptCurrency("liquid balance"); err != nil {
		return nil, err
	}
	if balance.Locked, err = acc.Balance.Locked.OptCurrency("locked balance"); err != nil {
		return nil, err
	}

	account := &AccountData{
		PublicKey:        acc.PublicKey,
		Nonce:            acc.Nonce.Int(),
		Balance:          balance,
		Delegate:         strOrEmpty(acc.Delegate),
		TokenID:          acc.TokenID,
		TokenSymbol:      strOrEmpty(acc.TokenSymbol),
		VotingFor:        strOrEmpty(acc.VotingFor),
		ReceiptChainHash: strOrEmpty(acc.ReceiptChainHash),
		ZkappState:       acc.ZkappState,
		ProvedState:      acc.ProvedState,
		ZkappURI:         strOrEmpty(acc.ZkappURI),
	}
	if t := acc.Timing; t != nil {
		timing := &AccountTiming{CliffTime: t.CliffTime.OptInt(), VestingPeriod: t.VestingPeriod.OptInt()}
		if timing.InitialMinimumBalance, err = t.InitialMinimumBalance.OptCurrency("timing.initialMinimumBalance"); err != nil {
			return nil, err
		}
		if timing.CliffAmount, err = t.CliffAmount.OptCurrency("timing.cliffAmount"); err != nil {
			return nil, err
		}
		if timing.VestingIncrement, err = t.VestingIncrement.OptCurrency("timing.vestingIncrement"); err != nil {
			return nil, err
		}
		// The daemon returns a timing object with every field null for an
		// untimed account.
		if *timing != (AccountTiming{}) {
			account.Timing = timing
		}
	}
	if p := acc.Permissions; p != nil {
		perms := &AccountPermissions{
			EditState:       p.EditState,
			Send:            p.Send,
			Receive:         p.Receive,
			Access:          p.Access,
			SetDelegate:     p.SetDelegate,
			SetPermissions:  p.SetPermissions,
			SetZkappURI:     p.SetZkappURI,
			EditActionState: p.EditActionState,
			SetTokenSymbol:  p.SetTokenSymbol,
			IncrementNonce:  p.IncrementNonce,
			SetVotingFor:    p.SetVotingFor,
			SetTiming:       p.SetTiming,
		}
		perms.SetVerification.Auth = p.SetVerificationKey.Auth
		perms.SetVerification.TxnVersion = p.SetVerificationKey.TxnVersion.String()
		account.Permissions = perms
	}
	return account, nil
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
		Account *accountNode `json:"account"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.Account == nil {
		return nil, &AccountNotFoundError{PublicKey: publicKey}
	}
	return result.Account.toAccountData()
}

// epochData is the GraphQL shape of staking/next epoch data on a block.
type epochData struct {
	EpochLength scalar `json:"epochLength"`
	Seed        string `json:"seed"`
	Ledger      struct {
		Hash string `json:"hash"`
	} `json:"ledger"`
}

// blockNode is the GraphQL shape of a single block, shared by GetBestChain,
// GetGenesisBlock and GetBlock. Mina returns the numeric consensus fields as
// strings.
type blockNode struct {
	StateHash               string      `json:"stateHash"`
	CommandTransactionCount int         `json:"commandTransactionCount"`
	CreatorAccount          publicKeyOf `json:"creatorAccount"`
	Transactions            struct {
		Coinbase                string       `json:"coinbase"`
		CoinbaseReceiverAccount *publicKeyOf `json:"coinbaseReceiverAccount"`
		FeeTransfer             []struct {
			Recipient string `json:"recipient"`
			Fee       scalar `json:"fee"`
			Type      string `json:"type"`
		} `json:"feeTransfer"`
		UserCommands []struct {
			commandNode
			FailureReason *string `json:"failureReason"`
		} `json:"userCommands"`
	} `json:"transactions"`
	ProtocolState struct {
		PreviousStateHash string `json:"previousStateHash"`
		ConsensusState    struct {
			BlockHeight       string    `json:"blockHeight"`
			SlotSinceGenesis  string    `json:"slotSinceGenesis"`
			Slot              string    `json:"slot"`
			Epoch             string    `json:"epoch"`
			BlockCreator      string    `json:"blockCreator"`
			CoinbaseReceiever string    `json:"coinbaseReceiever"`
			StakingEpochData  epochData `json:"stakingEpochData"`
			NextEpochData     epochData `json:"nextEpochData"`
		} `json:"consensusState"`
		BlockchainState struct {
			Date              scalar `json:"date"`
			UTCDate           scalar `json:"utcDate"`
			StagedLedgerHash  string `json:"stagedLedgerHash"`
			SnarkedLedgerHash string `json:"snarkedLedgerHash"`
		} `json:"blockchainState"`
	} `json:"protocolState"`
}

func (b blockNode) toBlockInfo() (BlockInfo, error) {
	cs := b.ProtocolState.ConsensusState
	chain := b.ProtocolState.BlockchainState
	height, _ := strconv.Atoi(cs.BlockHeight)
	slotGenesis, _ := strconv.Atoi(cs.SlotSinceGenesis)
	slotFork, _ := strconv.Atoi(cs.Slot)
	epoch, _ := strconv.Atoi(cs.Epoch)

	// The daemon may omit the creator public key (e.g. for the genesis block);
	// fall back to a sentinel so callers get a stable value.
	creatorPK := b.CreatorAccount.PublicKey
	if creatorPK == "" {
		creatorPK = "unknown"
	}

	info := BlockInfo{
		StateHash:               b.StateHash,
		Height:                  height,
		GlobalSlotSinceHardFork: slotFork,
		GlobalSlotSinceGenesis:  slotGenesis,
		CreatorPK:               creatorPK,
		CommandTransactionCount: b.CommandTransactionCount,
		Epoch:                   epoch,
		StakingEpochLedgerHash:  cs.StakingEpochData.Ledger.Hash,
		StakingEpochSeed:        cs.StakingEpochData.Seed,
		StakingEpochLength:      cs.StakingEpochData.EpochLength.Int(),
		NextEpochLedgerHash:     cs.NextEpochData.Ledger.Hash,
		NextEpochSeed:           cs.NextEpochData.Seed,
		StagedLedgerHash:        chain.StagedLedgerHash,
		SnarkedLedgerHash:       chain.SnarkedLedgerHash,
		PreviousStateHash:       b.ProtocolState.PreviousStateHash,
		BlockCreator:            cs.BlockCreator,
		CoinbaseReceiver:        cs.CoinbaseReceiever,
		Date:                    chain.Date.String(),
		UTCDate:                 chain.UTCDate.String(),
		Coinbase:                b.Transactions.Coinbase,
		FeeTransferCount:        len(b.Transactions.FeeTransfer),
		FeeTransfers:            make([]FeeTransfer, len(b.Transactions.FeeTransfer)),
		UserCommands:            make([]BlockTransaction, len(b.Transactions.UserCommands)),
	}
	if r := b.Transactions.CoinbaseReceiverAccount; r != nil {
		info.CoinbaseReceiverAccount = r.PublicKey
	}
	for i, f := range b.Transactions.FeeTransfer {
		fee, err := f.Fee.Currency("feeTransfer.fee")
		if err != nil {
			return BlockInfo{}, err
		}
		info.FeeTransfers[i] = FeeTransfer{Recipient: f.Recipient, Fee: fee, Type: f.Type}
	}
	for i, u := range b.Transactions.UserCommands {
		amount, err := u.Amount.Currency("userCommands.amount")
		if err != nil {
			return BlockInfo{}, err
		}
		fee, err := u.Fee.Currency("userCommands.fee")
		if err != nil {
			return BlockInfo{}, err
		}
		info.UserCommands[i] = BlockTransaction{
			ID:            u.ID,
			Hash:          u.Hash,
			Kind:          u.Kind,
			Nonce:         u.Nonce.Int(),
			Source:        u.Source.PublicKey,
			Receiver:      u.Receiver.PublicKey,
			Amount:        amount,
			Fee:           fee,
			Memo:          u.Memo,
			FailureReason: u.FailureReason,
		}
	}
	return info, nil
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
		BestChain []blockNode `json:"bestChain"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.BestChain == nil {
		return nil, nil
	}

	blocks := make([]BlockInfo, len(result.BestChain))
	for i, b := range result.BestChain {
		if blocks[i], err = b.toBlockInfo(); err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

// GetGenesisBlock returns the network's genesis block.
func (c *Client) GetGenesisBlock() (*BlockInfo, error) {
	data, err := c.request(queryGenesisBlock, nil, "get_genesis_block")
	if err != nil {
		return nil, err
	}
	var result struct {
		GenesisBlock blockNode `json:"genesisBlock"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	info, err := result.GenesisBlock.toBlockInfo()
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// GetBlock returns one block, by state hash or by height. Exactly one of
// ref.StateHash and ref.Height must be set.
func (c *Client) GetBlock(ref BlockRef) (*BlockInfo, error) {
	vars := map[string]any{"stateHash": nil, "height": nil}
	switch {
	case ref.StateHash != "" && ref.Height == nil:
		vars["stateHash"] = ref.StateHash
	case ref.StateHash == "" && ref.Height != nil:
		vars["height"] = *ref.Height
	default:
		return nil, fmt.Errorf("get_block: set exactly one of StateHash and Height")
	}

	data, err := c.request(queryBlock, vars, "get_block")
	if err != nil {
		return nil, err
	}
	var result struct {
		Block *blockNode `json:"block"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.Block == nil {
		return nil, fmt.Errorf("get_block: no block in the response")
	}
	info, err := result.Block.toBlockInfo()
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// GetForkConfig returns the daemon's fork_config: the full configuration blob
// used to seed a hardfork's genesis ledger. It is returned verbatim as raw JSON
// (the shape is large and version-dependent, so callers parse what they need).
func (c *Client) GetForkConfig() (json.RawMessage, error) {
	data, err := c.request(queryForkConfig, nil, "get_fork_config")
	if err != nil {
		return nil, err
	}
	var result struct {
		ForkConfig json.RawMessage `json:"fork_config"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.ForkConfig, nil
}

// GetPeers returns the list of connected peers.
func (c *Client) GetPeers() ([]PeerInfo, error) {
	data, err := c.request(queryGetPeers, nil, "get_peers")
	if err != nil {
		return nil, err
	}
	var result struct {
		GetPeers []peerNode `json:"getPeers"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return toPeers(result.GetPeers), nil
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
			Source: raw.Source.PublicKey, Receiver: raw.Receiver.PublicKey,
			Memo: raw.Memo, FailureReason: raw.FailureReason,
		}
	}
	return cmds, nil
}

// GetPooledZkappCommands returns pending zkApp commands from the transaction
// pool. Pass an empty publicKey to get the commands of every fee payer.
func (c *Client) GetPooledZkappCommands(publicKey string) ([]ZkappCommandResult, error) {
	vars := map[string]any{"publicKey": nil}
	if publicKey != "" {
		vars["publicKey"] = publicKey
	}

	data, err := c.request(queryPooledZkappCommands, vars, "get_pooled_zkapp_commands")
	if err != nil {
		return nil, err
	}
	var result struct {
		PooledZkappCommands []zkappNode `json:"pooledZkappCommands"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	cmds := make([]ZkappCommandResult, len(result.PooledZkappCommands))
	for i, z := range result.PooledZkappCommands {
		r, err := z.toResult()
		if err != nil {
			return nil, err
		}
		cmds[i] = *r
	}
	return cmds, nil
}

// GetTransactionStatus returns the status of a payment, delegation or zkApp
// command. Exactly one of ref.Payment and ref.Zkapp must be set.
func (c *Client) GetTransactionStatus(ref TransactionRef) (TransactionStatus, error) {
	vars := map[string]any{"payment": nil, "zkappTransaction": nil}
	switch {
	case ref.Payment != "" && ref.Zkapp == "":
		vars["payment"] = ref.Payment
	case ref.Payment == "" && ref.Zkapp != "":
		vars["zkappTransaction"] = ref.Zkapp
	default:
		return "", fmt.Errorf("get_transaction_status: set exactly one of Payment and Zkapp")
	}

	data, err := c.request(queryTransactionStatus, vars, "get_transaction_status")
	if err != nil {
		return "", err
	}
	var result struct {
		TransactionStatus TransactionStatus `json:"transactionStatus"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.TransactionStatus, nil
}

// GetGenesisConstants returns the network's genesis constants.
func (c *Client) GetGenesisConstants() (*GenesisConstants, error) {
	data, err := c.request(queryGenesisConstants, nil, "get_genesis_constants")
	if err != nil {
		return nil, err
	}
	var result struct {
		GenesisConstants struct {
			GenesisTimestamp   string `json:"genesisTimestamp"`
			Coinbase           scalar `json:"coinbase"`
			AccountCreationFee scalar `json:"accountCreationFee"`
		} `json:"genesisConstants"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	g := result.GenesisConstants
	coinbase, err := g.Coinbase.Currency("coinbase")
	if err != nil {
		return nil, err
	}
	fee, err := g.AccountCreationFee.Currency("accountCreationFee")
	if err != nil {
		return nil, err
	}
	return &GenesisConstants{
		GenesisTimestamp:   g.GenesisTimestamp,
		Coinbase:           coinbase,
		AccountCreationFee: fee,
	}, nil
}

// GetTrackedAccounts returns the accounts the daemon tracks (its wallet keys).
func (c *Client) GetTrackedAccounts() ([]TrackedAccount, error) {
	data, err := c.request(queryTrackedAccounts, nil, "get_tracked_accounts")
	if err != nil {
		return nil, err
	}
	var result struct {
		TrackedAccounts []struct {
			PublicKey string `json:"publicKey"`
			Balance   struct {
				Total scalar `json:"total"`
			} `json:"balance"`
		} `json:"trackedAccounts"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	accounts := make([]TrackedAccount, len(result.TrackedAccounts))
	for i, a := range result.TrackedAccounts {
		balance, err := a.Balance.Total.Currency("balance.total")
		if err != nil {
			return nil, err
		}
		accounts[i] = TrackedAccount{PublicKey: a.PublicKey, Balance: balance}
	}
	return accounts, nil
}

// GetSnarkPool returns the completed snark work in the snark pool.
func (c *Client) GetSnarkPool() ([]CompletedWork, error) {
	data, err := c.request(querySnarkPool, nil, "get_snark_pool")
	if err != nil {
		return nil, err
	}
	var result struct {
		SnarkPool []struct {
			Prover  string   `json:"prover"`
			Fee     scalar   `json:"fee"`
			WorkIDs []scalar `json:"workIds"`
		} `json:"snarkPool"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	work := make([]CompletedWork, len(result.SnarkPool))
	for i, w := range result.SnarkPool {
		fee, err := w.Fee.Currency("snarkPool.fee")
		if err != nil {
			return nil, err
		}
		ids := make([]int, len(w.WorkIDs))
		for j, id := range w.WorkIDs {
			ids[j] = id.Int()
		}
		work[i] = CompletedWork{Prover: w.Prover, Fee: fee, WorkIDs: ids}
	}
	return work, nil
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
	// Signature is optional: a signature made outside the daemon. Without it
	// the daemon signs with the sender's key, which must be unlocked.
	Signature *SignatureInput
}

// SendPayment sends a payment transaction.
// Without params.Signature, the sender's account must be unlocked on the node
// (see UnlockAccount).
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

	vars := map[string]any{"input": input, "signature": signatureVar(params.Signature)}
	data, err := c.request(mutationSendPayment, vars, "send_payment")
	if err != nil {
		return nil, err
	}

	var result struct {
		SendPayment struct {
			Payment commandNode `json:"payment"`
		} `json:"sendPayment"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.SendPayment.Payment.toSubmitted()
}

// SendDelegationParams are the parameters for SendDelegation.
type SendDelegationParams struct {
	Sender     string
	DelegateTo string
	Fee        Currency
	Memo       string // optional
	Nonce      *int   // optional explicit nonce
	// Signature is optional: a signature made outside the daemon. Without it
	// the daemon signs with the sender's key, which must be unlocked.
	Signature *SignatureInput
}

// UnlockAccount unlocks an account in the node's keystore with its
// password, so that SendPayment and SendDelegation can send from it. It
// returns the public key of the unlocked account.
func (c *Client) UnlockAccount(publicKey, password string) (string, error) {
	data, err := c.request(mutationUnlockAccount, map[string]any{
		"input": map[string]any{"publicKey": publicKey, "password": password},
	}, "unlock_account")
	if err != nil {
		return "", err
	}
	var result struct {
		UnlockAccount struct {
			PublicKey string `json:"publicKey"`
		} `json:"unlockAccount"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("unlock_account: %w", err)
	}
	return result.UnlockAccount.PublicKey, nil
}

// SendDelegation sends a stake delegation transaction.
// Without params.Signature, the sender's account must be unlocked on the node.
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

	vars := map[string]any{"input": input, "signature": signatureVar(params.Signature)}
	data, err := c.request(mutationSendDelegation, vars, "send_delegation")
	if err != nil {
		return nil, err
	}

	var result struct {
		SendDelegation struct {
			Delegation commandNode `json:"delegation"`
		} `json:"sendDelegation"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.SendDelegation.Delegation.toSubmitted()
}

// SendZkapp sends a signed zkApp command. zkappCommand is the command in the
// daemon's ZkappCommandInput JSON form (for example from o1js toJSON()); a
// json.RawMessage is sent as is.
func (c *Client) SendZkapp(zkappCommand any) (*ZkappCommandResult, error) {
	vars := map[string]any{"input": map[string]any{"zkappCommand": zkappCommand}}
	data, err := c.request(mutationSendZkapp, vars, "send_zkapp")
	if err != nil {
		return nil, err
	}
	var result struct {
		SendZkapp struct {
			Zkapp zkappNode `json:"zkapp"`
		} `json:"sendZkapp"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.SendZkapp.Zkapp.toResult()
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
