package polymarket

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ivanzzeth/ethclient"
	polymarketclob "github.com/ivanzzeth/polymarket-go-clob-client"
	clobconst "github.com/ivanzzeth/polymarket-go-clob-client/constants"
	"github.com/ivanzzeth/polymarket-go-clob-client/types"
	"github.com/ivanzzeth/polymarket-go-order-utils/pkg/builder"

	polymarketcontracts "github.com/ivanzzeth/polymarket-go-contracts"
	polymarketdata "github.com/ivanzzeth/polymarket-go-data-client"
	polymarketgamma "github.com/ivanzzeth/polymarket-go-gamma-client"
	polymarketrealtime "github.com/ivanzzeth/polymarket-go-real-time-data-client"
)

type Client struct {
	gammaClient        *polymarketgamma.Client
	dataClient         *polymarketdata.Client
	realtimeDataClient *polymarketrealtime.Client
	contractInterface  *polymarketcontracts.ContractInterface
	clobClient         *polymarketclob.Client

	// Cache for complementary token mappings (key: tokenID string, value: complementary tokenID string)
	complementaryTokenCache sync.Map
}

type ClientConfig struct {
	RealtimeDataClientOptions []polymarketrealtime.ClientOption
	ContractInterfaceOptions  []polymarketcontracts.ContractInterfaceOption
	ClobClientOptions         []polymarketclob.ClobClientOption
}

type ClientOption func(c *ClientConfig)

func WithRealTimeOptions(options ...polymarketrealtime.ClientOption) ClientOption {
	return func(c *ClientConfig) {
		c.RealtimeDataClientOptions = options
	}
}

func WithContractInterfaceOptions(options ...polymarketcontracts.ContractInterfaceOption) ClientOption {
	return func(c *ClientConfig) {
		c.ContractInterfaceOptions = options
	}
}

func WithClobClientOptions(options ...polymarketclob.ClobClientOption) ClientOption {
	return func(c *ClientConfig) {
		c.ClobClientOptions = options
	}
}

func NewClient(ethclient ethclient.EthClientInterface, options ...ClientOption) (*Client, error) {
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
	contractInterface, err := polymarketcontracts.NewContractInterface(ethclient, defaultOptions.ContractInterfaceOptions...)
	if err != nil {
		return nil, err
	}

	var (
		orderSigner builder.Signer
		signerAddr  common.Address
		funderAddr  common.Address
	)
	switch contractInterface.GetSignatureType() {
	case polymarketcontracts.SignatureTypeEOA:
		orderSigner = contractInterface.GetEOATradingSigner()
		signerAddr = contractInterface.GetEOATradingSigner().GetAddress()
		funderAddr = signerAddr
	case polymarketcontracts.SignatureTypePolyGnosisSafe:
		orderSigner = contractInterface.GetSafeTradingSigner()
		signerAddr = contractInterface.GetSafeTradingSigner().GetAddress()
		funderAddr, err = contractInterface.GetSafeAddress(signerAddr)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported signature type")
	}

	defaultOptions.ClobClientOptions = append(defaultOptions.ClobClientOptions, polymarketclob.WithSigner(orderSigner, funderAddr, contractInterface.GetSignatureType()))

	clobClient, err := polymarketclob.NewClient(clobconst.CLOB_API_URL, chainID.Int64(), defaultOptions.ClobClientOptions...)
	if err != nil {
		return nil, err
	}

	_, err = clobClient.CreateOrDeriveApiKey()
	if err != nil {
		return nil, err
	}

	return &Client{
		gammaClient:        gammaClient,
		dataClient:         dataClient,
		realtimeDataClient: realtimeDataClient,
		contractInterface:  contractInterface,
		clobClient:         clobClient,
	}, nil
}

// GammaClient returns the gamma client
func (c *Client) GammaClient() *polymarketgamma.Client {
	return c.gammaClient
}

// DataClient returns the data client
func (c *Client) DataClient() *polymarketdata.Client {
	return c.dataClient
}

// RealtimeDataClient returns the realtime data client
func (c *Client) RealtimeDataClient() *polymarketrealtime.Client {
	return c.realtimeDataClient
}

// ContractInterface returns the contract interface
func (c *Client) ContractInterface() *polymarketcontracts.ContractInterface {
	return c.contractInterface
}

// ClobClient returns the CLOB client
func (c *Client) ClobClient() *polymarketclob.Client {
	return c.clobClient
}

func (c *Client) EnableTrading(ctx context.Context) ([]common.Hash, error) {
	return c.contractInterface.EnableTrading(ctx)
}

// Split splits collateral into conditional tokens for a binary market
// Uses standard Polymarket partition [1, 2] for binary outcomes (Yes/No)
func (c *Client) Split(ctx context.Context, conditionId common.Hash, amount *big.Int) (common.Hash, error) {
	return c.contractInterface.Split(ctx, conditionId, amount)
}

// Merge merges conditional tokens back into collateral for a binary market
// Uses standard Polymarket partition [1, 2] for binary outcomes (Yes/No)
func (c *Client) Merge(ctx context.Context, conditionId common.Hash, amount *big.Int) (common.Hash, error) {
	return c.contractInterface.Merge(ctx, conditionId, amount)
}

// Redeem redeems conditional tokens for a resolved binary market
// Uses standard Polymarket indexSets [1, 2] for binary outcomes (Yes/No)
func (c *Client) Redeem(ctx context.Context, conditionId common.Hash) (common.Hash, error) {
	return c.contractInterface.Redeem(ctx, conditionId)
}

// RedeemNegRisk redeems NegRisk market positions
// amounts is a slice containing the amount to redeem for each outcome
// For binary NegRisk markets, use a slice of two amounts [yesAmount, noAmount]
func (c *Client) RedeemNegRisk(ctx context.Context, conditionId common.Hash, amounts []*big.Int) (common.Hash, error) {
	return c.contractInterface.RedeemNegRisk(ctx, conditionId, amounts)
}

// DeploySafe deploys a Gnosis Safe wallet for the configured signer
// Returns the Safe proxy address and the deployment transaction hash
func (c *Client) DeploySafe() (safeProxy common.Address, txHash common.Hash, err error) {
	return c.contractInterface.DeploySafe()
}

// GetComplementaryTokenID returns the complementary token ID for a given position token
// In Polymarket's binary markets, every YES token has a corresponding NO token as its complement
// For example: if tokenID is YES, this returns the NO token ID, and vice versa
// Results are cached to avoid repeated contract calls
func (c *Client) GetComplementaryTokenID(ctx context.Context, tokenID string) (string, error) {
	// Check cache first
	if cached, ok := c.complementaryTokenCache.Load(tokenID); ok {
		return cached.(string), nil
	}

	// Convert tokenID string to *big.Int
	tokenIDBigInt := new(big.Int)
	tokenIDBigInt, ok := tokenIDBigInt.SetString(tokenID, 10)
	if !ok {
		return "", fmt.Errorf("invalid tokenID format: %s", tokenID)
	}

	// Call Exchange.GetComplement
	exchange := c.contractInterface.GetExchange()
	complementBigInt, err := exchange.GetComplement(nil, tokenIDBigInt)
	if err != nil {
		// Try with negRisk exchange
		negRiskExchange := c.contractInterface.GetNegRisk()
		complementBigInt, err = negRiskExchange.GetComplement(nil, tokenIDBigInt)
		if err != nil {
			return "", fmt.Errorf("failed to get complementary token from contract: %w", err)
		}
	}

	// Convert result *big.Int to string
	complementaryTokenID := complementBigInt.String()

	// Store both directions in cache (tokenID -> complement and complement -> tokenID)
	// This is because if A's complement is B, then B's complement is A
	c.complementaryTokenCache.Store(tokenID, complementaryTokenID)
	c.complementaryTokenCache.Store(complementaryTokenID, tokenID)

	return complementaryTokenID, nil
}

// ConvertLimitOrder converts a limit order to its complementary side
// This automatically queries the complementary token ID and performs the conversion
// Based on Polymarket's complementary token mechanism:
//   - Buy token @ P  → Sell complementary @ (1-P)
//   - Sell token @ P → Buy complementary @ (1-P)
//
// This allows traders to achieve the same position using whichever side has better liquidity
// For example: Buy YES @ 0.6 = Sell NO @ 0.4
func (c *Client) ConvertLimitOrder(ctx context.Context, order *types.UserOrder) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	// Get complementary token ID
	complementaryTokenID, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get complementary token ID: %w", err)
	}

	// Convert the order
	return convertOrder(order, complementaryTokenID)
}

// ConvertMarketOrder converts a market order to its complementary side
// This automatically queries the complementary token ID and performs the conversion
// The conversion works by: market order → limit order → convert → limit order → market order
func (c *Client) ConvertMarketOrder(ctx context.Context, order *types.UserMarketOrder) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	// Get complementary token ID
	complementaryTokenID, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get complementary token ID: %w", err)
	}

	// Convert the order
	return convertMarketOrder(order, complementaryTokenID)
}
