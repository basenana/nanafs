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
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	fileChunkSize          = 1 << 26 // 64MB
	fileChunkCommitTimeout = time.Second * 3
)

var (
	maximumChunkTaskParallel = utils.NewMaximumParallel(100)
)

type chunkReader struct {
	*types.Object

	page    *pageCache
	store   storage.ChunkStore
	storage storage.Storage
}

func NewChunkReader(obj *types.Object, chunkStore storage.ChunkStore, dataStore storage.Storage) Reader {
	cr := &chunkReader{
		Object:  obj,
		page:    newPageCache(obj.ID, fileChunkSize),
		store:   chunkStore,
		storage: dataStore,
	}
	return cr
}

func (c *chunkReader) ReadAt(ctx context.Context, dest []byte, off int64) (n int64, err error) {
	ctx, endF := utils.TraceTask(ctx, "chunkreader.readat")
	defer endF()

	if off >= c.Object.Size {
		return 0, io.EOF
	}

	var (
		readEnd = off + int64(len(dest))
		reqList = make([]*ioReq, 0, readEnd/fileChunkSize+1)
	)
	if readEnd > c.Object.Size {
		readEnd = c.Object.Size
	}

	if readEnd == 0 {
		return
	}

	for {
		index, _ := computeChunkIndex(off, fileChunkSize)
		chunkEnd := (index + 1) * fileChunkSize
		if chunkEnd > readEnd {
			chunkEnd = readEnd
		}

		readLen := chunkEnd - off
		var req *ioReq
		req, err = c.prepareData(ctx, index, off, dest[n:n+readLen])
		if err != nil {
			return 0, err
		}
		reqList = append(reqList, req)

		n += readLen
		off = chunkEnd
		if off == readEnd {
			break
		}
	}
	return n, c.waitIO(ctx, reqList)
}

func (c *chunkReader) waitIO(ctx context.Context, reqList []*ioReq) (err error) {
	allFinish := false
	for !allFinish {
		allFinish = true
	waitIO:
		for _, req := range reqList {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if !req.isReady {
					allFinish = false
					time.Sleep(time.Millisecond * 50)
					break waitIO
				}
			}
		}
	}
	return
}

func (c *chunkReader) prepareData(ctx context.Context, index, off int64, dest []byte) (*ioReq, error) {
	req := &ioReq{
		off:  off,
		dest: dest,
	}
	maximumChunkTaskParallel.Go(func() {
		c.readChunkRange(ctx, index, req)
	})
	return req, nil
}

func (c *chunkReader) readChunkRange(ctx context.Context, chunkID int64, req *ioReq) {
	ctx, endF := utils.TraceTask(ctx, "segreader.readrange")
	defer endF()

	segments, err := c.store.ListSegments(ctx, c.ID, chunkID)
	if err != nil {
		req.err = err
		return
	}

	dataSize := (chunkID + 1) * int64(fileChunkSize)
	if c.Size < dataSize {
		dataSize = c.Size
	}
	st := buildSegmentTree(chunkID*fileChunkSize, dataSize, segments)

	var (
		off     = req.off
		bufLeft = int64(len(req.dest))
		wg      = sync.WaitGroup{}
	)

	for {
		pageIdx, pos := computePageIndex(off)
		readStart := pageIdx*pageSize + pos
		readEnd := (pageIdx + 1) * pageSize
		if readEnd-off > bufLeft {
			readEnd = off + bufLeft
		}
		wg.Add(1)
		go func(ctx context.Context, segments []segment, pageID, off int64, dest []byte) {
			maximumChunkTaskParallel.BlockedGo(func() {
				defer wg.Done()
				if err = c.readPage(ctx, segments, pageID, off, dest); err != nil && req.err == nil {
					req.err = err
					endF()
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
	wg.Wait()
	req.isReady = true
}

func (c *chunkReader) readPage(ctx context.Context, segments []segment, pageIndex, off int64, dest []byte) error {
	pageStart := pageSize * pageIndex
	page, err := c.page.read(ctx, pageIndex, func(page *pageNode) error {
		var (
			crt, onceRead, readEnd int64
			err                    error
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
			onceRead, err = c.storage.Get(ctx, seg.id, pageIndex, seg.off-pageStart, page.data[crt:readEnd])
			for err != nil {
				return err
			}
			crt += onceRead
		}
		return nil
	})
	if err != nil {
		return err
	}
	copy(dest, page.data[off-pageStart:])
	return nil
}

type chunkWriter struct {
	*chunkReader
	unready int32
	chunks  map[int64]*segWriter
	mux     sync.Mutex
}

func NewChunkWriter(reader Reader) Writer {
	r, ok := reader.(*chunkReader)
	if !ok {
		return nil
	}
	return &chunkWriter{chunkReader: r, chunks: map[int64]*segWriter{}}
}

func (c *chunkWriter) WriteAt(ctx context.Context, data []byte, off int64) (n int64, err error) {
	var (
		writeEnd = off + int64(len(data))
	)
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
		c.writeSegData(ctx, index, off, data[n:n+readLen])
		n += readLen
		off = chunkEnd
		if off == writeEnd {
			break
		}
	}
	return n, nil
}
func (c *chunkWriter) writeSegData(ctx context.Context, index, off int64, dest []byte) {
	c.mux.Lock()
	defer c.mux.Unlock()
	sw, ok := c.chunks[index]
	if !ok {
		sw = &segWriter{
			chunkWriter: c,
			chunkID:     index,
			dirtyPages:  make([]*dirtyPage, 0, 1),
		}
		c.chunks[index] = sw
	}

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
}

func (c *chunkWriter) Fsync(ctx context.Context) error {
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

type segWriter struct {
	*chunkWriter

	chunkID    int64
	chunkErr   error
	dirtyPages []*dirtyPage
}

func (w *segWriter) put(ctx context.Context, pageIdx, pagePos int64, data []byte) {
	atomic.AddInt32(&w.unready, 1)

	var tgt, p *dirtyPage
	for i := 1; i <= len(w.dirtyPages); i++ {
		p = w.dirtyPages[len(w.dirtyPages)-i]
		if !p.readyToCommit && p.pIdx == pageIdx {
			tgt = p
			break
		} else if i > 4 || time.Since(p.modifyAt) > fileChunkCommitTimeout {
			p.tryCommit()
		}
	}

	if tgt != nil {
		dataLen := pagePos + int64(copy(tgt.node.data[pagePos:], data))
		if pagePos < tgt.node.pos {
			tgt.node.pos = pagePos
		}
		if dataLen > tgt.node.length {
			tgt.node.length = dataLen
		}
		if tgt.node.length == pageSize {
			tgt.tryCommit()
		}
		atomic.AddInt32(&w.unready, -1)
		return
	}

	pNode, err := w.page.read(ctx, pageIdx, func(page *pageNode) error {
		page.pos = pagePos
		page.length = int64(copy(page.data[pagePos:], data))
		page.mode |= pageModeDirty
		return nil
	})
	if err != nil {
		w.chunkErr = err
		return
	}
	pCtx, tryCommit := context.WithCancel(ctx)
	w.dirtyPages = append(w.dirtyPages, &dirtyPage{
		ctx:           pCtx,
		cancelWait:    tryCommit,
		pIdx:          pageIdx,
		node:          pNode,
		modifyAt:      time.Now(),
		readyToCommit: false,
	})
	if len(w.dirtyPages) == 1 {
		maximumChunkTaskParallel.Go(func() {
			w.commitPages()
		})
	}
}

func (w *segWriter) commitPages() {
	t := time.NewTicker(time.Millisecond * 100)
	defer t.Stop()
Next:
	for len(w.dirtyPages) > 0 {
		page := w.dirtyPages[0]
		for !page.readyToCommit {
			select {
			case <-page.ctx.Done():
				if !page.readyToCommit {
					w.dirtyPages = w.dirtyPages[1:]
					atomic.AddInt32(&w.unready, -1)
					continue Next
				}
			case <-t.C:
				if time.Since(page.modifyAt) > fileChunkCommitTimeout {
					page.tryCommit()
				}
			}
		}
		chunkSegId, err := w.store.NextSegmentID(context.Background())
		if err != nil {
			w.chunkErr = err
			continue
		}

		if err = w.storage.Put(context.Background(), chunkSegId, page.pIdx, page.node.pos, page.node.data); err != nil {
			w.chunkErr = err
			continue
		}

		if err = w.store.AppendSegments(context.Background(), types.ChunkSeg{
			ID:       chunkSegId,
			ChunkID:  w.chunkID,
			ObjectID: w.Object.ID,
			Off:      page.pIdx*pageSize + page.node.pos,
			Len:      page.node.length,
			State:    0,
		}, w.Object); err != nil {
			w.chunkErr = err
			continue
		}
		page.node.commit()
		w.dirtyPages = w.dirtyPages[1:]
		atomic.AddInt32(&w.unready, -1)
	}
}

type dirtyPage struct {
	ctx           context.Context
	cancelWait    func()
	pIdx          int64
	node          *pageNode
	modifyAt      time.Time
	readyToCommit bool
}

func (d *dirtyPage) tryCommit() {
	d.readyToCommit = true
	d.cancelWait()
}

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}

type ioReq struct {
	off     int64
	dest    []byte
	isReady bool
	err     error
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
