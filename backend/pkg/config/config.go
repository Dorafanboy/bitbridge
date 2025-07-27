package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Bitcoin  BitcoinConfig
	Ethereum EthereumConfig
}

type ServerConfig struct {
	Port string
}

type BitcoinConfig struct {
	RPCHost     string
	RPCPort     int
	RPCUser     string
	RPCPassword string
	Network     string // mainnet, testnet, regtest
}

type EthereumConfig struct {
	RPCEndpoint     string
	ChainID         int64
	PrivateKey      string
	ContractAddress string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Bitcoin: BitcoinConfig{
			RPCHost:     getEnv("BITCOIN_RPC_HOST", "localhost"),
			RPCPort:     getEnvInt("BITCOIN_RPC_PORT", 18332), // testnet default
			RPCUser:     getEnv("BITCOIN_RPC_USER", ""),
			RPCPassword: getEnv("BITCOIN_RPC_PASSWORD", ""),
			Network:     getEnv("BITCOIN_NETWORK", "testnet"),
		},
		Ethereum: EthereumConfig{
			RPCEndpoint:     getEnv("ETHEREUM_RPC_ENDPOINT", "https://sepolia.infura.io/v3/YOUR_PROJECT_ID"),
			ChainID:         getEnvInt64("ETHEREUM_CHAIN_ID", 11155111), // Sepolia
			PrivateKey:      getEnv("ETHEREUM_PRIVATE_KEY", ""),
			ContractAddress: getEnv("CONTRACT_ADDRESS", ""),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}