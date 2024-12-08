// Copyright (c) 2019-2020 The Zcash developers
// Copyright (c) 2019-2024 Pirate Chain developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or https://www.opensource.org/licenses/mit-license.php .

package parser

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	protobuf "google.golang.org/protobuf/proto"
)

func TestCompactBlocks(t *testing.T) {
	type compactTest struct {
		BlockHeight int    `json:"block"`
		BlockHash   string `json:"hash"`
		PrevHash    string `json:"prev"`
		Full        string `json:"full"`
		Compact     string `json:"compact"`
	}
	var compactTests []compactTest

	blockJSON, err := io.ReadFile("../testdata/compact_blocks.json")
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(blockJSON, &compactTests)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range compactTests {
		blockData, _ := hex.DecodeString(test.Full)
		block := NewBlock()
		blockData, err = block.ParseFromSlice(blockData)
		if err != nil {
			t.Error(errors.Wrap(err, fmt.Sprintf("parsing testnet block %d", test.BlockHeight)))
			continue
		}
		if len(blockData) > 0 {
			t.Error("Extra data remaining")
		}
		if block.GetHeight() != test.BlockHeight {
			t.Errorf("incorrect block height in testnet block %d", test.BlockHeight)
			continue
		}
		if hex.EncodeToString(block.GetDisplayHash()) != test.BlockHash {
			t.Errorf("incorrect block hash in testnet block %x", test.BlockHash)
			continue
		}
		if hex.EncodeToString(block.GetDisplayPrevHash()) != test.PrevHash {
			t.Errorf("incorrect block prevhash in testnet block %x", test.BlockHash)
			continue
		}
		if !bytes.Equal(block.GetPrevHash(), block.hdr.HashPrevBlock) {
			t.Error("block and block header prevhash don't match")
		}

		compact := block.ToCompact()
		marshaled, err := protobuf.Marshal(compact)
		if err != nil {
			t.Errorf("could not marshal compact testnet block %d", test.BlockHeight)
			continue
		}
		encodedCompact := hex.EncodeToString(marshaled)
		if encodedCompact != test.Compact {
			t.Errorf("wrong data for compact testnet block %d\nhave: %s\nwant: %s\n", test.BlockHeight, encodedCompact, test.Compact)
			break
		}
	}
}

// New tests for MerkleFrontier integration

func getValidBlockData() []byte {
	// Mock valid block data
	return []byte{
		/* Add valid block bytes here */
	}
}

func getEmptyBlockData() []byte {
	// Mock empty block data
	return []byte{
		/* Add empty block bytes here */
	}
}

func getCorruptBlockData() []byte {
	// Mock corrupt block data
	return []byte{
		/* Add corrupt block bytes here */
	}
}

func TestParseFromSliceWithMerkleFrontier(t *testing.T) {
	// Case 1: Valid block with multiple transactions
	block := NewBlock()
	data := getValidBlockData() // Mock valid block data
	_, err := block.ParseFromSlice(data)
	assert.NoError(t, err, "Parsing a valid block should not produce an error")
	assert.NotNil(t, block.MerkleFrontier, "MerkleFrontier should be initialized")
	assert.NotEmpty(t, block.MerkleFrontier.GetRoot(), "Merkle root should not be empty for valid block")

	// Case 2: Empty block
	block = NewBlock()
	data = getEmptyBlockData() // Mock empty block data
	_, err = block.ParseFromSlice(data)
	assert.NoError(t, err, "Parsing an empty block should not produce an error")
	assert.Empty(t, block.MerkleFrontier.GetRoot(), "Merkle root should be empty for an empty block")

	// Case 3: Invalid block with corrupt transaction
	block = NewBlock()
	data = getCorruptBlockData() // Mock corrupt block data
	_, err = block.ParseFromSlice(data)
	assert.Error(t, err, "Parsing a block with corrupt transaction should produce an error")
}

func TestMerkleFrontierQueries(t *testing.T) {
	// Mock a block with multiple transactions
	block := NewBlock()
	data := getValidBlockData() // Mock valid block data
	_, err := block.ParseFromSlice(data)
	assert.NoError(t, err)

	// Case 1: Generate Merkle root
	root, err := block.GetMerkleRoot()
	assert.NoError(t, err, "Getting Merkle root should not produce an error")
	assert.NotEmpty(t, root, "Merkle root should not be empty")

	// Case 2: Generate proof for valid transaction index
	proof, err := block.MerkleFrontier.GenerateProof(0) // Proof for first transaction
	assert.NoError(t, err, "Generating proof for valid transaction should not produce an error")
	assert.NotNil(t, proof, "Proof should not be nil")

	// Case 3: Generate proof for invalid transaction index
	proof, err = block.MerkleFrontier.GenerateProof(999) // Out-of-bounds index
	assert.Error(t, err, "Generating proof for invalid index should produce an error")
	assert.Nil(t, proof, "Proof for invalid index should be nil")
}
