package polymarket

import (
	"testing"

	"github.com/shopspring/decimal"

	clobtypes "github.com/ivanzzeth/polymarket-go-clob-client/v2/types"
)

func TestConvertOrder(t *testing.T) {
	tests := []struct {
		name                 string
		order                *clobtypes.UserOrder
		complementaryTokenID string
		wantErr              bool
		validateResult       func(*testing.T, *clobtypes.UserOrder)
	}{
		{
			name: "Buy YES @ 0.6 converts to Sell NO @ 0.4",
			order: &clobtypes.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.NewFromFloat(0.6),
				Size:    decimal.NewFromInt(100),
				Side:    clobtypes.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *clobtypes.UserOrder) {
				if result.TokenID != "token-no" {
					t.Errorf("TokenID = %v, want token-no", result.TokenID)
				}
				if !result.Price.Equal(decimal.NewFromFloat(0.4)) {
					t.Errorf("Price = %v, want 0.4", result.Price)
				}
				if result.Side != clobtypes.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, clobtypes.OrderSideSell)
				}
				if !result.Size.Equal(decimal.NewFromInt(100)) {
					t.Errorf("Size = %v, want 100", result.Size)
				}
			},
		},
		{
			name: "Sell YES @ 0.3 converts to Buy NO @ 0.7",
			order: &clobtypes.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.NewFromFloat(0.3),
				Size:    decimal.NewFromInt(50),
				Side:    clobtypes.OrderSideSell,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *clobtypes.UserOrder) {
				if !result.Price.Equal(decimal.NewFromFloat(0.7)) {
					t.Errorf("Price = %v, want 0.7", result.Price)
				}
				if result.Side != clobtypes.OrderSideBuy {
					t.Errorf("Side = %v, want %v", result.Side, clobtypes.OrderSideBuy)
				}
			},
		},
		{
			name: "Edge case: Price 0 converts to Price 1",
			order: &clobtypes.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.Zero,
				Size:    decimal.NewFromInt(10),
				Side:    clobtypes.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *clobtypes.UserOrder) {
				if !result.Price.Equal(decimal.NewFromInt(1)) {
					t.Errorf("Price = %v, want 1", result.Price)
				}
			},
		},
		{
			name: "Edge case: Price 1 converts to Price 0",
			order: &clobtypes.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.NewFromInt(1),
				Size:    decimal.NewFromInt(10),
				Side:    clobtypes.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *clobtypes.UserOrder) {
				if !result.Price.Equal(decimal.Zero) {
					t.Errorf("Price = %v, want 0", result.Price)
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
			result, err := ConvertToComplementaryOrder(tt.order, tt.complementaryTokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToComplementaryOrder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestConvertOrderPriceFormula(t *testing.T) {
	testPrices := []float64{0.0, 0.1, 0.25, 0.5, 0.75, 0.9, 1.0}
	for _, price := range testPrices {
		t.Run(decimal.NewFromFloat(price).String(), func(t *testing.T) {
			order := &clobtypes.UserOrder{
				TokenID: "token-a",
				Price:   decimal.NewFromFloat(price),
				Size:    decimal.NewFromInt(1),
				Side:    clobtypes.OrderSideBuy,
			}
			result, err := ConvertToComplementaryOrder(order, "token-b")
			if err != nil {
				t.Fatalf("ConvertToComplementaryOrder() error = %v", err)
			}
			expected := decimal.NewFromInt(1).Sub(decimal.NewFromFloat(price))
			if !result.Price.Equal(expected) {
				t.Errorf("price = %v, want %v (1 - %v)", result.Price, expected, price)
			}
		})
	}
}

func TestConvertOrderSideConversion(t *testing.T) {
	tests := []struct {
		originalSide clobtypes.OrderSide
		expectedSide clobtypes.OrderSide
	}{
		{clobtypes.OrderSideBuy, clobtypes.OrderSideSell},
		{clobtypes.OrderSideSell, clobtypes.OrderSideBuy},
	}
	for _, tt := range tests {
		t.Run(string(tt.originalSide), func(t *testing.T) {
			order := &clobtypes.UserOrder{
				TokenID: "token-a",
				Price:   decimal.NewFromFloat(0.5),
				Size:    decimal.NewFromInt(1),
				Side:    tt.originalSide,
			}
			result, err := ConvertToComplementaryOrder(order, "token-b")
			if err != nil {
				t.Fatalf("ConvertToComplementaryOrder() error = %v", err)
			}
			if result.Side != tt.expectedSide {
				t.Errorf("side = %v, want %v", result.Side, tt.expectedSide)
			}
		})
	}
}

func TestConvertMarketOrder(t *testing.T) {
	tests := []struct {
		name                 string
		order                *clobtypes.UserMarketOrder
		complementaryTokenID string
		wantErr              bool
		validateResult       func(*testing.T, *clobtypes.UserMarketOrder)
	}{
		{
			name: "Buy market order YES @ 0.6 converts to Sell NO @ 0.4",
			order: &clobtypes.UserMarketOrder{
				TokenID: "token-yes",
				Price:   decimal.NewFromFloat(0.6),
				Amount:  decimal.NewFromInt(100),
				Side:    clobtypes.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              false,
			validateResult: func(t *testing.T, result *clobtypes.UserMarketOrder) {
				if result.TokenID != "token-no" {
					t.Errorf("TokenID = %v, want token-no", result.TokenID)
				}
				if !result.Price.Equal(decimal.NewFromFloat(0.4)) {
					t.Errorf("Price = %v, want 0.4", result.Price)
				}
				if result.Side != clobtypes.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, clobtypes.OrderSideSell)
				}
			},
		},
		{
			name: "Buy market order NO @ 0.45 converts to Sell YES @ 0.55",
			order: &clobtypes.UserMarketOrder{
				TokenID: "token-no",
				Price:   decimal.NewFromFloat(0.45),
				Amount:  decimal.NewFromInt(200),
				Side:    clobtypes.OrderSideBuy,
			},
			complementaryTokenID: "token-yes",
			wantErr:              false,
			validateResult: func(t *testing.T, result *clobtypes.UserMarketOrder) {
				if !result.Price.Equal(decimal.NewFromFloat(0.55)) {
					t.Errorf("Price = %v, want 0.55", result.Price)
				}
				if result.Side != clobtypes.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, clobtypes.OrderSideSell)
				}
			},
		},
		{
			name: "Error: market order without price",
			order: &clobtypes.UserMarketOrder{
				TokenID: "token-yes",
				Amount:  decimal.NewFromInt(10),
				Side:    clobtypes.OrderSideBuy,
			},
			complementaryTokenID: "token-no",
			wantErr:              true,
		},
		{
			name:                 "Error: nil market order",
			complementaryTokenID: "token-no",
			wantErr:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertMarketOrderToComplementary(tt.order, tt.complementaryTokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertMarketOrderToComplementary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestConvertToOppositeSideOrder(t *testing.T) {
	tests := []struct {
		name     string
		order    *clobtypes.UserOrder
		spread   decimal.Decimal
		wantErr  bool
		validate func(*testing.T, *clobtypes.UserOrder)
	}{
		{
			name: "Buy YES @ 0.49 converts to Sell YES @ 0.49",
			order: &clobtypes.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.NewFromFloat(0.49),
				Size:    decimal.NewFromInt(100),
				Side:    clobtypes.OrderSideBuy,
			},
			validate: func(t *testing.T, result *clobtypes.UserOrder) {
				if result.Side != clobtypes.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, clobtypes.OrderSideSell)
				}
				if !result.Price.Equal(decimal.NewFromFloat(0.49)) {
					t.Errorf("Price = %v, want 0.49", result.Price)
				}
			},
		},
		{
			name: "Buy YES @ 0.49 with spread 0.02 -> Sell YES @ 0.51",
			order: &clobtypes.UserOrder{
				TokenID: "token-yes",
				Price:   decimal.NewFromFloat(0.49),
				Size:    decimal.NewFromInt(100),
				Side:    clobtypes.OrderSideBuy,
			},
			spread: decimal.NewFromFloat(0.02),
			validate: func(t *testing.T, result *clobtypes.UserOrder) {
				if !result.Price.Equal(decimal.NewFromFloat(0.51)) {
					t.Errorf("Price = %v, want 0.51", result.Price)
				}
				if result.Side != clobtypes.OrderSideSell {
					t.Errorf("Side = %v, want %v", result.Side, clobtypes.OrderSideSell)
				}
			},
		},
		{
			name:    "Nil order returns error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertToOppositeSideOrder(tt.order, tt.spread)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToOppositeSideOrder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
