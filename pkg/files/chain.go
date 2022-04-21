package files

import (
	"bytes"
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"sync"
	"time"
)

type chain interface {
	readAt(ctx context.Context, index, off int64, data []byte) (n int, err error)
	writeAt(ctx context.Context, index int64, off int64, data []byte) (n int, err error)
	close(ctx context.Context) error
}

var factory *chainFactory

type chainFactory struct {
	cfg config.Config
	s   storage.Storage
}

func (f chainFactory) build(obj *types.Object) chain {
	return f.buildPageChain(obj)
}

func (f chainFactory) buildPageChain(obj *types.Object) chain {
	return &pageCacheChain{
		root:   &pageRoot{},
		data:   f.buildChunkChain(obj),
		logger: logger.NewLogger("pageChain").With(zap.String("key", obj.ID)),
	}
}

func (f chainFactory) buildChunkChain(obj *types.Object) chain {
	c := &chunkCacheChain{
		key:      obj.ID,
		cacheDir: f.cfg.CacheDir,
		mode:     0,
		mapping:  map[string]cacheInfo{},
		bufQ:     newBuf(),
		storage:  f.s,
		logger:   logger.NewLogger("chunkChain").With(zap.String("key", obj.ID)),
	}
	switch f.s.ID() {
	case storage.MemoryStorage, storage.LocalStorage:
		c.mode |= chunkChainLocalMode
	}
	return c
}

func InitFileIoChain(cfg config.Config, s storage.Storage, stopCh chan struct{}) {
	factory = &chainFactory{
		cfg: cfg,
		s:   s,
	}
}

type pageRoot struct {
	rootNode   *pageNode
	totalCount int
	dirtyCount int
}

const (
	pageModeDirty    = 1
	pageModeInternal = 1 << 1
	pageModeData     = 1 << 2
	pageTreeShift    = 6
	pageTreeSize     = 1 << pageTreeShift
	pageTreeMask     = pageTreeSize - 1
)

type pageNode struct {
	index  int64
	slots  []*pageNode
	parent *pageNode
	length int
	data   []byte
	mode   int8
	shift  int
}

// TODO: need a page pool
func newPage(shift int, mode int8) *pageNode {
	p := &pageNode{
		shift: shift,
		mode:  mode,
	}
	switch {
	case p.mode&pageModeData > 0:
		p.data = make([]byte, pageSize)
	case p.mode&pageModeInternal > 0:
		p.slots = make([]*pageNode, pageTreeSize)
	}
	return p
}

type pageCacheChain struct {
	root   *pageRoot
	data   chain
	mux    sync.Mutex
	logger *zap.SugaredLogger
}

var _ chain = &pageCacheChain{}

func (p *pageCacheChain) readAt(ctx context.Context, index, off int64, data []byte) (n int, err error) {
	var (
		pageStart = off
		bufSize   = len(data)
		page      *pageNode
	)

	for {
		pageIndex, pos := computePageIndex(index, pageStart)
		pageEnd := pageStart + pageSize
		if pageEnd-pageStart > int64(bufSize-n) {
			pageEnd = pageStart + int64(bufSize-n)
		}
		page = p.findPage(pageIndex)
		if page == nil {
			page, err = p.readUncachedData(ctx, index, pageStart)
			if err != nil {
				return
			}
		}

		n += copy(data[n:], page.data[pos:pageEnd-pageStart])
		if n == len(data) {
			break
		}
		pageStart = pageEnd
	}
	return
}

func (p *pageCacheChain) writeAt(ctx context.Context, index int64, off int64, data []byte) (n int, err error) {
	var (
		pageStart = off
		bufSize   = int64(len(data))
	)

	for {
		pageIndex, pos := computePageIndex(index, pageStart)
		pageEnd := pageStart + pageSize
		if pageEnd-pageStart > bufSize-int64(n) {
			pageEnd = pageStart + bufSize - int64(n)
		}

		page := p.findPage(pageIndex)
		if page == nil {
			page, err = p.readUncachedData(ctx, index, pageStart)
			if err != nil {
				return
			}
		}

		copy(page.data[pos:], data[pageStart+pos:pageEnd])
		if page.mode&pageModeDirty == 0 {
			page.mode |= pageModeDirty
			p.root.dirtyCount += 1
		}
		p.commitDirtyPage(ctx, index, pageStart, page)

		n += len(data[pageStart:pageEnd])
		if n == len(data) {
			break
		}
		pageStart = pageEnd
	}
	return
}

func (p *pageCacheChain) readUncachedData(ctx context.Context, index, offset int64) (page *pageNode, err error) {
	var (
		data      = make([]byte, pageSize)
		pageStart = offset
		n         int
		preRead   = 0 // TODO: need a pre read control
	)
	for pageStart < fileChunkSize && preRead >= 0 {
		n, err = p.data.readAt(ctx, index, pageStart, data)
		if err != nil && err != types.ErrNotFound {
			break
		}
		page = p.insertPage(pageStart, data[:n])
		pageStart += pageSize
		preRead -= 1
	}
	return page, nil
}

func (p *pageCacheChain) commitDirtyPage(ctx context.Context, index, offset int64, node *pageNode) {
	if node.mode&pageModeData == 0 || node.mode&pageModeDirty == 0 {
		return
	}
	p.root.dirtyCount -= 1
	n, err := p.data.writeAt(ctx, index, offset, node.data)
	if err != nil {
		p.logger.Errorw("commit dirty page error", "err", err.Error())
		return
	}
	p.logger.Infow("commit dirty page finish", "count", n)
}

func (p *pageCacheChain) close(ctx context.Context) (err error) {
	return p.data.close(ctx)
}

func (p *pageCacheChain) insertPage(pageIdx int64, data []byte) *pageNode {
	p.mux.Lock()
	if p.root.rootNode == nil {
		p.root.rootNode = newPage(0, pageModeInternal)
	}

	if (pageTreeSize<<p.root.rootNode.shift)-1 < pageIdx {
		p.extendPageTree(pageIdx)
	}

	var (
		node  = p.root.rootNode
		shift = p.root.rootNode.shift
		slot  int64
	)
	for shift > 0 {
		slot = pageIdx >> node.shift & pageTreeMask
		next := node.slots[slot]
		if next == nil {
			next = newPage(shift-pageTreeShift, pageModeInternal)
			node.slots[slot] = next
		}
		node = next
		shift -= pageTreeShift
	}

	dataNode := newPage(0, pageModeData)
	dataNode.length = copy(dataNode.data, data)
	dataNode.mode |= pageModeDirty

	slot = pageIdx & pageTreeMask
	node.slots[slot] = dataNode

	p.root.totalCount += 1
	p.root.dirtyCount += 1
	p.mux.Unlock()
	return dataNode
}

func (p *pageCacheChain) findPage(pageIdx int64) *pageNode {
	if p.root.rootNode == nil {
		return nil
	}

	if (pageTreeSize<<p.root.rootNode.shift)-1 < pageIdx {
		return nil
	}

	p.mux.Lock()
	var (
		node  = p.root.rootNode
		shift = node.shift
		slot  int64
	)
	for shift >= 0 {
		slot = pageIdx >> node.shift & pageTreeMask
		next := node.slots[slot]
		if next == nil {
			p.mux.Unlock()
			return nil
		}
		node = next
		shift -= pageTreeShift
	}
	p.mux.Unlock()

	if node.mode&pageModeData > 0 {
		return node
	}

	return nil
}

func (p *pageCacheChain) extendPageTree(index int64) {
	var (
		node  = p.root.rootNode
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
		parent.shift = shift
		parent.slots[0] = node

		node.parent = parent
		node = parent
	}
	p.root.rootNode = node
}

const (
	chunkChainLocalMode  = 1
	chunkChainClosedMode = 1 << 1
)

type chunkCacheChain struct {
	key      string
	cacheDir string
	mode     int8
	mapping  map[string]cacheInfo
	bufQ     *ringbuf
	storage  storage.Storage
	mux      sync.Mutex
	logger   *zap.SugaredLogger
}

var _ chain = &chunkCacheChain{}

type cacheInfo struct {
	key      string
	idx      int64
	updateAt int64
	f        *os.File
}

func (l *chunkCacheChain) readAt(ctx context.Context, index, off int64, data []byte) (n int, err error) {
	if l.mode&chunkChainLocalMode > 0 {
		return l.readThroughAt(ctx, index, off, data)
	}
	l.mux.Lock()
	cacheKey := l.cachePath(index)
	cInfo, ok := l.mapping[cacheKey]
	if !ok {
		if cInfo, err = l.fetchChunkWithLock(ctx, index); err != nil {
			return
		}
	}
	l.mux.Unlock()
	n, err = cInfo.f.ReadAt(data, off)
	if err != nil {
		if err != io.EOF {
			l.logger.Errorw("read cached file error", "index", index, "err", err.Error())
			return
		}
	}
	l.logger.Debugw("read cached file", "count", n)
	return
}

func (l *chunkCacheChain) writeAt(ctx context.Context, index int64, off int64, data []byte) (n int, err error) {
	if l.mode&chunkChainLocalMode > 0 {
		return l.writeThroughAt(ctx, index, off, data)
	}

	err = l.bufQ.put(index, off, data)
	n = len(data)
	return
}

func (l *chunkCacheChain) close(ctx context.Context) (err error) {
	l.mux.Lock()
	l.mode |= chunkChainClosedMode
	for _, cInfo := range l.mapping {
		_ = cInfo.f.Close()
	}
	l.mux.Unlock()
	return nil
}

func (l *chunkCacheChain) sync() {
	var (
		err    error
		ticker = time.NewTicker(time.Second)
	)
	for {
		select {
		case <-ticker.C:
			if l.mode&chunkChainLocalMode > 0 {
				return
			}
		}
		r, ok := l.bufQ.pop()
		if !ok {
			continue
		}
		l.mux.Lock()
		cacheKey := l.cachePath(r.index)
		cInfo, ok := l.mapping[cacheKey]
		if !ok {
			if cInfo, err = l.fetchChunkWithLock(context.Background(), r.index); err != nil {
				continue
			}
		}
		l.mux.Unlock()

		var n int
		n, err = cInfo.f.WriteAt(r.data, r.offset)
		if err != nil {
			l.logger.Errorw("write cached file error", "index", r.index, "err", err.Error())
			return
		}

		l.logger.Debugw("write cached file", "index", r.index, "count", n)
	}
}

func (l *chunkCacheChain) fetchChunkWithLock(ctx context.Context, index int64) (cInfo cacheInfo, err error) {
	cacheFilePath := l.cachePath(cInfo.idx)
	rc, err := l.storage.Get(ctx, l.key, cInfo.idx, 0)
	if err != nil {
		l.logger.Errorw("fetch chunk error", "chunkIndex", cInfo.idx, "err", err.Error())
		return
	}
	defer rc.Close()

	f, err := os.OpenFile(cacheFilePath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0655)
	if err != nil {
		l.logger.Errorw("create cache file error", "file", cacheFilePath, "err", err.Error())
		return
	}

	n, err := io.Copy(f, rc)
	if err != nil {
		l.logger.Errorw("copy storage data to cache file error", "file", cacheFilePath, "err", err.Error())
		_ = f.Close()
		return
	}

	cInfo = cacheInfo{idx: index, updateAt: time.Now().UnixNano(), f: f}
	l.mapping[l.cachePath(index)] = cInfo
	l.logger.Debugw("fetch chunk data", "file", cacheFilePath, "count", n)
	return
}

func (l *chunkCacheChain) uploadChunkWithLock(ctx context.Context, cInfo cacheInfo) (err error) {
	cacheFilePath := l.cachePath(cInfo.idx)

	var f *os.File
	f, err = os.OpenFile(cacheFilePath, os.O_RDONLY, 0655)
	if err != nil {
		l.logger.Errorw("open cache file error", "file", cacheFilePath, "err", err.Error())
		return err
	}
	defer f.Close()

	if err = l.storage.Put(ctx, l.key, cInfo.idx, 0, f); err != nil {
		l.logger.Errorw("upload chunk file error", "file", cacheFilePath, "err", err.Error())
		return err
	}
	return nil
}

func (l *chunkCacheChain) writeThroughAt(ctx context.Context, chunkIdx int64, off int64, data []byte) (n int, err error) {
	chunkDataReader, err := l.storage.Get(ctx, l.key, chunkIdx, 0)
	if err != nil && err != types.ErrNotFound {
		l.logger.Errorw("write through error", "index", chunkIdx, "err", err.Error())
		return
	}

	chunkData := make([]byte, fileChunkSize)
	if chunkDataReader != nil {
		_, err = chunkDataReader.Read(chunkData)
		if err != nil {
			return
		}
	}

	copy(chunkData[off:], data)
	err = l.storage.Put(ctx, l.key, chunkIdx, 0, bytes.NewBuffer(chunkData))
	return len(chunkData), err
}

func (l *chunkCacheChain) readThroughAt(ctx context.Context, chunkIdx int64, off int64, data []byte) (int, error) {
	chunkDataReader, err := l.storage.Get(ctx, l.key, chunkIdx, 0)
	if err != nil {
		l.logger.Errorw("read through error", "index", chunkIdx, "err", err.Error())
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

func (l *chunkCacheChain) cacheKey(key string, off int64) string {
	return fmt.Sprintf("%s_%d", key, off)
}

func (l *chunkCacheChain) cachePath(off int64) string {
	return path.Join(l.cacheDir, fmt.Sprintf("%s/%d", l.key, off))
}
