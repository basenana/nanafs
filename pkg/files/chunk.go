package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultChunkSize = 1 << 22 // 4MB
	pageSize         = 1 << 12 // 4k
	pageCacheLimit   = 1 << 24 // 16MB
	bufQueueLen      = 256
)

var fileChunkSize int64 = defaultChunkSize

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}

func computePageIndex(off int64) (idx int64, pos int64) {
	idx = off / pageSize
	pos = off % pageSize
	return
}

type cRange struct {
	key    string
	index  int64
	offset int64
	limit  int64
	data   []byte
	errCh  chan error
}

const (
	pageModeDirty    = 1
	pageModeInternal = 1 << 1
	pageModeData     = 1 << 2
	pageTreeShift    = 6
	pageTreeSize     = 1 << pageTreeShift
	pageTreeMask     = pageTreeShift - 1
)

type pageRoot struct {
	rootNode   *pageNode
	totalCount int
	dirtyCount int
}

type pageNode struct {
	index  int64
	slots  []*pageNode
	parent *pageNode
	length int64
	data   []byte
	mode   int8
	shift  int
}

func insertPage(root *pageRoot, pageIdx int64, data []byte) *pageNode {
	if root.rootNode == nil {
		root.rootNode = newPage(0, pageModeData)
	}

	if (pageTreeSize<<root.rootNode.shift)-1 < pageIdx {
		extendPageTree(root, pageIdx)
	}

	var (
		node  = root.rootNode
		shift = root.rootNode.shift
		slot  int64
	)
	for shift >= 0 {
		slot = pageIdx >> node.shift & pageTreeMask
		next := node.slots[slot]
		if next == nil {
			var mode int8 = pageModeInternal
			if shift == 0 {
				mode = pageModeData | pageModeDirty
			}
			next = newPage(shift, mode)
			node.slots[slot] = next
		}
		node = next
		shift -= pageTreeShift
	}
	node.data = data
	node.length = int64(len(data))
	node.mode |= pageModeDirty
	root.totalCount += 1
	root.dirtyCount += 1
	return node
}

func findPage(root *pageRoot, pageIdx int64) *pageNode {
	if root.rootNode == nil {
		return nil
	}

	if (pageTreeSize<<root.rootNode.shift)-1 < pageIdx {
		return nil
	}

	var (
		node  = root.rootNode
		shift = node.shift
		slot  int64
	)

	for shift > 0 {
		slot = pageIdx >> node.shift & pageTreeMask
		next := node.slots[slot]
		if next == nil {
			return nil
		}
		node = next
		shift -= pageTreeShift
	}

	if node.mode&pageModeData > 0 {
		return node
	}

	return nil
}

func extendPageTree(root *pageRoot, index int64) {
	var (
		node  = root.rootNode
		shift = node.shift
	)

	// how max shift to extend
	maxShift := shift
	for index > (pageTreeSize<<maxShift)-1 {
		maxShift += pageTreeShift
	}
	for shift <= maxShift {
		shift += pageTreeShift
		parent := newPage(shift, pageModeInternal)
		node.parent = parent
		parent.slots[0] = node

		node = parent
	}
	root.rootNode = node
}

func commitDirtyPage(root *pageRoot) error {
	return nil
}

// TODO: need a page pool
func newPage(shift int, mode int8) *pageNode {
	return &pageNode{
		shift: shift,
		mode:  mode,
	}
}

var local *localCache

type cacheInfo struct {
	key      string
	idx      int64
	updateAt int64
}

type localCache struct {
	path      string
	sizeLimit int64
	buf       *buf
	mapping   map[string]cacheInfo
	storage   storage.Storage
	direct    bool // TODO: Delete this
	mux       sync.RWMutex
	logger    *zap.SugaredLogger
}

func (l *localCache) writeAt(ctx context.Context, key string, chunkIdx int64, off int64, data []byte) (err error) {
	if l.direct {
		return l.writeThroughAt(ctx, key, chunkIdx, off, data)
	}

	l.mux.Lock()
	defer l.mux.Unlock()
	cacheKey := l.cachePath(key, chunkIdx)
	cInfo, ok := l.mapping[cacheKey]
	if !ok {
		cInfo = cacheInfo{key: key, idx: chunkIdx, updateAt: time.Now().UnixNano()}
		if err = l.fetchChunkWithLock(ctx, cInfo); err != nil {
			return err
		}
		l.mapping[cacheKey] = cInfo
	}

	cacheFilePath := l.cachePath(cInfo.key, cInfo.idx)
	f, err := os.OpenFile(cacheFilePath, os.O_WRONLY, 0655)
	if err != nil {
		l.logger.Errorw("open cache file to update error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}
	defer f.Close()

	var n int
	n, err = f.WriteAt(data, off)
	if err != nil {
		l.logger.Errorw("write cached file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}

	l.logger.Debugw("write cached file", "key", cInfo.key, "file", cacheFilePath, "count", n)
	return nil
}

func (l *localCache) readAt(ctx context.Context, key string, chunkIdx, off int64, data []byte) (n int, err error) {
	if l.direct {
		return l.readThroughAt(ctx, key, chunkIdx, off, data)
	}
	l.mux.RLock()
	defer l.mux.RUnlock()
	cacheKey := l.cachePath(key, chunkIdx)
	cInfo, ok := l.mapping[cacheKey]
	if !ok {
		cInfo = cacheInfo{key: key, idx: chunkIdx, updateAt: time.Now().UnixNano()}
		if err = l.fetchChunkWithLock(ctx, cInfo); err != nil {
			return
		}
		l.mapping[cacheKey] = cInfo
	}

	cacheFilePath := l.cachePath(cInfo.key, cInfo.idx)
	f, err := os.OpenFile(cacheFilePath, os.O_RDONLY, 0655)
	if err != nil {
		l.logger.Errorw("open cache file to read error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return
	}
	defer f.Close()

	n, err = f.ReadAt(data, off)
	if err != nil {
		if err != io.EOF {
			l.logger.Errorw("read cached file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
			return
		}
	}

	l.logger.Debugw("read cached file", "key", cInfo.key, "file", cacheFilePath, "count", n)
	return
}

func (l *localCache) fetchChunkWithLock(ctx context.Context, cInfo cacheInfo) error {
	cacheFilePath := l.cachePath(cInfo.key, cInfo.idx)
	rc, err := l.storage.Get(ctx, cInfo.key, cInfo.idx, 0)
	if err != nil {
		l.logger.Errorw("fetch chunk error", "key", cInfo.key, "chunkIndex", cInfo.idx, "err", err.Error())
		return err
	}
	defer rc.Close()

	f, err := os.OpenFile(cacheFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0655)
	if err != nil {
		l.logger.Errorw("create cache file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}
	defer f.Close()

	n, err := io.Copy(f, rc)
	if err != nil {
		l.logger.Errorw("copy storage data to cache file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}
	l.logger.Debugw("fetch chunk data", "key", cInfo.key, "file", cacheFilePath, "count", n)
	return nil
}

func (l *localCache) uploadChunkWithLock(ctx context.Context, cInfo cacheInfo) (err error) {
	cacheFilePath := l.cachePath(cInfo.key, cInfo.idx)

	var f *os.File
	f, err = os.OpenFile(cacheFilePath, os.O_RDONLY, 0655)
	if err != nil {
		l.logger.Errorw("open cache file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}
	defer f.Close()

	if err = l.storage.Put(ctx, cInfo.key, cInfo.idx, 0, f); err != nil {
		l.logger.Errorw("upload chunk file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}
	return nil
}

func (l *localCache) writeThroughAt(ctx context.Context, key string, chunkIdx int64, off int64, data []byte) error {
	chunkDataReader, err := l.storage.Get(ctx, key, chunkIdx, 0)
	if err != nil {
		l.logger.Errorw("write through error", "key", key, "index", chunkIdx, "err", err.Error())
		return err
	}

	chunkData := make([]byte, fileChunkSize)
	_, err = chunkDataReader.Read(chunkData)
	if err != nil {
		return err
	}

	copy(chunkData[off:], data)
	err = l.storage.Put(ctx, key, chunkIdx, 0, bytes.NewBuffer(chunkData))
	return err
}

func (l *localCache) readThroughAt(ctx context.Context, key string, chunkIdx int64, off int64, data []byte) (int, error) {
	chunkDataReader, err := l.storage.Get(ctx, key, chunkIdx, 0)
	if err != nil {
		l.logger.Errorw("write through error", "key", key, "index", chunkIdx, "err", err.Error())
		return 0, err
	}

	chunkData := make([]byte, fileChunkSize)
	size, err := chunkDataReader.Read(chunkData)
	if err != nil {
		return 0, err
	}

	n := copy(data, chunkData[off:size])
	return n, err
}

func (l *localCache) cacheKey(key string, off int64) string {
	return fmt.Sprintf("%s_%d", key, off)
}

func (l *localCache) cachePath(key string, off int64) string {
	return path.Join(l.path, fmt.Sprintf("%s/%d", key, off))
}

func InitLocalCache(cfg config.Config, sto storage.Storage) {
	local = &localCache{
		path:      cfg.CacheDir,
		sizeLimit: cfg.CacheSize,
		buf:       newBuf(),
		storage:   sto,
		logger:    logger.NewLogger("LocalCache"),
	}

	// TODO: optimize for local storage
	switch sto.ID() {
	case storage.MemoryStorage, storage.LocalStorage:
		local.direct = true
	}
}

var (
	bufIsEmptyErr = errors.New("ring buffer is empty")
	bufIsFullErr  = errors.New("ring buffer is full")
)

type buf struct {
	head  uint32
	tail  uint32
	queue [bufQueueLen]cRange
	mask  uint32
}

func (b *buf) put(key string, index int64, offset int64, limit int64, data []byte) error {
	var tail, head, next uint32
	for {
		tail = atomic.LoadUint32(&b.tail)
		head = atomic.LoadUint32(&b.head)

		next = (tail + 1) & b.mask
		if next == head&b.mask {
			return bufIsFullErr
		}

		if atomic.CompareAndSwapUint32(&b.tail, tail, next) {
			break
		}
	}
	b.queue[next].key = key
	b.queue[next].index = index
	b.queue[next].offset = offset
	b.queue[next].limit = limit
	b.queue[next].data = data
	return nil
}

func (b *buf) pop() (cRange, error) {
	var tail, head, next uint32
	for {
		tail = atomic.LoadUint32(&b.tail)
		head = atomic.LoadUint32(&b.head)
		if tail == head {
			return cRange{}, bufIsEmptyErr
		}

		next = (head - 1) & b.mask
		if atomic.CompareAndSwapUint32(&b.head, head, next) {
			break
		}
	}
	return b.queue[next], nil
}

func (b *buf) len() int {
	return int(b.head - b.tail)
}

func newBuf() *buf {
	return &buf{
		queue: [bufQueueLen]cRange{},
		mask:  bufQueueLen - 1,
	}
}
