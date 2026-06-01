package main

import (
	"context"
	"fmt"
	"log"
	"strings"

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

	fmt.Println("=== Polymarket Volume Generation Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates how to create matching orders on the same side")
	fmt.Println("by combining opposite side and complementary conversions.")
	fmt.Println()

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Example 1: Creating Two Matching BUY Orders")
	fmt.Println(strings.Repeat("=", 60))

	originalOrder := &clobtypes.UserOrder{
		TokenID: "example-token-id",
		Price:   decimal.NewFromFloat(0.49),
		Size:    decimal.NewFromInt(100),
		Side:    clobtypes.OrderSideBuy,
	}

	fmt.Printf("\n1️⃣  Original Order: %s YES @ %s (%s shares)\n",
		originalOrder.Side,
		originalOrder.Price.String(),
		originalOrder.Size.String(),
	)

	oppositeOrder, err := client.ConvertLimitOrderToOppositeSide(originalOrder, decimal.Zero)
	if err != nil {
		log.Fatalf("Failed to convert to opposite side: %v", err)
	}

	fmt.Printf("\n2️⃣  After Opposite Side Conversion: %s YES @ %s (%s shares)\n",
		oppositeOrder.Side,
		oppositeOrder.Price.String(),
		oppositeOrder.Size.String(),
	)
	fmt.Println("   ⮑ Only changed BUY → SELL, same token and price")

	complementaryOrder, err := client.ConvertLimitOrderToComplementary(ctx, oppositeOrder)
	if err != nil {
		log.Fatalf("Failed to convert to complementary: %v", err)
	}

	fmt.Printf("\n3️⃣  After Complementary Conversion: %s NO @ %s (%s shares)\n",
		complementaryOrder.Side,
		complementaryOrder.Price.String(),
		complementaryOrder.Size.String(),
	)
	fmt.Printf("   Token ID: %s\n", complementaryOrder.TokenID)
	fmt.Println("   ⮑ Changed SELL YES → BUY NO, price 0.49 → 0.51")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 Result: Two BUY Orders That Can Match Each Other")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Order A: %s YES @ %s\n", originalOrder.Side, originalOrder.Price.String())
	fmt.Printf("Order B: %s NO @ %s\n", complementaryOrder.Side, complementaryOrder.Price.String())
	fmt.Println("\n✅ Prices sum to 1.0 (0.49 + 0.51 = 1.0)")
	fmt.Println("   • This triggers the CTF split operation")

	fmt.Println("\n\n" + strings.Repeat("=", 60))
	fmt.Println("Example 2: Market Making with Spread (Guaranteed Profit)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\nUsing ConvertLimitOrderToMatchingSameSide convenience method!")

	marketMakingOrder := &clobtypes.UserOrder{
		TokenID: "example-token-id",
		Price:   decimal.NewFromFloat(0.48),
		Size:    decimal.NewFromInt(100),
		Side:    clobtypes.OrderSideBuy,
	}

	spread := decimal.NewFromFloat(0.02)

	fmt.Printf("\n1️⃣  Original Order: %s YES @ %s (%s shares)\n",
		marketMakingOrder.Side,
		marketMakingOrder.Price.String(),
		marketMakingOrder.Size.String(),
	)
	fmt.Printf("   Spread: %s\n", spread.String())

	matchingOrder, err := client.ConvertLimitOrderToMatchingSameSide(ctx, marketMakingOrder, spread)
	if err != nil {
		log.Fatalf("Failed to convert to matching same-side order: %v", err)
	}

	fmt.Printf("\n2️⃣  Matching Order (ONE call!): %s NO @ %s (%s shares)\n",
		matchingOrder.Side,
		matchingOrder.Price.String(),
		matchingOrder.Size.String(),
	)
	fmt.Printf("   Token ID: %s\n", matchingOrder.TokenID)
	fmt.Println("   ⮑ Automatically converted: BUY YES @ 0.48 → BUY NO @ 0.50")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("💰 Result: Two BUY Orders with GUARANTEED PROFIT")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Order A: %s YES @ %s\n", marketMakingOrder.Side, marketMakingOrder.Price.String())
	fmt.Printf("Order B: %s NO @ %s\n", matchingOrder.Side, matchingOrder.Price.String())

	costA := marketMakingOrder.Price
	costB := matchingOrder.Price
	totalCost := costA.Add(costB)
	valueWhenMerged := decimal.NewFromInt(1)
	profit := valueWhenMerged.Sub(totalCost)

	fmt.Println("\n💡 Profit Calculation:")
	fmt.Printf("   • Cost A (Buy YES): %s USDC\n", costA.String())
	fmt.Printf("   • Cost B (Buy NO):  %s USDC\n", costB.String())
	fmt.Printf("   • Total Cost:       %s USDC\n", totalCost.String())
	fmt.Printf("   • Value after merge: %s USDC\n", valueWhenMerged.String())
	fmt.Printf("   • Guaranteed Profit: %s USDC ✅\n", profit.String())
	fmt.Println("\n✅ Both orders are BUY on complementary tokens")
	fmt.Println("   • When both fill, own YES + NO → merge → 1.0 USDC")
	fmt.Printf("   • Profit equals the spread: %s USDC\n", spread.String())

	fmt.Println("\n=== Example Complete ===")
}
