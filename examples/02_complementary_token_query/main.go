package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ivanzzeth/polymarket-go/examples/helper"
)

func main() {
	// Load .env file
	helper.LoadEnv()

	ctx := context.Background()

	// Create Polymarket client with signer
	client, err := helper.NewClientWithSigner(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("=== Polymarket Complementary Token Query Example ===\n")

	// Example: Get complementary token ID for a given token
	// Note: Replace with actual token ID from your market
	tokenID := os.Getenv("TEST_TOKEN_ID")
	if tokenID == "" {
		fmt.Println("TEST_TOKEN_ID environment variable not set, using example token ID")
		// Example token ID (this is just for demonstration, use a real one)
		tokenID = "26789704146335935253327636432210126325965975622942457950139452145379746946996"
	}

	fmt.Printf("1. Querying Complementary Token for Token ID: %s\n", tokenID)

	// Query complementary token
	complementaryTokenID, err := client.GetComplementaryTokenID(ctx, tokenID)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Original Token ID:      %s\n", tokenID)
		fmt.Printf("  Complementary Token ID: %s\n", complementaryTokenID)
		fmt.Println("\n  Note: If original token is YES, complementary is NO, and vice versa")
	}

	// Example 2: Query complementary token again (will use cache)
	fmt.Println("\n2. Querying Again (from cache):")
	complementaryTokenID2, err := client.GetComplementaryTokenID(ctx, tokenID)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Complementary Token ID: %s (from cache)\n", complementaryTokenID2)
	}

	// Example 3: Query reverse direction (should also use cache)
	if complementaryTokenID != "" {
		fmt.Println("\n3. Querying Reverse Direction (from cache):")
		originalTokenID, err := client.GetComplementaryTokenID(ctx, complementaryTokenID)
		if err != nil {
			log.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("  Input: %s\n", complementaryTokenID)
			fmt.Printf("  Output: %s\n", originalTokenID)
			fmt.Printf("  Matches original: %v\n", originalTokenID == tokenID)
		}
	}

	fmt.Println("\n=== Example Complete ===")
}
