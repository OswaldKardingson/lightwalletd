// Copyright (c) 2019-2020 The Zcash developers
// Copyright (c) 2019-2024 Pirate Chain developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or https://www.opensource.org/licenses/mit-license.php .

// Package common contains utilities that are shared by other packages.
package common

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/PirateNetwork/lightwalletd/walletrpc"
	"google.golang.org/protobuf/proto"
)

// BlockCache contains a consecutive set of recent compact blocks in marshalled form.
type BlockCache struct {
	lengthsName, blocksName string // pathnames
	lengthsFile, blocksFile *os.File
	starts                  []int64 // Starting offset of each block within blocksFile
	firstBlock              int     // height of the first block in the cache (usually Sapling activation)
	nextBlock               int     // height of the first block not in the cache
	latestHash              []byte  // hash of the most recent (highest height) block, for detecting reorgs.
	mutex                   sync.RWMutex
}

// ---------- LRU Caching for Merkle Subtrees and Deltas ----------

// LRUCache represents a thread-safe LRU cache
type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	list     *list.List
	mutex    sync.RWMutex
}

type cacheEntry struct {
	key   string
	value interface{}
}

// NewLRUCache creates a new LRU cache with the given capacity
func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		panic("LRUCache capacity must be greater than 0")
	}
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		list:     list.New(),
	}
}

// Get retrieves a value from the cache and marks it as recently used
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if elem, ok := c.cache[key]; ok {
		// Move the accessed element to the front (most recently used)
		c.list.MoveToFront(elem)
		return elem.Value.(*cacheEntry).value, true
	}
	return nil, false
}

// Put adds a value to the cache. If the key already exists, it updates the value and marks it as recently used.
func (c *LRUCache) Put(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if elem, ok := c.cache[key]; ok {
		// Key exists: update the value and mark as recently used
		elem.Value.(*cacheEntry).value = value
		c.list.MoveToFront(elem)
	} else {
		// Key does not exist: add new entry
		if c.list.Len() == c.capacity {
			// Cache is full: evict the least recently used item
			evicted := c.list.Back()
			if evicted != nil {
				c.list.Remove(evicted)
				delete(c.cache, evicted.Value.(*cacheEntry).key)
			}
		}

		// Add the new entry to the cache
		entry := &cacheEntry{key: key, value: value}
		elem := c.list.PushFront(entry)
		c.cache[key] = elem
	}
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.list.Remove(elem)
		delete(c.cache, key)
	}
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.list.Init()
	c.cache = make(map[string]*list.Element)
}

// ---------- Specialized Caches for Merkle Subtrees and Deltas ----------

var (
	merkleSubtreeCache *LRUCache
	merkleDeltaCache   *LRUCache
	once               sync.Once
)

// InitializeMerkleCaches initializes the LRU caches for Merkle subtrees and deltas
func InitializeMerkleCaches(subtreeCapacity, deltaCapacity int) {
	once.Do(func() {
		merkleSubtreeCache = NewLRUCache(subtreeCapacity)
		merkleDeltaCache = NewLRUCache(deltaCapacity)
	})
}

// SetMerkleSubtreeCache stores a Merkle subtree in the cache
func SetMerkleSubtreeCache(key string, value interface{}) {
	merkleSubtreeCache.Put(key, value)
}

// GetMerkleSubtreeCache retrieves a Merkle subtree from the cache
func GetMerkleSubtreeCache(key string) (interface{}, bool) {
	return merkleSubtreeCache.Get(key)
}

// SetMerkleDeltaCache stores deltas in the cache
func SetMerkleDeltaCache(key string, value interface{}) {
	merkleDeltaCache.Put(key, value)
}

// GetMerkleDeltaCache retrieves deltas from the cache
func GetMerkleDeltaCache(key string) (interface{}, bool) {
	return merkleDeltaCache.Get(key)
}

// ---------- Existing BlockCache Logic Continues Below ----------

// GetNextHeight returns the height of the lowest unobtained block.
func (c *BlockCache) GetNextHeight() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.nextBlock
}

// GetFirstHeight returns the height of the lowest block (usually Sapling activation).
func (c *BlockCache) GetFirstHeight() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.firstBlock
}

// GetLatestHash returns the hash (block ID) of the most recent (highest) known block.
func (c *BlockCache) GetLatestHash() []byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.latestHash
}

// HashMatch indicates if the given prev-hash matches the most recent block's hash
// so reorgs can be detected.
func (c *BlockCache) HashMatch(prevhash []byte) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.latestHash == nil || bytes.Equal(c.latestHash, prevhash)
}

// Make the block at the given height the lowest height that we don't have.
// In other words, wipe out this height and beyond.
// This should never increase the size of the cache, only decrease.
// Caller should hold c.mutex.Lock().
func (c *BlockCache) setDbFiles(height int) {
	if height <= c.nextBlock {
		if height < c.firstBlock {
			height = c.firstBlock
		}
		index := height - c.firstBlock
		if err := c.lengthsFile.Truncate(int64(index * 4)); err != nil {
			Log.Fatal("truncate lengths file failed: ", err)
		}
		if err := c.blocksFile.Truncate(c.starts[index]); err != nil {
			Log.Fatal("truncate blocks file failed: ", err)
		}
		c.Sync()
		c.starts = c.starts[:index+1]
		c.nextBlock = height
		c.setLatestHash()
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// Caller should hold c.mutex.Lock().
func (c *BlockCache) recoverFromCorruption(height int) {
	Log.Warning("CORRUPTION detected in db blocks-cache files, height ", height, " redownloading")

	// Save the corrupted files for post-mortem analysis.
	save := c.lengthsName + "-corrupted"
	if err := copyFile(c.lengthsName, save); err != nil {
		Log.Warning("Could not copy db lengths file: ", err)
	}
	save = c.blocksName + "-corrupted"
	if err := copyFile(c.blocksName, save); err != nil {
		Log.Warning("Could not copy db lengths file: ", err)
	}

	c.setDbFiles(height)
}

// not including the checksum
func (c *BlockCache) blockLength(height int) int {

	//Don't check block that will be out of index
	if height < c.firstBlock || height >= c.nextBlock {
		return 0
	}

	index := height - c.firstBlock
	return int(c.starts[index+1] - c.starts[index] - 8)
}

// Calculate the 8-byte checksum that precedes each block in the blocks file.
func checksum(height int, b []byte) []byte {
	h := make([]byte, 8)
	binary.LittleEndian.PutUint64(h, uint64(height))
	cs := fnv.New64a()
	cs.Write(h)
	cs.Write(b)
	return cs.Sum(nil)
}

// Caller should hold (at least) c.mutex.RLock().
func (c *BlockCache) readBlock(height int) *walletrpc.CompactBlock {
	blockLen := c.blockLength(height)
	b := make([]byte, blockLen+8)
	offset := c.starts[height-c.firstBlock]
	n, err := c.blocksFile.ReadAt(b, offset)
	if err != nil || n != len(b) {
		Log.Warning("blocks read offset: ", offset, " failed: ", n, err)
		return nil
	}
	diskcs := b[:8]
	b = b[8 : blockLen+8]
	if !bytes.Equal(checksum(height, b), diskcs) {
		Log.Warning("bad block checksum at height: ", height, " offset: ", offset)
		return nil
	}
	block := &walletrpc.CompactBlock{}
	err = proto.Unmarshal(b, block)
	if err != nil {
		// Could be file corruption.
		Log.Warning("blocks unmarshal at offset: ", offset, " failed: ", err)
		return nil
	}
	if int(block.Height) != height {
		// Could be file corruption.
		Log.Warning("block unexpected height at height ", height, " offset: ", offset)
		return nil
	}
	return block
}

// Caller should hold c.mutex.Lock().
func (c *BlockCache) setLatestHash() {
	c.latestHash = nil
	// There is at least one block; get the last block's hash
	if c.nextBlock > c.firstBlock {
		// At least one block remains; get the last block's hash
		block := c.readBlock(c.nextBlock - 1)
		if block == nil {
			c.recoverFromCorruption(c.nextBlock - 10000)
			return
		}
		c.latestHash = make([]byte, len(block.Hash))
		copy(c.latestHash, block.Hash)
	}
}

// Reset is used only for darkside testing.
func (c *BlockCache) Reset(startHeight int) {
	c.setDbFiles(c.firstBlock) // empty the cache
	c.firstBlock = startHeight
	c.nextBlock = startHeight
}

// NewBlockCache returns an instance of a block cache object.
// (No locking here, we assume this is single-threaded.)
// syncFromHeight < 0 means latest (tip) height.
func NewBlockCache(dbPath string, chainName string, startHeight int, syncFromHeight int) *BlockCache {
	c := &BlockCache{}
	c.firstBlock = startHeight
	c.nextBlock = startHeight
	c.lengthsName, c.blocksName = dbFileNames(dbPath, chainName)
	var err error
	if err := os.MkdirAll(filepath.Join(dbPath, chainName), 0755); err != nil {
		Log.Fatal("mkdir ", dbPath, " failed: ", err)
	}
	c.blocksFile, err = os.OpenFile(c.blocksName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		Log.Fatal("open ", c.blocksName, " failed: ", err)
	}
	c.lengthsFile, err = os.OpenFile(c.lengthsName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		Log.Fatal("open ", c.lengthsName, " failed: ", err)
	}
	lengths, err := os.ReadFile(c.lengthsName)
	if err != nil {
		Log.Fatal("read ", c.lengthsName, " failed: ", err)
	}
	// 4 bytes per lengths[] value (block length)
	if syncFromHeight >= 0 {
		if syncFromHeight < startHeight {
			syncFromHeight = startHeight
		}
		if (syncFromHeight-startHeight)*4 < len(lengths) {
			// discard the entries at and beyond (newer than) the specified height
			lengths = lengths[:(syncFromHeight-startHeight)*4]
		}
	}

	// The last entry in starts[] is where to write the next block.
	var offset int64
	c.starts = nil
	c.starts = append(c.starts, 0)
	for i := 0; i < len(lengths)/4; i++ {
		if len(lengths[:4]) < 4 {
			Log.Warning("lengths file has a partial entry")
			c.recoverFromCorruption(c.nextBlock)
			break
		}
		length := binary.LittleEndian.Uint32(lengths[i*4 : (i+1)*4])
		if length < 74 || length > 4*1000*1000 {
			Log.Warning("lengths file has impossible value ", length)
			c.recoverFromCorruption(c.nextBlock)
			break
		}
		offset += int64(length) + 8
		c.starts = append(c.starts, offset)
		// Check for corruption.
		block := c.readBlock(c.nextBlock)
		if block == nil {
			Log.Warning("error reading block")
			c.recoverFromCorruption(c.nextBlock)
			break
		}
		c.nextBlock++
	}
	c.setDbFiles(c.nextBlock)
	Log.Info("Found ", c.nextBlock-c.firstBlock, " blocks in cache")
	return c
}

func dbFileNames(dbPath string, chainName string) (string, string) {
	return filepath.Join(dbPath, chainName, "lengths"),
		filepath.Join(dbPath, chainName, "blocks")
}

// Add adds the given block to the cache at the given height, returning true
// if a reorg was detected.
func (c *BlockCache) Add(height int, block *walletrpc.CompactBlock) error {
	// Invariant: m[firstBlock..nextBlock) are valid.
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if height > c.nextBlock {
		// Cache has been reset (for example, checksum error)
		return nil
	}
	if height < c.firstBlock {
		// Should never try to add a block before Sapling activation height
		Log.Fatal("cache.Add height below Sapling: ", height)
		return nil
	}
	if height < c.nextBlock {
		// Should never try to "backup" (call Reorg() instead).
		Log.Fatal("cache.Add height going backwards: ", height)
		return nil
	}
	bheight := int(block.Height)

	if bheight != height {
		// This could only happen if pirated returned the wrong
		// block (not the height we requested).
		Log.Fatal("cache.Add wrong height: ", bheight, " expecting: ", height)
		return nil
	}

	// Add the new block and its length to the db files.
	data, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	b := append(checksum(height, data), data...)
	n, err := c.blocksFile.Write(b)
	if err != nil {
		Log.Fatal("blocks write failed: ", err)
	}
	if n != len(b) {
		Log.Fatal("blocks write incorrect length: expected: ", len(b), "written: ", n)
	}
	b = make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(len(data)))
	n, err = c.lengthsFile.Write(b)
	if err != nil {
		Log.Fatal("lengths write failed: ", err)
	}
	if n != len(b) {
		Log.Fatal("lengths write incorrect length: expected: ", len(b), "written: ", n)
	}

	// update the in-memory variables
	offset := c.starts[len(c.starts)-1]
	c.starts = append(c.starts, offset+int64(len(data)+8))

	if c.latestHash == nil {
		c.latestHash = make([]byte, len(block.Hash))
	}
	copy(c.latestHash, block.Hash)
	c.nextBlock++
	// Invariant: m[firstBlock..nextBlock) are valid.
	return nil
}

// Reorg resets nextBlock (the block that should be Add()ed next)
// downward to the given height.
func (c *BlockCache) Reorg(height int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Allow the caller not to have to worry about Sapling start height.
	if height < c.firstBlock {
		height = c.firstBlock
	}
	if height >= c.nextBlock {
		// Timing window, ignore this request
		return
	}
	// Remove the end of the cache.
	c.nextBlock = height
	newCacheLen := height - c.firstBlock
	c.starts = c.starts[:newCacheLen+1]

	if err := c.lengthsFile.Truncate(int64(4 * newCacheLen)); err != nil {
		Log.Fatal("truncate failed: ", err)
	}
	if err := c.blocksFile.Truncate(c.starts[newCacheLen]); err != nil {
		Log.Fatal("truncate failed: ", err)
	}
	c.setLatestHash()
}

// Get returns the compact block at the requested height if it's
// in the cache, else nil.
func (c *BlockCache) Get(height int) *walletrpc.CompactBlock {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if height < c.firstBlock || height >= c.nextBlock {
		return nil
	}
	block := c.readBlock(height)
	if block == nil {
		go func() {
			// We hold only the read lock, need the exclusive lock.
			c.mutex.Lock()
			c.recoverFromCorruption(height - 10000)
			c.mutex.Unlock()
		}()
		return nil
	}
	return block
}

// GetLatestHeight returns the height of the most recent block, or -1
// if the cache is empty.
func (c *BlockCache) GetLiteWalletBlockGroup(height int) *walletrpc.BlockID {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	targetLength := 4000000
	groupLength := 0

	if height < c.firstBlock || height >= c.nextBlock {
		return nil
	}

	for groupLength < targetLength {
		groupLength += c.blockLength(height)
		height++
		if height >= c.nextBlock {
			height--
			break
		}
	}

	block := c.readBlock(height)

	return &walletrpc.BlockID{Height: uint64(height), Hash: make([]byte, len(block.Hash))}
}

// GetLatestHeight returns the height of the most recent block, or -1
// if the cache is empty.
func (c *BlockCache) GetLatestHeight() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.firstBlock == c.nextBlock {
		return -1
	}
	return c.nextBlock - 1
}

// Sync ensures that the db files are flushed to disk, can be called unnecessarily.
func (c *BlockCache) Sync() {
	c.lengthsFile.Sync()
	c.blocksFile.Sync()
}

// Close is Currently used only for testing.
func (c *BlockCache) Close() {
	// Some operating system require you to close files before you can remove them.
	if c.lengthsFile != nil {
		c.lengthsFile.Close()
		c.lengthsFile = nil
	}
	if c.blocksFile != nil {
		c.blocksFile.Close()
		c.blocksFile = nil
	}
}
