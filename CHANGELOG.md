# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- ITN methods for harness support, which need a daemon with
  MinaProtocol/mina#19616: `CommitID`, `ScheduledTransactions`,
  `SchedulePaymentsWithHandle`, `ScheduleZkappCommandsWithHandle` and
  `CreateAccounts` (`CreateAccountsDetails`, `CreatedAccounts`). Live tests
  run with `MINA_ITN_HARNESS=1`, and `CreateAccounts` also needs
  `MINA_ITN_FEE_PAYER`.
- `spec/` is mina-sdk-spec v0.2.0.

### Added
- The common API of the Mina SDKs, from
  [mina-sdk-spec](https://github.com/o1-labs/mina-sdk-spec) v0.1.2: `spec/`
  is a copy at the tag in `spec/VERSION`. `spec_test.go` and
  `spec_itn_test.go` check that the query strings (daemon and ITN) are the
  specification's documents, and a CI job checks that `spec/` is the tag's
  copy. mina-sdk-spec's CI validates the documents against the daemon's
  schemas.
- Methods of the common API that this SDK did not have: `GetDaemonMetrics`,
  `GetBlock` (`BlockRef`), `GetPooledZkappCommands`, `GetTransactionStatus`
  (`TransactionRef`), `GetGenesisConstants`, `GetTrackedAccounts`,
  `GetSnarkPool` and `SendZkapp`.
- Signatures made outside the daemon: `SendPaymentParams.Signature` and
  `SendDelegationParams.Signature` (`SignatureInput`).
- Result fields of the common API (the union of what the three SDKs
  returned): more `DaemonStatus` fields and `AddrsAndPorts`; `AccountData`
  token symbol, voting-for, receipt chain hash, timing, permissions and zkApp
  state; `AccountBalance.BlockHeight`; `BlockInfo` previous state hash, block
  creator, coinbase receiver, dates, staking epoch length, fee transfers and
  user commands; `PooledUserCommand` source, receiver, memo and failure
  reason; kind, source, receiver, amount, fee and memo of a sent command.
- Package `itn`: `itn.Client` for the daemon's ITN GraphQL server
  (`--itn-graphql-port`), with ed25519 request signing (`itn.Key`), the `auth`
  handshake, sequence numbers and recovery from HTTP 412. Every method takes a
  `context.Context`. It covers every field of `schema_itn`: `auth`,
  `slotsWon`, `internalLogs`, `flushInternalLogs`, `schedulePayments`,
  `scheduleZkappCommands`, `stopScheduledTransactions`, `updateGating`,
  `stopDaemon`, `zkAppCommandLimit`.
- `Client.UnlockAccount`.

### Changed
- Every query, including the ITN queries, is a named operation of the
  specification. `GetAccount` uses
  one document with an optional `$token`.
- `SendPaymentResult` and `SendDelegationResult` are aliases of the new
  `SubmittedCommand`. The `signature` variable is always sent, as null when
  `Signature` is nil.

### Removed
- The schema drift check (`scripts/check_schema_drift.go`, the Schema
  Drift Check workflow and `schema/graphql_schema.json`). The documents of
  this SDK are the documents of mina-sdk-spec, whose weekly drift job
  validates them against the lightnet daemons of `master`, `compatible` and
  `develop`.

## [0.1.0] - 2026-04-14

### Added
- `Client` with functional options (`WithGraphQLURI`, `WithRetries`, `WithRetryDelay`, `WithTimeout`)
- Query methods: `GetSyncStatus`, `GetDaemonStatus`, `GetNetworkID`, `GetAccount`, `GetBestChain`, `GetPeers`, `GetPooledUserCommands`
- Mutation methods: `SendPayment`, `SendDelegation`, `SetSnarkWorker`, `SetSnarkWorkFee`
- `Currency` type with nanomina precision arithmetic (Add, Sub, Mul, comparisons)
- Typed error types: `GraphQLError`, `ConnectionError`, `AccountNotFoundError`, `CurrencyUnderflowError`
- Response types: `DaemonStatus`, `AccountData`, `BlockInfo`, `PeerInfo`, `PooledUserCommand`
- Unit tests with HTTP mocking (28 tests)
- Integration tests against live daemon (13 tests, skipped without MINA_GRAPHQL_URI)
- CI workflows: test (Go 1.21-1.23), integration, release, schema drift
- Schema drift detection script
- Apache 2.0 license
