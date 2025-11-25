# Arbitrage Scanner Design

## Executive Summary

**CRITICAL FINDING**: Traditional Dutch-book arbitrage (YES + NO ≠ 1.0) is **NOT POSSIBLE** on Polymarket due to mirrored order books.

**What This Means**:
- ❌ Premium Arbitrage (YES Bid + NO Bid > 1.0) - **Impossible**
- ❌ Discount Arbitrage (YES Ask + NO Ask < 1.0) - **Impossible**
- ✅ Market Making (spread capture) - **Possible but requires risk**
- ✅ Cross-platform arbitrage - **Possible if you have accounts on multiple platforms**
- ⚠️ NegRisk market arbitrage - **Needs verification**

**Recommendation**: Focus on market making strategies rather than pure arbitrage, or cross-platform opportunities.

## Overview

Based on initial research, $40M+ was reportedly extracted from Polymarket. However, analysis of the mirrored order book mechanism reveals this was likely **market making profits** rather than pure risk-free arbitrage.

## Critical Constraint: Mirrored Order Books

**IMPORTANT**: Polymarket's order books are **mirrored** - YES and NO orders are automatically linked:

```
YES Bid = 1 - NO Ask
YES Ask = 1 - NO Bid
NO Bid = 1 - YES Ask
NO Ask = 1 - YES Bid
```

**Implication**: This mirroring means certain arbitrage types are **impossible**:
- Premium Arbitrage (YES Bid + NO Bid > 1.0) **cannot exist**
  - If YES Bid = 0.54, then NO Ask = 0.46 (matched immediately)
  - If NO Bid = 0.48, then YES Ask = 0.52 (matched immediately)
  - These orders would execute against each other instantly - no arbitrage window

**What CAN exist**:
- Spread-based opportunities (buying at bid, selling at ask)
- Cross-market arbitrage (different markets, same event)
- Temporary mispricing during high volatility
- NegRisk market arbitrage (multi-outcome markets have different dynamics)

## Arbitrage Types

### 1. Spread Capture (Market Making) ⭐ **Realistic Opportunity**

**Principle**: Profit from the bid-ask spread by providing liquidity.

**Market State**:
```
YES: Bid 0.53, Ask 0.55 (spread: 0.02)
NO:  Bid 0.45, Ask 0.47 (spread: 0.02)

Verification of mirrored relationship:
YES Bid (0.53) + NO Ask (0.47) = 1.00 ✓
YES Ask (0.55) + NO Bid (0.45) = 1.00 ✓
```

**Strategy**:
```
1. Place limit orders to provide liquidity:
   - Buy YES @ 0.53 (at the bid)
   - Sell YES @ 0.55 (at the ask)

2. When both fill:
   - Bought at 0.53, sold at 0.55
   - Profit: 0.02 per token (3.7% return)

3. Risk: Directional exposure if only one side fills
```

**This is NOT risk-free arbitrage** - it's market making with inventory risk.

**Implementation**:
```go
type SpreadOpportunity struct {
    ConditionID string
    YesBid      decimal.Decimal
    YesAsk      decimal.Decimal
    Spread      decimal.Decimal
    SpreadBps   int // Basis points
}

func DetectSpreadOpportunity(orderbook *OrderBook, minSpreadBps int) *SpreadOpportunity {
    spread := orderbook.YesAsk.Sub(orderbook.YesBid)
    spreadBps := int(spread.Div(orderbook.YesBid).Mul(decimal.NewFromInt(10000)).IntPart())

    if spreadBps >= minSpreadBps {
        return &SpreadOpportunity{
            ConditionID: orderbook.ConditionID,
            YesBid:      orderbook.YesBid,
            YesAsk:      orderbook.YesAsk,
            Spread:      spread,
            SpreadBps:   spreadBps,
        }
    }
    return nil
}
```

**Reality Check**: This is how market makers profit, not arbitrageurs. It requires:
- Capital to hold inventory
- Tolerance for directional risk
- Frequent rebalancing

---

### 2. NegRisk Market Arbitrage ⚠️ **Theoretically Possible, Needs Verification**

**Principle**: In multi-outcome markets, sum of all outcome prices should equal 1.0.

**IMPORTANT**: NegRisk markets may also have mirroring mechanisms. Need to verify if sum(asks) < 1.0 can actually exist, or if the matching engine prevents this.

**Example Market** (if such mispricing exists): NFL Team Winner
```
Team A Ask: 0.30
Team B Ask: 0.25
Team C Ask: 0.20
Team D Ask: 0.15
Other Ask:  0.08
Total:      0.98 USDC ← Arbitrage opportunity (2% discount) IF this can exist
```

**Strategy** (if applicable):
```
1. Buy all outcomes at market price
2. Total cost: 0.98 USDC
3. Guaranteed redemption: 1.00 USDC (one outcome must win)
4. Profit: 0.02 USDC
```

**Questions to investigate**:
- Do NegRisk markets have automatic balancing?
- Can sum(outcome_asks) actually be < 1.0?
- Are there arbitrage bots already eliminating these opportunities instantly?

**Implementation**:
```go
type NegRiskArbitrage struct {
    ConditionID    string
    Outcomes       []Outcome
    TotalPrice     decimal.Decimal
    ExpectedProfit decimal.Decimal
}

type Outcome struct {
    TokenID string
    Price   decimal.Decimal
    Amount  decimal.Decimal
}

func DetectNegRiskArbitrage(market *NegRiskMarket) *NegRiskArbitrage {
    total := decimal.Zero
    for _, outcome := range market.Outcomes {
        total = total.Add(outcome.BestAsk)
    }

    threshold := decimal.NewFromFloat(0.98) // 2% minimum profit
    if total.LessThan(threshold) {
        profit := decimal.NewFromFloat(1.0).Sub(total)
        return &NegRiskArbitrage{
            ConditionID:    market.ConditionID,
            Outcomes:       market.Outcomes,
            TotalPrice:     total,
            ExpectedProfit: profit,
        }
    }

    return nil
}
```

---

### 3. Cross-Market Correlation Arbitrage

**Principle**: Related markets should have correlated prices.

**Example**:
```
Market A: "Trump wins election" @ 0.55
Market B: "Democrat loses election" @ 0.50

Logic: These events are nearly identical (if Trump wins, Democrat loses)
Arbitrage: Buy B @ 0.50, Sell A @ 0.55
Expected profit: 5% (if correlation = 100%)
```

**Challenges**:
- Correlation not always 100%
- Timing risk (markets resolve at different times)
- Requires manual verification of market relationship

**Implementation Priority**: **Low** (requires significant research and verification)

---

### 4. Time-based Arbitrage

**Principle**: Early market pricing vs. near-settlement pricing.

**Example**:
```
Event: Presidential Election (6 months out)
Early pricing: Trump @ 0.45 (based on polls, long-term uncertainty)

Event: Presidential Election (1 day out)
Late pricing: Trump @ 0.65 (based on late polls, prediction models)

Strategy: Buy early @ 0.45, sell late @ 0.65
Profit: 20%
```

**Challenges**:
- Capital locked for extended period
- Market uncertainty and volatility
- Not true "arbitrage" (directional bet)

**Implementation Priority**: **Not recommended** (more like speculation than arbitrage)

---

## Architecture Design

### Scanner Architecture

```go
type ArbitrageScanner struct {
    client              *Client
    markets             []string // Market condition IDs to monitor
    minProfitThreshold  decimal.Decimal
    gasPrice            decimal.Decimal
    scanInterval        time.Duration

    // Channels
    opportunities chan ArbitrageOpportunity
    errors        chan error
    done          chan struct{}
}

type ArbitrageOpportunity struct {
    Type           ArbitrageType // "Premium", "Discount", "NegRisk"
    ConditionID    string
    Actions        []Action
    ExpectedProfit decimal.Decimal
    GasCost        decimal.Decimal
    NetProfit      decimal.Decimal
    Confidence     float64
    DetectedAt     time.Time
    ExpiresAt      time.Time
}

type ArbitrageType string

const (
    TypePremium  ArbitrageType = "Premium"
    TypeDiscount ArbitrageType = "Discount"
    TypeNegRisk  ArbitrageType = "NegRisk"
)

type Action struct {
    Operation string // "Split", "Merge", "Buy", "Sell"
    TokenID   string
    Amount    decimal.Decimal
    Price     decimal.Decimal
}
```

### Core Workflow

```go
func (s *ArbitrageScanner) Start(ctx context.Context) error {
    ticker := time.NewTicker(s.scanInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            close(s.done)
            return nil

        case <-ticker.C:
            s.scan(ctx)
        }
    }
}

func (s *ArbitrageScanner) scan(ctx context.Context) {
    for _, conditionID := range s.markets {
        // Get market data
        market, err := s.client.GetMarket(ctx, conditionID)
        if err != nil {
            s.errors <- fmt.Errorf("failed to get market %s: %w", conditionID, err)
            continue
        }

        // Check different arbitrage types
        if opp := s.checkPremiumArbitrage(market); opp != nil {
            s.opportunities <- *opp
        }

        if opp := s.checkDiscountArbitrage(market); opp != nil {
            s.opportunities <- *opp
        }

        if market.NegRisk {
            if opp := s.checkNegRiskArbitrage(market); opp != nil {
                s.opportunities <- *opp
            }
        }
    }
}
```

### Gas Cost Optimization

**Challenge**: Arbitrage profit can be eaten by gas costs.

**Strategy**:
```go
func (s *ArbitrageScanner) checkProfitability(profit, gasCost decimal.Decimal) bool {
    // Only execute if profit > gas * safety factor
    safetyFactor := decimal.NewFromInt(5)
    minProfit := gasCost.Mul(safetyFactor)
    return profit.GreaterThan(minProfit)
}

func (s *ArbitrageScanner) estimateGasCost(actions []Action) decimal.Decimal {
    // Estimate based on action types
    gasPerSplit := 150000
    gasPerMerge := 150000
    gasPerTrade := 100000

    totalGas := 0
    for _, action := range actions {
        switch action.Operation {
        case "Split":
            totalGas += gasPerSplit
        case "Merge":
            totalGas += gasPerMerge
        case "Buy", "Sell":
            totalGas += gasPerTrade
        }
    }

    // Convert to USDC cost
    gasPrice := s.getGasPrice() // in gwei
    gasCostInETH := decimal.NewFromInt(int64(totalGas)).Mul(gasPrice).Div(decimal.NewFromFloat(1e9))
    gasCostInUSDC := gasCostInETH.Mul(s.getETHPrice()) // ETH/USDC price

    return gasCostInUSDC
}
```

### Speed Optimization

**Challenge**: Arbitrage windows last 5-30 seconds. Speed is critical.

**Optimizations**:

1. **Parallel Market Scanning**:
```go
func (s *ArbitrageScanner) scanParallel(ctx context.Context) {
    var wg sync.WaitGroup

    for _, conditionID := range s.markets {
        wg.Add(1)
        go func(id string) {
            defer wg.Done()
            s.scanMarket(ctx, id)
        }(conditionID)
    }

    wg.Wait()
}
```

2. **WebSocket Real-time Updates**:
```go
// Instead of polling, subscribe to price changes
func (s *ArbitrageScanner) subscribeToMarkets(ctx context.Context) error {
    for _, conditionID := range s.markets {
        err := s.client.RealtimeClient().SubscribeToCLOBMarketPriceChanges(
            func(changes PriceChanges) error {
                // Immediately check for arbitrage on price change
                s.checkMarketForArbitrage(changes.TokenID)
                return nil
            },
            &CLOBMarketFilter{
                TokenIDs: []string{conditionID},
            },
        )
        if err != nil {
            return err
        }
    }
    return nil
}
```

3. **Order Book Caching**:
```go
type OrderBookCache struct {
    sync.RWMutex
    cache map[string]*CachedOrderBook
}

type CachedOrderBook struct {
    OrderBook  *OrderBookSummary
    UpdatedAt  time.Time
    ValidUntil time.Time
}

func (c *OrderBookCache) Get(tokenID string) (*OrderBookSummary, bool) {
    c.RLock()
    defer c.RUnlock()

    cached, ok := c.cache[tokenID]
    if !ok || time.Now().After(cached.ValidUntil) {
        return nil, false
    }

    return cached.OrderBook, true
}
```

4. **Pre-signed Orders**:
```go
// Pre-create and sign orders for faster execution
type PreSignedOrderPool struct {
    orders map[string]*SignedOrder
    mu     sync.RWMutex
}

func (p *PreSignedOrderPool) PrepareOrders(ctx context.Context, markets []Market) error {
    for _, market := range markets {
        // Pre-sign buy orders for YES/NO at various price levels
        for price := 0.1; price < 0.9; price += 0.05 {
            order := createBuyOrder(market, price)
            signed, err := signOrder(order)
            if err != nil {
                return err
            }
            key := fmt.Sprintf("%s_%s_%.2f", market.ID, "BUY", price)
            p.orders[key] = signed
        }
    }
    return nil
}
```

---

## Execution Strategy

### Atomic Execution

**Challenge**: Multi-step arbitrage requires all steps to succeed, or profit disappears.

**Solution**: Transaction batching where possible

```go
func (s *ArbitrageScanner) executeAtomic(ctx context.Context, opp ArbitrageOpportunity) error {
    // Attempt to batch transactions
    if s.canBatch(opp.Actions) {
        return s.executeBatch(ctx, opp)
    }

    // Otherwise execute sequentially with rollback on failure
    executed := []Action{}

    for _, action := range opp.Actions {
        err := s.executeAction(ctx, action)
        if err != nil {
            // Attempt rollback
            s.rollback(ctx, executed)
            return fmt.Errorf("action %s failed: %w", action.Operation, err)
        }
        executed = append(executed, action)
    }

    return nil
}
```

### Risk Management

```go
type RiskConfig struct {
    MaxPositionSize    decimal.Decimal // Max USDC per arbitrage
    MaxConcurrent      int              // Max concurrent arbitrage executions
    MinProfitPercent   decimal.Decimal  // Minimum profit threshold (e.g., 0.5%)
    GasSafetyFactor    int              // Profit must be > gas * factor
    MaxSlippage        decimal.Decimal  // Max acceptable slippage
}

func (s *ArbitrageScanner) checkRisk(opp ArbitrageOpportunity, config RiskConfig) error {
    // Check profit threshold
    profitPercent := opp.NetProfit.Div(opp.CapitalRequired).Mul(decimal.NewFromInt(100))
    if profitPercent.LessThan(config.MinProfitPercent) {
        return fmt.Errorf("profit too low: %s%%", profitPercent)
    }

    // Check gas safety
    if opp.NetProfit.LessThan(opp.GasCost.Mul(decimal.NewFromInt(int64(config.GasSafetyFactor)))) {
        return fmt.Errorf("profit does not meet gas safety factor")
    }

    // Check position size
    if opp.CapitalRequired.GreaterThan(config.MaxPositionSize) {
        return fmt.Errorf("position size exceeds limit")
    }

    return nil
}
```

---

## API Design

### Scanner Configuration

```go
type ScannerConfig struct {
    // Scanning parameters
    Markets            []string        // Market condition IDs to monitor
    ScanInterval       time.Duration   // Polling interval (if not using WebSocket)
    UseWebSocket       bool            // Use real-time updates vs polling

    // Profitability thresholds
    MinProfitPercent   decimal.Decimal // e.g., 0.5%
    MinAbsoluteProfit  decimal.Decimal // e.g., 1 USDC
    GasSafetyFactor    int             // e.g., 5x

    // Execution settings
    AutoExecute        bool
    MaxConcurrent      int
    MaxPositionSize    decimal.Decimal

    // Callbacks
    OnOpportunity      func(ArbitrageOpportunity) error
    OnExecution        func(ArbitrageResult) error
    OnError            func(error)
}

// Create scanner
func NewArbitrageScanner(client *Client, config ScannerConfig) *ArbitrageScanner {
    return &ArbitrageScanner{
        client:             client,
        config:             config,
        opportunities:      make(chan ArbitrageOpportunity, 100),
        errors:             make(chan error, 100),
        done:               make(chan struct{}),
        orderBookCache:     NewOrderBookCache(),
        executionSemaphore: make(chan struct{}, config.MaxConcurrent),
    }
}
```

### Usage Example

```go
package main

import (
    "context"
    "log"
    polymarket "github.com/ivanzzeth/polymarket-go"
)

func main() {
    ctx := context.Background()

    // Create client
    client, err := polymarket.NewClient(ethClient)
    if err != nil {
        log.Fatal(err)
    }

    // Configure scanner
    scanner := polymarket.NewArbitrageScanner(client, polymarket.ScannerConfig{
        Markets: []string{
            "0x123...", // Trump wins election
            "0x456...", // Bitcoin above $100k
        },
        ScanInterval:      5 * time.Second,
        UseWebSocket:      true,
        MinProfitPercent:  decimal.NewFromFloat(0.5), // 0.5%
        MinAbsoluteProfit: decimal.NewFromFloat(1.0), // 1 USDC
        GasSafetyFactor:   5,
        AutoExecute:       false, // Manual approval
        MaxConcurrent:     3,
        MaxPositionSize:   decimal.NewFromFloat(1000), // 1000 USDC max

        OnOpportunity: func(opp polymarket.ArbitrageOpportunity) error {
            log.Printf("Found arbitrage: %s, profit: %s USDC", opp.Type, opp.NetProfit)
            // Manually review and approve
            return nil
        },

        OnExecution: func(result polymarket.ArbitrageResult) error {
            log.Printf("Executed arbitrage: profit %s USDC", result.ActualProfit)
            return nil
        },

        OnError: func(err error) {
            log.Printf("Error: %v", err)
        },
    })

    // Start scanning
    if err := scanner.Start(ctx); err != nil {
        log.Fatal(err)
    }

    // Listen for opportunities
    for {
        select {
        case opp := <-scanner.Opportunities():
            // Review opportunity
            if shouldExecute(opp) {
                err := scanner.Execute(ctx, opp)
                if err != nil {
                    log.Printf("Execution failed: %v", err)
                }
            }

        case <-ctx.Done():
            scanner.Stop()
            return
        }
    }
}
```

---

## Implementation Priority

### Phase 1: Core Dutch-book Arbitrage ⭐

**Week 1-2**:
- [x] Premium arbitrage detection
- [x] Discount arbitrage detection
- [x] Gas cost estimation
- [x] Profitability calculation
- [ ] Basic execution logic

**Week 3**:
- [ ] WebSocket integration for real-time updates
- [ ] Order book caching
- [ ] Parallel market scanning

**Week 4**:
- [ ] Atomic execution with rollback
- [ ] Risk management rules
- [ ] Comprehensive testing

### Phase 2: NegRisk Market Arbitrage

**Week 5-6**:
- [ ] Multi-outcome market detection
- [ ] NegRisk arbitrage logic
- [ ] Execution strategy

### Phase 3: Optimizations

**Week 7-8**:
- [ ] Speed optimizations (pre-signed orders, batching)
- [ ] Advanced risk management
- [ ] Monitoring and alerting

---

## Success Criteria

- ✅ Detect arbitrage opportunities within 1 second of price change
- ✅ Execute profitable arbitrage in < 5 seconds end-to-end
- ✅ Accuracy > 95% (no false positives leading to losses)
- ✅ Support scanning 100+ markets concurrently
- ✅ Gas cost estimation within 10% of actual cost
- ✅ Profitable arbitrage execution rate > 80% (when auto-execute enabled)

---

## Realistic Assessment

**Previous research claimed $40M+ in arbitrage profits**, but given the mirrored order book constraint, we must reconsider:

### What the $40M Likely Represents:

1. **Market Making Profits** (not pure arbitrage):
   - Capturing bid-ask spreads
   - Providing liquidity during volatile periods
   - Inventory risk management

2. **Information Arbitrage**:
   - Trading ahead of news/events
   - Faster price discovery than other traders
   - NOT risk-free structural arbitrage

3. **Cross-Platform Arbitrage**:
   - Polymarket vs Kalshi, PredictIt, etc.
   - Different platforms have independent order books
   - Real arbitrage opportunity, but requires capital on multiple platforms

4. **Temporary Mispricing During High Volatility**:
   - During major events (election night, breaking news)
   - Brief windows before mirroring catches up
   - Extremely short-lived (milliseconds to seconds)

### Reality Check:

**Pure risk-free arbitrage on Polymarket is likely RARE or NON-EXISTENT** due to:
- Mirrored order books automatically prevent YES + NO ≠ 1.0
- Matching engine eliminates obvious arbitrage instantly
- Sophisticated bots already monitor any edge cases

**What IS profitable**:
- Market making (requires capital and risk tolerance)
- Cross-platform arbitrage (requires multi-platform setup)
- Information edge (trading on news/data faster than market)
- Volatility trading during major events

**Competition Level**: Extremely high - professional market makers dominate

**Key to Success**:
- Speed (sub-second execution)
- Capital (to handle inventory)
- Risk management (not pure arbitrage)
