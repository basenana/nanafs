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

package storage

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"
)

const (
	cacheNodeSize = 1 << 21 // 2M
)

var (
	localCacheDir       string
	localCacheSizeLimit int64
	localCacheSizeUsage int64

	cacheLog *zap.SugaredLogger
)

func InitLocalCache(config config.Config) {
	cacheLog = logger.NewLogger("localCache")
	localCacheDir = config.CacheDir
	if localCacheDir == "" {
		cacheLog.Panic("init local cache dri dir failed: empty")
	}
	localCacheSizeLimit = int64(config.CacheSize) * (1 << 30) // config.CacheSize Gi
	if err := utils.Mkdir(localCacheDir); err != nil {
		cacheLog.Panicf("init local cache dri dir failed: %s", err)
	}
}

type LocalCache struct {
	s       Storage
	nodes   map[int64]map[int64]CacheNode
	pending map[string]struct{}
	cond    *sync.Cond
	mux     sync.Mutex
}

func NewLocalCache(s Storage) *LocalCache {
	lc := &LocalCache{
		s:       s,
		nodes:   map[int64]map[int64]CacheNode{},
		pending: map[string]struct{}{},
	}
	lc.cond = sync.NewCond(&lc.mux)
	return lc
}

func (c *LocalCache) OpenChunkNode(ctx context.Context, key, idx int64) (CacheNode, error) {
	var err error

	c.mux.Lock()
	nodes, ok := c.nodes[key]
	if !ok {
		nodes = map[int64]CacheNode{}
		c.nodes[key] = nodes
	}
	node, ok := nodes[idx]
	c.mux.Unlock()
	if !ok {
		node, err = c.makeLocalCache(ctx, key, idx)
		if err != nil {
			cacheLog.Errorw("make cache node failed", "key", key, "idx", idx, "err", err)
			return nil, err
		}
	}

	if err = node.Attach(); err != nil {
		cacheLog.Errorw("attach cache node failed", "key", key, "idx", idx, "err", err)
		return nil, err
	}
	return node, nil
}

func (c *LocalCache) OpenTemporaryNode(ctx context.Context, oid, off int64) (CacheNode, error) {
	tFile := c.localTemporaryFilePath(oid, off)
	f, err := os.Create(tFile)
	if err != nil {
		return nil, err
	}
	if err = f.Truncate(cacheNodeSize); err != nil {
		return nil, err
	}
	defer f.Close()
	node := &cacheNode{path: tFile, uncommitted: true}
	if err = node.Attach(); err != nil {
		return nil, err
	}
	return node, nil
}

func (c *LocalCache) CommitTemporaryNode(ctx context.Context, segID, idx int64, node CacheNode) error {
	no := node.(*cacheNode)
	if err := c.s.Put(ctx, segID, idx, 0, no.data[:no.size]); err != nil {
		return err
	}
	no.uncommitted = false
	// TODO: rename
	defer os.Remove(no.path)
	return node.Close()
}

func (c *LocalCache) makeLocalCache(ctx context.Context, key, idx int64) (CacheNode, error) {
	if _, isLocalStorage := c.s.(*local); isLocalStorage {
		return c.mappingLocalStorage(ctx, key, idx)
	}

	fetchKey := fmt.Sprintf("%d_%d", key, idx)
	c.mux.Lock()
	_, stillFetching := c.pending[fetchKey]
	if stillFetching {
		for stillFetching {
			c.cond.Wait()
			_, stillFetching = c.pending[fetchKey]
		}
		node := c.nodes[key][idx]
		c.mux.Unlock()
		return node, nil
	} else {
		c.pending[fetchKey] = struct{}{}
	}
	c.mux.Unlock()

	info, err := c.s.Head(ctx, key, idx)
	if err != nil {
		return nil, err
	}

	for {
		crtCached := atomic.LoadInt64(&localCacheSizeUsage)
		if crtCached+info.Size > localCacheSizeLimit {
			if err = c.cleanSpaceAndWait(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if atomic.CompareAndSwapInt64(&localCacheSizeUsage, crtCached, crtCached+info.Size) {
			break
		}
	}

	cacheFilePath := c.localCacheFilePath(key, idx)
	f, err := os.Create(cacheFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		off, n int64
		buf    = make([]byte, 1024)
	)
	for {
		n, err = c.s.Get(ctx, key, idx, off, buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		off += n
		_, err = f.Write(buf[:n])
		if err != nil {
			return nil, err
		}
	}

	node := &cacheNode{path: cacheFilePath}
	c.mux.Lock()
	c.nodes[key][idx] = node
	delete(c.pending, fetchKey)
	c.mux.Unlock()
	c.cond.Broadcast()

	return node, nil
}

func (c *LocalCache) mappingLocalStorage(ctx context.Context, key, idx int64) (CacheNode, error) {
	node := &cacheNode{path: c.s.(*local).key2LocalPath(key, idx)}
	c.mux.Lock()
	c.nodes[key][idx] = node
	c.mux.Unlock()
	return node, nil
}

func (c *LocalCache) cleanSpaceAndWait(ctx context.Context) error {
	return nil
}

func (c *LocalCache) localTemporaryFilePath(oid, off int64) string {
	return path.Join(localCacheDir, fmt.Sprintf("t_%d_%d_%d", oid, off, time.Now().UnixNano()))
}

func (c *LocalCache) localCacheFilePath(key, idx int64) string {
	return path.Join(localCacheDir, fmt.Sprintf("%d_%d", key, idx))
}

type CacheNode interface {
	Attach() error
	io.ReaderAt
	io.WriterAt
	io.Closer
}

type cacheNode struct {
	ref         int32
	f           *os.File
	path        string
	data        []byte
	size        int64
	uncommitted bool
}

var _ CacheNode = &cacheNode{}

func (c *cacheNode) Attach() error {
	if atomic.LoadInt32(&c.ref) > 0 {
		atomic.AddInt32(&c.ref, 1)
		return nil
	}
	f, err := os.Open(c.path)
	if err != nil {
		return err
	}
	c.data, err = unix.Mmap(int(f.Fd()), 0, cacheNodeSize, unix.PROT_WRITE|unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		_ = f.Close()
		return err
	}
	c.f = f
	atomic.AddInt32(&c.ref, 1)
	return nil
}

func (c *cacheNode) WriteAt(p []byte, off int64) (n int, err error) {
	if off >= cacheNodeSize {
		return 0, io.EOF
	}
	n = copy(c.data[off:], p)
	if off+int64(n) > c.size {
		c.size = off + int64(n)
	}
	return n, nil
}

func (c *cacheNode) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= cacheNodeSize {
		return 0, io.EOF
	}
	n = copy(p, c.data[off:])
	return n, nil
}

func (c *cacheNode) Close() error {
	if atomic.AddInt32(&c.ref, -1) == 0 {
		if err := unix.Munmap(c.data); err != nil {
			cacheLog.Errorw("detach cache node failed", "path", c.path, "err", err)
			return err
		}
		c.data = nil
		f := c.f
		c.f = nil
		return f.Close()
	}
	return nil
}
