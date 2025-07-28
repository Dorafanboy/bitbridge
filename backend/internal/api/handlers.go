package api

import (
	"strconv"
	"time"

	"bitbridge/internal/bitcoin"
	"bitbridge/internal/contracts"
	"bitbridge/internal/ethereum"
	"bitbridge/internal/fusion"
	"bitbridge/internal/proof"

	"github.com/gin-gonic/gin"
)

// APIServer represents the main API server
type APIServer struct {
	bitcoinService   *bitcoin.Service
	ethereumService  *ethereum.Service
	fusionService    *fusion.Service
	proofService     *proof.Service
	contractsService *contracts.Service
	wsManager        *WebSocketManager
	startTime        time.Time
}

// NewAPIServer creates a new API server instance
func NewAPIServer(
	bitcoinService *bitcoin.Service,
	ethereumService *ethereum.Service,
	fusionService *fusion.Service,
	proofService *proof.Service,
	contractsService *contracts.Service,
	wsManager *WebSocketManager,
) *APIServer {
	return &APIServer{
		bitcoinService:   bitcoinService,
		ethereumService:  ethereumService,
		fusionService:    fusionService,
		proofService:     proofService,
		contractsService: contractsService,
		wsManager:        wsManager,
		startTime:        time.Now(),
	}
}

// RegisterRoutes registers all API routes
func (s *APIServer) RegisterRoutes(r *gin.Engine) {
	// Apply global middleware
	r.Use(CORSMiddleware())
	r.Use(APIVersionMiddleware())
	r.Use(ContentTypeMiddleware())
	r.Use(RequestLoggingMiddleware())
	r.Use(SecurityMiddleware())
	r.Use(MetricsMiddleware())
	r.Use(ValidationMiddleware())
	r.Use(RateLimitMiddleware(100)) // 100 requests per minute
	
	// API info endpoint
	r.GET("/", APIInfo)
	r.GET("/info", APIInfo)
	
	// Health check
	r.GET("/health", s.healthCheck)
	r.GET("/status", s.healthCheck)
	
	// WebSocket endpoint
	r.GET("/ws", s.wsManager.HandleWebSocket)
	
	// API version 1 routes
	v1 := r.Group("/v1")
	{
		s.registerBitcoinRoutes(v1)
		s.registerEthereumRoutes(v1)
		s.registerFusionRoutes(v1)
		s.registerProofRoutes(v1)
		s.registerContractRoutes(v1)
		s.registerUtilityRoutes(v1)
	}
	
	// Legacy routes (for backward compatibility)
	s.registerLegacyRoutes(r)
}

// registerBitcoinRoutes registers Bitcoin-related routes
func (s *APIServer) registerBitcoinRoutes(rg *gin.RouterGroup) {
	if s.bitcoinService == nil {
		return
	}
	
	bitcoin := rg.Group("/bitcoin")
	{
		bitcoin.GET("/status", s.bitcoinStatus)
		bitcoin.GET("/network-info", s.bitcoinNetworkInfo)
		bitcoin.POST("/generate-address", s.generateBitcoinAddress)
		bitcoin.GET("/addresses", s.getBitcoinAddresses)
		bitcoin.GET("/address/:address/utxos", s.getAddressUTXOs)
		bitcoin.GET("/address/:address/balance", s.getAddressBalance)
		bitcoin.GET("/utxo/:txid/:vout", s.getUTXO)
		bitcoin.GET("/utxos", s.getAllUTXOs)
		bitcoin.POST("/validate-address", s.validateBitcoinAddress)
		bitcoin.POST("/watch-address", s.watchBitcoinAddress)
	}
}

// registerEthereumRoutes registers Ethereum-related routes
func (s *APIServer) registerEthereumRoutes(rg *gin.RouterGroup) {
	if s.ethereumService == nil {
		return
	}
	
	ethereum := rg.Group("/ethereum")
	{
		ethereum.GET("/status", s.ethereumStatus)
		ethereum.GET("/balance/:address", s.getEthereumBalance)
		ethereum.GET("/block-number", s.getEthereumBlockNumber)
		ethereum.GET("/gas-price", s.getEthereumGasPrice)
		ethereum.POST("/send-transaction", s.sendEthereumTransaction)
		ethereum.GET("/transaction/:hash", s.getEthereumTransaction)
	}
}

// registerFusionRoutes registers 1inch Fusion+ routes
func (s *APIServer) registerFusionRoutes(rg *gin.RouterGroup) {
	if s.fusionService == nil {
		return
	}
	
	fusion := rg.Group("/fusion")
	{
		fusion.POST("/quote", s.getFusionQuote)
		fusion.POST("/swap", s.prepareFusionSwap)
		fusion.POST("/execute-swap", s.executeFusionSwap)
		fusion.GET("/tokens/:symbol", s.getFusionToken)
		fusion.GET("/tokens", s.getFusionTokens)
		fusion.GET("/orders/:address", s.getFusionOrders)
		fusion.POST("/cancel-order", s.cancelFusionOrder)
	}
}

// registerProofRoutes registers SPV proof routes
func (s *APIServer) registerProofRoutes(rg *gin.RouterGroup) {
	if s.proofService == nil {
		return
	}
	
	proof := rg.Group("/proof")
	{
		proof.POST("/generate", s.generateProof)
		proof.POST("/verify", s.verifyProof)
		proof.POST("/contract-format", s.getProofForContract)
		proof.POST("/batch", s.batchGenerateProofs)
		proof.GET("/cache/stats", s.getProofCacheStats)
		proof.DELETE("/cache", s.clearProofCache)
		proof.GET("/merkle-tree/:txid", s.getMerkleTree)
		proof.POST("/validate-merkle", s.validateMerkleProof)
	}
}

// registerContractRoutes registers smart contract routes
func (s *APIServer) registerContractRoutes(rg *gin.RouterGroup) {
	if s.contractsService == nil {
		return
	}
	
	contracts := rg.Group("/contracts")
	{
		contracts.POST("/deploy", s.deployContract)
		contracts.POST("/verify", s.verifyTransactionOnContract)
		contracts.POST("/batch-verify", s.batchVerifyTransactions)
		contracts.GET("/info", s.getContractInfo)
		contracts.GET("/is-verified/:txhash", s.isTransactionVerified)
		contracts.GET("/gas-estimate", s.estimateContractGas)
		contracts.GET("/events", s.getContractEvents)
	}
}

// registerUtilityRoutes registers utility routes
func (s *APIServer) registerUtilityRoutes(rg *gin.RouterGroup) {
	utils := rg.Group("/utils")
	{
		utils.POST("/validate-bitcoin-address", s.validateBitcoinAddressUtil)
		utils.POST("/validate-ethereum-address", s.validateEthereumAddressUtil)
		utils.POST("/validate-transaction-hash", s.validateTransactionHashUtil)
		utils.GET("/network-status", s.getNetworkStatus)
		utils.GET("/system-info", s.getSystemInfo)
		utils.GET("/websocket-stats", s.getWebSocketStats)
	}
}

// registerLegacyRoutes registers legacy routes for backward compatibility
func (s *APIServer) registerLegacyRoutes(r *gin.Engine) {
	// Legacy routes that don't have /v1 prefix
	if s.ethereumService != nil {
		r.GET("/ethereum/status", s.ethereumStatus)
	}
	
	if s.fusionService != nil {
		r.POST("/fusion/quote", s.getFusionQuote)
		r.POST("/fusion/swap", s.prepareFusionSwap)
		r.POST("/fusion/execute-swap", s.executeFusionSwap)
		r.GET("/fusion/tokens/:symbol", s.getFusionToken)
	}
	
	if s.proofService != nil {
		r.POST("/proof/generate", s.generateProof)
		r.POST("/proof/verify", s.verifyProof)
		r.POST("/proof/contract-format", s.getProofForContract)
		r.POST("/proof/batch", s.batchGenerateProofs)
		r.GET("/proof/cache/stats", s.getProofCacheStats)
		r.DELETE("/proof/cache", s.clearProofCache)
	}
	
	if s.contractsService != nil {
		r.POST("/contracts/deploy", s.deployContract)
		r.POST("/contracts/verify", s.verifyTransactionOnContract)
		r.POST("/contracts/batch-verify", s.batchVerifyTransactions)
		r.GET("/contracts/info", s.getContractInfo)
		r.GET("/contracts/is-verified/:txhash", s.isTransactionVerified)
	}
}

// Health check handler
func (s *APIServer) healthCheck(c *gin.Context) {
	services := make(map[string]ServiceStatus)
	
	// Check Bitcoin service
	if s.bitcoinService != nil {
		network, blockCount, err := s.bitcoinService.GetNetworkInfo()
		services["bitcoin"] = ServiceStatus{
			Status:  "connected",
			Healthy: err == nil,
			Details: map[string]interface{}{
				"network":     network,
				"block_count": blockCount,
				"error":       formatError(err),
			},
		}
	} else {
		services["bitcoin"] = ServiceStatus{
			Status:  "disabled",
			Healthy: true,
			Details: map[string]interface{}{
				"reason": "Bitcoin service not configured",
			},
		}
	}
	
	// Check Ethereum service
	if s.ethereumService != nil {
		// Would implement ethereum health check
		services["ethereum"] = ServiceStatus{
			Status:  "connected",
			Healthy: true,
		}
	} else {
		services["ethereum"] = ServiceStatus{
			Status:  "disabled", 
			Healthy: true,
			Details: map[string]interface{}{
				"reason": "Ethereum service not configured",
			},
		}
	}
	
	// Check other services
	services["fusion"] = ServiceStatus{
		Status:  getServiceStatus(s.fusionService != nil),
		Healthy: true,
	}
	
	services["proof"] = ServiceStatus{
		Status:  getServiceStatus(s.proofService != nil),
		Healthy: true,
	}
	
	services["contracts"] = ServiceStatus{
		Status:  getServiceStatus(s.contractsService != nil),
		Healthy: true,
	}
	
	services["websocket"] = ServiceStatus{
		Status:  "connected",
		Healthy: true,
		Details: map[string]interface{}{
			"connected_clients": s.wsManager.GetClientCount(),
		},
	}
	
	uptime := time.Since(s.startTime)
	HealthCheck(c, services, uptime, "1.0.0")
}

// Bitcoin handlers
func (s *APIServer) bitcoinStatus(c *gin.Context) {
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	network, blockCount, err := s.bitcoinService.GetNetworkInfo()
	if err != nil {
		InternalServerError(c, "Failed to get Bitcoin network info", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	SuccessResponse(c, map[string]interface{}{
		"network":     network,
		"block_count": blockCount,
		"addresses":   len(s.bitcoinService.GetDepositAddresses()),
	})
}

func (s *APIServer) bitcoinNetworkInfo(c *gin.Context) {
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	network, blockCount, err := s.bitcoinService.GetNetworkInfo()
	if err != nil {
		InternalServerError(c, "Failed to get network info", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	SuccessResponse(c, map[string]interface{}{
		"network":     network,
		"block_count": blockCount,
		"timestamp":   time.Now(),
	})
}

func (s *APIServer) generateBitcoinAddress(c *gin.Context) {
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	address, err := s.bitcoinService.GenerateDepositAddress()
	if err != nil {
		InternalServerError(c, "Failed to generate address", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Notify WebSocket clients
	s.wsManager.BroadcastToTopic(TopicBitcoinTransactions, EventTypeTransaction, "address_generated", map[string]interface{}{
		"address": address,
	})
	
	CreatedResponse(c, map[string]interface{}{
		"address": address,
	})
}

// Utility functions
func formatError(err error) interface{} {
	if err == nil {
		return nil
	}
	return err.Error()
}

func getServiceStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// Pagination helper
func getPaginationParams(c *gin.Context) (page, perPage int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))
	
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	
	return page, perPage
}