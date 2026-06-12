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

### Queries

| Method | Returns | Description |
|--------|---------|-------------|
| `GetSyncStatus()` | `SyncStatus` | Node sync status (SYNCED, BOOTSTRAP, etc.) |
| `GetDaemonStatus()` | `*DaemonStatus` | Comprehensive daemon status |
| `GetNetworkID()` | `string` | Network identifier |
| `GetAccount(publicKey, tokenID)` | `*AccountData` | Account balance, nonce, delegate |
| `GetBestChain(maxLength)` | `[]BlockInfo` | Recent blocks from best chain |
| `GetPeers()` | `[]PeerInfo` | Connected peers |
| `GetPooledUserCommands(publicKey)` | `[]PooledUserCommand` | Pending transactions |

### Mutations

| Method | Returns | Description |
|--------|---------|-------------|
| `SendPayment(params)` | `*SendPaymentResult` | Send a payment |
| `SendDelegation(params)` | `*SendDelegationResult` | Delegate stake |
| `SetSnarkWorker(publicKey)` | `string` | Set/unset SNARK worker |
| `SetSnarkWorkFee(fee)` | `string` | Set SNARK work fee |

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

## Troubleshooting

**Connection refused** -- Make sure the Mina daemon is running and the GraphQL endpoint is accessible. The default URI is `http://127.0.0.1:3085/graphql`.

**Account not found** -- The account may not exist on the network. `GetAccount` returns `*AccountNotFoundError` which you can check with `errors.As`.

**Schema drift** -- If queries fail with unexpected GraphQL errors, the daemon version may have changed its schema. Run: `go run scripts/check_schema_drift.go --endpoint http://your-node:3085/graphql`

## License

[Apache License 2.0](LICENSE)
