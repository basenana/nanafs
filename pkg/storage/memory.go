package storage

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/object"
	"sync"
)

const (
	metaStoreMemory = "memory"
)

type memoryMetaStore struct {
	objects map[string]*dentry.Entry
	mux     sync.Mutex
}

var _ MetaStore = &memoryMetaStore{}

func (m *memoryMetaStore) GetEntry(ctx context.Context, id string) (*dentry.Entry, error) {
	m.mux.Lock()
	result, ok := m.objects[id]
	if !ok {
		m.mux.Unlock()
		return nil, object.ErrNotFound
	}
	m.mux.Unlock()
	return result, nil
}

func (m *memoryMetaStore) ListEntries(ctx context.Context, filter Filter) ([]*dentry.Entry, error) {
	m.mux.Lock()
	result := make([]*dentry.Entry, 0)
	for oID, obj := range m.objects {
		if isObjectFiltered(obj, filter) {
			result = append(result, m.objects[oID])
		}
	}
	m.mux.Unlock()
	return result, nil
}

func (m *memoryMetaStore) SaveEntry(ctx context.Context, obj *dentry.Entry) error {
	m.mux.Lock()
	m.objects[obj.GetObjectMeta().ID] = obj
	m.mux.Unlock()
	return nil
}

func (m *memoryMetaStore) DestroyEntry(ctx context.Context, obj *dentry.Entry) error {
	m.mux.Lock()
	_, ok := m.objects[obj.GetObjectMeta().ID]
	if !ok {
		m.mux.Unlock()
		return object.ErrNotFound
	}
	delete(m.objects, obj.GetObjectMeta().ID)
	m.mux.Unlock()
	return nil
}

func (m *memoryMetaStore) ListChildren(ctx context.Context, id string) (Iterator, error) {
	children, err := m.ListEntries(ctx, Filter{ParentID: id})
	if err != nil {
		return nil, err
	}

	return &iterator{objects: children}, nil
}

func (m *memoryMetaStore) ChangeParent(ctx context.Context, old *dentry.Entry, parent *dentry.Entry) error {
	return nil
}

func newMemoryMetaStore() MetaStore {
	return &memoryMetaStore{
		objects: map[string]*dentry.Entry{},
	}
}
