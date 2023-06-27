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
	"container/heap"
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"
)

const (
	cacheNodeBaseSize = 1 << 21 // 2M
	retryInterval     = time.Millisecond * 100
	retryTimes        = 50
)

var (
	localCacheDir       string
	localCacheSizeUsage int64
	localCacheSizeLimit int64 = 10 * (1 << 30)

	cachedFile       *cachedFileMapper
	cacheLog         *zap.SugaredLogger
	configs          []config.Storage
	globalEncryptCfg *config.Encryption
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

	if localCacheDir != "" {
		if err := utils.Mkdir(localCacheDir); err != nil {
			cacheLog.Panicf("init local cache dir failed: %s", err)
		}

		if err := cachedFile.load(localCacheDir); err != nil {
			cacheLog.Panicf("load cached file failed: %s", err)
		}
	}
	cacheLog.Infof("local size usage: %d, limit: %d", localCacheSizeUsage, localCacheSizeLimit)

	if config.GlobalEncryption.Enable {
		globalEncryptCfg = &config.GlobalEncryption
	}
	configs = config.Storages
}

type LocalCache struct {
	s       Storage
	cfg     config.Storage
	isLocal bool
}

func NewLocalCache(s Storage) *LocalCache {
	lc := &LocalCache{s: s}

	for i, cfg := range configs {
		if cfg.ID == s.ID() {
			lc.cfg = configs[i]
			break
		}
	}
	if lc.cfg.Type == LocalStorage {
		lc.isLocal = true
	}
	return lc
}

func (c *LocalCache) OpenTemporaryNode(ctx context.Context, oid, off int64) (CacheNode, error) {
	const opOpenTemporaryNode = "open_temporary_node"
	defer logLocalCacheOperationLatency(opOpenTemporaryNode, time.Now())
	defer trace.StartRegion(ctx, "storage.localCache.OpenTemporaryNode").End()

	size := int64(cacheNodeBaseSize)
	if off > size {
		size *= 2
	}
	return &memCacheNode{data: utils.NewMemoryBlock(size), size: 0}, nil
}

func (c *LocalCache) CommitTemporaryNode(ctx context.Context, segID, idx int64, node CacheNode) error {
	const opCommitTemporaryNode = "commit_temporary_node"
	defer logLocalCacheOperationLatency(opCommitTemporaryNode, time.Now())
	defer trace.StartRegion(ctx, "storage.localCache.CommitTemporaryNode").End()

	no := node.(*memCacheNode)
	upload := func() error {
		pr, pw := io.Pipe()
		var encodeErr error
		go func() {
			defer pw.Close()
			if encodeErr = c.nodeDataEncode(ctx, segID^idx, utils.NewReader(no), pw); encodeErr != nil {
				cacheLog.Errorw("encode temporary node data error", "segment", segID, "page", idx, "err", encodeErr)
			}
		}()
		defer pr.Close()
		if err := c.s.Put(ctx, segID, idx, pr); err != nil {
			cacheLog.Errorw("send cache data to storage error", "segment", segID, "page", idx, "err", err)
			_, _ = io.Copy(ioutil.Discard, pr)
			return err
		}
		return encodeErr
	}

	var err error
	for i := 0; i < retryTimes; i++ {
		if err = upload(); err == nil {
			break
		}
		time.Sleep(retryInterval)
		cacheLog.Errorw("upload chunk page error, try again", "segment", segID, "page", idx, "tryTime", i+1)
	}
	if err != nil {
		return logErr(localCacheOperationErrorCounter, err, opCommitTemporaryNode)
	}

	return nil
}

func (c *LocalCache) OpenCacheNode(ctx context.Context, key, idx int64) (CacheNode, error) {
	const opOpenCacheNode = "open_cache_node"
	defer logLocalCacheOperationLatency(opOpenCacheNode, time.Now())
	defer trace.StartRegion(ctx, "storage.localCache.OpenCacheNode").End()
	if c.isLocal || localCacheSizeLimit == 0 {
		return c.openDirectNode(ctx, key, idx)
	}

	var (
		node CacheNode
		err  error
	)
	node, err = cachedFile.fetchCacheNode(ctx, key, idx, c.makeLocalCache, c.openLocalCache)
	if err != nil {
		return nil, logErr(localCacheOperationErrorCounter, err, opOpenCacheNode)
	}
	return node, nil
}

func (c *LocalCache) openDirectNode(ctx context.Context, key, idx int64) (CacheNode, error) {
	const opOpenDirectNode = "open_direct_node"
	defer logLocalCacheOperationLatency(opOpenDirectNode, time.Now())
	defer trace.StartRegion(ctx, "storage.localCache.openDirectNode").End()
	info, err := c.s.Head(ctx, key, idx)
	if err != nil {
		return nil, logErr(localCacheOperationErrorCounter, err, opOpenDirectNode)
	}

	var (
		reader io.ReadCloser
		node   = &memCacheNode{data: utils.NewMemoryBlock(info.Size)}
	)
	for i := 0; i < retryTimes; i++ {
		reader, err = c.s.Get(ctx, key, idx)
		if err == nil {
			break
		}
		time.Sleep(retryInterval)
		cacheLog.Errorw("read chunk page error, try again", "segment", key, "page", idx, "tryTime", i+1)
	}
	if err != nil {
		return nil, logErr(localCacheOperationErrorCounter, err, opOpenDirectNode)
	}
	defer reader.Close()
	if decodeErr := c.nodeDataDecode(ctx, key^idx, reader, utils.NewWriter(node)); decodeErr != nil {
		cacheLog.Errorw("decode direct node data error", "segment", key, "page", idx, "err", decodeErr)
		return nil, logErr(localCacheOperationErrorCounter, decodeErr, opOpenDirectNode)
	}

	return node, nil
}

func (c *LocalCache) makeLocalCache(ctx context.Context, key, idx int64, filename string) (*cacheNode, error) {
	const opMakeLocalCache = "make_local_cache"
	defer logLocalCacheOperationLatency(opMakeLocalCache, time.Now())
	defer trace.StartRegion(ctx, "storage.localCache.makeLocalCache").End()
	info, err := c.s.Head(ctx, key, idx)
	if err != nil {
		cacheLog.Errorw("head segment failed", "segment", key, "page", idx, "err", err)
		return nil, logErr(localCacheOperationErrorCounter, err, opMakeLocalCache)
	}

	if err = c.mustAccountCacheUsage(ctx, info.Size); err != nil {
		cacheLog.Errorw("account cache usage failed", "segment", key, "page", idx, "err", err)
		return nil, logErr(localCacheOperationErrorCounter, err, opMakeLocalCache)
	}

	cacheFilePath := localCacheFilePath(filename)
	f, err := os.Create(cacheFilePath)
	if err != nil {
		cacheLog.Errorw("create cache file failed", "segment", key, "page", idx, "err", err)
		return nil, logErr(localCacheOperationErrorCounter, err, opMakeLocalCache)
	}
	defer f.Close()

	var reader io.ReadCloser
	for i := 0; i < retryTimes; i++ {
		reader, err = c.s.Get(ctx, key, idx)
		if err == nil {
			break
		}
		time.Sleep(retryInterval)
		cacheLog.Errorw("read chunk page error, try again", "segment", key, "page", idx, "tryTime", i+1, "err", err)
	}
	if err != nil {
		return nil, logErr(localCacheOperationErrorCounter, err, opMakeLocalCache)
	}
	defer reader.Close()

	pipeOut, pipeIn := io.Pipe()
	writer := io.MultiWriter(f, pipeIn)

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		defer pipeIn.Close()
		_, copyErr := io.Copy(writer, reader)
		if copyErr != nil {
			cacheLog.Errorw("copy segment raw data error", "segment", key, "page", idx, "err", copyErr)
			errCh <- copyErr
		}
	}()

	defer pipeOut.Close()
	memCache := &memCacheNode{data: utils.NewMemoryBlock(info.Size)}
	err = c.nodeDataDecode(ctx, key^idx, pipeOut, utils.NewWriter(memCache))
	if err != nil {
		cacheLog.Errorw("node data encode error", "segment", key, "page", idx, "err", err)
		return nil, logErr(localCacheOperationErrorCounter, err, opMakeLocalCache)
	}

	err = <-errCh
	if err != nil {
		return nil, logErr(localCacheOperationErrorCounter, err, opMakeLocalCache)
	}

	node := &cacheNode{memCacheNode: memCache, ref: 1, path: cacheFilePath}
	return node, nil
}

func (c *LocalCache) openLocalCache(ctx context.Context, key, idx int64, cacheFilePath string) (*memCacheNode, error) {
	const opOpenLocalCache = "open_local_cache"
	defer logLocalCacheOperationLatency(opOpenLocalCache, time.Now())
	f, err := os.Open(cacheFilePath)
	if err != nil {
		cacheLog.Errorw("open cache file failed", "segment", key, "page", idx, "err", err)
		return nil, logErr(localCacheOperationErrorCounter, err, opOpenLocalCache)
	}
	defer f.Close()

	fInfo, err := f.Stat()
	if err != nil {
		cacheLog.Errorw("stat cache file error", "segment", key, "page", idx, "err", err)
		return nil, logErr(localCacheOperationErrorCounter, err, opOpenLocalCache)
	}

	memCache := &memCacheNode{data: utils.NewMemoryBlock(fInfo.Size())}
	err = c.nodeDataDecode(ctx, key^idx, f, utils.NewWriter(memCache))
	if err != nil {
		cacheLog.Errorw("node data encode error", "segment", key, "page", idx, "err", err)
		return nil, logErr(localCacheOperationErrorCounter, err, opOpenLocalCache)
	}

	return memCache, nil
}

func (c *LocalCache) mustAccountCacheUsage(ctx context.Context, usage int64) error {
	const opAccountCacheUsage = "account_cache_usage"
	defer logLocalCacheOperationLatency(opAccountCacheUsage, time.Now())
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
					return logErr(localCacheOperationErrorCounter, err, opAccountCacheUsage)
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

func (c *LocalCache) nodeDataEncode(ctx context.Context, segIDKey int64, in io.Reader, out io.Writer) (err error) {
	var (
		pipeOut  = out
		cryptOut io.ReadCloser
		cryptIn  io.WriteCloser
	)
	encryptCfg := globalEncryptCfg
	if c.cfg.Encryption != nil && c.cfg.Encryption.Enable {
		encryptCfg = c.cfg.Encryption
	}

	wg := sync.WaitGroup{}
	if encryptCfg != nil {
		cryptOut, cryptIn = io.Pipe()
		pipeOut = cryptIn

		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				_ = cryptOut.Close()
			}()
			if err = encrypt(ctx, segIDKey, encryptCfg.Method, encryptCfg.SecretKey, cryptOut, out); err != nil {
				_ = logErr(localCacheOperationErrorCounter, err, "node_data_encrypt")
				return
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			if cryptIn != nil {
				_ = cryptIn.Close()
			}
		}()
		if err = compress(ctx, in, pipeOut); err != nil {
			_ = logErr(localCacheOperationErrorCounter, err, "node_data_compress")
			return
		}
	}()
	wg.Wait()
	return
}

func (c *LocalCache) nodeDataDecode(ctx context.Context, segIDKey int64, in io.Reader, out io.Writer) (err error) {
	var (
		pipeIn   = in
		cryptOut io.ReadCloser
		cryptIn  io.WriteCloser
	)
	encryptCfg := globalEncryptCfg
	if c.cfg.Encryption != nil && c.cfg.Encryption.Enable {
		encryptCfg = c.cfg.Encryption
	}

	wg := sync.WaitGroup{}
	if encryptCfg != nil {
		cryptOut, cryptIn = io.Pipe()
		pipeIn = cryptOut

		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				_ = cryptIn.Close()
			}()
			if err = decrypt(ctx, segIDKey, encryptCfg.Method, encryptCfg.SecretKey, in, cryptIn); err != nil {
				_ = logErr(localCacheOperationErrorCounter, err, "node_data_decrypt")
				return
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			if cryptOut != nil {
				_ = cryptOut.Close()
			}
		}()
		if err = decompress(ctx, pipeIn, out); err != nil {
			_ = logErr(localCacheOperationErrorCounter, err, "node_data_decompress")
			return
		}
	}()

	wg.Wait()
	return
}

func (c *LocalCache) releaseCacheUsage(usage int64) {
	atomic.AddInt64(&localCacheSizeUsage, -1*usage)
}

func (c *LocalCache) cleanSpace(fileCount int) error {
	var err error
	for i := 0; i < fileCount; i++ {
		if err = cachedFile.removeNextCacheFile(); err != nil {
			return logErr(localCacheOperationErrorCounter, err, "clean_space")
		}
	}
	return nil
}

type CacheNode interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
	Size() int64
	freq() int
}

type memCacheNode struct {
	data []byte
	size int64
}

var _ CacheNode = &memCacheNode{}

func NewMemCacheNode(needBig bool) CacheNode {
	size := int64(cacheNodeBaseSize)
	if needBig {
		size *= 2
	}
	return &memCacheNode{data: utils.NewMemoryBlock(size)}
}

func (m *memCacheNode) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= m.size {
		return 0, io.EOF
	}

	n = copy(p, m.data[off:m.size])
	return
}

func (m *memCacheNode) WriteAt(p []byte, off int64) (n int, err error) {
	if off+int64(len(p)) > int64(len(m.data)) {
		m.data = utils.ExtendMemoryBlock(m.data, off+int64(len(p)))
	}
	if off+int64(len(p)) > int64(len(m.data)) {
		return 0, io.ErrShortBuffer
	}
	n = copy(m.data[off:], p)
	if off+int64(n) > m.size {
		m.size = off + int64(n)
	}
	return
}

func (m *memCacheNode) Close() error {
	utils.ReleaseMemoryBlock(m.data)
	m.data = nil
	return nil
}

func (m *memCacheNode) Size() int64 {
	return m.size
}

func (m *memCacheNode) freq() int {
	return 1
}

type cacheNode struct {
	*memCacheNode
	path      string
	openTimes int
	ref       int32
}

var _ CacheNode = &cacheNode{}

func (c *cacheNode) Open(loader func(path string) (*memCacheNode, error)) (err error) {
	c.openTimes += 1
	crtRef := atomic.LoadInt32(&c.ref)
	if crtRef == 0 {
		if atomic.CompareAndSwapInt32(&c.ref, 0, 1) {
			c.memCacheNode, err = loader(c.path)
			if err != nil {
				return err
			}
			return nil
		}
	}
	atomic.AddInt32(&c.ref, 1)
	return nil
}

func (c *cacheNode) Close() error {
	if atomic.AddInt32(&c.ref, -1) == 0 {
		cachedFile.returnNode(path.Base(c.path))
		return c.memCacheNode.Close()
	}
	return nil
}

func (c *cacheNode) freq() int {
	return c.openTimes
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
		node := &cacheNode{path: filePath}
		c.cachedNode[filename] = &nodeUsingInfo{filename: filename, node: node, updateAt: time.Now().UnixNano()}
		heap.Push(c.unusedNodeQ, c.cachedNode[filename])
		atomic.AddInt64(&localCacheSizeUsage, info.Size())
		localCachedNodeGauge.Inc()
		return nil
	})
}

type cacheBuilder func(ctx context.Context, key, idx int64, filename string) (*cacheNode, error)
type cacheOpener func(ctx context.Context, key, idx int64, cacheFilePath string) (*memCacheNode, error)

func (c *cachedFileMapper) fetchCacheNode(ctx context.Context, key int64, idx int64, builder cacheBuilder, opener cacheOpener) (CacheNode, error) {
	const opFetchCacheNode = "fetch_cache_node"
	defer logLocalCacheOperationLatency(opFetchCacheNode, time.Now())
	localCacheFetchingGauge.Inc()
	defer localCacheFetchingGauge.Dec()

	fileName := c.filename(key, idx)
	openHandler := func(path string) (*memCacheNode, error) {
		return opener(ctx, key, idx, path)
	}

	c.mux.Lock()
	vNode, ok := c.cachedNode[fileName]
	if ok {
		if vNode.index != -1 && len(*c.unusedNodeQ) > 0 {
			heap.Remove(c.unusedNodeQ, vNode.index)
			vNode.index = -1
		}
		c.mux.Unlock()
		return vNode.node, logErr(localCacheOperationErrorCounter, vNode.node.Open(openHandler), opFetchCacheNode)
	}

	_, stillFetching := c.pending[fileName]
	if stillFetching {
		for stillFetching {
			c.cond.Wait()
			_, stillFetching = c.pending[fileName]
		}
		vNode = c.cachedNode[fileName]
		c.mux.Unlock()
		if vNode == nil {
			return nil, fmt.Errorf("cached node not found")
		}
		return vNode.node, logErr(localCacheOperationErrorCounter, vNode.node.Open(openHandler), opFetchCacheNode)
	} else {
		// try fetch
		c.pending[fileName] = struct{}{}
	}
	c.mux.Unlock()

	node, err := builder(ctx, key, idx, fileName)
	if err != nil {
		c.mux.Lock()
		delete(c.pending, fileName)
		c.mux.Unlock()
		c.cond.Broadcast()
		return nil, logErr(localCacheOperationErrorCounter, err, opFetchCacheNode)
	}

	localCachedNodeGauge.Inc()
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
	localCachedNodeGauge.Dec()
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
