package fusion

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"bitbridge/internal/ethereum"
	"bitbridge/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type Service struct {
	client    *Client
	ethClient *ethereum.Client
	chainID   int64
}

type ServiceConfig struct {
	Client    *Client
	EthClient *ethereum.Client
	ChainID   int64
}

// Token addresses for common tokens
var (
	// Ethereum mainnet addresses
	WETH_MAINNET = "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"
	USDC_MAINNET = "0xA0b86a33E6411A3AbBCA62af4C7e0b9CcB7bf2C8"
	USDT_MAINNET = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
	
	// Sepolia testnet addresses (placeholder - need actual testnet addresses)
	WETH_SEPOLIA = "0x7b79995e5f793A07Bc00c21412e50Ecae098E7f9"
	USDC_SEPOLIA = "0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8"
)

func NewService(config ServiceConfig) *Service {
	return &Service{
		client:    config.Client,
		ethClient: config.EthClient,
		chainID:   config.ChainID,
	}
}

// SwapUTXOToken swaps UTXO token for another token using 1inch
func (s *Service) SwapUTXOToken(ctx context.Context, req *types.SwapRequest) (*types.SwapResponse, error) {
	// Convert amount from string to verify it's valid
	_, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %s", req.Amount)
	}

	// Get quote first
	_, err := s.client.GetQuote(ctx, req.TokenAddress, req.ToToken, req.Amount, req.FromAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}

	// Parse slippage
	slippage, err := strconv.ParseFloat(req.Slippage, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid slippage: %s", req.Slippage)
	}

	// Get swap transaction data
	swap, err := s.client.GetSwap(ctx, req.TokenAddress, req.ToToken, req.Amount, req.FromAddress, slippage)
	if err != nil {
		return nil, fmt.Errorf("failed to get swap: %w", err)
	}

	// Convert protocols to our format
	protocols := make([][]types.Protocol, len(swap.Protocols))
	for i, protocolGroup := range swap.Protocols {
		protocols[i] = make([]types.Protocol, len(protocolGroup))
		for j, protocol := range protocolGroup {
			protocols[i][j] = types.Protocol{
				Name: protocol.Name,
				Part: fmt.Sprintf("%.2f", protocol.Part),
			}
		}
	}

	return &types.SwapResponse{
		ToAmount: swap.DstAmount,
		Tx: types.SwapTxData{
			From:     swap.Tx.From,
			To:       swap.Tx.To,
			Data:     swap.Tx.Data,
			Value:    swap.Tx.Value,
			GasPrice: swap.Tx.GasPrice,
			Gas:      swap.Tx.Gas,
		},
		Protocols: protocols,
	}, nil
}

// ExecuteSwap executes the swap transaction on Ethereum
func (s *Service) ExecuteSwap(ctx context.Context, swapResponse *types.SwapResponse) (*ethtypes.Transaction, error) {
	// Convert swap data to Ethereum transaction
	to := common.HexToAddress(swapResponse.Tx.To)
	value, ok := new(big.Int).SetString(swapResponse.Tx.Value, 10)
	if !ok {
		value = big.NewInt(0)
	}

	gasLimit, err := strconv.ParseUint(swapResponse.Tx.Gas, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid gas limit: %s", swapResponse.Tx.Gas)
	}

	gasPrice, ok := new(big.Int).SetString(swapResponse.Tx.GasPrice, 10)
	if !ok {
		return nil, fmt.Errorf("invalid gas price: %s", swapResponse.Tx.GasPrice)
	}

	// Get nonce
	nonce, err := s.ethClient.GetNonce(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Create transaction
	data := common.FromHex(swapResponse.Tx.Data)
	tx := ethtypes.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)

	// Sign transaction
	chainID := s.ethClient.GetChainID()
	signer := ethtypes.NewEIP155Signer(chainID)
	signedTx, err := ethtypes.SignTx(tx, signer, s.ethClient.GetPrivateKey())
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	err = s.ethClient.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx, nil
}

// GetBestQuote gets quotes from multiple sources and returns the best one
func (s *Service) GetBestQuote(ctx context.Context, tokenFrom, tokenTo, amount, fromAddress string) (*QuoteResponse, error) {
	// For now, just use 1inch. In the future, could compare with other DEX aggregators
	return s.client.GetQuote(ctx, tokenFrom, tokenTo, amount, fromAddress)
}

// CreateFusionOrder creates a Fusion+ order for better pricing
func (s *Service) CreateFusionOrder(ctx context.Context, req *types.SwapRequest) (*FusionOrder, error) {
	quote := FusionQuote{
		FromTokenAddress: req.TokenAddress,
		ToTokenAddress:   req.ToToken,
		Amount:           req.Amount,
		FromAddress:      req.FromAddress,
		Slippage:         req.Slippage,
		DisableEstimate:  false,
		AllowPartialFill: true,
	}

	// Get Fusion+ quote
	_, err := s.client.GetFusionQuote(ctx, quote)
	if err != nil {
		return nil, fmt.Errorf("failed to get fusion quote: %w", err)
	}

	// Create order (simplified - in real implementation would need proper order creation)
	order := FusionOrder{
		Maker:        req.FromAddress,
		MakerAsset:   req.TokenAddress,
		TakerAsset:   req.ToToken,
		MakingAmount: req.Amount,
		TakingAmount: "0", // Would be calculated from quote
		Salt:         "0", // Would generate proper salt
		Receiver:     req.FromAddress,
		Interactions: "0x",
	}

	return s.client.CreateFusionOrder(ctx, order)
}

// GetTokenAddress returns the contract address for a given token symbol
func (s *Service) GetTokenAddress(symbol string) (string, error) {
	var addresses map[string]string
	
	if s.chainID == 1 { // Mainnet
		addresses = map[string]string{
			"WETH": WETH_MAINNET,
			"USDC": USDC_MAINNET,
			"USDT": USDT_MAINNET,
		}
	} else if s.chainID == 11155111 { // Sepolia
		addresses = map[string]string{
			"WETH": WETH_SEPOLIA,
			"USDC": USDC_SEPOLIA,
		}
	} else {
		return "", fmt.Errorf("unsupported chain ID: %d", s.chainID)
	}

	address, exists := addresses[symbol]
	if !exists {
		return "", fmt.Errorf("unsupported token: %s", symbol)
	}

	return address, nil
}

// EstimateSwapGas estimates gas cost for a swap
func (s *Service) EstimateSwapGas(ctx context.Context, req *types.SwapRequest) (uint64, error) {
	quote, err := s.client.GetQuote(ctx, req.TokenAddress, req.ToToken, req.Amount, req.FromAddress)
	if err != nil {
		return 0, fmt.Errorf("failed to get quote for gas estimation: %w", err)
	}

	if quote.EstimatedGas == "" {
		return 300000, nil // Default gas limit for complex swaps
	}

	gasLimit, err := strconv.ParseUint(quote.EstimatedGas, 10, 64)
	if err != nil {
		return 300000, nil // Fallback to default
	}

	return gasLimit, nil
}