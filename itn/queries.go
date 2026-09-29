package itn

// GraphQL documents for the daemon's ITN server. They follow
// schema/itn_graphql_schema.json, an introspection dump of
// Mina_graphql.schema_itn taken from a running daemon; schema_test.go
// checks each of them against it. Use them with Client.Request for custom
// selections.
const (
	// QueryAuth returns the server UUID and the signer's sequence number.
	// It is the only operation that accepts an unsequenced signature.
	QueryAuth = `query {
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
	QuerySlotsWon = `query {
  slotsWon
}`

	// QueryInternalLogs returns the internal logs with an ID of at least
	// $startLogId.
	QueryInternalLogs = `query ($startLogId: Int!) {
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
	MutationFlushInternalLogs = `mutation ($endLogId: Int!) {
  flushInternalLogs(endLogId: $endLogId)
}`

	// MutationSchedulePayments starts sending payments and returns a handle
	// for stopScheduledTransactions.
	MutationSchedulePayments = `mutation ($input: PaymentsDetails!) {
  schedulePayments(input: $input)
}`

	// MutationScheduleZkappCommands starts sending zkApp commands and returns
	// a handle for stopScheduledTransactions.
	MutationScheduleZkappCommands = `mutation ($input: ZkappCommandsDetails!) {
  scheduleZkappCommands(input: $input)
}`

	// MutationStopScheduledTransactions stops the transactions of a handle.
	MutationStopScheduledTransactions = `mutation ($handle: String!) {
  stopScheduledTransactions(handle: $handle)
}`

	// MutationUpdateGating changes the node's connection gating.
	MutationUpdateGating = `mutation ($input: GatingUpdate!) {
  updateGating(input: $input)
}`

	// MutationStopDaemon stops the daemon after $delaySeconds, optionally
	// deleting its configuration directory.
	MutationStopDaemon = `mutation ($delaySeconds: Int, $cleanConfig: Boolean) {
  stopDaemon(delaySeconds: $delaySeconds, cleanConfig: $cleanConfig)
}`

	// MutationZkappCommandLimit sets the block producer's limit of zkApp
	// commands per block; null removes it.
	MutationZkappCommandLimit = `mutation ($limit: Int) {
  zkAppCommandLimit(limit: $limit)
}`
)
