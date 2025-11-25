# Auto Management Example

This example demonstrates the automatic management features of the Polymarket Go SDK:
- **Auto-Redeem**: Automatically redeem resolved positions
- **Auto-Merge**: Automatically merge complementary token pairs (YES + NO → USDC)

## Features Demonstrated

### 1. Auto-Redeem Service
Automatically monitors user positions and redeems them when markets are resolved:
```go
polymarket.WithAutoRedeem(&polymarket.AutoRedeemConfig{
    PollingInterval: 30 * time.Second, // Check every 30 seconds
    Enabled:         true,
    OnSuccess: func(tokenID string, amount decimal.Decimal) {
        log.Printf("✓ Redeemed %s USDC", amount.String())
    },
    OnError: func(err error) {
        log.Printf("✗ Error: %v", err)
    },
})
```

### 2. Auto-Merge Service
Automatically monitors and merges complementary token pairs into USDC:
```go
polymarket.WithAutoMerge(&polymarket.AutoMergeConfig{
    PollingInterval: 60 * time.Second,          // Check every 60 seconds
    Enabled:         true,
    MinMergeAmount:  decimal.NewFromFloat(1.0), // Only merge if >= 1 USDC
    OnSuccess: func(conditionID string, amount decimal.Decimal) {
        log.Printf("✓ Merged %s USDC", amount.String())
    },
    OnError: func(err error) {
        log.Printf("✗ Error: %v", err)
    },
})
```

### 3. Complete Auto Management
Enable both services together:
```go
client, err := polymarket.NewClient(
    ethClient,
    polymarket.WithAutoRedeem(&polymarket.AutoRedeemConfig{...}),
    polymarket.WithAutoMerge(&polymarket.AutoMergeConfig{...}),
)
defer client.Close() // Ensure graceful shutdown
```

## Running the Example

1. Set up environment variables:
```bash
export PRIVATE_KEY="your_private_key_hex"
export RPC_URL="https://polygon-rpc.com"  # Optional, defaults to public RPC
```

2. Run the example:
```bash
cd examples/04_auto_management
go run main.go
```

3. The services will run in the background. Press `Ctrl+C` to stop gracefully.

## Key Concepts

### Graceful Shutdown
Always use `defer client.Close()` to ensure proper cleanup:
```go
client, err := polymarket.NewClient(ethClient, options...)
if err != nil {
    log.Fatal(err)
}
defer client.Close() // All background services will be stopped
```

### Manual Control
You can manually control services:
```go
client.StopAutoRedeem()      // Stop only redeem service
client.StopAutoMerge()       // Stop only merge service
client.StopAutoManagement()  // Stop all services
client.Close()               // Stop all and cleanup (recommended)
```

### Production Pattern
For long-running services, use signal handling:
```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

// Wait for shutdown signal
<-sigChan
log.Println("Shutting down gracefully...")
client.Close()
```

## Use Cases

### Auto-Redeem
- Automatically claim winnings when markets resolve
- No need to manually monitor resolved markets
- Ensures you don't miss redemption opportunities

### Auto-Merge
- Automatically consolidate complementary positions (YES + NO) back to USDC
- Useful after quick scalping or hedging strategies
- Reduces token fragmentation in your wallet

### Combined
- Set-and-forget position management
- Perfect for automated trading bots
- Maximizes capital efficiency

## Configuration Tips

### Polling Intervals
- **Auto-Redeem**: 30-60 seconds (markets don't resolve frequently)
- **Auto-Merge**: 60-120 seconds (less time-sensitive)

### Minimum Merge Amount
- Set based on gas costs vs. position size
- Recommended: 1-10 USDC to avoid frequent small merges
- Lower for testing: 0.1-0.5 USDC

### Error Handling
- Always implement `OnError` callbacks for monitoring
- Log errors to your monitoring system
- Consider retry logic for transient failures

## Notes

- Services run in background goroutines
- Minimal CPU/memory overhead
- Safe for production use
- Handles both standard and NegRisk markets automatically
