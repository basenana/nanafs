package files

import (
	"errors"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sync/atomic"
)

const (
	defaultChunkSize = 1 << 22 // 4MB
	pageSize         = 1 << 12 // 4k
	pageCacheLimit   = 1 << 22 // 4MB
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
	pageIsDirty = 1
)

type pageNode struct {
	prefix int

	date []byte
	len  int
	mode int8

	child *pageNode
	next  *pageNode
}

func insertPage(root *pageNode, pageIdx, off int64, data []byte) *pageNode {
	return nil
}

func findPage(root *pageNode, pageIdx int64) *pageNode {
	return nil
}

func commitDirtyPage(root *pageNode) error {
	return nil
}

var local *localCache

type localCache struct {
	path      string
	sizeLimit int64
	buf       *buf
	storage   storage.Storage
	logger    *zap.SugaredLogger
}

func (l *localCache) writeAt(key string, chunkIdx int64, off int64, data []byte) error {
	return nil
}

func (l *localCache) readAt(key string, off int64, data []byte) error {
	return nil
}

func InitLocalCache(cfg config.Config, storage storage.Storage) {
	local = &localCache{
		path:      cfg.CacheDir,
		sizeLimit: cfg.CacheSize,
		buf:       newBuf(),
		storage:   storage,
		logger:    logger.NewLogger("LocalCache"),
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
