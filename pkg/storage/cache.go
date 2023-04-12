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
	"io/fs"
	"os"
	"path"
	"path/filepath"
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

	cachedFile *cachedFileMapper
	cacheLog   *zap.SugaredLogger
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
	if err := cachedFile.load(localCacheDir); err != nil {
		cacheLog.Panicf("load cached file failed: %s", err)
	}
	// TODO: load uncommitted data
	cacheLog.Infow("local size usage: %d, limit: %d", localCacheSizeUsage, localCacheSizeLimit)
}

type LocalCache struct {
	s       Storage
	lfu     *utils.LFU
	isLocal bool
}

func NewLocalCache(s Storage) *LocalCache {
	lc := &LocalCache{
		s:   s,
		lfu: utils.NewLFURegistry(),
	}

	if _, isLocalStorage := s.(*local); isLocalStorage {
		lc.isLocal = true
	}
	return lc
}

func (c *LocalCache) OpenTemporaryNode(ctx context.Context, oid, off int64) (CacheNode, error) {
	if err := c.mustAccountCacheUsage(ctx, cacheNodeSize); err != nil {
		return nil, err
	}

	tFile := c.localTemporaryFilePath(oid, off)
	f, err := os.Create(tFile)
	if err != nil {
		return nil, err
	}
	return &fileCacheNode{file: f, path: tFile}, nil
}

func (c *LocalCache) CommitTemporaryNode(ctx context.Context, segID, idx int64, node CacheNode) error {
	no := node.(*fileCacheNode)
	f := no.file
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		cacheLog.Errorw("seek node to start error", "page", idx, "err", err)
		return err
	}
	buffer := make([]byte, cacheNodeSize)
	n, err := f.Read(buffer)
	if err != nil && err != io.EOF {
		cacheLog.Errorw("read local cache node error", "page", idx, "err", err)
		return err
	}
	if err = c.s.Put(ctx, segID, idx, 0, buffer[:n]); err != nil {
		cacheLog.Errorw("send cache data to storage error", "page", idx, "err", err)
		return err
	}
	defer func() {
		if innerErr := os.Remove(no.path); innerErr != nil {
			cacheLog.Errorw("clean cache data error", "page", idx, "err", innerErr)
		}
		c.releaseCacheUsage(no.size)
	}()
	return node.Close()
}

func (c *LocalCache) OpenCacheNode(ctx context.Context, key, idx int64) (CacheNode, error) {
	if c.isLocal {
		return c.mappingLocalStorage(key, idx)
	}

	node, err := cachedFile.fetchCacheNode(key, idx, func(filename string) (CacheNode, error) {
		return c.makeLocalCache(ctx, key, idx, filename)
	})
	if err != nil {
		return nil, err
	}
	return node, node.Attach()
}

func (c *LocalCache) makeLocalCache(ctx context.Context, key, idx int64, filename string) (*cacheNode, error) {
	info, err := c.s.Head(ctx, key, idx)
	if err != nil {
		return nil, err
	}

	if err = c.mustAccountCacheUsage(ctx, info.Size); err != nil {
		return nil, err
	}

	cacheFilePath := localCacheFilePath(filename)
	f, err := os.Create(cacheFilePath)
	if err != nil {
		return nil, err
	}

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

	node := &cacheNode{key: key, idx: idx, path: cacheFilePath}
	c.lfu.Register(filename, node)
	return node, nil
}

func (c *LocalCache) mappingLocalStorage(key, idx int64) (*cacheNode, error) {
	node := &cacheNode{key: key, idx: idx, path: c.s.(*local).key2LocalPath(key, idx)}
	return node, nil
}

func (c *LocalCache) mustAccountCacheUsage(ctx context.Context, usage int64) error {
	rmCacheFile := 1
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			crtCached := atomic.LoadInt64(&localCacheSizeUsage)
			if crtCached+cacheNodeSize > localCacheSizeLimit {
				if err := c.cleanSpace(rmCacheFile); err != nil {
					return err
				}
				rmCacheFile += 1
				continue
			}
			if atomic.CompareAndSwapInt64(&localCacheSizeUsage, crtCached, crtCached+usage) {
				return nil
			}
		}
	}
}

func (c *LocalCache) releaseCacheUsage(usage int64) {
	atomic.AddInt64(&localCacheSizeUsage, -1*usage)
}

func (c *LocalCache) cleanSpace(fileCount int) error {
	c.lfu.TryEvict(func(key string, val interface{}) bool {
		no, ok := val.(*cacheNode)
		if !ok {
			return false
		}
		if atomic.LoadInt32(&no.ref) == 0 {
			return cachedFile.removeCacheNode(no.key, no.idx) == nil
		}
		return false
	}, fileCount)
	return nil
}

func (c *LocalCache) localTemporaryFilePath(oid, off int64) string {
	return path.Join(localCacheDir, fmt.Sprintf("t_%d_%d_%d", oid, off, time.Now().UnixNano()))
}

type CacheNode interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
	Attach() error
	Size() int64
}

type fileCacheNode struct {
	file *os.File
	path string
	size int64
	mux  sync.Mutex
}

var _ CacheNode = &fileCacheNode{}

func (f *fileCacheNode) Attach() error {
	if f.file != nil {
		return nil
	}
	s, err := os.Stat(f.path)
	if err != nil {
		return err
	}
	f.size = s.Size()
	f.file, err = os.Open(f.path)
	return err
}

func (f *fileCacheNode) ReadAt(p []byte, off int64) (n int, err error) {
	return f.file.ReadAt(p, off)
}

func (f *fileCacheNode) WriteAt(p []byte, off int64) (n int, err error) {
	n, err = f.file.WriteAt(p, off)
	if off+int64(n) > f.size {
		f.size = off + int64(n)
	}
	return
}

func (f *fileCacheNode) Close() error {
	file := f.file
	f.file = nil
	return file.Close()
}

func (f *fileCacheNode) Size() int64 {
	return f.size
}

type cacheNode struct {
	key, idx int64

	ref  int32
	path string
	data []byte
	size int64
}

var _ CacheNode = &cacheNode{}

func (c *cacheNode) Attach() error {
	if atomic.LoadInt32(&c.ref) > 0 {
		atomic.AddInt32(&c.ref, 1)
		return nil
	}
	atomic.AddInt32(&c.ref, 1)

	s, err := os.Stat(c.path)
	if err != nil {
		return err
	}
	c.size = s.Size()
	f, err := os.Open(c.path)
	if err != nil {
		return err
	}
	defer f.Close()

	c.data, err = unix.Mmap(int(f.Fd()), 0, cacheNodeSize, unix.PROT_WRITE|unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		return err
	}
	return nil
}

func (c *cacheNode) WriteAt(p []byte, off int64) (n int, err error) {
	if off >= c.size {
		return 0, io.EOF
	}
	n = copy(c.data[off:], p)
	if off+int64(n) > c.size {
		c.size = off + int64(n)
	}
	return n, nil
}

func (c *cacheNode) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= c.size {
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
		return nil
	}
	return nil
}

func (c *cacheNode) Size() int64 {
	return c.size
}

type cachedFileMapper struct {
	cachedNode map[string]CacheNode
	pending    map[string]struct{}
	cond       *sync.Cond
	mux        sync.Mutex
}

func (c *cachedFileMapper) load(cacheDir string) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	return filepath.Walk(cacheDir, func(filePath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		filename := path.Base(filePath)
		c.cachedNode[filename] = &cacheNode{
			path: filePath,
			size: info.Size(),
		}
		atomic.AddInt64(&localCacheSizeUsage, info.Size())
		return nil
	})
}

type cacheBuilder func(filename string) (CacheNode, error)

func (c *cachedFileMapper) fetchCacheNode(key int64, idx int64, builder cacheBuilder) (CacheNode, error) {
	fileName := c.filename(key, idx)

	c.mux.Lock()
	node, ok := c.cachedNode[fileName]
	if ok {
		c.mux.Unlock()
		return node, nil
	}

	_, stillFetching := c.pending[fileName]
	if stillFetching {
		for stillFetching {
			c.cond.Wait()
			_, stillFetching = c.pending[fileName]
		}
		node = c.cachedNode[fileName]
		c.mux.Unlock()
		return node, nil
	} else {
		c.pending[fileName] = struct{}{}
	}
	c.mux.Unlock()

	var err error
	node, err = builder(fileName)
	if err != nil {
		return nil, err
	}

	c.mux.Lock()
	delete(c.pending, fileName)
	c.cachedNode[fileName] = node
	c.mux.Unlock()
	return node, nil
}

func (c *cachedFileMapper) removeCacheNode(key int64, idx int64) error {
	fileName := c.filename(key, idx)
	c.mux.Lock()
	_, ok := c.cachedNode[fileName]
	if !ok {
		c.mux.Unlock()
		return nil
	}
	c.mux.Unlock()

	fPath := localCacheFilePath(fileName)
	info, err := os.Stat(fPath)
	if err != nil {
		if err == os.ErrNotExist {
			return nil
		}
	}
	err = os.Remove(fPath)
	if err != nil {
		return err
	}

	c.mux.Lock()
	delete(c.cachedNode, fileName)
	c.mux.Unlock()
	atomic.AddInt64(&localCacheSizeUsage, -1*info.Size())
	return nil
}

func (c *cachedFileMapper) filename(key int64, idx int64) string {
	return fmt.Sprintf("%d_%d", key, idx)
}

func localCacheFilePath(filename string) string {
	return path.Join(localCacheDir, filename)
}

func init() {
	cachedFile = &cachedFileMapper{
		cachedNode: map[string]CacheNode{},
		pending:    map[string]struct{}{},
	}
	cachedFile.cond = sync.NewCond(&cachedFile.mux)
}
