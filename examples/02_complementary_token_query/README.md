# Complementary Token Query Example

This example demonstrates how to query complementary tokens in Polymarket binary markets.

## Background

In Polymarket's binary markets, every position token has a complementary counterpart:
- **YES token** ↔ **NO token**
- If you know one token's ID, you can query its complement using the Exchange contract

## How It Works

The `GetComplementaryTokenID` method:
1. Accepts a token ID as a string (user-friendly)
2. Converts to `*big.Int` internally
3. Calls `Exchange.GetComplement()` contract method
4. Caches the result bidirectionally (A→B and B→A)
5. Returns the complementary token ID as a string

## Cache Benefits

Since complementary relationships are permanent and bidirectional:
- First query: Calls the contract
- Subsequent queries: Returns from cache (no contract call)
- Both directions cached: If you query A→B, then B→A is also cached

## Usage

```bash
# Set RPC URL (optional, defaults to public Polygon RPC)
export RPC_URL="https://polygon-rpc.com"

# Set token ID to query
export TOKEN_ID="21742633143463906290569050155826241533067272736897614950488156847949938836455"

# Run the example
go run main.go
```

## Example Output

```
=== Polymarket Complementary Token Query Example ===

1. Querying Complementary Token for Token ID: 21742633143463906290569050155826241533067272736897614950488156847949938836455
  Original Token ID:      21742633143463906290569050155826241533067272736897614950488156847949938836455
  Complementary Token ID: 21742633143463906290569050155826241533067272736897614950488156847949938836456

  Note: If original token is YES, complementary is NO, and vice versa

2. Querying Again (from cache):
  Complementary Token ID: 21742633143463906290569050155826241533067272736897614950488156847949938836456 (from cache)

3. Querying Reverse Direction (from cache):
  Input: 21742633143463906290569050155826241533067272736897614950488156847949938836456
  Output: 21742633143463906290569050155826241533067272736897614950488156847949938836455
  Matches original: true

=== Example Complete ===
```

## Notes

- This is a **read-only** operation (no gas fees, no private key required)
- Token IDs are large numbers, represented as strings for user convenience
- The complementary relationship is permanent and stored on-chain
- Caching makes subsequent queries very fast
