package polymarket

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/ivanzzeth/polymarket-go-clob-client/types"
	polymarketcontracts "github.com/ivanzzeth/polymarket-go-contracts"
)

// BalanceDetail contains detailed balance information including locked amounts
type BalanceDetail struct {
	TotalBalance     decimal.Decimal // Total balance including locked amounts
	LockedBalance    decimal.Decimal // Balance locked in open orders
	AvailableBalance decimal.Decimal // Available balance = Total - Locked
}

// GetCollateralBalance gets the USDC collateral balance in decimal units
func (c *Client) GetCollateralBalance(ctx context.Context) (decimal.Decimal, error) {
	return c.getBalanceInDecimal(types.AssetTypeCollateral, "", polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)
}

// GetCollateralBalanceDetail gets detailed USDC collateral balance including locked amounts
func (c *Client) GetCollateralBalanceDetail(ctx context.Context) (*BalanceDetail, error) {
	return c.getBalanceDetailInDecimal(types.AssetTypeCollateral, "", polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)
}

// GetAvailableCollateralBalance gets the available (unlocked) USDC collateral balance
// This should be used before placing orders to ensure sufficient available balance
func (c *Client) GetAvailableCollateralBalance(ctx context.Context) (decimal.Decimal, error) {
	return c.getAvailableBalanceInDecimal(types.AssetTypeCollateral, "", polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)
}

// GetPositionBalance gets the position token balance in decimal units
// A position token represents a YES or NO outcome in a market (internally: conditional token)
// tokenID: the token ID (e.g., "0x123...")
func (c *Client) GetPositionBalance(ctx context.Context, tokenID string) (decimal.Decimal, error) {
	return c.getBalanceInDecimal(types.AssetTypeConditional, tokenID, polymarketcontracts.CONDITIONAL_TOKEN_DECIMALS)
}

// GetPositionBalanceDetail gets detailed position token balance including locked amounts
// A position token represents a YES or NO outcome in a market (internally: conditional token)
func (c *Client) GetPositionBalanceDetail(ctx context.Context, tokenID string) (*BalanceDetail, error) {
	return c.getBalanceDetailInDecimal(types.AssetTypeConditional, tokenID, polymarketcontracts.CONDITIONAL_TOKEN_DECIMALS)
}

// GetAvailablePositionBalance gets the available (unlocked) position token balance
// This should be used before placing orders to ensure sufficient available balance
// A position token represents a YES or NO outcome in a market (internally: conditional token)
func (c *Client) GetAvailablePositionBalance(ctx context.Context, tokenID string) (decimal.Decimal, error) {
	return c.getAvailableBalanceInDecimal(types.AssetTypeConditional, tokenID, polymarketcontracts.CONDITIONAL_TOKEN_DECIMALS)
}

// getBalanceInDecimal is a helper function that gets balance/allowance and converts it to decimal units
// It divides the raw balance by 10^decimals to get the actual decimal value
// assetType: types.AssetTypeCollateral (USDC, 6 decimals) or types.AssetTypeConditional (Conditional Token, 6 decimals)
// tokenID: required for AssetTypeConditional, empty string for AssetTypeCollateral
func (c *Client) getBalanceInDecimal(assetType types.AssetType, tokenID string, decimals int32) (decimal.Decimal, error) {
	detail, err := c.getBalanceDetailInDecimal(assetType, tokenID, decimals)
	if err != nil {
		return decimal.Zero, err
	}
	return detail.TotalBalance, nil
}

// getAvailableBalanceInDecimal gets the available (unlocked) balance in decimal units
// This should be used before placing orders to ensure sufficient available balance
// Available balance = Total balance - Balance locked in open orders
func (c *Client) getAvailableBalanceInDecimal(
	assetType types.AssetType,
	tokenID string,
	decimals int32,
) (decimal.Decimal, error) {
	detail, err := c.getBalanceDetailInDecimal(assetType, tokenID, decimals)
	if err != nil {
		return decimal.Zero, err
	}
	return detail.AvailableBalance, nil
}

// getBalanceDetailInDecimal gets detailed balance information including locked amounts in open orders
// Available balance = Total balance - Locked balance (no dust calculation)
func (c *Client) getBalanceDetailInDecimal(
	assetType types.AssetType,
	tokenID string,
	decimals int32,
) (*BalanceDetail, error) {
	params := &types.BalanceAllowanceParams{
		AssetType: assetType,
	}
	if tokenID != "" {
		params.TokenID = tokenID
	}

	balanceAllowance, err := c.clobClient.GetBalanceAllowance(params)
	if err != nil {
		return nil, err
	}

	// Convert raw balance (with decimals) to decimal value
	// e.g., 2000000 raw units with 6 decimals = 2.0 actual units
	totalBalance := balanceAllowance.Balance.Div(decimal.New(1, decimals))

	// Get open orders to calculate locked balance
	lockedBalance := decimal.Zero

	// Query open orders for this asset
	openOrderParams := &types.OpenOrderParams{}
	if tokenID != "" {
		openOrderParams.AssetID = tokenID
	}

	openOrders, err := c.clobClient.GetOpenOrders(openOrderParams, false, "")
	if err != nil {
		// If we can't get open orders, we cannot accurately calculate available balance
		// Return error instead of potentially incorrect balance information
		return nil, err
	}

	// Calculate locked balance from open orders
	for _, order := range openOrders {
		// Filter orders for the specific asset
		if assetType == types.AssetTypeCollateral {
			// For collateral: check BUY orders (they lock collateral)
			if order.Side == types.OrderSideBuy {
				// Locked amount = (OriginalSize - SizeMatched) * Price
				remaining := order.OriginalSize.Sub(order.SizeMatched)
				lockedAmount := remaining.Mul(order.Price)
				lockedBalance = lockedBalance.Add(lockedAmount)
			}
		} else if assetType == types.AssetTypeConditional && order.AssetID == tokenID {
			// For conditional tokens: check SELL orders for this specific token
			if order.Side == types.OrderSideSell {
				// Locked amount = OriginalSize - SizeMatched
				remaining := order.OriginalSize.Sub(order.SizeMatched)
				lockedBalance = lockedBalance.Add(remaining)
			}
		}
	}

	// Calculate available balance
	availableBalance := totalBalance.Sub(lockedBalance)
	if availableBalance.LessThan(decimal.Zero) {
		availableBalance = decimal.Zero
	}

	return &BalanceDetail{
		TotalBalance:     totalBalance,
		LockedBalance:    lockedBalance,
		AvailableBalance: availableBalance,
	}, nil
}
