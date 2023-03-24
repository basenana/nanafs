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
	"encoding/json"
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

type memoryMetaStore struct {
	objects     map[int64]*types.Object
	chunks      map[int64][]types.ChunkSeg
	inodeCount  uint64
	nextChunkID int64
	mux         sync.Mutex
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

func (m *memoryMetaStore) PluginRecorder(plugin types.PlugScope) PluginRecorder {
	return &memoryPluginRecorder{
		plugin:      plugin,
		data:        make(map[string][]byte),
		recordGroup: make(map[string]string),
		groups:      make(map[string]map[string]struct{}),
	}
}

func (m *memoryMetaStore) NextSegmentID(ctx context.Context) (int64, error) {
	m.mux.Lock()
	n := m.nextChunkID
	m.nextChunkID += 1
	m.mux.Unlock()
	return n, nil
}

func (m *memoryMetaStore) ListSegments(ctx context.Context, oid, chunkID int64) ([]types.ChunkSeg, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.chunks[chunkID], nil
}

func (m *memoryMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg, obj *types.Object) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	chunks := m.chunks[seg.ChunkID]
	chunks = append(chunks, seg)
	m.chunks[seg.ChunkID] = chunks
	obj = m.objects[obj.ID]
	if obj != nil && seg.Off+seg.Len > obj.Size {
		obj.Size = seg.Off + seg.Len
		m.objects[obj.ID] = obj
	}
	return nil
}

type memoryPluginRecorder struct {
	plugin      types.PlugScope
	data        map[string][]byte
	recordGroup map[string]string
	groups      map[string]map[string]struct{}
	mux         sync.Mutex
}

func (m *memoryPluginRecorder) GetRecord(ctx context.Context, rid string, record interface{}) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	rawData, ok := m.data[rid]
	if !ok {
		return types.ErrNotFound
	}
	return json.Unmarshal(rawData, record)
}

func (m *memoryPluginRecorder) ListRecords(ctx context.Context, groupId string) ([]string, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	groups, ok := m.groups[groupId]
	if !ok {
		return []string{}, nil
	}
	result := make([]string, 0, len(groups))
	for rid := range groups {
		result = append(result, rid)
	}
	return result, nil
}

func (m *memoryPluginRecorder) SaveRecord(ctx context.Context, groupId, rid string, record interface{}) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	rawData, err := json.Marshal(record)
	if err != nil {
		return err
	}
	m.data[rid] = rawData
	oldGroupId, ok := m.recordGroup[rid]
	if !ok {
		m.recordGroup[rid] = groupId
		groups, inited := m.groups[groupId]
		if !inited {
			groups = map[string]struct{}{}
		}
		groups[rid] = struct{}{}
		m.groups[groupId] = groups
		return nil
	}

	if groupId != oldGroupId {
		m.recordGroup[rid] = groupId
		delete(m.groups[oldGroupId], rid)

		groups, inited := m.groups[groupId]
		if !inited {
			groups = map[string]struct{}{}
		}
		groups[rid] = struct{}{}
		m.groups[groupId] = groups
	}
	return nil
}

func (m *memoryPluginRecorder) DeleteRecord(ctx context.Context, rid string) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	_, ok := m.data[rid]
	if !ok {
		return types.ErrNotFound
	}
	delete(m.data, rid)

	groupId, ok := m.recordGroup[rid]
	if !ok {
		return nil
	}
	delete(m.recordGroup, rid)
	delete(m.groups[groupId], rid)
	return nil
}

func newMemoryMetaStore() Meta {
	return &memoryMetaStore{objects: map[int64]*types.Object{}}
}

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
		ck = &chunk{data: make([]byte, 1<<22)}
	}

	var (
		n   int
		buf = make([]byte, 1024)
	)

	for {
		n = copy(buf, data)
		if n == 0 {
			break
		}
		offset += int64(copy(ck.data[offset:], buf[:n]))
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
