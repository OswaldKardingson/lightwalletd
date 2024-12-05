package merkle

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/PirateNetwork/lightwalletd/common"
)

// MerkleSyncResponse defines the structure of the response for /merkle/sync
type MerkleSyncResponse struct {
	ServerRoot string   `json:"serverRoot"`
	Deltas     []string `json:"deltas"`
	Proof      []Proof  `json:"proof"`
	Page       int      `json:"page,omitempty"`
	PageSize   int      `json:"pageSize,omitempty"`
}

// Proof represents a proof for a Merkle tree node
type Proof struct {
	Hashes []string `json:"hashes"`
	Index  int      `json:"index"`
}

// MerkleNode represents a node in the Merkle tree
type MerkleNode struct {
	Data  string
	Index int
}

// MerkleTree represents the Merkle tree structure
type MerkleTree struct {
	RootHash string
	Nodes    []*MerkleNode
}

// MerkleSyncHandler handles the /merkle/sync endpoint
func MerkleSyncHandler(w http.ResponseWriter, r *http.Request) {
	localRoot := r.URL.Query().Get("root")
	if localRoot == "" {
		http.Error(w, "Missing local Merkle root", http.StatusBadRequest)
		return
	}

	// Validate query parameters for pagination
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 0 {
		page = 0
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil || pageSize <= 0 {
		pageSize = 100
	}

	// Load the server's Merkle tree
	serverTree, err := LoadServerMerkleTree()
	if err != nil {
		log.Printf("Error loading server Merkle tree: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If the client's tree matches the server's, return no deltas
	if serverTree.Root() == localRoot {
		response := MerkleSyncResponse{
			ServerRoot: serverTree.Root(),
			Deltas:     []string{},
			Proof:      []Proof{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Generate deltas and proofs
	deltas, proofs, err := GenerateDeltasAndProofs(localRoot, serverTree)
	if err != nil {
		log.Printf("Error generating deltas and proofs: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Paginate deltas
	paginatedDeltas := PaginateDeltas(deltas, page, pageSize)

	// Construct response
	response := MerkleSyncResponse{
		ServerRoot: serverTree.Root(),
		Deltas:     paginatedDeltas,
		Proof:      proofs,
		Page:       page,
		PageSize:   pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// PaginateDeltas splits deltas into pages based on page size
func PaginateDeltas(deltas []string, page int, pageSize int) []string {
	start := page * pageSize
	if start >= len(deltas) {
		return []string{}
	}
	end := start + pageSize
	if end > len(deltas) {
		end = len(deltas)
	}
	return deltas[start:end]
}

// LoadServerMerkleTree loads the Merkle tree from the database
func LoadServerMerkleTree() (*MerkleTree, error) {
	// Get the database connection
	db, err := common.GetDBConnection()
	if err != nil {
		return nil, err
	}

	// Query the database for the Merkle tree data
	var treeDataJSON string
	err = db.QueryRow("SELECT tree_data FROM merkle_tree WHERE id = 1").Scan(&treeDataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("no Merkle tree found in database")
		}
		return nil, err
	}

	// Parse the JSON data into a Merkle tree structure
	var treeData map[string]interface{}
	err = json.Unmarshal([]byte(treeDataJSON), &treeData)
	if err != nil {
		return nil, err
	}

	return NewMerkleTreeFromData(treeData), nil
}

// NewMerkleTreeFromData creates a new Merkle tree from raw data
func NewMerkleTreeFromData(data map[string]interface{}) *MerkleTree {
	nodes := make([]*MerkleNode, 0)
	for _, rawNode := range data["Nodes"].([]interface{}) {
		nodeMap := rawNode.(map[string]interface{})
		nodes = append(nodes, &MerkleNode{
			Data:  nodeMap["Data"].(string),
			Index: int(nodeMap["Index"].(float64)),
		})
	}

	return &MerkleTree{
		RootHash: data["RootHash"].(string),
		Nodes:    nodes,
	}
}

// Root returns the root hash of the Merkle tree
func (t *MerkleTree) Root() string {
	return t.RootHash
}

// GetNodesNotIn identifies missing nodes based on the client's root
func (t *MerkleTree) GetNodesNotIn(localRoot string) ([]*MerkleNode, error) {
	var missingNodes []*MerkleNode

	for _, node := range t.Nodes {
		if !node.IsPartOf(localRoot, t) {
			missingNodes = append(missingNodes, node)
		}
	}
	return missingNodes, nil
}

// GenerateProof creates a proof for a given node
func (t *MerkleTree) GenerateProof(node *MerkleNode) (Proof, error) {
	var hashes []string
	currentIndex := node.Index

	for currentIndex > 0 {
		siblingIndex := currentIndex ^ 1
		var siblingHash string
		if siblingIndex < len(t.Nodes) {
			siblingHash = t.Nodes[siblingIndex].Data
		} else {
			siblingHash = hashEmptyNode()
		}
		hashes = append(hashes, siblingHash)
		currentIndex = (currentIndex - 1) / 2
	}

	return Proof{
		Hashes: hashes,
		Index:  node.Index,
	}, nil
}

// IsPartOf checks if the node belongs to the tree
func (n *MerkleNode) IsPartOf(root string, tree *MerkleTree) bool {
	currentHash := n.Data
	currentIndex := n.Index

	for currentIndex > 0 {
		siblingIndex := currentIndex ^ 1
		var siblingHash string
		if siblingIndex < len(tree.Nodes) {
			siblingHash = tree.Nodes[siblingIndex].Data
		} else {
			siblingHash = hashEmptyNode()
		}
		if currentIndex%2 == 0 {
			currentHash = hashFunction(currentHash + siblingHash)
		} else {
			currentHash = hashFunction(siblingHash + currentHash)
		}
		currentIndex = (currentIndex - 1) / 2
	}

	return currentHash == root
}

// Hash helpers
func hashFunction(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func hashEmptyNode() string {
	return hashFunction("")
}
