package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/shopspring/decimal"

	"github.com/ivanzzeth/polymarket-go-clob-client/types"
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

	fmt.Println("=== Polymarket Order Conversion Example ===\n")

	// Get token ID from environment or use example
	tokenID := os.Getenv("TEST_TOKEN_ID")
	if tokenID == "" {
		fmt.Println("TEST_TOKEN_ID environment variable not set, using example token ID")
		// Example token ID (this is just for demonstration, use a real one)
		tokenID = "26789704146335935253327636432210126325965975622942457950139452145379746946996"
	}

	// Example 1: Convert a limit order (Buy YES @ 0.6)
	fmt.Println("1. Converting Limit Order:")
	fmt.Println("   Original: Buy YES @ 0.6 (100 shares)")

	originalLimitOrder := &types.UserOrder{
		TokenID:    tokenID,
		Price:      decimal.NewFromFloat(0.6),
		Size:       decimal.NewFromInt(100),
		Side:       types.OrderSideBuy,
		FeeRateBps: 10,
		Nonce:      big.NewInt(12345),
	}

	convertedLimitOrder, err := client.ConvertLimitOrder(ctx, originalLimitOrder)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Converted: %s NO @ %s (%s shares)\n",
			convertedLimitOrder.Side,
			convertedLimitOrder.Price.String(),
			convertedLimitOrder.Size.String(),
		)
		fmt.Printf("  Complementary Token ID: %s\n", convertedLimitOrder.TokenID)
		fmt.Println("  Note: Buy YES @ 0.6 = Sell NO @ 0.4 (same economic position)")
	}

	// Example 2: Convert a market order (Sell YES)
	fmt.Println("\n2. Converting Market Order:")
	fmt.Println("   Original: Sell YES (50 shares)")

	originalMarketOrder := &types.UserMarketOrder{
		TokenID: tokenID,
		Price:   func() *decimal.Decimal { p := decimal.NewFromFloat(0.3); return &p }(),
		Amount:  decimal.NewFromInt(50),
		Side:    types.OrderSideSell,
	}

	convertedMarketOrder, err := client.ConvertMarketOrder(ctx, originalMarketOrder)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Converted: %s NO @ %s (%s shares)\n",
			convertedMarketOrder.Side,
			convertedMarketOrder.Price.String(),
			convertedMarketOrder.Amount.String(),
		)
		fmt.Printf("  Complementary Token ID: %s\n", convertedMarketOrder.TokenID)
		fmt.Println("  Note: Sell YES @ 0.3 = Buy NO @ 0.7 (same economic position)")
	}

	// Example 3: Demonstrate conversion is cached
	fmt.Println("\n3. Converting Another Order (using cached complementary token):")
	fmt.Println("   Original: Buy YES @ 0.75 (200 shares)")

	anotherLimitOrder := &types.UserOrder{
		TokenID: tokenID,
		Price:   decimal.NewFromFloat(0.75),
		Size:    decimal.NewFromInt(200),
		Side:    types.OrderSideBuy,
	}

	convertedAnotherOrder, err := client.ConvertLimitOrder(ctx, anotherLimitOrder)
	if err != nil {
		log.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Converted: %s NO @ %s (%s shares)\n",
			convertedAnotherOrder.Side,
			convertedAnotherOrder.Price.String(),
			convertedAnotherOrder.Size.String(),
		)
		fmt.Println("  Note: Complementary token retrieved from cache (no contract call)")
	}

	// Example 4: Use case - finding better liquidity
	fmt.Println("\n4. Use Case - Smart Order Routing:")
	fmt.Println("   Goal: Buy YES @ 0.6")
	fmt.Println("   Strategy:")
	fmt.Println("   - Check YES side orderbook for Buy @ 0.6")
	fmt.Println("   - Convert to: Sell NO @ 0.4")
	fmt.Println("   - Check NO side orderbook for Sell @ 0.4")
	fmt.Println("   - Choose the side with better liquidity/lower slippage")
	fmt.Println("   - Can save 15-30% on trading costs by using optimal side")

	fmt.Println("\n=== Example Complete ===")
}
