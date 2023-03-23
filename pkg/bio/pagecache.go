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
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
)

const (
	pageModeEmpty   = 0
	pageModeInitial = 1
	pageModeInvalid = 1 << 1
	pageTreeShift   = 6
	pageTreeSize    = 1 << pageTreeShift
	pageTreeMask    = pageTreeSize - 1
	pageSize        = 1 << 21 // 2M
)

type pageCache struct {
	data      *pageRoot
	size      int64
	chunkSize int64
	storage   storage.Storage
	mux       sync.Mutex
	logger    *zap.SugaredLogger
}

func newPageCache(entryID, chunkSize int64) *pageCache {
	pc := &pageCache{
		data:      &pageRoot{},
		chunkSize: chunkSize,
		logger:    logger.NewLogger("pageCache").With(zap.Int64("entry", entryID)),
	}
	return pc
}

func (p *pageCache) read(ctx context.Context, pageIndex int64, segments []segment) (page *pageNode, err error) {
	ctx, endF := utils.TraceTask(ctx, "pagecache.read")
	defer endF()
	page = p.findPage(pageIndex)
	if page == nil {
		var (
			n         int64
			onceRead  int64
			pageStart = pageSize * pageIndex
		)
		page = p.insertPage(ctx, pageIndex)
		page.mux.Lock()
		for _, seg := range segments {
			onceRead, err = p.storage.Get(ctx, seg.id, pageIndex, seg.off-pageStart, page.data[n:n+seg.len])
			for err != nil {
				return
			}
			n += onceRead
		}
		page.mode ^= pageModeInitial
		page.mux.Unlock()
		return nil, types.ErrPageFault
	}
	return page, nil
}

func (p *pageCache) invalidate(pageIndex int64) {
	page := p.findPage(pageIndex)
	if page == nil {
		return
	}
	page.mux.Lock()
	page.mode |= pageModeInvalid
	page.data = nil
	page.mux.Unlock()
}

func (p *pageCache) insertPage(ctx context.Context, pageIdx int64) *pageNode {
	ctx, endF := utils.TraceTask(ctx, "pagecache.insert")
	defer endF()
	p.mux.Lock()
	if p.data.rootNode == nil {
		p.data.rootNode = newPage(0, pageSize, pageModeEmpty)
	}

	if (pageTreeSize<<p.data.rootNode.shift)-1 < pageIdx {
		p.extendPageTree(pageIdx)
	}

	var (
		node  = p.data.rootNode
		shift = p.data.rootNode.shift
		slot  int64
	)
	for shift > 0 {
		slot = pageIdx >> node.shift & pageTreeMask
		next := node.slots[slot]
		if next == nil {
			next = newPage(shift-pageTreeShift, pageSize, 0)
			node.slots[slot] = next
		}
		node = next
		shift -= pageTreeShift
	}

	dataNode := newPage(0, pageSize, pageModeInitial)

	slot = pageIdx & pageTreeMask
	node.slots[slot] = dataNode

	p.data.totalCount += 1
	p.mux.Unlock()
	return dataNode
}

func (p *pageCache) findPage(pageIdx int64) *pageNode {
	if p.data.rootNode == nil {
		return nil
	}

	if (pageTreeSize<<p.data.rootNode.shift)-1 < pageIdx {
		return nil
	}

	p.mux.Lock()
	var (
		node  = p.data.rootNode
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

	return node
}

func (p *pageCache) extendPageTree(index int64) {
	var (
		node  = p.data.rootNode
		shift = node.shift
	)

	// how max shift to extend
	maxShift := shift
	for index > (pageTreeSize<<maxShift)-1 {
		maxShift += pageTreeShift
	}
	for shift <= maxShift {
		shift += pageTreeShift
		parent := newPage(shift, pageSize, pageModeEmpty)
		parent.shift = shift
		parent.slots[0] = node

		node.parent = parent
		node = parent
	}
	p.data.rootNode = node
}

type pageRoot struct {
	rootNode   *pageNode
	cachedSize int64
	pageLists  []*pageNode
	totalCount int
}

type pageNode struct {
	slots  []*pageNode
	parent *pageNode
	shift  int
	pos    int64
	data   []byte
	length int64
	ref    int32
	mode   int8
	mux    sync.Mutex
}

func (n *pageNode) release() {
	if atomic.AddInt32(&n.ref, -1) == 0 {
		// TODO: need delay release
		n.mode = pageModeInitial
		n.data = nil
	}
}

// TODO: need a page pool
func newPage(shift int, pageSize int64, mode int8) *pageNode {
	p := &pageNode{
		shift:  shift,
		mode:   mode,
		length: pageSize,
	}
	switch {
	case p.mode == pageModeEmpty:
		p.slots = make([]*pageNode, pageTreeSize)
	case p.mode&pageModeInitial > 0:
		p.data = make([]byte, pageSize)
	}
	return p
}

func computePageIndex(off int64) (idx int64, pos int64) {
	idx = off / pageSize
	pos = off % pageSize
	return
}
