// Copyright (c) 2019-2020 The Zcash developers
// Copyright (c) 2019-2021 Pirate Chain developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or https://www.opensource.org/licenses/mit-license.php .
package common

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/PirateNetwork/lightwalletd/parser"
	"github.com/PirateNetwork/lightwalletd/walletrpc"
	"github.com/golang/protobuf/proto"
)

var compacts []*walletrpc.CompactBlock
var cache *BlockCache

const (
	unitTestPath  = "unittestcache"
	unitTestChain = "unittestnet"
)

func ensureCompactsLoaded(t *testing.T) {
	if len(compacts) > 0 {
		return
	}

	type compactTest struct {
		BlockHeight int    `json:"block"`
		BlockHash   string `json:"hash"`
		PrevHash    string `json:"prev"`
		Full        string `json:"full"`
		Compact     string `json:"compact"`
	}
	var compactTests []compactTest

	blockJSON, err := ioutil.ReadFile("../testdata/compact_blocks.json")
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(blockJSON, &compactTests)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range compactTests {
		blockData, _ := hex.DecodeString(test.Full)
		block := parser.NewBlock()
		blockData, err = block.ParseFromSlice(blockData)
		if err != nil {
			t.Fatal(err)
		}
		if len(blockData) > 0 {
			t.Error("Extra data remaining")
		}
		compacts = append(compacts, block.ToCompact())
	}
}

func cloneCompactsWithStartHeight(startHeight int, count int) []*walletrpc.CompactBlock {
	cloned := make([]*walletrpc.CompactBlock, 0, count)
	for i := 0; i < count && i < len(compacts); i++ {
		src := compacts[i]
		block := proto.Clone(src).(*walletrpc.CompactBlock)
		block.Height = uint64(startHeight + i)
		cloned = append(cloned, block)
	}
	return cloned
}

func TestCache(t *testing.T) {
	ensureCompactsLoaded(t)

	// Pretend Sapling starts at 289460.
	os.RemoveAll(unitTestPath)
	cache = NewBlockCache(unitTestPath, unitTestChain, 289460, 0)

	// Initially cache is empty.
	if cache.GetLatestHeight() != -1 {
		t.Fatal("unexpected GetLatestHeight")
	}
	if cache.firstBlock != 289460 {
		t.Fatal("unexpected initial firstBlock")
	}
	if cache.nextBlock != 289460 {
		t.Fatal("unexpected initial nextBlock")
	}
	fillCache(t)
	reorgCache(t)
	fillCache(t)

	// Simulate a restart to ensure the db files are read correctly.
	cache = NewBlockCache(unitTestPath, unitTestChain, 289460, -1)

	// Should still be 6 blocks.
	if cache.nextBlock != 289466 {
		t.Fatal("unexpected nextBlock height")
	}
	reorgCache(t)

	// Reorg to before the first block moves back to only the first block
	cache.Reorg(289459)
	if cache.latestHash != nil {
		t.Fatal("unexpected latestHash, should be nil")
	}
	if cache.nextBlock != 289460 {
		t.Fatal("unexpected nextBlock: ", cache.nextBlock)
	}

	// Clean up the test files.
	cache.Close()
	os.RemoveAll(unitTestPath)
}

func TestCacheRestartCorruptLastBlock(t *testing.T) {
	ensureCompactsLoaded(t)

	testPath := t.TempDir()
	cache = NewBlockCache(testPath, unitTestChain, 289460, 0)
	fillCache(t)
	cache.Sync()

	lastHeight := cache.nextBlock - 1
	lastOffset := cache.starts[lastHeight-cache.firstBlock]
	blocksName := cache.blocksName
	cache.Close()
	blocksFile, err := os.OpenFile(blocksName, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocksFile.WriteAt([]byte{0x7f}, lastOffset+8); err != nil {
		blocksFile.Close()
		t.Fatal(err)
	}
	if err := blocksFile.Close(); err != nil {
		t.Fatal(err)
	}

	cache = NewBlockCache(testPath, unitTestChain, 289460, -1)
	if cache.nextBlock != lastHeight {
		t.Fatalf("unexpected nextBlock after restart corruption recovery: got %d want %d", cache.nextBlock, lastHeight)
	}
	if cache.GetLatestHeight() != lastHeight-1 {
		t.Fatalf("unexpected latest height after restart corruption recovery: got %d want %d", cache.GetLatestHeight(), lastHeight-1)
	}

	cache.Close()
}

func TestCacheRestartCorruptFirstBlockRestartsFromStartHeight(t *testing.T) {
	ensureCompactsLoaded(t)

	const saplingHeight = 152855
	testPath := t.TempDir()
	localCompacts := cloneCompactsWithStartHeight(saplingHeight, 4)

	cache = NewBlockCache(testPath, unitTestChain, saplingHeight, 0)
	cache.Reorg(saplingHeight)
	for i, compact := range localCompacts {
		if err := cache.Add(saplingHeight+i, compact); err != nil {
			t.Fatal(err)
		}
	}
	cache.Sync()

	blocksName := cache.blocksName
	cache.Close()

	blocksFile, err := os.OpenFile(blocksName, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocksFile.WriteAt([]byte{0x7f}, 8); err != nil {
		blocksFile.Close()
		t.Fatal(err)
	}
	if err := blocksFile.Close(); err != nil {
		t.Fatal(err)
	}

	cache = NewBlockCache(testPath, unitTestChain, saplingHeight, -1)
	if cache.nextBlock != saplingHeight {
		t.Fatalf("unexpected nextBlock after first-block corruption recovery: got %d want %d", cache.nextBlock, saplingHeight)
	}
	if cache.GetLatestHeight() != -1 {
		t.Fatalf("unexpected latest height after first-block corruption recovery: got %d want -1", cache.GetLatestHeight())
	}
	if cache.latestHash != nil {
		t.Fatal("expected latestHash to be nil after first-block corruption recovery")
	}

	cache.Close()
}

func TestCacheRestartWithDifferentConfiguredStartHeightUsesMetadata(t *testing.T) {
	ensureCompactsLoaded(t)

	testPath := t.TempDir()
	cache = NewBlockCache(testPath, unitTestChain, 289460, 0)
	fillCache(t)
	cache.Sync()
	cache.Close()

	// Reopen the same cache files while configured with a different start height.
	// The persisted cache metadata should preserve the original firstBlock so the
	// cache continues instead of being reinterpreted as corrupt.
	cache = NewBlockCache(testPath, unitTestChain, 289461, -1)
	if cache.firstBlock != 289460 {
		t.Fatalf("unexpected firstBlock after metadata restore: got %d want %d", cache.firstBlock, 289460)
	}
	if cache.nextBlock != 289466 {
		t.Fatalf("unexpected nextBlock after metadata restore: got %d want %d", cache.nextBlock, 289466)
	}
	if cache.GetLatestHeight() != 289465 {
		t.Fatalf("unexpected latest height after metadata restore: got %d want %d", cache.GetLatestHeight(), 289465)
	}
	if cache.latestHash == nil {
		t.Fatal("expected latestHash to be preserved after metadata restore")
	}

	cache.Close()
}

func reorgCache(t *testing.T) {
	// Simulate a reorg by adding a block whose height is lower than the latest;
	// we're replacing the second block, so there should be only two blocks.
	cache.Reorg(289461)
	err := cache.Add(289461, compacts[1])
	if err != nil {
		t.Fatal(err)
	}
	if cache.firstBlock != 289460 {
		t.Fatal("unexpected firstBlock height")
	}
	if cache.nextBlock != 289462 {
		t.Fatal("unexpected nextBlock height")
	}
	if len(cache.starts) != 3 {
		t.Fatal("unexpected len(cache.starts)")
	}

	// some "black-box" tests (using exported interfaces)
	if cache.GetLatestHeight() != 289461 {
		t.Fatal("unexpected GetLatestHeight")
	}
	if int(cache.Get(289461).Height) != 289461 {
		t.Fatal("unexpected block contents")
	}

	// Make sure we can go forward from here
	err = cache.Add(289462, compacts[2])
	if err != nil {
		t.Fatal(err)
	}
	if cache.firstBlock != 289460 {
		t.Fatal("unexpected firstBlock height")
	}
	if cache.nextBlock != 289463 {
		t.Fatal("unexpected nextBlock height")
	}
	if len(cache.starts) != 4 {
		t.Fatal("unexpected len(cache.starts)")
	}

	if cache.GetLatestHeight() != 289462 {
		t.Fatal("unexpected GetLatestHeight")
	}
	if int(cache.Get(289462).Height) != 289462 {
		t.Fatal("unexpected block contents")
	}
}

// Whatever the state of the cache, add 6 blocks starting at the
// pretend Sapling height, 289460 (this could cause a reorg).
func fillCache(t *testing.T) {
	next := 289460
	cache.Reorg(next)
	for i, compact := range compacts {
		err := cache.Add(next, compact)
		if err != nil {
			t.Fatal(err)
		}
		next++

		// some "white-box" checks
		if cache.firstBlock != 289460 {
			t.Fatal("unexpected firstBlock height")
		}
		if cache.nextBlock != 289460+i+1 {
			t.Fatal("unexpected nextBlock height")
		}
		if len(cache.starts) != i+2 {
			t.Fatal("unexpected len(cache.starts)")
		}

		// some "black-box" tests (using exported interfaces)
		if cache.GetLatestHeight() != 289460+i {
			t.Fatal("unexpected GetLatestHeight")
		}
		b := cache.Get(289460 + i)
		if b == nil {
			t.Fatal("unexpected Get failure")
		}
		if int(b.Height) != 289460+i {
			t.Fatal("unexpected block contents")
		}
	}
}
