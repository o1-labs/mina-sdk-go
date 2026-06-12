//go:build ignore

// Basic usage of the Mina Go SDK.
package main

import (
	"fmt"
	"log"
	"time"

	mina "github.com/MinaProtocol/mina-sdk-go"
)

func main() {
	// Connect to a local Mina daemon (default: http://127.0.0.1:3085/graphql)
	client := mina.NewClient()
	defer client.Close()

	// Check sync status
	syncStatus, err := client.GetSyncStatus()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Sync status: %s\n", syncStatus)

	// Get daemon status with peer info
	status, err := client.GetDaemonStatus()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Blockchain length: %d\n", *status.BlockchainLength)
	fmt.Printf("Peers: %d\n", len(status.Peers))

	// Get network ID
	networkID, err := client.GetNetworkID()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Network: %s\n", networkID)

	// Query an account
	account, err := client.GetAccount("B62qrPN5Y5yq8kGE3FbVKbGTdTAJNdtNtS5vH1tH...", "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Balance: %s MINA\n", account.Balance.Total)
	fmt.Printf("Nonce: %d\n", account.Nonce)

	// Get recent blocks
	blocks, err := client.GetBestChain(5)
	if err != nil {
		log.Fatal(err)
	}
	for _, block := range blocks {
		fmt.Printf("Block %d: %s... (%d txns)\n",
			block.Height, block.StateHash[:20], block.CommandTransactionCount)
	}

	// Send a payment (requires sender account unlocked on node)
	result, err := client.SendPayment(mina.SendPaymentParams{
		Sender:   "B62qsender...",
		Receiver: "B62qreceiver...",
		Amount:   mina.MustCurrencyFromString("1.5"), // 1.5 MINA
		Fee:      mina.MustCurrencyFromString("0.01"),
		Memo:     "hello from SDK",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Payment sent! Hash: %s, Nonce: %d\n", result.Hash, result.Nonce)
}

func connectToRemoteNode() {
	client := mina.NewClient(
		mina.WithGraphQLURI("http://my-mina-node:3085/graphql"),
		mina.WithRetries(5),
		mina.WithRetryDelay(10*time.Second),
		mina.WithTimeout(60*time.Second),
	)
	defer client.Close()

	status, err := client.GetSyncStatus()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Remote node status: %s\n", status)
}
