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

package dentry

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"path"
	"runtime/trace"
	"strings"
	"time"
)

type Group interface {
	FindEntry(ctx context.Context, name string) (*types.Metadata, error)
	CreateEntry(ctx context.Context, attr EntryAttr) (*types.Metadata, error)
	UpdateEntry(ctx context.Context, entryID int64, patch *types.Metadata) error
	RemoveEntry(ctx context.Context, entryID int64) error
	ListChildren(ctx context.Context) ([]*types.Metadata, error)
}

type emptyGroup struct{}

var _ Group = emptyGroup{}

func (e emptyGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	return nil, types.ErrNotFound
}

func (e emptyGroup) CreateEntry(ctx context.Context, attr EntryAttr) (*types.Metadata, error) {
	return nil, types.ErrNoAccess
}

func (e emptyGroup) UpdateEntry(ctx context.Context, entryID int64, patch *types.Metadata) error {
	return types.ErrNoAccess
}

func (e emptyGroup) RemoveEntry(ctx context.Context, enId int64) error {
	return types.ErrNoAccess
}

func (e emptyGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	return make([]*types.Metadata, 0), nil
}

type stdGroup struct {
	entryID    int64
	name       string
	store      metastore.ObjectStore
	cacheStore *metaCache
}

var _ Group = &stdGroup{}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.FindEntry").End()
	entryLifecycleLock.RLock()
	defer entryLifecycleLock.RUnlock()
	objects, err := g.store.ListObjects(ctx, types.Filter{Name: name, ParentID: g.entryID})
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, types.ErrNotFound
	}
	if len(objects) > 1 {
		objNames := make([]string, len(objects))
		for i, obj := range objects {
			objNames[i] = fmt.Sprintf(`{"id":%d,"name":"%s"}`, obj.ID, obj.Name)
		}
		logger.NewLogger("stdGroup").Warnf("lookup group %s with name %s, got objects: %s", g.name, name, strings.Join(objNames, ","))
	}
	obj := objects[0]
	return &obj.Metadata, nil
}

func (g *stdGroup) CreateEntry(ctx context.Context, attr EntryAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.CreateEntry").End()
	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()
	objects, err := g.store.ListObjects(ctx, types.Filter{Name: attr.Name, ParentID: g.entryID})
	if err != nil {
		return nil, err
	}
	if len(objects) > 0 {
		return nil, types.ErrIsExist
	}

	group, err := g.cacheStore.getEntry(ctx, g.entryID)
	if err != nil {
		return nil, err
	}

	obj, err := types.InitNewObject(group, types.ObjectAttr{
		Name:   attr.Name,
		Kind:   attr.Kind,
		Access: attr.Access,
	})
	if err != nil {
		return nil, err
	}
	obj.Dev = attr.Dev
	if obj.Kind == types.ExternalGroupKind {
		obj.Storage = externalStorage
		if attr.PlugScope.Parameters == nil {
			attr.PlugScope.Parameters = map[string]string{}
		}
		obj.PlugScope = attr.PlugScope
		obj.PlugScope.Parameters[types.PlugScopeEntryName] = attr.Name
		obj.PlugScope.Parameters[types.PlugScopeEntryPath] = "/"
	}
	if obj.Kind == types.SmartGroupKind {
		obj.GroupFilter = attr.GroupFilter
	}

	group.ModifiedAt = time.Now()
	group.ChangedAt = time.Now()
	if obj.IsGroup() {
		group.RefCount += 1
	}
	if err = g.cacheStore.createEntry(ctx, obj, group); err != nil {
		return nil, err
	}
	return &obj.Metadata, nil
}

func (g *stdGroup) UpdateEntry(ctx context.Context, entryId int64, patch *types.Metadata) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.UpdateEntry").End()
	patch.ID = entryId
	if err := g.cacheStore.updateEntries(ctx, patch); err != nil {
		return err
	}
	return nil
}

func (g *stdGroup) RemoveEntry(ctx context.Context, entryId int64) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.RemoveEntry").End()
	group, err := g.cacheStore.getEntry(ctx, g.entryID)
	if err != nil {
		return err
	}

	entry, err := g.cacheStore.getEntry(ctx, entryId)
	if err != nil {
		return err
	}

	if entry.RefID != 0 {
		return types.ErrUnsupported
	}
	if types.IsGroup(entry.Kind) && entry.RefCount > 2 {
		return types.ErrNotEmpty
	}
	if !types.IsGroup(entry.Kind) && entry.RefCount > 1 {
		return types.ErrUnsupported
	}

	entry.ParentID = 0
	if !types.IsGroup(entry.Kind) && entry.RefCount > 0 {
		entry.RefCount -= 1
	}
	group.ModifiedAt = time.Now()
	if types.IsGroup(entry.Kind) {
		group.RefCount -= 1
	}
	if err := g.cacheStore.updateEntries(ctx, entry, group); err != nil {
		return err
	}
	return nil
}

func (g *stdGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.ListChildren").End()
	it, err := g.store.ListChildren(ctx, g.entryID)
	if err != nil {
		return nil, err
	}
	result := make([]*types.Metadata, 0)
	for it.HasNext() {
		next := it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, &next.Metadata)
	}
	return result, nil
}

type dynamicGroup struct {
	*stdGroup
}

type extGroup struct {
	mgr      Manager
	stdGroup *stdGroup
	mirror   plugin.MirrorPlugin
}

func (e *extGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	mirrorEn, err := e.mirror.FindEntry(ctx, name)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}

	en, err := e.stdGroup.FindEntry(ctx, name)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	return e.syncEntry(ctx, mirrorEn, en)
}

func (e *extGroup) CreateEntry(ctx context.Context, attr EntryAttr) (*types.Metadata, error) {
	mirrorEn, err := e.mirror.CreateEntry(ctx, stub.EntryAttr{
		Name: attr.Name,
		Kind: attr.Kind,
	})
	if err != nil {
		return nil, err
	}
	en, err := e.stdGroup.FindEntry(ctx, attr.Name)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	return e.syncEntry(ctx, mirrorEn, en)
}

func (e *extGroup) UpdateEntry(ctx context.Context, entryId int64, patch *types.Metadata) error {
	group, err := e.stdGroup.cacheStore.getEntry(ctx, e.stdGroup.entryID)
	if err != nil {
		return err
	}
	mirrorEn, err := e.mirror.FindEntry(ctx, group.Name)
	if err != nil {
		return err
	}

	mirrorEn.Size = patch.Size

	// query old and write back
	entry, err := e.stdGroup.cacheStore.getEntry(ctx, entryId)
	if err != nil {
		return err
	}

	err = e.mirror.UpdateEntry(ctx, mirrorEn)
	if err != nil {
		return err
	}

	_, err = e.syncEntry(ctx, mirrorEn, entry)
	return err
}

func (e *extGroup) RemoveEntry(ctx context.Context, entryId int64) error {
	entry, err := e.stdGroup.cacheStore.getEntry(ctx, entryId)
	if err != nil {
		return err
	}
	mirrorEn, err := e.mirror.FindEntry(ctx, entry.Name)
	if err != nil {
		return err
	}

	err = e.mirror.RemoveEntry(ctx, mirrorEn)
	if err != nil {
		return err
	}

	objects, err := e.stdGroup.store.ListObjects(ctx, types.Filter{Name: entry.Name, ParentID: entry.ID})
	if err != nil {
		return err
	}
	if len(objects) > 0 {
		obj := objects[0]
		_, err = e.syncEntry(ctx, nil, &obj.Metadata)
		if err == types.ErrNotFound {
			return nil
		}
		return err
	}
	return nil
}

func (e *extGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	recordChild, err := e.stdGroup.ListChildren(ctx)
	if err != nil {
		return nil, err
	}
	actualChild, err := e.mirror.ListChildren(ctx)
	if err != nil {
		return nil, err
	}

	recordChildMap := make(map[string]*types.Metadata)
	actualChildMap := make(map[string]*stub.Entry)
	for i := range recordChild {
		recordChildMap[recordChild[i].Name] = recordChild[i]
	}
	for i := range actualChild {
		actualChildMap[actualChild[i].Name] = actualChild[i]
	}

	result := make([]*types.Metadata, 0, len(actualChild))
	for k := range actualChildMap {
		en, err := e.syncEntry(ctx, actualChildMap[k], recordChildMap[k])
		if err != nil && err != types.ErrNotFound {
			return nil, err
		}
		if en != nil {
			result = append(result, en)
		}
		delete(actualChildMap, k)
		delete(recordChildMap, k)
	}

	for k := range recordChildMap {
		_, err = e.syncEntry(ctx, nil, recordChildMap[k])
		if err != nil && err != types.ErrNotFound {
			return nil, err
		}
	}
	return result, nil
}

func (e *extGroup) syncEntry(ctx context.Context, mirrored *stub.Entry, crt *types.Metadata) (en *types.Metadata, err error) {
	grp, err := e.stdGroup.cacheStore.getEntry(ctx, e.stdGroup.entryID)
	if err != nil {
		return nil, err
	}
	grpEd, err := e.mgr.GetEntryExtendData(ctx, e.stdGroup.entryID)
	if err != nil {
		return
	}
	if grpEd.PlugScope == nil {
		err = fmt.Errorf("not ext group")
		return
	}

	if mirrored != nil && crt != nil && mirrored.IsGroup != types.IsGroup(crt.Kind) {
		// FIXME
		return nil, fmt.Errorf("entry and mirrored entry incorrect")
	}

	if mirrored == nil {
		err = types.ErrNotFound
		if crt != nil {
			// clean
			_ = e.stdGroup.store.DestroyObject(ctx, &types.Object{Metadata: *grp}, &types.Object{Metadata: *crt})
			e.stdGroup.cacheStore.delEntryCache(crt.ID)
			e.stdGroup.cacheStore.delEntryCache(grp.ID)
		}
		return nil, err
	}

	if crt == nil {
		// create mirror record
		var obj *types.Object
		obj, err = types.InitNewObject(grp, types.ObjectAttr{
			Name:   mirrored.Name,
			Kind:   mirrored.Kind,
			Access: grp.Access,
		})
		if err != nil {
			return nil, err
		}

		obj.Storage = externalStorage
		obj.PlugScope = &types.PlugScope{
			PluginName: grpEd.PlugScope.PluginName,
			Version:    grpEd.PlugScope.Version,
			PluginType: grpEd.PlugScope.PluginType,
			Parameters: map[string]string{},
		}
		if mirrored.Parameters != nil {
			for k, v := range mirrored.Parameters {
				obj.PlugScope.Parameters[k] = v
			}
		}
		for k, v := range grpEd.PlugScope.Parameters {
			obj.PlugScope.Parameters[k] = v
		}
		obj.PlugScope.Parameters[types.PlugScopeEntryName] = mirrored.Name
		obj.PlugScope.Parameters[types.PlugScopeEntryPath] = path.Join(grpEd.PlugScope.Parameters[types.PlugScopeEntryPath], mirrored.Name)

		grp.ModifiedAt = time.Now()
		grp.ChangedAt = time.Now()
		if obj.IsGroup() {
			grp.RefCount += 1
		}
		if err = e.stdGroup.cacheStore.createEntry(ctx, obj, grp); err != nil {
			return nil, err
		}
		en = &obj.Metadata
		return
	}

	// update mirror record
	crt.Size = mirrored.Size
	if err = e.stdGroup.cacheStore.updateEntries(ctx, crt); err != nil {
		return nil, err
	}
	en = crt
	return
}
