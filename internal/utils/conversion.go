package utils

import (
	"fmt"
	"math/big"

	polymarketcontracts "github.com/ivanzzeth/polymarket-go-contracts/v2"
	"github.com/shopspring/decimal"
)

// DecimalToRawAmount converts a decimal amount to raw token amount (big.Int)
// Polymarket uses 6 decimals for both USDC collateral and conditional tokens
// Example: 1.5 USDC -> 1500000 (raw units)
func DecimalToRawAmount(amount decimal.Decimal) *big.Int {
	// Multiply by 10^6 to convert to raw units
	rawAmount := amount.Mul(decimal.New(1, polymarketcontracts.COLLATERAL_TOKEN_DECIMALS))
	return rawAmount.BigInt()
}

// ValidateConditionId validates that the condition ID is a valid hex string
func ValidateConditionId(conditionId string) error {
	if conditionId == "" {
		return fmt.Errorf("condition ID cannot be empty")
	}

	// Remove "0x" prefix if present
	hexStr := conditionId
	if len(hexStr) >= 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	// Check if it's a valid hex string (should only contain 0-9, a-f, A-F)
	if len(hexStr) == 0 {
		return fmt.Errorf("invalid condition ID: empty after removing 0x prefix")
	}

	for i, c := range hexStr {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("invalid condition ID: contains non-hex character at position %d", i)
		}
	}

	// Condition ID should be 32 bytes = 64 hex characters
	// But we allow shorter strings (they will be left-padded by common.HexToHash)
	if len(hexStr) > 64 {
		return fmt.Errorf("invalid condition ID: hex string too long (%d characters, max 64)", len(hexStr))
	}

	return nil
}
