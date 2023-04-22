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
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pageModeEmpty   = 0
	pageModeData    = 1
	pageModeInvalid = 1 << 1
	pageModeDirty   = 1 << 2
	pageTreeShift   = 6
	pageTreeSize    = 1 << pageTreeShift
	pageTreeMask    = pageTreeSize - 1
	pageSize        = 1 << 21 // 2M
)

var (
	crtPageCacheTotal   int32 = 0
	maxPageCacheTotal         = 1024
	pageCacheMux              = sync.Mutex{}
	pageReleaseInterval       = time.Minute * 5
	pageCacheReleaseQ         = make(chan *pageNode, maxPageCacheTotal/2)
	pageCacheCond             = sync.NewCond(&pageCacheMux)
	pageCacheLFU              = utils.NewLFUPool(maxPageCacheTotal - 1)
	pageCacheDataPool         = sync.Pool{New: func() any { return make([]byte, pageSize) }}
)

type pageCache struct {
	entryID   int64
	data      *pageRoot
	chunkSize int64
	mux       sync.Mutex
}

func newPageCache(entryID, chunkSize int64) *pageCache {
	pc := &pageCache{
		entryID:   entryID,
		data:      &pageRoot{},
		chunkSize: chunkSize,
	}
	return pc
}

func (p *pageCache) read(ctx context.Context, pageIndex int64, initDataFn func(*pageNode) error) (page *pageNode, err error) {
	defer trace.StartRegion(ctx, "bio.pageCache.read").End()
	p.mux.Lock()
	page = p.findPage(pageIndex)
	if page == nil {
		page = p.insertPage(ctx, pageIndex)
		page.entry = p.entryID
		page.idx = pageIndex
	}
	pageCacheLFU.Put(pageCacheKey(p.entryID, pageIndex), page)
	p.mux.Unlock()

	atomic.AddInt32(&page.ref, 1)
	page.mux.Lock()
	if page.mode&pageModeInvalid > 0 {
		for page.data == nil {
			page.data = pageCacheDataPool.Get().([]byte)
		}
		err = initDataFn(page)
		if err != nil {
			page.mode |= pageModeInvalid
			page.mux.Unlock()
			return nil, err
		}
		page.mode &^= pageModeInvalid
	}
	page.mux.Unlock()
	return page, nil
}

func (p *pageCache) insertPage(ctx context.Context, pageIdx int64) *pageNode {
	defer trace.StartRegion(ctx, "bio.pageCache.insertPage").End()
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

	dataNode := newPage(0, pageSize, pageModeData|pageModeInvalid)

	slot = pageIdx & pageTreeMask
	node.slots[slot] = dataNode

	p.data.totalCount += 1
	return dataNode
}

func (p *pageCache) findPage(pageIdx int64) *pageNode {
	if p.data.rootNode == nil {
		return nil
	}

	if (pageTreeSize<<p.data.rootNode.shift)-1 < pageIdx {
		return nil
	}

	var (
		node  = p.data.rootNode
		shift = node.shift
		slot  int64
	)
	for shift >= 0 {
		slot = pageIdx >> node.shift & pageTreeMask
		next := node.slots[slot]
		if next == nil {
			return nil
		}
		node = next
		shift -= pageTreeShift
	}

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

func (p *pageCache) close() {
	p.data.Visit(func(pNode *pageNode) {
		pageCacheLFU.Remove(pageCacheKey(pNode.entry, pNode.idx))
	})
}

type pageRoot struct {
	rootNode   *pageNode
	totalCount int
}

func (r *pageRoot) Visit(fn func(pNode *pageNode)) {
	r.visit(r.rootNode, fn)
}

func (r *pageRoot) visit(node *pageNode, fn func(pNode *pageNode)) {
	if node == nil {
		return
	}
	for _, nextNode := range node.slots {
		r.visit(nextNode, fn)
	}
	if node.mode&pageModeData > 0 {
		fn(node)
	}
}

type pageNode struct {
	entry  int64
	idx    int64
	slots  []*pageNode
	parent *pageNode
	shift  int
	pos    int64
	data   []byte
	length int64
	ref    int32
	mode   int8
	mux    sync.RWMutex
}

func (n *pageNode) release() {
	atomic.AddInt32(&n.ref, -1)
}

func (n *pageNode) commit() {
	n.mode |= pageModeInvalid
	n.mode &^= pageModeDirty
	n.release()
}

func newPage(shift int, pageSize int64, mode int8) *pageNode {
	p := &pageNode{
		shift:  shift,
		mode:   mode,
		length: pageSize,
	}
	switch {
	case p.mode == pageModeEmpty:
		p.slots = make([]*pageNode, pageTreeSize)
	case p.mode&pageModeData > 0:
		pageCacheMux.Lock()
		for {
			crtTotal := atomic.LoadInt32(&crtPageCacheTotal)
			if crtTotal >= int32(maxPageCacheTotal) {
				pageCacheCond.Wait()
				continue
			}
			if atomic.CompareAndSwapInt32(&crtPageCacheTotal, crtTotal, crtTotal+1) {
				break
			}
		}
		pageCacheMux.Unlock()
	}
	return p
}

func releasePage(pNode *pageNode) {
	atomic.AddInt32(&crtPageCacheTotal, -1)
	pNode.mux.Lock()
	pNode.mode |= pageModeInvalid
	if pNode.data != nil {
		pageCacheDataPool.Put(pNode.data)
		pNode.data = nil
	}
	pNode.mux.Unlock()
	pageCacheCond.Signal()
}

func computePageIndex(off int64) (idx int64, pos int64) {
	idx = off / pageSize
	pos = off % pageSize
	return
}

func pageCacheKey(oid, pIdx int64) string {
	return fmt.Sprintf("pg_%d_%d", oid, pIdx)
}

func init() {
	pageCacheLFU.HandlerRemove = func(k string, v interface{}) {
		pNode := v.(*pageNode)
		if atomic.LoadInt32(&pNode.ref) == 0 {
			releasePage(pNode)
			return
		}
		pageCacheReleaseQ <- pNode
	}

	go func() {
		ticker := time.NewTicker(pageReleaseInterval)
		for {
			select {
			case <-ticker.C:
				if atomic.LoadInt32(&crtPageCacheTotal) > int32(float64(maxPageCacheTotal)*0.8) {
					pageCacheLFU.Visit(func(k string, v interface{}) {
						pNode := v.(*pageNode)
						logger.NewLogger("pageCache").Infow("visit page", "pageIdx", pNode.idx, "ref", pNode.ref)
						if atomic.LoadInt32(&pNode.ref) == 0 {
							pageCacheLFU.Remove(k)
						}
					})
				}
			case pNode := <-pageCacheReleaseQ:
				if atomic.LoadInt32(&pNode.ref) == 0 {
					releasePage(pNode)
					continue
				}
				pageCacheLFU.Put(pageCacheKey(pNode.entry, pNode.idx), pNode)
			}
		}
	}()
}
