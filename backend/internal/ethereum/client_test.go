package ethereum

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	// Test with invalid private key
	config := Config{
		RpcURL:     "https://sepolia.infura.io/v3/test",
		PrivateKey: "invalid_key",
		ChainID:    11155111,
	}

	_, err := NewClient(config)
	if err == nil {
		t.Error("Expected error with invalid private key")
	}
}

func TestClientMethods(t *testing.T) {
	// Skip integration tests if no valid config is provided
	t.Skip("Skipping integration tests - requires valid Ethereum RPC endpoint and private key")
	
	// Example test structure for when real config is available:
	/*
	config := Config{
		RpcURL:     "https://sepolia.infura.io/v3/YOUR_PROJECT_ID",
		PrivateKey: "your_private_key_here",
		ChainID:    11155111,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test get balance
	balance, err := client.GetBalance(ctx, client.GetAddress())
	if err != nil {
		t.Errorf("Failed to get balance: %v", err)
	}
	
	t.Logf("Account balance: %s", balance.String())

	// Test get block number
	blockNumber, err := client.GetBlockNumber(ctx)
	if err != nil {
		t.Errorf("Failed to get block number: %v", err)
	}
	
	t.Logf("Current block number: %d", blockNumber)
	*/
}