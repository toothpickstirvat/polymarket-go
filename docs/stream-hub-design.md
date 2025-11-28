# StreamHub Design Document

## Project Goals

**Target**: Build infrastructure for **institutional-grade, production-ready sniper bots** - a money-printing machine.

**Design Principles**:
1. **Correctness over speed** - Wrong fast data is worse than slow correct data
2. **Fail-safe defaults** - When in doubt, reject the trade
3. **Observable everything** - If you can't measure it, you can't improve it
4. **Defense in depth** - Multiple layers of validation

---

## Performance Requirements (Non-negotiable)

| Operation | Target Latency | P99 | P999 | Description |
|-----------|---------------|-----|------|-------------|
| Local Query | **< 1 us** | < 5 us | < 10 us | Balance, position, order book queries |
| State Update | **< 1 ms** | < 5 ms | < 10 ms | Processing WebSocket events |
| Event Propagation | **< 100 us** | < 500 us | < 1 ms | State change to callback |
| Order Submission | **< 50 ms** | < 100 ms | < 200 ms | Full round-trip to exchange |

**Throughput Requirements**:
- Order book updates: **10,000+ messages/second** per market
- Concurrent markets: **100+** markets simultaneously
- Open orders tracking: **1,000+** orders per account

## Core Principle

**ALL state is maintained locally. Zero network calls for queries.**

Local state includes:
- Collateral balance (USDC)
- Collateral locked amount
- Position balances (per tokenID)
- Position locked amounts (per tokenID)
- Open orders (full order details)
- Order book snapshots (per tokenID)

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         StreamHub                                    │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    StateStore (in-memory)                    │   │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐ │   │
│  │  │ Collateral   │ │  Positions   │ │    Order Books       │ │   │
│  │  │ Total/Locked │ │ Total/Locked │ │  Bids/Asks/Spread    │ │   │
│  │  └──────────────┘ └──────────────┘ └──────────────────────┘ │   │
│  │  ┌──────────────────────────────────────────────────────────┐│   │
│  │  │                    Open Orders                           ││   │
│  │  │              orderID -> Order (full details)             ││   │
│  │  └──────────────────────────────────────────────────────────┘│   │
│  └─────────────────────────────────────────────────────────────┘   │
│                              │                                      │
│                              │ State Changes                        │
│                              ▼                                      │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Event Emitter                             │   │
│  │  OnBalanceUpdate | OnPositionUpdate | OnOrderUpdate | ...    │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                              ▲                                      │
│                              │ Updates                              │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Update Sources                            │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │   │
│  │  │  WebSocket  │  │  REST Sync  │  │  On-Chain Verify    │  │   │
│  │  │  (fastest)  │  │  (fallback) │  │  (source of truth)  │  │   │
│  │  │   ~10ms     │  │   ~100ms    │  │      ~2s            │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

## Data Structures

### Reused Existing Types

The following types from the existing codebase are reused directly:

```go
// From data_source.go
type DataSource int
const (
    DataSourceCLOB    DataSource = iota  // Query from CLOB API
    DataSourceOnChain                     // Query from blockchain
)

// From balance.go
type BalanceDetail struct {
    TotalBalance     decimal.Decimal
    LockedBalance    decimal.Decimal
    AvailableBalance decimal.Decimal
}
```

### New Types to Define

```go
// =============================================================================
// Order Types
// =============================================================================

type OrderSide string
const (
    OrderSideBuy  OrderSide = "BUY"
    OrderSideSell OrderSide = "SELL"
)

type OrderStatus string
const (
    OrderStatusOpen            OrderStatus = "LIVE"
    OrderStatusPartiallyFilled OrderStatus = "MATCHED"  // Partially filled
    OrderStatusFilled          OrderStatus = "MATCHED"  // Fully filled (SizeMatched == Size)
    OrderStatusCancelled       OrderStatus = "CANCELLED"
)

// Order represents a trading order (mirrors CLOB API order structure)
type Order struct {
    ID          string
    TokenID     string           // Asset ID (conditional token ID)
    Side        OrderSide
    Price       decimal.Decimal
    Size        decimal.Decimal  // Original size
    SizeMatched decimal.Decimal  // Amount filled
    Status      OrderStatus
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

func (o *Order) RemainingSize() decimal.Decimal {
    return o.Size.Sub(o.SizeMatched)
}

// OrderUpdate represents an order state change event
type OrderUpdate struct {
    OrderID     string
    Status      OrderStatus
    SizeMatched decimal.Decimal
    Timestamp   time.Time
    Source      UpdateSource     // Where this update came from
    Sequence    uint64           // For ordering/deduplication
}

type UpdateSource string
const (
    UpdateSourceWebSocket UpdateSource = "websocket"
    UpdateSourceREST      UpdateSource = "rest"
    UpdateSourceLocal     UpdateSource = "local"      // Optimistic update from local action
    UpdateSourceOnChain   UpdateSource = "onchain"
)

// =============================================================================
// Order Book Types
// =============================================================================

type PriceLevel struct {
    Price decimal.Decimal
    Size  decimal.Decimal
}

type OrderBook struct {
    TokenID    string
    Bids       []PriceLevel      // Sorted descending by price
    Asks       []PriceLevel      // Sorted ascending by price
    BestBid    decimal.Decimal
    BestAsk    decimal.Decimal
    Spread     decimal.Decimal
    LastUpdate time.Time
    Sequence   uint64
}

type OrderBookDelta struct {
    TokenID  string
    Sequence uint64
    Bids     []PriceLevel  // Updated bid levels (Size=0 means remove)
    Asks     []PriceLevel  // Updated ask levels (Size=0 means remove)
}

type OrderBookUpdate struct {
    TokenID   string
    BestBid   decimal.Decimal
    BestAsk   decimal.Decimal
    Spread    decimal.Decimal
    Timestamp time.Time
}

// =============================================================================
// Position Types (internal state)
// =============================================================================

type PositionState struct {
    TokenID string
    Total   decimal.Decimal
    Locked  decimal.Decimal  // Locked in SELL orders
}

// =============================================================================
// Trade Types
// =============================================================================

type Trade struct {
    ID        string
    OrderID   string
    TokenID   string
    Side      OrderSide
    Price     decimal.Decimal
    Size      decimal.Decimal
    Timestamp time.Time
}

// =============================================================================
// Event Types
// =============================================================================

type EventType string
const (
    EventTypeBalanceUpdate   EventType = "balance_update"
    EventTypePositionUpdate  EventType = "position_update"
    EventTypeOrderUpdate     EventType = "order_update"
    EventTypeOrderBookUpdate EventType = "orderbook_update"
    EventTypeTrade           EventType = "trade"
    EventTypeConnection      EventType = "connection"
    EventTypeReconciliation  EventType = "reconciliation"
)

type Event interface {
    Type() EventType
    Timestamp() time.Time
}

type BalanceUpdateEvent struct {
    Balance   *BalanceDetail
    Source    UpdateSource
    EventTime time.Time
}

func (e *BalanceUpdateEvent) Type() EventType      { return EventTypeBalanceUpdate }
func (e *BalanceUpdateEvent) Timestamp() time.Time { return e.EventTime }

type PositionUpdateEvent struct {
    TokenID   string
    Balance   *BalanceDetail
    Source    UpdateSource
    EventTime time.Time
}

func (e *PositionUpdateEvent) Type() EventType      { return EventTypePositionUpdate }
func (e *PositionUpdateEvent) Timestamp() time.Time { return e.EventTime }

type OrderUpdateEvent struct {
    Order     *Order
    Update    OrderUpdate
    EventTime time.Time
}

func (e *OrderUpdateEvent) Type() EventType      { return EventTypeOrderUpdate }
func (e *OrderUpdateEvent) Timestamp() time.Time { return e.EventTime }

// =============================================================================
// Connection Types
// =============================================================================

type ConnectionState int
const (
    ConnectionStateDisconnected ConnectionState = iota
    ConnectionStateConnecting
    ConnectionStateConnected
)

type ConnectionEvent string
const (
    ConnectionEventConnected    ConnectionEvent = "connected"
    ConnectionEventDisconnected ConnectionEvent = "disconnected"
    ConnectionEventReconnected  ConnectionEvent = "reconnected"
)

// =============================================================================
// Reconciliation Types
// =============================================================================

type ReconciliationType string
const (
    ReconciliationTypeBalance  ReconciliationType = "balance"
    ReconciliationTypePosition ReconciliationType = "position"
    ReconciliationTypeOrder    ReconciliationType = "order"
)

type ReconciliationEvent struct {
    Type        ReconciliationType
    TokenID     string           // For position reconciliation
    LocalValue  decimal.Decimal
    TrueValue   decimal.Decimal
    Discrepancy decimal.Decimal
    Timestamp   time.Time
}

// =============================================================================
// State Change Types (for optimistic update rollback)
// =============================================================================

type StateChange interface {
    Apply()
    Rollback()
}

type CollateralLockChange struct {
    store  *StateStore
    Amount decimal.Decimal
}

func (c *CollateralLockChange) Apply() {
    c.store.collateralLocked = c.store.collateralLocked.Add(c.Amount)
}

func (c *CollateralLockChange) Rollback() {
    c.store.collateralLocked = c.store.collateralLocked.Sub(c.Amount)
}

type PositionLockChange struct {
    store   *StateStore
    TokenID string
    Amount  decimal.Decimal
}

func (c *PositionLockChange) Apply() {
    if pos, ok := c.store.positions[c.TokenID]; ok {
        pos.Locked = pos.Locked.Add(c.Amount)
    }
}

func (c *PositionLockChange) Rollback() {
    if pos, ok := c.store.positions[c.TokenID]; ok {
        pos.Locked = pos.Locked.Sub(c.Amount)
    }
}

// =============================================================================
// Error Types
// =============================================================================

var (
    ErrInsufficientBalance = errors.New("insufficient balance")
    ErrInvariantViolation  = errors.New("state invariant violation")
    ErrGapTooLarge         = errors.New("sequence gap too large, resync required")
    ErrRateLimited         = errors.New("rate limited")
    ErrOrderNotFound       = errors.New("order not found")
)
```

### StateStore

```go
type StateStore struct {
    // Lock-free reads using atomic pointers where possible
    // RWMutex for complex structures
    mu sync.RWMutex

    // Collateral (USDC)
    collateralTotal  decimal.Decimal
    collateralLocked decimal.Decimal  // Pre-calculated, not derived

    // Positions: tokenID -> balance state
    positions map[string]*PositionState

    // Open Orders: orderID -> order
    // Also indexed by tokenID for fast lookup
    openOrders      map[string]*Order
    ordersByToken   map[string]map[string]*Order  // tokenID -> orderID -> Order

    // Order Books: tokenID -> order book
    orderBooks map[string]*OrderBook

    // Metadata
    lastUpdateTime time.Time
    updateSequence uint64
}

type PositionState struct {
    TokenID string
    Total   decimal.Decimal
    Locked  decimal.Decimal  // Pre-calculated from SELL orders
}

type OrderBook struct {
    TokenID    string
    Bids       []PriceLevel  // Sorted descending by price
    Asks       []PriceLevel  // Sorted ascending by price
    BestBid    decimal.Decimal
    BestAsk    decimal.Decimal
    Spread     decimal.Decimal
    LastUpdate time.Time
}
```

### Query Interface (Zero-copy where possible)

```go
// All queries return cached values - NO network calls
// Target: < 1 microsecond

func (s *StateStore) GetCollateralBalance() *BalanceDetail {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return &BalanceDetail{
        TotalBalance:     s.collateralTotal,
        LockedBalance:    s.collateralLocked,
        AvailableBalance: s.collateralTotal.Sub(s.collateralLocked),
    }
}

func (s *StateStore) GetPositionBalance(tokenID string) *BalanceDetail {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if pos, ok := s.positions[tokenID]; ok {
        return &BalanceDetail{
            TotalBalance:     pos.Total,
            LockedBalance:    pos.Locked,
            AvailableBalance: pos.Total.Sub(pos.Locked),
        }
    }
    return &BalanceDetail{}
}

func (s *StateStore) GetOrderBook(tokenID string) *OrderBook {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.orderBooks[tokenID]
}

func (s *StateStore) GetOpenOrders() []*Order {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // Return slice, pre-allocated
}
```

## Locked Amount Tracking

**Critical**: Locked amounts are tracked incrementally, NOT calculated on query.

### When Order is Created (local action)

```go
func (s *StateStore) OnOrderCreated(order *Order) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Add to open orders
    s.openOrders[order.ID] = order

    // Update locked amount immediately
    if order.Side == OrderSideBuy {
        // BUY order locks collateral
        lockAmount := order.RemainingSize().Mul(order.Price)
        s.collateralLocked = s.collateralLocked.Add(lockAmount)
    } else {
        // SELL order locks position tokens
        lockAmount := order.RemainingSize()
        if pos, ok := s.positions[order.TokenID]; ok {
            pos.Locked = pos.Locked.Add(lockAmount)
        }
    }

    s.updateSequence++
    s.lastUpdateTime = time.Now()
}
```

### When Order is Filled/Cancelled (from WebSocket)

```go
func (s *StateStore) OnOrderUpdate(update OrderUpdate) {
    s.mu.Lock()
    defer s.mu.Unlock()

    order, exists := s.openOrders[update.OrderID]
    if !exists {
        return
    }

    switch update.Status {
    case OrderStatusFilled, OrderStatusCancelled:
        // Release locked amount
        if order.Side == OrderSideBuy {
            lockAmount := order.RemainingSize().Mul(order.Price)
            s.collateralLocked = s.collateralLocked.Sub(lockAmount)
        } else {
            if pos, ok := s.positions[order.TokenID]; ok {
                pos.Locked = pos.Locked.Sub(order.RemainingSize())
            }
        }
        delete(s.openOrders, order.ID)

    case OrderStatusPartiallyFilled:
        // Adjust locked amount for the filled portion
        filledDelta := update.SizeMatched.Sub(order.SizeMatched)
        if order.Side == OrderSideBuy {
            s.collateralLocked = s.collateralLocked.Sub(filledDelta.Mul(order.Price))
            // Also update collateral total (received position, spent USDC)
        } else {
            if pos, ok := s.positions[order.TokenID]; ok {
                pos.Locked = pos.Locked.Sub(filledDelta)
            }
        }
        order.SizeMatched = update.SizeMatched
    }

    s.updateSequence++
    s.lastUpdateTime = time.Now()
}
```

## Event System

```go
type StreamHub struct {
    state  *StateStore
    client *Client

    // Callbacks - use slices for multiple subscribers
    // Consider lock-free append for high-frequency scenarios
    balanceUpdateCallbacks   []func(*BalanceDetail)
    positionUpdateCallbacks  []func(tokenID string, balance *BalanceDetail)
    orderUpdateCallbacks     []func(OrderUpdate)
    orderBookUpdateCallbacks []func(OrderBookUpdate)
    tradeCallbacks           []func(Trade)
}

// Event emission - target < 100us
func (h *StreamHub) emitBalanceUpdate(balance *BalanceDetail) {
    for _, cb := range h.balanceUpdateCallbacks {
        cb(balance)  // Consider async dispatch for slow callbacks
    }
}
```

---

## Concurrency Model

### Thread Safety Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                          StreamHub Goroutines                          │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐   │
│  │  WebSocket      │    │  REST Sync      │    │  On-Chain       │   │
│  │  Reader         │    │  Worker         │    │  Reconciler     │   │
│  │  (1 goroutine)  │    │  (1 goroutine)  │    │  (1 goroutine)  │   │
│  └────────┬────────┘    └────────┬────────┘    └────────┬────────┘   │
│           │                      │                      │            │
│           ▼                      ▼                      ▼            │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                     Update Channel (buffered)                 │    │
│  │                     capacity: 10000                           │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                  │                                    │
│                                  ▼                                    │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                   State Updater (1 goroutine)                 │    │
│  │   - Single writer to StateStore                               │    │
│  │   - Processes updates sequentially                            │    │
│  │   - Emits events after state mutation                         │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                  │                                    │
│                                  ▼                                    │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                   Event Dispatcher (N goroutines)             │    │
│  │   - Async callback invocation                                 │    │
│  │   - Non-blocking (slow callbacks don't block updates)         │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                   Query Handlers (unlimited)                  │    │
│  │   - Read-only access to StateStore                            │    │
│  │   - RLock for concurrent reads                                │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### Single Writer Principle

```go
// CRITICAL: Only one goroutine writes to StateStore
// This eliminates write contention and simplifies reasoning

type StateUpdater struct {
    updates chan StateUpdate
    state   *StateStore
    events  *EventDispatcher
}

func (u *StateUpdater) Run(ctx context.Context) {
    for {
        select {
        case update := <-u.updates:
            // Process update (single writer - no write lock contention)
            u.state.applyUpdate(update)

            // Emit events (async, non-blocking)
            u.events.Dispatch(update.ToEvent())

        case <-ctx.Done():
            return
        }
    }
}

// applyUpdate is called from single goroutine - minimal locking needed
func (s *StateStore) applyUpdate(update StateUpdate) {
    s.mu.Lock()
    defer s.mu.Unlock()

    switch u := update.(type) {
    case *OrderCreatedUpdate:
        s.openOrders[u.Order.ID] = u.Order
        if u.Order.Side == OrderSideBuy {
            s.collateralLocked = s.collateralLocked.Add(u.LockAmount)
        }
        // ...
    }

    s.updateSequence++
    s.lastUpdateTime = time.Now()
}
```

### Non-Blocking Event Dispatch

```go
type EventDispatcher struct {
    workers    int
    eventQueue chan Event
    callbacks  map[EventType][]func(Event)
}

func NewEventDispatcher(workers int, queueSize int) *EventDispatcher {
    d := &EventDispatcher{
        workers:    workers,
        eventQueue: make(chan Event, queueSize),
        callbacks:  make(map[EventType][]func(Event)),
    }

    // Start worker pool
    for i := 0; i < workers; i++ {
        go d.worker()
    }

    return d
}

func (d *EventDispatcher) Dispatch(event Event) {
    select {
    case d.eventQueue <- event:
        // Queued successfully
    default:
        // Queue full - drop event and emit warning
        // This is acceptable: events are notifications, not critical data
        emitMetric("event.dropped", 1, "type", event.Type())
    }
}

func (d *EventDispatcher) worker() {
    for event := range d.eventQueue {
        callbacks := d.callbacks[event.Type()]
        for _, cb := range callbacks {
            // Recover from panics in callbacks
            func() {
                defer func() {
                    if r := recover(); r != nil {
                        emitAlert(AlertLevelWarning, "callback_panic", map[string]string{
                            "event_type": string(event.Type()),
                            "panic":      fmt.Sprintf("%v", r),
                        })
                    }
                }()
                cb(event)
            }()
        }
    }
}
```

---

## Data Flow Diagrams

### Order Placement Flow (Optimistic Update)

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│   Bot    │     │  Client  │     │ StreamHub│     │ Exchange │
└────┬─────┘     └────┬─────┘     └────┬─────┘     └────┬─────┘
     │                │                │                │
     │ PlaceOrder()   │                │                │
     │───────────────>│                │                │
     │                │                │                │
     │                │ Submit to API  │                │
     │                │───────────────────────────────>│
     │                │                │                │
     │                │ API Response   │                │
     │                │<───────────────────────────────│
     │                │                │                │
     │                │ Optimistic     │                │
     │                │ Update         │                │
     │                │───────────────>│                │
     │                │                │                │
     │                │                │ Update State   │
     │                │                │ (lock balance) │
     │                │                │                │
     │                │                │ Emit Event     │
     │                │                │───────┐        │
     │                │                │       │        │
     │ Return Order   │                │<──────┘        │
     │<───────────────│                │                │
     │                │                │                │
     │ [Later: WS confirms order]      │                │
     │                │                │ WS Notification│
     │                │                │<───────────────│
     │                │                │                │
     │                │                │ Confirm        │
     │                │                │ Optimistic     │
     │                │                │ Update         │
     │                │                │                │
     └                └                └                └

Timeline: ~50ms total for PlaceOrder() return
          State updated immediately (< 1ms after API response)
```

### Balance Update Flow (Multi-Source)

```
┌─────────────────────────────────────────────────────────────────┐
│                        Data Sources                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   WebSocket (~10ms)         REST (~100ms)        On-Chain (~2s) │
│        │                         │                     │        │
│        │ Balance                 │ Full               │ True    │
│        │ Update                  │ Sync               │ Balance │
│        ▼                         ▼                     ▼        │
│   ┌─────────┐              ┌─────────┐           ┌─────────┐   │
│   │ Sequence│              │ Replace │           │ Verify  │   │
│   │ Check   │              │ Snapshot│           │ & Alert │   │
│   └────┬────┘              └────┬────┘           └────┬────┘   │
│        │                        │                     │        │
│        └────────────┬───────────┴─────────────────────┘        │
│                     │                                           │
│                     ▼                                           │
│              ┌─────────────┐                                    │
│              │   Merge &   │                                    │
│              │  Reconcile  │                                    │
│              └──────┬──────┘                                    │
│                     │                                           │
│                     ▼                                           │
│              ┌─────────────┐                                    │
│              │ StateStore  │                                    │
│              │  (Single    │                                    │
│              │   Source    │                                    │
│              │   of Local  │                                    │
│              │   Truth)    │                                    │
│              └─────────────┘                                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘

Priority Rules:
1. WebSocket: Apply immediately (fastest)
2. REST: Replace if sequence > local sequence
3. On-Chain: Alert if discrepancy > threshold, force resync
```

---

## Error Handling Strategy

### Error Categories

```go
type ErrorSeverity int

const (
    // Recoverable: Retry and continue
    ErrorSeverityRecoverable ErrorSeverity = iota

    // Degraded: Continue with reduced functionality
    ErrorSeverityDegraded

    // Critical: Halt trading, alert operator
    ErrorSeverityCritical
)

type CategorizedError struct {
    Severity    ErrorSeverity
    Category    string
    Message     string
    Retryable   bool
    RetryAfter  time.Duration
    Underlying  error
}

// Error categorization
func categorizeError(err error) *CategorizedError {
    switch {
    case errors.Is(err, ErrRateLimited):
        return &CategorizedError{
            Severity:   ErrorSeverityRecoverable,
            Category:   "rate_limit",
            Retryable:  true,
            RetryAfter: 1 * time.Second,
        }

    case errors.Is(err, ErrInsufficientBalance):
        return &CategorizedError{
            Severity:   ErrorSeverityDegraded,
            Category:   "balance",
            Retryable:  false,
        }

    case errors.Is(err, ErrInvariantViolation):
        return &CategorizedError{
            Severity:   ErrorSeverityCritical,
            Category:   "invariant",
            Retryable:  false,
        }

    case isNetworkError(err):
        return &CategorizedError{
            Severity:   ErrorSeverityRecoverable,
            Category:   "network",
            Retryable:  true,
            RetryAfter: 100 * time.Millisecond,
        }

    default:
        return &CategorizedError{
            Severity:   ErrorSeverityDegraded,
            Category:   "unknown",
            Retryable:  false,
        }
    }
}
```

### Retry Strategy

```go
type RetryConfig struct {
    MaxAttempts     int
    InitialDelay    time.Duration
    MaxDelay        time.Duration
    BackoffFactor   float64
    JitterFactor    float64
}

var DefaultRetryConfig = RetryConfig{
    MaxAttempts:   5,
    InitialDelay:  100 * time.Millisecond,
    MaxDelay:      10 * time.Second,
    BackoffFactor: 2.0,
    JitterFactor:  0.2,
}

func RetryWithBackoff[T any](
    ctx context.Context,
    config RetryConfig,
    operation func() (T, error),
) (T, error) {
    var zero T
    var lastErr error

    delay := config.InitialDelay

    for attempt := 0; attempt < config.MaxAttempts; attempt++ {
        result, err := operation()
        if err == nil {
            return result, nil
        }

        catErr := categorizeError(err)
        if !catErr.Retryable {
            return zero, err
        }

        lastErr = err

        // Calculate delay with jitter
        jitter := time.Duration(float64(delay) * config.JitterFactor * (rand.Float64()*2 - 1))
        sleepTime := delay + jitter

        if catErr.RetryAfter > 0 {
            sleepTime = catErr.RetryAfter
        }

        select {
        case <-time.After(sleepTime):
            delay = time.Duration(float64(delay) * config.BackoffFactor)
            if delay > config.MaxDelay {
                delay = config.MaxDelay
            }
        case <-ctx.Done():
            return zero, ctx.Err()
        }
    }

    return zero, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

---

## Edge Cases & Failure Modes

### 1. Stale Optimistic Update

**Scenario**: Order placed, optimistic update applied, but WS confirmation never arrives.

```go
// Solution: Timeout-based rollback + REST verification
func (m *OptimisticUpdateManager) handleStaleUpdate(id string) {
    // 1. Rollback optimistic changes
    m.rollback(id)

    // 2. Query REST API for true order status
    order, err := m.client.GetOrder(context.Background(), id)
    if err != nil {
        // Order doesn't exist - rollback was correct
        return
    }

    // 3. Order exists - re-apply based on actual state
    if order.Status == OrderStatusOpen {
        m.hub.state.OnOrderCreated(order)
    }
}
```

### 2. Balance Drift

**Scenario**: Local balance gradually drifts from true value due to missed updates.

```go
// Solution: Periodic reconciliation with increasing frequency on discrepancy
func (r *Reconciler) adaptiveReconciliation(ctx context.Context) {
    baseInterval := 30 * time.Second
    currentInterval := baseInterval
    consecutiveDiscrepancies := 0

    for {
        select {
        case <-time.After(currentInterval):
            onChain, _ := r.getOnChainBalance(ctx)
            local := r.hub.state.GetCollateralBalance()

            discrepancy := onChain.Sub(local.TotalBalance).Abs()

            if discrepancy.GreaterThan(r.threshold) {
                consecutiveDiscrepancies++

                // Increase reconciliation frequency
                currentInterval = baseInterval / time.Duration(1<<min(consecutiveDiscrepancies, 4))

                // Force REST sync
                r.forceRESTSync(ctx)

                if consecutiveDiscrepancies >= 3 {
                    // Persistent discrepancy - alert and potentially halt
                    r.hub.emitAlert(AlertLevelCritical, "persistent_balance_drift", nil)
                }
            } else {
                // Reset on success
                consecutiveDiscrepancies = 0
                currentInterval = baseInterval
            }

        case <-ctx.Done():
            return
        }
    }
}
```

### 3. Partial Fill Race Condition

**Scenario**: Order partially fills while we're placing another order using stale available balance.

```go
// Solution: Pessimistic balance reservation
func (rm *RiskManager) ReserveBalance(amount decimal.Decimal) (func(), error) {
    rm.mu.Lock()
    defer rm.mu.Unlock()

    available := rm.hub.state.GetCollateralBalance().AvailableBalance
    pendingReservations := rm.getTotalReservations()

    effectiveAvailable := available.Sub(pendingReservations)

    if effectiveAvailable.LessThan(amount) {
        return nil, ErrInsufficientBalance
    }

    reservationID := uuid.New().String()
    rm.reservations[reservationID] = amount

    // Return release function
    return func() {
        rm.mu.Lock()
        delete(rm.reservations, reservationID)
        rm.mu.Unlock()
    }, nil
}

// Usage in order placement
func (c *Client) PlaceOrderSafe(ctx context.Context, input OrderInput) (*Order, error) {
    requiredAmount := input.Size.Mul(input.Price)

    release, err := c.riskManager.ReserveBalance(requiredAmount)
    if err != nil {
        return nil, err
    }
    defer release()  // Release reservation regardless of outcome

    return c.PlaceOrder(ctx, input)
}
```

### 4. Order Book Snapshot vs Delta Desync

**Scenario**: Received deltas but missed snapshot, order book is corrupted.

```go
// Solution: Sequence tracking + automatic resync
type OrderBookManager struct {
    books           map[string]*OrderBook
    lastSequences   map[string]uint64
    snapshotPending map[string]bool
}

func (m *OrderBookManager) ApplyDelta(tokenID string, delta OrderBookDelta) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    book, exists := m.books[tokenID]
    if !exists || m.snapshotPending[tokenID] {
        // No snapshot yet - request one
        m.requestSnapshot(tokenID)
        return nil
    }

    expectedSeq := m.lastSequences[tokenID] + 1
    if delta.Sequence != expectedSeq {
        if delta.Sequence > expectedSeq {
            // Gap detected - request snapshot
            m.snapshotPending[tokenID] = true
            m.requestSnapshot(tokenID)
            emitMetric("orderbook.sequence_gap", float64(delta.Sequence-expectedSeq))
        }
        // Old delta - ignore
        return nil
    }

    book.ApplyDelta(delta)
    m.lastSequences[tokenID] = delta.Sequence

    return nil
}
```

---

## Data Source Hierarchy

| Priority | Source | Latency | Use Case |
|----------|--------|---------|----------|
| 1 | WebSocket | ~10ms | Real-time updates, primary source |
| 2 | REST API | ~100ms | Periodic sync, gap fill, initial load |
| 3 | On-Chain | ~2s | Verification, reconciliation, source of truth |

### Reconciliation Strategy

```go
type Reconciler struct {
    hub    *StreamHub
    client *Client

    // Configurable intervals
    restSyncInterval    time.Duration  // Default: 5s
    onChainSyncInterval time.Duration  // Default: 30s

    // Discrepancy thresholds
    balanceDiscrepancyThreshold decimal.Decimal  // e.g., 0.01 USDC
}

func (r *Reconciler) Start(ctx context.Context) {
    go r.runWebSocketStream(ctx)      // Primary: real-time
    go r.runRESTSync(ctx)             // Secondary: periodic full sync
    go r.runOnChainReconciliation(ctx) // Tertiary: truth verification
}

// On-chain reconciliation
func (r *Reconciler) runOnChainReconciliation(ctx context.Context) {
    ticker := time.NewTicker(r.onChainSyncInterval)
    for {
        select {
        case <-ticker.C:
            onChainBalance, _ := r.client.GetCollateralBalance(ctx, &BalanceQueryOption{
                Source: DataSourceOnChain,
            })

            localBalance := r.hub.state.GetCollateralBalance()

            discrepancy := onChainBalance.Sub(localBalance.TotalBalance).Abs()
            if discrepancy.GreaterThan(r.balanceDiscrepancyThreshold) {
                // Log warning, emit reconciliation event
                r.hub.emitReconciliationEvent(ReconciliationEvent{
                    Type:       ReconciliationTypeBalance,
                    LocalValue: localBalance.TotalBalance,
                    TrueValue:  onChainBalance,
                    Discrepancy: discrepancy,
                })

                // Optionally: force sync from REST API
                r.forceRESTSync(ctx)
            }

        case <-ctx.Done():
            return
        }
    }
}
```

## Internal Event Emission (Local Actions)

When bot places an order locally, state updates immediately without waiting for WebSocket confirmation:

```go
// Client method with internal event emission
func (c *Client) PlaceOrder(ctx context.Context, input OrderInput) (*Order, error) {
    // 1. Submit to exchange
    order, err := c.submitOrder(ctx, input)
    if err != nil {
        return nil, err
    }

    // 2. Immediately update local state (optimistic update)
    if c.streamHub != nil {
        c.streamHub.state.OnOrderCreated(order)
        c.streamHub.emitOrderUpdate(OrderUpdate{
            OrderID: order.ID,
            Status:  OrderStatusOpen,
            Source:  UpdateSourceLocal,
        })
        c.streamHub.emitBalanceUpdate(c.streamHub.state.GetCollateralBalance())
    }

    return order, nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
    // 1. Submit cancellation
    err := c.submitCancelOrder(ctx, orderID)
    if err != nil {
        return err
    }

    // 2. Immediately update local state
    if c.streamHub != nil {
        c.streamHub.state.OnOrderCancelled(orderID)
        c.streamHub.emitOrderUpdate(OrderUpdate{
            OrderID: orderID,
            Status:  OrderStatusCancelled,
            Source:  UpdateSourceLocal,
        })
        c.streamHub.emitBalanceUpdate(c.streamHub.state.GetCollateralBalance())
    }

    return nil
}
```

## Usage Example

```go
// Initialize StreamHub
hub := polymarket.NewStreamHub(client, polymarket.StreamHubConfig{
    RESTSyncInterval:    5 * time.Second,
    OnChainSyncInterval: 30 * time.Second,
})

// Subscribe to events
hub.OnBalanceUpdate(func(balance *polymarket.BalanceDetail) {
    // UI update, risk check, etc.
    log.Printf("Balance: Total=%s, Locked=%s, Available=%s",
        balance.TotalBalance, balance.LockedBalance, balance.AvailableBalance)
})

hub.OnOrderBookUpdate(func(update polymarket.OrderBookUpdate) {
    // Sniper logic, spread monitoring, etc.
})

// Start streaming
hub.Start(ctx)

// Query local state (< 1us)
balance := hub.GetCollateralBalance()
position := hub.GetPositionBalance(tokenID)
orderBook := hub.GetOrderBook(tokenID)
orders := hub.GetOpenOrders()

// Place order - immediate local state update
order, err := client.PlaceOrder(ctx, input)
// balance already updated in hub.state
```

---

## Reliability & Fault Tolerance

### WebSocket Connection Management

**Note**: WebSocket reconnection is handled by the underlying `realtime` client library. StreamHub only needs to:
1. Listen for reconnection events from the underlying library
2. Trigger state resync after reconnection
3. Handle potential message gaps during reconnection

```go
type ConnectionListener struct {
    hub             *StreamHub
    lastSequenceNum uint64
}

// Subscribe to underlying realtime client events
func (l *ConnectionListener) Setup(realtimeClient *realtime.Client) {
    // Listen for reconnection event from underlying library
    realtimeClient.OnReconnect(func() {
        l.hub.emitConnectionEvent(ConnectionEventReconnected, nil)

        // CRITICAL: Request full state sync after reconnect
        // During disconnect, we may have missed messages
        l.hub.forceRESTSync()

        // Reset sequence tracking
        l.lastSequenceNum = 0

        emitMetric("websocket.reconnect", 1)
    })

    realtimeClient.OnDisconnect(func(err error) {
        l.hub.emitConnectionEvent(ConnectionEventDisconnected, err)
        emitMetric("websocket.disconnect", 1)
    })
}
```

### Message Sequence Validation

```go
type SequenceValidator struct {
    expectedSeq uint64
    gapBuffer   map[uint64][]byte  // Buffer out-of-order messages
    maxGapSize  int                // Max messages to buffer (e.g., 1000)
    gapTimeout  time.Duration      // Max time to wait for gap fill (e.g., 5s)
}

func (sv *SequenceValidator) Process(seq uint64, msg []byte) ([][]byte, error) {
    if seq == sv.expectedSeq {
        // In-order message
        sv.expectedSeq++

        // Check if buffered messages can now be processed
        var ordered [][]byte
        ordered = append(ordered, msg)

        for {
            if buffered, ok := sv.gapBuffer[sv.expectedSeq]; ok {
                ordered = append(ordered, buffered)
                delete(sv.gapBuffer, sv.expectedSeq)
                sv.expectedSeq++
            } else {
                break
            }
        }
        return ordered, nil

    } else if seq > sv.expectedSeq {
        // Gap detected - buffer this message
        if len(sv.gapBuffer) >= sv.maxGapSize {
            // Gap too large - force resync
            return nil, ErrGapTooLarge
        }
        sv.gapBuffer[seq] = msg

        // Emit gap warning for monitoring
        emitMetric("websocket.sequence_gap", float64(seq-sv.expectedSeq))
        return nil, nil

    } else {
        // Duplicate or old message - ignore
        emitMetric("websocket.duplicate_message", 1)
        return nil, nil
    }
}
```

### Optimistic Update Rollback

```go
type OptimisticUpdate struct {
    ID           string
    Timestamp    time.Time
    StateChanges []StateChange
    Confirmed    bool
    RolledBack   bool
}

type OptimisticUpdateManager struct {
    mu            sync.Mutex
    pendingUpdates map[string]*OptimisticUpdate
    confirmTimeout time.Duration  // e.g., 5s
}

// Apply optimistic update (e.g., after placing order)
func (m *OptimisticUpdateManager) Apply(id string, changes []StateChange) {
    m.mu.Lock()
    defer m.mu.Unlock()

    update := &OptimisticUpdate{
        ID:           id,
        Timestamp:    time.Now(),
        StateChanges: changes,
    }
    m.pendingUpdates[id] = update

    // Schedule rollback check
    go m.scheduleRollbackCheck(id)
}

// Confirm when WebSocket confirms the action
func (m *OptimisticUpdateManager) Confirm(id string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if update, ok := m.pendingUpdates[id]; ok {
        update.Confirmed = true
        delete(m.pendingUpdates, id)
    }
}

// Rollback if not confirmed within timeout
func (m *OptimisticUpdateManager) scheduleRollbackCheck(id string) {
    time.Sleep(m.confirmTimeout)

    m.mu.Lock()
    defer m.mu.Unlock()

    if update, ok := m.pendingUpdates[id]; ok && !update.Confirmed {
        // CRITICAL: Rollback the optimistic changes
        for _, change := range update.StateChanges {
            change.Rollback()
        }
        update.RolledBack = true
        delete(m.pendingUpdates, id)

        // Emit alert - this is a serious issue
        emitAlert(AlertLevelWarning, "optimistic_update_timeout", map[string]string{
            "update_id": id,
            "age_ms":    fmt.Sprintf("%d", time.Since(update.Timestamp).Milliseconds()),
        })

        // Force REST sync to get true state
        m.forceRESTSync()
    }
}
```

---

## Correctness Guarantees

### State Invariants (Must Always Hold)

```go
// Invariant checks - run periodically and after every state mutation
func (s *StateStore) ValidateInvariants() error {
    s.mu.RLock()
    defer s.mu.RUnlock()

    var errs []error

    // Invariant 1: Available balance >= 0
    available := s.collateralTotal.Sub(s.collateralLocked)
    if available.LessThan(decimal.Zero) {
        errs = append(errs, fmt.Errorf("CRITICAL: negative available balance: %s", available))
    }

    // Invariant 2: Locked <= Total
    if s.collateralLocked.GreaterThan(s.collateralTotal) {
        errs = append(errs, fmt.Errorf("CRITICAL: locked > total: %s > %s",
            s.collateralLocked, s.collateralTotal))
    }

    // Invariant 3: Sum of order locks == collateralLocked
    calculatedLocked := decimal.Zero
    for _, order := range s.openOrders {
        if order.Side == OrderSideBuy {
            calculatedLocked = calculatedLocked.Add(order.RemainingSize().Mul(order.Price))
        }
    }
    if !calculatedLocked.Equal(s.collateralLocked) {
        errs = append(errs, fmt.Errorf("CRITICAL: locked mismatch: calculated=%s, stored=%s",
            calculatedLocked, s.collateralLocked))
    }

    // Invariant 4: Position locked <= Position total (for each position)
    for tokenID, pos := range s.positions {
        if pos.Locked.GreaterThan(pos.Total) {
            errs = append(errs, fmt.Errorf("CRITICAL: position %s locked > total: %s > %s",
                tokenID, pos.Locked, pos.Total))
        }
    }

    if len(errs) > 0 {
        // HALT TRADING - invariant violation is critical
        s.haltTrading()
        return fmt.Errorf("invariant violations: %v", errs)
    }

    return nil
}
```

### Idempotent Updates

```go
type UpdateDeduplicator struct {
    seen     map[string]time.Time  // updateID -> timestamp
    maxAge   time.Duration         // e.g., 5 minutes
    mu       sync.RWMutex
}

func (d *UpdateDeduplicator) IsDuplicate(updateID string) bool {
    d.mu.RLock()
    _, exists := d.seen[updateID]
    d.mu.RUnlock()
    return exists
}

func (d *UpdateDeduplicator) MarkSeen(updateID string) {
    d.mu.Lock()
    d.seen[updateID] = time.Now()
    d.mu.Unlock()
}

// Periodic cleanup of old entries
func (d *UpdateDeduplicator) cleanup() {
    d.mu.Lock()
    defer d.mu.Unlock()

    cutoff := time.Now().Add(-d.maxAge)
    for id, ts := range d.seen {
        if ts.Before(cutoff) {
            delete(d.seen, id)
        }
    }
}
```

---

## Observability & Monitoring

### Metrics (Prometheus-style)

```go
type Metrics struct {
    // Latency histograms
    QueryLatency      *prometheus.HistogramVec   // labels: operation
    UpdateLatency     *prometheus.HistogramVec   // labels: source, type
    EventLatency      *prometheus.HistogramVec   // labels: event_type

    // Counters
    MessagesReceived  *prometheus.CounterVec     // labels: source, type
    MessagesDropped   *prometheus.CounterVec     // labels: reason
    UpdatesApplied    *prometheus.CounterVec     // labels: type
    InvariantChecks   *prometheus.CounterVec     // labels: result

    // Gauges
    OpenOrdersCount   prometheus.Gauge
    PositionsCount    prometheus.Gauge
    CollateralTotal   prometheus.Gauge
    CollateralLocked  prometheus.Gauge
    CollateralAvail   prometheus.Gauge
    WebSocketState    prometheus.Gauge           // 0=disconnected, 1=connecting, 2=connected
    SequenceGap       prometheus.Gauge

    // State health
    LastUpdateAge     prometheus.Gauge           // Seconds since last update
    ReconcileDiscrepancy prometheus.Gauge        // Latest discrepancy amount
}

// Instrumented state update
func (s *StateStore) OnOrderCreatedInstrumented(order *Order) {
    start := time.Now()

    s.OnOrderCreated(order)

    metrics.UpdateLatency.WithLabelValues("local", "order_created").Observe(
        float64(time.Since(start).Microseconds()) / 1000.0,
    )
    metrics.UpdatesApplied.WithLabelValues("order_created").Inc()
    metrics.OpenOrdersCount.Set(float64(len(s.openOrders)))
}
```

### Health Checks

```go
type HealthChecker struct {
    hub *StreamHub
}

type HealthStatus struct {
    Healthy     bool
    Components  map[string]ComponentHealth
    LastCheck   time.Time
}

type ComponentHealth struct {
    Healthy bool
    Message string
    Metrics map[string]interface{}
}

func (h *HealthChecker) Check() HealthStatus {
    status := HealthStatus{
        Healthy:    true,
        Components: make(map[string]ComponentHealth),
        LastCheck:  time.Now(),
    }

    // Check WebSocket connection
    wsHealth := h.checkWebSocket()
    status.Components["websocket"] = wsHealth
    if !wsHealth.Healthy {
        status.Healthy = false
    }

    // Check state freshness
    stateHealth := h.checkStateFreshness()
    status.Components["state"] = stateHealth
    if !stateHealth.Healthy {
        status.Healthy = false
    }

    // Check invariants
    invariantHealth := h.checkInvariants()
    status.Components["invariants"] = invariantHealth
    if !invariantHealth.Healthy {
        status.Healthy = false
    }

    // Check reconciliation status
    reconHealth := h.checkReconciliation()
    status.Components["reconciliation"] = reconHealth
    if !reconHealth.Healthy {
        status.Healthy = false
    }

    return status
}

func (h *HealthChecker) checkStateFreshness() ComponentHealth {
    age := time.Since(h.hub.state.lastUpdateTime)

    if age > 30*time.Second {
        return ComponentHealth{
            Healthy: false,
            Message: fmt.Sprintf("State is stale: %v since last update", age),
            Metrics: map[string]interface{}{"age_seconds": age.Seconds()},
        }
    }

    return ComponentHealth{
        Healthy: true,
        Message: "State is fresh",
        Metrics: map[string]interface{}{"age_seconds": age.Seconds()},
    }
}
```

### Alerting

```go
type AlertLevel int

const (
    AlertLevelInfo AlertLevel = iota
    AlertLevelWarning
    AlertLevelCritical
    AlertLevelFatal  // Should halt trading
)

type Alert struct {
    Level     AlertLevel
    Type      string
    Message   string
    Metadata  map[string]string
    Timestamp time.Time
}

type Alerter struct {
    handlers []func(Alert)
}

func (a *Alerter) Emit(level AlertLevel, alertType string, metadata map[string]string) {
    alert := Alert{
        Level:     level,
        Type:      alertType,
        Metadata:  metadata,
        Timestamp: time.Now(),
    }

    for _, handler := range a.handlers {
        handler(alert)
    }

    // Critical/Fatal alerts should also log prominently
    if level >= AlertLevelCritical {
        log.Printf("CRITICAL ALERT: %s - %v", alertType, metadata)
    }
}

// Pre-defined alert types
const (
    AlertTypeInvariantViolation    = "invariant_violation"
    AlertTypeReconcileDiscrepancy  = "reconcile_discrepancy"
    AlertTypeConnectionLost        = "connection_lost"
    AlertTypeSequenceGap           = "sequence_gap"
    AlertTypeOptimisticTimeout     = "optimistic_timeout"
    AlertTypeHighLatency           = "high_latency"
    AlertTypeBalanceInsufficient   = "balance_insufficient"
)
```

---

## Risk Management

### Pre-Trade Validation

```go
type RiskManager struct {
    hub *StreamHub

    // Configurable limits
    maxOrderSize        decimal.Decimal
    maxPositionSize     decimal.Decimal
    minAvailableBalance decimal.Decimal  // Reserve buffer
    maxOpenOrders       int
    maxDailyVolume      decimal.Decimal

    // Tracking
    dailyVolume         decimal.Decimal
    dailyVolumeResetAt  time.Time
}

func (rm *RiskManager) ValidateOrder(order OrderInput) error {
    balance := rm.hub.state.GetCollateralBalance()

    // Check 1: Sufficient available balance (with buffer)
    requiredAmount := order.Size.Mul(order.Price)
    minRequired := requiredAmount.Add(rm.minAvailableBalance)

    if balance.AvailableBalance.LessThan(minRequired) {
        return fmt.Errorf("insufficient balance: available=%s, required=%s (including %s buffer)",
            balance.AvailableBalance, minRequired, rm.minAvailableBalance)
    }

    // Check 2: Order size limit
    if order.Size.GreaterThan(rm.maxOrderSize) {
        return fmt.Errorf("order size %s exceeds max %s", order.Size, rm.maxOrderSize)
    }

    // Check 3: Position size limit (post-trade)
    position := rm.hub.state.GetPositionBalance(order.TokenID)
    postTradePosition := position.TotalBalance.Add(order.Size)
    if postTradePosition.GreaterThan(rm.maxPositionSize) {
        return fmt.Errorf("post-trade position %s would exceed max %s",
            postTradePosition, rm.maxPositionSize)
    }

    // Check 4: Open orders limit
    openOrders := rm.hub.state.GetOpenOrders()
    if len(openOrders) >= rm.maxOpenOrders {
        return fmt.Errorf("open orders count %d at limit %d", len(openOrders), rm.maxOpenOrders)
    }

    // Check 5: Daily volume limit
    rm.resetDailyVolumeIfNeeded()
    postTradeVolume := rm.dailyVolume.Add(requiredAmount)
    if postTradeVolume.GreaterThan(rm.maxDailyVolume) {
        return fmt.Errorf("daily volume %s would exceed max %s", postTradeVolume, rm.maxDailyVolume)
    }

    return nil
}

// Circuit breaker - halt trading on critical issues
func (rm *RiskManager) HaltTrading(reason string) {
    rm.hub.state.tradingHalted = true
    rm.hub.emitAlert(AlertLevelFatal, "trading_halted", map[string]string{
        "reason": reason,
    })
}
```

---

## Performance Optimization (Deep Dive)

### Memory Layout & Cache Optimization

```go
// Bad: Fields scattered, poor cache locality
type OrderBad struct {
    ID          string           // 16 bytes
    Active      bool             // 1 byte + 7 padding
    Price       decimal.Decimal  // 40+ bytes
    TokenID     string           // 16 bytes
    Side        OrderSide        // 1 byte + 7 padding
    Size        decimal.Decimal  // 40+ bytes
}

// Good: Hot fields together, cold fields at end
type OrderGood struct {
    // Hot path fields (frequently accessed together) - 64 bytes = 1 cache line
    Price       decimal.Decimal  // 40 bytes
    Size        decimal.Decimal  // 40 bytes (next cache line)
    SizeMatched decimal.Decimal  // 40 bytes
    Side        OrderSide        // 1 byte
    Active      bool             // 1 byte
    _padding    [6]byte          // Explicit padding

    // Cold fields (accessed less frequently)
    ID          string
    TokenID     string
    CreatedAt   time.Time
}
```

### Lock-Free Order Book (For Extreme Performance)

```go
import "sync/atomic"

type LockFreeOrderBook struct {
    // Atomic pointer swap for updates
    current atomic.Pointer[OrderBookSnapshot]
}

type OrderBookSnapshot struct {
    Bids      []PriceLevel
    Asks      []PriceLevel
    BestBid   decimal.Decimal
    BestAsk   decimal.Decimal
    Spread    decimal.Decimal
    Timestamp time.Time
    Sequence  uint64
}

// Read: No locks, just atomic load
func (ob *LockFreeOrderBook) Get() *OrderBookSnapshot {
    return ob.current.Load()
}

// Write: Create new snapshot, atomic swap
func (ob *LockFreeOrderBook) Update(delta OrderBookDelta) {
    for {
        old := ob.current.Load()

        // Create new snapshot with delta applied
        new := applyDelta(old, delta)

        // Atomic compare-and-swap
        if ob.current.CompareAndSwap(old, new) {
            return
        }
        // Retry if concurrent update
    }
}
```

### Object Pool for Reduced GC Pressure

```go
var orderPool = sync.Pool{
    New: func() interface{} {
        return &Order{}
    },
}

var orderBookDeltaPool = sync.Pool{
    New: func() interface{} {
        return &OrderBookDelta{
            Bids: make([]PriceLevel, 0, 100),
            Asks: make([]PriceLevel, 0, 100),
        }
    },
}

func acquireOrder() *Order {
    return orderPool.Get().(*Order)
}

func releaseOrder(o *Order) {
    // Reset fields
    *o = Order{}
    orderPool.Put(o)
}
```

### Batch Processing for High-Frequency Updates

```go
type BatchProcessor struct {
    buffer    chan Update
    batchSize int
    interval  time.Duration
    handler   func([]Update)
}

func (bp *BatchProcessor) Start(ctx context.Context) {
    batch := make([]Update, 0, bp.batchSize)
    ticker := time.NewTicker(bp.interval)

    for {
        select {
        case update := <-bp.buffer:
            batch = append(batch, update)
            if len(batch) >= bp.batchSize {
                bp.handler(batch)
                batch = batch[:0]
            }

        case <-ticker.C:
            if len(batch) > 0 {
                bp.handler(batch)
                batch = batch[:0]
            }

        case <-ctx.Done():
            return
        }
    }
}
```

---

## Testing Strategy

### 1. Unit Tests - StateStore

```go
// state_store_test.go

func TestStateStore_OrderCreated_LocksCollateral(t *testing.T) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(1000))

    order := &Order{
        ID:       "order-1",
        Side:     OrderSideBuy,
        Price:    decimal.NewFromFloat(0.5),
        Size:     decimal.NewFromFloat(100),
        TokenID:  "token-123",
    }

    store.OnOrderCreated(order)

    balance := store.GetCollateralBalance()

    // 100 * 0.5 = 50 USDC locked
    assert.Equal(t, decimal.NewFromFloat(50), balance.LockedBalance)
    assert.Equal(t, decimal.NewFromFloat(950), balance.AvailableBalance)
}

func TestStateStore_OrderCancelled_UnlocksCollateral(t *testing.T) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(1000))

    order := &Order{
        ID:       "order-1",
        Side:     OrderSideBuy,
        Price:    decimal.NewFromFloat(0.5),
        Size:     decimal.NewFromFloat(100),
        TokenID:  "token-123",
    }

    store.OnOrderCreated(order)
    store.OnOrderCancelled("order-1")

    balance := store.GetCollateralBalance()

    assert.Equal(t, decimal.Zero, balance.LockedBalance)
    assert.Equal(t, decimal.NewFromFloat(1000), balance.AvailableBalance)
}

func TestStateStore_PartialFill_AdjustsLocked(t *testing.T) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(1000))

    order := &Order{
        ID:          "order-1",
        Side:        OrderSideBuy,
        Price:       decimal.NewFromFloat(0.5),
        Size:        decimal.NewFromFloat(100),
        SizeMatched: decimal.Zero,
        TokenID:     "token-123",
    }

    store.OnOrderCreated(order)

    // Partial fill: 40 out of 100
    store.OnOrderUpdate(OrderUpdate{
        OrderID:     "order-1",
        Status:      OrderStatusPartiallyFilled,
        SizeMatched: decimal.NewFromFloat(40),
    })

    balance := store.GetCollateralBalance()

    // Remaining: (100 - 40) * 0.5 = 30 USDC locked
    assert.Equal(t, decimal.NewFromFloat(30), balance.LockedBalance)
}

func TestStateStore_SellOrder_LocksPosition(t *testing.T) {
    store := NewStateStore()
    store.SetPositionTotal("token-123", decimal.NewFromFloat(500))

    order := &Order{
        ID:      "order-1",
        Side:    OrderSideSell,
        Price:   decimal.NewFromFloat(0.6),
        Size:    decimal.NewFromFloat(200),
        TokenID: "token-123",
    }

    store.OnOrderCreated(order)

    position := store.GetPositionBalance("token-123")

    assert.Equal(t, decimal.NewFromFloat(200), position.LockedBalance)
    assert.Equal(t, decimal.NewFromFloat(300), position.AvailableBalance)
}

func TestStateStore_InvariantViolation_NegativeAvailable(t *testing.T) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(100))

    // Force invalid state (should not happen in production)
    store.collateralLocked = decimal.NewFromFloat(150)

    err := store.ValidateInvariants()

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "negative available balance")
    assert.True(t, store.tradingHalted)
}

func TestStateStore_InvariantViolation_LockedMismatch(t *testing.T) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(1000))

    order := &Order{
        ID:    "order-1",
        Side:  OrderSideBuy,
        Price: decimal.NewFromFloat(0.5),
        Size:  decimal.NewFromFloat(100),
    }
    store.openOrders["order-1"] = order

    // Manually set wrong locked amount
    store.collateralLocked = decimal.NewFromFloat(100) // Should be 50

    err := store.ValidateInvariants()

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "locked mismatch")
}

func TestStateStore_ConcurrentReads(t *testing.T) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(1000))

    var wg sync.WaitGroup
    errors := make(chan error, 100)

    // 100 concurrent readers
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                balance := store.GetCollateralBalance()
                if balance.TotalBalance.LessThan(decimal.Zero) {
                    errors <- fmt.Errorf("invalid balance: %s", balance.TotalBalance)
                }
            }
        }()
    }

    wg.Wait()
    close(errors)

    for err := range errors {
        t.Error(err)
    }
}
```

### 2. Unit Tests - Sequence Validator

```go
// sequence_validator_test.go

func TestSequenceValidator_InOrder(t *testing.T) {
    sv := NewSequenceValidator(1000, 5*time.Second)

    msgs, err := sv.Process(0, []byte("msg0"))
    assert.NoError(t, err)
    assert.Len(t, msgs, 1)

    msgs, err = sv.Process(1, []byte("msg1"))
    assert.NoError(t, err)
    assert.Len(t, msgs, 1)

    msgs, err = sv.Process(2, []byte("msg2"))
    assert.NoError(t, err)
    assert.Len(t, msgs, 1)
}

func TestSequenceValidator_OutOfOrder_Buffered(t *testing.T) {
    sv := NewSequenceValidator(1000, 5*time.Second)

    // Receive seq 0
    msgs, _ := sv.Process(0, []byte("msg0"))
    assert.Len(t, msgs, 1)

    // Receive seq 2 (gap - seq 1 missing)
    msgs, _ = sv.Process(2, []byte("msg2"))
    assert.Len(t, msgs, 0) // Buffered, not returned

    // Receive seq 1 (fills gap)
    msgs, _ = sv.Process(1, []byte("msg1"))
    assert.Len(t, msgs, 2) // msg1 + buffered msg2
    assert.Equal(t, []byte("msg1"), msgs[0])
    assert.Equal(t, []byte("msg2"), msgs[1])
}

func TestSequenceValidator_Duplicate_Ignored(t *testing.T) {
    sv := NewSequenceValidator(1000, 5*time.Second)

    sv.Process(0, []byte("msg0"))
    sv.Process(1, []byte("msg1"))

    // Duplicate seq 0
    msgs, err := sv.Process(0, []byte("msg0-dup"))
    assert.NoError(t, err)
    assert.Len(t, msgs, 0) // Ignored
}

func TestSequenceValidator_GapTooLarge(t *testing.T) {
    sv := NewSequenceValidator(10, 5*time.Second) // Max gap size: 10

    sv.Process(0, []byte("msg0"))

    // Create gap of 15 messages
    for i := 2; i <= 12; i++ {
        sv.Process(uint64(i), []byte(fmt.Sprintf("msg%d", i)))
    }

    _, err := sv.Process(13, []byte("msg13"))
    assert.ErrorIs(t, err, ErrGapTooLarge)
}
```

### 3. Unit Tests - Optimistic Update Manager

```go
// optimistic_update_test.go

func TestOptimisticUpdate_Confirmed(t *testing.T) {
    manager := NewOptimisticUpdateManager(100 * time.Millisecond)
    state := NewStateStore()
    state.SetCollateralTotal(decimal.NewFromFloat(1000))

    // Apply optimistic update
    changes := []StateChange{
        &CollateralLockChange{Amount: decimal.NewFromFloat(50)},
    }
    manager.Apply("order-1", changes)

    // Confirm before timeout
    time.Sleep(50 * time.Millisecond)
    manager.Confirm("order-1")

    // Wait past timeout
    time.Sleep(100 * time.Millisecond)

    // Should NOT be rolled back
    assert.False(t, manager.WasRolledBack("order-1"))
}

func TestOptimisticUpdate_Timeout_RolledBack(t *testing.T) {
    manager := NewOptimisticUpdateManager(50 * time.Millisecond)

    rollbackCalled := false
    changes := []StateChange{
        &MockStateChange{
            rollbackFn: func() { rollbackCalled = true },
        },
    }

    manager.Apply("order-1", changes)

    // Wait for timeout
    time.Sleep(100 * time.Millisecond)

    assert.True(t, rollbackCalled)
    assert.True(t, manager.WasRolledBack("order-1"))
}

func TestOptimisticUpdate_MultipleUpdates(t *testing.T) {
    manager := NewOptimisticUpdateManager(100 * time.Millisecond)

    manager.Apply("order-1", []StateChange{})
    manager.Apply("order-2", []StateChange{})
    manager.Apply("order-3", []StateChange{})

    manager.Confirm("order-1")
    manager.Confirm("order-3")

    time.Sleep(150 * time.Millisecond)

    assert.False(t, manager.WasRolledBack("order-1"))
    assert.True(t, manager.WasRolledBack("order-2"))  // Not confirmed
    assert.False(t, manager.WasRolledBack("order-3"))
}
```

### 4. Unit Tests - Risk Manager

```go
// risk_manager_test.go

func TestRiskManager_InsufficientBalance(t *testing.T) {
    hub := NewMockStreamHub()
    hub.state.SetCollateralTotal(decimal.NewFromFloat(100))

    rm := NewRiskManager(hub, RiskConfig{
        MinAvailableBuffer: decimal.NewFromFloat(10),
    })

    order := OrderInput{
        Size:    decimal.NewFromFloat(200),
        Price:   decimal.NewFromFloat(0.5),
        TokenID: "token-123",
    }

    err := rm.ValidateOrder(order)

    assert.ErrorContains(t, err, "insufficient balance")
}

func TestRiskManager_ExceedsMaxOrderSize(t *testing.T) {
    hub := NewMockStreamHub()
    hub.state.SetCollateralTotal(decimal.NewFromFloat(10000))

    rm := NewRiskManager(hub, RiskConfig{
        MaxOrderSize: decimal.NewFromFloat(100),
    })

    order := OrderInput{
        Size:  decimal.NewFromFloat(150),
        Price: decimal.NewFromFloat(0.5),
    }

    err := rm.ValidateOrder(order)

    assert.ErrorContains(t, err, "exceeds max")
}

func TestRiskManager_ExceedsPositionLimit(t *testing.T) {
    hub := NewMockStreamHub()
    hub.state.SetCollateralTotal(decimal.NewFromFloat(10000))
    hub.state.SetPositionTotal("token-123", decimal.NewFromFloat(800))

    rm := NewRiskManager(hub, RiskConfig{
        MaxPositionSize: decimal.NewFromFloat(1000),
    })

    order := OrderInput{
        Size:    decimal.NewFromFloat(300), // Would make total 1100
        Price:   decimal.NewFromFloat(0.5),
        TokenID: "token-123",
    }

    err := rm.ValidateOrder(order)

    assert.ErrorContains(t, err, "would exceed max")
}

func TestRiskManager_ReserveBalance_Concurrent(t *testing.T) {
    hub := NewMockStreamHub()
    hub.state.SetCollateralTotal(decimal.NewFromFloat(100))

    rm := NewRiskManager(hub, RiskConfig{})

    var successCount int32
    var wg sync.WaitGroup

    // 10 goroutines trying to reserve 20 each (only 5 should succeed)
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            release, err := rm.ReserveBalance(decimal.NewFromFloat(20))
            if err == nil {
                atomic.AddInt32(&successCount, 1)
                time.Sleep(10 * time.Millisecond)
                release()
            }
        }()
    }

    wg.Wait()

    assert.Equal(t, int32(5), successCount)
}
```

### 5. Integration Tests

```go
// integration_test.go

func TestIntegration_WebSocketReconnect(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    mockServer := NewMockWebSocketServer()
    defer mockServer.Close()

    hub := NewStreamHub(StreamHubConfig{
        WebSocketURL:       mockServer.URL,
        ReconnectBaseDelay: 10 * time.Millisecond,
        ReconnectMaxDelay:  100 * time.Millisecond,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    hub.Start(ctx)

    // Wait for connection
    assert.Eventually(t, func() bool {
        return hub.ConnectionState() == ConnectionStateConnected
    }, 1*time.Second, 10*time.Millisecond)

    // Simulate disconnect
    mockServer.DisconnectAll()

    // Should reconnect
    assert.Eventually(t, func() bool {
        return hub.ConnectionState() == ConnectionStateConnected
    }, 2*time.Second, 10*time.Millisecond)

    // Verify state was resynced
    assert.True(t, hub.LastSyncTime().After(time.Now().Add(-1*time.Second)))
}

func TestIntegration_FullOrderLifecycle(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    mockServer := NewMockWebSocketServer()
    mockAPI := NewMockRESTAPI()
    defer mockServer.Close()
    defer mockAPI.Close()

    hub := NewStreamHub(StreamHubConfig{
        WebSocketURL: mockServer.URL,
        RESTBaseURL:  mockAPI.URL,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    hub.Start(ctx)
    hub.state.SetCollateralTotal(decimal.NewFromFloat(1000))

    // Track events
    var events []string
    var mu sync.Mutex

    hub.OnBalanceUpdate(func(b *BalanceDetail) {
        mu.Lock()
        events = append(events, fmt.Sprintf("balance:%s", b.AvailableBalance))
        mu.Unlock()
    })

    hub.OnOrderUpdate(func(u OrderUpdate) {
        mu.Lock()
        events = append(events, fmt.Sprintf("order:%s:%s", u.OrderID, u.Status))
        mu.Unlock()
    })

    // Place order
    order := &Order{
        ID:    "order-1",
        Side:  OrderSideBuy,
        Price: decimal.NewFromFloat(0.5),
        Size:  decimal.NewFromFloat(100),
    }
    hub.state.OnOrderCreated(order)

    // Simulate WS confirmation
    mockServer.SendMessage(OrderUpdateMessage{
        OrderID: "order-1",
        Status:  "LIVE",
    })

    time.Sleep(50 * time.Millisecond)

    // Simulate partial fill
    mockServer.SendMessage(OrderUpdateMessage{
        OrderID:     "order-1",
        Status:      "MATCHED",
        SizeMatched: "40",
    })

    time.Sleep(50 * time.Millisecond)

    // Simulate full fill
    mockServer.SendMessage(OrderUpdateMessage{
        OrderID:     "order-1",
        Status:      "MATCHED",
        SizeMatched: "100",
    })

    time.Sleep(100 * time.Millisecond)

    // Verify final state
    balance := hub.state.GetCollateralBalance()
    assert.Equal(t, decimal.Zero, balance.LockedBalance)

    // Verify event sequence
    mu.Lock()
    assert.Contains(t, events, "order:order-1:LIVE")
    mu.Unlock()
}

func TestIntegration_OnChainReconciliation(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    mockRPC := NewMockRPCServer()
    defer mockRPC.Close()

    hub := NewStreamHub(StreamHubConfig{
        RPCEndpoint:         mockRPC.URL,
        OnChainSyncInterval: 100 * time.Millisecond,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Set local state with wrong value
    hub.state.SetCollateralTotal(decimal.NewFromFloat(1000))

    // Mock on-chain balance is different
    mockRPC.SetBalance(decimal.NewFromFloat(1200))

    var reconcileEvents []ReconciliationEvent
    hub.OnReconciliation(func(e ReconciliationEvent) {
        reconcileEvents = append(reconcileEvents, e)
    })

    hub.Start(ctx)

    // Wait for reconciliation
    time.Sleep(300 * time.Millisecond)

    assert.NotEmpty(t, reconcileEvents)
    assert.Equal(t, decimal.NewFromFloat(200), reconcileEvents[0].Discrepancy)
}
```

### 6. Chaos Tests

```go
// chaos_test.go

func TestChaos_RandomDisconnects(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping chaos test")
    }

    mockServer := NewMockWebSocketServer()
    defer mockServer.Close()

    hub := NewStreamHub(StreamHubConfig{
        WebSocketURL:       mockServer.URL,
        ReconnectBaseDelay: 5 * time.Millisecond,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    hub.Start(ctx)
    hub.state.SetCollateralTotal(decimal.NewFromFloat(1000))

    // Random order activity
    go func() {
        for i := 0; i < 100; i++ {
            order := &Order{
                ID:    fmt.Sprintf("order-%d", i),
                Side:  OrderSideBuy,
                Price: decimal.NewFromFloat(0.5),
                Size:  decimal.NewFromFloat(float64(rand.Intn(10) + 1)),
            }
            hub.state.OnOrderCreated(order)
            time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

            if rand.Float32() > 0.3 {
                hub.state.OnOrderCancelled(order.ID)
            }
        }
    }()

    // Random disconnects
    go func() {
        for i := 0; i < 10; i++ {
            time.Sleep(time.Duration(rand.Intn(500)+100) * time.Millisecond)
            mockServer.DisconnectAll()
        }
    }()

    time.Sleep(10 * time.Second)

    // Verify invariants still hold
    err := hub.state.ValidateInvariants()
    assert.NoError(t, err)
}

func TestChaos_MessageReordering(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping chaos test")
    }

    hub := NewStreamHub(StreamHubConfig{})
    hub.state.SetCollateralTotal(decimal.NewFromFloat(10000))

    // Generate ordered messages
    messages := make([]OrderUpdate, 100)
    for i := 0; i < 100; i++ {
        messages[i] = OrderUpdate{
            Sequence: uint64(i),
            OrderID:  fmt.Sprintf("order-%d", i%10),
            Status:   OrderStatusOpen,
        }
    }

    // Shuffle messages
    rand.Shuffle(len(messages), func(i, j int) {
        messages[i], messages[j] = messages[j], messages[i]
    })

    // Process in random order
    for _, msg := range messages {
        hub.ProcessOrderUpdate(msg)
    }

    // Wait for processing
    time.Sleep(100 * time.Millisecond)

    // All messages should eventually be processed
    assert.Equal(t, uint64(100), hub.state.updateSequence)
}

func TestChaos_HighLoadStress(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping stress test")
    }

    hub := NewStreamHub(StreamHubConfig{})
    hub.state.SetCollateralTotal(decimal.NewFromFloat(1000000))

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    hub.Start(ctx)

    var ops int64
    var errors int64

    // Concurrent writers
    for i := 0; i < 10; i++ {
        go func(workerID int) {
            for j := 0; j < 10000; j++ {
                order := &Order{
                    ID:    fmt.Sprintf("order-%d-%d", workerID, j),
                    Side:  OrderSideBuy,
                    Price: decimal.NewFromFloat(0.5),
                    Size:  decimal.NewFromFloat(1),
                }
                hub.state.OnOrderCreated(order)
                atomic.AddInt64(&ops, 1)

                hub.state.OnOrderCancelled(order.ID)
                atomic.AddInt64(&ops, 1)
            }
        }(i)
    }

    // Concurrent readers
    for i := 0; i < 20; i++ {
        go func() {
            for j := 0; j < 50000; j++ {
                balance := hub.state.GetCollateralBalance()
                if balance.AvailableBalance.LessThan(decimal.Zero) {
                    atomic.AddInt64(&errors, 1)
                }
                atomic.AddInt64(&ops, 1)
            }
        }()
    }

    time.Sleep(10 * time.Second)

    t.Logf("Operations completed: %d", atomic.LoadInt64(&ops))
    t.Logf("Errors: %d", atomic.LoadInt64(&errors))

    assert.Zero(t, atomic.LoadInt64(&errors))
    assert.NoError(t, hub.state.ValidateInvariants())
}
```

### 7. Performance Benchmarks

```go
// benchmark_test.go

func BenchmarkStateStore_GetCollateralBalance(b *testing.B) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(1000))

    // Add some orders to make it realistic
    for i := 0; i < 100; i++ {
        store.OnOrderCreated(&Order{
            ID:    fmt.Sprintf("order-%d", i),
            Side:  OrderSideBuy,
            Price: decimal.NewFromFloat(0.5),
            Size:  decimal.NewFromFloat(10),
        })
    }

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _ = store.GetCollateralBalance()
        }
    })

    // Target: < 1us per operation
    b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

func BenchmarkStateStore_OnOrderCreated(b *testing.B) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(1000000))

    orders := make([]*Order, b.N)
    for i := 0; i < b.N; i++ {
        orders[i] = &Order{
            ID:    fmt.Sprintf("order-%d", i),
            Side:  OrderSideBuy,
            Price: decimal.NewFromFloat(0.5),
            Size:  decimal.NewFromFloat(1),
        }
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        store.OnOrderCreated(orders[i])
    }

    // Target: < 1ms per operation
}

func BenchmarkOrderBook_Update(b *testing.B) {
    ob := NewLockFreeOrderBook()

    // Initialize with snapshot
    ob.ApplySnapshot(OrderBookSnapshot{
        Bids: generatePriceLevels(100),
        Asks: generatePriceLevels(100),
    })

    deltas := make([]OrderBookDelta, b.N)
    for i := 0; i < b.N; i++ {
        deltas[i] = OrderBookDelta{
            Sequence: uint64(i),
            Bids:     []PriceLevel{{Price: decimal.NewFromFloat(rand.Float64()), Size: decimal.NewFromFloat(10)}},
        }
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        ob.Update(deltas[i])
    }
}

func BenchmarkOrderBook_Get_Concurrent(b *testing.B) {
    ob := NewLockFreeOrderBook()
    ob.ApplySnapshot(OrderBookSnapshot{
        Bids: generatePriceLevels(1000),
        Asks: generatePriceLevels(1000),
    })

    // Concurrent updater
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        seq := uint64(0)
        for {
            select {
            case <-ctx.Done():
                return
            default:
                ob.Update(OrderBookDelta{
                    Sequence: seq,
                    Bids:     []PriceLevel{{Price: decimal.NewFromFloat(rand.Float64()), Size: decimal.NewFromFloat(10)}},
                })
                seq++
            }
        }
    }()

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            snapshot := ob.Get()
            _ = snapshot.BestBid
        }
    })
}

func BenchmarkEventDispatch(b *testing.B) {
    dispatcher := NewEventDispatcher(4, 10000)

    callbackCount := int64(0)
    dispatcher.OnBalanceUpdate(func(b *BalanceDetail) {
        atomic.AddInt64(&callbackCount, 1)
    })

    events := make([]Event, b.N)
    for i := 0; i < b.N; i++ {
        events[i] = &BalanceUpdateEvent{
            Balance: &BalanceDetail{TotalBalance: decimal.NewFromFloat(float64(i))},
        }
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        dispatcher.Dispatch(events[i])
    }

    // Wait for all callbacks
    for atomic.LoadInt64(&callbackCount) < int64(b.N) {
        runtime.Gosched()
    }
}

// Memory allocation benchmark
func BenchmarkStateStore_NoAllocs(b *testing.B) {
    store := NewStateStore()
    store.SetCollateralTotal(decimal.NewFromFloat(1000))

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _ = store.GetCollateralBalance()
    }

    // Target: 0 allocs per operation
}
```

### 8. Test Utilities

```go
// test_helpers.go

type MockStreamHub struct {
    state *StateStore
}

func NewMockStreamHub() *MockStreamHub {
    return &MockStreamHub{
        state: NewStateStore(),
    }
}

type MockWebSocketServer struct {
    *httptest.Server
    clients []*websocket.Conn
    mu      sync.Mutex
}

func NewMockWebSocketServer() *MockWebSocketServer {
    s := &MockWebSocketServer{}

    upgrader := websocket.Upgrader{}
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        conn, _ := upgrader.Upgrade(w, r, nil)
        s.mu.Lock()
        s.clients = append(s.clients, conn)
        s.mu.Unlock()
    })

    s.Server = httptest.NewServer(handler)
    return s
}

func (s *MockWebSocketServer) SendMessage(msg interface{}) {
    data, _ := json.Marshal(msg)
    s.mu.Lock()
    defer s.mu.Unlock()

    for _, conn := range s.clients {
        conn.WriteMessage(websocket.TextMessage, data)
    }
}

func (s *MockWebSocketServer) DisconnectAll() {
    s.mu.Lock()
    defer s.mu.Unlock()

    for _, conn := range s.clients {
        conn.Close()
    }
    s.clients = nil
}

func generatePriceLevels(n int) []PriceLevel {
    levels := make([]PriceLevel, n)
    for i := 0; i < n; i++ {
        levels[i] = PriceLevel{
            Price: decimal.NewFromFloat(float64(i) * 0.01),
            Size:  decimal.NewFromFloat(float64(rand.Intn(1000))),
        }
    }
    return levels
}
```

---

## Deployment Considerations

### Configuration

```go
type StreamHubConfig struct {
    // Connection
    WebSocketURL        string
    RESTBaseURL         string
    RPCEndpoint         string

    // Timing
    RESTSyncInterval    time.Duration  `default:"5s"`
    OnChainSyncInterval time.Duration  `default:"30s"`
    PingInterval        time.Duration  `default:"15s"`
    PongTimeout         time.Duration  `default:"5s"`
    ReconnectBaseDelay  time.Duration  `default:"100ms"`
    ReconnectMaxDelay   time.Duration  `default:"30s"`
    OptimisticTimeout   time.Duration  `default:"5s"`

    // Limits
    MaxOpenOrders       int            `default:"1000"`
    MaxMarketsTracked   int            `default:"100"`
    MaxSequenceGap      int            `default:"1000"`

    // Risk
    MinAvailableBuffer  decimal.Decimal
    MaxOrderSize        decimal.Decimal
    MaxPositionSize     decimal.Decimal
    MaxDailyVolume      decimal.Decimal

    // Observability
    MetricsEnabled      bool           `default:"true"`
    MetricsPort         int            `default:"9090"`
    HealthCheckPort     int            `default:"8080"`
}
```

### Graceful Shutdown

```go
func (h *StreamHub) Shutdown(ctx context.Context) error {
    // 1. Stop accepting new operations
    h.state.tradingHalted = true

    // 2. Cancel all pending optimistic updates
    h.optimisticManager.CancelAll()

    // 3. Close WebSocket gracefully
    h.connectionManager.Close()

    // 4. Flush metrics
    h.metrics.Flush()

    // 5. Final state snapshot for recovery
    if err := h.saveStateSnapshot(); err != nil {
        return fmt.Errorf("failed to save state: %w", err)
    }

    return nil
}
```

---

## Future Considerations

- **Lock-free data structures**: For markets with extreme update frequency
- **State persistence**: WAL (Write-Ahead Log) for crash recovery
- **Multi-region**: Geo-distributed deployment for latency
- **Hardware acceleration**: FPGA for order book matching (if self-hosted)
- **Custom allocator**: Arena allocator for zero-GC hot path
