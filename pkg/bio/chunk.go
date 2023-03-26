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
	"sync"
	"sync/atomic"
	"time"
)

const (
	fileChunkSize = 1 << 26 // 64MB
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
		pageStart := pageIdx*pageSize + pos
		pageEnd := (pageIdx + 1) * pageSize
		if pageEnd-off > bufLeft {
			pageEnd = off + bufLeft
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
		}(ctx, st.query(pageIdx*pageSize, (pageIdx+1)*pageSize), pageIdx, pageStart, req.dest[off-req.off:pageEnd-req.off])
		bufLeft -= pageEnd - off
		off = pageEnd
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
			for i := crt; i < seg.off-off; i++ {
				page.data[i] = 0
			}
			crt = seg.off - off
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
}

func NewChunkWriter(reader Reader) Writer {
	r, ok := reader.(*chunkReader)
	if !ok {
		return nil
	}
	return &chunkWriter{chunkReader: r}
}

func (c *chunkWriter) WriteAt(ctx context.Context, data []byte, off int64) (n int64, err error) {
	var (
		writeEnd = off + int64(len(data))
		reqList  = make([]*ioReq, 0, writeEnd/fileChunkSize+1)
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
		reqList = append(reqList, c.flushData(ctx, index, off, data[n:n+readLen]))
		n += readLen
		off = chunkEnd
		if off == writeEnd {
			break
		}
	}
	return n, c.waitIO(ctx, reqList)
}
func (c *chunkWriter) flushData(ctx context.Context, index, off int64, dest []byte) *ioReq {
	req := &ioReq{
		off:  off,
		dest: dest,
	}
	atomic.AddInt32(&c.unready, 1)
	maximumChunkTaskParallel.BlockedGo(func() {
		chunkSegId, err := c.store.NextSegmentID(ctx)
		if err != nil {
			req.err = err
			return
		}
		c.writeChunkRange(ctx, index, chunkSegId, req)
		atomic.AddInt32(&c.unready, -1)
	})
	return req
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

func (c *chunkWriter) writeChunkRange(ctx context.Context, chunkID, segID int64, req *ioReq) {
	var (
		off    = req.off
		bufEnd = off + int64(len(req.dest))
		pages  = make([]int64, 0)
		wg     = sync.WaitGroup{}
	)

	for {
		pageIdx, pos := computePageIndex(off)
		pageStart := pageIdx*pageSize + pos
		pageEnd := (pageIdx + 1) * pageSize
		if pageEnd > bufEnd {
			pageEnd = bufEnd
		}
		wg.Add(1)
		pages = append(pages, pageIdx)
		go func(ctx context.Context, pageID, off int64, data []byte) {
			maximumChunkTaskParallel.BlockedGo(func() {
				defer wg.Done()
				if err := c.storage.Put(ctx, segID, pageID, off-pageStart, data); err != nil {
					req.err = err
					return
				}
			})
		}(ctx, pageIdx, pageStart, req.dest[off-req.off:pageEnd-req.off])
		off = pageEnd
		if off == bufEnd {
			break
		}
	}
	wg.Wait()

	if err := c.store.AppendSegments(ctx, types.ChunkSeg{
		ID:       segID,
		ChunkID:  chunkID,
		ObjectID: c.Object.ID,
		Off:      req.off,
		Len:      int64(len(req.dest)),
		State:    0,
	}, c.Object); err != nil {
		req.err = err
		return
	}
	for _, pid := range pages {
		c.page.invalidate(pid)
	}
	req.isReady = true
	return
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
