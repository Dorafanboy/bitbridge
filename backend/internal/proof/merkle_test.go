package proof

import (
	"encoding/hex"
	"testing"
)

func TestNewMerkleTree(t *testing.T) {
	// Test with sample transaction hashes
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

	// Verify all leaves are marked as leaf nodes
	for i, leaf := range tree.Leaves {
		if !leaf.IsLeaf {
			t.Errorf("Leaf %d is not marked as leaf", i)
		}
		if leaf.Index != i {
			t.Errorf("Leaf %d has wrong index: %d", i, leaf.Index)
		}
	}
}

func TestMerkleTreeWithSingleTransaction(t *testing.T) {
	txHashes := []string{
		"a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
	}

	tree, err := NewMerkleTree(txHashes)
	if err != nil {
		t.Fatalf("Failed to create merkle tree: %v", err)
	}

	if tree.Root != tree.Leaves[0] {
		t.Error("For single transaction, root should be the leaf itself")
	}
}

func TestMerkleTreeWithOddNumberOfTransactions(t *testing.T) {
	// Test with 3 transactions (odd number)
	txHashes := []string{
		"a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
		"b2c3d4e5f67890123456789012345678901234567890123456789012345678901234",
		"c3d4e5f678901234567890123456789012345678901234567890123456789012345678",
	}

	tree, err := NewMerkleTree(txHashes)
	if err != nil {
		t.Fatalf("Failed to create merkle tree: %v", err)
	}

	if tree.Root == nil {
		t.Error("Merkle tree root is nil")
	}

	// Should handle odd number by duplicating last transaction
	if len(tree.Leaves) != 3 {
		t.Errorf("Expected 3 leaves, got %d", len(tree.Leaves))
	}
}

func TestGenerateProof(t *testing.T) {
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

	// Generate proof for first transaction
	proof, err := tree.GenerateProof(0)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	if proof.TxHash != txHashes[0] {
		t.Errorf("Expected tx hash %s, got %s", txHashes[0], proof.TxHash)
	}

	if proof.Index != 0 {
		t.Errorf("Expected index 0, got %d", proof.Index)
	}

	if proof.TotalTxs != 4 {
		t.Errorf("Expected total txs 4, got %d", proof.TotalTxs)
	}

	if len(proof.Proof) == 0 {
		t.Error("Proof should not be empty")
	}

	// Test invalid index
	_, err = tree.GenerateProof(10)
	if err == nil {
		t.Error("Expected error for invalid index")
	}
}

func TestVerifyProof(t *testing.T) {
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

	// Test each transaction's proof
	for i := range txHashes {
		proof, err := tree.GenerateProof(i)
		if err != nil {
			t.Fatalf("Failed to generate proof for tx %d: %v", i, err)
		}

		if !VerifyProof(proof) {
			t.Errorf("Proof verification failed for transaction %d", i)
		}
	}
}

func TestVerifyProofWithInvalidData(t *testing.T) {
	// Test with nil proof
	if VerifyProof(nil) {
		t.Error("Verification should fail for nil proof")
	}

	// Test with empty proof
	emptyProof := &MerkleProof{}
	if VerifyProof(emptyProof) {
		t.Error("Verification should fail for empty proof")
	}

	// Test with invalid hex
	invalidProof := &MerkleProof{
		TxHash:     "invalid_hex",
		MerkleRoot: "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
		Proof:      []string{"b2c3d4e5f67890123456789012345678901234567890123456789012345678901234"},
		Index:      0,
	}
	if VerifyProof(invalidProof) {
		t.Error("Verification should fail for invalid hex")
	}
}

func TestDoubleSHA256(t *testing.T) {
	// Test with known input
	input := []byte("hello world")
	result := doubleSHA256(input)

	// Result should be 32 bytes
	if len(result) != 32 {
		t.Errorf("Expected 32 bytes, got %d", len(result))
	}

	// Should be deterministic
	result2 := doubleSHA256(input)
	if hex.EncodeToString(result) != hex.EncodeToString(result2) {
		t.Error("doubleSHA256 should be deterministic")
	}
}

func TestMerkleTreeGetMethods(t *testing.T) {
	txHashes := []string{
		"a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
		"b2c3d4e5f67890123456789012345678901234567890123456789012345678901234",
	}

	tree, err := NewMerkleTree(txHashes)
	if err != nil {
		t.Fatalf("Failed to create merkle tree: %v", err)
	}

	// Test GetMerkleRoot
	root := tree.GetMerkleRoot()
	if root == "" {
		t.Error("Merkle root should not be empty")
	}

	// Test GetLeafCount
	count := tree.GetLeafCount()
	if count != 2 {
		t.Errorf("Expected leaf count 2, got %d", count)
	}
}

func TestEmptyTransactionList(t *testing.T) {
	_, err := NewMerkleTree([]string{})
	if err == nil {
		t.Error("Expected error for empty transaction list")
	}
}