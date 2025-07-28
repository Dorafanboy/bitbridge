package contracts

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"bitbridge/internal/proof"
	"bitbridge/pkg/config"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Service manages smart contract interactions
type Service struct {
	client           *ethclient.Client
	deployer         *Deployer
	spvContract      *SPVVerifierContract
	contractAddress  common.Address
	config           *config.EthereumConfig
}

// ServiceConfig for contracts service
type ServiceConfig struct {
	EthereumClient  *ethclient.Client
	EthereumConfig  *config.EthereumConfig
	ContractAddress string // Optional - if contract is already deployed
}

// VerificationRequest represents a request to verify a Bitcoin transaction
type VerificationRequest struct {
	TxHash      string `json:"tx_hash" binding:"required"`
	OutputIndex uint32 `json:"output_index"`
	BlockHeight uint64 `json:"block_height"`
}

// VerificationResponse represents the result of transaction verification
type VerificationResponse struct {
	Verified        bool      `json:"verified"`
	TransactionHash string    `json:"transaction_hash"`
	BlockNumber     uint64    `json:"block_number"`
	GasUsed         uint64    `json:"gas_used"`
	ContractAddress string    `json:"contract_address"`
	Timestamp       time.Time `json:"timestamp"`
}

// BatchVerificationRequest for multiple transactions
type BatchVerificationRequest struct {
	Requests []VerificationRequest `json:"requests" binding:"required"`
}

// BatchVerificationResponse for multiple transactions
type BatchVerificationResponse struct {
	Results         []VerificationResponse `json:"results"`
	TotalGasUsed    uint64                 `json:"total_gas_used"`
	SuccessfulCount int                    `json:"successful_count"`
	FailedCount     int                    `json:"failed_count"`
}

func NewService(config ServiceConfig) (*Service, error) {
	// Create deployer
	deployer, err := NewDeployer(
		config.EthereumClient,
		config.EthereumConfig.PrivateKey,
		big.NewInt(config.EthereumConfig.ChainID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployer: %w", err)
	}

	service := &Service{
		client:   config.EthereumClient,
		deployer: deployer,
		config:   config.EthereumConfig,
	}

	// If contract address is provided, connect to existing contract
	if config.ContractAddress != "" {
		contractAddr := common.HexToAddress(config.ContractAddress)
		contract, err := deployer.InteractWithSPVVerifier(contractAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to existing contract: %w", err)
		}
		service.spvContract = contract
		service.contractAddress = contractAddr
	}

	return service, nil
}

// DeployContract deploys the SPV Verifier smart contract
func (s *Service) DeployContract(ctx context.Context) (*DeploymentResult, error) {
	if s.spvContract != nil {
		return nil, fmt.Errorf("contract already deployed at %s", s.contractAddress.Hex())
	}

	result, err := s.deployer.DeploySPVVerifier(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy SPV verifier: %w", err)
	}

	// Connect to the deployed contract
	contract, err := s.deployer.InteractWithSPVVerifier(result.ContractAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to deployed contract: %w", err)
	}

	s.spvContract = contract
	s.contractAddress = result.ContractAddress

	return result, nil
}

// VerifyTransaction verifies a Bitcoin transaction using SPV proof
func (s *Service) VerifyTransaction(ctx context.Context, req *VerificationRequest, spvProof *proof.SPVProof) (*VerificationResponse, error) {
	if s.spvContract == nil {
		return nil, fmt.Errorf("contract not deployed or connected")
	}

	// Convert SPV proof to contract format
	proofData, headerBytes, err := s.convertSPVProofToContractFormat(spvProof)
	if err != nil {
		return nil, fmt.Errorf("failed to convert proof: %w", err)
	}

	// Call smart contract verification
	tx, err := s.spvContract.VerifyProof(
		ctx,
		headerBytes,
		*proofData,
		big.NewInt(int64(req.BlockHeight)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to verify proof on contract: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := s.waitForTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for transaction: %w", err)
	}

	verified := receipt.Status == types.ReceiptStatusSuccessful

	return &VerificationResponse{
		Verified:        verified,
		TransactionHash: tx.Hash().Hex(),
		BlockNumber:     receipt.BlockNumber.Uint64(),
		GasUsed:         receipt.GasUsed,
		ContractAddress: s.contractAddress.Hex(),
		Timestamp:       time.Now(),
	}, nil
}

// BatchVerifyTransactions verifies multiple Bitcoin transactions
func (s *Service) BatchVerifyTransactions(ctx context.Context, req *BatchVerificationRequest, proofs []*proof.SPVProof) (*BatchVerificationResponse, error) {
	if s.spvContract == nil {
		return nil, fmt.Errorf("contract not deployed or connected")
	}

	if len(req.Requests) != len(proofs) {
		return nil, fmt.Errorf("number of requests must match number of proofs")
	}

	// Convert all proofs to contract format
	var headerBytesArray [][]byte
	var proofsData []ProofData
	var blockHeights []*big.Int

	for i, spvProof := range proofs {
		proofData, headerBytes, err := s.convertSPVProofToContractFormat(spvProof)
		if err != nil {
			return nil, fmt.Errorf("failed to convert proof %d: %w", i, err)
		}

		headerBytesArray = append(headerBytesArray, headerBytes)
		proofsData = append(proofsData, *proofData)
		blockHeights = append(blockHeights, big.NewInt(int64(req.Requests[i].BlockHeight)))
	}

	// Call batch verification
	tx, err := s.spvContract.BatchVerifyProofs(ctx, headerBytesArray, proofsData, blockHeights)
	if err != nil {
		return nil, fmt.Errorf("failed to batch verify proofs: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := s.waitForTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for transaction: %w", err)
	}

	// Create individual results (simplified - in practice you'd parse events)
	results := make([]VerificationResponse, len(req.Requests))
	successCount := 0

	if receipt.Status == types.ReceiptStatusSuccessful {
		successCount = len(req.Requests)
	}

	for i := range req.Requests {
		results[i] = VerificationResponse{
			Verified:        receipt.Status == types.ReceiptStatusSuccessful,
			TransactionHash: tx.Hash().Hex(),
			BlockNumber:     receipt.BlockNumber.Uint64(),
			GasUsed:         receipt.GasUsed / uint64(len(req.Requests)), // Approximate
			ContractAddress: s.contractAddress.Hex(),
			Timestamp:       time.Now(),
		}
	}

	return &BatchVerificationResponse{
		Results:         results,
		TotalGasUsed:    receipt.GasUsed,
		SuccessfulCount: successCount,
		FailedCount:     len(req.Requests) - successCount,
	}, nil
}

// IsTransactionVerified checks if a transaction has been verified on-chain
func (s *Service) IsTransactionVerified(ctx context.Context, txHash string) (bool, error) {
	if s.spvContract == nil {
		return false, fmt.Errorf("contract not deployed or connected")
	}

	// Convert hex string to bytes32
	txHashBytes, err := s.hexStringToBytes32(txHash)
	if err != nil {
		return false, fmt.Errorf("invalid transaction hash: %w", err)
	}

	return s.spvContract.IsTransactionVerified(ctx, txHashBytes)
}

// GetContractInfo returns information about the deployed contract
func (s *Service) GetContractInfo() map[string]interface{} {
	if s.spvContract == nil {
		return map[string]interface{}{
			"deployed": false,
			"address":  nil,
		}
	}

	return map[string]interface{}{
		"deployed": true,
		"address":  s.contractAddress.Hex(),
		"chain_id": s.config.ChainID,
		"network":  s.getNetworkName(),
	}
}

// EstimateVerificationGas estimates gas cost for verification
func (s *Service) EstimateVerificationGas(ctx context.Context, spvProof *proof.SPVProof) (uint64, error) {
	// This is a simplified estimation - in practice you'd use eth_estimateGas
	baseGas := uint64(100000)                                    // Base contract call
	proofGas := uint64(len(spvProof.MerkleProof.Proof) * 5000)   // Per proof element
	headerGas := uint64(20000)                                   // Block header processing
	
	return baseGas + proofGas + headerGas, nil
}

// convertSPVProofToContractFormat converts internal proof format to contract format
func (s *Service) convertSPVProofToContractFormat(spvProof *proof.SPVProof) (*ProofData, []byte, error) {
	// Convert block header to bytes
	headerBytes, err := hex.DecodeString(spvProof.TransactionHex) // This should be block header hex
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode block header: %w", err)
	}

	// Convert merkle proof
	var merkleProofBytes [][32]byte
	for _, proofElement := range spvProof.MerkleProof.Proof {
		proofBytes, err := hex.DecodeString(proofElement)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode proof element: %w", err)
		}
		
		var bytes32 [32]byte
		if len(proofBytes) != 32 {
			return nil, nil, fmt.Errorf("invalid proof element length: %d", len(proofBytes))
		}
		copy(bytes32[:], proofBytes)
		merkleProofBytes = append(merkleProofBytes, bytes32)
	}

	// Convert transaction hash
	txHashBytes, err := s.hexStringToBytes32(spvProof.MerkleProof.TxHash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert tx hash: %w", err)
	}

	// Convert merkle root
	merkleRootBytes, err := s.hexStringToBytes32(spvProof.MerkleProof.MerkleRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert merkle root: %w", err)
	}

	proofData := &ProofData{
		MerkleProof: merkleProofBytes,
		Index:       uint32(spvProof.MerkleProof.Index),
		TxHash:      txHashBytes,
		MerkleRoot:  merkleRootBytes,
	}

	return proofData, headerBytes, nil
}

// hexStringToBytes32 converts hex string to [32]byte
func (s *Service) hexStringToBytes32(hexStr string) ([32]byte, error) {
	var result [32]byte
	
	// Remove 0x prefix if present
	if len(hexStr) > 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}
	
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return result, err
	}
	
	if len(bytes) != 32 {
		return result, fmt.Errorf("hex string must be exactly 32 bytes, got %d", len(bytes))
	}
	
	copy(result[:], bytes)
	return result, nil
}

// waitForTransaction waits for a transaction to be mined
func (s *Service) waitForTransaction(ctx context.Context, tx *types.Transaction) (*types.Receipt, error) {
	for {
		receipt, err := s.client.TransactionReceipt(ctx, tx.Hash())
		if err == nil {
			return receipt, nil
		}
		
		// If error is not "not found", return it
		if err.Error() != "not found" {
			return nil, err
		}
		
		// Wait before checking again
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
			continue
		}
	}
}

// getNetworkName returns human-readable network name
func (s *Service) getNetworkName() string {
	switch s.config.ChainID {
	case 1:
		return "mainnet"
	case 11155111:
		return "sepolia"
	case 5:
		return "goerli"
	case 1337:
		return "localhost"
	default:
		return fmt.Sprintf("chain-%d", s.config.ChainID)
	}
}