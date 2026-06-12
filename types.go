package mina

import "encoding/json"

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
}

// AccountData represents a Mina account.
type AccountData struct {
	PublicKey string
	Nonce     int
	Balance   AccountBalance
	Delegate  string
	TokenID   string
}

// PeerInfo represents a connected peer.
type PeerInfo struct {
	PeerID string
	Host   string
	Port   int
}

// DaemonStatus represents the status of the Mina daemon.
type DaemonStatus struct {
	SyncStatus                 SyncStatus
	BlockchainLength           *int
	HighestBlockLengthReceived *int
	UptimeSecs                 *int
	Peers                      []PeerInfo
	CommitID                   string
	StateHash                  string
}

// BlockInfo represents a block in the best chain.
type BlockInfo struct {
	StateHash               string
	Height                  int
	GlobalSlotSinceHardFork int
	GlobalSlotSinceGenesis  int
	CreatorPK               string
	CommandTransactionCount int
}

// SendPaymentResult is the result of a send_payment mutation.
type SendPaymentResult struct {
	ID    string
	Hash  string
	Nonce int
}

// SendDelegationResult is the result of a send_delegation mutation.
type SendDelegationResult struct {
	ID    string
	Hash  string
	Nonce int
}

// PooledUserCommand represents a pending transaction in the mempool.
type PooledUserCommand struct {
	ID     string `json:"id"`
	Hash   string `json:"hash"`
	Kind   string `json:"kind"`
	Nonce  string `json:"nonce"`
	Amount string `json:"amount"`
	Fee    string `json:"fee"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// pooledUserCommandRaw is used internally for JSON deserialization
// since the daemon may return nonce as a number.
type pooledUserCommandRaw struct {
	ID     string      `json:"id"`
	Hash   string      `json:"hash"`
	Kind   string      `json:"kind"`
	Nonce  json.Number `json:"nonce"`
	Amount string      `json:"amount"`
	Fee    string      `json:"fee"`
	From   string      `json:"from"`
	To     string      `json:"to"`
}
