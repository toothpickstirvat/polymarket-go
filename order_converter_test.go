package polymarket

import (
	"math/big"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/ivanzzeth/polymarket-go-clob-client/types"
)

func TestConvertOrder(t *testing.T) {
	tests := []struct {
		name                 string
		order                *types.UserOrder
		complementaryTokenID string
		wantErr              bool
		validateResult       func(*testing.T, *types.UserOrder)
	}{
		{
			name: "Buy YES @ 0.6 converts to Sell NO @ 0.4",
			order: &types.UserOrder{
				TokenID:    "token-yes",
				Price:      decimal.NewFromFloat(0.6),
				Size:       decimal.NewFromInt(100),
				Side:       types.OrderSideBuy,
				FeeRateBps: 10,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				if result.TokenID != "token-no" {
					t.Errorf("TokenID = %v, want token-no", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.4)
				if !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if result.Side != types.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideSell)
				}
				if !result.Size.Equal(decimal.NewFromInt(100)) {
					t.Errorf("Size = %v, want 100", result.Size)
				}
				if result.FeeRateBps != 10 {
					t.Errorf("FeeRateBps = %v, want 10", result.FeeRateBps)
				}
			},
		},
		{
			name: "Sell YES @ 0.3 converts to Buy NO @ 0.7",
			order: &types.UserOrder{
				TokenID:    "token-yes",
				Price:      decimal.NewFromFloat(0.3),
				Size:       decimal.NewFromInt(50),
				Side:       types.OrderSideSell,
				FeeRateBps: 5,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				if result.TokenID != "token-no" {
					t.Errorf("TokenID = %v, want token-no", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.7)
				if !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if result.Side != types.OrderSideBuy {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideBuy)
				}
			},
		},
		{
			name: "Buy NO @ 0.45 converts to Sell YES @ 0.55",
			order: &types.UserOrder{
				TokenID:    "token-no",
				Price:      decimal.NewFromFloat(0.45),
				Size:       decimal.NewFromInt(200),
				Side:       types.OrderSideBuy,
				FeeRateBps: 0,
			},
			complementaryTokenID: "token-yes",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				if result.TokenID != "token-yes" {
					t.Errorf("TokenID = %v, want token-yes", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.55)
				if !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if result.Side != types.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideSell)
				}
			},
		},
		{
			name: "Sell NO @ 0.8 converts to Buy YES @ 0.2",
			order: &types.UserOrder{
				TokenID:    "token-no",
				Price:      decimal.NewFromFloat(0.8),
				Size:       decimal.NewFromInt(75),
				Side:       types.OrderSideSell,
				FeeRateBps: 15,
			},
			complementaryTokenID: "token-yes",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				if result.TokenID != "token-yes" {
					t.Errorf("TokenID = %v, want token-yes", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.2)
				if !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if result.Side != types.OrderSideBuy {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideBuy)
				}
			},
		},
		{
			name: "Edge case: Price 0 converts to Price 1",
			order: &types.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.Zero,
				Size:    decimal.NewFromInt(10),
				Side:    types.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				expectedPrice := decimal.NewFromInt(1)
				if !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
			},
		},
		{
			name: "Edge case: Price 1 converts to Price 0",
			order: &types.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.NewFromInt(1),
				Size:    decimal.NewFromInt(10),
				Side:    types.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				if !result.Price.Equal(decimal.Zero) {
					t.Errorf("Price = %v, want 0", result.Price)
				}
			},
		},
		{
			name: "Preserves optional fields: Nonce, Expiration, Taker",
			order: &types.UserOrder{
				TokenID:    "token-yes",
				Price:      decimal.NewFromFloat(0.5),
				Size:       decimal.NewFromInt(100),
				Side:       types.OrderSideBuy,
				FeeRateBps: 10,
				Nonce:      big.NewInt(12345),
				Expiration: func() *int64 { v := int64(1234567890); return &v }(),
				Taker:      "0xABCDEF1234567890",
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				if result.Nonce.Cmp(big.NewInt(12345)) != 0 {
					t.Errorf("Nonce = %v, want 12345", result.Nonce)
				}
				if result.Expiration == nil || *result.Expiration != 1234567890 {
					t.Errorf("Expiration = %v, want 1234567890", result.Expiration)
				}
				if result.Taker != "0xABCDEF1234567890" {
					t.Errorf("Taker = %v, want 0xABCDEF1234567890", result.Taker)
				}
			},
		},
		{
			name:                 "Error: nil order",
			order:                nil,
			complementaryTokenID: "token-no",
			wantErr:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertOrder(tt.order, tt.complementaryTokenID)

			if (err != nil) != tt.wantErr {
				t.Errorf("convertOrder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestConvertOrderPriceFormula(t *testing.T) {
	// Test the mathematical relationship: P_converted = 1 - P_original
	testPrices := []float64{0.0, 0.1, 0.25, 0.5, 0.75, 0.9, 1.0}

	for _, price := range testPrices {
		t.Run(decimal.NewFromFloat(price).String(), func(t *testing.T) {
			order := &types.UserOrder{
				TokenID: "token-a",
				Price:   decimal.NewFromFloat(price),
				Size:    decimal.NewFromInt(1),
				Side:    types.OrderSideBuy,
			}

			result, err := convertOrder(order, "token-b")
			if err != nil {
				t.Fatalf("convertOrder() error = %v", err)
			}

			expected := decimal.NewFromInt(1).Sub(decimal.NewFromFloat(price))
			if !result.Price.Equal(expected) {
				t.Errorf("convertOrder() price = %v, want %v (1 - %v)", result.Price, expected, price)
			}
		})
	}
}

func TestConvertOrderSideConversion(t *testing.T) {
	tests := []struct {
		originalSide types.OrderSide
		expectedSide types.OrderSide
	}{
		{types.OrderSideBuy, types.OrderSideSell},
		{types.OrderSideSell, types.OrderSideBuy},
	}

	for _, tt := range tests {
		t.Run(string(tt.originalSide), func(t *testing.T) {
			order := &types.UserOrder{
				TokenID: "token-a",
				Price:   decimal.NewFromFloat(0.5),
				Size:    decimal.NewFromInt(1),
				Side:    tt.originalSide,
			}

			result, err := convertOrder(order, "token-b")
			if err != nil {
				t.Fatalf("convertOrder() error = %v", err)
			}

			if result.Side != tt.expectedSide {
				t.Errorf("convertOrder() side = %v, want %v", result.Side, tt.expectedSide)
			}
		})
	}
}

func TestConvertMarketOrder(t *testing.T) {
	tests := []struct {
		name                 string
		order                *types.UserMarketOrder
		complementaryTokenID string
		wantErr              bool
		validateResult       func(*testing.T, *types.UserMarketOrder)
	}{
		{
			name: "Buy market order YES @ 0.6 converts to Sell market order NO @ 0.4",
			order: &types.UserMarketOrder{
				TokenID: "token-yes",
				Price:   func() *decimal.Decimal { p := decimal.NewFromFloat(0.6); return &p }(),
				Amount:  decimal.NewFromInt(100),
				Side:    types.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				if result.TokenID != "token-no" {
					t.Errorf("TokenID = %v, want token-no", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.4)
				if result.Price == nil || !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if result.Side != types.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideSell)
				}
				if !result.Amount.Equal(decimal.NewFromInt(100)) {
					t.Errorf("Amount = %v, want 100", result.Amount)
				}
			},
		},
		{
			name: "Sell market order YES @ 0.3 converts to Buy market order NO @ 0.7",
			order: &types.UserMarketOrder{
				TokenID: "token-yes",
				Price:   func() *decimal.Decimal { p := decimal.NewFromFloat(0.3); return &p }(),
				Amount:  decimal.NewFromInt(50),
				Side:    types.OrderSideSell,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				if result.TokenID != "token-no" {
					t.Errorf("TokenID = %v, want token-no", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.7)
				if result.Price == nil || !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if result.Side != types.OrderSideBuy {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideBuy)
				}
			},
		},
		{
			name: "Buy market order NO @ 0.45 converts to Sell market order YES @ 0.55",
			order: &types.UserMarketOrder{
				TokenID: "token-no",
				Price:   func() *decimal.Decimal { p := decimal.NewFromFloat(0.45); return &p }(),
				Amount:  decimal.NewFromInt(200),
				Side:    types.OrderSideBuy,
			},
			complementaryTokenID: "token-yes",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				if result.TokenID != "token-yes" {
					t.Errorf("TokenID = %v, want token-yes", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.55)
				if result.Price == nil || !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if result.Side != types.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideSell)
				}
			},
		},
		{
			name: "Sell market order NO @ 0.8 converts to Buy market order YES @ 0.2",
			order: &types.UserMarketOrder{
				TokenID: "token-no",
				Price:   func() *decimal.Decimal { p := decimal.NewFromFloat(0.8); return &p }(),
				Amount:  decimal.NewFromInt(75),
				Side:    types.OrderSideSell,
			},
			complementaryTokenID: "token-yes",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				if result.TokenID != "token-yes" {
					t.Errorf("TokenID = %v, want token-yes", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.2)
				if result.Price == nil || !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if result.Side != types.OrderSideBuy {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideBuy)
				}
			},
		},
		{
			name: "Preserves optional fields: FeeRateBps, Nonce, Taker",
			order: &types.UserMarketOrder{
				TokenID:    "token-yes",
				Price:      func() *decimal.Decimal { p := decimal.NewFromFloat(0.5); return &p }(),
				Amount:     decimal.NewFromInt(100),
				Side:       types.OrderSideBuy,
				FeeRateBps: func() *int { v := 10; return &v }(),
				Nonce:      big.NewInt(12345),
				Taker:      "0xABCDEF1234567890",
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				if result.FeeRateBps == nil || *result.FeeRateBps != 10 {
					t.Errorf("FeeRateBps = %v, want 10", result.FeeRateBps)
				}
				if result.Nonce.Cmp(big.NewInt(12345)) != 0 {
					t.Errorf("Nonce = %v, want 12345", result.Nonce)
				}
				if result.Taker != "0xABCDEF1234567890" {
					t.Errorf("Taker = %v, want 0xABCDEF1234567890", result.Taker)
				}
			},
		},
		{
			name: "Edge case: Price 0 converts to Price 1",
			order: &types.UserMarketOrder{
				TokenID: "token-yes",
				Price:   func() *decimal.Decimal { p := decimal.Zero; return &p }(),
				Amount:  decimal.NewFromInt(10),
				Side:    types.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				expectedPrice := decimal.NewFromInt(1)
				if result.Price == nil || !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
			},
		},
		{
			name: "Edge case: Price 1 converts to Price 0",
			order: &types.UserMarketOrder{
				TokenID: "token-yes",
				Price:   func() *decimal.Decimal { p := decimal.NewFromInt(1); return &p }(),
				Amount:  decimal.NewFromInt(10),
				Side:    types.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				if result.Price == nil || !result.Price.Equal(decimal.Zero) {
					t.Errorf("Price = %v, want 0", result.Price)
				}
			},
		},
		{
			name:                 "Error: nil market order",
			order:                nil,
			complementaryTokenID: "token-no",
			wantErr:              true,
		},
		{
			name: "Error: market order without price",
			order: &types.UserMarketOrder{
				TokenID: "token-yes",
				Price:   nil,
				Amount:  decimal.NewFromInt(10),
				Side:    types.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertMarketOrder(tt.order, tt.complementaryTokenID)

			if (err != nil) != tt.wantErr {
				t.Errorf("convertMarketOrder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestMarketOrderToLimitOrder(t *testing.T) {
	tests := []struct {
		name           string
		order          *types.UserMarketOrder
		wantErr        bool
		validateResult func(*testing.T, *types.UserOrder)
	}{
		{
			name: "Convert market order with all fields",
			order: &types.UserMarketOrder{
				TokenID:    "token-yes",
				Price:      func() *decimal.Decimal { p := decimal.NewFromFloat(0.6); return &p }(),
				Amount:     decimal.NewFromInt(100),
				Side:       types.OrderSideBuy,
				FeeRateBps: func() *int { v := 10; return &v }(),
				Nonce:      big.NewInt(12345),
				Taker:      "0xABCDEF1234567890",
			},
			wantErr: false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				if result.TokenID != "token-yes" {
					t.Errorf("TokenID = %v, want token-yes", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.6)
				if !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if !result.Size.Equal(decimal.NewFromInt(100)) {
					t.Errorf("Size = %v, want 100", result.Size)
				}
				if result.Side != types.OrderSideBuy {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideBuy)
				}
				if result.FeeRateBps != 10 {
					t.Errorf("FeeRateBps = %v, want 10", result.FeeRateBps)
				}
				if result.Nonce.Cmp(big.NewInt(12345)) != 0 {
					t.Errorf("Nonce = %v, want 12345", result.Nonce)
				}
				if result.Taker != "0xABCDEF1234567890" {
					t.Errorf("Taker = %v, want 0xABCDEF1234567890", result.Taker)
				}
			},
		},
		{
			name: "Convert market order without optional fields",
			order: &types.UserMarketOrder{
				TokenID: "token-yes",
				Price:   func() *decimal.Decimal { p := decimal.NewFromFloat(0.5); return &p }(),
				Amount:  decimal.NewFromInt(50),
				Side:    types.OrderSideSell,
			},
			wantErr: false,
			validateResult: func(t *testing.T, result *types.UserOrder) {
				if result.FeeRateBps != 0 {
					t.Errorf("FeeRateBps = %v, want 0", result.FeeRateBps)
				}
			},
		},
		{
			name: "Error: market order without price",
			order: &types.UserMarketOrder{
				TokenID: "token-yes",
				Price:   nil,
				Amount:  decimal.NewFromInt(10),
				Side:    types.OrderSideBuy,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marketOrderToLimitOrder(tt.order)

			if (err != nil) != tt.wantErr {
				t.Errorf("marketOrderToLimitOrder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestLimitOrderToMarketOrder(t *testing.T) {
	tests := []struct {
		name           string
		order          *types.UserOrder
		amount         decimal.Decimal
		validateResult func(*testing.T, *types.UserMarketOrder)
	}{
		{
			name: "Convert limit order with all fields",
			order: &types.UserOrder{
				TokenID:    "token-yes",
				Price:      decimal.NewFromFloat(0.6),
				Size:       decimal.NewFromInt(100),
				Side:       types.OrderSideBuy,
				FeeRateBps: 10,
				Nonce:      big.NewInt(12345),
				Expiration: func() *int64 { v := int64(1234567890); return &v }(),
				Taker:      "0xABCDEF1234567890",
			},
			amount: decimal.NewFromInt(100),
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				if result.TokenID != "token-yes" {
					t.Errorf("TokenID = %v, want token-yes", result.TokenID)
				}
				expectedPrice := decimal.NewFromFloat(0.6)
				if result.Price == nil || !result.Price.Equal(expectedPrice) {
					t.Errorf("Price = %v, want %v", result.Price, expectedPrice)
				}
				if !result.Amount.Equal(decimal.NewFromInt(100)) {
					t.Errorf("Amount = %v, want 100", result.Amount)
				}
				if result.Side != types.OrderSideBuy {
					t.Errorf("Side = %v, want %v", result.Side, types.OrderSideBuy)
				}
				if result.FeeRateBps == nil || *result.FeeRateBps != 10 {
					t.Errorf("FeeRateBps = %v, want 10", result.FeeRateBps)
				}
				if result.Nonce.Cmp(big.NewInt(12345)) != 0 {
					t.Errorf("Nonce = %v, want 12345", result.Nonce)
				}
				if result.Taker != "0xABCDEF1234567890" {
					t.Errorf("Taker = %v, want 0xABCDEF1234567890", result.Taker)
				}
			},
		},
		{
			name: "Convert limit order without optional fields",
			order: &types.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.NewFromFloat(0.5),
				Size:    decimal.NewFromInt(50),
				Side:    types.OrderSideSell,
			},
			amount: decimal.NewFromInt(50),
			validateResult: func(t *testing.T, result *types.UserMarketOrder) {
				if result.FeeRateBps != nil {
					t.Errorf("FeeRateBps = %v, want nil", result.FeeRateBps)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := limitOrderToMarketOrder(tt.order, tt.amount)
			if err != nil {
				t.Fatalf("limitOrderToMarketOrder() error = %v", err)
			}

			if tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}
