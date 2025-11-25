package polymarket

// ============================================================================
// KLine Functionality - DISABLED
// ============================================================================
//
// This file contains KLine (candlestick) functionality that is currently DISABLED
// due to Polymarket API limitations:
//
// LIMITATIONS:
// 1. CLOB API (GetTrades) - Only supports querying user's own trades, not market-wide trades
// 2. Data API (GetTrades) - Supports market-wide trades but has NO time range parameters,
//    only returns recent trades (typically last few hours/days)
//
// CURRENT STATUS: DISABLED - Interface not exported
//
// The implementation exists but is not exported (lowercase function name).
// This will be enabled in the future when:
// - Polymarket provides historical trade data API with time range support
// - Or we implement on-chain event scanning for complete historical data
//
// ============================================================================

import (
	"context"
	"fmt"
	"sort"
	"time"

	polymarketdata "github.com/ivanzzeth/polymarket-go-data-client"
	"github.com/shopspring/decimal"
)

// KLine represents a candlestick bar with OHLCV data
// This is compatible with BBGO's types.KLine structure
//
// NOTE: This type is currently not usable as GetKLines is disabled.
// It is kept for future implementation.
type KLine struct {
	StartTime time.Time       `json:"startTime"` // Start time of the interval
	EndTime   time.Time       `json:"endTime"`   // End time of the interval
	Interval  time.Duration   `json:"interval"`  // Time interval duration
	Open      decimal.Decimal `json:"open"`      // Opening price
	High      decimal.Decimal `json:"high"`      // Highest price
	Low       decimal.Decimal `json:"low"`       // Lowest price
	Close     decimal.Decimal `json:"close"`     // Closing price
	Volume    decimal.Decimal `json:"volume"`    // Total volume (in shares)
	NumTrades int             `json:"numTrades"` // Number of trades in this bar
}

// getKLines is DISABLED - not exported (lowercase)
//
// This function exists but is not exported to prevent usage until API limitations are resolved.
// When enabled in the future, it will be renamed to GetKLines (uppercase).
//
// See file header for detailed explanation of limitations.
func (c *Client) getKLines(ctx context.Context, tokenID string, interval time.Duration, startTime, endTime time.Time) ([]KLine, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be positive")
	}
	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be after startTime")
	}

	// Get condition ID from token ID (for Market filter in TradeParams)
	conditionID, err := c.GetConditionIDByTokenID(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get condition ID: %w", err)
	}

	// Fetch all trades using DataClient with pagination (supports up to 10000 per request)
	const pageSize = 1000 // Use 1000 per page for better performance
	var allTrades []polymarketdata.Trade

	for offset := 0; offset < 10000; offset += pageSize {
		params := &polymarketdata.GetTradesParams{
			Market: []string{conditionID},
			Limit:  pageSize,
			Offset: offset,
		}

		trades, err := c.DataClient().GetTrades(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch trades (offset %d): %w", offset, err)
		}

		if len(trades) == 0 {
			break // No more trades
		}

		allTrades = append(allTrades, trades...)

		// If we got fewer trades than page size, we've reached the end
		if len(trades) < pageSize {
			break
		}
	}

	if len(allTrades) == 0 {
		return []KLine{}, nil
	}

	// Filter by tokenID and time range
	var filteredTrades []polymarketdata.Trade
	for _, trade := range allTrades {
		// Filter by asset (tokenID)
		if trade.Asset != tokenID {
			continue
		}

		// Filter by time range
		tradeTime := time.Unix(trade.Timestamp, 0)
		if tradeTime.Before(startTime) || tradeTime.After(endTime) {
			continue
		}

		filteredTrades = append(filteredTrades, trade)
	}

	if len(filteredTrades) == 0 {
		return []KLine{}, nil
	}

	// Sort trades by time (ascending)
	sort.Slice(filteredTrades, func(i, j int) bool {
		return filteredTrades[i].Timestamp < filteredTrades[j].Timestamp
	})

	// Aggregate trades into candlestick bars
	klines := aggregateTradesIntoKLines(filteredTrades, interval, startTime, endTime)

	return klines, nil
}

// aggregateTradesIntoKLines groups trades into time intervals and creates candlestick bars
func aggregateTradesIntoKLines(trades []polymarketdata.Trade, interval time.Duration, startTime, endTime time.Time) []KLine {
	if len(trades) == 0 {
		return []KLine{}
	}

	// Initialize map to store bars by interval start time
	barsMap := make(map[int64]*KLine)

	// Helper function to get interval start time for a given timestamp
	getIntervalStart := func(ts time.Time) time.Time {
		unix := ts.Unix()
		intervalSeconds := int64(interval.Seconds())
		alignedUnix := (unix / intervalSeconds) * intervalSeconds
		return time.Unix(alignedUnix, 0).UTC()
	}

	// Process each trade
	for _, trade := range trades {
		tradeTime := time.Unix(trade.Timestamp, 0).UTC()

		// Skip trades outside our time range
		if tradeTime.Before(startTime) || tradeTime.After(endTime) {
			continue
		}

		intervalStart := getIntervalStart(tradeTime)
		intervalStartUnix := intervalStart.Unix()

		// Get or create bar for this interval
		bar, exists := barsMap[intervalStartUnix]
		if !exists {
			bar = &KLine{
				StartTime: intervalStart,
				EndTime:   intervalStart.Add(interval),
				Interval:  interval,
				Open:      trade.Price,
				High:      trade.Price,
				Low:       trade.Price,
				Close:     trade.Price,
				Volume:    decimal.Zero,
				NumTrades: 0,
			}
			barsMap[intervalStartUnix] = bar
		}

		// Update OHLC
		if bar.NumTrades == 0 {
			// First trade in this bar
			bar.Open = trade.Price
		}
		bar.Close = trade.Price // Always update to latest trade price

		if trade.Price.GreaterThan(bar.High) {
			bar.High = trade.Price
		}
		if trade.Price.LessThan(bar.Low) {
			bar.Low = trade.Price
		}

		// Add volume
		bar.Volume = bar.Volume.Add(trade.Size)
		bar.NumTrades++
	}

	// Convert map to sorted slice
	var klines []KLine
	for _, bar := range barsMap {
		klines = append(klines, *bar)
	}

	// Sort by time (ascending)
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].StartTime.Before(klines[j].StartTime)
	})

	return klines
}
