package proof

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"bitbridge/internal/bitcoin"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

// SPVProof represents a complete SPV proof for a Bitcoin transaction
type SPVProof struct {
	BlockHeader    *wire.BlockHeader `json:"block_header"`
	MerkleProof    *MerkleProof      `json:"merkle_proof"`
	Transaction    *wire.MsgTx       `json:"transaction"`
	BlockHeight    int32             `json:"block_height"`
	Confirmations  int32             `json:"confirmations"`
	BlockHash      string            `json:"block_hash"`
	TransactionHex string            `json:"transaction_hex"`
}

// Generator handles SPV proof generation
type Generator struct {
	bitcoinClient *bitcoin.Client
	rpcClient     *rpcclient.Client
}

// Config for SPV proof generator
type Config struct {
	BitcoinClient *bitcoin.Client
	RPCClient     *rpcclient.Client
}

func NewGenerator(config Config) *Generator {
	return &Generator{
		bitcoinClient: config.BitcoinClient,
		rpcClient:     config.RPCClient,
	}
}

// GenerateProof generates an SPV proof for a given transaction hash
func (g *Generator) GenerateProof(txHashStr string) (*SPVProof, error) {
	// Parse transaction hash
	txHash, err := chainhash.NewHashFromStr(txHashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction hash: %w", err)
	}

	// Get transaction details
	txResult, err := g.rpcClient.GetRawTransactionVerbose(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	if txResult.BlockHash == "" {
		return nil, fmt.Errorf("transaction not yet included in a block")
	}

	// Parse block hash
	blockHash, err := chainhash.NewHashFromStr(txResult.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("invalid block hash: %w", err)
	}

	// Get block header
	blockHeader, err := g.rpcClient.GetBlockHeader(blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block header: %w", err)
	}

	// Get full block to extract transaction list
	block, err := g.rpcClient.GetBlock(blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	// Extract transaction hashes
	txHashes := make([]string, len(block.Transactions))
	txIndex := -1
	for i, tx := range block.Transactions {
		txHashes[i] = tx.TxHash().String()
		if tx.TxHash().String() == txHashStr {
			txIndex = i
		}
	}

	if txIndex == -1 {
		return nil, fmt.Errorf("transaction not found in block")
	}

	// Build Merkle tree
	merkleTree, err := NewMerkleTree(txHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to build merkle tree: %w", err)
	}

	// Generate Merkle proof
	merkleProof, err := merkleTree.GenerateProof(txIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to generate merkle proof: %w", err)
	}

	// Get the actual transaction
	tx := block.Transactions[txIndex]

	// Get current block height for confirmations
	bestBlockHash, err := g.rpcClient.GetBestBlockHash()
	if err != nil {
		return nil, fmt.Errorf("failed to get best block hash: %w", err)
	}

	bestBlock, err := g.rpcClient.GetBlockVerbose(bestBlockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get best block: %w", err)
	}

	confirmations := bestBlock.Height - int64(txResult.Confirmations) + 1

	// Serialize transaction to hex
	txHex, err := g.serializeTransaction(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	return &SPVProof{
		BlockHeader:    blockHeader,
		MerkleProof:    merkleProof,
		Transaction:    tx,
		BlockHeight:    int32(bestBlock.Height - confirmations + 1),
		Confirmations:  int32(confirmations),
		BlockHash:      blockHash.String(),
		TransactionHex: txHex,
	}, nil
}

// VerifyProof verifies an SPV proof
func (g *Generator) VerifyProof(proof *SPVProof) error {
	// Verify Merkle proof
	if !VerifyProof(proof.MerkleProof) {
		return fmt.Errorf("invalid merkle proof")
	}

	// Verify block header hash matches what's in merkle proof
	blockHash := proof.BlockHeader.BlockHash()
	if blockHash.String() != proof.BlockHash {
		return fmt.Errorf("block hash mismatch")
	}

	// Verify merkle root in block header matches proof
	merkleRootFromHeader := proof.BlockHeader.MerkleRoot.String()
	if merkleRootFromHeader != proof.MerkleProof.MerkleRoot {
		return fmt.Errorf("merkle root mismatch")
	}

	// Verify transaction hash matches
	txHash := proof.Transaction.TxHash().String()
	if txHash != proof.MerkleProof.TxHash {
		return fmt.Errorf("transaction hash mismatch")
	}

	return nil
}

// GetProofForUTXO generates proof for a specific UTXO
func (g *Generator) GetProofForUTXO(txHash string, outputIndex uint32) (*SPVProof, error) {
	proof, err := g.GenerateProof(txHash)
	if err != nil {
		return nil, err
	}

	// Verify the output exists
	if int(outputIndex) >= len(proof.Transaction.TxOut) {
		return nil, fmt.Errorf("output index %d out of range", outputIndex)
	}

	return proof, nil
}

// serializeTransaction serializes a transaction to hex string
func (g *Generator) serializeTransaction(tx *wire.MsgTx) (string, error) {
	var buf bytes.Buffer
	err := tx.Serialize(&buf)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(buf.Bytes()), nil
}

// GetBlockHeaderHex returns block header as hex string for smart contract verification
func (g *Generator) GetBlockHeaderHex(blockHash string) (string, error) {
	hash, err := chainhash.NewHashFromStr(blockHash)
	if err != nil {
		return "", fmt.Errorf("invalid block hash: %w", err)
	}

	header, err := g.rpcClient.GetBlockHeader(hash)
	if err != nil {
		return "", fmt.Errorf("failed to get block header: %w", err)
	}

	// Serialize block header
	var headerBuf bytes.Buffer
	err = header.Serialize(&headerBuf)
	if err != nil {
		return "", fmt.Errorf("failed to serialize header: %w", err)
	}

	return hex.EncodeToString(headerBuf.Bytes()), nil
}

// ValidateMinimumConfirmations checks if transaction has minimum confirmations
func (g *Generator) ValidateMinimumConfirmations(proof *SPVProof, minConfirmations int32) error {
	if proof.Confirmations < minConfirmations {
		return fmt.Errorf("insufficient confirmations: %d < %d", proof.Confirmations, minConfirmations)
	}
	return nil
}

// GetProofSize returns the size of the proof in bytes (useful for gas estimation)
func (g *Generator) GetProofSize(proof *SPVProof) int {
	size := 80 // Block header size
	size += len(proof.MerkleProof.Proof) * 32 // Each proof element is 32 bytes
	size += len(proof.TransactionHex) / 2 // Transaction size
	return size
}

// FormatProofForContract formats proof data for smart contract consumption
func (g *Generator) FormatProofForContract(proof *SPVProof) map[string]interface{} {
	// Convert block header to hex
	var headerBuf bytes.Buffer
	proof.BlockHeader.Serialize(&headerBuf)

	return map[string]interface{}{
		"blockHeader":   hex.EncodeToString(headerBuf.Bytes()),
		"merkleProof":   proof.MerkleProof.Proof,
		"txHash":        proof.MerkleProof.TxHash,
		"txIndex":       proof.MerkleProof.Index,
		"merkleRoot":    proof.MerkleProof.MerkleRoot,
		"blockHeight":   proof.BlockHeight,
		"confirmations": proof.Confirmations,
	}
}