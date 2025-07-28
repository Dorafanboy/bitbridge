package main

import (
	"context"
	"log"
	"time"

	"bitbridge/internal/api"
	"bitbridge/internal/bitcoin"
	"bitbridge/internal/contracts"
	"bitbridge/internal/ethereum"
	"bitbridge/internal/fusion"
	"bitbridge/internal/proof"
	"bitbridge/pkg/config"

	"github.com/gin-gonic/gin"
)

func mainV2() {
	log.Println("Starting UTXO-EVM Gateway v2...")
	
	// Load configuration
	cfg := config.Load()
	
	// Initialize WebSocket manager
	wsManager := api.NewWebSocketManager()
	ctx := context.Background()
	wsManager.Start(ctx)
	
	// Initialize services
	var bitcoinService *bitcoin.Service
	var ethereumService *ethereum.Service
	var fusionService *fusion.Service
	var proofService *proof.Service
	var contractsService *contracts.Service
	
	// Initialize Bitcoin service
	if cfg.Bitcoin.RPCUser != "" && cfg.Bitcoin.RPCPassword != "" {
		service, err := bitcoin.NewService(&cfg.Bitcoin)
		if err != nil {
			log.Printf("Warning: Failed to initialize Bitcoin service: %v", err)
		} else {
			bitcoinService = service
			err = bitcoinService.Start()
			if err != nil {
				log.Printf("Warning: Failed to start Bitcoin service: %v", err)
				bitcoinService = nil
			} else {
				log.Println("Bitcoin service initialized successfully")
			}
		}
	} else {
		log.Println("Warning: Bitcoin RPC credentials not provided, Bitcoin functionality disabled")
	}
	
	// Initialize Ethereum and related services
	if cfg.Ethereum.PrivateKey != "" {
		ethClient, err := ethereum.NewClient(ethereum.Config{
			RpcURL:     cfg.Ethereum.RPCEndpoint,
			PrivateKey: cfg.Ethereum.PrivateKey,
			ChainID:    cfg.Ethereum.ChainID,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize Ethereum client: %v", err)
		} else {
			ethereumService = ethereum.NewService(ethereum.ServiceConfig{
				Client:           ethClient,
				UTXORegistryAddr: cfg.Ethereum.UTXORegistryAddr,
				TokenFactoryAddr: cfg.Ethereum.TokenFactoryAddr,
			})
			log.Println("Ethereum service initialized successfully")
			
			// Initialize Fusion+ service if enabled
			if cfg.Fusion.Enabled && cfg.Fusion.APIKey != "" {
				fusionClient := fusion.NewClient(fusion.Config{
					BaseURL: cfg.Fusion.BaseURL,
					APIKey:  cfg.Fusion.APIKey,
					ChainID: cfg.Ethereum.ChainID,
				})
				
				fusionService = fusion.NewService(fusion.ServiceConfig{
					Client:    fusionClient,
					EthClient: ethClient,
					ChainID:   cfg.Ethereum.ChainID,
				})
				log.Println("1inch Fusion+ service initialized successfully")
			} else {
				log.Println("Warning: 1inch Fusion+ service disabled (missing API key or disabled)")
			}
			
			// Initialize contracts service
			contractsService, err = contracts.NewService(contracts.ServiceConfig{
				EthereumClient:  ethClient.GetClient(),
				EthereumConfig:  &cfg.Ethereum,
				ContractAddress: cfg.Ethereum.SPVVerifierAddr,
			})
			if err != nil {
				log.Printf("Warning: Failed to initialize contracts service: %v", err)
			} else {
				log.Println("Smart contracts service initialized successfully")
			}
		}
	} else {
		log.Println("Warning: Ethereum private key not provided, Ethereum functionality disabled")
	}
	
	// Initialize SPV proof service
	if bitcoinService != nil {
		btcClient, err := bitcoin.NewClient(bitcoin.Config{
			Host:     cfg.Bitcoin.RPCHost,
			Port:     cfg.Bitcoin.RPCPort,
			User:     cfg.Bitcoin.RPCUser,
			Password: cfg.Bitcoin.RPCPassword,
			Network:  cfg.Bitcoin.Network,
		})
		if err != nil {
			log.Printf("Warning: Failed to create Bitcoin client for proofs: %v", err)
		} else {
			proofService = proof.NewService(proof.ServiceConfig{
				BitcoinClient:    btcClient,
				RPCClient:        btcClient.GetRPCClient(),
				MinConfirmations: 6,
				MaxCacheSize:     1000,
				CacheExpiration:  24 * time.Hour,
			})
			log.Println("SPV proof service initialized successfully")
		}
	}
	
	// Create API server
	apiServer := api.NewAPIServer(
		bitcoinService,
		ethereumService,
		fusionService,
		proofService,
		contractsService,
		wsManager,
	)
	
	// Setup Gin router
	r := gin.Default()
	
	// Register all routes
	apiServer.RegisterRoutes(r)
	
	log.Printf("Server starting on :%s", cfg.Server.Port)
	log.Println("Available endpoints:")
	log.Println("  GET  /              - API information")
	log.Println("  GET  /health        - Health check")
	log.Println("  GET  /ws            - WebSocket connection")
	log.Println("  GET  /v1/*          - API v1 endpoints")
	log.Println("")
	log.Println("WebSocket topics:")
	log.Println("  bitcoin.blocks      - Bitcoin block events")
	log.Println("  bitcoin.transactions - Bitcoin transaction events")
	log.Println("  ethereum.blocks     - Ethereum block events")
	log.Println("  ethereum.transactions - Ethereum transaction events")
	log.Println("  utxo.events         - UTXO monitoring events")
	log.Println("  proof.generation    - SPV proof generation events")
	log.Println("  swap.events         - Fusion+ swap events")
	log.Println("  contract.events     - Smart contract events")
	
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}