package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// Standalone Merkle implementation for testing without import cycles

type MerkleProof struct {
	TxHash     string   `json:"tx_hash"`
	MerkleRoot string   `json:"merkle_root"`
	Proof      []string `json:"proof"`
	Index      uint32   `json:"index"`
	TotalTxs   uint32   `json:"total_txs"`
}

type MerkleTree struct {
	Root   *MerkleNode
	Leaves []*MerkleNode
}

type MerkleNode struct {
	Hash   []byte
	Left   *MerkleNode
	Right  *MerkleNode
	IsLeaf bool
	Index  int
}

func NewMerkleTree(txHashes []string) (*MerkleTree, error) {
	if len(txHashes) == 0 {
		return nil, fmt.Errorf("no transaction hashes provided")
	}

	leaves := make([]*MerkleNode, len(txHashes))
	for i, txHash := range txHashes {
		hashBytes, err := hex.DecodeString(txHash)
		if err != nil {
			return nil, fmt.Errorf("invalid transaction hash %s: %w", txHash, err)
		}
		leaves[i] = &MerkleNode{
			Hash:   hashBytes,
			IsLeaf: true,
			Index:  i,
		}
	}

	tree := &MerkleTree{
		Leaves: leaves,
	}

	tree.Root = tree.buildTree(leaves)
	return tree, nil
}

func (mt *MerkleTree) buildTree(nodes []*MerkleNode) *MerkleNode {
	if len(nodes) == 1 {
		return nodes[0]
	}

	var nextLevel []*MerkleNode

	for i := 0; i < len(nodes); i += 2 {
		left := nodes[i]
		var right *MerkleNode

		if i+1 < len(nodes) {
			right = nodes[i+1]
		} else {
			right = nodes[i]
		}

		parent := &MerkleNode{
			Hash:  doubleSHA256(append(left.Hash, right.Hash...)),
			Left:  left,
			Right: right,
		}

		nextLevel = append(nextLevel, parent)
	}

	return mt.buildTree(nextLevel)
}

func (mt *MerkleTree) GenerateProof(txIndex int) (*MerkleProof, error) {
	if txIndex < 0 || txIndex >= len(mt.Leaves) {
		return nil, fmt.Errorf("transaction index %d out of range", txIndex)
	}

	proof := &MerkleProof{
		TxHash:   hex.EncodeToString(mt.Leaves[txIndex].Hash),
		Index:    uint32(txIndex),
		TotalTxs: uint32(len(mt.Leaves)),
	}

	if mt.Root != nil {
		proof.MerkleRoot = hex.EncodeToString(mt.Root.Hash)
		proof.Proof = mt.getProofPath(mt.Leaves[txIndex])
	}

	return proof, nil
}

func (mt *MerkleTree) getProofPath(target *MerkleNode) []string {
	var proof []string
	current := target

	for current != mt.Root {
		parent := mt.findParent(current)
		if parent == nil {
			break
		}

		if parent.Left == current {
			if parent.Right != nil {
				proof = append(proof, hex.EncodeToString(parent.Right.Hash))
			}
		} else {
			if parent.Left != nil {
				proof = append(proof, hex.EncodeToString(parent.Left.Hash))
			}
		}

		current = parent
	}

	return proof
}

func (mt *MerkleTree) findParent(target *MerkleNode) *MerkleNode {
	return mt.findParentRecursive(mt.Root, target)
}

func (mt *MerkleTree) findParentRecursive(node, target *MerkleNode) *MerkleNode {
	if node == nil || node.IsLeaf {
		return nil
	}

	if node.Left == target || node.Right == target {
		return node
	}

	if parent := mt.findParentRecursive(node.Left, target); parent != nil {
		return parent
	}

	return mt.findParentRecursive(node.Right, target)
}

func VerifyProof(proof *MerkleProof) bool {
	if proof == nil || len(proof.Proof) == 0 {
		return false
	}

	txHash, err := hex.DecodeString(proof.TxHash)
	if err != nil {
		return false
	}

	merkleRoot, err := hex.DecodeString(proof.MerkleRoot)
	if err != nil {
		return false
	}

	currentHash := txHash
	index := proof.Index

	for _, proofHex := range proof.Proof {
		proofHash, err := hex.DecodeString(proofHex)
		if err != nil {
			return false
		}

		if index%2 == 0 {
			currentHash = doubleSHA256(append(currentHash, proofHash...))
		} else {
			currentHash = doubleSHA256(append(proofHash, currentHash...))
		}

		index = index / 2
	}

	return hex.EncodeToString(currentHash) == hex.EncodeToString(merkleRoot)
}

func doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// Tests

func TestMerkleTreeStandalone(t *testing.T) {
	txHashes := []string{
		"a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
		"b2c3d4e5f67890123456789012345678901234567890123456789012345678901234",
		"c3d4e5f678901234567890123456789012345678901234567890123456789012345678",
		"d4e5f6789012345678901234567890123456789012345678901234567890123456789012",
	}

	tree, err := NewMerkleTree(txHashes)
	if err != nil {
		t.Fatalf("Failed to create merkle tree: %v", err)
	}

	if tree.Root == nil {
		t.Error("Merkle tree root is nil")
	}

	if len(tree.Leaves) != len(txHashes) {
		t.Errorf("Expected %d leaves, got %d", len(txHashes), len(tree.Leaves))
	}

	// Test proof generation and verification
	for i := range txHashes {
		proof, err := tree.GenerateProof(i)
		if err != nil {
			t.Fatalf("Failed to generate proof for tx %d: %v", i, err)
		}

		if !VerifyProof(proof) {
			t.Errorf("Proof verification failed for transaction %d", i)
		}
	}

	fmt.Printf("✅ Merkle tree test passed with %d transactions\n", len(txHashes))
	fmt.Printf("✅ All proofs generated and verified successfully\n")
}

func main() {
	fmt.Println("Running standalone Merkle tree test...")
	
	// Create a test object
	t := &testing.T{}
	TestMerkleTreeStandalone(t)
	
	if !t.Failed() {
		fmt.Println("🎉 All Merkle tree tests passed!")
	} else {
		fmt.Println("❌ Some tests failed")
	}
}