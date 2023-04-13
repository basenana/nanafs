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
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

const (
	LocalStorage         = "local"
	MemoryStorage        = "memory"
	defaultLocalDirMode  = 0755
	defaultLocalFileMode = 0644
)

type local struct {
	sid    string
	dir    string
	logger *zap.SugaredLogger
}

var _ Storage = &local{}

func (l *local) ID() string {
	return l.sid
}

func (l *local) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	defer utils.TraceRegion(ctx, "local.get")()
	file, err := l.openLocalFile(l.key2LocalPath(key, idx), os.O_RDWR)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (l *local) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	defer utils.TraceRegion(ctx, "local.put")()
	file, err := l.openLocalFile(l.key2LocalPath(key, idx), os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, dataReader)
	if err != nil {
		l.logger.Errorw("copy file failed", "key", key, "err", err.Error())
		return err
	}
	return err
}

func (l *local) Delete(ctx context.Context, key int64) error {
	defer utils.TraceRegion(ctx, "local.delete")()
	p := path.Join(l.dir, fmt.Sprintf("%d", key))
	_, err := os.Stat(p)
	if err != nil && !os.IsNotExist(err) {
		l.logger.Errorw("delete file failed", "key", key, "err", err.Error())
		return err
	}

	return os.RemoveAll(p)
}

func (l *local) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer utils.TraceRegion(ctx, "local.head")()
	info, err := os.Stat(l.key2LocalPath(key, idx))
	if err != nil && !os.IsNotExist(err) {
		return Info{}, err
	}
	if os.IsNotExist(err) {
		return Info{}, types.ErrNotFound
	}
	return Info{
		Key:  info.Name(),
		Size: info.Size(),
	}, nil
}

func (l *local) openLocalFile(path string, flag int) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if os.IsNotExist(err) && flag&os.O_CREATE == 0 {
		return nil, types.ErrNotFound
	}

	if info != nil && info.IsDir() {
		return nil, types.ErrIsGroup
	}

	f, err := os.OpenFile(path, flag, defaultLocalFileMode)
	if err != nil {
		l.logger.Errorw("open file failed", "path", path, "err", err.Error())
	}
	return f, err
}

func (l *local) key2LocalPath(key int64, idx int64) string {
	dataPath := path.Join(l.dir, fmt.Sprintf("%d", key))
	if _, err := os.Stat(dataPath); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(dataPath, defaultLocalDirMode)
			if err != nil {
				l.logger.Errorw("data path mkdir failed", "path", dataPath)
			}
		}
	}
	return path.Join(l.dir, fmt.Sprintf("%d/%d", key, idx))
}

func newLocalStorage(sid, dir string) (Storage, error) {
	if err := utils.Mkdir(dir); err != nil {
		return nil, fmt.Errorf("init local data dir failed: %s", err)
	}
	return &local{
		sid:    sid,
		dir:    dir,
		logger: logger.NewLogger("localStorage"),
	}, nil
}

type memoryStorage struct {
	storageID string
	storage   map[string]memChunkData
	mux       sync.Mutex
}

func (m *memoryStorage) ID() string {
	return m.storageID
}

func (m *memoryStorage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	defer utils.TraceRegion(ctx, "memory.get")()
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		return nil, err
	}
	ck.vernier = 0
	return ck, nil
}

func (m *memoryStorage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	defer utils.TraceRegion(ctx, "memory.put")()
	cKey := m.chunkKey(key, idx)
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		ck = &memChunkData{data: make([]byte, cacheNodeSize)}
	}

	if _, err = io.Copy(ck, dataReader); err != nil {
		return err
	}
	return m.saveChunk(ctx, cKey, *ck)
}

func (m *memoryStorage) Delete(ctx context.Context, key int64) error {
	defer utils.TraceRegion(ctx, "memory.delete")()
	m.mux.Lock()
	defer m.mux.Unlock()
	for k := range m.storage {
		if strings.HasPrefix(k, strconv.FormatInt(key, 10)) {
			delete(m.storage, k)
		}
	}
	return nil
}

func (m *memoryStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer utils.TraceRegion(ctx, "memory.head")()
	result := Info{Key: strconv.FormatInt(key, 10)}
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		return result, err
	}

	result.Size = int64(len(ck.data))
	return result, nil
}

func (m *memoryStorage) chunkKey(key int64, idx int64) string {
	return fmt.Sprintf("%d_%d", key, idx)
}

func (m *memoryStorage) getChunk(ctx context.Context, key string) (*memChunkData, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	chunk, ok := m.storage[key]
	if !ok {
		return nil, types.ErrNotFound
	}
	return &chunk, nil
}

func (m *memoryStorage) saveChunk(ctx context.Context, key string, chunk memChunkData) error {
	m.mux.Lock()
	m.storage[key] = chunk
	m.mux.Unlock()
	return nil
}

func newMemoryStorage(storageID string) Storage {
	return &memoryStorage{
		storageID: storageID,
		storage:   map[string]memChunkData{},
	}
}

type memChunkData struct {
	vernier int
	data    []byte
}

func (c *memChunkData) Write(p []byte) (n int, err error) {
	if c.vernier == len(c.data) {
		return 0, io.ErrShortBuffer
	}

	n = copy(c.data[c.vernier:], p)
	c.vernier += n
	return n, nil
}

func (c *memChunkData) Read(p []byte) (n int, err error) {
	n = copy(p, c.data[c.vernier:])
	c.vernier += n
	if c.vernier == len(c.data) {
		return n, io.EOF
	}
	return n, nil
}

func (c *memChunkData) Close() error {
	return nil
}
