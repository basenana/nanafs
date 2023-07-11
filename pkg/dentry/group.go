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
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
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
	// FIXME: use same attr object
	obj, err := types.InitNewObject(groupMd, types.ObjectAttr{
		Name:   attr.Name,
		Dev:    attr.Dev,
		Kind:   attr.Kind,
		Access: attr.Access,
	})
	if err != nil {
		return nil, err
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
	*stdGroup
}

func (e *extGroup) FindEntry(ctx context.Context, name string) (Entry, error) {
	return e.stdGroup.FindEntry(ctx, name)
}

func (e *extGroup) CreateEntry(ctx context.Context, attr EntryAttr) (Entry, error) {
	return e.stdGroup.CreateEntry(ctx, attr)
}

func (e *extGroup) UpdateEntry(ctx context.Context, en Entry) error {
	return e.stdGroup.UpdateEntry(ctx, en)
}

func (e *extGroup) RemoveEntry(ctx context.Context, en Entry) error {
	return e.stdGroup.RemoveEntry(ctx, en)
}

func (e *extGroup) ListChildren(ctx context.Context) ([]Entry, error) {
	return e.stdGroup.ListChildren(ctx)
}
