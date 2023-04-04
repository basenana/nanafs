/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package bio

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	fileChunkSize          = 1 << 26 // 64MB
	fileChunkCommitTimeout = time.Minute
)

var (
	maximumChunkTaskParallel = utils.NewMaximumParallel(100)
	fileChunkReaders         = make(map[int64]*chunkReader)
	fileChunkWriters         = make(map[int64]*chunkWriter)
	fileChunkMux             sync.Mutex
)

type chunkReader struct {
	*types.Object

	page    *pageCache
	store   storage.ChunkStore
	storage storage.Storage
	cache   *storage.LocalCache
	readers map[int64]*segReader
	readMux sync.Mutex
	ref     int32
	logger  *zap.SugaredLogger
}

func NewChunkReader(obj *types.Object, chunkStore storage.ChunkStore, dataStore storage.Storage) Reader {
	fileChunkMux.Lock()
	defer fileChunkMux.Unlock()

	cr, ok := fileChunkReaders[obj.ID]
	if ok {
		atomic.AddInt32(&cr.ref, 1)
		return cr
	}

	cr = &chunkReader{
		Object:  obj,
		page:    newPageCache(obj.ID, fileChunkSize),
		store:   chunkStore,
		storage: dataStore,
		cache:   storage.NewLocalCache(dataStore),
		readers: map[int64]*segReader{},
		ref:     1,
		logger:  logger.NewLogger("chunkIO").With("oid", obj.ID),
	}
	return cr
}

func (c *chunkReader) ReadAt(ctx context.Context, dest []byte, off int64) (n int64, err error) {
	ctx, endF := utils.TraceTask(ctx, "chunkreader.readat")
	defer endF()

	if off >= c.Object.Size {
		return 0, io.EOF
	}

	readEnd := off + int64(len(dest))
	if readEnd > c.Object.Size {
		readEnd = c.Object.Size
	}
	if readEnd == 0 {
		return
	}

	var (
		wg      = &sync.WaitGroup{}
		reqList = make([]*ioReq, 0, len(dest)/fileChunkSize+1)
	)
	for {
		index, _ := computeChunkIndex(off, fileChunkSize)
		chunkEnd := (index + 1) * fileChunkSize
		if chunkEnd > readEnd {
			chunkEnd = readEnd
		}

		readLen := chunkEnd - off
		wg.Add(1)
		reqList = append(reqList, c.prepareData(ctx, index, off, dest[n:n+readLen], wg))

		n += readLen
		off = chunkEnd
		if off == readEnd {
			break
		}
	}
	wg.Wait()
	for _, req := range reqList {
		if req.err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (c *chunkReader) prepareData(ctx context.Context, index, off int64, dest []byte, wg *sync.WaitGroup) *ioReq {
	req := &ioReq{
		WaitGroup: wg,
		off:       off,
		dest:      dest,
	}
	c.readMux.Lock()
	reader, ok := c.readers[index]
	if !ok {
		reader = &segReader{r: c, chunkID: index}
		c.readers[index] = reader
		c.logger.Debugw("builder segment reader", "entry", c.ID, "chunk", index)
	}
	c.readMux.Unlock()

	maximumChunkTaskParallel.Go(func() {
		defer req.Done()
		reader.readChunkRange(ctx, req)
	})
	return req
}

func (c *chunkReader) invalidate(index int64) {
	c.readMux.Lock()
	reader, ok := c.readers[index]
	c.readMux.Unlock()
	if ok {
		reader.mux.Lock()
		reader.st = nil
		reader.mux.Unlock()
	}
}

func (c *chunkReader) Close() {
	if atomic.AddInt32(&c.ref, -1) == 0 {
		fileChunkMux.Lock()
		delete(fileChunkReaders, c.ID)
		fileChunkMux.Unlock()
		c.page.close()
	}
}

type segReader struct {
	r       *chunkReader
	st      *segTree
	chunkID int64
	mux     sync.Mutex
}

func (c *segReader) readChunkRange(ctx context.Context, req *ioReq) {
	ctx, endF := utils.TraceTask(ctx, "segreader.readrange")
	defer endF()

	c.mux.Lock()
	if c.st == nil {
		segments, err := c.r.store.ListSegments(ctx, c.r.ID, c.chunkID)
		if err != nil {
			c.mux.Unlock()
			c.r.logger.Errorw("list segment reader", "entry", c.r.ID, "chunk", c.chunkID, "err", err)
			req.err = err
			return
		}

		dataSize := (c.chunkID + 1) * int64(fileChunkSize)
		if c.r.Size < dataSize {
			dataSize = c.r.Size
		}
		c.st = buildSegmentTree(c.chunkID*fileChunkSize, dataSize, segments)
	}
	st := c.st
	c.mux.Unlock()

	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			c.r.logger.Errorw("read chunk range panic", "entry", c.r.ID, "chunk", c.chunkID, "err", rErr)
			req.err = rErr
		}
	}()

	var (
		off          = req.off
		bufLeft      = int64(len(req.dest))
		pageIdx, pos int64
	)

	for {
		pageIdx, pos = computePageIndex(off)
		readStart := pageIdx*pageSize + pos
		readEnd := (pageIdx + 1) * pageSize
		if readEnd-off > bufLeft {
			readEnd = off + bufLeft
		}
		req.Add(1)
		go func(ctx context.Context, segments []segment, pageID, off int64, dest []byte) {
			maximumChunkTaskParallel.BlockedGo(func() {
				defer req.Done()
				if err := c.readPage(ctx, segments, pageID, off, dest); err != nil && req.err == nil {
					req.err = err
					endF()
					c.r.logger.Errorw("read chunk page error", "entry", c.r.ID, "chunk", c.chunkID, "page", pageID, "err", err)
					return
				}
			})
		}(ctx, st.query(pageIdx*pageSize, (pageIdx+1)*pageSize), pageIdx, readStart, req.dest[off-req.off:readEnd-req.off])
		bufLeft -= readEnd - off
		off = readEnd
		if bufLeft == 0 {
			break
		}
	}

	preRead := len(req.dest)/pageSize + 1
	maxPage := int64(fileChunkSize / pageSize)
	for preRead > 0 {
		preRead -= 1
		pageIdx += 1
		if pageIdx >= maxPage || pageIdx*pageSize > c.r.Size {
			break
		}
		go func(ctx context.Context, segments []segment, pageID int64) {
			maximumChunkTaskParallel.BlockedGo(func() {
				if err := c.readPage(ctx, segments, pageID, 0, nil); err != nil {
					c.r.logger.Errorw("pre-read chunk page error", "entry", c.r.ID, "chunk", c.chunkID, "page", pageIdx, "err", err)
					return
				}
			})
		}(ctx, st.query(pageIdx*pageSize, (pageIdx+1)*pageSize), pageIdx)
	}
}

func (c *segReader) readPage(ctx context.Context, segments []segment, pageIndex, off int64, dest []byte) (err error) {
	pageStart := pageSize * pageIndex
	var page *pageNode

	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			c.r.logger.Errorw("read chunk page panic", "entry", c.r.ID, "chunk", c.chunkID, "err", err)
			err = rErr
		}
	}()

	page, err = c.r.page.read(ctx, pageIndex, func(page *pageNode) error {
		var (
			crt, onceRead, readEnd int64
			innerErr               error
		)
		for _, seg := range segments {
			for i := crt; i < seg.off-pageStart; i++ {
				page.data[i] = 0
			}
			crt = seg.off - pageStart
			readEnd = crt + seg.len
			if readEnd > pageSize {
				readEnd = pageSize
			}
			if seg.id == 0 {
				for i := crt; i < readEnd; i++ {
					page.data[i] = 0
				}
				crt = readEnd
				continue
			}
			onceRead, innerErr = c.r.storage.Get(ctx, seg.id, pageIndex, seg.off-pageStart, page.data[crt:readEnd])
			for innerErr != nil {
				return innerErr
			}
			crt += onceRead
		}
		page.length = crt
		return nil
	})
	if err != nil {
		return err
	}
	defer page.release()
	if dest != nil {
		page.mux.RLock()
		defer page.mux.RUnlock()
		copy(dest, page.data[off-pageStart:page.length])
	}
	return nil
}

type chunkWriter struct {
	*chunkReader
	unready   int32
	writers   map[int64]*segWriter
	ref       int32
	writerMux sync.Mutex
}

func NewChunkWriter(reader Reader) Writer {
	r, ok := reader.(*chunkReader)
	if !ok {
		return nil
	}

	fileChunkMux.Lock()
	defer fileChunkMux.Unlock()
	cw, ok := fileChunkWriters[r.ID]
	if ok {
		atomic.AddInt32(&cw.ref, 1)
		return cw
	}

	return &chunkWriter{chunkReader: r, writers: map[int64]*segWriter{}, ref: 1}
}

func (c *chunkWriter) WriteAt(ctx context.Context, data []byte, off int64) (n int64, err error) {
	writeEnd := off + int64(len(data))
	for {
		index, _ := computeChunkIndex(off, fileChunkSize)

		chunkEnd := (index + 1) * fileChunkSize
		if chunkEnd > writeEnd {
			chunkEnd = writeEnd
		}

		readLen := chunkEnd - off
		if readLen == 0 {
			break
		}
		if err = c.writeSegData(ctx, index, off, data[n:n+readLen]); err != nil {
			return n, err
		}
		n += readLen
		off = chunkEnd
		if off == writeEnd {
			break
		}
	}
	return n, nil
}

func (c *chunkWriter) writeSegData(ctx context.Context, index, off int64, dest []byte) (err error) {
	c.writerMux.Lock()
	sw, ok := c.writers[index]
	if !ok {
		sw = &segWriter{
			chunkWriter: c,
			chunkID:     index,
			cache:       c.cache,
			uncommitted: make([]*uncommittedSeg, 0, 1),
		}
		sw.cond = sync.NewCond(&sw.mux)
		c.writers[index] = sw
		c.logger.Debugw("build segment writer", "entry", c.ID, "chunk", index)
	}
	c.writerMux.Unlock()

	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			c.logger.Errorw("write segment data panic", "entry", c.ID, "chunk", index, "err", rErr)
			err = rErr
		}
	}()

	var (
		crt    = off
		bufEnd = off + int64(len(dest))
	)
	for {
		pageIdx, pos := computePageIndex(crt)
		writeEnd := (pageIdx + 1) * pageSize
		if writeEnd > bufEnd {
			writeEnd = bufEnd
		}
		sw.put(ctx, pageIdx, pos, dest[crt-off:writeEnd-off])
		crt = writeEnd
		if crt == bufEnd {
			break
		}
	}

	if sw.chunkErr != nil {
		err = sw.chunkErr
		sw.chunkErr = nil
		return err
	}
	return nil
}

func (c *chunkWriter) Flush(ctx context.Context) error {
	c.writerMux.Lock()
	var resultErr error
	for _, cw := range c.writers {
		if cw.chunkErr != nil {
			resultErr = cw.chunkErr
			cw.chunkErr = nil
			break
		}
		for _, p := range cw.uncommitted {
			p.tryCommit(ctx)
		}
		cw.cond.Broadcast()
	}
	c.writerMux.Unlock()
	return resultErr
}

func (c *chunkWriter) Fsync(ctx context.Context) error {
	err := c.Flush(ctx)
	if err != nil {
		return err
	}

	if atomic.LoadInt32(&c.unready) == 0 {
		return nil
	}
	waitTicker := time.NewTicker(time.Millisecond * 100)
	defer waitTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-waitTicker.C:
			if atomic.LoadInt32(&c.unready) == 0 {
				return nil
			}
		}
	}
}

func (c *chunkWriter) Close() {
	if atomic.AddInt32(&c.ref, -1) == 0 {
		fileChunkMux.Lock()
		delete(fileChunkWriters, c.ID)
		fileChunkMux.Unlock()
	}
}

type segWriter struct {
	*chunkWriter

	chunkID     int64
	chunkErr    error
	cache       *storage.LocalCache
	uncommitted []*uncommittedSeg
	cond        *sync.Cond
	mux         sync.Mutex
}

func (w *segWriter) put(ctx context.Context, pageIdx, pagePos int64, data []byte) {
	w.mux.Lock()
	seg, page := w.findUncommittedPage(ctx, pageIdx, pageIdx*pageSize+pagePos)
	if seg == nil {
		seg = &uncommittedSeg{off: pageIdx*pageSize + pagePos, w: w}
		w.uncommitted = append(w.uncommitted, seg)
		atomic.AddInt32(&w.unready, 1)
		if len(w.uncommitted) == 1 {
			go w.commitSegment()
		}
	}
	w.mux.Unlock()

	if page == nil {
		seg.mux.Lock()
		page = &uncommittedPage{idx: pageIdx}
		var err error
		page.node, err = w.cache.OpenTemporaryNode(ctx, w.Object.ID, pageIdx*pageSize+pagePos)
		if err != nil {
			w.logger.Errorw("open temporary node error", "entry", w.ID, "chunk", w.chunkID, "err", err)
			w.chunkErr = err
			return
		}
		seg.pages = append(seg.pages, page)
		seg.mux.Unlock()
	}

	seg.uploads.Add(1)
	page.mux.Lock()
	go w.preWrite(ctx, pagePos, seg, page, data)
}
func (w *segWriter) preWrite(ctx context.Context, pagePos int64, seg *uncommittedSeg, page *uncommittedPage, data []byte) {
	defer seg.uploads.Done()

	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			w.logger.Errorw("pre-write segment panic", "entry", w.ID, "chunk", w.chunkID, "err", rErr)
			w.chunkErr = rErr
		}
	}()

	n, err := page.node.WriteAt(data, pagePos)
	if err != nil {
		w.chunkErr = err
		w.logger.Errorw("write to cache node error", "entry", w.ID, "chunk", w.chunkID, "err", err)
		page.mux.Unlock()
		atomic.AddInt32(&page.visitor, -1)
		return
	}
	dataEnd := page.idx*pageSize + pagePos + int64(n)
	if dataEnd == (page.idx+1)*pageSize && !page.committed {
		seg.uploads.Add(1)
		page.committed = true
		go w.flushData(ctx, seg, page)
	}
	page.mux.Unlock()
	atomic.AddInt32(&page.visitor, -1)

	seg.mux.Lock()
	if dataEnd > seg.off+seg.size {
		seg.size = dataEnd - seg.off
	}

	if dataEnd == (w.chunkID+1)*fileChunkSize {
		seg.tryCommit(ctx)
		w.cond.Broadcast()
	}
	seg.mux.Unlock()
}

func (w *segWriter) flushData(ctx context.Context, seg *uncommittedSeg, page *uncommittedPage) {
	defer seg.uploads.Done()

	waitingTime := 0
	for atomic.LoadInt32(&page.visitor) == 0 {
		select {
		case <-ctx.Done():
			w.chunkErr = fmt.Errorf("flush data timeout, chunk=%d, page=%d", w.chunkID, page.idx)
			w.logger.Errorw("flush data timeout", "entry", w.ID, "chunk", w.chunkID, "page", page.idx, "visitor", atomic.LoadInt32(&page.visitor))
			return
		default:
			time.Sleep(time.Millisecond)
			waitingTime += 1
		}
		if waitingTime > 3 {
			w.logger.Warnw("page still has visitors, waiting to flush", "entry", w.ID, "chunk", w.chunkID, "page", page.idx, "visitor", atomic.LoadInt32(&page.visitor))
		}
	}

	segID, err := seg.prepareID(ctx, w.store.NextSegmentID)
	if err != nil {
		w.chunkErr = err
		w.logger.Errorw("prepare segment id error", "entry", w.ID, "chunk", w.chunkID, "page", page.idx, "err", err)
		return
	}

	if err = w.cache.CommitTemporaryNode(ctx, segID, page.idx, page.node); err != nil {
		w.chunkErr = err
		w.logger.Errorw("commit page data error", "entry", w.ID, "chunk", w.chunkID, "page", page.idx, "err", err)
		return
	}
}

func (w *segWriter) findUncommittedPage(ctx context.Context, pageIdx, off int64) (*uncommittedSeg, *uncommittedPage) {
	var (
		seg  *uncommittedSeg
		page *uncommittedPage
	)
	for i := 1; i <= len(w.uncommitted); i++ {
		s := w.uncommitted[len(w.uncommitted)-i]
		if !s.readyToCommit && s.off+s.size <= off {
			seg = s
			break
		} else if i > 4 || time.Since(s.modifyAt) > fileChunkCommitTimeout {
			s.tryCommit(ctx)
		}
	}
	if seg == nil {
		return nil, nil
	}

	seg.mux.Lock()
	for i, p := range seg.pages {
		if p.idx == pageIdx {
			page = seg.pages[i]
			break
		}
	}
	seg.mux.Unlock()
	if page == nil {
		return seg, nil
	}
	page.mux.Lock()
	canRewrite := !page.committed
	page.mux.Unlock()
	if canRewrite {
		atomic.AddInt32(&page.visitor, 1)
		return seg, page
	}
	return nil, nil
}

func (w *segWriter) commitSegment() {
	defer logger.CostLog(w.logger.With(zap.Int64("entry", w.ID), zap.Int64("chunk", w.chunkID)), "commit segment data")()
	for len(w.uncommitted) > 0 {
		w.mux.Lock()
		seg := w.uncommitted[0]
		for !seg.readyToCommit {
			w.cond.Wait()
			if time.Since(seg.modifyAt) > fileChunkCommitTimeout {
				seg.tryCommit(context.Background())
			}
		}
		w.mux.Unlock()

		seg.uploads.Wait()
		if err := w.store.AppendSegments(context.Background(), types.ChunkSeg{
			ID:       seg.segID,
			ChunkID:  w.chunkID,
			ObjectID: w.Object.ID,
			Off:      seg.off,
			Len:      seg.size,
			State:    0,
		}, w.Object); err != nil {
			w.logger.Errorw("append segment error", "entry", w.ID, "chunk", w.chunkID, "err", err)
			w.chunkErr = err
			continue
		}
		w.uncommitted = w.uncommitted[1:]
		atomic.AddInt32(&w.unready, -1)
	}
}

type uncommittedSeg struct {
	segID         int64
	off, size     int64
	pages         []*uncommittedPage
	uploads       sync.WaitGroup
	modifyAt      time.Time
	mux           sync.Mutex
	readyToCommit bool

	w *segWriter
}

func (s *uncommittedSeg) tryCommit(ctx context.Context) {
	if s.readyToCommit {
		return
	}
	for i := range s.pages {
		page := s.pages[i]
		page.mux.Lock()
		if page.committed {
			page.mux.Unlock()
			continue
		}
		page.committed = true
		s.uploads.Add(1)
		go s.w.flushData(ctx, s, page)
		page.mux.Unlock()
	}
	s.readyToCommit = true
}

func (s *uncommittedSeg) prepareID(ctx context.Context, prepareFn func(context.Context) (int64, error)) (int64, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.segID != 0 {
		return s.segID, nil
	}

	var err error
	s.segID, err = prepareFn(ctx)
	if err != nil {
		return 0, err
	}
	return s.segID, nil
}

type uncommittedPage struct {
	idx       int64
	committed bool
	visitor   int32
	node      storage.CacheNode
	mux       sync.Mutex
}

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}

type ioReq struct {
	*sync.WaitGroup

	off  int64
	dest []byte
	err  error
}

type segTree struct {
	start, end  int64
	id, pos     int64
	left, right *segTree
}

func (t *segTree) query(start, end int64) []segment {
	if t == nil {
		return nil
	}

	var result []segment
	if start < t.start {
		result = append(result, t.left.query(start, minOff(t.start, end))...)
	}

	segStart := maxOff(t.start, start)
	segEnd := minOff(t.end, end)
	if segEnd-segStart > 0 {
		result = append(result, segment{
			id:  t.id,
			off: segStart,
			pos: t.pos + segStart - t.start,
			len: segEnd - segStart,
		})
	}

	if end > t.end {
		result = append(result, t.right.query(maxOff(t.end, start), end)...)
	}
	return result
}

func (t *segTree) cut(off int64) (left, right *segTree) {
	if t == nil {
		return nil, nil
	}
	switch {
	case off < t.start:
		left, _ = t.left.cut(off)
		right = t
		return
	case off > t.end:
		left = t
		t.right, right = t.right.cut(off)
		return
	case off == t.start:
		left = t.left
		right = t
		t.left = nil
		return
	case off == t.end:
		left = t
		right = t.right
		t.right = nil
		return
	default:
		cutSize := off - t.start
		if cutSize == 0 {
			return nil, t
		}
		if off == t.end {
			return t, nil
		}
		left = &segTree{start: t.start, end: off, id: t.id, pos: t.pos, left: t.left}
		right = &segTree{start: off, end: t.end, id: t.id, pos: t.pos + cutSize, right: t.right}
		return
	}
}

func buildSegmentTree(dataStart, dataEnd int64, segList []types.ChunkSeg) *segTree {
	st := &segTree{start: dataStart, end: dataEnd}
	for _, seg := range segList {
		newSt := &segTree{id: seg.ID, start: seg.Off, end: seg.Off + seg.Len}
		var r *segTree
		newSt.left, r = st.cut(seg.Off)
		_, newSt.right = r.cut(seg.Off + seg.Len)
		st = newSt
	}
	return st
}

type segment struct {
	id  int64 // segment id
	pos int64 // segment pos
	off int64 // file offset
	len int64 // segment remaining length after pos
}

func maxOff(off1, off2 int64) int64 {
	if off1 > off2 {
		return off1
	}
	return off2
}

func minOff(off1, off2 int64) int64 {
	if off1 < off2 {
		return off1
	}
	return off2
}

func DeleteChunksData(ctx context.Context, md *types.Metadata, chunkStore storage.ChunkStore, dataStore storage.Storage) error {
	maxChunkID := (md.Size / fileChunkSize) + 1

	var (
		segments  []types.ChunkSeg
		resultErr error
	)
	for cid := int64(0); cid < maxChunkID; cid++ {
		chunkSegment, err := chunkStore.ListSegments(ctx, md.ID, cid)
		if err != nil {
			resultErr = err
			continue
		}
		if len(chunkSegment) > 0 {
			segments = append(segments, chunkSegment...)
		}
	}

	for _, seg := range segments {
		if err := dataStore.Delete(ctx, seg.ID); err != nil {
			resultErr = err
			continue
		}
	}
	return resultErr
}

func CloseAll() {
	allReaders := make(map[int64]*chunkReader)
	allWriters := make(map[int64]*chunkWriter)
	fileChunkMux.Lock()
	for fid := range fileChunkWriters {
		allWriters[fid] = fileChunkWriters[fid]
	}
	for fid := range fileChunkReaders {
		allReaders[fid] = fileChunkReaders[fid]
	}
	fileChunkMux.Unlock()

	wg := sync.WaitGroup{}
	wg.Add(len(allWriters))
	for i := range allWriters {
		go func(w *chunkWriter) {
			defer wg.Done()
			w.Close()
		}(allWriters[i])
	}
	wg.Add(len(allReaders))
	for i := range allReaders {
		go func(r *chunkReader) {
			defer wg.Done()
			r.Close()
		}(allReaders[i])
	}
	wg.Wait()
}
