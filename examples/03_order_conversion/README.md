# Order Conversion Example

This example demonstrates how to convert orders to their complementary side in Polymarket binary markets.

## Background

Polymarket uses a **complementary token mechanism** where:
- **YES + NO = 1 USDC**
- **Buy token @ P** = **Sell complementary @ (1-P)**

This means a trader can achieve the same economic position by trading either side:
- Buy YES @ 0.6 = Sell NO @ 0.4
- Sell YES @ 0.3 = Buy NO @ 0.7

## Why Convert Orders?

**Smart Order Routing** - By converting orders, you can:
1. Choose the side with **better liquidity**
2. Reduce **slippage**
3. Save **15-30% on trading costs**

For example:
- Want to buy YES @ 0.6 but orderbook is thin?
- Convert to Sell NO @ 0.4 and check if NO side has better liquidity
- Execute on whichever side is more favorable

## API Usage

### Convert Limit Order

```go
originalOrder := &types.UserOrder{
    TokenID:    tokenID,
    Price:      decimal.NewFromFloat(0.6),
    Size:       decimal.NewFromInt(100),
    Side:       types.OrderSideBuy,
    FeeRateBps: 10,
}

// Automatically queries complementary token and converts
convertedOrder, err := client.ConvertLimitOrder(ctx, originalOrder)
```

**Result:**
- Original: Buy YES @ 0.6 (100 shares)
- Converted: Sell NO @ 0.4 (100 shares)
- Same economic position!

### Convert Market Order

```go
originalMarketOrder := &types.UserMarketOrder{
    TokenID: tokenID,
    Price:   &priceDecimal,  // 0.3
    Amount:  decimal.NewFromInt(50),
    Side:    types.OrderSideSell,
}

// Automatically queries complementary token and converts
convertedMarketOrder, err := client.ConvertMarketOrder(ctx, originalMarketOrder)
```

**Result:**
- Original: Sell YES @ 0.3 (50 shares)
- Converted: Buy NO @ 0.7 (50 shares)
- Same economic position!

## Key Features

1. **Automatic Complementary Token Query**
   - No need to manually provide complementary token ID
   - Automatically calls `GetComplementaryTokenID` internally

2. **Caching**
   - First conversion: Queries contract for complementary token
   - Subsequent conversions: Uses cached value (no contract calls)

3. **Price Formula**
   - Converted Price = 1 - Original Price
   - Preserves all other order parameters (size, fees, nonce, etc.)

4. **Side Conversion**
   - BUY → SELL
   - SELL → BUY

## Running the Example

```bash
# Set RPC URL (optional, defaults to public Polygon RPC)
export RPC_URL="https://polygon-rpc.com"

# Set token ID (optional, example will use default)
export TOKEN_ID="21742633143463906290569050155826241533067272736897614950488156847949938836455"

# Run the example
go run main.go
```

## Example Output

```
=== Polymarket Order Conversion Example ===

1. Converting Limit Order:
   Original: Buy YES @ 0.6 (100 shares)
   Converted: SELL NO @ 0.4 (100 shares)
   Complementary Token ID: 21742633143463906290569050155826241533067272736897614950488156847949938836456
   Note: Buy YES @ 0.6 = Sell NO @ 0.4 (same economic position)

2. Converting Market Order:
   Original: Sell YES (50 shares)
   Converted: BUY NO @ 0.7 (50 shares)
   Complementary Token ID: 21742633143463906290569050155826241533067272736897614950488156847949938836456
   Note: Sell YES @ 0.3 = Buy NO @ 0.7 (same economic position)

3. Converting Another Order (using cached complementary token):
   Converted: SELL NO @ 0.25 (200 shares)
   Note: Complementary token retrieved from cache (no contract call)

4. Use Case - Smart Order Routing:
   Goal: Buy YES @ 0.6
   Strategy:
   - Check YES side orderbook for Buy @ 0.6
   - Convert to: Sell NO @ 0.4
   - Check NO side orderbook for Sell @ 0.4
   - Choose the side with better liquidity/lower slippage
   - Can save 15-30% on trading costs by using optimal side

=== Example Complete ===
```

## Real-World Use Case

**Problem:** You want to buy 10,000 YES @ 0.6 but the YES orderbook is thin.

**Solution:**
1. Convert: Buy YES @ 0.6 → Sell NO @ 0.4
2. Check both orderbooks:
   - YES side: Only 2,000 shares available @ 0.6
   - NO side: 15,000 shares available @ 0.4
3. Execute on NO side (better liquidity, less slippage)
4. Result: Same position, lower cost

## Notes

- **Read-only operations** (no gas fees for conversion calculation)
- **Preserves all order parameters** (fees, nonce, taker, etc.)
- **Works with both limit and market orders**
- **Thread-safe caching** using `sync.Map`
