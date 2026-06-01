package polymarket

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"

	"github.com/ivanzzeth/polymarket-go-clob-client/v2/types"
	polymarketcontracts "github.com/ivanzzeth/polymarket-go-contracts/v2"
)

type BalanceQueryOption struct {
	Source      DataSource
	BlockNumber *big.Int
	Address     *common.Address
}

type BalanceDetail struct {
	TotalBalance     decimal.Decimal
	LockedBalance    decimal.Decimal
	AvailableBalance decimal.Decimal
}

// GetCollateralBalance gets the pUSD collateral balance (primary collateral).
func (c *Client) GetCollateralBalance(ctx context.Context, opt *BalanceQueryOption) (decimal.Decimal, error) {
	if opt == nil {
		opt = &BalanceQueryOption{Source: DataSourceCLOB}
	}
	if opt.Source == DataSourceOnChain {
		addr := c.funderAddr
		if opt.Address != nil {
			addr = *opt.Address
		}
		return c.getChainBalance(ctx, addr, opt.BlockNumber)
	}
	return c.getCLOBBalance(types.AssetTypeCollateral, "", polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)
}

func (c *Client) GetCollateralBalanceDetail(ctx context.Context) (*BalanceDetail, error) {
	return c.getBalanceDetail(types.AssetTypeCollateral, "", polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)
}

func (c *Client) GetAvailableCollateralBalance(ctx context.Context) (decimal.Decimal, error) {
	d, err := c.getBalanceDetail(types.AssetTypeCollateral, "", polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)
	if err != nil {
		return decimal.Zero, err
	}
	return d.AvailableBalance, nil
}

// GetPUSDBalance queries on-chain pUSD balance directly from CollateralToken contract.
func (c *Client) GetPUSDBalance(ctx context.Context) (decimal.Decimal, error) {
	if c.contractInterface == nil {
		return decimal.Zero, fmt.Errorf("contracts not configured")
	}
	info, err := c.contractInterface.GetBalances(ctx, c.funderAddr)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromBigInt(info.PUSDBalance, 0).Div(decimal.New(1, polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)), nil
}

// GetUSDCEBalance queries on-chain USDC.e balance.
func (c *Client) GetUSDCEBalance(ctx context.Context) (decimal.Decimal, error) {
	if c.contractInterface == nil {
		return decimal.Zero, fmt.Errorf("contracts not configured")
	}
	info, err := c.contractInterface.GetBalances(ctx, c.funderAddr)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromBigInt(info.USDCEBalance, 0).Div(decimal.New(1, polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)), nil
}

// GetMultiTokenBalance returns pUSD, USDC.e, USDC, and total balances at once.
func (c *Client) GetMultiTokenBalance(ctx context.Context) (*types.MultiTokenBalance, error) {
	if c.clobClient == nil {
		return nil, fmt.Errorf("CLOB client not configured")
	}
	return c.clobClient.GetMultiTokenBalance(ctx)
}

// GetPositionBalance gets position (conditional) token balance.
func (c *Client) GetPositionBalance(ctx context.Context, tokenID string, opt *BalanceQueryOption) (decimal.Decimal, error) {
	if opt == nil {
		opt = &BalanceQueryOption{Source: DataSourceCLOB}
	}
	if opt.Source == DataSourceOnChain {
		addr := c.funderAddr
		if opt.Address != nil {
			addr = *opt.Address
		}
		return c.getOnChainConditional(ctx, addr, tokenID, polymarketcontracts.CONDITIONAL_TOKEN_DECIMALS, opt.BlockNumber)
	}
	return c.getCLOBBalance(types.AssetTypeConditional, tokenID, polymarketcontracts.CONDITIONAL_TOKEN_DECIMALS)
}

func (c *Client) GetPositionBalanceDetail(ctx context.Context, tokenID string) (*BalanceDetail, error) {
	return c.getBalanceDetail(types.AssetTypeConditional, tokenID, polymarketcontracts.CONDITIONAL_TOKEN_DECIMALS)
}

func (c *Client) GetAvailablePositionBalance(ctx context.Context, tokenID string) (decimal.Decimal, error) {
	d, err := c.getBalanceDetail(types.AssetTypeConditional, tokenID, polymarketcontracts.CONDITIONAL_TOKEN_DECIMALS)
	if err != nil {
		return decimal.Zero, err
	}
	return d.AvailableBalance, nil
}

// === internal ===

func (c *Client) getCLOBBalance(assetType types.AssetType, tokenID string, decimals int32) (decimal.Decimal, error) {
	d, err := c.getBalanceDetail(assetType, tokenID, decimals)
	if err != nil {
		return decimal.Zero, err
	}
	return d.TotalBalance, nil
}

func (c *Client) getBalanceDetail(assetType types.AssetType, tokenID string, decimals int32) (*BalanceDetail, error) {
	params := &types.BalanceAllowanceParams{AssetType: assetType}
	if tokenID != "" {
		params.TokenID = tokenID
	}
	ba, err := c.clobClient.GetBalanceAllowance(params)
	if err != nil {
		return nil, err
	}
	total := ba.Balance.Div(decimal.New(1, decimals))

	locked := decimal.Zero
	oo := &types.OpenOrderParams{}
	if tokenID != "" {
		oo.AssetID = tokenID
	}
	orders, err := c.clobClient.GetOpenOrders(oo, false, "")
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		if assetType == types.AssetTypeCollateral && o.Side == types.OrderSideBuy {
			rem := o.OriginalSize.Sub(o.SizeMatched)
			locked = locked.Add(rem.Mul(o.Price))
		} else if assetType == types.AssetTypeConditional && o.AssetID == tokenID && o.Side == types.OrderSideSell {
			rem := o.OriginalSize.Sub(o.SizeMatched)
			locked = locked.Add(rem)
		}
	}
	avail := total.Sub(locked)
	if avail.LessThan(decimal.Zero) {
		avail = decimal.Zero
	}
	return &BalanceDetail{TotalBalance: total, LockedBalance: locked, AvailableBalance: avail}, nil
}

func (c *Client) getChainBalance(ctx context.Context, addr common.Address, block *big.Int) (decimal.Decimal, error) {
	if c.contractInterface == nil {
		return decimal.Zero, fmt.Errorf("contracts not configured")
	}
	info, err := c.contractInterface.GetBalancesAtBlock(ctx, addr, block)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromBigInt(info.PUSDBalance, 0).Div(decimal.New(1, polymarketcontracts.COLLATERAL_TOKEN_DECIMALS)), nil
}

func (c *Client) getOnChainConditional(ctx context.Context, addr common.Address, tokenID string, decimals int32, block *big.Int) (decimal.Decimal, error) {
	if c.contractInterface == nil {
		return decimal.Zero, fmt.Errorf("contracts not configured")
	}
	n := new(big.Int)
	if strings.HasPrefix(tokenID, "0x") {
		n.SetString(tokenID[2:], 16)
	} else {
		n.SetString(tokenID, 10)
	}
	b, err := c.contractInterface.ConditionalTokens().BalanceOf(&bind.CallOpts{Context: ctx, BlockNumber: block}, addr, n)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromBigInt(b, 0).Div(decimal.New(1, decimals)), nil
}
