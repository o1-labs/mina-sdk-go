# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Package `itn`: `itn.Client` for the daemon's ITN GraphQL server
  (`--itn-graphql-port`), with ed25519 request signing (`itn.Key`), the `auth`
  handshake, sequence numbers and recovery from HTTP 412. Every method takes a
  `context.Context`. It covers every field of `schema_itn`: `auth`,
  `slotsWon`, `internalLogs`, `flushInternalLogs`, `schedulePayments`,
  `scheduleZkappCommands`, `stopScheduledTransactions`, `updateGating`,
  `stopDaemon`, `zkAppCommandLimit`.
- `schema/itn_graphql_schema.json`, an introspection dump of the ITN schema,
  and an offline test of the ITN documents against it.
- `Client.UnlockAccount`.

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
