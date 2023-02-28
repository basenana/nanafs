package files

import (
	"bytes"
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
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

func (f chainFactory) build(obj *types.Object, attr Attr) chain {
	if attr.Direct {
		return f.buildDirectChain(obj)
	}
	return f.buildPageChain(obj)
}

func (f chainFactory) buildPageChain(obj *types.Object) chain {
	return &pageCacheChain{
		root:   &pageRoot{},
		data:   f.buildChunkChain(obj),
		logger: logger.NewLogger("pageChain").With(zap.Int64("key", obj.ID)),
	}
}

func (f chainFactory) buildChunkChain(obj *types.Object) chain {
	switch f.s.ID() {
	case storage.MemoryStorage, storage.LocalStorage:
		return f.buildDirectChain(obj)
	default:
		return f.buildLocalCacheChain(obj)
	}
}

func (f chainFactory) buildLocalCacheChain(obj *types.Object) chain {
	c := &chunkCacheChain{
		key:      obj.ID,
		cacheDir: f.cfg.CacheDir,
		mode:     0,
		mapping:  map[string]cacheInfo{},
		bufQ:     newBuf(),
		storage:  f.s,
		logger:   logger.NewLogger("chunkChain").With(zap.Int64("key", obj.ID)),
	}
	return c
}

func (f chainFactory) buildDirectChain(obj *types.Object) chain {
	return &chunkDirectChain{
		key:     obj.ID,
		storage: f.s,
		opens:   map[int64]io.ReadWriteSeeker{},
		logger:  logger.NewLogger("chunkChain").With(zap.Int64("key", obj.ID)),
	}
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
	case p.mode&pageModeInternal > 0:
		p.slots = make([]*pageNode, pageTreeSize)
	}
	return p
}

type pageCacheChain struct {
	enable   bool
	root     *pageRoot
	farthest int64
	data     chain
	mux      sync.Mutex
	logger   *zap.SugaredLogger
}

var _ chain = &pageCacheChain{}

func (p *pageCacheChain) readAt(ctx context.Context, index, off int64, data []byte) (n int, err error) {
	if off > p.farthest {
		p.farthest = off
	}

	if !p.enable && off < p.farthest {
		p.enable = true
	}

	if !p.enable {
		return p.data.readAt(ctx, index, off, data)
	}

	var (
		bufSize  = len(data)
		readOnce int
		page     *pageNode
	)
	ctx, endF := utils.TraceTask(ctx, "pagecache.read")
	defer endF()

	for {
		pageIndex, pagePos := computePageIndex(index, off)
		pageStart := (off / pageSize) * pageSize
		if pageStart >= fileChunkSize {
			break
		}
		page = p.findPage(pageIndex)
		if page == nil {
			page, err = p.readUncachedData(ctx, index, pageIndex, pageStart)
			if err != nil {
				return
			}
		}

		readOnce = copy(data[n:], page.data[pagePos:page.length])
		n += readOnce
		if n == bufSize {
			break
		}
		off += int64(readOnce)
		if off >= fileChunkSize {
			break
		}
	}
	return
}

func (p *pageCacheChain) writeAt(ctx context.Context, index int64, off int64, data []byte) (n int, err error) {
	if !p.enable {
		return p.data.writeAt(ctx, index, off, data)
	}

	var (
		bufSize   = int64(len(data))
		onceWrite int
	)
	ctx, endF := utils.TraceTask(ctx, "pagecache.write")
	defer endF()

	for {
		pageIndex, pagePos := computePageIndex(index, off)
		pageStart := (off / pageSize) * pageSize
		if pageStart >= fileChunkSize {
			break
		}
		page := p.findPage(pageIndex)
		if page == nil {
			page, err = p.readUncachedData(ctx, index, pageIndex, pageStart)
			if err != nil {
				return
			}
		}

		onceWrite = copy(page.data[pagePos:], data[n:])
		if page.mode&pageModeDirty == 0 {
			page.mode |= pageModeDirty
			p.root.dirtyCount += 1
		}
		page.length = int(pagePos) + onceWrite
		p.commitDirtyPage(ctx, index, pageStart, page)
		n += onceWrite

		if int64(n) == bufSize {
			break
		}

		off += int64(onceWrite)
	}
	return
}

func (p *pageCacheChain) readUncachedData(ctx context.Context, chunkIndex, pageIndex, pageStartOff int64) (result *pageNode, err error) {
	var (
		n       int
		page    *pageNode
		preRead = 2 // TODO: need a pre read control
	)
	for preRead >= 0 {
		data := make([]byte, pageSize)
		n, err = p.data.readAt(ctx, chunkIndex, pageStartOff, data)
		if err != nil && err != types.ErrNotFound && err != io.EOF {
			if result == nil {
				p.logger.Panicw("read uncached data failed", "err", err.Error())
			}
			break
		}
		page = p.insertPage(pageIndex, data, n)
		if result == nil {
			result = page
		}
		pageIndex += 1
		pageStartOff += pageSize
		if pageStartOff >= fileChunkSize {
			break
		}
		preRead -= 1
	}
	return result, nil
}

func (p *pageCacheChain) commitDirtyPage(ctx context.Context, index, offset int64, node *pageNode) {
	ctx, endF := utils.TraceTask(ctx, "pagecache.commit")
	defer endF()

	if node.mode&pageModeData == 0 || node.mode&pageModeDirty == 0 {
		return
	}
	p.root.dirtyCount -= 1
	_, err := p.data.writeAt(ctx, index, offset, node.data)
	if err != nil {
		p.logger.Errorw("commit dirty page error", "err", err.Error())
		return
	}
}

func (p *pageCacheChain) close(ctx context.Context) (err error) {
	ctx, endF := utils.TraceTask(ctx, "pagecache.close")
	defer endF()

	return p.data.close(ctx)
}

func (p *pageCacheChain) insertPage(pageIdx int64, data []byte, dataLen int) *pageNode {
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
	dataNode.data = data
	dataNode.length = dataLen
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
	chunkChainClosedMode = 1 << 0
)

type chunkCacheChain struct {
	key      int64
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
	ctx, endF := utils.TraceTask(ctx, "cache.read")
	defer endF()
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
	return
}

func (l *chunkCacheChain) writeAt(ctx context.Context, index int64, off int64, data []byte) (n int, err error) {
	ctx, endF := utils.TraceTask(ctx, "cache.write")
	defer endF()
	err = l.bufQ.put(index, off, data)
	n = len(data)
	return
}

func (l *chunkCacheChain) close(ctx context.Context) (err error) {
	ctx, endF := utils.TraceTask(ctx, "cache.close")
	defer endF()

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
			if l.mode&chunkChainClosedMode > 0 {
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

		_, err = cInfo.f.WriteAt(r.data, r.offset)
		if err != nil {
			l.logger.Errorw("write cached file error", "index", r.index, "err", err.Error())
			return
		}
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

func (l *chunkCacheChain) cacheKey(key string, off int64) string {
	return fmt.Sprintf("%s_%d", key, off)
}

func (l *chunkCacheChain) cachePath(off int64) string {
	return path.Join(l.cacheDir, fmt.Sprintf("%d/%d", l.key, off))
}

type chunkDirectChain struct {
	key     int64
	opens   map[int64]io.ReadWriteSeeker
	storage storage.Storage
	logger  *zap.SugaredLogger
	mux     sync.Mutex
}

var _ chain = &chunkDirectChain{}

func (d *chunkDirectChain) readAt(ctx context.Context, index, off int64, data []byte) (n int, err error) {
	ctx, endF := utils.TraceTask(ctx, "direct.read")
	defer endF()

	d.mux.Lock()
	defer d.mux.Unlock()

	cached, ok := d.opens[index]
	if ok {
		if _, err = cached.Seek(off, io.SeekStart); err != nil {
			d.logger.Errorw("cached reader seed failed", "err", err.Error())
			return
		}
		return cached.Read(data)
	}

	chunkDataReader, err := d.storage.Get(ctx, d.key, index, off)
	if err != nil {
		if err != types.ErrNotFound {
			d.logger.Errorw("read through error", "index", index, "err", err.Error())
			return 0, err
		}
		// for truncate
		return len(data), nil
	}

	if seekCloser, needCache := chunkDataReader.(io.ReadWriteSeeker); needCache {
		d.opens[index] = seekCloser
	}

	return chunkDataReader.Read(data)
}

func (d *chunkDirectChain) writeAt(ctx context.Context, index int64, off int64, data []byte) (n int, err error) {
	ctx, endF := utils.TraceTask(ctx, "direct.write")
	defer endF()

	d.mux.Lock()
	defer d.mux.Unlock()
	n = len(data)
	if n > fileChunkSize {
		n = fileChunkSize
	}
	cached, ok := d.opens[index]
	if ok {
		if _, err = cached.Seek(off, io.SeekStart); err != nil {
			d.logger.Errorw("cached reader seed failed", "err", err.Error())
			return
		}
		return cached.Write(data[:n])
	}

	err = d.storage.Put(ctx, d.key, index, off, bytes.NewReader(data[:n]))
	return
}

func (d *chunkDirectChain) close(ctx context.Context) error {
	ctx, endF := utils.TraceTask(ctx, "direct.close")
	defer endF()

	for _, seeker := range d.opens {
		if closer, ok := seeker.(io.Closer); ok {
			_ = closer.Close()
		}
	}
	return nil
}

type dummyChain struct {
}

func (d dummyChain) readAt(ctx context.Context, index, off int64, data []byte) (n int, err error) {
	ctx, endF := utils.TraceTask(ctx, "dummy.read")
	defer endF()

	return 0, nil
}

func (d dummyChain) writeAt(ctx context.Context, index int64, off int64, data []byte) (n int, err error) {
	ctx, endF := utils.TraceTask(ctx, "dummy.write")
	defer endF()

	return len(data), nil
}

func (d dummyChain) close(ctx context.Context) error {
	ctx, endF := utils.TraceTask(ctx, "dummy.close")
	defer endF()

	return nil
}

var _ chain = dummyChain{}
