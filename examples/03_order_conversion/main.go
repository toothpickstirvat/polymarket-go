package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/shopspring/decimal"

	clobtypes "github.com/ivanzzeth/polymarket-go-clob-client/v2/types"
	"github.com/ivanzzeth/polymarket-go/examples/helper"
)

func main() {
	helper.LoadEnv()

	ctx := context.Background()

	client, err := helper.NewClientWithSigner(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("=== Polymarket Order Conversion Example ===\n")

	tokenID := os.Getenv("TEST_TOKEN_ID")
	if tokenID == "" {
		fmt.Println("TEST_TOKEN_ID environment variable not set, using example token ID")
		tokenID = "26789704146335935253327636432210126325965975622942457950139452145379746946996"
	}

	fmt.Println("1. Converting Limit Order:")
	fmt.Println("   Original: Buy YES @ 0.6 (100 shares)")

	originalLimitOrder := &clobtypes.UserOrder{
		TokenID: tokenID,
		Price:   decimal.NewFromFloat(0.6),
		Size:    decimal.NewFromInt(100),
		Side:    clobtypes.OrderSideBuy,
	}

	convertedLimitOrder, err := client.ConvertLimitOrderToComplementary(ctx, originalLimitOrder)
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

	fmt.Println("\n2. Converting Market Order:")
	fmt.Println("   Original: Sell YES (50 shares)")

	originalMarketOrder := &clobtypes.UserMarketOrder{
		TokenID: tokenID,
		Price:   decimal.NewFromFloat(0.3),
		Amount:  decimal.NewFromInt(50),
		Side:    clobtypes.OrderSideSell,
	}

	convertedMarketOrder, err := client.ConvertMarketOrderToComplementary(ctx, originalMarketOrder)
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

	fmt.Println("\n3. Converting Another Order (using cached complementary token):")
	fmt.Println("   Original: Buy YES @ 0.75 (200 shares)")

	anotherLimitOrder := &clobtypes.UserOrder{
		TokenID: tokenID,
		Price:   decimal.NewFromFloat(0.75),
		Size:    decimal.NewFromInt(200),
		Side:    clobtypes.OrderSideBuy,
	}

	convertedAnotherOrder, err := client.ConvertLimitOrderToComplementary(ctx, anotherLimitOrder)
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

	fmt.Println("\n4. Use Case - Smart Order Routing:")
	fmt.Println("   Goal: Buy YES @ 0.6")
	fmt.Println("   Strategy:")
	fmt.Println("   - Check YES side orderbook for Buy @ 0.6")
	fmt.Println("   - Convert to: Sell NO @ 0.4")
	fmt.Println("   - Check NO side orderbook for Sell @ 0.4")
	fmt.Println("   - Choose the side with better liquidity/lower slippage")

	fmt.Println("\n=== Example Complete ===")
}
