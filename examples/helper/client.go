package helper

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ivanzzeth/ethclient"
	polymarket "github.com/ivanzzeth/polymarket-go"
	polymarketcontracts "github.com/ivanzzeth/polymarket-go-contracts"
	"github.com/ivanzzeth/polymarket-go-contracts/signer"
	"github.com/joho/godotenv"
)

// LoadEnv loads environment variables from .env file
// Call this from the example directory (e.g., examples/01_balance_query)
func LoadEnv() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}
}

// NewClientWithSigner creates a Polymarket client with Safe signer from environment variables
// Requires PRIVATE_KEY and optionally RPC_URL in environment
func NewClientWithSigner(ctx context.Context) (*polymarket.Client, error) {
	// Load private key from environment
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		return nil, fmt.Errorf("PRIVATE_KEY environment variable not set")
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Connect to Polygon network
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://polygon-rpc.com" // Default mainnet RPC
	}

	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}

	// Get chain ID
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Create Safe signer from private key
	safeSigner, err := signer.NewSafeTradingPrivateKeySigner(chainID, ethClient, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Safe signer: %w", err)
	}

	// Create Polymarket client with Safe signer
	client, err := polymarket.NewClient(
		ethClient,
		polymarket.WithContractInterfaceOptions(
			polymarketcontracts.WithSafeSigner(safeSigner),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Polymarket client: %w", err)
	}

	return client, nil
}

// NewClientWithOptions creates a Polymarket client with additional options
// This is useful for examples that need to configure auto-management features
func NewClientWithOptions(ctx context.Context, options ...polymarket.ClientOption) (*polymarket.Client, error) {
	// Load private key from environment
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		return nil, fmt.Errorf("PRIVATE_KEY environment variable not set")
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Connect to Polygon network
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://polygon-rpc.com" // Default mainnet RPC
	}

	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}

	// Get chain ID
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Create Safe signer from private key
	safeSigner, err := signer.NewSafeTradingPrivateKeySigner(chainID, ethClient, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Safe signer: %w", err)
	}

	// Prepend contract interface options with Safe signer
	allOptions := append(
		[]polymarket.ClientOption{
			polymarket.WithContractInterfaceOptions(
				polymarketcontracts.WithSafeSigner(safeSigner),
			),
		},
		options...,
	)

	// Create Polymarket client with all options
	client, err := polymarket.NewClient(ethClient, allOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Polymarket client: %w", err)
	}

	return client, nil
}
