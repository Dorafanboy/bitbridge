package api

import (
	"context"
	"strconv"
	"time"

	"bitbridge/internal/contracts"
	"bitbridge/internal/proof"
	"bitbridge/pkg/types"

	"github.com/gin-gonic/gin"
)

// Implementation of specific API handlers

// Bitcoin handlers implementation
func (s *APIServer) getBitcoinAddresses(c *gin.Context) {
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	addresses := s.bitcoinService.GetDepositAddresses()
	SuccessResponse(c, map[string]interface{}{
		"addresses": addresses,
		"count":     len(addresses),
	})
}

func (s *APIServer) getAddressUTXOs(c *gin.Context) {
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	address := c.Param("address")
	if address == "" {
		BadRequestError(c, "Address parameter is required", nil)
		return
	}
	
	utxos, err := s.bitcoinService.GetAddressUTXOs(address)
	if err != nil {
		InternalServerError(c, "Failed to get UTXOs", map[string]interface{}{
			"error":   err.Error(),
			"address": address,
		})
		return
	}
	
	SuccessResponse(c, map[string]interface{}{
		"address": address,
		"utxos":   utxos,
		"count":   len(utxos),
	})
}

func (s *APIServer) getAddressBalance(c *gin.Context) {
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	address := c.Param("address")
	if address == "" {
		BadRequestError(c, "Address parameter is required", nil)
		return
	}
	
	utxos, err := s.bitcoinService.GetAddressUTXOs(address)
	if err != nil {
		InternalServerError(c, "Failed to get balance", map[string]interface{}{
			"error":   err.Error(),
			"address": address,
		})
		return
	}
	
	var totalBalance int64
	for _, utxo := range utxos {
		totalBalance += utxo.Amount
	}
	
	SuccessResponse(c, map[string]interface{}{
		"address":     address,
		"balance":     totalBalance,
		"balance_btc": float64(totalBalance) / 100000000,
		"utxo_count":  len(utxos),
	})
}

func (s *APIServer) getUTXO(c *gin.Context) {
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	txid := c.Param("txid")
	voutStr := c.Param("vout")
	
	if txid == "" || voutStr == "" {
		BadRequestError(c, "Both txid and vout parameters are required", nil)
		return
	}
	
	vout, err := strconv.ParseUint(voutStr, 10, 32)
	if err != nil {
		BadRequestError(c, "Invalid vout parameter", map[string]interface{}{
			"vout": voutStr,
		})
		return
	}
	
	utxo, err := s.bitcoinService.GetUTXO(txid, uint32(vout))
	if err != nil {
		NotFoundError(c, "UTXO not found")
		return
	}
	
	SuccessResponse(c, utxo)
}

func (s *APIServer) getAllUTXOs(c *gin.Context) {
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	page, perPage := getPaginationParams(c)
	allUTXOs := s.bitcoinService.GetAllWatchedUTXOs()
	
	// Simple pagination
	start := (page - 1) * perPage
	end := start + perPage
	
	if start >= len(allUTXOs) {
		SuccessResponse(c, map[string]interface{}{
			"utxos": []interface{}{},
			"pagination": PaginationInfo{
				Page:       page,
				PerPage:    perPage,
				Total:      len(allUTXOs),
				TotalPages: (len(allUTXOs) + perPage - 1) / perPage,
			},
		})
		return
	}
	
	if end > len(allUTXOs) {
		end = len(allUTXOs)
	}
	
	paginatedUTXOs := allUTXOs[start:end]
	
	PaginatedResponse(c, map[string]interface{}{
		"utxos": paginatedUTXOs,
	}, PaginationInfo{
		Page:       page,
		PerPage:    perPage,
		Total:      len(allUTXOs),
		TotalPages: (len(allUTXOs) + perPage - 1) / perPage,
	})
}

func (s *APIServer) validateBitcoinAddress(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	isValid := s.bitcoinService.IsValidBitcoinAddress(req.Address)
	
	SuccessResponse(c, map[string]interface{}{
		"address": req.Address,
		"valid":   isValid,
	})
}

func (s *APIServer) watchBitcoinAddress(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	if s.bitcoinService == nil {
		ServiceUnavailableError(c, "Bitcoin service not available")
		return
	}
	
	err := s.bitcoinService.WatchAddress(req.Address)
	if err != nil {
		InternalServerError(c, "Failed to watch address", map[string]interface{}{
			"error":   err.Error(),
			"address": req.Address,
		})
		return
	}
	
	// Notify WebSocket clients
	s.wsManager.BroadcastToTopic(TopicBitcoinTransactions, EventTypeTransaction, "address_watched", map[string]interface{}{
		"address": req.Address,
	})
	
	SuccessResponse(c, map[string]interface{}{
		"address": req.Address,
		"message": "Address is now being watched",
	})
}

// Ethereum handlers implementation
func (s *APIServer) ethereumStatus(c *gin.Context) {
	if s.ethereumService == nil {
		ServiceUnavailableError(c, "Ethereum service not available")
		return
	}
	
	// Implementation would depend on ethereum service methods
	SuccessResponse(c, map[string]interface{}{
		"status":  "connected",
		"message": "Ethereum service is running",
	})
}

func (s *APIServer) getEthereumBalance(c *gin.Context) {
	if s.ethereumService == nil {
		ServiceUnavailableError(c, "Ethereum service not available")
		return
	}
	
	address := c.Param("address")
	if address == "" {
		BadRequestError(c, "Address parameter is required", nil)
		return
	}
	
	// Implementation would call ethereum service
	SuccessResponse(c, map[string]interface{}{
		"address": address,
		"balance": "0", // Placeholder
	})
}

func (s *APIServer) getEthereumBlockNumber(c *gin.Context) {
	if s.ethereumService == nil {
		ServiceUnavailableError(c, "Ethereum service not available")
		return
	}
	
	// Implementation would call ethereum service
	SuccessResponse(c, map[string]interface{}{
		"block_number": 0, // Placeholder
	})
}

func (s *APIServer) getEthereumGasPrice(c *gin.Context) {
	if s.ethereumService == nil {
		ServiceUnavailableError(c, "Ethereum service not available")
		return
	}
	
	// Implementation would call ethereum service
	SuccessResponse(c, map[string]interface{}{
		"gas_price": "0", // Placeholder
	})
}

func (s *APIServer) sendEthereumTransaction(c *gin.Context) {
	if s.ethereumService == nil {
		ServiceUnavailableError(c, "Ethereum service not available")
		return
	}
	
	var req struct {
		To       string `json:"to" binding:"required"`
		Amount   string `json:"amount" binding:"required"`
		GasLimit uint64 `json:"gas_limit,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Implementation would call ethereum service
	SuccessResponse(c, map[string]interface{}{
		"transaction_hash": "0x...", // Placeholder
		"status":           "submitted",
	})
}

func (s *APIServer) getEthereumTransaction(c *gin.Context) {
	if s.ethereumService == nil {
		ServiceUnavailableError(c, "Ethereum service not available")
		return
	}
	
	hash := c.Param("hash")
	if hash == "" {
		BadRequestError(c, "Transaction hash parameter is required", nil)
		return
	}
	
	// Implementation would call ethereum service
	SuccessResponse(c, map[string]interface{}{
		"hash":   hash,
		"status": "confirmed", // Placeholder
	})
}

// Fusion handlers implementation
func (s *APIServer) getFusionQuote(c *gin.Context) {
	if s.fusionService == nil {
		ServiceUnavailableError(c, "Fusion service not available")
		return
	}
	
	var req struct {
		TokenFrom   string `json:"token_from" binding:"required"`
		TokenTo     string `json:"token_to" binding:"required"`
		Amount      string `json:"amount" binding:"required"`
		FromAddress string `json:"from_address" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	ctx := context.Background()
	quote, err := s.fusionService.GetBestQuote(ctx, req.TokenFrom, req.TokenTo, req.Amount, req.FromAddress)
	if err != nil {
		InternalServerError(c, "Failed to get quote", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	SuccessResponse(c, quote)
}

func (s *APIServer) prepareFusionSwap(c *gin.Context) {
	if s.fusionService == nil {
		ServiceUnavailableError(c, "Fusion service not available")
		return
	}
	
	var req types.SwapRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	ctx := context.Background()
	swap, err := s.fusionService.SwapUTXOToken(ctx, &req)
	if err != nil {
		InternalServerError(c, "Failed to prepare swap", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	SuccessResponse(c, swap)
}

func (s *APIServer) executeFusionSwap(c *gin.Context) {
	if s.fusionService == nil {
		ServiceUnavailableError(c, "Fusion service not available")
		return
	}
	
	var req types.SwapResponse
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	ctx := context.Background()
	tx, err := s.fusionService.ExecuteSwap(ctx, &req)
	if err != nil {
		InternalServerError(c, "Failed to execute swap", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Notify WebSocket clients
	s.wsManager.BroadcastToTopic(TopicSwapEvents, EventTypeSwap, "swap_executed", map[string]interface{}{
		"transaction_hash": tx.Hash().Hex(),
		"gas_used":         tx.Gas(),
		"gas_price":        tx.GasPrice().String(),
	})
	
	SuccessResponse(c, map[string]interface{}{
		"transaction_hash": tx.Hash().Hex(),
		"gas_used":         tx.Gas(),
		"gas_price":        tx.GasPrice().String(),
		"status":           "submitted",
	})
}

func (s *APIServer) getFusionToken(c *gin.Context) {
	if s.fusionService == nil {
		ServiceUnavailableError(c, "Fusion service not available")
		return
	}
	
	symbol := c.Param("symbol")
	if symbol == "" {
		BadRequestError(c, "Token symbol parameter is required", nil)
		return
	}
	
	address, err := s.fusionService.GetTokenAddress(symbol)
	if err != nil {
		NotFoundError(c, "Token not found")
		return
	}
	
	SuccessResponse(c, map[string]interface{}{
		"symbol":  symbol,
		"address": address,
	})
}

func (s *APIServer) getFusionTokens(c *gin.Context) {
	if s.fusionService == nil {
		ServiceUnavailableError(c, "Fusion service not available")
		return
	}
	
	// Implementation would get list of supported tokens
	SuccessResponse(c, map[string]interface{}{
		"tokens": []string{}, // Placeholder
	})
}

func (s *APIServer) getFusionOrders(c *gin.Context) {
	if s.fusionService == nil {
		ServiceUnavailableError(c, "Fusion service not available")
		return
	}
	
	address := c.Param("address")
	if address == "" {
		BadRequestError(c, "Address parameter is required", nil)
		return
	}
	
	// Implementation would get fusion orders for address
	SuccessResponse(c, map[string]interface{}{
		"orders": []interface{}{}, // Placeholder
	})
}

func (s *APIServer) cancelFusionOrder(c *gin.Context) {
	if s.fusionService == nil {
		ServiceUnavailableError(c, "Fusion service not available")
		return
	}
	
	var req struct {
		OrderHash string `json:"order_hash" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Implementation would cancel fusion order
	SuccessResponse(c, map[string]interface{}{
		"order_hash": req.OrderHash,
		"status":     "cancelled",
	})
}

// Proof handlers implementation
func (s *APIServer) generateProof(c *gin.Context) {
	if s.proofService == nil {
		ServiceUnavailableError(c, "Proof service not available")
		return
	}
	
	var req proof.ProofRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	ctx := context.Background()
	proofResp, err := s.proofService.GenerateProof(ctx, &req)
	if err != nil {
		InternalServerError(c, "Failed to generate proof", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Notify WebSocket clients
	s.wsManager.BroadcastToTopic(TopicProofGeneration, EventTypeProof, "proof_generated", map[string]interface{}{
		"tx_hash":       req.TxHash,
		"proof_size":    proofResp.ProofSize,
		"cached":        proofResp.Cached,
		"generated_at":  proofResp.GeneratedAt,
	})
	
	SuccessResponse(c, proofResp)
}

func (s *APIServer) verifyProof(c *gin.Context) {
	if s.proofService == nil {
		ServiceUnavailableError(c, "Proof service not available")
		return
	}
	
	var spvProof proof.SPVProof
	
	if err := c.ShouldBindJSON(&spvProof); err != nil {
		BadRequestError(c, "Invalid proof format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	ctx := context.Background()
	err := s.proofService.VerifyProof(ctx, &spvProof)
	
	SuccessResponse(c, map[string]interface{}{
		"valid": err == nil,
		"error": formatError(err),
	})
}

func (s *APIServer) getProofForContract(c *gin.Context) {
	if s.proofService == nil {
		ServiceUnavailableError(c, "Proof service not available")
		return
	}
	
	var req proof.ProofRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	ctx := context.Background()
	contractData, err := s.proofService.GetProofForContract(ctx, &req)
	if err != nil {
		InternalServerError(c, "Failed to format proof for contract", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	SuccessResponse(c, contractData)
}

func (s *APIServer) batchGenerateProofs(c *gin.Context) {
	if s.proofService == nil {
		ServiceUnavailableError(c, "Proof service not available")
		return
	}
	
	var requests []*proof.ProofRequest
	
	if err := c.ShouldBindJSON(&requests); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	if len(requests) == 0 {
		BadRequestError(c, "No proof requests provided", nil)
		return
	}
	
	if len(requests) > 50 { // Limit batch size
		BadRequestError(c, "Too many requests in batch (max 50)", map[string]interface{}{
			"provided": len(requests),
			"maximum":  50,
		})
		return
	}
	
	ctx := context.Background()
	responses, err := s.proofService.BatchGenerateProofs(ctx, requests)
	if err != nil {
		// Partial success scenario
		SuccessResponse(c, map[string]interface{}{
			"proofs":          responses,
			"partial_success": true,
			"error":           err.Error(),
		})
		return
	}
	
	// Notify WebSocket clients
	s.wsManager.BroadcastToTopic(TopicProofGeneration, EventTypeProof, "batch_proofs_generated", map[string]interface{}{
		"count":     len(responses),
		"requested": len(requests),
	})
	
	SuccessResponse(c, map[string]interface{}{
		"proofs": responses,
	})
}

func (s *APIServer) getProofCacheStats(c *gin.Context) {
	if s.proofService == nil {
		ServiceUnavailableError(c, "Proof service not available")
		return
	}
	
	stats := s.proofService.GetCacheStats()
	SuccessResponse(c, stats)
}

func (s *APIServer) clearProofCache(c *gin.Context) {
	if s.proofService == nil {
		ServiceUnavailableError(c, "Proof service not available")
		return
	}
	
	s.proofService.ClearCache()
	SuccessResponse(c, map[string]interface{}{
		"message": "Proof cache cleared successfully",
	})
}

func (s *APIServer) getMerkleTree(c *gin.Context) {
	// Implementation would get merkle tree for transaction
	SuccessResponse(c, map[string]interface{}{
		"message": "Not implemented yet",
	})
}

func (s *APIServer) validateMerkleProof(c *gin.Context) {
	// Implementation would validate merkle proof
	SuccessResponse(c, map[string]interface{}{
		"message": "Not implemented yet",
	})
}

// Contract handlers implementation
func (s *APIServer) deployContract(c *gin.Context) {
	if s.contractsService == nil {
		ServiceUnavailableError(c, "Contracts service not available")
		return
	}
	
	ctx := context.Background()
	result, err := s.contractsService.DeployContract(ctx)
	if err != nil {
		InternalServerError(c, "Failed to deploy contract", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Notify WebSocket clients
	s.wsManager.BroadcastToTopic(TopicContractEvents, EventTypeSystem, "contract_deployed", map[string]interface{}{
		"contract_address": result.ContractAddress.Hex(),
		"transaction_hash": result.TransactionHash.Hex(),
		"block_number":     result.BlockNumber,
		"gas_used":         result.GasUsed,
	})
	
	CreatedResponse(c, result)
}

func (s *APIServer) verifyTransactionOnContract(c *gin.Context) {
	if s.contractsService == nil {
		ServiceUnavailableError(c, "Contracts service not available")
		return
	}
	
	var req contracts.VerificationRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	if s.proofService == nil {
		ServiceUnavailableError(c, "SPV proof service not available")
		return
	}
	
	// Generate SPV proof
	ctx := context.Background()
	proofReq := &proof.ProofRequest{
		TxHash:                req.TxHash,
		OutputIndex:           req.OutputIndex,
		RequiredConfirmations: 6,
	}
	
	proofResp, err := s.proofService.GenerateProof(ctx, proofReq)
	if err != nil {
		InternalServerError(c, "Failed to generate proof", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Verify on contract
	verifyResp, err := s.contractsService.VerifyTransaction(ctx, &req, proofResp.Proof)
	if err != nil {
		InternalServerError(c, "Failed to verify on contract", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Notify WebSocket clients
	s.wsManager.BroadcastToTopic(TopicContractEvents, EventTypeTransaction, "transaction_verified", map[string]interface{}{
		"tx_hash":          req.TxHash,
		"verified":         verifyResp.Verified,
		"transaction_hash": verifyResp.TransactionHash,
		"gas_used":         verifyResp.GasUsed,
	})
	
	SuccessResponse(c, verifyResp)
}

func (s *APIServer) batchVerifyTransactions(c *gin.Context) {
	if s.contractsService == nil {
		ServiceUnavailableError(c, "Contracts service not available")
		return
	}
	
	var req contracts.BatchVerificationRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	if s.proofService == nil {
		ServiceUnavailableError(c, "SPV proof service not available")
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
	
	proofResponses, err := s.proofService.BatchGenerateProofs(ctx, proofRequests)
	if err != nil {
		InternalServerError(c, "Failed to generate proofs", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Extract proofs
	var proofs []*proof.SPVProof
	for _, proofResp := range proofResponses {
		if proofResp != nil {
			proofs = append(proofs, proofResp.Proof)
		} else {
			BadRequestError(c, "Failed to generate some proofs", nil)
			return
		}
	}
	
	// Verify on contract
	verifyResp, err := s.contractsService.BatchVerifyTransactions(ctx, &req, proofs)
	if err != nil {
		InternalServerError(c, "Failed to batch verify on contract", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Notify WebSocket clients
	s.wsManager.BroadcastToTopic(TopicContractEvents, EventTypeTransaction, "batch_transactions_verified", map[string]interface{}{
		"total_count":      len(req.Requests),
		"successful_count": verifyResp.SuccessfulCount,
		"failed_count":     verifyResp.FailedCount,
		"total_gas_used":   verifyResp.TotalGasUsed,
	})
	
	SuccessResponse(c, verifyResp)
}

func (s *APIServer) getContractInfo(c *gin.Context) {
	if s.contractsService == nil {
		ServiceUnavailableError(c, "Contracts service not available")
		return
	}
	
	info := s.contractsService.GetContractInfo()
	SuccessResponse(c, info)
}

func (s *APIServer) isTransactionVerified(c *gin.Context) {
	if s.contractsService == nil {
		ServiceUnavailableError(c, "Contracts service not available")
		return
	}
	
	txHash := c.Param("txhash")
	if txHash == "" {
		BadRequestError(c, "Transaction hash parameter is required", nil)
		return
	}
	
	ctx := context.Background()
	verified, err := s.contractsService.IsTransactionVerified(ctx, txHash)
	if err != nil {
		InternalServerError(c, "Failed to check verification status", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	SuccessResponse(c, map[string]interface{}{
		"tx_hash":  txHash,
		"verified": verified,
	})
}

func (s *APIServer) estimateContractGas(c *gin.Context) {
	// Implementation would estimate gas for contract operations
	SuccessResponse(c, map[string]interface{}{
		"gas_estimate": 200000, // Placeholder
	})
}

func (s *APIServer) getContractEvents(c *gin.Context) {
	// Implementation would get contract events
	SuccessResponse(c, map[string]interface{}{
		"events": []interface{}{}, // Placeholder
	})
}

// Utility handlers implementation
func (s *APIServer) validateBitcoinAddressUtil(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	isValid := validateBitcoinAddress(req.Address)
	
	SuccessResponse(c, map[string]interface{}{
		"address": req.Address,
		"valid":   isValid,
	})
}

func (s *APIServer) validateEthereumAddressUtil(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	isValid := validateEthereumAddress(req.Address)
	
	SuccessResponse(c, map[string]interface{}{
		"address": req.Address,
		"valid":   isValid,
	})
}

func (s *APIServer) validateTransactionHashUtil(c *gin.Context) {
	var req struct {
		Hash string `json:"hash" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestError(c, "Invalid request format", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	isValid := validateTransactionHash(req.Hash)
	
	SuccessResponse(c, map[string]interface{}{
		"hash":  req.Hash,
		"valid": isValid,
	})
}

func (s *APIServer) getNetworkStatus(c *gin.Context) {
	networkStatus := map[string]interface{}{
		"bitcoin":   getServiceStatus(s.bitcoinService != nil),
		"ethereum":  getServiceStatus(s.ethereumService != nil),
		"fusion":    getServiceStatus(s.fusionService != nil),
		"proof":     getServiceStatus(s.proofService != nil),
		"contracts": getServiceStatus(s.contractsService != nil),
	}
	
	SuccessResponse(c, networkStatus)
}

func (s *APIServer) getSystemInfo(c *gin.Context) {
	systemInfo := map[string]interface{}{
		"service":   "utxo-evm-gateway",
		"version":   "1.0.0",
		"uptime":    time.Since(s.startTime).String(),
		"timestamp": time.Now(),
		"features": []string{
			"Bitcoin UTXO monitoring",
			"Ethereum smart contract integration",
			"1inch Fusion+ protocol support",
			"SPV proof generation and verification",
			"Real-time WebSocket updates",
		},
	}
	
	SuccessResponse(c, systemInfo)
}

func (s *APIServer) getWebSocketStats(c *gin.Context) {
	stats := map[string]interface{}{
		"connected_clients": s.wsManager.GetClientCount(),
		"topics": map[string]interface{}{
			"bitcoin_blocks":        s.wsManager.GetTopicSubscribers(TopicBitcoinBlocks),
			"bitcoin_transactions":  s.wsManager.GetTopicSubscribers(TopicBitcoinTransactions),
			"ethereum_blocks":       s.wsManager.GetTopicSubscribers(TopicEthereumBlocks),
			"ethereum_transactions": s.wsManager.GetTopicSubscribers(TopicEthereumTransactions),
			"utxo_events":           s.wsManager.GetTopicSubscribers(TopicUTXOEvents),
			"proof_generation":      s.wsManager.GetTopicSubscribers(TopicProofGeneration),
			"swap_events":           s.wsManager.GetTopicSubscribers(TopicSwapEvents),
			"contract_events":       s.wsManager.GetTopicSubscribers(TopicContractEvents),
		},
	}
	
	SuccessResponse(c, stats)
}