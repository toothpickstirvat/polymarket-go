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
	"github.com/shopspring/decimal"

	polymarketcontracts "github.com/ivanzzeth/polymarket-go-contracts"
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
	contractInterface  *polymarketcontracts.ContractInterface
	clobClient         *polymarketclob.Client
	ethClient          *ethclient.Client // Ethereum client for transaction operations

	// Signer address (the EOA that signs transactions)
	signerAddr common.Address
	// Funder address (the actual address that holds funds)
	funderAddr common.Address

	// Cache for complementary token mappings (key: tokenID string, value: complementary tokenID string)
	complementaryTokenCache sync.Map

	// Cache for condition ID mappings (key: tokenID string, value: conditionID string)
	conditionIDCache sync.Map

	// Auto management fields
	autoRedeemConfig *AutoRedeemConfig
	autoMergeConfig  *AutoMergeConfig
	autoRedeemCancel context.CancelFunc
	autoMergeCancel  context.CancelFunc
	autoMu           sync.Mutex // Protects auto management state
}

type ClientConfig struct {
	RealtimeDataClientOptions []polymarketrealtime.ClientOption
	ContractInterfaceOptions  []polymarketcontracts.ContractInterfaceOption
	ClobClientOptions         []polymarketclob.ClobClientOption
	AutoRedeemConfig          *AutoRedeemConfig
	AutoMergeConfig           *AutoMergeConfig
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

func WithAutoRedeem(config *AutoRedeemConfig) ClientOption {
	return func(c *ClientConfig) {
		c.AutoRedeemConfig = config
	}
}

func WithAutoMerge(config *AutoMergeConfig) ClientOption {
	return func(c *ClientConfig) {
		c.AutoMergeConfig = config
	}
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

	// Start auto management services if configured
	// Auto management runs in background goroutines and can be stopped via Stop methods
	ctx := context.Background()
	if defaultOptions.AutoRedeemConfig != nil || defaultOptions.AutoMergeConfig != nil {
		if err := client.startAutoManagement(ctx, defaultOptions.AutoRedeemConfig, defaultOptions.AutoMergeConfig); err != nil {
			return nil, fmt.Errorf("failed to start auto management: %w", err)
		}
	}

	return client, nil
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

// EthClient returns the Ethereum client
func (c *Client) EthClient() *ethclient.Client {
	return c.ethClient
}

// GetSignerAddress returns the signer's EOA address
func (c *Client) GetSignerAddress() common.Address {
	return c.signerAddr
}

// FunderAddress returns the funder address (the actual address that holds funds)
func (c *Client) FunderAddress() common.Address {
	return c.funderAddr
}

func (c *Client) EnableTrading(ctx context.Context) ([]common.Hash, error) {
	return c.contractInterface.EnableTrading(ctx)
}

// Split splits collateral into conditional tokens for a binary market
// Uses standard Polymarket partition [1, 2] for binary outcomes (Yes/No)
// conditionId: the condition ID as a hex string (e.g., "0x123..." or "123...")
// amount: the amount of USDC collateral to split (in decimal units, e.g., 1.5 for 1.5 USDC)
func (c *Client) Split(ctx context.Context, conditionId string, amount decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	rawAmount := utils.DecimalToRawAmount(amount)
	return c.contractInterface.Split(ctx, common.HexToHash(conditionId), rawAmount)
}

// Merge merges conditional tokens back into collateral for a binary market
// Uses standard Polymarket partition [1, 2] for binary outcomes (Yes/No)
// conditionId: the condition ID as a hex string (e.g., "0x123..." or "123...")
// amount: the amount of tokens to merge (in decimal units, e.g., 1.5 for 1.5 tokens)
func (c *Client) Merge(ctx context.Context, conditionId string, amount decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	rawAmount := utils.DecimalToRawAmount(amount)
	return c.contractInterface.Merge(ctx, common.HexToHash(conditionId), rawAmount)
}

// SplitNegRisk splits collateral into conditional tokens for a NegRisk market
// conditionId: the condition ID as a hex string (e.g., "0x123..." or "123...")
// amount: the amount of USDC collateral to split (in decimal units, e.g., 1.5 for 1.5 USDC)
func (c *Client) SplitNegRisk(ctx context.Context, conditionId string, amount decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	rawAmount := utils.DecimalToRawAmount(amount)
	return c.contractInterface.SplitNegRisk(ctx, common.HexToHash(conditionId), rawAmount)
}

// MergeNegRisk merges conditional tokens back into collateral for a NegRisk market
// conditionId: the condition ID as a hex string (e.g., "0x123..." or "123...")
// amount: the amount of tokens to merge (in decimal units, e.g., 1.5 for 1.5 tokens)
func (c *Client) MergeNegRisk(ctx context.Context, conditionId string, amount decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	rawAmount := utils.DecimalToRawAmount(amount)
	return c.contractInterface.MergeNegRisk(ctx, common.HexToHash(conditionId), rawAmount)
}

// Redeem redeems conditional tokens for a resolved binary market
// Uses standard Polymarket indexSets [1, 2] for binary outcomes (Yes/No)
// conditionId: the condition ID as a hex string (e.g., "0x123..." or "123...")
func (c *Client) Redeem(ctx context.Context, conditionId string) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	return c.contractInterface.Redeem(ctx, common.HexToHash(conditionId))
}

// RedeemNegRisk redeems NegRisk market positions
// conditionId: the condition ID as a hex string (e.g., "0x123..." or "123...")
// amounts: a slice containing the amount to redeem for each outcome (in decimal units)
// For binary NegRisk markets, use a slice of two amounts [yesAmount, noAmount]
func (c *Client) RedeemNegRisk(ctx context.Context, conditionId string, amounts []decimal.Decimal) (common.Hash, error) {
	if err := utils.ValidateConditionId(conditionId); err != nil {
		return common.Hash{}, err
	}
	// Convert decimal amounts to raw amounts
	rawAmounts := make([]*big.Int, len(amounts))
	for i, amount := range amounts {
		rawAmounts[i] = utils.DecimalToRawAmount(amount)
	}
	return c.contractInterface.RedeemNegRisk(ctx, common.HexToHash(conditionId), rawAmounts)
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

	// If error or zero result (not found on chain), try with negRisk exchange
	if err != nil || complementBigInt == nil || complementBigInt.Sign() == 0 {
		negRiskExchange := c.contractInterface.GetNegRisk()
		complementBigInt, err = negRiskExchange.GetComplement(nil, tokenIDBigInt)
		if err != nil {
			return "", fmt.Errorf("failed to get complementary token from contract: %w", err)
		}

		// Check if negRisk also returned zero
		if complementBigInt == nil || complementBigInt.Sign() == 0 {
			return "", fmt.Errorf("tokenID not found on chain (both Exchange and NegRisk returned zero)")
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

// GetConditionIDByTokenID returns the condition ID for a given token ID
// In Polymarket, each position token (YES/NO) is associated with a specific market condition
// This function retrieves the underlying condition ID from a token ID
// Results are cached to avoid repeated contract calls
func (c *Client) GetConditionIDByTokenID(ctx context.Context, tokenID string) (string, error) {
	// Check cache first
	if cached, ok := c.conditionIDCache.Load(tokenID); ok {
		return cached.(string), nil
	}

	// Convert tokenID string to *big.Int
	tokenIDBigInt := new(big.Int)
	tokenIDBigInt, ok := tokenIDBigInt.SetString(tokenID, 10)
	if !ok {
		return "", fmt.Errorf("invalid tokenID format: %s", tokenID)
	}

	// Call Exchange.GetConditionId to retrieve the condition ID for this token
	exchange := c.contractInterface.GetExchange()
	conditionIDHash, err := exchange.GetConditionId(nil, tokenIDBigInt)

	// If error or zero result (not found on chain), try with negRisk exchange
	if err != nil || conditionIDHash == (common.Hash{}) {
		negRiskExchange := c.contractInterface.GetNegRisk()
		conditionIDHash, err = negRiskExchange.GetConditionId(nil, tokenIDBigInt)
		if err != nil {
			return "", fmt.Errorf("failed to get condition ID from contract: %w", err)
		}

		// Check if negRisk also returned zero
		if conditionIDHash == (common.Hash{}) {
			return "", fmt.Errorf("tokenID not found on chain (both Exchange and NegRisk returned zero)")
		}
	}

	// Convert [32]byte hash to hex string
	conditionID := common.BytesToHash(conditionIDHash[:]).Hex()

	// Store in cache
	c.conditionIDCache.Store(tokenID, conditionID)

	return conditionID, nil
}

// ConvertLimitOrderToComplementary converts a limit order to its complementary token order
// This automatically queries the complementary token ID and performs the conversion
// Based on Polymarket's complementary token mechanism:
//   - Buy token @ P  → Sell complementary @ (1-P)
//   - Sell token @ P → Buy complementary @ (1-P)
//
// This allows traders to achieve the same position using whichever side has better liquidity
// For example: Buy YES @ 0.6 = Sell NO @ 0.4
func (c *Client) ConvertLimitOrderToComplementary(ctx context.Context, order *types.UserOrder) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	// Get complementary token ID
	complementaryTokenID, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get complementary token ID: %w", err)
	}

	// Convert the order
	return ConvertToComplementaryOrder(order, complementaryTokenID)
}

// ConvertMarketOrderToComplementary converts a market order to its complementary token order
// This automatically queries the complementary token ID and performs the conversion
// The conversion works by: market order → limit order → convert → limit order → market order
func (c *Client) ConvertMarketOrderToComplementary(ctx context.Context, order *types.UserMarketOrder) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	// Get complementary token ID
	complementaryTokenID, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get complementary token ID: %w", err)
	}

	// Convert the order
	return ConvertMarketOrderToComplementary(order, complementaryTokenID)
}

// ConvertLimitOrderToOppositeSide converts a limit order to the opposite side (BUY ↔ SELL) with optional spread.
// The spread parameter adjusts the price to create a market making spread:
//   - BUY → SELL: price becomes P + spread (sell at higher price)
//   - SELL → BUY: price becomes P - spread (buy at lower price)
//
// Example without spread:
//   - Buy YES @ 0.49  → Sell YES @ 0.49
//   - Sell NO @ 0.51  → Buy NO @ 0.51
//
// Example with spread (0.02):
//   - Buy YES @ 0.49  → Sell YES @ 0.51 (0.49 + 0.02)
//   - Sell NO @ 0.51  → Buy NO @ 0.49 (0.51 - 0.02)
//
// When combined with ConvertLimitOrderToComplementary, you can create market making strategies:
//
//	Buy YES @ 0.49 → (opposite + spread) → Sell YES @ 0.51 → (complementary) → Buy NO @ 0.49
//	Result: Buy YES @ 0.49 + Buy NO @ 0.49 = 0.98 cost, merge to 1.0, profit = 0.02
//
// This is useful for:
//   - Market making with spreads
//   - Creating arbitrage opportunities
//   - Testing order matching with spreads
func (c *Client) ConvertLimitOrderToOppositeSide(order *types.UserOrder, spread decimal.Decimal) (*types.UserOrder, error) {
	return ConvertToOppositeSideOrder(order, spread)
}

// ConvertMarketOrderToOppositeSide converts a market order to the opposite side (BUY ↔ SELL) with optional spread.
// The spread parameter adjusts the price to create a market making spread.
func (c *Client) ConvertMarketOrderToOppositeSide(order *types.UserMarketOrder, spread decimal.Decimal) (*types.UserMarketOrder, error) {
	return ConvertMarketOrderToOppositeSide(order, spread)
}

// ConvertLimitOrderToMatchingSameSide converts a limit order to a matching same-side order on the complementary token.
// This is a convenience method that automatically queries the complementary token ID and creates a matching order.
//
// The conversion keeps the order side (BUY/SELL) the same while switching to the complementary token:
//   - Buy YES @ P  → Buy NO @ (1-P) - spread
//   - Sell YES @ P → Sell NO @ (1-P) + spread
//
// The spread parameter creates market making opportunities:
//   - For BUY orders: reduces the matching order price, creating profit when both filled
//   - For SELL orders: increases the matching order price, creating profit when both filled
//
// Example use case - Market making with guaranteed profit:
//
//	original := Buy YES @ 0.48
//	spread := decimal.NewFromFloat(0.02)
//	matching, _ := client.ConvertLimitOrderToMatchingSameSide(ctx, original, spread)
//	// Result: matching = Buy NO @ 0.50  (1 - 0.50)
//	// Where 0.50 comes from: 0.48 + 0.02 (spread) = 0.50, then 1 - 0.50 = 0.50
//	// Cost: 0.48 + 0.50 = 0.98, merge to 1.0, profit = 0.02 ✅
//
// This creates two orders on the same side that can match each other, triggering CTF operations:
//   - Two BUY orders trigger the split operation (mint YES + NO from collateral)
//   - Two SELL orders trigger the merge operation (burn YES + NO to collateral)
//
// This is useful for:
//   - Market making with guaranteed profit (when both orders fill)
//   - Liquidity provision while earning spreads
//   - Testing CTF split/merge operations
//   - Arbitrage strategies utilizing CTF operations
func (c *Client) ConvertLimitOrderToMatchingSameSide(ctx context.Context, order *types.UserOrder, spread decimal.Decimal) (*types.UserOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	// Get complementary token ID
	complementaryTokenID, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get complementary token ID: %w", err)
	}

	// Convert the order using the base function
	return ConvertToMatchingSameSideOrder(order, complementaryTokenID, spread)
}

// ConvertMarketOrderToMatchingSameSide converts a market order to a matching same-side order on the complementary token.
// This is the market order version of ConvertLimitOrderToMatchingSameSide.
// This automatically queries the complementary token ID and performs the conversion.
func (c *Client) ConvertMarketOrderToMatchingSameSide(ctx context.Context, order *types.UserMarketOrder, spread decimal.Decimal) (*types.UserMarketOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order cannot be nil")
	}

	// Get complementary token ID
	complementaryTokenID, err := c.GetComplementaryTokenID(ctx, order.TokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get complementary token ID: %w", err)
	}

	// Convert the order using the base function
	return ConvertMarketOrderToMatchingSameSide(order, complementaryTokenID, spread)
}

// Close stops all background services and releases resources
// This should be called when the client is no longer needed to ensure graceful shutdown
func (c *Client) Close() error {
	return c.StopAutoManagement()
}
