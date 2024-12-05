package merkle

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/PirateNetwork/lightwalletd/common"
)

// FetchMerkleTreeFromDB retrieves the Merkle tree data from the database.
func FetchMerkleTreeFromDB() (map[string]interface{}, error) {
	db, err := common.GetDBConnection()
	if err != nil {
		log.Printf("Error connecting to the database: %v", err)
		return nil, errors.New("database connection failed")
	}

	var treeDataJSON string
	err = db.QueryRow("SELECT tree_data FROM merkle_tree WHERE id = 1").Scan(&treeDataJSON)
	if err != nil {
		log.Printf("Error querying Merkle tree from database: %v", err)
		return nil, errors.New("failed to fetch Merkle tree")
	}

	var treeData map[string]interface{}
	err = json.Unmarshal([]byte(treeDataJSON), &treeData)
	if err != nil {
		log.Printf("Error unmarshalling Merkle tree data: %v", err)
		return nil, errors.New("invalid Merkle tree data")
	}

	return treeData, nil
}

// UpdateMerkleTreeInDB updates the Merkle tree data in the database.
func UpdateMerkleTreeInDB(tree *MerkleTree) error {
	db, err := common.GetDBConnection()
	if err != nil {
		log.Printf("Error connecting to the database: %v", err)
		return errors.New("database connection failed")
	}

	// Serialize the Merkle tree into JSON format
	treeData := map[string]interface{}{
		"RootHash": tree.Root(),
		"Nodes":    serializeNodes(tree.Nodes),
	}

	treeDataJSON, err := json.Marshal(treeData)
	if err != nil {
		log.Printf("Error marshalling Merkle tree data: %v", err)
		return errors.New("failed to serialize Merkle tree data")
	}

	_, err = db.Exec("UPDATE merkle_tree SET tree_data = $1 WHERE id = 1", string(treeDataJSON))
	if err != nil {
		log.Printf("Error updating Merkle tree in database: %v", err)
		return errors.New("failed to update Merkle tree in database")
	}

	log.Println("Merkle tree successfully updated in the database")
	return nil
}

// serializeNodes converts MerkleNode slices into a format suitable for JSON serialization.
func serializeNodes(nodes []*MerkleNode) []map[string]interface{} {
	serialized := make([]map[string]interface{}, len(nodes))
	for i, node := range nodes {
		serialized[i] = map[string]interface{}{
			"Data":  node.Data,
			"Index": node.Index,
		}
	}
	return serialized
}

// AddBlockToMerkleTree updates the Merkle tree with a new block and updates the database.
func AddBlockToMerkleTree(blockData string) error {
	// Fetch the current Merkle tree
	treeData, err := FetchMerkleTreeFromDB()
	if err != nil {
		return err
	}

	// Reconstruct the Merkle tree from the database data
	tree := NewMerkleTreeFromData(treeData)

	// Add the new block to the tree
	newNode := &MerkleNode{
		Data:  hashFunction(blockData),
		Index: len(tree.Nodes), // Assign the next index
	}
	tree.Nodes = append(tree.Nodes, newNode)

	// Update the root hash
	tree.RootHash = calculateRootHash(tree.Nodes)

	// Save the updated tree back to the database
	return UpdateMerkleTreeInDB(tree)
}

// calculateRootHash computes the Merkle root hash from the nodes
func calculateRootHash(nodes []*MerkleNode) string {
	if len(nodes) == 0 {
		return hashEmptyNode()
	}

	// Rebuild the tree to compute the new root hash
	level := nodes
	for len(level) > 1 {
		var nextLevel []*MerkleNode
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			var right *MerkleNode
			if i+1 < len(level) {
				right = level[i+1]
			} else {
				right = &MerkleNode{Data: hashEmptyNode()}
			}

			combinedHash := hashFunction(left.Data + right.Data)
			nextLevel = append(nextLevel, &MerkleNode{Data: combinedHash})
		}
		level = nextLevel
	}
	return level[0].Data
}
