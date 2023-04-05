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
	cacheLog.Infow("local cache dir: %s", localCacheDir)
	localCacheSizeLimit = int64(config.CacheSize) * (1 << 30) // config.CacheSize Gi
	if err := utils.Mkdir(localCacheDir); err != nil {
		cacheLog.Panicf("init local cache dir failed: %s", err)
	}
	// TODO: load uncommitted data
	cacheLog.Infow("local size usage: %d, limit: %d", localCacheSizeUsage, localCacheSizeLimit)
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

func (c *LocalCache) OpenTemporaryNode(ctx context.Context, oid, off int64) (CacheNode, error) {
	tFile := c.localTemporaryFilePath(oid, off)
	f, err := os.Create(tFile)
	if err != nil {
		return nil, err
	}
	return &fileCacheNode{File: f, path: tFile}, nil
}

func (c *LocalCache) CommitTemporaryNode(ctx context.Context, segID, idx int64, node CacheNode) error {
	no := node.(*fileCacheNode)
	f := no.File
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		cacheLog.Errorw("seek node to start error", "page", idx, "err", err)
		return err
	}
	var (
		buffer = make([]byte, 524288)
		total  int64
	)
	for {
		n, err := f.Read(buffer)
		if err != nil && err != io.EOF {
			cacheLog.Errorw("read local cache node error", "page", idx, "err", err)
			return err
		}
		if n == 0 {
			break
		}
		if err = c.s.Put(ctx, segID, idx, total, buffer[:n]); err != nil {
			cacheLog.Errorw("send cache data to storage error", "page", idx, "err", err)
			return err
		}
		total += int64(n)
	}
	defer func() {
		if innerErr := os.Remove(no.path); innerErr != nil {
			cacheLog.Errorw("clean cache data error", "page", idx, "err", innerErr)
		}
	}()
	return node.Close()
}

func (c *LocalCache) cleanSpaceAndWait(ctx context.Context) error {
	return nil
}

func (c *LocalCache) localTemporaryFilePath(oid, off int64) string {
	return path.Join(localCacheDir, fmt.Sprintf("t_%d_%d_%d", oid, off, time.Now().UnixNano()))
}

type CacheNode interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
}

type fileCacheNode struct {
	*os.File
	path string
}

var _ CacheNode = &fileCacheNode{}

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
