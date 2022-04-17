package files

import (
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
	pageModeDirty = 1
	pageTreeShift = 6
	pageTreeSize  = 1 << pageTreeShift
	pageTreeMask  = pageTreeShift - 1
)

type pageNode struct {
	offsetPrefix int

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
	mux       sync.RWMutex
	logger    *zap.SugaredLogger
}

func (l *localCache) writeAt(key string, chunkIdx int64, off int64, data []byte) (err error) {
	l.mux.Lock()
	defer l.mux.Unlock()
	cacheKey := l.cachePath(key, chunkIdx)
	cInfo, ok := l.mapping[cacheKey]
	if !ok {
		cInfo = cacheInfo{key: key, idx: chunkIdx, updateAt: time.Now().UnixNano()}
		if err = l.fetchChunkWithLock(cInfo); err != nil {
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

func (l *localCache) readAt(key string, chunkIdx, off int64, data []byte) (err error) {
	l.mux.RLock()
	defer l.mux.RUnlock()
	cacheKey := l.cachePath(key, chunkIdx)
	cInfo, ok := l.mapping[cacheKey]
	if !ok {
		cInfo = cacheInfo{key: key, idx: chunkIdx, updateAt: time.Now().UnixNano()}
		if err = l.fetchChunkWithLock(cInfo); err != nil {
			return err
		}
		l.mapping[cacheKey] = cInfo
	}

	cacheFilePath := l.cachePath(cInfo.key, cInfo.idx)
	f, err := os.OpenFile(cacheFilePath, os.O_RDONLY, 0655)
	if err != nil {
		l.logger.Errorw("open cache file to read error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}
	defer f.Close()

	var n int
	n, err = f.ReadAt(data, off)
	if err != nil {
		if err != io.EOF {
			l.logger.Errorw("read cached file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
			return err
		}
	}

	l.logger.Debugw("read cached file", "key", cInfo.key, "file", cacheFilePath, "count", n)
	return nil
}

func (l *localCache) fetchChunkWithLock(cInfo cacheInfo) error {
	cacheFilePath := l.cachePath(cInfo.key, cInfo.idx)
	rc, err := l.storage.Get(context.Background(), cInfo.key, cInfo.idx)
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

func (l *localCache) uploadChunkWithLock(cInfo cacheInfo) (err error) {
	cacheFilePath := l.cachePath(cInfo.key, cInfo.idx)

	var f *os.File
	f, err = os.OpenFile(cacheFilePath, os.O_RDONLY, 0655)
	if err != nil {
		l.logger.Errorw("open cache file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}
	defer f.Close()

	if err = l.storage.Put(context.Background(), cInfo.key, f, cInfo.idx); err != nil {
		l.logger.Errorw("upload chunk file error", "key", cInfo.key, "file", cacheFilePath, "err", err.Error())
		return err
	}
	return nil
}

func (l *localCache) cacheKey(key string, off int64) string {
	return fmt.Sprintf("%s_%d", key, off)
}

func (l *localCache) cachePath(key string, off int64) string {
	return path.Join(l.path, fmt.Sprintf("%s/%d", key, off))
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
