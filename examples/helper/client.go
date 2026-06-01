package helper

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ivanzzeth/ethclient"
	polymarket "github.com/ivanzzeth/polymarket-go"
	polymarketclob "github.com/ivanzzeth/polymarket-go-clob-client/v2"
	polymarketcontracts "github.com/ivanzzeth/polymarket-go-contracts/v2"
	"github.com/ivanzzeth/polymarket-go-contracts/v2/signer"
	"github.com/joho/godotenv"
)

func LoadEnv() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}
}

func NewClientWithSigner(ctx context.Context) (*polymarket.Client, error) {
	return NewClientWithOptions(ctx)
}

func NewClientWithOptions(ctx context.Context, options ...polymarket.ClientOption) (*polymarket.Client, error) {
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		return nil, fmt.Errorf("PRIVATE_KEY environment variable not set")
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://polygon-rpc.com"
	}

	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	safeSigner, err := signer.NewSafeTradingPrivateKeySigner(chainID, ethClient, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Safe signer: %w", err)
	}

	base := []polymarket.ClientOption{
		polymarket.WithContractInterfaceOptions(
			polymarketcontracts.WithV2SafeSigner(safeSigner),
		),
		polymarket.WithClobClientOptions(
			polymarketclob.WithSigner(safeSigner, safeSigner.GetAddress(), polymarketcontracts.SignatureTypePolyGnosisSafe),
		),
	}

	client, err := polymarket.NewClient(ethClient, append(base, options...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Polymarket client: %w", err)
	}

	return client, nil
}
