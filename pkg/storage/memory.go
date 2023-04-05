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
	"strconv"
	"strings"
	"sync"
)

const (
	MemoryMeta    = "memory"
	MemoryStorage = "memory"
)

type memoryStorage struct {
	storageID string
	storage   map[string]chunk
	mux       sync.Mutex
}

func (m *memoryStorage) ID() string {
	return m.storageID
}

func (m *memoryStorage) Get(ctx context.Context, key int64, idx, offset int64, dest []byte) (int64, error) {
	defer utils.TraceRegion(ctx, "memory.get")()
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		return 0, err
	}
	return int64(copy(dest, ck.data[offset:])), nil
}

func (m *memoryStorage) Put(ctx context.Context, key int64, idx, offset int64, data []byte) error {
	defer utils.TraceRegion(ctx, "memory.put")()
	cKey := m.chunkKey(key, idx)
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		ck = &chunk{data: make([]byte, cacheNodeSize)}
	}

	copy(ck.data[offset:], data)
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

func (m *memoryStorage) getChunk(ctx context.Context, key string) (*chunk, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	chunk, ok := m.storage[key]
	if !ok {
		return nil, types.ErrNotFound
	}
	return &chunk, nil
}

func (m *memoryStorage) saveChunk(ctx context.Context, key string, chunk chunk) error {
	m.mux.Lock()
	m.storage[key] = chunk
	m.mux.Unlock()
	return nil
}

func newMemoryStorage(storageID string) Storage {
	return &memoryStorage{
		storageID: storageID,
		storage:   map[string]chunk{},
	}
}

type chunk struct {
	data []byte
}
