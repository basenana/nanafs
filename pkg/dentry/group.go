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
	PatchEntry(ctx context.Context, entryID int64, patch *types.Metadata) error
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

func (e emptyGroup) PatchEntry(ctx context.Context, entryID int64, patch *types.Metadata) error {
	return types.ErrNoAccess
}

func (e emptyGroup) RemoveEntry(ctx context.Context, enId int64) error {
	return types.ErrNoAccess
}

func (e emptyGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	return make([]*types.Metadata, 0), nil
}

type stdGroup struct {
	meta  *types.Metadata
	store metastore.ObjectStore
}

var _ Group = &stdGroup{}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.FindEntry").End()
	entryLifecycleLock.RLock()
	defer entryLifecycleLock.RUnlock()
	objects, err := g.store.ListObjects(ctx, types.Filter{Name: name, ParentID: g.meta.ID})
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
		logger.NewLogger("stdGroup").Warnf("lookup group %s with name %s, got objects: %s", g.meta.Name, name, strings.Join(objNames, ","))
	}
	obj := objects[0]
	return &obj.Metadata, nil
}

func (g *stdGroup) CreateEntry(ctx context.Context, attr EntryAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.CreateEntry").End()
	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()
	objects, err := g.store.ListObjects(ctx, types.Filter{Name: attr.Name, ParentID: g.meta.ID})
	if err != nil {
		return nil, err
	}
	if len(objects) > 0 {
		return nil, types.ErrIsExist
	}

	group, err := cacheStore.getEntry(ctx, g.meta.ID)
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

	parentPatch := &types.Metadata{ID: group.ID, ModifiedAt: time.Now()}
	if obj.IsGroup() {
		parentPatch.RefCount += 1
	}
	if err = cacheStore.createEntry(ctx, obj, parentPatch); err != nil {
		return nil, err
	}
	return &obj.Metadata, nil
}

func (g *stdGroup) PatchEntry(ctx context.Context, entryId int64, patch *types.Metadata) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.UpdateEntry").End()
	patch.ID = entryId
	if err := cacheStore.patchEntryMeta(ctx, patch); err != nil {
		return err
	}
	return nil
}

func (g *stdGroup) RemoveEntry(ctx context.Context, entryId int64) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.RemoveEntry").End()
	group, err := cacheStore.getEntry(ctx, g.meta.ID)
	if err != nil {
		return err
	}

	entry, err := cacheStore.getEntry(ctx, entryId)
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

	var patches = make([]*types.Metadata, 2)
	patches[0] = &types.Metadata{ID: entry.ID, ParentID: 0}
	if !types.IsGroup(entry.Kind) && entry.RefCount > 0 {
		patches[0].RefCount -= 1
	}
	patches[1] = &types.Metadata{ID: group.ID, ModifiedAt: time.Now()}
	if types.IsGroup(entry.Kind) {
		patches[1].RefCount -= 1
	}
	if err := cacheStore.patchEntryMeta(ctx, patches...); err != nil {
		return err
	}
	return nil
}

func (g *stdGroup) ListChildren(ctx context.Context) ([]*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.ListChildren").End()
	it, err := g.store.ListChildren(ctx, &types.Object{Metadata: *g.meta})
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

func (e *extGroup) PatchEntry(ctx context.Context, entryId int64, patch *types.Metadata) error {
	group, err := cacheStore.getEntry(ctx, e.stdGroup.meta.ID)
	if err != nil {
		return err
	}
	mirrorEn, err := e.mirror.FindEntry(ctx, group.Name)
	if err != nil {
		return err
	}

	mirrorEn.Size = patch.Size

	// query old and write back
	entry, err := cacheStore.getEntry(ctx, entryId)
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
	entry, err := cacheStore.getEntry(ctx, entryId)
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
	var (
		grpMd = e.stdGroup.meta
		grpEd types.ExtendData
	)
	grpEd, err = e.mgr.GetEntryExtendData(ctx, e.stdGroup.meta.ID)
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
			_ = e.stdGroup.store.DestroyObject(ctx, &types.Object{Metadata: *grpMd}, &types.Object{Metadata: *crt})
			cacheStore.delEntryCache(crt.ID)
			cacheStore.delEntryCache(grpMd.ID)
		}
		return nil, err
	}

	if crt == nil {
		// create mirror record
		var obj *types.Object
		obj, err = types.InitNewObject(grpMd, types.ObjectAttr{
			Name:   mirrored.Name,
			Kind:   mirrored.Kind,
			Access: grpMd.Access,
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

		grpPatch := &types.Metadata{ID: grpMd.ID, ModifiedAt: time.Now(), ChangedAt: time.Now()}
		if obj.IsGroup() {
			grpPatch.RefCount = grpMd.RefCount + 1
		}
		if err = cacheStore.createEntry(ctx, obj, grpPatch); err != nil {
			return nil, err
		}
		en = &obj.Metadata
		return
	}

	// update mirror record
	crt.Size = mirrored.Size
	var (
		grpPatch = &types.Metadata{ID: grpMd.ID}
		enPatch  = &types.Metadata{ID: crt.ID}
	)
	patchChangeableMetadata(grpPatch, grpMd)
	patchChangeableMetadata(enPatch, crt)
	if err = cacheStore.patchEntryMeta(ctx, grpPatch, enPatch); err != nil {
		return nil, err
	}
	en = crt
	return
}
