package itn

import (
	"encoding/json"

	mina "github.com/MinaProtocol/mina-sdk-go"
)

// Auth is the result of the auth query.
type Auth struct {
	// ServerUUID is the UUID of the ITN GraphQL server; it is new after each
	// daemon restart.
	ServerUUID string
	// SignerSequenceNumber is the number the next sequenced request of this
	// key must carry.
	SignerSequenceNumber uint16
	// Libp2pPort is the node's libp2p port.
	Libp2pPort uint16
	// PeerID is the node's libp2p peer ID; empty if the daemon sent none.
	PeerID string
	// IsBlockProducer reports whether the node produces blocks.
	IsBlockProducer bool
}

// LogMetadatum is one (item, value) pair of an internal log; the value is
// arbitrary JSON.
type LogMetadatum struct {
	Item  string          `json:"item"`
	Value json.RawMessage `json:"value"`
}

// Log is one entry of the daemon's internal log (internalLogs).
type Log struct {
	// ID of the log; IDs increase.
	ID int64 `json:"id"`
	// Timestamp of the log.
	Timestamp string `json:"timestamp"`
	// Message of the log.
	Message string `json:"message"`
	// Metadata of the log.
	Metadata []LogMetadatum `json:"metadata"`
	// Process that sent the log if it is not the daemon (prover or
	// verifier); nil for the daemon.
	Process *string `json:"process"`
}

// PaymentsDetails is the input of SchedulePayments.
type PaymentsDetails struct {
	// DurationMin is the length of the scheduler run, in minutes.
	DurationMin int
	// TPS is the frequency of transactions, per second.
	TPS float64
	// MemoPrefix is the memo, up to 32 characters.
	MemoPrefix string
	MaxFee     mina.Currency
	MinFee     mina.Currency
	// Amount of each payment.
	Amount mina.Currency
	// Receiver is the public key of the receiver of the payments.
	Receiver string
	// Senders are the base58 private keys of the accounts to send from.
	Senders []string
}

func (d PaymentsDetails) vars() map[string]any {
	return map[string]any{
		"durationMin": d.DurationMin,
		"tps":         d.TPS,
		"memoPrefix":  d.MemoPrefix,
		"maxFee":      d.MaxFee.NanominaString(),
		"minFee":      d.MinFee.NanominaString(),
		"amount":      d.Amount.NanominaString(),
		"receiver":    d.Receiver,
		"senders":     nonNil(d.Senders),
	}
}

// ZkappCommandsDetails is the input of ScheduleZkappCommands.
type ZkappCommandsDetails struct {
	// MaxAccountUpdates: each generated zkApp transaction has
	// 2*MaxAccountUpdates+2 account updates (including balancing and fee
	// payer). Nil sends null, and the daemon uses its default.
	MaxAccountUpdates *int
	// MaxCost generates max-cost zkApp commands.
	MaxCost bool
	// AccountQueueSize is the size of the queue of recently used accounts.
	AccountQueueSize int
	// DeploymentFee is the fee of the initial deployment of zkApp accounts.
	DeploymentFee mina.Currency
	MaxFee        mina.Currency
	MinFee        mina.Currency
	// InitBalance is the initial balance of the zkApp accounts deployed for
	// the test.
	InitBalance        mina.Currency
	MaxNewZkappBalance mina.Currency
	MinNewZkappBalance mina.Currency
	MaxBalanceChange   mina.Currency
	MinBalanceChange   mina.Currency
	// NoPrecondition disables the precondition in account updates.
	NoPrecondition bool
	MemoPrefix     string
	// DurationMin is the length of the scheduler run, in minutes.
	DurationMin int
	// TPS is the frequency of transactions, per second.
	TPS float64
	// NumNewAccounts is the number of zkApp accounts the scheduler creates
	// during the test.
	NumNewAccounts int
	// NumZkappsToDeploy is the number of zkApp accounts deployed at the start
	// of the test.
	NumZkappsToDeploy int
	// FeePayers are the base58 private keys of the fee payers, which also
	// create the accounts.
	FeePayers []string
	// NonDefaultToken loads a custom (non-default) owned token. Only an
	// unreleased daemon branch has this field; released daemons (for example
	// 4.0.0) ignore it, because ocaml-graphql-server does not check input
	// fields that its schema does not declare. It is sent only when set.
	NonDefaultToken *bool
}

func (d ZkappCommandsDetails) vars() map[string]any {
	v := map[string]any{
		"maxAccountUpdates":  d.MaxAccountUpdates,
		"maxCost":            d.MaxCost,
		"accountQueueSize":   d.AccountQueueSize,
		"deploymentFee":      d.DeploymentFee.NanominaString(),
		"maxFee":             d.MaxFee.NanominaString(),
		"minFee":             d.MinFee.NanominaString(),
		"initBalance":        d.InitBalance.NanominaString(),
		"maxNewZkappBalance": d.MaxNewZkappBalance.NanominaString(),
		"minNewZkappBalance": d.MinNewZkappBalance.NanominaString(),
		"maxBalanceChange":   d.MaxBalanceChange.NanominaString(),
		"minBalanceChange":   d.MinBalanceChange.NanominaString(),
		"noPrecondition":     d.NoPrecondition,
		"memoPrefix":         d.MemoPrefix,
		"durationMin":        d.DurationMin,
		"tps":                d.TPS,
		"numNewAccounts":     d.NumNewAccounts,
		"numZkappsToDeploy":  d.NumZkappsToDeploy,
		"feePayers":          nonNil(d.FeePayers),
	}
	if d.NonDefaultToken != nil {
		v["nonDefaultToken"] = *d.NonDefaultToken
	}
	return v
}

// NetworkPeer is a peer in a GatingUpdate.
type NetworkPeer struct {
	// Host is the IP address of the remote host.
	Host       string `json:"host"`
	Libp2pPort int    `json:"libp2pPort"`
	// PeerID is the base58 peer ID.
	PeerID string `json:"peerId"`
}

// GatingUpdate is the input of UpdateGating.
type GatingUpdate struct {
	// AddedPeers are peers to connect to.
	AddedPeers []NetworkPeer
	// CleanAddedPeers resets the added peers, including the seeds, to an
	// empty list.
	CleanAddedPeers bool
	// Isolate allows connections only from trusted peers.
	Isolate bool
	// BannedPeers are never allowed to connect, unless they are also trusted.
	BannedPeers []NetworkPeer
	// TrustedPeers are always allowed to connect.
	TrustedPeers []NetworkPeer
}

func (g GatingUpdate) vars() map[string]any {
	return map[string]any{
		"addedPeers":      nonNil(g.AddedPeers),
		"cleanAddedPeers": g.CleanAddedPeers,
		"isolate":         g.Isolate,
		"bannedPeers":     nonNil(g.BannedPeers),
		"trustedPeers":    nonNil(g.TrustedPeers),
	}
}

// nonNil turns a nil slice into an empty one, because the daemon's list
// inputs are non-null and a nil slice marshals as null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
