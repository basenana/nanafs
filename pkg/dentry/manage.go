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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
)

type Manage interface {
	Root(ctx context.Context) (Entry, error)
	CreateEntry(ctx context.Context, parent Entry, attr EntryAttr) (Entry, error)
	DestroyEntry(ctx context.Context, parent, en Entry) error
	MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error)
	ChangeEntryParent(ctx context.Context, old, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error
}

type manage struct {
	store  storage.ObjectStore
	cfg    config.Config
	logger *zap.SugaredLogger
}

func (m *manage) Root(ctx context.Context) (Entry, error) {
	root, err := m.store.GetObject(ctx, RootEntryID)
	if err == nil {
		return BuildEntry(root, m.store), nil
	}
	if err != types.ErrNotFound {
		m.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	root = initRootEntryObject()
	root.Access.UID = m.cfg.Owner.Uid
	root.Access.GID = m.cfg.Owner.Gid
	return BuildEntry(root, m.store), m.store.SaveObject(ctx, nil, root)
}

func (m *manage) CreateObject(ctx context.Context, parent Entry, attr EntryAttr) (Entry, error) {
	if !parent.IsGroup() {
		return nil, types.ErrNoGroup
	}
	grp := parent.Group()
	return grp.CreateEntry(ctx, attr)
}

func (m *manage) DestroyObject(ctx context.Context, parent, en Entry) error {
	if !parent.IsGroup() {
		return types.ErrNoGroup
	}
	parentGrp := parent.Group()
	meta := en.Metadata()
	if meta.RefID == 0 && ((en.IsGroup() && meta.RefCount == 2) || (!en.IsGroup() && meta.RefCount == 1)) {
		return parentGrp.DestroyEntry(ctx, en)
	}
	// TODO:
	return nil
}

func (m *manage) MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (m *manage) ChangeObjectParent(ctx context.Context, old, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error {
	//TODO implement me
	panic("implement me")
}

type EntryAttr struct {
	Name   string
	Dev    int64
	Kind   types.Kind
	Access types.Access
}

type ChangeParentAttr struct {
	Replace  bool
	Exchange bool
}
