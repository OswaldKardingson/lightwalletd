package merkle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Node represents a node in the Merkle tree.
type Node struct {
	Value string
	Left  *Node
	Right *Node
}

// MerkleFrontier maintains the state of the Merkle tree.
type MerkleFrontier struct {
	Leaves []*Node
	Root   *Node
}

// NewMerkleFrontier initializes an empty Merkle Frontier.
func NewMerkleFrontier() *MerkleFrontier {
	return &MerkleFrontier{
		Leaves: make([]*Node, 0),
		Root:   nil,
	}
}

// AddTransaction adds a single transaction to the Merkle Frontier.
func (mf *MerkleFrontier) AddTransaction(tx string) {
	leaf := &Node{Value: tx}
	mf.Leaves = append(mf.Leaves, leaf)
	mf.updateTree()
}

// AddBlock adds multiple transactions from a block to the Merkle Frontier.
func (mf *MerkleFrontier) AddBlock(transactions []string) {
	for _, tx := range transactions {
		mf.AddTransaction(tx)
	}
}

// GetRoot returns the root hash of the Merkle tree.
func (mf *MerkleFrontier) GetRoot() (string, error) {
	if mf.Root == nil {
		return "", errors.New("merkle tree is empty")
	}
	return mf.Root.Value, nil
}

// GenerateProof generates a Merkle proof for a specific transaction index.
func (mf *MerkleFrontier) GenerateProof(index int) ([]string, error) {
	if index < 0 || index >= len(mf.Leaves) {
		return nil, errors.New("index out of range")
	}

	var proof []string
	nodes := mf.Leaves
	for len(nodes) > 1 {
		siblingIndex := index ^ 1
		if siblingIndex < len(nodes) {
			proof = append(proof, nodes[siblingIndex].Value)
		} else {
			proof = append(proof, "")
		}
		index /= 2
		nodes = mf.buildParentLevel(nodes)
	}
	return proof, nil
}

func (mf *MerkleFrontier) updateTree() {
	mf.Root = mf.buildTree(mf.Leaves)
}

func (mf *MerkleFrontier) buildTree(nodes []*Node) *Node {
	for len(nodes) > 1 {
		nodes = mf.buildParentLevel(nodes)
	}
	if len(nodes) == 1 {
		return nodes[0]
	}
	return nil
}

func (mf *MerkleFrontier) buildParentLevel(nodes []*Node) []*Node {
	var parentLevel []*Node
	for i := 0; i < len(nodes); i += 2 {
		left := nodes[i]
		var right *Node
		if i+1 < len(nodes) {
			right = nodes[i+1]
		}
		combinedHash := combineHashes(left.Value, right)
		parent := &Node{
			Value: combinedHash,
			Left:  left,
			Right: right,
		}
		parentLevel = append(parentLevel, parent)
	}
	return parentLevel
}

func combineHashes(left string, right *Node) string {
	hash := sha256.New()
	hash.Write([]byte(left))
	if right != nil {
		hash.Write([]byte(right.Value))
	}
	return hex.EncodeToString(hash.Sum(nil))
}
