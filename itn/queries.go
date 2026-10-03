package itn

// GraphQL documents for the daemon's ITN server. They are the documents of
// spec/itn-operations.graphql (a copy of o1-labs/mina-sdk-spec);
// spec_itn_test.go in the root package checks that they stay identical, and
// mina-sdk-spec's CI validates them against the daemon's schema_itn. Use them
// with Client.Request for custom selections.
const (
	// QueryAuth returns the server UUID and the signer's sequence number.
	// It is the only operation that accepts an unsequenced signature.
	QueryAuth = `query Auth {
  auth {
    serverUuid
    signerSequenceNumber
    libp2pPort
    peerId
    isBlockProducer
  }
}`

	// QuerySlotsWon returns the global slots the node's block producer keys
	// won in the current epoch.
	QuerySlotsWon = `query SlotsWon {
  slotsWon
}`

	// QueryInternalLogs returns the internal logs with an ID of at least
	// $startLogId.
	QueryInternalLogs = `query InternalLogs($startLogId: Int!) {
  internalLogs(startLogId: $startLogId) {
    id
    timestamp
    message
    metadata {
      item
      value
    }
    process
  }
}`

	// MutationFlushInternalLogs drops the internal logs up to and including
	// $endLogId.
	MutationFlushInternalLogs = `mutation FlushInternalLogs($endLogId: Int!) {
  flushInternalLogs(endLogId: $endLogId)
}`

	// MutationSchedulePayments starts sending payments and returns a handle
	// for stopScheduledTransactions.
	MutationSchedulePayments = `mutation SchedulePayments($input: PaymentsDetails!) {
  schedulePayments(input: $input)
}`

	// MutationScheduleZkappCommands starts sending zkApp commands and returns
	// a handle for stopScheduledTransactions.
	MutationScheduleZkappCommands = `mutation ScheduleZkappCommands($input: ZkappCommandsDetails!) {
  scheduleZkappCommands(input: $input)
}`

	// MutationStopScheduledTransactions stops the transactions of a handle.
	MutationStopScheduledTransactions = `mutation StopScheduledTransactions($handle: String!) {
  stopScheduledTransactions(handle: $handle)
}`

	// MutationUpdateGating changes the node's connection gating.
	MutationUpdateGating = `mutation UpdateGating($input: GatingUpdate!) {
  updateGating(input: $input)
}`

	// MutationStopDaemon stops the daemon after $delaySeconds, optionally
	// deleting its configuration directory.
	MutationStopDaemon = `mutation StopDaemon($delaySeconds: Int, $cleanConfig: Boolean) {
  stopDaemon(delaySeconds: $delaySeconds, cleanConfig: $cleanConfig)
}`

	// MutationZkappCommandLimit sets the block producer's limit of zkApp
	// commands per block; null removes it.
	MutationZkappCommandLimit = `mutation ZkappCommandLimit($limit: Int) {
  zkAppCommandLimit(limit: $limit)
}`

	// QueryCommitID returns the daemon's git commit. It needs a daemon with
	// MinaProtocol/mina#19616; older daemons answer with a GraphQL error.
	QueryCommitID = `query CommitId {
  auth {
    commitId
  }
}`

	// QueryScheduledTransactions returns the handles of the running payment
	// and zkApp schedulers and account-creation jobs (mina#19616).
	QueryScheduledTransactions = `query ScheduledTransactions {
  scheduledTransactions
}`

	// MutationSchedulePaymentsWithHandle starts sending payments under a
	// handle that the caller chose (mina#19616).
	MutationSchedulePaymentsWithHandle = `mutation SchedulePaymentsWithHandle($input: PaymentsDetails!, $handle: String!) {
  schedulePayments(input: $input, handle: $handle)
}`

	// MutationScheduleZkappCommandsWithHandle starts sending zkApp commands
	// under a handle that the caller chose (mina#19616).
	MutationScheduleZkappCommandsWithHandle = `mutation ScheduleZkappCommandsWithHandle($input: ZkappCommandsDetails!, $handle: String!) {
  scheduleZkappCommands(input: $input, handle: $handle)
}`

	// MutationCreateAccounts creates and funds new accounts in the
	// background under a handle (mina#19616).
	MutationCreateAccounts = `mutation CreateAccounts($input: CreateAccountsDetails!, $handle: String) {
  createAccounts(input: $input, handle: $handle) {
    handle
    accounts {
      publicKey
      privateKey
    }
  }
}`
)
