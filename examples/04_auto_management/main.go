package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shopspring/decimal"

	polymarket "github.com/ivanzzeth/polymarket-go"
	"github.com/ivanzzeth/polymarket-go/examples/helper"
)

func main() {
	// Load .env file
	helper.LoadEnv()

	ctx := context.Background()

	fmt.Println("=== Polymarket Auto Management Example ===\n")

	// Create client with both auto-redeem and auto-merge enabled
	client, err := helper.NewClientWithOptions(
		ctx,
		// Enable auto-redeem: automatically redeem resolved positions
		polymarket.WithAutoRedeem(&polymarket.AutoRedeemConfig{
			PollingInterval: 30 * time.Second, // Check every 30 seconds
			Enabled:         true,
			OnSuccess: func(tokenID string, amount decimal.Decimal) {
				log.Printf("✓ [AUTO-REDEEM] Redeemed %s USDC from token %s", amount.String(), tokenID)
			},
			OnError: func(err error) {
				log.Printf("✗ [AUTO-REDEEM] Error: %v", err)
			},
		}),
		// Enable auto-merge: automatically merge YES + NO positions into USDC
		polymarket.WithAutoMerge(&polymarket.AutoMergeConfig{
			PollingInterval: 60 * time.Second,          // Check every 60 seconds
			Enabled:         true,
			MinMergeAmount:  decimal.NewFromFloat(0.5), // Only merge if >= 0.5 USDC
			OnSuccess: func(conditionID string, amount decimal.Decimal) {
				log.Printf("✓ [AUTO-MERGE] Merged %s USDC from condition %s", amount.String(), conditionID)
			},
			OnError: func(err error) {
				log.Printf("✗ [AUTO-MERGE] Error: %v", err)
			},
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close() // Ensure graceful shutdown

	fmt.Println("✓ Auto management started successfully")
	fmt.Println()
	fmt.Println("Services running:")
	fmt.Println("  - Auto-Redeem: Every 30 seconds")
	fmt.Println("    → Automatically redeems resolved market positions")
	fmt.Println()
	fmt.Println("  - Auto-Merge:  Every 60 seconds (min 0.5 USDC)")
	fmt.Println("    → Automatically merges YES + NO positions into USDC")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop gracefully...")
	fmt.Println()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n\nReceived shutdown signal, stopping services...")

	// Services will be stopped by defer client.Close()
	fmt.Println("✓ All services stopped gracefully")
	fmt.Println("\n=== Example Complete ===")
}
