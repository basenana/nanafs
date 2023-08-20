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
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"io"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"
)

const (
	fileChunkSize          int64 = 1 << 26 // 64MB
	fileChunkCommitTimeout       = time.Minute
)

var (
	maxReadChunkTaskParallel  = utils.NewParallelWorker(256)
	maxWriteChunkTaskParallel = utils.NewParallelWorker(64)
)

type chunkReader struct {
	entry *types.Metadata

	page        *pageCache
	store       metastore.ChunkStore
	cache       *storage.LocalCache
	readers     map[int64]*segReader
	readMux     sync.Mutex
	ref         int32
	logger      *zap.SugaredLogger
	needCompact bool
}

func NewChunkReader(entry *types.Metadata, chunkStore metastore.ChunkStore, dataStore storage.Storage) Reader {
	fileChunkMux.Lock()
	defer fileChunkMux.Unlock()

	r, ok := fileChunkReaders[entry.ID]
	if !ok {
		cr := &chunkReader{
			entry:   entry,
			page:    newPageCache(entry.ID, fileChunkSize),
			store:   chunkStore,
			cache:   storage.NewLocalCache(dataStore),
			ref:     1,
			readers: map[int64]*segReader{},
			logger:  logger.NewLogger("chunkIO").With("entry", entry.ID),
		}
		fileChunkReaders[entry.ID] = cr
		return cr
	}

	cr, ok := r.(*chunkReader)
	if !ok {
		return nil
	}
	atomic.AddInt32(&cr.ref, 1)
	return cr
}

func (c *chunkReader) ReadAt(ctx context.Context, dest []byte, off int64) (n int64, err error) {
	ctx, task := trace.NewTask(ctx, "bio.chunkReader.ReadAt")
	defer task.End()

	if off >= c.entry.Size {
		return 0, io.EOF
	}

	readEnd := off + int64(len(dest))
	if readEnd > c.entry.Size {
		readEnd = c.entry.Size
	}
	if readEnd == 0 {
		return
	}

	var (
		wg      = &sync.WaitGroup{}
		reqList = make([]*ioReq, 0, int64(len(dest))/fileChunkSize+1)
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
	defer trace.StartRegion(ctx, "bio.chunkReader.prepareData").End()
	req := &ioReq{
		WaitGroup: wg,
		off:       off,
		data:      dest,
	}
	c.readMux.Lock()
	reader, ok := c.readers[index]
	if !ok {
		reader = &segReader{r: c, chunkID: index, preReadIdx: fileChunkSize * index / pageSize}
		c.readers[index] = reader
		c.logger.Debugw("builder segment reader", "entry", c.entry.ID, "chunk", index)
	}
	c.readMux.Unlock()

	maxReadChunkTaskParallel.Dispatch(ctx, func() {
		defer req.Done()
		defer logLatency(chunkReaderLatency, "read_chunk", time.Now())
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
		c.page.invalid(fileChunkSize*index/pageSize, fileChunkSize*(index+1)/pageSize)
		reader.mux.Unlock()
	}
}

func (c *chunkReader) Close() {
	if atomic.AddInt32(&c.ref, -1) == 0 {
		fileChunkMux.Lock()
		delete(fileChunkReaders, c.entry.ID)
		fileChunkMux.Unlock()
		if c.needCompact {
			events.Publish(events.EntryActionTopic(events.TopicFileActionFmt, events.ActionTypeCompact),
				buildCompactEvent(c.entry))
		}
		c.page.close()
	}
}

type segReader struct {
	r          *chunkReader
	st         *segTree
	chunkID    int64
	crtPreRead int32
	preReadIdx int64
	mux        sync.Mutex
}

func (c *segReader) readChunkRange(ctx context.Context, req *ioReq) {
	defer trace.StartRegion(ctx, "bio.segReader.readChunkRange").End()
	c.mux.Lock()
	if c.st == nil {
		segments, err := c.r.store.ListSegments(ctx, c.r.entry.ID, c.chunkID, false)
		if err != nil {
			c.mux.Unlock()
			c.r.logger.Errorw("list segment reader", "entry", c.r.entry.ID, "chunk", c.chunkID, "err", err)
			req.err = logErr(chunkReadErrorCounter, err, "read_chunk")
			return
		}

		if len(segments) > 1 {
			c.r.needCompact = true
		}

		dataSize := (c.chunkID + 1) * int64(fileChunkSize)
		if c.r.entry.Size < dataSize {
			dataSize = c.r.entry.Size
		}
		c.st = buildSegmentTree(c.chunkID*fileChunkSize, dataSize, segments)
	}
	st := c.st
	c.mux.Unlock()

	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			c.r.logger.Errorw("read chunk range panic", "entry", c.r.entry.ID, "chunk", c.chunkID, "err", rErr)
			req.err = rErr
		}
	}()

	var (
		off          = req.off
		bufLeft      = int64(len(req.data))
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
		chunkReadingGauge.Inc()
		go func(ctx context.Context, segments []segment, pageID, off int64, dest []byte) {
			maxReadChunkTaskParallel.Dispatch(ctx, func() {
				defer chunkReadingGauge.Dec()
				defer req.Done()
				if err := c.readPage(ctx, segments, pageID, off, dest); err != nil && req.err == nil {
					req.err = err
					c.r.logger.Errorw("read chunk page error", "entry", c.r.entry.ID, "chunk", c.chunkID, "page", pageID, "err", err)
					return
				}
			})
		}(ctx, st.query(pageIdx*pageSize, (pageIdx+1)*pageSize), pageIdx, readStart, req.data[off-req.off:readEnd-req.off])
		bufLeft -= readEnd - off
		off = readEnd
		if bufLeft == 0 {
			break
		}
	}

	if req.off == 0 {
		// The first read skips pre-reading to
		// make the data ready as quickly as possible.
		return
	}

	maxPage := (c.chunkID + 1) * fileChunkSize / pageSize
	for atomic.LoadInt32(&c.crtPreRead) < expectPreRead(int64(len(req.data))/pageSize+1) {
		c.preReadIdx += 1
		if c.preReadIdx >= maxPage || c.preReadIdx*pageSize > c.r.entry.Size {
			break
		}
		atomic.AddInt32(&c.crtPreRead, 1)
		chunkReadingGauge.Inc()
		go func(ctx context.Context, segments []segment, pageID int64) {
			maxReadChunkTaskParallel.Dispatch(ctx, func() {
				defer chunkReadingGauge.Dec()
				defer atomic.AddInt32(&c.crtPreRead, -1)
				if err := c.readPage(ctx, segments, pageID, 0, nil); err != nil {
					c.r.logger.Errorw("pre-read chunk page error", "entry", c.r.entry.ID, "chunk", c.chunkID, "page", pageID, "err", err)
					return
				}
			})
		}(ctx, st.query(c.preReadIdx*pageSize, (c.preReadIdx+1)*pageSize), c.preReadIdx)
	}
}

func (c *segReader) readPage(ctx context.Context, segments []segment, pageIndex, off int64, dest []byte) (err error) {
	defer logLatency(chunkReaderLatency, "read_page", time.Now())
	defer trace.StartRegion(ctx, "bio.segReader.readPage").End()

	ctx, task := trace.NewTask(ctx, "bio.chunkReader.readPage")
	defer task.End()

	pageStart := pageSize * pageIndex
	var page *pageNode

	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			c.r.logger.Errorw("read chunk page panic", "entry", c.r.entry.ID, "chunk", c.chunkID, "err", err)
			err = rErr
		}
	}()

	page, err = c.r.page.read(ctx, pageIndex, func(page *pageNode) error {
		if len(segments) == 1 && segments[0].id > 0 {
			openedCachedNode, innerErr := c.r.cache.OpenCacheNode(ctx, segments[0].id, pageIndex)
			for innerErr != nil {
				return innerErr
			}
			page.node = openedCachedNode
			return nil
		}
		chunkMergePageCounter.Inc()
		return c.mergePage(ctx, pageIndex, page, segments)
	})
	if err != nil {
		return logErr(chunkReadErrorCounter, err, "read_page")
	}
	defer page.release()
	if len(dest) > 0 {
		page.mux.RLock()
		defer page.mux.RUnlock()
		_, err = page.node.ReadAt(dest, off-pageStart)
		return logErr(chunkReadErrorCounter, err, "read_page")
	}
	return nil
}

func (c *segReader) mergePage(ctx context.Context, pageIndex int64, page *pageNode, segments []segment) error {
	var (
		crt, readEnd     int64
		onceRead         int64
		innerErr         error
		pageStart        = pageSize * pageIndex
		openedCachedNode storage.CacheNode
	)
	node := storage.NewMemCacheNode(pageIndex > 1)
	for _, seg := range segments {
		crt = seg.off - pageStart
		readEnd = crt + seg.len
		if readEnd > pageSize {
			readEnd = pageSize
		}
		if seg.id == 0 {
			_, _ = io.CopyN(utils.NewWriterWithOffset(node, crt), utils.ZeroDevice, readEnd-crt)
			crt = readEnd
			continue
		}
		openedCachedNode, innerErr = c.r.cache.OpenCacheNode(ctx, seg.id, pageIndex)
		for innerErr != nil {
			_ = node.Close()
			return logErr(chunkReadErrorCounter, innerErr, "merge_page")
		}

		onceRead, innerErr = io.CopyN(utils.NewWriterWithOffset(node, crt), utils.NewReaderWithOffset(openedCachedNode, seg.off-pageStart), readEnd-crt)
		for innerErr != nil {
			_ = openedCachedNode.Close()
			_ = node.Close()
			return logErr(chunkReadErrorCounter, innerErr, "merge_page")
		}
		if onceRead == 0 {
			c.r.logger.Warnw("read cached node error: got empty", "segment", seg.id, "page", pageIndex)
		}
		_ = openedCachedNode.Close()
		crt += onceRead
	}
	page.node = node
	page.length = crt
	return nil
}

type chunkWriter struct {
	*chunkReader
	unready      int32
	writers      map[int64]*segWriter
	ref          int32
	commitLimit  int32
	invalidCache InvalidCacheHook
	writerMux    sync.Mutex
}

func NewChunkWriter(reader Reader, hook InvalidCacheHook) Writer {
	r, ok := reader.(*chunkReader)
	if !ok {
		return nil
	}
	atomic.AddInt32(&r.ref, 1)

	fileChunkMux.Lock()
	defer fileChunkMux.Unlock()
	w, ok := fileChunkWriters[r.entry.ID]
	if !ok {
		cw := &chunkWriter{chunkReader: r, writers: map[int64]*segWriter{}, ref: 1, invalidCache: hook}
		fileChunkWriters[r.entry.ID] = cw
		return cw
	}

	cw, ok := w.(*chunkWriter)
	if !ok {
		return nil
	}
	atomic.AddInt32(&cw.ref, 1)
	return cw
}

func (c *chunkWriter) WriteAt(ctx context.Context, data []byte, off int64) (n int64, err error) {
	ctx, task := trace.NewTask(ctx, "bio.chunkWriter.WriteAt")
	defer task.End()

	var (
		wg      = &sync.WaitGroup{}
		fileLen = c.entry.Size
		reqList = make([]*ioReq, 0, int64(len(data))/fileChunkSize+1)
	)
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
		wg.Add(1)
		req := &ioReq{WaitGroup: wg, off: off, data: data[n : n+readLen]}
		if err = c.writeSegData(ctx, index, req); err != nil {
			return n, err
		}
		reqList = append(reqList, req)

		n += readLen
		off = chunkEnd
		if off == writeEnd {
			break
		}
	}
	if fileLen > off {
		c.needCompact = true
	}
	wg.Wait()
	for _, req := range reqList {
		if req.err != nil {
			return n, req.err
		}
	}
	return n, nil
}

func (c *chunkWriter) writeSegData(ctx context.Context, index int64, req *ioReq) (err error) {
	defer logLatency(chunkWriterLatency, "write_segment", time.Now())
	defer trace.StartRegion(ctx, "bio.chunkWriter.writeSegData").End()
	defer req.Done()
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
		c.logger.Debugw("build segment writer", "entry", c.entry.ID, "chunk", index)
	}
	c.writerMux.Unlock()

	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			c.logger.Errorw("write segment data panic", "entry", c.entry.ID, "chunk", index, "err", rErr)
			err = rErr
		}
	}()

	var (
		crt    = req.off
		bufEnd = req.off + int64(len(req.data))
	)
	for {
		pageIdx, pos := computePageIndex(crt)
		writeEnd := (pageIdx + 1) * pageSize
		if writeEnd > bufEnd {
			writeEnd = bufEnd
		}
		sw.put(ctx, pageIdx, pos, req, req.data[crt-req.off:writeEnd-req.off])
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
	defer trace.StartRegion(ctx, "bio.chunkWriter.Flush").End()
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
	defer trace.StartRegion(ctx, "bio.chunkWriter.Fsync").End()
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
		delete(fileChunkWriters, c.entry.ID)
		fileChunkMux.Unlock()
	}
	c.chunkReader.Close()
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

func (w *segWriter) put(ctx context.Context, pageIdx, pagePos int64, req *ioReq, data []byte) {
	defer trace.StartRegion(ctx, "bio.segWriter.put").End()
	w.mux.Lock()
	seg, page := w.findUncommittedPage(ctx, pageIdx, pageIdx*pageSize+pagePos)
	if seg == nil {
		seg = &uncommittedSeg{off: pageIdx*pageSize + pagePos, w: w}
		w.uncommitted = append(w.uncommitted, seg)
		atomic.AddInt32(&w.unready, 1)
		if len(w.uncommitted) == 1 {
			chunkUncommittedGauge.Inc()
			for {
				canCommit := atomic.LoadInt32(&w.commitLimit)
				if canCommit >= 5 {
					time.Sleep(time.Second * 5)
					continue
				}
				if atomic.CompareAndSwapInt32(&w.commitLimit, canCommit, canCommit+1) {
					go w.commitSegment(ctx)
					break
				}
			}
		}
	}
	w.mux.Unlock()

	if page == nil {
		seg.mux.Lock()
		page = &uncommittedPage{idx: pageIdx}
		var err error
		page.node, err = w.cache.OpenTemporaryNode(ctx, w.entry.ID, pageIdx*pageSize+pagePos)
		if err != nil {
			w.logger.Errorw("open temporary node error", "entry", w.entry.ID, "chunk", w.chunkID, "err", err)
			req.err = logErr(chunkWriteErrorCounter, err, "write_dirty_page")
			return
		}
		seg.pages = append(seg.pages, page)
		seg.mux.Unlock()
	}

	seg.uploads.Add(1)
	page.mux.Lock()
	req.Add(1)
	go w.preWrite(ctx, pagePos, seg, page, req, data)
}
func (w *segWriter) preWrite(ctx context.Context, pagePos int64, seg *uncommittedSeg, page *uncommittedPage, req *ioReq, data []byte) {
	defer logLatency(chunkWriterLatency, "write_dirty_page", time.Now())
	defer trace.StartRegion(ctx, "bio.segWriter.preWrite").End()
	defer seg.uploads.Done()
	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			w.logger.Errorw("pre-write segment panic", "entry", w.entry.ID, "chunk", w.chunkID, "err", rErr)
			w.chunkErr = rErr
		}
	}()

	n, err := page.node.WriteAt(data, pagePos)
	if err != nil {
		req.Done()
		req.err = logErr(chunkWriteErrorCounter, err, "write_dirty_page")
		w.logger.Errorw("write to cache node error", "entry", w.entry.ID, "chunk", w.chunkID, "err", err)
		page.mux.Unlock()
		atomic.AddInt32(&page.visitor, -1)
		return
	}
	req.Done()

	dataEnd := page.idx*pageSize + pagePos + int64(n)
	if dataEnd == (page.idx+1)*pageSize && !page.committed {
		page.committed = true
		chunkDirtyPageGauge.Inc()
		seg.uploads.Add(1)
		maxWriteChunkTaskParallel.Dispatch(ctx, func() {
			w.flushData(ctx, seg, page)
		})
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
	defer logLatency(chunkCommitSegmentLatency, "flush_dirty_page", time.Now())
	defer trace.StartRegion(ctx, "bio.segWriter.flushData").End()
	defer chunkDirtyPageGauge.Dec()
	defer seg.uploads.Done()
	defer func() {
		if page.err != nil {
			w.chunkErr = logErr(chunkWriteErrorCounter, page.err, "flush_dirty_page")
		}
	}()
	waitingTime := 0
	for atomic.LoadInt32(&page.visitor) == 0 {
		select {
		case <-ctx.Done():
			page.err = fmt.Errorf("flush data timeout, chunk=%d, page=%d", w.chunkID, page.idx)
			w.logger.Errorw("flush data timeout", "entry", w.entry.ID, "chunk", w.chunkID, "page", page.idx, "visitor", atomic.LoadInt32(&page.visitor))
			return
		default:
			time.Sleep(time.Millisecond)
			waitingTime += 1
		}
		if waitingTime > 3 {
			w.logger.Warnw("page still has visitors, waiting to flush", "entry", w.entry.ID, "chunk", w.chunkID, "page", page.idx, "visitor", atomic.LoadInt32(&page.visitor))
		}
	}

	segID, err := seg.prepareID(ctx, w.store.NextSegmentID)
	if err != nil {
		page.err = err
		w.logger.Errorw("prepare segment id error", "entry", w.entry.ID, "chunk", w.chunkID, "page", page.idx, "err", err)
		return
	}

	// TODO: close page itself?
	defer page.node.Close()
	if err = w.cache.CommitTemporaryNode(ctx, segID, page.idx, page.node); err != nil {
		page.err = err
		w.logger.Errorw("commit page data error", "entry", w.entry.ID, "chunk", w.chunkID, "page", page.idx, "err", err)
		return
	}
}

func (w *segWriter) findUncommittedPage(ctx context.Context, pageIdx, off int64) (*uncommittedSeg, *uncommittedPage) {
	defer trace.StartRegion(ctx, "bio.segWriter.findUncommittedPage").End()
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

func (w *segWriter) commitSegment(ctx context.Context) {
	defer chunkUncommittedGauge.Dec()
	defer logLatency(chunkCommitSegmentLatency, "commit_segment", time.Now())
	defer logger.CostLog(w.logger.With(zap.Int64("chunk", w.chunkID)), "commit segment data")()

	ctx, task := trace.NewTask(ctx, "bio.segWriter.commitSegment")
	defer task.End()
	defer atomic.AddInt32(&w.commitLimit, -1)

	for len(w.uncommitted) > 0 {
		select {
		case <-ctx.Done():
			w.logger.Errorw("[DISCARD] commit segment closed", "entry", w.entry.ID, "chunk", w.chunkID, "err", ctx.Err())
			if ctx.Err() == context.DeadlineExceeded {
				sentry.CaptureMessage("commit segment closed: deadline exceeded")
			}
			return
		default:

		}
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
		if uploadErr := seg.uploaded(); uploadErr != nil {
			w.uncommitted = w.uncommitted[1:]
			atomic.AddInt32(&w.unready, -1)
			w.logger.Errorw("[DISCARD] upload segment error, discard segment info", "entry", w.entry.ID, "chunk", w.chunkID, "err", uploadErr)
			w.chunkErr = logErr(chunkWriteErrorCounter, uploadErr, "commit_segment")
			sentry.CaptureException(w.chunkErr)
			chunkDiscardSegmentCounter.Inc()
			continue
		}

		newObj, err := w.store.AppendSegments(context.Background(), types.ChunkSeg{
			ID:       seg.segID,
			ChunkID:  w.chunkID,
			ObjectID: w.entry.ID,
			Off:      seg.off,
			Len:      seg.size,
			State:    0,
		})
		if err != nil {
			w.logger.Errorw("append segment error", "entry", w.entry.ID, "chunk", w.chunkID, "err", err)
			w.chunkErr = logErr(chunkWriteErrorCounter, err, "commit_segment")
			sentry.CaptureException(w.chunkErr)
			chunkDiscardSegmentCounter.Inc()
			continue
		}
		w.invalidCache(w.entry.ID)
		w.invalidate(w.chunkID)
		w.entry = &newObj.Metadata
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
		chunkDirtyPageGauge.Inc()
		maxWriteChunkTaskParallel.Dispatch(ctx, func() {
			s.w.flushData(ctx, s, page)
		})
		page.mux.Unlock()
	}
	s.readyToCommit = true
}

func (s *uncommittedSeg) uploaded() error {
	for _, p := range s.pages {
		if p.err != nil {
			return p.err
		}
	}
	return nil
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
	err       error
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
	data []byte
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

func CompactChunksData(ctx context.Context, entry *types.Metadata, chunkStore metastore.ChunkStore, dataStore storage.Storage) (resultErr error) {
	maxChunkID := (entry.Size / fileChunkSize) + 1
	var (
		reader Reader
		writer Writer
		buf    []byte
		readN  int64
	)
	for cid := int64(0); cid < maxChunkID; cid++ {
		chunkSegment, err := chunkStore.ListSegments(ctx, entry.ID, cid, false)
		if err != nil {
			resultErr = err
			continue
		}

		segmentCount := len(chunkSegment)
		if segmentCount <= 1 {
			continue
		}

		// compact chunk
		chunkStart := cid * fileChunkSize
		chunkEnd := (cid + 1) * fileChunkSize
		if chunkEnd > entry.Size {
			chunkEnd = entry.Size
		}
		if chunkSegment[segmentCount-1].Off == chunkStart && chunkSegment[segmentCount-1].Len == chunkEnd-chunkStart {
			// clean overed write segments
			if err = deleteSegmentAndData(ctx, chunkSegment[:segmentCount-1], chunkStore, dataStore); err != nil {
				resultErr = err
			}
			continue
		}

		// rebuild new sequential segment
		if reader == nil {
			reader = NewChunkReader(entry, chunkStore, dataStore)
			writer = NewChunkWriter(reader, noneInvalidCache)
			buf = make([]byte, fileChunkSize)
		}
		readN, err = reader.ReadAt(ctx, buf, chunkStart)
		if err != nil {
			return err
		}
		// write sequential data
		_, err = writer.WriteAt(ctx, buf[:readN], chunkStart)
		if err != nil {
			return err
		}
		err = writer.Fsync(ctx)
		if err != nil {
			return err
		}

		if err = deleteSegmentAndData(ctx, chunkSegment, chunkStore, dataStore); err != nil {
			resultErr = err
		}
	}

	if reader != nil {
		reader.Close()
		writer.Close()
	}

	return resultErr
}

func DeleteChunksData(ctx context.Context, entry *types.Metadata, chunkStore metastore.ChunkStore, dataStore storage.Storage) error {
	segments, err := chunkStore.ListSegments(ctx, entry.ID, 0, true)
	if err != nil {
		return err
	}
	if err = deleteSegmentAndData(ctx, segments, chunkStore, dataStore); err != nil {
		return err
	}
	return nil
}

func deleteSegmentAndData(ctx context.Context, segments []types.ChunkSeg, chunkStore metastore.ChunkStore, dataStore storage.Storage) (resultErr error) {
	for _, seg := range segments {
		if err := dataStore.Delete(ctx, seg.ID); err != nil && err != types.ErrNotFound {
			resultErr = err
			continue
		}
		if err := chunkStore.DeleteSegment(ctx, seg.ID); err != nil && err != types.ErrNotFound {
			resultErr = err
			continue
		}
	}
	return
}
