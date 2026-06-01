package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ivanzzeth/ethclient"
	polymarketclob "github.com/ivanzzeth/polymarket-go-clob-client/v2"
	clobconst "github.com/ivanzzeth/polymarket-go-clob-client/v2/constants"
	"github.com/ivanzzeth/polymarket-go-clob-client/v2/types"
	"github.com/ivanzzeth/polymarket-go-order-utils/v2/pkg/builder"
	"github.com/shopspring/decimal"

	polymarketcontracts "github.com/ivanzzeth/polymarket-go-contracts/v2"
	polymarketdata "github.com/ivanzzeth/polymarket-go-data-client"
	polymarketgamma "github.com/ivanzzeth/polymarket-go-gamma-client"
	polymarketrealtime "github.com/ivanzzeth/polymarket-go-real-time-data-client"

	"github.com/ivanzzeth/polymarket-go/internal/utils"
)

type Client struct {
	httpClient         *http.Client
	gammaClient        *polymarketgamma.Client
	dataClient         *polymarketdata.Client
	realtimeDataClient *polymarketrealtime.Client
	contractInterface  *polymarketcontracts.ContractInterfaceV2
	clobClient         *polymarketclob.Client
	ethClient          *ethclient.Client

	signerAddr common.Address
	funderAddr common.Address

	complementaryTokenCache sync.Map
	conditionIDCache        sync.Map

	autoRedeemConfig *AutoRedeemConfig
	autoMergeConfig  *AutoMergeConfig
	autoRedeemCancel context.CancelFunc
	autoMergeCancel  context.CancelFunc
	autoMu           sync.Mutex
}

type ClientConfig struct {
	RealtimeDataClientOptions []polymarketrealtime.ClientOption
	ContractInterfaceOptions  []polymarketcontracts.ContractInterfaceV2Option
	ClobClientOptions         []polymarketclob.ClobClientOption
	OrderSigner               builder.Signer
	AutoRedeemConfig          *AutoRedeemConfig
	AutoMergeConfig           *AutoMergeConfig
}

type ClientOption func(c *ClientConfig)

func WithRealTimeOptions(options ...polymarketrealtime.ClientOption) ClientOption {
	return func(c *ClientConfig) { c.RealtimeDataClientOptions = options }
}

func WithContractInterfaceOptions(options ...polymarketcontracts.ContractInterfaceV2Option) ClientOption {
	return func(c *ClientConfig) { c.ContractInterfaceOptions = options }
}

func WithClobClientOptions(options ...polymarketclob.ClobClientOption) ClientOption {
	return func(c *ClientConfig) { c.ClobClientOptions = options }
}

func WithAutoRedeem(config *AutoRedeemConfig) ClientOption {
	return func(c *ClientConfig) { c.AutoRedeemConfig = config }
}

func WithAutoMerge(config *AutoMergeConfig) ClientOption {
	return func(c *ClientConfig) { c.AutoMergeConfig = config }
}

func NewClient(ethclient *ethclient.Client, options ...ClientOption) (*Client, error) {
	defaultOptions := &ClientConfig{}
	for _, opFn := range options {
		opFn(defaultOptions)
	}

	chainID, err := ethclient.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	httpClient := http.DefaultClient

	gammaClient := polymarketgamma.NewClient(httpClient)
	dataClient, err := polymarketdata.NewClient(httpClient)
	if err != nil {
		return nil, err
	}

	realtimeDataClient := polymarketrealtime.New(defaultOptions.RealtimeDataClientOptions...)

	// Default contract config resolves V2 addresses for the chain
	contractConfig := polymarketcontracts.GetContractConfig(chainID)

	// Create V2 contract interface — NewContractInterface accepts V2ContractInterfaceOptions
	contractInterface, err := polymarketcontracts.NewContractInterfaceV2(
		ethclient, contractConfig, nil, chainID, defaultOptions.ContractInterfaceOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create V2 contract interface: %w", err)
	}

	var (
		orderSigner builder.Signer
		signerAddr  common.Address
		funderAddr  common.Address
	)
	switch contractInterface.GetSignatureType() {
	case polymarketcontracts.SignatureTypeEOA:
		orderSigner = contractInterface.GetEOATradingSigner()
		signerAddr = orderSigner.GetAddress()
		funderAddr = signerAddr
	case polymarketcontracts.SignatureTypePolyGnosisSafe:
		orderSigner = contractInterface.GetSafeTradingSigner()
		signerAddr = orderSigner.GetAddress()
		funderAddr, err = contractInterface.GetSafeAddress(signerAddr)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported signature type")
	}

	defaultOptions.ClobClientOptions = append(defaultOptions.ClobClientOptions,
		polymarketclob.WithSigner(orderSigner, funderAddr, contractInterface.GetSignatureType()),
	)

	// Attach V2 contracts to CLOB client for on-chain ops (wrap/unwrap pUSD, EnableTrading)
	defaultOptions.ClobClientOptions = append(defaultOptions.ClobClientOptions,
		polymarketclob.WithContractsV2(contractInterface),
	)

	clobClient, err := polymarketclob.NewClient(clobconst.CLOB_API_URL, chainID.Int64(), defaultOptions.ClobClientOptions...)
	if err != nil {
		return nil, err
	}

	_, err = clobClient.CreateOrDeriveApiKey()
	if err != nil {
		return nil, err
	}

	client := &Client{
		httpClient:         httpClient,
		gammaClient:        gammaClient,
		dataClient:         dataClient,
		realtimeDataClient: realtimeDataClient,
		contractInterface:  contractInterface,
		clobClient:         clobClient,
		ethClient:          ethclient,
		signerAddr:         signerAddr,
		funderAddr:         funderAddr,
	}

	ctx := context.Background()
	if defaultOptions.AutoRedeemConfig != nil || defaultOptions.AutoMergeConfig != nil {
		if err := client.startAutoManagement(ctx, defaultOptions.AutoRedeemConfig, defaultOptions.AutoMergeConfig); err != nil {
			return nil, fmt.Errorf("failed to start auto management: %w", err)
		}
	}

	return client, nil
}

// GammaClient returns the gamma client
func (c *Client) GammaClient() *polymarketgamma.Client { return c.gammaClient }

// DataClient returns the data client
func (c *Client) DataClient() *polymarketdata.Client { return c.dataClient }

// RealtimeDataClient returns the realtime data client
func (c *Client) RealtimeDataClient() *polymarketrealtime.Client { return c.realtimeDataClient }

// ContractInterface returns the V2 contract interface
func (c *Client) ContractInterface() *polymarketcontracts.ContractInterfaceV2 {
	return c.contractInterface
}

// ClobClient returns the V2 CLOB client
func (c *Client) ClobClient() *polymarketclob.Client { return c.clobClient }

// EthClient returns the Ethereum client
func (c *Client) EthClient() *ethclient.Client { return c.ethClient }

// GetSignerAddress returns the signer's EOA address
func (c *Client) GetSignerAddress() common.Address { return c.signerAddr }

// FunderAddress returns the funder address
func (c *Client) FunderAddress() common.Address { return c.funderAddr }

// EnableTrading performs one-time V2 contract approvals (pUSD allowances + CTF approvals).
// In V2 this approves pUSD for ExchangeV2, NegRiskExchangeV2, CfCollateralAdapter,
// and sets CTF approval for ExchangeV2 and NegRiskExchangeV2.
func (c *Client) EnableTrading(ctx context.Context) ([]common.Hash, error) {
	return c.contractInterface.EnableTrading(ctx)
}

// Split splits pUSD collateral into conditional tokens via V2 CfCollateralAdapter.
func (c *Client) Split(ctx context.Context, conditionId string, amount decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	return c.contractInterface.Split(ctx, common.HexToHash(conditionId), utils.DecimalToRawAmount(amount))
}

// Merge merges conditional tokens back into pUSD via V2 CfCollateralAdapter.
func (c *Client) Merge(ctx context.Context, conditionId string, amount decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	return c.contractInterface.Merge(ctx, common.HexToHash(conditionId), utils.DecimalToRawAmount(amount))
}

// SplitNegRisk splits pUSD into conditional tokens for NegRisk via V2 adapter.
func (c *Client) SplitNegRisk(ctx context.Context, conditionId string, amount decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	return c.contractInterface.SplitNegRisk(ctx, common.HexToHash(conditionId), utils.DecimalToRawAmount(amount))
}

// MergeNegRisk merges conditional tokens into pUSD for NegRisk via V2 adapter.
func (c *Client) MergeNegRisk(ctx context.Context, conditionId string, amount decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	return c.contractInterface.MergeNegRisk(ctx, common.HexToHash(conditionId), utils.DecimalToRawAmount(amount))
}

// Redeem redeems V2 conditional tokens for a resolved binary market.
func (c *Client) Redeem(ctx context.Context, conditionId string) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	return c.contractInterface.Redeem(ctx, common.HexToHash(conditionId))
}

// RedeemNegRisk redeems NegRisk market positions.
func (c *Client) RedeemNegRisk(ctx context.Context, conditionId string, amounts []decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	raw := make([]*big.Int, len(amounts))
	for i, a := range amounts {
		raw[i] = utils.DecimalToRawAmount(a)
	}
	return c.contractInterface.RedeemNegRisk(ctx, common.HexToHash(conditionId), raw)
}

// DeploySafe deploys a Gnosis Safe wallet for the configured signer.
func (c *Client) DeploySafe() (safeProxy common.Address, txHash common.Hash, err error) {
	if c.contractInterface.GetSignatureType() != polymarketcontracts.SignatureTypePolyGnosisSafe {
		return common.Address{}, common.Hash{}, fmt.Errorf("DeploySafe requires Safe signature type")
	}
	return c.contractInterface.DeploySafe(c.contractInterface.GetSafeTradingSigner())
}

// GetComplementaryTokenID returns the complementary token ID (YES ↔ NO).
// Queries the CLOB market API to find the other token in the same condition.
func (c *Client) GetComplementaryTokenID(ctx context.Context, tokenID string) (string, error) {
	if cached, ok := c.complementaryTokenCache.Load(tokenID); ok {
		return cached.(string), nil
	}

	raw, err := c.clobClient.GetMarketByToken(tokenID)
	if err != nil {
		return "", fmt.Errorf("failed to get market by token: %w", err)
	}

	var marketResp struct {
		ConditionID string `json:"condition_id"`
		Tokens      []struct {
			TokenID string `json:"token_id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(raw, &marketResp); err != nil {
		return "", fmt.Errorf("failed to parse market response: %w", err)
	}

	if len(marketResp.Tokens) == 0 {
		return "", fmt.Errorf("no tokens found for token %s", tokenID)
	}

	var comp string
	for _, t := range marketResp.Tokens {
		if t.TokenID != tokenID {
			comp = t.TokenID
			break
		}
	}
	if comp == "" {
		return "", fmt.Errorf("no complementary token found for %s", tokenID)
	}

	c.complementaryTokenCache.Store(tokenID, comp)
	c.complementaryTokenCache.Store(comp, tokenID)
	return comp, nil
}

// GetConditionIDByTokenID returns the condition ID for a given token ID.
func (c *Client) GetConditionIDByTokenID(ctx context.Context, tokenID string) (string, error) {
	if cached, ok := c.conditionIDCache.Load(tokenID); ok {
		return cached.(string), nil
	}

	raw, err := c.clobClient.GetMarketByToken(tokenID)
	if err != nil {
		return "", fmt.Errorf("failed to get market by token: %w", err)
	}

	var d struct {
		ConditionID string `json:"condition_id"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return "", fmt.Errorf("failed to parse: %w", err)
	}
	if d.ConditionID == "" {
		return "", fmt.Errorf("empty condition_id for token %s", tokenID)
	}

	c.conditionIDCache.Store(tokenID, d.ConditionID)
	return d.ConditionID, nil
}

// === Order conversion ===

func (c *Client) ConvertLimitOrderToComplementary(ctx context.Context, order *types.UserOrder) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	id, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, err
	}
	return ConvertToComplementaryOrder(order, id)
}

func (c *Client) ConvertMarketOrderToComplementary(ctx context.Context, order *types.UserMarketOrder) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	id, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, err
	}
	return ConvertMarketOrderToComplementary(order, id)
}

func (c *Client) ConvertLimitOrderToOppositeSide(order *types.UserOrder, spread decimal.Decimal) (*types.UserOrder, error) {
	return ConvertToOppositeSideOrder(order, spread)
}

func (c *Client) ConvertMarketOrderToOppositeSide(order *types.UserMarketOrder, spread decimal.Decimal) (*types.UserMarketOrder, error) {
	return ConvertMarketOrderToOppositeSide(order, spread)
}

func (c *Client) ConvertLimitOrderToMatchingSameSide(ctx context.Context, order *types.UserOrder, spread decimal.Decimal) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	id, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, err
	}
	return ConvertToMatchingSameSideOrder(order, id, spread)
}

func (c *Client) ConvertMarketOrderToMatchingSameSide(ctx context.Context, order *types.UserMarketOrder, spread decimal.Decimal) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}
	id, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, err
	}
	return ConvertMarketOrderToMatchingSameSide(order, id, spread)
}

// Close stops all background services
func (c *Client) Close() error { return c.StopAutoManagement() }
