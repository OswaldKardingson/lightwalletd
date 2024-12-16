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

// FetchMerkleFrontier retrieves the Merkle Frontier for a given block height.
func FetchMerkleFrontier(blockHeight int) (map[string]interface{}, error) {
	db, err := common.GetDBConnection()
	if err != nil {
		log.Printf("Error connecting to the database: %v", err)
		return nil, errors.New("database connection failed")
	}

	var frontierJSON string
	err = db.QueryRow(`
		SELECT frontier_data
		FROM merkle_frontiers
		WHERE block_height = $1
	`, blockHeight).Scan(&frontierJSON)
	if err != nil {
		log.Printf("Error fetching Merkle Frontier from database: %v", err)
		return nil, errors.New("failed to fetch Merkle Frontier")
	}

	var frontier map[string]interface{}
	err = json.Unmarshal([]byte(frontierJSON), &frontier)
	if err != nil {
		log.Printf("Error unmarshalling Merkle Frontier: %v", err)
		return nil, errors.New("invalid Merkle Frontier data")
	}

	return frontier, nil
}

// StoreMerkleFrontier inserts or updates the Merkle Frontier for a given block height.
func StoreMerkleFrontier(blockHeight int, frontierData map[string]interface{}) error {
	db, err := common.GetDBConnection()
	if err != nil {
		log.Printf("Error connecting to the database: %v", err)
		return errors.New("database connection failed")
	}

	frontierJSON, err := json.Marshal(frontierData)
	if err != nil {
		log.Printf("Error marshalling Merkle Frontier data: %v", err)
		return errors.New("failed to serialize Merkle Frontier data")
	}

	_, err = db.Exec(`
		INSERT INTO merkle_frontiers (block_height, frontier_data)
		VALUES ($1, $2)
		ON CONFLICT (block_height) DO UPDATE SET frontier_data = excluded.frontier_data
	`, blockHeight, string(frontierJSON))
	if err != nil {
		log.Printf("Error storing Merkle Frontier in database: %v", err)
		return errors.New("failed to store Merkle Frontier in database")
	}

	log.Printf("Merkle Frontier successfully stored for block height %d", blockHeight)
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
