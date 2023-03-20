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
	"time"
)

const (
	fileChunkSize = 1 << 26 // 64MB
)

type chunkReader struct {
	*types.Object

	page    *pageCache
	store   storage.ChunkStore
	storage storage.Storage
	readers map[int64]*segReader
	mux     sync.Mutex
}

func NewChunkReader(obj *types.Object, store storage.ChunkStore) Reader {
	cr := &chunkReader{
		Object:  obj,
		page:    newPageCache(obj.ID, fileChunkSize),
		store:   store,
		readers: make(map[int64]*segReader, obj.Size/fileChunkSize+1),
	}
	cr.mux.Lock()
	return cr
}

func (c *chunkReader) ReadAt(ctx context.Context, dest []byte, off int64) (uint32, error) {
	ctx, endF := utils.TraceTask(ctx, "chunkreader.readat")
	defer endF()

	var (
		leftSize = int64(len(dest))
		n        = uint32(0)
		reqList  = make([]*readReq, 0)
		req      *readReq
		err      error
	)

	for int64(n) < leftSize {
		index, _ := computeChunkIndex(off, fileChunkSize)
		readEnd := (index + 1) * fileChunkSize
		if readEnd-off > leftSize {
			readEnd = off + leftSize
		}
		if readEnd > c.Object.Size {
			readEnd = c.Object.Size
		}

		req, err = c.prepareData(ctx, index, off, dest[n:readEnd-off])
		if err != nil {
			return 0, err
		}
		reqList = append(reqList, req)

		n += uint32(readEnd - off)
		off = readEnd
	}
	return n, c.waitIO(ctx, reqList)
}

func (c *chunkReader) waitIO(ctx context.Context, reqList []*readReq) (err error) {
	allFinish := false
	for !allFinish {
		allFinish = true
	waitIO:
		for _, req := range reqList {
			if !req.isReady {
				allFinish = false
				time.Sleep(time.Millisecond * 50)
				break waitIO
			}
		}
	}
	return
}

func (c *chunkReader) prepareData(ctx context.Context, index, off int64, dest []byte) (*readReq, error) {
	c.mux.Lock()
	sr, ok := c.readers[index]
	if !ok {
		sr = &segReader{chunkReader: c, chunkID: index}
		c.readers[index] = sr
	}
	c.mux.Unlock()

	req := &readReq{
		off:  off,
		dest: dest,
	}
	go sr.readRange(ctx, req)
	return req, nil
}

type chunkWriter struct {
	*chunkReader
	writers map[int64]*segWriter
}

func (c *chunkWriter) WriteAt(ctx context.Context, data []byte, off int64) (n uint32, err error) {
	var (
		leftSize  = uint32(len(data))
		onceWrite uint32
	)
	for {
		index, pos := computeChunkIndex(off, fileChunkSize)

		chunkEnd := (index+1)*fileChunkSize - off
		if chunkEnd > int64(leftSize) {
			chunkEnd = int64(leftSize)
		}

		cw, ok := c.writers[index]
		if !ok {
			// TODO
		}

		onceWrite, err = cw.writeAt(ctx, index, pos, data[n:chunkEnd])
		if err != nil {
			return
		}
		n += onceWrite
		if n == leftSize {
			break
		}
		off += int64(n)
	}
	return
}

func (c *chunkWriter) Fsync(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

type segReader struct {
	*chunkReader

	chunkID  int64
	segments []segment
	table    *segTable
	mux      sync.Mutex
}

func (r *segReader) readRange(ctx context.Context, req *readReq) {
	ctx, endF := utils.TraceTask(ctx, "segreader.readrange")
	defer endF()

	segments, err := r.store.ListSegments(ctx, r.ID, r.chunkID)
	if err != nil {
		req.err = err
		return
	}

	dataSize := (r.chunkID + 1) * int64(fileChunkSize)
	if r.Size < dataSize {
		dataSize = r.Size
	}
	st := buildSegmentTree(r.chunkID*fileChunkSize, dataSize, segments)

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
			defer wg.Done()
			if err = r.readPage(ctx, segments, pageID, off, dest); err != nil && req.err == nil {
				req.err = err
				endF()
				return
			}
		}(ctx, st.query(pageIdx*pageSize, (pageIdx+1)*pageSize), pageIdx, pageStart, req.dest[off-req.off:pageEnd-req.off])
		bufLeft -= pageEnd - off
		off = pageEnd
		if bufLeft == 0 {
			break
		}
	}
	wg.Wait()
}

func (r *segReader) readPage(ctx context.Context, segments []segment, pageID, off int64, dest []byte) error {
	page, err := r.page.read(ctx, pageID, segments)
	if err != nil {
		return err
	}
	copy(dest, page.data[off:])
	return nil
}

type segWriter struct {
	chunkID int64
	len     int64
}

func (w *segWriter) writeAt(ctx context.Context, index int64, off int64, data []byte) (n uint32, err error) {
	return 0, err
}

type segTable struct {
	objID   int64
	segID   int64
	off     int64
	len     uint32
	active  bool
	isReady bool
	err     error
	storage storage.Storage
	pages   []*pageNode
	next    *segTable
}

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}

type readReq struct {
	off     int64
	dest    []byte
	isReady bool
	err     error
}
