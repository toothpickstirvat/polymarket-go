package polymarket

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/ivanzzeth/polymarket-go-clob-client/types"
)

// convertOrder converts a limit order to its equivalent opposite side
// Based on Polymarket's complementary token mechanism:
//   - Buy token @ P  → Sell complementary @ (1-P)
//   - Sell token @ P → Buy complementary @ (1-P)
//
// This allows traders to achieve the same position using whichever side has better liquidity
// For example: Buy YES @ 0.6 = Sell NO @ 0.4
func convertOrder(order *types.UserOrder, complementaryTokenID string) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	// Convert side: BUY ↔ SELL
	var convertedSide types.OrderSide
	if order.Side == types.OrderSideBuy {
		convertedSide = types.OrderSideSell
	} else if order.Side == types.OrderSideSell {
		convertedSide = types.OrderSideBuy
	} else {
		return nil, fmt.Errorf("invalid order side: %s", order.Side)
	}

	// Convert price: P → (1 - P)
	one := decimal.NewFromInt(1)
	convertedPrice := one.Sub(order.Price)

	// Validate converted price is in valid range [0, 1]
	if convertedPrice.LessThan(decimal.Zero) || convertedPrice.GreaterThan(one) {
		return nil, fmt.Errorf("converted price %s is out of valid range [0, 1]", convertedPrice)
	}

	// Create converted order with same parameters except side, token, and price
	converted := &types.UserOrder{
		TokenID:    complementaryTokenID,
		Price:      convertedPrice,
		Size:       order.Size,
		Side:       convertedSide,
		FeeRateBps: order.FeeRateBps,
		Nonce:      order.Nonce,
		Expiration: order.Expiration,
		Taker:      order.Taker,
	}

	return converted, nil
}

// convertMarketOrder converts a market order to its equivalent opposite side
// It works by: market order → limit order → convert → limit order → market order
func convertMarketOrder(order *types.UserMarketOrder, complementaryTokenID string) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	// Convert to limit order
	limitOrder, err := marketOrderToLimitOrder(order)
	if err != nil {
		return nil, fmt.Errorf("failed to convert market order to limit order: %w", err)
	}

	// Convert limit order
	convertedLimit, err := convertOrder(limitOrder, complementaryTokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert limit order: %w", err)
	}

	// Convert back to market order
	convertedMarket, err := limitOrderToMarketOrder(convertedLimit, order.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert limit order to market order: %w", err)
	}

	return convertedMarket, nil
}

// marketOrderToLimitOrder converts a market order to a limit order
func marketOrderToLimitOrder(order *types.UserMarketOrder) (*types.UserOrder, error) {
	if order.Price == nil {
		return nil, fmt.Errorf("market order must have price set for conversion")
	}

	limitOrder := &types.UserOrder{
		TokenID: order.TokenID,
		Price:   *order.Price,
		Size:    order.Amount,
		Side:    order.Side,
		Nonce:   order.Nonce,
		Taker:   order.Taker,
	}

	// Convert FeeRateBps if present
	if order.FeeRateBps != nil {
		limitOrder.FeeRateBps = *order.FeeRateBps
	}

	return limitOrder, nil
}

// limitOrderToMarketOrder converts a limit order back to a market order
func limitOrderToMarketOrder(order *types.UserOrder, amount decimal.Decimal) (*types.UserMarketOrder, error) {
	marketOrder := &types.UserMarketOrder{
		TokenID: order.TokenID,
		Price:   &order.Price,
		Amount:  amount,
		Side:    order.Side,
		Nonce:   order.Nonce,
		Taker:   order.Taker,
	}

	// Convert FeeRateBps to pointer
	if order.FeeRateBps != 0 {
		marketOrder.FeeRateBps = &order.FeeRateBps
	}

	return marketOrder, nil
}
