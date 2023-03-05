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
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"time"
)

type Group interface {
	FindEntry(ctx context.Context, name string) (Entry, error)
	GetEntry(ctx context.Context, id int64) (Entry, error)
	CreateEntry(ctx context.Context, attr EntryAttr) (Entry, error)
	UpdateEntry(ctx context.Context, en Entry) error
	DestroyEntry(ctx context.Context, en Entry) error
	ListChildren(ctx context.Context) ([]Entry, error)
}

type stdGroup struct {
	Entry
	store storage.ObjectStore
}

var _ Group = &stdGroup{}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (Entry, error) {
	children, err := g.ListChildren(ctx)
	if err != nil {
		return nil, err
	}
	for i := range children {
		tgt := children[i]
		if tgt.Metadata().Name == name {
			return tgt, nil
		}
	}
	return nil, types.ErrNotFound
}

func (g *stdGroup) GetEntry(ctx context.Context, id int64) (Entry, error) {
	obj, err := g.store.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}
	return BuildEntry(obj, g.store), nil
}

func (g *stdGroup) CreateEntry(ctx context.Context, attr EntryAttr) (Entry, error) {
	_, err := g.FindEntry(ctx, attr.Name)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	if err == nil {
		return nil, types.ErrIsExist
	}
	groupObject := g.Object()
	// FIXME: use same attr object
	obj, err := types.InitNewObject(groupObject, types.ObjectAttr{
		Name:   attr.Name,
		Dev:    attr.Dev,
		Kind:   attr.Kind,
		Access: attr.Access,
	})
	if err != nil {
		return nil, err
	}
	if obj.IsGroup() {
		groupObject.RefCount += 1
	}
	groupObject.ChangedAt = time.Now()
	groupObject.ModifiedAt = time.Now()
	if err = g.store.SaveObject(ctx, groupObject, obj); err != nil {
		return nil, err
	}
	return BuildEntry(obj, g.store), nil
}

func (g *stdGroup) UpdateEntry(ctx context.Context, en Entry) error {
	err := g.store.SaveObject(ctx, g.Object(), en.Object())
	if err != nil {
		return err
	}
	return nil
}

func (g *stdGroup) DestroyEntry(ctx context.Context, en Entry) error {
	obj := en.Object()
	if obj.RefID != 0 {
		return types.ErrUnsupported
	}
	if en.IsGroup() && obj.RefCount > 2 {
		return types.ErrNotEmpty
	}
	if !en.IsGroup() && obj.RefCount > 1 {
		return types.ErrUnsupported
	}

	grpObj := g.Object()
	grpObj.RefCount -= 1
	grpObj.ChangedAt = time.Now()
	grpObj.ModifiedAt = time.Now()

	if err := g.store.DestroyObject(ctx, nil, grpObj, obj); err != nil {
		return err
	}
	return nil
}

func (g *stdGroup) ListChildren(ctx context.Context) ([]Entry, error) {
	defer utils.TraceRegion(ctx, "stdGroup.list")()
	it, err := g.store.ListChildren(ctx, g.Object())
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0)
	for it.HasNext() {
		next := it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, BuildEntry(next, g.store))
	}
	return result, nil
}

type dynamicGroup struct {
	*stdGroup
}

type mirroredGroup struct {
	*stdGroup
}
