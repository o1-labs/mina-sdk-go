// Package mina provides a Go SDK for interacting with Mina Protocol daemon nodes.
//
// It wraps the daemon's GraphQL API with typed Go functions, providing:
//   - Node status queries (sync status, daemon status, bootstrap status,
//     transaction pool status, network ID)
//   - Account queries (balance, nonce, delegation)
//   - Blockchain queries (best chain blocks, peers, mempool)
//   - Transaction mutations (payments, delegations, SNARK worker)
//   - Currency type with nanomina-precision arithmetic
//
// Quick start:
//
//	client := mina.NewClient()
//	defer client.Close()
//
//	status, err := client.GetSyncStatus()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Sync status:", status)
package mina
