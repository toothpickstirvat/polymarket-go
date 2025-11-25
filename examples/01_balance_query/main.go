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

	fmt.Println("=== Polymarket Balance Query Example ===")
	fmt.Println()

	// Example 1: Get USDC collateral balance
	fmt.Println("1. Querying USDC Collateral Balance:")
	collateralBalance, err := client.GetCollateralBalance(ctx)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Total Balance: %s USDC\n", collateralBalance.String())
	}

	// Example 2: Get detailed USDC balance (including locked in orders)
	fmt.Println("\n2. Querying Detailed USDC Balance:")
	collateralDetail, err := client.GetCollateralBalanceDetail(ctx)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Total Balance:     %s USDC\n", collateralDetail.TotalBalance.String())
		fmt.Printf("  Locked Balance:    %s USDC\n", collateralDetail.LockedBalance.String())
		fmt.Printf("  Available Balance: %s USDC\n", collateralDetail.AvailableBalance.String())
	}

	// Example 3: Get available USDC balance (for placing orders)
	fmt.Println("\n3. Querying Available USDC Balance (for placing orders):")
	availableCollateral, err := client.GetAvailableCollateralBalance(ctx)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Available: %s USDC\n", availableCollateral.String())
	}

	// Example 4: Get position token balance (YES/NO tokens)
	// Note: Use TEST_TOKEN_ID from .env file
	tokenID := os.Getenv("TEST_TOKEN_ID")
	if tokenID != "" {
		fmt.Printf("\n4. Querying Position Token Balance (Token ID: %s):\n", tokenID)
		fmt.Println("   (Position tokens represent YES/NO outcomes in markets)")
		tokenBalance, err := client.GetPositionBalance(ctx, tokenID)
		if err != nil {
			log.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("  Balance: %s tokens\n", tokenBalance.String())
		}

		// Example 5: Get detailed position token balance
		fmt.Println("\n5. Querying Detailed Position Token Balance:")
		tokenDetail, err := client.GetPositionBalanceDetail(ctx, tokenID)
		if err != nil {
			log.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("  Total Balance:     %s tokens\n", tokenDetail.TotalBalance.String())
			fmt.Printf("  Locked Balance:    %s tokens\n", tokenDetail.LockedBalance.String())
			fmt.Printf("  Available Balance: %s tokens\n", tokenDetail.AvailableBalance.String())
		}
	} else {
		fmt.Println("\n4-5. Skipping position token queries (TEST_TOKEN_ID not set in .env)")
	}

	// Example 6: Check if we have enough balance to place an order
	fmt.Println("\n6. Pre-order Balance Check:")
	orderAmount := "10.0" // 10 USDC order
	availableCollateral, err = client.GetAvailableCollateralBalance(ctx)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Want to place order for: %s USDC\n", orderAmount)
		fmt.Printf("  Available balance: %s USDC\n", availableCollateral.String())
		// You would check: availableCollateral >= orderAmount before placing order
	}

	fmt.Println("\n=== Example Complete ===")
}
