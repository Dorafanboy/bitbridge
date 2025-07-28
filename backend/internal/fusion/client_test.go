package fusion

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	config := Config{
		BaseURL: "https://api.1inch.dev",
		APIKey:  "test-api-key",
		ChainID: 1,
	}

	client := NewClient(config)
	
	if client.baseURL != config.BaseURL {
		t.Errorf("Expected baseURL %s, got %s", config.BaseURL, client.baseURL)
	}
	
	if client.apiKey != config.APIKey {
		t.Errorf("Expected apiKey %s, got %s", config.APIKey, client.apiKey)
	}
	
	if client.chainID != config.ChainID {
		t.Errorf("Expected chainID %d, got %d", config.ChainID, client.chainID)
	}
}

func TestDefaultBaseURL(t *testing.T) {
	config := Config{
		APIKey:  "test-api-key",
		ChainID: 1,
	}

	client := NewClient(config)
	
	expectedURL := "https://api.1inch.dev"
	if client.baseURL != expectedURL {
		t.Errorf("Expected default baseURL %s, got %s", expectedURL, client.baseURL)
	}
}

func TestGetQuote(t *testing.T) {
	// Skip integration tests if no API key is provided
	t.Skip("Skipping integration tests - requires valid 1inch API key")
	
	// Example test structure for when real API key is available:
	/*
	config := Config{
		APIKey:  "your-api-key-here",
		ChainID: 1,
	}
	
	client := NewClient(config)
	ctx := context.Background()
	
	// Test quote for USDC -> WETH
	quote, err := client.GetQuote(ctx, 
		"0xA0b86a33E6411A3AbBCA62af4C7e0b9CcB7bf2C8", // USDC
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
		"1000000", // 1 USDC (6 decimals)
		"0x742d35Cc6634C0532925a3b8D9C9BB5CB5E8C8dF", // From address
	)
	
	if err != nil {
		t.Fatalf("Failed to get quote: %v", err)
	}
	
	if quote.DstAmount == "" {
		t.Error("Expected non-empty destination amount")
	}
	
	t.Logf("Quote: %s USDC -> %s WETH", quote.SrcAmount, quote.DstAmount)
	*/
}

func TestGetSwap(t *testing.T) {
	// Skip integration tests if no API key is provided
	t.Skip("Skipping integration tests - requires valid 1inch API key")
	
	// Example test structure for when real API key is available:
	/*
	config := Config{
		APIKey:  "your-api-key-here",
		ChainID: 1,
	}
	
	client := NewClient(config)
	ctx := context.Background()
	
	// Test swap for USDC -> WETH
	swap, err := client.GetSwap(ctx,
		"0xA0b86a33E6411A3AbBCA62af4C7e0b9CcB7bf2C8", // USDC
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH  
		"1000000", // 1 USDC (6 decimals)
		"0x742d35Cc6634C0532925a3b8D9C9BB5CB5E8C8dF", // From address
		1.0, // 1% slippage
	)
	
	if err != nil {
		t.Fatalf("Failed to get swap: %v", err)
	}
	
	if swap.Tx.To == "" {
		t.Error("Expected non-empty transaction recipient")
	}
	
	if swap.Tx.Data == "" {
		t.Error("Expected non-empty transaction data")
	}
	
	t.Logf("Swap transaction to: %s", swap.Tx.To)
	*/
}