package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"bitbridge/internal/bitcoin"
	"bitbridge/internal/contracts"
	"bitbridge/internal/ethereum"
	"bitbridge/internal/fusion"
	"bitbridge/internal/proof"
	"bitbridge/pkg/config"
	"bitbridge/pkg/types"

	"github.com/gin-gonic/gin"
)

func main() {
	// Choose API version based on environment variable
	if os.Getenv("API_VERSION") == "v2" {
		mainV2()
		return
	}
	
	log.Println("Starting UTXO-EVM Gateway...")

	// Load configuration
	cfg := config.Load()
	
	// Initialize services
	var ethClient *ethereum.Client
	var ethService *ethereum.Service
	var fusionService *fusion.Service
	var proofService *proof.Service
	var contractsService *contracts.Service
	var bitcoinClient *bitcoin.Client
	
	if cfg.Ethereum.PrivateKey != "" {
		client, err := ethereum.NewClient(ethereum.Config{
			RpcURL:     cfg.Ethereum.RPCEndpoint,
			PrivateKey: cfg.Ethereum.PrivateKey,
			ChainID:    cfg.Ethereum.ChainID,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize Ethereum client: %v", err)
		} else {
			ethClient = client
			ethService = ethereum.NewService(ethereum.ServiceConfig{
				Client:           client,
				UTXORegistryAddr: cfg.Ethereum.UTXORegistryAddr,
				TokenFactoryAddr: cfg.Ethereum.TokenFactoryAddr,
			})
			log.Println("Ethereum client initialized successfully")
			
			// Initialize Fusion+ service if enabled
			if cfg.Fusion.Enabled && cfg.Fusion.APIKey != "" {
				fusionClient := fusion.NewClient(fusion.Config{
					BaseURL: cfg.Fusion.BaseURL,
					APIKey:  cfg.Fusion.APIKey,
					ChainID: cfg.Ethereum.ChainID,
				})
				
				fusionService = fusion.NewService(fusion.ServiceConfig{
					Client:    fusionClient,
					EthClient: client,
					ChainID:   cfg.Ethereum.ChainID,
				})
				log.Println("1inch Fusion+ service initialized successfully")
			} else {
				log.Println("Warning: 1inch Fusion+ service disabled (missing API key or disabled)")
			}
		}
	} else {
		log.Println("Warning: Ethereum private key not provided, Ethereum functionality disabled")
	}

	// Initialize Bitcoin client and proof service
	if cfg.Bitcoin.RPCUser != "" && cfg.Bitcoin.RPCPassword != "" {
		btcClient, err := bitcoin.NewClient(bitcoin.Config{
			Host:     cfg.Bitcoin.RPCHost,
			Port:     cfg.Bitcoin.RPCPort,
			User:     cfg.Bitcoin.RPCUser,
			Password: cfg.Bitcoin.RPCPassword,
			Network:  cfg.Bitcoin.Network,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize Bitcoin client: %v", err)
		} else {
			bitcoinClient = btcClient
			log.Println("Bitcoin client initialized successfully")

			// Initialize SPV proof service
			proofService = proof.NewService(proof.ServiceConfig{
				BitcoinClient:    btcClient,
				RPCClient:        btcClient.GetRPCClient(),
				MinConfirmations: 6,
				MaxCacheSize:     1000,
				CacheExpiration:  24 * time.Hour,
			})
			log.Println("SPV proof service initialized successfully")
		}
	} else {
		log.Println("Warning: Bitcoin RPC credentials not provided, proof generation disabled")
	}

	// Initialize contracts service if Ethereum is available
	if ethClient != nil {
		var err error
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

	r := gin.Default()
	
	r.GET("/health", func(c *gin.Context) {
		status := gin.H{
			"status":    "healthy",
			"service":   "utxo-evm-gateway",
			"ethereum":  ethClient != nil,
			"fusion":    fusionService != nil,
			"bitcoin":   bitcoinClient != nil,
			"spv_proof": proofService != nil,
			"contracts": contractsService != nil,
		}
		
		if ethClient != nil {
			ctx := context.Background()
			if blockNumber, err := ethClient.GetBlockNumber(ctx); err == nil {
				status["ethereum_block"] = blockNumber
			}
		}
		
		c.JSON(http.StatusOK, status)
	})

	// Add Ethereum-specific endpoints
	if ethService != nil {
		r.GET("/ethereum/status", func(c *gin.Context) {
			ctx := context.Background()
			blockNumber, err := ethClient.GetBlockNumber(ctx)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			balance, err := ethClient.GetBalance(ctx, ethClient.GetAddress())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, gin.H{
				"block_number": blockNumber,
				"balance":      balance.String(),
				"address":      ethClient.GetAddress().Hex(),
				"chain_id":     ethClient.GetChainID().Int64(),
			})
		})
	}

	// Add Fusion+ endpoints
	if fusionService != nil {
		r.POST("/fusion/quote", func(c *gin.Context) {
			var req struct {
				TokenFrom   string `json:"token_from" binding:"required"`
				TokenTo     string `json:"token_to" binding:"required"`
				Amount      string `json:"amount" binding:"required"`
				FromAddress string `json:"from_address" binding:"required"`
			}
			
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			ctx := context.Background()
			quote, err := fusionService.GetBestQuote(ctx, req.TokenFrom, req.TokenTo, req.Amount, req.FromAddress)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, quote)
		})
		
		r.POST("/fusion/swap", func(c *gin.Context) {
			var req types.SwapRequest
			
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			ctx := context.Background()
			swap, err := fusionService.SwapUTXOToken(ctx, &req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, swap)
		})
		
		r.POST("/fusion/execute-swap", func(c *gin.Context) {
			var req types.SwapResponse
			
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			ctx := context.Background()
			tx, err := fusionService.ExecuteSwap(ctx, &req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, gin.H{
				"transaction_hash": tx.Hash().Hex(),
				"gas_used":        tx.Gas(),
				"gas_price":       tx.GasPrice().String(),
			})
		})
		
		r.GET("/fusion/tokens/:symbol", func(c *gin.Context) {
			symbol := c.Param("symbol")
			
			address, err := fusionService.GetTokenAddress(symbol)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, gin.H{
				"symbol":  symbol,
				"address": address,
			})
		})
	}

	// Add SPV Proof endpoints
	if proofService != nil {
		r.POST("/proof/generate", func(c *gin.Context) {
			var req proof.ProofRequest
			
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			ctx := context.Background()
			proofResp, err := proofService.GenerateProof(ctx, &req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, proofResp)
		})
		
		r.POST("/proof/verify", func(c *gin.Context) {
			var spvProof proof.SPVProof
			
			if err := c.ShouldBindJSON(&spvProof); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			ctx := context.Background()
			err := proofService.VerifyProof(ctx, &spvProof)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"valid": false,
					"error": err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, gin.H{"valid": true})
		})
		
		r.POST("/proof/contract-format", func(c *gin.Context) {
			var req proof.ProofRequest
			
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			ctx := context.Background()
			contractData, err := proofService.GetProofForContract(ctx, &req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, contractData)
		})
		
		r.POST("/proof/batch", func(c *gin.Context) {
			var requests []*proof.ProofRequest
			
			if err := c.ShouldBindJSON(&requests); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			ctx := context.Background()
			responses, err := proofService.BatchGenerateProofs(ctx, requests)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
					"partial_results": responses,
				})
				return
			}
			
			c.JSON(http.StatusOK, gin.H{"proofs": responses})
		})
		
		r.GET("/proof/cache/stats", func(c *gin.Context) {
			stats := proofService.GetCacheStats()
			c.JSON(http.StatusOK, stats)
		})
		
		r.DELETE("/proof/cache", func(c *gin.Context) {
			proofService.ClearCache()
			c.JSON(http.StatusOK, gin.H{"message": "Cache cleared"})
		})
	}

	// Add Smart Contract endpoints
	if contractsService != nil {
		r.POST("/contracts/deploy", func(c *gin.Context) {
			ctx := context.Background()
			result, err := contractsService.DeployContract(ctx)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, result)
		})
		
		r.POST("/contracts/verify", func(c *gin.Context) {
			var req contracts.VerificationRequest
			
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			if proofService == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SPV proof service not available"})
				return
			}
			
			// Generate SPV proof
			ctx := context.Background()
			proofReq := &proof.ProofRequest{
				TxHash:                req.TxHash,
				OutputIndex:           req.OutputIndex,
				RequiredConfirmations: 6,
			}
			
			proofResp, err := proofService.GenerateProof(ctx, proofReq)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate proof: " + err.Error()})
				return
			}
			
			// Verify on contract
			verifyResp, err := contractsService.VerifyTransaction(ctx, &req, proofResp.Proof)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, verifyResp)
		})
		
		r.POST("/contracts/batch-verify", func(c *gin.Context) {
			var req contracts.BatchVerificationRequest
			
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			if proofService == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SPV proof service not available"})
				return
			}
			
			// Generate SPV proofs for all requests
			ctx := context.Background()
			var proofRequests []*proof.ProofRequest
			
			for _, verifyReq := range req.Requests {
				proofRequests = append(proofRequests, &proof.ProofRequest{
					TxHash:                verifyReq.TxHash,
					OutputIndex:           verifyReq.OutputIndex,
					RequiredConfirmations: 6,
				})
			}
			
			proofResponses, err := proofService.BatchGenerateProofs(ctx, proofRequests)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate proofs: " + err.Error()})
				return
			}
			
			// Extract proofs
			var proofs []*proof.SPVProof
			for _, proofResp := range proofResponses {
				if proofResp != nil {
					proofs = append(proofs, proofResp.Proof)
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate some proofs"})
					return
				}
			}
			
			// Verify on contract
			verifyResp, err := contractsService.BatchVerifyTransactions(ctx, &req, proofs)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, verifyResp)
		})
		
		r.GET("/contracts/info", func(c *gin.Context) {
			info := contractsService.GetContractInfo()
			c.JSON(http.StatusOK, info)
		})
		
		r.GET("/contracts/is-verified/:txhash", func(c *gin.Context) {
			txHash := c.Param("txhash")
			
			ctx := context.Background()
			verified, err := contractsService.IsTransactionVerified(ctx, txHash)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, gin.H{
				"tx_hash":  txHash,
				"verified": verified,
			})
		})
	}

	log.Printf("Server starting on :%s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}