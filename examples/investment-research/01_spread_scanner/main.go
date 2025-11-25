package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	polymarket "github.com/ivanzzeth/polymarket-go"
	"github.com/ivanzzeth/polymarket-go/examples/helper"
	polymarketgamma "github.com/ivanzzeth/polymarket-go-gamma-client"
)

type SpreadOpportunity struct {
	Market      *polymarketgamma.Market
	YesBid      decimal.Decimal
	YesAsk      decimal.Decimal
	Spread      decimal.Decimal
	SpreadBps   int     // Basis points (1 bps = 0.01%)
	SpreadPct   float64 // Percentage
	TickSize    decimal.Decimal
	MinSpread   decimal.Decimal // Minimum possible spread (1 tick)
	SpreadTicks int             // How many ticks is the spread
}

func main() {
	// Load .env file
	helper.LoadEnv()

	ctx := context.Background()

	fmt.Println("=== Polymarket Spread Scanner ===")
	fmt.Println()

	// Create client
	client, err := helper.NewClientWithSigner(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Get active markets with liquidity
	// Based on research findings: focus on markets with liquidity > $10K for tradeable opportunities
	fmt.Println("Fetching active markets...")
	now := time.Now()
	endDateMin := polymarketgamma.NormalizedTime(now)
	markets, err := client.GammaClient().GetMarkets(ctx, &polymarketgamma.GetMarketsParams{
		Limit:           100,
		Closed:          polymarket.BoolPtr(false),
		LiquidityNumMin: polymarket.Float64Ptr(10000), // $10K minimum for tradeable liquidity
		EndDateMin:      &endDateMin,                  // Only markets ending in the future
	})
	if err != nil {
		log.Fatalf("Failed to get markets: %v", err)
	}

	fmt.Printf("Found %d active markets\n\n", len(markets))

	// Scan each market for spread opportunities
	opportunities := []SpreadOpportunity{}

	for i, market := range markets {
		fmt.Printf("[%d/%d] Scanning: %s\n", i+1, len(markets), market.Question)
		fmt.Printf("    Liquidity: $%.0f, Volume 24h: $%.0f\n", market.LiquidityNum, market.Volume24hr)

		// Get tick size for this market
		tickSize := getTickSize(market)
		minSpread := tickSize // Minimum spread is 1 tick

		// Parse best bid/ask from market data
		yesBid, yesAsk, err := parsePrices(market)
		if err != nil {
			fmt.Printf("  ⚠️  Error parsing prices: %v\n", err)

			// Fetch order book to check depth issues
			fmt.Printf("  📊 Checking order book depth for condition: %s\n", market.ConditionID)
			if market.ClobTokenIDs != "" {
				// Use ClobTokenIDs (for binary markets, this is typically the YES token)
				tokenID := market.ClobTokenIDs

				orderbook, obErr := client.ClobClient().GetOrderBook(tokenID)
				if obErr != nil {
					fmt.Printf("  ❌ Failed to fetch order book: %v\n", obErr)
					fmt.Printf("  Token ID: %s\n\n", tokenID)
				} else {
					fmt.Printf("  Token ID: %s\n", tokenID)
					fmt.Printf("  Bids: %d levels, Asks: %d levels\n", len(orderbook.Bids), len(orderbook.Asks))
					// Bids are sorted from low to high, so best bid is the LAST element
					if len(orderbook.Bids) > 0 {
						bestBid := orderbook.Bids[len(orderbook.Bids)-1]
						fmt.Printf("  Best Bid: Price=%.4f, Size=%.2f\n", bestBid.Price, bestBid.Size)
					} else {
						fmt.Printf("  Best Bid: NONE (no buy orders)\n")
					}
					// Asks are sorted from high to low, so best ask is the LAST element
					if len(orderbook.Asks) > 0 {
						bestAsk := orderbook.Asks[len(orderbook.Asks)-1]
						fmt.Printf("  Best Ask: Price=%.4f, Size=%.2f\n", bestAsk.Price, bestAsk.Size)
					} else {
						fmt.Printf("  Best Ask: NONE (no sell orders)\n")
					}
					fmt.Printf("\n")
				}
			} else {
				fmt.Printf("  ❌ No ClobTokenIDs available\n\n")
			}
			continue
		}

		// Calculate spread
		spread := yesAsk.Sub(yesBid)

		// Verify mirrored relationship (YES Bid + NO Ask should = 1.0)
		// NO Ask = 1 - YES Bid
		noAsk := decimal.NewFromInt(1).Sub(yesBid)
		noBid := decimal.NewFromInt(1).Sub(yesAsk)

		mirrorCheckSum := yesBid.Add(noAsk)
		expectedOne := decimal.NewFromInt(1)

		// Allow small rounding errors (< 0.0001)
		tolerance := decimal.NewFromFloat(0.0001)
		mirrorValid := mirrorCheckSum.Sub(expectedOne).Abs().LessThan(tolerance)

		if !mirrorValid {
			fmt.Printf("  ⚠️  Mirror check failed: YES Bid (%.4f) + NO Ask (%.4f) = %.4f (expected 1.0)\n",
				yesBid.InexactFloat64(), noAsk.InexactFloat64(), mirrorCheckSum.InexactFloat64())
			continue
		}

		// Calculate spread in basis points and percentage
		spreadBps := 0
		spreadPct := 0.0
		if !yesBid.IsZero() {
			spreadBps = int(spread.Div(yesBid).Mul(decimal.NewFromInt(10000)).IntPart())
			spreadPct = spread.Div(yesBid).Mul(decimal.NewFromInt(100)).InexactFloat64()
		}

		spreadTicks := int(spread.Div(tickSize).IntPart())

		// Only consider if spread > minimum (1 tick)
		if spread.LessThanOrEqual(minSpread) {
			fmt.Printf("  ⏭️  Spread too small: %.4f (<= %.4f min)\n", spread.InexactFloat64(), minSpread.InexactFloat64())
			continue
		}

		// Filter by minimum volume (based on research: active markets are more tradeable)
		minVolume := 1000.0 // $1K minimum 24h volume
		if market.Volume24hr < minVolume {
			fmt.Printf("  ⏭️  Volume too low: $%.0f (< $%.0f min)\n", market.Volume24hr, minVolume)
			continue
		}

		// Found opportunity
		opportunities = append(opportunities, SpreadOpportunity{
			Market:      market,
			YesBid:      yesBid,
			YesAsk:      yesAsk,
			Spread:      spread,
			SpreadBps:   spreadBps,
			SpreadPct:   spreadPct,
			TickSize:    tickSize,
			MinSpread:   minSpread,
			SpreadTicks: spreadTicks,
		})

		fmt.Printf("  ✓ Spread: %.4f (%.2f%%, %d bps, %d ticks)\n",
			spread.InexactFloat64(), spreadPct, spreadBps, spreadTicks)
		fmt.Printf("    YES: Bid %.4f, Ask %.4f\n", yesBid.InexactFloat64(), yesAsk.InexactFloat64())
		fmt.Printf("    NO:  Bid %.4f, Ask %.4f (calculated)\n\n", noBid.InexactFloat64(), noAsk.InexactFloat64())
	}

	// Sort opportunities by spread (largest first)
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].Spread.GreaterThan(opportunities[j].Spread)
	})

	// Display results
	fmt.Println("===========================================")
	fmt.Printf("Found %d markets with spread > 1 tick\n\n", len(opportunities))

	if len(opportunities) == 0 {
		fmt.Println("No spread opportunities found.")
		return
	}

	fmt.Println("Top 20 Spread Opportunities:")
	fmt.Println("-------------------------------------------")

	displayCount := 20
	if len(opportunities) < displayCount {
		displayCount = len(opportunities)
	}

	for i := 0; i < displayCount; i++ {
		opp := opportunities[i]
		fmt.Printf("\n#%d: %s\n", i+1, opp.Market.Question)
		fmt.Printf("    Spread:     %.4f (%.2f%%, %d bps, %d ticks)\n",
			opp.Spread.InexactFloat64(), opp.SpreadPct, opp.SpreadBps, opp.SpreadTicks)
		fmt.Printf("    YES Bid:    %.4f\n", opp.YesBid.InexactFloat64())
		fmt.Printf("    YES Ask:    %.4f\n", opp.YesAsk.InexactFloat64())
		fmt.Printf("    Tick Size:  %.4f\n", opp.TickSize.InexactFloat64())
		fmt.Printf("    Liquidity:  $%.2f\n", opp.Market.LiquidityNum)
		fmt.Printf("    Volume 24h: $%.2f\n", opp.Market.Volume24hr)
	}

	// Statistics
	fmt.Println("\n===========================================")
	fmt.Println("Statistics:")
	fmt.Println("-------------------------------------------")

	if len(opportunities) > 0 {
		totalSpread := decimal.Zero
		totalSpreadBps := 0
		for _, opp := range opportunities {
			totalSpread = totalSpread.Add(opp.Spread)
			totalSpreadBps += opp.SpreadBps
		}

		avgSpread := totalSpread.Div(decimal.NewFromInt(int64(len(opportunities))))
		avgSpreadBps := totalSpreadBps / len(opportunities)

		fmt.Printf("Average Spread: %.4f (%d bps)\n", avgSpread.InexactFloat64(), avgSpreadBps)
		fmt.Printf("Max Spread:     %.4f (%d bps)\n",
			opportunities[0].Spread.InexactFloat64(), opportunities[0].SpreadBps)
		fmt.Printf("Min Spread:     %.4f (%d bps)\n",
			opportunities[len(opportunities)-1].Spread.InexactFloat64(),
			opportunities[len(opportunities)-1].SpreadBps)
	}

	fmt.Println("\n=== Scan Complete ===")
}

// getTickSize extracts tick size from market data
func getTickSize(market *polymarketgamma.Market) decimal.Decimal {
	// Read actual tick size from market data
	if market.OrderPriceMinTickSize > 0 {
		return decimal.NewFromFloat(market.OrderPriceMinTickSize)
	}

	// Fallback to 0.01 if not available
	return decimal.NewFromFloat(0.01)
}

// parsePrices extracts best bid/ask from market data
func parsePrices(market *polymarketgamma.Market) (yesBid, yesAsk decimal.Decimal, err error) {
	// BestBid and BestAsk are float64 in the Market struct
	if market.BestBid == 0 {
		return decimal.Zero, decimal.Zero, fmt.Errorf("BestBid not available")
	}

	if market.BestAsk == 0 {
		return decimal.Zero, decimal.Zero, fmt.Errorf("BestAsk not available")
	}

	yesBid = decimal.NewFromFloat(market.BestBid)
	yesAsk = decimal.NewFromFloat(market.BestAsk)

	return yesBid, yesAsk, nil
}
