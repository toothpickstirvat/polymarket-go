# polymarket-go Development Decision

## Final Decision

Focus on **Polymarket-specific features** only, delegate generic quant features to BBGO framework.

## Core Modules

### 1. Smart Order Conversion (✅ Completed)
- `GetComplementaryTokenID` - Get complementary token ID
- `ConvertLimitOrder` - Convert limit orders
- `ConvertMarketOrder` - Convert market orders
- `GetConditionIDByTokenID` - Query condition ID
- **Value**: Solve position sync delay, enable immediate take-profit

### 2. Auto Management (✅ Completed)
- `WithAutoRedeem` - Auto-redeem resolved positions
- `WithAutoMerge` - Auto-merge complementary tokens (YES + NO → USDC)
- `Close()` - Graceful shutdown
- **Value**: Hands-free position management

### 3. Arbitrage Scanner (⚠️ Needs Redesign)

**CRITICAL FINDING**: Polymarket order books are **mirrored** - traditional Dutch-book arbitrage is **NOT POSSIBLE**.

**Mirrored Order Book Constraint**:
```
YES Bid = 1 - NO Ask
YES Ask = 1 - NO Bid
```
This means YES + NO always equals 1.0, eliminating traditional arbitrage.

**What Actually Works**:

**A. Market Making (Spread Capture)**:
- Provide liquidity by placing orders at bid/ask
- Profit from spread when both sides fill
- **Not risk-free**: Requires inventory management and directional risk
- Example: Buy YES @ 0.53, Sell @ 0.55 → 2 cent profit per share

**B. Cross-Platform Arbitrage**:
- Polymarket vs Kalshi, PredictIt, etc.
- Different platforms = independent order books
- Example: Buy on Kalshi @ 0.60, Sell on Polymarket @ 0.65 → 5% profit
- **Challenge**: Requires capital on multiple platforms, withdrawal delays

**C. NegRisk Market Arbitrage** (needs verification):
- Multi-outcome markets may not have same mirroring
- If sum(outcome_asks) < 1.0, buy all outcomes
- **Status**: Unverified if this can actually exist

**Historical "$40M" Reality Check**:
- Likely market making profits, NOT pure arbitrage
- Information edge trading (news/events)
- Cross-platform opportunities
- High volatility event trading

**Revised Focus**: Market making and cross-platform arbitrage, NOT Dutch-book arbitrage.

**Value**: Understanding market microstructure is valuable, but pure arbitrage is limited.

### 4. NegRisk Handler (Planned)
- NegRisk market detection
- Special redemption logic
- Risk assessment
- **Value**: Handle Polymarket's unique market type

### 5. BBGO Integration (Planned)
- Implement Exchange interface
- Implement Stream interface
- Type conversion layer
- **Value**: Reuse mature quant ecosystem

## What We Don't Build

Delegate to BBGO:
- ❌ Analytics - Performance analysis (win rate, profit/loss ratio, returns)
- ❌ RiskManager - Risk management
- ❌ Monitor - Real-time monitoring

**Reason**: BBGO already provides complete quant infrastructure for these generic features, avoid reinventing the wheel.

## Key Principles

1. **Focus on differentiation**: Build only Polymarket-specific features
2. **Reuse ecosystem**: Integrate with BBGO for generic features
3. **Production quality**: All code must be production-ready
4. **Simple is better**: Avoid over-engineering

## Implementation Status

### Phase 1: Smart Order Conversion (✅ Completed)
- Order conversion tools
- Auto-management features
- Examples and documentation
- Coding standards

### Phase 2: Market Making / Cross-Platform Arbitrage (Redesigned)

**Status**: On hold pending decision on direction

**Options**:

**Option A: Market Making Bot**
- Spread detection and liquidity provision
- Inventory risk management
- Not pure arbitrage - requires capital and risk tolerance

**Option B: Cross-Platform Arbitrage**
- Requires integration with Kalshi, PredictIt APIs
- Multi-platform capital requirements
- Withdrawal/deposit timing challenges

**Option C: Pivot to Other Polymarket-Specific Features**
- Enhanced order routing with slippage optimization
- Advanced position management tools
- Strategy backtesting framework

**Recommendation**: Discuss with users what direction makes most sense given the arbitrage limitations discovered.

### Phase 3: BBGO Integration (Future)
- Exchange interface
- Stream interface
- Strategy examples

## Success Criteria

### Phase 1 (Completed)
- ✅ Smart order conversion accuracy > 95%
- ✅ Average cost savings > 15%
- ✅ Order optimization < 100ms
- ✅ Clear documentation with runnable examples

### Phase 2 (Arbitrage Scanner)
- Detection latency < 1 second after price change
- End-to-end execution < 5 seconds
- Accuracy > 95% (no false positives leading to losses)
- Support 100+ markets concurrently
- Gas cost estimation within 10% of actual
- Profitable execution rate > 80% (when auto-execute enabled)
