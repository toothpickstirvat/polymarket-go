package polymarket

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/ivanzzeth/polymarket-go-clob-client/v2/types"
)

// ConvertToComplementaryOrder converts a limit order to its complementary token order.
func ConvertToComplementaryOrder(order *types.UserOrder, complementaryTokenID string) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	var convertedSide types.OrderSide
	switch order.Side {
	case types.OrderSideBuy:
		convertedSide = types.OrderSideSell
	case types.OrderSideSell:
		convertedSide = types.OrderSideBuy
	default:
		return nil, fmt.Errorf("invalid order side: %s", order.Side)
	}

	one := decimal.NewFromInt(1)
	convertedPrice := one.Sub(order.Price)
	if convertedPrice.LessThan(decimal.Zero) || convertedPrice.GreaterThan(one) {
		return nil, fmt.Errorf("converted price %s is out of valid range [0, 1]", convertedPrice)
	}

	return &types.UserOrder{
		TokenID:     complementaryTokenID,
		Price:       convertedPrice,
		Size:        order.Size,
		Side:        convertedSide,
		Expiration:  order.Expiration,
		Metadata:    order.Metadata,
		BuilderCode: order.BuilderCode,
	}, nil
}

// ConvertMarketOrderToComplementary converts a market order to its complementary token order.
func ConvertMarketOrderToComplementary(order *types.UserMarketOrder, complementaryTokenID string) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	if order.Price.IsZero() {
		return nil, fmt.Errorf("market order must have price set for conversion")
	}

	limitOrder := &types.UserOrder{
		TokenID: order.TokenID,
		Price:   order.Price,
		Size:    order.Amount,
		Side:    order.Side,
	}

	convertedLimit, err := ConvertToComplementaryOrder(limitOrder, complementaryTokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert limit order: %w", err)
	}

	return &types.UserMarketOrder{
		TokenID: convertedLimit.TokenID,
		Price:   convertedLimit.Price,
		Amount:  order.Amount,
		Side:    convertedLimit.Side,
	}, nil
}

// ConvertToOppositeSideOrder converts a limit order to the opposite side with optional spread.
func ConvertToOppositeSideOrder(order *types.UserOrder, spread decimal.Decimal) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	if spread.LessThan(decimal.Zero) {
		return nil, fmt.Errorf("spread must be non-negative, got %s", spread)
	}

	var convertedSide types.OrderSide
	var convertedPrice decimal.Decimal

	switch order.Side {
	case types.OrderSideBuy:
		convertedSide = types.OrderSideSell
		convertedPrice = order.Price.Add(spread)
	case types.OrderSideSell:
		convertedSide = types.OrderSideBuy
		convertedPrice = order.Price.Sub(spread)
	default:
		return nil, fmt.Errorf("invalid order side: %s", order.Side)
	}

	one := decimal.NewFromInt(1)
	if convertedPrice.LessThan(decimal.Zero) || convertedPrice.GreaterThan(one) {
		return nil, fmt.Errorf("converted price %s is out of valid range [0, 1]", convertedPrice)
	}

	return &types.UserOrder{
		TokenID:     order.TokenID,
		Price:       convertedPrice,
		Size:        order.Size,
		Side:        convertedSide,
		Expiration:  order.Expiration,
		Metadata:    order.Metadata,
		BuilderCode: order.BuilderCode,
	}, nil
}

// ConvertMarketOrderToOppositeSide converts a market order to the opposite side with optional spread.
func ConvertMarketOrderToOppositeSide(order *types.UserMarketOrder, spread decimal.Decimal) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	if spread.LessThan(decimal.Zero) {
		return nil, fmt.Errorf("spread must be non-negative, got %s", spread)
	}

	var convertedSide types.OrderSide
	var convertedPrice decimal.Decimal
	isPriceSet := !order.Price.IsZero()

	switch order.Side {
	case types.OrderSideBuy:
		convertedSide = types.OrderSideSell
		if isPriceSet {
			convertedPrice = order.Price.Add(spread)
			one := decimal.NewFromInt(1)
			if convertedPrice.LessThan(decimal.Zero) || convertedPrice.GreaterThan(one) {
				return nil, fmt.Errorf("converted price %s is out of valid range [0, 1]", convertedPrice)
			}
		}
	case types.OrderSideSell:
		convertedSide = types.OrderSideBuy
		if isPriceSet {
			convertedPrice = order.Price.Sub(spread)
			one := decimal.NewFromInt(1)
			if convertedPrice.LessThan(decimal.Zero) || convertedPrice.GreaterThan(one) {
				return nil, fmt.Errorf("converted price %s is out of valid range [0, 1]", convertedPrice)
			}
		}
	default:
		return nil, fmt.Errorf("invalid order side: %s", order.Side)
	}

	return &types.UserMarketOrder{
		TokenID: order.TokenID,
		Price:   convertedPrice,
		Amount:  order.Amount,
		Side:    convertedSide,
	}, nil
}

// ConvertToMatchingSameSideOrder combines opposite side + complementary conversions.
func ConvertToMatchingSameSideOrder(order *types.UserOrder, complementaryTokenID string, spread decimal.Decimal) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	opposite, err := ConvertToOppositeSideOrder(order, spread)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to opposite side: %w", err)
	}
	return ConvertToComplementaryOrder(opposite, complementaryTokenID)
}

// ConvertMarketOrderToMatchingSameSide converts a market order to a matching same-side order.
func ConvertMarketOrderToMatchingSameSide(order *types.UserMarketOrder, complementaryTokenID string, spread decimal.Decimal) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	opposite, err := ConvertMarketOrderToOppositeSide(order, spread)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to opposite side: %w", err)
	}
	return ConvertMarketOrderToComplementary(opposite, complementaryTokenID)
}
