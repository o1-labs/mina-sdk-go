package mina

import "encoding/json"

// The result types of the common API (spec/SPEC.md). They are the union of
// what the Rust, Go and JS SDKs returned; fields this SDK had before keep
// their names and types.

// SyncStatus is a Mina node's synchronization status, as reported by the
// daemon's syncStatus / daemonStatus.syncStatus fields.
type SyncStatus string

// Known sync statuses returned by the Mina daemon.
const (
	SyncStatusConnecting SyncStatus = "CONNECTING"
	SyncStatusListening  SyncStatus = "LISTENING"
	SyncStatusOffline    SyncStatus = "OFFLINE"
	SyncStatusBootstrap  SyncStatus = "BOOTSTRAP"
	SyncStatusSynced     SyncStatus = "SYNCED"
	SyncStatusCatchup    SyncStatus = "CATCHUP"
)

// AccountBalance represents the balance of a Mina account.
type AccountBalance struct {
	Total  Currency
	Liquid *Currency
	Locked *Currency
	// BlockHeight is the height of the block the balance was read at.
	BlockHeight *int
}

// AccountTiming is the vesting schedule of a timed account.
type AccountTiming struct {
	InitialMinimumBalance *Currency
	CliffTime             *int
	CliffAmount           *Currency
	VestingPeriod         *int
	VestingIncrement      *Currency
}

// AccountPermissions are an account's permissions, as the daemon names the
// authorization levels (for example "Signature", "Proof", "None").
type AccountPermissions struct {
	EditState       string
	Send            string
	Receive         string
	Access          string
	SetDelegate     string
	SetPermissions  string
	SetVerification struct {
		Auth       string
		TxnVersion string
	}
	SetZkappURI     string
	EditActionState string
	SetTokenSymbol  string
	IncrementNonce  string
	SetVotingFor    string
	SetTiming       string
}

// AccountData represents a Mina account.
type AccountData struct {
	PublicKey        string
	Nonce            int
	Balance          AccountBalance
	Delegate         string
	TokenID          string
	TokenSymbol      string
	VotingFor        string
	ReceiptChainHash string
	// Timing is nil on an account without a vesting schedule.
	Timing      *AccountTiming
	Permissions *AccountPermissions
	// ZkappState is nil on an account that is not a zkApp.
	ZkappState  []string
	ProvedState *bool
	ZkappURI    string
}

// PeerInfo represents a connected peer.
type PeerInfo struct {
	PeerID string
	Host   string
	Port   int
}

// AddrsAndPorts are the daemon's network addresses and ports.
type AddrsAndPorts struct {
	ExternalIP string
	BindIP     string
	ClientPort int
	Libp2pPort int
}

// DaemonStatus represents the status of the Mina daemon.
type DaemonStatus struct {
	SyncStatus                            SyncStatus
	BlockchainLength                      *int
	HighestBlockLengthReceived            *int
	HighestUnvalidatedBlockLengthReceived *int
	UptimeSecs                            *int
	Peers                                 []PeerInfo
	CommitID                              string
	StateHash                             string
	NumAccounts                           *int
	LedgerMerkleRoot                      string
	ChainID                               string
	CatchupStatus                         []string
	BlockProductionKeys                   []string
	CoinbaseReceiver                      string
	AddrsAndPorts                         *AddrsAndPorts
}

// DaemonMetrics are the daemon's metrics (daemonStatus.metrics).
type DaemonMetrics struct {
	BlockProductionDelay           []int
	TransactionPoolDiffReceived    int
	TransactionPoolDiffBroadcasted int
	TransactionsAddedToPool        int
	TransactionPoolSize            int
	SnarkPoolDiffReceived          int
	SnarkPoolDiffBroadcasted       int
	PendingSnarkWork               int
	SnarkPoolSize                  int
}

// BlockInfo represents a block (from GetBestChain, GetGenesisBlock or
// GetBlock).
type BlockInfo struct {
	StateHash               string
	Height                  int
	GlobalSlotSinceHardFork int
	GlobalSlotSinceGenesis  int
	CreatorPK               string
	CommandTransactionCount int

	// Consensus / protocol-state details (useful for hardfork validation, where
	// epoch-ledger and ledger-hash continuity across the fork must be checked).
	Epoch                  int
	StakingEpochLedgerHash string
	StakingEpochSeed       string
	StakingEpochLength     int
	NextEpochLedgerHash    string
	NextEpochSeed          string
	StagedLedgerHash       string
	SnarkedLedgerHash      string
	PreviousStateHash      string
	BlockCreator           string
	// CoinbaseReceiver is the consensus state's coinbaseReceiever.
	CoinbaseReceiver string
	Date             string
	UTCDate          string

	// Transaction summary for the block.
	Coinbase                string
	FeeTransferCount        int
	CoinbaseReceiverAccount string
	FeeTransfers            []FeeTransfer
	UserCommands            []BlockTransaction
}

// FeeTransfer is a fee transfer in a block.
type FeeTransfer struct {
	Recipient string
	Fee       Currency
	Type      string
}

// BlockTransaction is a user command in a block.
type BlockTransaction struct {
	ID            string
	Hash          string
	Kind          string
	Nonce         int
	Source        string
	Receiver      string
	Amount        Currency
	Fee           Currency
	Memo          string
	FailureReason *string
}

// SubmittedCommand is a command the daemon accepted into its pool.
type SubmittedCommand struct {
	ID       string
	Hash     string
	Nonce    int
	Kind     string
	Source   string
	Receiver string
	Amount   *Currency
	Fee      *Currency
	Memo     string
}

// SendPaymentResult is the result of a send_payment mutation.
type SendPaymentResult = SubmittedCommand

// SendDelegationResult is the result of a send_delegation mutation.
type SendDelegationResult = SubmittedCommand

// SignatureInput is a signature made outside the daemon, for example with
// mina-signer.
type SignatureInput struct {
	Field  string `json:"field"`
	Scalar string `json:"scalar"`
}

// PooledUserCommand represents a pending transaction in the mempool.
type PooledUserCommand struct {
	ID            string  `json:"id"`
	Hash          string  `json:"hash"`
	Kind          string  `json:"kind"`
	Nonce         string  `json:"nonce"`
	Amount        string  `json:"amount"`
	Fee           string  `json:"fee"`
	From          string  `json:"from"`
	To            string  `json:"to"`
	Source        string  `json:"-"`
	Receiver      string  `json:"-"`
	Memo          string  `json:"memo"`
	FailureReason *string `json:"failureReason"`
}

// pooledUserCommandRaw is used internally for JSON deserialization
// since the daemon may return nonce as a number.
type pooledUserCommandRaw struct {
	ID            string      `json:"id"`
	Hash          string      `json:"hash"`
	Kind          string      `json:"kind"`
	Nonce         json.Number `json:"nonce"`
	Amount        string      `json:"amount"`
	Fee           string      `json:"fee"`
	From          string      `json:"from"`
	To            string      `json:"to"`
	Source        publicKeyOf `json:"source"`
	Receiver      publicKeyOf `json:"receiver"`
	Memo          string      `json:"memo"`
	FailureReason *string     `json:"failureReason"`
}

// publicKeyOf is the GraphQL shape { publicKey } of an account reference.
type publicKeyOf struct {
	PublicKey string `json:"publicKey"`
}

// ZkappCommandResult is a zkApp command in the pool or just sent.
type ZkappCommandResult struct {
	ID       string
	Hash     string
	Memo     string
	FeePayer ZkappFeePayer
	// FailureReason has one entry for each failing account update; nil if
	// the command did not fail.
	FailureReason []ZkappFailure
}

// ZkappFeePayer is the fee payer of a zkApp command.
type ZkappFeePayer struct {
	PublicKey  string
	Fee        Currency
	Nonce      int
	ValidUntil *int
}

// ZkappFailure says why an account update of a zkApp command failed.
type ZkappFailure struct {
	Index    *int
	Failures []string
}

// CompletedWork is completed snark work in the snark pool.
type CompletedWork struct {
	Prover  string
	Fee     Currency
	WorkIDs []int
}

// TransactionStatus is the status of a transaction.
type TransactionStatus string

// Transaction statuses returned by the daemon.
const (
	TransactionStatusPending  TransactionStatus = "PENDING"
	TransactionStatusIncluded TransactionStatus = "INCLUDED"
	TransactionStatusUnknown  TransactionStatus = "UNKNOWN"
)

// GenesisConstants are the network's genesis constants.
type GenesisConstants struct {
	GenesisTimestamp   string
	Coinbase           Currency
	AccountCreationFee Currency
}

// TrackedAccount is an account the daemon tracks (one of its wallet keys).
type TrackedAccount struct {
	PublicKey string
	Balance   Currency
}

// BlockRef selects the block for GetBlock: exactly one of StateHash and
// Height.
type BlockRef struct {
	StateHash string
	Height    *int
}

// TransactionRef selects the transaction for GetTransactionStatus: exactly
// one of Payment (a payment or delegation ID) and Zkapp (a zkApp command ID).
type TransactionRef struct {
	Payment string
	Zkapp   string
}
