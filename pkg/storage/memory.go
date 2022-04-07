package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"io"
	"sync"
)

const (
	metaStoreMemory = "memory"
	memoryStorageID = "memory"
)

type memoryMetaStore struct {
	objects    map[string]*types.Object
	content    map[string][]byte
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

func (m *memoryMetaStore) SaveContent(ctx context.Context, obj *types.Object, cType, version string, content interface{}) error {
	raw, err := json.Marshal(content)
	if err != nil {
		return err
	}
	m.mux.Lock()
	m.content[m.contentKey(obj, cType, version)] = raw
	m.mux.Unlock()
	return nil
}

func (m *memoryMetaStore) LoadContent(ctx context.Context, obj *types.Object, cType, version string, content interface{}) error {
	m.mux.Lock()
	raw, ok := m.content[m.contentKey(obj, cType, version)]
	m.mux.Unlock()
	if !ok {
		return types.ErrNotFound
	}
	return json.Unmarshal(raw, content)
}

func (m *memoryMetaStore) DeleteContent(ctx context.Context, obj *types.Object, cType, version string) error {
	m.mux.Lock()
	cKey := m.contentKey(obj, cType, version)
	_, ok := m.content[cKey]
	if !ok {
		m.mux.Unlock()
		return types.ErrNotFound
	}
	delete(m.content, cKey)
	m.mux.Unlock()
	return nil
}

func (m *memoryMetaStore) contentKey(obj *types.Object, cType, version string) string {
	return fmt.Sprintf("%s_%s_%s", obj.ID, cType, version)
}

func newMemoryMetaStore() MetaStore {
	return &memoryMetaStore{
		objects: map[string]*types.Object{},
	}
}

type memoryStorage struct {
	storage map[string][]chunk
	mux     sync.Mutex
}

func (m *memoryStorage) ID() string {
	return memoryStorageID
}

func (m *memoryStorage) Get(ctx context.Context, key string, idx, off, limit int64) (io.ReadCloser, error) {
	chunks, err := m.getChunks(ctx, key)
	if err != nil {
		return nil, err
	}

	if idx > int64(len(chunks)) {
		return nil, fmt.Errorf("out of range")
	}

	c := chunks[idx]
	if int64(len(c.data)) < limit {
		limit = int64(len(c.data))
	}
	return utils.NewDateReader(bytes.NewReader(c.data[off:limit])), nil
}

func (m *memoryStorage) Put(ctx context.Context, key string, in io.Reader, idx, off int64) error {
	chunks, err := m.getChunks(ctx, key)
	if err != nil {
		chunks = make([]chunk, 0)
	}

	nowIdx := len(chunks)
	if idx+1 > int64(nowIdx) {
		sub := int(idx + 1 - int64(nowIdx))
		for sub > 0 {
			chunks = append(chunks, chunk{data: []byte{}})
			sub--
		}
	}

	var (
		c   = chunks[idx]
		buf = make([]byte, defaultChunkSize)
	)
	for {
		n, err := in.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		canRead := int(defaultChunkSize - off)
		if n > canRead {
			return fmt.Errorf("out of range")
		}

		c.data = append(c.data[:off], buf[:n]...)
	}
	chunks[idx] = c
	return m.saveChunks(ctx, key, chunks)
}

func (m *memoryStorage) Delete(ctx context.Context, key string) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	_, ok := m.storage[key]
	if !ok {
		return types.ErrNotFound
	}
	delete(m.storage, key)
	return nil
}

func (m *memoryStorage) Fsync(ctx context.Context, key string) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	return nil
}

func (m *memoryStorage) Head(ctx context.Context, key string) (Info, error) {
	result := Info{Key: key}
	chunks, err := m.getChunks(ctx, key)
	if err != nil {
		return result, err
	}

	for _, c := range chunks {
		result.Size += int64(len(c.data))
	}
	return result, nil
}

func (m *memoryStorage) getChunks(ctx context.Context, key string) ([]chunk, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	chunks, ok := m.storage[key]
	if !ok {
		return nil, types.ErrNotFound
	}
	return chunks, nil
}

func (m *memoryStorage) saveChunks(ctx context.Context, key string, chunks []chunk) error {
	m.mux.Lock()
	m.storage[key] = chunks
	m.mux.Unlock()
	return nil
}

func newMemoryStorage() Storage {
	return &memoryStorage{
		storage: map[string][]chunk{},
	}
}

type chunk struct {
	data   []byte
	offset int64
}
