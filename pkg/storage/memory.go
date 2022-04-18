package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"io"
	"strings"
	"sync"
)

const (
	MemoryMeta    = "memory"
	MemoryStorage = "memory"
)

type memoryMetaStore struct {
	objects    map[string]*types.Object
	inodeCount uint64
	mux        sync.Mutex
}

var _ MetaStore = &memoryMetaStore{}

func (m *memoryMetaStore) GetObject(ctx context.Context, id string) (*types.Object, error) {
	m.mux.Lock()
	result, ok := m.objects[id]
	if !ok {
		m.mux.Unlock()
		return nil, types.ErrNotFound
	}
	m.mux.Unlock()
	return result, nil
}

func (m *memoryMetaStore) ListObjects(ctx context.Context, filter Filter) ([]*types.Object, error) {
	m.mux.Lock()
	result := make([]*types.Object, 0)
	for oID, obj := range m.objects {
		if isObjectFiltered(obj, filter) {
			result = append(result, m.objects[oID])
		}
	}
	m.mux.Unlock()
	return result, nil
}

func (m *memoryMetaStore) SaveObject(ctx context.Context, obj *types.Object) error {
	m.mux.Lock()
	if obj.Inode == 0 {
		m.inodeCount++
		obj.Inode = m.inodeCount
	}
	m.objects[obj.ID] = obj
	m.mux.Unlock()
	return nil
}

func (m *memoryMetaStore) DestroyObject(ctx context.Context, obj *types.Object) error {
	m.mux.Lock()
	_, ok := m.objects[obj.ID]
	if !ok {
		m.mux.Unlock()
		return types.ErrNotFound
	}
	delete(m.objects, obj.ID)
	m.mux.Unlock()
	return nil
}

func (m *memoryMetaStore) ListChildren(ctx context.Context, id string) (Iterator, error) {
	children, err := m.ListObjects(ctx, Filter{ParentID: id})
	if err != nil {
		return nil, err
	}

	return &iterator{objects: children}, nil
}

func (m *memoryMetaStore) ChangeParent(ctx context.Context, old *types.Object, parent *types.Object) error {
	old.ParentID = parent.ID
	return m.SaveObject(ctx, old)
}

func newMemoryMetaStore() MetaStore {
	return &memoryMetaStore{
		objects: map[string]*types.Object{},
	}
}

type memoryStorage struct {
	storage map[string]chunk
	mux     sync.Mutex
}

func (m *memoryStorage) ID() string {
	return MemoryStorage
}

func (m *memoryStorage) Get(ctx context.Context, key string, idx, offset int64) (io.ReadCloser, error) {
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		return nil, err
	}
	return utils.NewDateReader(bytes.NewReader(ck.data)), nil
}

func (m *memoryStorage) Put(ctx context.Context, key string, idx, offset int64, in io.Reader) error {
	cKey := m.chunkKey(key, idx)
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		ck = &chunk{}
	}

	var (
		buf = make([]byte, 1024)
		n   int
	)

	for {
		n, err = in.Read(buf)
		if err == io.EOF {
			break
		}
		ck.data = append(ck.data, buf[:n]...)
	}

	return m.saveChunk(ctx, cKey, *ck)
}

func (m *memoryStorage) Delete(ctx context.Context, key string) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	for k := range m.storage {
		if strings.HasPrefix(k, key) {
			delete(m.storage, key)
		}
	}
	return nil
}

func (m *memoryStorage) Head(ctx context.Context, key string, idx int64) (Info, error) {
	result := Info{Key: key}
	ck, err := m.getChunk(ctx, m.chunkKey(key, idx))
	if err != nil {
		return result, err
	}

	result.Size = int64(len(ck.data))
	return result, nil
}

func (m *memoryStorage) chunkKey(key string, idx int64) string {
	return fmt.Sprintf("%s_%d", key, idx)
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

func newMemoryStorage() Storage {
	return &memoryStorage{
		storage: map[string]chunk{},
	}
}

type chunk struct {
	data []byte
}
