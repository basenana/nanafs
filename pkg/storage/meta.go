package storage

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"sync"
)

type Meta interface {
	ObjectStore
}

type ObjectStore interface {
	GetObject(ctx context.Context, id int64) (*types.Object, error)
	ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error)
	SaveObject(ctx context.Context, parent, obj *types.Object) error
	DestroyObject(ctx context.Context, src, parent, obj *types.Object) error

	ListChildren(ctx context.Context, obj *types.Object) (Iterator, error)
	MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error
	ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error
}

type PluginStore interface {
	GetRecord(ctx context.Context, rid string) error
	ListRecords(ctx context.Context, groupId string, records interface{}) error
	SaveRecord(ctx context.Context, rid string, record interface{}) error
	DeleteRecord(ctx context.Context, rid string) error
}

const (
	MemoryMeta = "memory"
)

func NewMetaStorage(metaType string, meta config.Meta) (Meta, error) {
	switch metaType {
	case MemoryMeta:
		return newMemoryMetaStore(), nil
	case SqliteMeta:
		return newSqliteMetaStore(meta)
	default:
		return nil, fmt.Errorf("unknow meta store type: %s", metaType)
	}
}

type PluginRecorder interface {
	NewRecord(ctx context.Context, recordID, groupID string, data interface{}) error
	GetRecord(ctx context.Context, recordID string, data interface{}) error
	ListRecord(ctx context.Context, groupID string, data interface{}) error
	ListGroup(ctx context.Context) ([]string, error)
}

func NewPluginRecorder(pluginName string) (PluginRecorder, error) {
	return nil, nil
}

type memoryMetaStore struct {
	objects    map[int64]*types.Object
	inodeCount uint64
	mux        sync.Mutex
}

var _ Meta = &memoryMetaStore{}

func (m *memoryMetaStore) GetObject(ctx context.Context, id int64) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "memory.getobject")()
	m.mux.Lock()
	result, ok := m.objects[id]
	if !ok {
		m.mux.Unlock()
		return nil, types.ErrNotFound
	}
	m.mux.Unlock()
	return result, nil
}

func (m *memoryMetaStore) ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "memory.listobject")()
	m.mux.Lock()
	result := make([]*types.Object, 0)
	for oID, obj := range m.objects {
		if types.IsObjectFiltered(obj, filter) {
			result = append(result, m.objects[oID])
		}
	}
	m.mux.Unlock()
	return result, nil
}

func (m *memoryMetaStore) SaveObject(ctx context.Context, parent, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "memory.saveobject")()
	m.mux.Lock()
	if obj.Inode == 0 {
		m.inodeCount++
		obj.Inode = m.inodeCount
	}
	m.objects[obj.ID] = obj
	if parent != nil {
		m.objects[parent.ID] = parent
	}
	m.mux.Unlock()
	return nil
}

func (m *memoryMetaStore) DestroyObject(ctx context.Context, src, parent, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "memory.destroyobject")()
	m.mux.Lock()
	_, ok := m.objects[obj.ID]
	if !ok {
		m.mux.Unlock()
		return types.ErrNotFound
	}
	m.objects[parent.ID] = parent
	if src != nil {
		if src.RefCount > 0 {
			m.objects[src.ID] = src
		} else {
			delete(m.objects, src.ID)
		}
	}
	if !obj.IsGroup() && obj.RefCount > 0 {
		m.objects[obj.ID] = obj
	} else {
		delete(m.objects, obj.ID)
	}
	m.mux.Unlock()
	return nil
}

func (m *memoryMetaStore) ListChildren(ctx context.Context, obj *types.Object) (Iterator, error) {
	defer utils.TraceRegion(ctx, "memory.listchildren")()
	f := types.Filter{ParentID: obj.ID}
	if obj.Labels.Get(types.KindKey) != nil && obj.Labels.Get(types.KindKey).Value != "" {
		f.Kind = types.Kind(obj.Labels.Get(types.KindKey).Value)
		f.Label = types.LabelMatch{Include: []types.Label{{
			Key:   types.VersionKey,
			Value: obj.Labels.Get(types.VersionKey).Value,
		}}}
	}
	children, err := m.ListObjects(ctx, types.Filter{ParentID: obj.ID})
	if err != nil {
		return nil, err
	}

	return &iterator{objects: children}, nil
}

func (m *memoryMetaStore) ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	defer utils.TraceRegion(ctx, "memory.changeparent")()
	m.mux.Lock()
	obj.ParentID = dstParent.ID
	m.objects[srcParent.ID] = srcParent
	m.objects[dstParent.ID] = dstParent
	m.objects[obj.ID] = obj

	m.mux.Unlock()
	return nil
}
func (m *memoryMetaStore) MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	defer utils.TraceRegion(ctx, "memory.mirrorobject")()
	m.mux.Lock()
	m.objects[srcObj.ID] = srcObj
	m.objects[dstParent.ID] = dstParent
	m.objects[object.ID] = object
	m.mux.Unlock()
	return nil
}
