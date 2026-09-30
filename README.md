# Mina Go SDK

[![CI](https://github.com/MinaProtocol/mina-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/MinaProtocol/mina-sdk-go/actions/workflows/ci.yml)
[![Integration Tests](https://github.com/MinaProtocol/mina-sdk-go/actions/workflows/integration.yml/badge.svg)](https://github.com/MinaProtocol/mina-sdk-go/actions/workflows/integration.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/MinaProtocol/mina-sdk-go.svg)](https://pkg.go.dev/github.com/MinaProtocol/mina-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/MinaProtocol/mina-sdk-go)](https://goreportcard.com/report/github.com/MinaProtocol/mina-sdk-go)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Go SDK for interacting with [Mina Protocol](https://minaprotocol.com) nodes via GraphQL.

## Features

- **Daemon GraphQL client** -- query node status, accounts, blocks; send payments and delegations
- Typed response structs with `Currency` arithmetic
- Automatic retry with configurable backoff
- Functional options for client configuration

## Requirements

- Go 1.21+
- A running [Mina daemon](https://docs.minaprotocol.com/node-operators/getting-started) with GraphQL enabled

## Installation

```bash
go get github.com/MinaProtocol/mina-sdk-go
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    mina "github.com/MinaProtocol/mina-sdk-go"
)

func main() {
    client := mina.NewClient()
    defer client.Close()

    // Check sync status
    status, _ := client.GetSyncStatus()
    fmt.Println(status) // "SYNCED"

    // Query an account
    account, _ := client.GetAccount("B62q...", "")
    fmt.Printf("Balance: %s MINA\n", account.Balance.Total)

    // Send a payment
    result, err := client.SendPayment(mina.SendPaymentParams{
        Sender:   "B62qsender...",
        Receiver: "B62qreceiver...",
        Amount:   mina.MustCurrencyFromString("1.5"),
        Fee:      mina.MustCurrencyFromString("0.01"),
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Tx hash: %s\n", result.Hash)
}
```

## Configuration

```go
client := mina.NewClient(
    mina.WithGraphQLURI("http://127.0.0.1:3085/graphql"), // default
    mina.WithRetries(3),                                   // retry failed requests
    mina.WithRetryDelay(5 * time.Second),                  // delay between retries
    mina.WithTimeout(30 * time.Second),                    // HTTP timeout
)
```

## API Reference

Full API documentation is available on [pkg.go.dev](https://pkg.go.dev/github.com/MinaProtocol/mina-sdk-go).

The Mina SDKs have the same API, defined in
[mina-sdk-spec](https://github.com/o1-labs/mina-sdk-spec). `spec/` is a copy
of it at the tag in `spec/VERSION`. `spec_test.go` and `spec_itn_test.go`
check that this SDK's queries, including the ITN queries, are the
specification's documents, and CI checks that `spec/` is the tag's copy.

### Queries

| Method | Returns | Description |
|--------|---------|-------------|
| `GetSyncStatus()` | `SyncStatus` | Node sync status (SYNCED, BOOTSTRAP, etc.) |
| `GetDaemonStatus()` | `*DaemonStatus` | Daemon status: chain length, peers, addresses, block production keys |
| `GetDaemonMetrics()` | `*DaemonMetrics` | Transaction and snark pool metrics, block production delay |
| `GetNetworkID()` | `string` | Network identifier |
| `GetAccount(publicKey, tokenID)` | `*AccountData` | Balance, nonce, delegate, timing, permissions, zkApp state |
| `GetBestChain(maxLength)` | `[]BlockInfo` | Recent blocks from best chain |
| `GetGenesisBlock()` | `*BlockInfo` | The genesis block |
| `GetBlock(BlockRef)` | `*BlockInfo` | One block, by state hash or height |
| `GetPeers()` | `[]PeerInfo` | Connected peers |
| `GetPooledUserCommands(publicKey)` | `[]PooledUserCommand` | Pending payments and delegations |
| `GetPooledZkappCommands(publicKey)` | `[]ZkappCommandResult` | Pending zkApp commands |
| `GetTransactionStatus(TransactionRef)` | `TransactionStatus` | PENDING, INCLUDED or UNKNOWN |
| `GetGenesisConstants()` | `*GenesisConstants` | Genesis timestamp, coinbase, account creation fee |
| `GetTrackedAccounts()` | `[]TrackedAccount` | Accounts in the daemon's keystore |
| `GetSnarkPool()` | `[]CompletedWork` | Completed snark work |
| `GetForkConfig()` | `json.RawMessage` | The daemon's fork configuration |

### Mutations

| Method | Returns | Description |
|--------|---------|-------------|
| `SendPayment(params)` | `*SendPaymentResult` | Send a payment; `params.Signature` for one made outside the daemon |
| `SendDelegation(params)` | `*SendDelegationResult` | Delegate stake; `params.Signature` likewise |
| `SendZkapp(command)` | `*ZkappCommandResult` | Send a signed zkApp command (JSON) |
| `UnlockAccount(publicKey, password)` | `string` | Unlock a keystore account so the node can send from it |
| `SetSnarkWorker(publicKey)` | `string` | Set/unset SNARK worker |
| `SetSnarkWorkFee(fee)` | `string` | Set SNARK work fee |

### ITN server (package `itn`)

A daemon started with `ITN_FEATURES=1`, `--itn-graphql-port` and `--itn-keys`
serves a second GraphQL API, which load testing tools use. `itn.Client` signs
each request with an ed25519 `itn.Key` whose public half must be in
`--itn-keys`, and handles the daemon's sequence numbers (a new `auth` after a
daemon restart, HTTP 412). Every method takes a `context.Context`.

```go
import "github.com/MinaProtocol/mina-sdk-go/itn"

key, err := itn.KeyFromBase64(seedB64)      // base64 32-byte ed25519 seed
fmt.Println("--itn-keys", key.PublicKeyBase64())

c := itn.NewClient("http://127.0.0.1:3086/graphql", key)
logs, err := c.InternalLogs(ctx, 0)
handle, err := c.SchedulePayments(ctx, itn.PaymentsDetails{ /* ... */ })
_, err = c.StopScheduledTransactions(ctx, handle)
```

| Method | GraphQL |
|--------|---------|
| `Auth(ctx)` | `auth` (server UUID, sequence number, peer ID, block producer) |
| `SlotsWon(ctx)` | `slotsWon` |
| `InternalLogs(ctx, start)` / `FlushInternalLogs(ctx, end)` | `internalLogs` / `flushInternalLogs` |
| `SchedulePayments(ctx, PaymentsDetails)` | `schedulePayments` |
| `ScheduleZkappCommands(ctx, ZkappCommandsDetails)` | `scheduleZkappCommands` |
| `StopScheduledTransactions(ctx, handle)` | `stopScheduledTransactions` |
| `UpdateGating(ctx, GatingUpdate)` | `updateGating` |
| `StopDaemon(ctx, delay, clean)` | `stopDaemon` |
| `SetZkappCommandLimit(ctx, limit)` | `zkAppCommandLimit` |
| `Request(ctx, query, vars, name)` | any document, sequenced and signed |

A sequenced request is never repeated after a transport error, because the
daemon may already have run it. The documents in `itn/queries.go` are those
of `spec/itn-operations.graphql`, which mina-sdk-spec validates against the
daemon's ITN schema. The Rust SDK has the
same client (`mina_sdk::itn`, feature `itn`).

### Currency

```go
a := mina.NewCurrency(10)                       // 10 MINA
b := mina.MustCurrencyFromString("1.5")          // 1.5 MINA
c := mina.CurrencyFromNanomina(1_000_000_000)    // 1 MINA
d, _ := mina.CurrencyFromGraphQL("1500000000")   // from GraphQL response

sum := a.Add(b)             // 11.500000000
fmt.Println(a.Nanomina())   // 10000000000
fmt.Println(a.Greater(b))   // true

diff, err := a.Sub(b)       // 8.500000000 (returns error on underflow)
```

### Error Handling

```go
result, err := client.GetAccount("B62q...", "")
if err != nil {
    var gqlErr *mina.GraphQLError
    var connErr *mina.ConnectionError
    var notFound *mina.AccountNotFoundError

    switch {
    case errors.As(err, &notFound):
        fmt.Printf("Account does not exist: %s\n", notFound.PublicKey)
    case errors.As(err, &gqlErr):
        fmt.Printf("GraphQL error: %s\n", gqlErr)
    case errors.As(err, &connErr):
        fmt.Printf("Connection failed after %d retries\n", connErr.Retries)
    }
}
```

## Development

```bash
git clone https://github.com/MinaProtocol/mina-sdk-go.git
cd mina-sdk-go
go test -v ./...
go vet ./...
```

### Integration tests

Integration tests run against a live Mina node and are skipped by default.
To run them locally with a [lightnet](https://docs.minaprotocol.com/zkapps/writing-a-zkapp/introduction-to-zkapps/testing-zkapps-lightnet) Docker container:

```bash
docker run --rm -d -p 8080:8080 -p 8181:8181 -p 3085:3085 \
  -e NETWORK_TYPE=single-node -e PROOF_LEVEL=none \
  o1labs/mina-local-network:compatible-latest-lightnet

# Wait for the network to sync, then:
MINA_GRAPHQL_URI=http://127.0.0.1:8080/graphql \
MINA_TEST_SENDER_KEY=B62q... \
MINA_TEST_RECEIVER_KEY=B62q... \
go test -v -run Integration ./...
```

The ITN integration tests (package `itn`) need a daemon with
`ITN_FEATURES=1`, `--itn-graphql-port 3086`, `--itn-keys <public key>` and,
for non-empty internal logs, `--internal-tracing`; they are skipped otherwise:

```bash
MINA_ITN_URI=http://127.0.0.1:3086/graphql MINA_ITN_KEY=<base64 seed> \
go test -v -run Integration ./itn/
```

## Troubleshooting

**Connection refused** -- Make sure the Mina daemon is running and the GraphQL endpoint is accessible. The default URI is `http://127.0.0.1:3085/graphql`.

**Account not found** -- The account may not exist on the network. `GetAccount` returns `*AccountNotFoundError` which you can check with `errors.As`.

**Schema drift** -- If queries fail with unexpected GraphQL errors, the daemon version may have changed its schema. Run: `go run scripts/check_schema_drift.go --endpoint http://your-node:3085/graphql`

## License

[Apache License 2.0](LICENSE)
