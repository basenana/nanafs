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
	"bytes"
	"container/heap"
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"
)

const (
	cacheNodeSize = 1 << 21 // 2M
)

var (
	localCacheDir       string
	localCacheSizeUsage int64
	localCacheSizeLimit int64 = 10 * (1 << 30)

	cachedFile *cachedFileMapper
	cacheLog   *zap.SugaredLogger
)

func init() {
	cachedFile = &cachedFileMapper{
		cachedNode:  map[string]*nodeUsingInfo{},
		pending:     map[string]struct{}{},
		unusedNodeQ: &priorityNodeQueue{},
	}
	cachedFile.cond = sync.NewCond(&cachedFile.mux)

	heap.Init(cachedFile.unusedNodeQ)
}

func InitLocalCache(config config.Config) {
	cacheLog = logger.NewLogger("localCache")
	localCacheDir = config.CacheDir
	if localCacheDir == "" {
		cacheLog.Panic("init local cache dri dir failed: empty")
	}
	cacheLog.Infof("local cache dir: %s", localCacheDir)
	localCacheSizeLimit = int64(config.CacheSize) * (1 << 30) // config.CacheSize Gi
	if err := utils.Mkdir(localCacheDir); err != nil {
		cacheLog.Panicf("init local cache dir failed: %s", err)
	}
	if err := cachedFile.load(localCacheDir); err != nil {
		cacheLog.Panicf("load cached file failed: %s", err)
	}
	// TODO: load uncommitted data
	cacheLog.Infof("local size usage: %d, limit: %d", localCacheSizeUsage, localCacheSizeLimit)
}

type LocalCache struct {
	s       Storage
	isLocal bool
}

func NewLocalCache(s Storage) *LocalCache {
	lc := &LocalCache{s: s}

	if _, isLocalStorage := s.(*local); isLocalStorage {
		lc.isLocal = true
	}
	return lc
}

func (c *LocalCache) OpenTemporaryNode(ctx context.Context, oid, off int64) (CacheNode, error) {
	defer trace.StartRegion(ctx, "storage.localCache.OpenTemporaryNode").End()
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
	defer trace.StartRegion(ctx, "storage.localCache.CommitTemporaryNode").End()
	no := node.(*fileCacheNode)
	f := no.file
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		cacheLog.Errorw("seek node to start error", "page", idx, "err", err)
		return err
	}
	var buf bytes.Buffer
	if compressErr := compress(ctx, f, &buf); compressErr != nil {
		cacheLog.Errorw("compress temporary node data error", "page", idx, "err", compressErr)
	}
	if err := c.s.Put(ctx, segID, idx, &buf); err != nil {
		cacheLog.Errorw("send cache data to storage error", "page", idx, "err", err)
		return err
	}
	defer func() {
		if innerErr := os.Remove(no.path); innerErr != nil {
			cacheLog.Errorw("clean cache data error", "page", idx, "err", innerErr)
		}
		c.releaseCacheUsage(cacheNodeSize)
	}()
	return node.Close()
}

func (c *LocalCache) OpenCacheNode(ctx context.Context, key, idx int64, readBack int) (CacheNode, error) {
	defer trace.StartRegion(ctx, "storage.localCache.OpenCacheNode").End()
	if c.isLocal {
		return c.mappingLocalStorage(key, idx)
	}

	var (
		node CacheNode
		err  error
	)
	node, err = cachedFile.fetchCacheNode(
		key, idx,
		func(filename string) (CacheNode, error) { return c.makeLocalCache(ctx, key, idx, filename) },
		func(_ string) (CacheNode, error) { return c.openDirectNode(ctx, key, idx) },
		atomic.LoadInt64(&localCacheSizeUsage)+cacheNodeSize > localCacheSizeLimit || readBack+30 > rand.Int()%100,
	)
	if err != nil {
		return nil, err
	}
	return node, node.Attach()
}

func (c *LocalCache) openDirectNode(ctx context.Context, key, idx int64) (*directNode, error) {
	defer trace.StartRegion(ctx, "storage.localCache.openDirectNode").End()
	info, err := c.s.Head(ctx, key, idx)
	if err != nil {
		return nil, err
	}

	var (
		reader io.ReadCloser
		buf    bytes.Buffer
	)
	reader, err = c.s.Get(ctx, key, idx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if decompressErr := decompress(ctx, reader, &buf); decompressErr != nil {
		cacheLog.Errorw("decompress direct node data error", "page", idx, "err", decompressErr)
	}

	node := &directNode{data: buf.Bytes(), info: info}
	return node, nil
}

func (c *LocalCache) makeLocalCache(ctx context.Context, key, idx int64, filename string) (*cacheNode, error) {
	defer trace.StartRegion(ctx, "storage.localCache.makeLocalCache").End()
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
		reader io.ReadCloser
		buf    bytes.Buffer
	)
	reader, err = c.s.Get(ctx, key, idx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if decompressErr := decompress(ctx, reader, &buf); decompressErr != nil {
		cacheLog.Errorw("decompress local node data error", "page", idx, "err", decompressErr)
	}

	_, err = io.Copy(f, &buf)
	if err != nil {
		return nil, err
	}

	node := &cacheNode{path: cacheFilePath}
	return node, nil
}

func (c *LocalCache) mappingLocalStorage(key, idx int64) (*cacheNode, error) {
	node := &cacheNode{path: c.s.(*local).key2LocalPath(key, idx)}
	return node, node.Attach()
}

func (c *LocalCache) mustAccountCacheUsage(ctx context.Context, usage int64) error {
	defer trace.StartRegion(ctx, "storage.localCache.mustAccountCacheUsage").End()
	rmCacheFile := 1
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			crtCached := atomic.LoadInt64(&localCacheSizeUsage)
			if crtCached+usage > localCacheSizeLimit {
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
	var err error
	for i := 0; i < fileCount; i++ {
		if err = cachedFile.removeNextCacheFile(); err != nil {
			return err
		}
	}
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
	freq() int
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

func (f *fileCacheNode) freq() int {
	return 1
}

type cacheNode struct {
	ref    int32
	path   string
	data   []byte
	size   int64
	attach int
}

var _ CacheNode = &cacheNode{}

func (c *cacheNode) Attach() error {
	c.attach += 1
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
		cachedFile.returnNode(path.Base(c.path))
		return nil
	}
	return nil
}

func (c *cacheNode) freq() int {
	return c.attach
}

func (c *cacheNode) Size() int64 {
	return c.size
}

type directNode struct {
	info Info
	data []byte
}

func (d *directNode) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= d.info.Size {
		return 0, io.EOF
	}
	n = copy(p, d.data[off:])
	if int64(n+len(p)) == d.info.Size {
		return n, io.EOF
	}
	return
}

func (d *directNode) WriteAt(p []byte, off int64) (n int, err error) {
	// TODO
	return 0, nil
}

func (d *directNode) Close() error {
	return nil
}

func (d *directNode) Attach() error {
	return nil
}

func (d *directNode) Size() int64 {
	return d.info.Size
}

func (d *directNode) freq() int {
	return 0
}

type cachedFileMapper struct {
	cachedNode  map[string]*nodeUsingInfo
	pending     map[string]struct{}
	unusedNodeQ *priorityNodeQueue
	cond        *sync.Cond
	mux         sync.Mutex
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
		node := &cacheNode{path: filePath, size: info.Size()}
		c.cachedNode[filename] = &nodeUsingInfo{filename: filename, node: node, updateAt: time.Now().UnixNano()}
		heap.Push(c.unusedNodeQ, c.cachedNode[filename])
		atomic.AddInt64(&localCacheSizeUsage, info.Size())
		return nil
	})
}

type cacheBuilder func(filename string) (CacheNode, error)

func (c *cachedFileMapper) fetchCacheNode(key int64, idx int64, builder, directer cacheBuilder, canDirect bool) (CacheNode, error) {
	fileName := c.filename(key, idx)

	c.mux.Lock()
	vNode, ok := c.cachedNode[fileName]
	if ok {
		if vNode.index != -1 && len(*c.unusedNodeQ) > 0 {
			heap.Remove(c.unusedNodeQ, vNode.index)
			vNode.index = -1
		}
		c.mux.Unlock()
		return vNode.node, nil
	}

	_, stillFetching := c.pending[fileName]
	if stillFetching {
		for stillFetching {
			c.cond.Wait()
			_, stillFetching = c.pending[fileName]
		}
		vNode = c.cachedNode[fileName]
		c.mux.Unlock()
		return vNode.node, nil
	} else {
		if canDirect {
			c.mux.Unlock()
			node, err := directer(fileName)
			return node, err
		}
		// try fetch
		c.pending[fileName] = struct{}{}
	}
	c.mux.Unlock()

	var (
		err  error
		node CacheNode
	)
	node, err = builder(fileName)
	if err != nil {
		c.mux.Lock()
		delete(c.pending, fileName)
		c.mux.Unlock()
		c.cond.Broadcast()
		return nil, err
	}

	c.mux.Lock()
	delete(c.pending, fileName)
	c.cachedNode[fileName] = &nodeUsingInfo{filename: fileName, node: node, updateAt: time.Now().UnixNano()}
	c.mux.Unlock()
	c.cond.Broadcast()
	return node, nil
}

func (c *cachedFileMapper) removeNextCacheFile() error {
	c.mux.Lock()
	if len(*c.unusedNodeQ) == 0 {
		c.mux.Unlock()
		return nil
	}
	vNode := heap.Pop(c.unusedNodeQ).(*nodeUsingInfo)
	if vNode == nil {
		c.mux.Unlock()
		return nil
	}
	filename := vNode.filename
	node, ok := c.cachedNode[filename]
	if !ok {
		c.mux.Unlock()
		return nil
	}
	c.mux.Unlock()

	fPath := localCacheFilePath(filename)
	info, err := os.Stat(fPath)
	if err != nil && err != os.ErrNotExist {
		return err
	}
	if err == nil {
		err = os.Remove(fPath)
		if err != nil {
			return err
		}
	}

	c.mux.Lock()
	if node.index != -1 && len(*c.unusedNodeQ) > 0 {
		heap.Remove(c.unusedNodeQ, node.index)
	}
	delete(c.cachedNode, filename)
	c.mux.Unlock()
	atomic.AddInt64(&localCacheSizeUsage, -1*info.Size())
	return nil
}

func (c *cachedFileMapper) returnNode(filename string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	node := c.cachedNode[filename]
	if node == nil {
		return
	}
	node.updateAt = time.Now().Unix()
	heap.Push(c.unusedNodeQ, node)
}

func (c *cachedFileMapper) filename(key int64, idx int64) string {
	return fmt.Sprintf("%d_%d", key, idx)
}

func localCacheFilePath(filename string) string {
	return path.Join(localCacheDir, filename)
}
