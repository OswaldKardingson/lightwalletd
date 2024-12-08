package merkle

import (
	"testing"
)

// Test adding a single transaction and verifying the root hash.
func TestAddTransaction(t *testing.T) {
	mf := NewMerkleFrontier()
	mf.AddTransaction("tx1")

	root, err := mf.GetRoot()
	if err != nil {
		t.Fatalf("expected root hash, got error: %v", err)
	}

	if root == "" {
		t.Fatalf("expected non-empty root hash, got empty string")
	}
}

// Test adding multiple transactions from a block and verifying the root hash.
func TestAddBlock(t *testing.T) {
	mf := NewMerkleFrontier()
	transactions := []string{"tx1", "tx2", "tx3", "tx4"}
	mf.AddBlock(transactions)

	root, err := mf.GetRoot()
	if err != nil {
		t.Fatalf("expected root hash, got error: %v", err)
	}

	if root == "" {
		t.Fatalf("expected non-empty root hash, got empty string")
	}
}

// Test generating a Merkle proof for a specific transaction.
func TestGenerateProof(t *testing.T) {
	mf := NewMerkleFrontier()
	transactions := []string{"tx1", "tx2", "tx3", "tx4"}
	mf.AddBlock(transactions)

	proof, err := mf.GenerateProof(0) // Proof for the first transaction "tx1"
	if err != nil {
		t.Fatalf("expected proof, got error: %v", err)
	}

	if len(proof) == 0 {
		t.Fatalf("expected non-empty proof, got empty proof")
	}
}

// Test error handling for GetRoot on an empty tree.
func TestEmptyTreeRoot(t *testing.T) {
	mf := NewMerkleFrontier()

	_, err := mf.GetRoot()
	if err == nil {
		t.Fatalf("expected error for empty tree, got nil")
	}
}

// Test error handling for GenerateProof with an invalid index.
func TestInvalidProofIndex(t *testing.T) {
	mf := NewMerkleFrontier()
	transactions := []string{"tx1", "tx2"}
	mf.AddBlock(transactions)

	_, err := mf.GenerateProof(10) // Invalid index
	if err == nil {
		t.Fatalf("expected error for invalid index, got nil")
	}
}
