package proof

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// MerkleProof represents a Merkle proof for a transaction
type MerkleProof struct {
	TxHash     string   `json:"tx_hash"`
	MerkleRoot string   `json:"merkle_root"`
	Proof      []string `json:"proof"`
	Index      uint32   `json:"index"`
	TotalTxs   uint32   `json:"total_txs"`
}

// MerkleTree represents a Merkle tree structure
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

// NewMerkleTree creates a new Merkle tree from transaction hashes
func NewMerkleTree(txHashes []string) (*MerkleTree, error) {
	if len(txHashes) == 0 {
		return nil, fmt.Errorf("no transaction hashes provided")
	}

	// Convert hex strings to bytes and create leaf nodes
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

	// Build the tree
	tree.Root = tree.buildTree(leaves)
	return tree, nil
}

// buildTree recursively builds the Merkle tree
func (mt *MerkleTree) buildTree(nodes []*MerkleNode) *MerkleNode {
	if len(nodes) == 1 {
		return nodes[0]
	}

	var nextLevel []*MerkleNode

	// Process pairs of nodes
	for i := 0; i < len(nodes); i += 2 {
		left := nodes[i]
		var right *MerkleNode

		if i+1 < len(nodes) {
			right = nodes[i+1]
		} else {
			// If odd number of nodes, duplicate the last one (Bitcoin's rule)
			right = nodes[i]
		}

		// Create parent node
		parent := &MerkleNode{
			Hash:  doubleSHA256(append(left.Hash, right.Hash...)),
			Left:  left,
			Right: right,
		}

		nextLevel = append(nextLevel, parent)
	}

	return mt.buildTree(nextLevel)
}

// GenerateProof generates a Merkle proof for a transaction at given index
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

// getProofPath collects sibling hashes along the path to root
func (mt *MerkleTree) getProofPath(target *MerkleNode) []string {
	var proof []string
	current := target

	// Traverse up the tree collecting sibling hashes
	for current != mt.Root {
		parent := mt.findParent(current)
		if parent == nil {
			break
		}

		// Add sibling hash to proof
		if parent.Left == current {
			// Current is left child, add right sibling
			if parent.Right != nil {
				proof = append(proof, hex.EncodeToString(parent.Right.Hash))
			}
		} else {
			// Current is right child, add left sibling
			if parent.Left != nil {
				proof = append(proof, hex.EncodeToString(parent.Left.Hash))
			}
		}

		current = parent
	}

	return proof
}

// findParent finds the parent node of the given node
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

// VerifyProof verifies a Merkle proof
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

	// Start with transaction hash
	currentHash := txHash
	index := proof.Index

	// Process each proof element
	for _, proofHex := range proof.Proof {
		proofHash, err := hex.DecodeString(proofHex)
		if err != nil {
			return false
		}

		// Determine if current hash is left or right child
		if index%2 == 0 {
			// Even index = left child
			currentHash = doubleSHA256(append(currentHash, proofHash...))
		} else {
			// Odd index = right child
			currentHash = doubleSHA256(append(proofHash, currentHash...))
		}

		// Move up one level
		index = index / 2
	}

	// Compare computed hash with merkle root
	return hex.EncodeToString(currentHash) == hex.EncodeToString(merkleRoot)
}

// doubleSHA256 performs double SHA256 hashing as used in Bitcoin
func doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// GetMerkleRoot returns the Merkle root hash
func (mt *MerkleTree) GetMerkleRoot() string {
	if mt.Root == nil {
		return ""
	}
	return hex.EncodeToString(mt.Root.Hash)
}

// GetLeafCount returns the number of leaf nodes (transactions)
func (mt *MerkleTree) GetLeafCount() int {
	return len(mt.Leaves)
}