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
	FindEntry(ctx context.Context, name string) (Entry, error)
	CreateEntry(ctx context.Context, attr EntryAttr) (Entry, error)
	UpdateEntry(ctx context.Context, en Entry) error
	RemoveEntry(ctx context.Context, en Entry) error
	ListChildren(ctx context.Context) ([]Entry, error)
}

type emptyGroup struct{}

var _ Group = emptyGroup{}

func (e emptyGroup) FindEntry(ctx context.Context, name string) (Entry, error) {
	return nil, types.ErrNotFound
}

func (e emptyGroup) CreateEntry(ctx context.Context, attr EntryAttr) (Entry, error) {
	return nil, types.ErrNoAccess
}

func (e emptyGroup) UpdateEntry(ctx context.Context, en Entry) error {
	return types.ErrNoAccess
}

func (e emptyGroup) RemoveEntry(ctx context.Context, en Entry) error {
	return types.ErrNoAccess
}

func (e emptyGroup) ListChildren(ctx context.Context) ([]Entry, error) {
	return make([]Entry, 0), nil
}

type stdGroup struct {
	Entry
	store metastore.ObjectStore
}

var _ Group = &stdGroup{}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (Entry, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.FindEntry").End()
	entryLifecycleLock.RLock()
	defer entryLifecycleLock.RUnlock()
	objects, err := g.store.ListObjects(ctx, types.Filter{Name: name, ParentID: g.Metadata().ID})
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
		logger.NewLogger("stdGroup").Warnf("lookup group %s with name %s, got objects: %s", g.Metadata().Name, name, strings.Join(objNames, ","))
	}
	return buildEntry(objects[0], g.store), nil
}

func (g *stdGroup) CreateEntry(ctx context.Context, attr EntryAttr) (Entry, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.CreateEntry").End()
	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()
	objects, err := g.store.ListObjects(ctx, types.Filter{Name: attr.Name, ParentID: g.Metadata().ID})
	if err != nil {
		return nil, err
	}
	if len(objects) > 0 {
		return nil, types.ErrIsExist
	}
	groupMd := g.Metadata()
	obj, err := types.InitNewObject(groupMd, types.ObjectAttr{
		Name:   attr.Name,
		Kind:   attr.Kind,
		Access: attr.Access,
	})
	if err != nil {
		return nil, err
	}
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

	if obj.IsGroup() {
		groupMd.RefCount += 1
	}
	groupMd.ChangedAt = time.Now()
	groupMd.ModifiedAt = time.Now()
	if err = g.store.SaveObjects(ctx, &types.Object{Metadata: *groupMd}, obj); err != nil {
		return nil, err
	}
	en := buildEntry(obj, g.store)
	return en, nil
}

func (g *stdGroup) UpdateEntry(ctx context.Context, en Entry) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.UpdateEntry").End()
	err := g.store.SaveObjects(ctx, &types.Object{Metadata: *g.Metadata()}, &types.Object{Metadata: *en.Metadata()})
	if err != nil {
		return err
	}
	return nil
}

func (g *stdGroup) RemoveEntry(ctx context.Context, en Entry) error {
	defer trace.StartRegion(ctx, "dentry.stdGroup.RemoveEntry").End()
	md := en.Metadata()
	if md.RefID != 0 {
		return types.ErrUnsupported
	}
	if en.IsGroup() && md.RefCount > 2 {
		return types.ErrNotEmpty
	}
	if !en.IsGroup() && md.RefCount > 1 {
		return types.ErrUnsupported
	}

	if !en.IsGroup() && md.RefCount > 0 {
		md.RefCount -= 1
		md.ChangedAt = time.Now()
	}

	grpMd := g.Metadata()
	if en.IsGroup() {
		grpMd.RefCount -= 1
	}
	grpMd.ChangedAt = time.Now()
	grpMd.ModifiedAt = time.Now()

	md.ParentID = 0
	if err := g.store.SaveObjects(ctx, &types.Object{Metadata: *grpMd}, &types.Object{Metadata: *md}); err != nil {
		return err
	}
	return nil
}

func (g *stdGroup) ListChildren(ctx context.Context) ([]Entry, error) {
	defer trace.StartRegion(ctx, "dentry.stdGroup.ListChildren").End()
	it, err := g.store.ListChildren(ctx, &types.Object{Metadata: *g.Metadata()})
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0)
	for it.HasNext() {
		next := it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, buildEntry(next, g.store))
	}
	return result, nil
}

type dynamicGroup struct {
	*stdGroup
}

type extGroup struct {
	stdGroup *stdGroup
	mirror   plugin.MirrorPlugin
}

func (e *extGroup) FindEntry(ctx context.Context, name string) (Entry, error) {
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

func (e *extGroup) CreateEntry(ctx context.Context, attr EntryAttr) (Entry, error) {
	mirrorEn, err := e.mirror.CreateEntry(ctx, stub.EntryAttr{
		Name:   attr.Name,
		Kind:   attr.Kind,
		Access: attr.Access,
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

func (e *extGroup) UpdateEntry(ctx context.Context, en Entry) error {
	md := en.Metadata()
	mirrorEn, err := e.mirror.FindEntry(ctx, md.Name)
	if err != nil {
		return err
	}

	mirrorEn.Size = en.Metadata().Size

	// query old and write back
	en, err = e.stdGroup.FindEntry(ctx, md.Name)
	if err != nil {
		return err
	}

	err = e.mirror.UpdateEntry(ctx, mirrorEn)
	if err != nil {
		return err
	}

	_, err = e.syncEntry(ctx, mirrorEn, en)
	return err
}

func (e *extGroup) RemoveEntry(ctx context.Context, en Entry) error {
	md := en.Metadata()
	mirrorEn, err := e.mirror.FindEntry(ctx, md.Name)
	if err != nil {
		return err
	}

	err = e.mirror.RemoveEntry(ctx, mirrorEn)
	if err != nil {
		return err
	}

	objects, err := e.stdGroup.store.ListObjects(ctx, types.Filter{Name: md.Name, ParentID: md.ParentID})
	if err != nil {
		return err
	}
	if len(objects) > 0 {
		en = buildEntry(objects[0], e.stdGroup.store)
		_, err = e.syncEntry(ctx, nil, en)
		if err == types.ErrNotFound {
			return nil
		}
		return err
	}
	return nil
}

func (e *extGroup) ListChildren(ctx context.Context) ([]Entry, error) {
	recordChild, err := e.stdGroup.ListChildren(ctx)
	if err != nil {
		return nil, err
	}
	actualChild, err := e.mirror.ListChildren(ctx)
	if err != nil {
		return nil, err
	}

	recordChildMap := make(map[string]Entry)
	actualChildMap := make(map[string]*stub.Entry)
	for i := range recordChild {
		recordChildMap[recordChild[i].Metadata().Name] = recordChild[i]
	}
	for i := range actualChild {
		actualChildMap[actualChild[i].Name] = actualChild[i]
	}

	result := make([]Entry, 0, len(actualChild))
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

func (e *extGroup) syncEntry(ctx context.Context, mirrored *stub.Entry, crt Entry) (en Entry, err error) {
	var (
		grpMd = e.stdGroup.Metadata()
		grpEd types.ExtendData
	)
	grpEd, err = e.stdGroup.GetExtendData(ctx)
	if err != nil {
		return
	}
	if grpEd.PlugScope == nil {
		err = fmt.Errorf("not ext group")
		return
	}

	if mirrored != nil && crt != nil && mirrored.IsGroup != crt.IsGroup() {
		// FIXME
		return nil, fmt.Errorf("entry and mirrored entry incorrect")
	}

	if mirrored == nil {
		err = types.ErrNotFound
		if crt != nil {
			// clean
			md := crt.Metadata()
			_ = e.stdGroup.store.DestroyObject(ctx, &types.Object{Metadata: *grpMd}, &types.Object{Metadata: *md})
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
		for k, v := range grpEd.PlugScope.Parameters {
			obj.PlugScope.Parameters[k] = v
		}
		obj.PlugScope.Parameters[types.PlugScopeEntryName] = mirrored.Name
		obj.PlugScope.Parameters[types.PlugScopeEntryPath] = path.Join(grpEd.PlugScope.Parameters[types.PlugScopeEntryPath], mirrored.Name)

		if obj.IsGroup() {
			grpMd.RefCount += 1
		}
		grpMd.ChangedAt = time.Now()
		grpMd.ModifiedAt = time.Now()
		if err = e.stdGroup.store.SaveObjects(ctx, &types.Object{Metadata: *grpMd}, obj); err != nil {
			return nil, err
		}
		en = buildEntry(obj, e.stdGroup.store)
		return
	}

	// update mirror record
	md := crt.Metadata()
	md.Size = mirrored.Size
	if err = e.stdGroup.store.SaveObjects(ctx, &types.Object{Metadata: *grpMd}, &types.Object{Metadata: *md}); err != nil {
		return nil, err
	}

	en = crt
	return
}
