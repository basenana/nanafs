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
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"time"
)

type Manager interface {
	Root(ctx context.Context) (Entry, error)
	GetEntry(ctx context.Context, id int64) (Entry, error)
	CreateEntry(ctx context.Context, parent Entry, attr EntryAttr) (Entry, error)
	DestroyEntry(ctx context.Context, parent, en Entry) error
	MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error)
	ChangeEntryParent(ctx context.Context, targetEntry, overwriteEntry, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error
}

func NewManager(store storage.ObjectStore, cfg config.Config) Manager {
	return &manager{
		store:  store,
		cfg:    cfg,
		logger: logger.NewLogger("entryManager"),
	}
}

type manager struct {
	store  storage.ObjectStore
	cfg    config.Config
	logger *zap.SugaredLogger
}

var _ Manager = &manager{}

func (m *manager) Root(ctx context.Context) (Entry, error) {
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
	root.Storage = m.cfg.Storages[0].ID
	return BuildEntry(root, m.store), m.store.SaveObject(ctx, nil, root)
}

func (m *manager) GetEntry(ctx context.Context, id int64) (Entry, error) {
	obj, err := m.store.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}
	return BuildEntry(obj, m.store), nil
}

func (m *manager) CreateEntry(ctx context.Context, parent Entry, attr EntryAttr) (Entry, error) {
	if !parent.IsGroup() {
		return nil, types.ErrNoGroup
	}
	grp := parent.Group()
	return grp.CreateEntry(ctx, attr)
}

func (m *manager) DestroyEntry(ctx context.Context, parent, en Entry) error {
	if !parent.IsGroup() {
		return types.ErrNoGroup
	}
	var (
		parentGrp = parent.Group()
		parentObj = parent.Object()
		obj       = en.Object()
		srcObj    *types.Object
		err       error
	)
	if en.IsMirror() {
		m.logger.Infow("entry is mirrored, delete ref count", "entry", obj.ID, "ref", obj.RefID)
		srcObj, err = m.store.GetObject(ctx, en.Metadata().RefID)
		if err != nil {
			m.logger.Errorw("query source object from meta server error", "entry", obj.ID, "ref", obj.RefID, "err", err.Error())
			return err
		}
	}
	if srcObj == nil && ((en.IsGroup() && obj.RefCount == 2) || (!en.IsGroup() && obj.RefCount == 1)) {
		return parentGrp.DestroyEntry(ctx, en)
	}

	if srcObj != nil {
		srcObj.RefCount -= 1
		srcObj.CreatedAt = time.Now()
	}

	if !obj.IsGroup() && obj.RefCount > 0 {
		m.logger.Infow("object has mirrors, remove parent id", "entry", obj.ID)
		obj.RefCount -= 1
		obj.ParentID = 0
		obj.ChangedAt = time.Now()
	}

	if obj.IsGroup() {
		parentObj.RefCount -= 1
	}
	parentObj.ChangedAt = time.Now()
	parentObj.ModifiedAt = time.Now()

	if err = m.store.DestroyObject(ctx, srcObj, parentObj, obj); err != nil {
		m.logger.Errorw("destroy object from meta server error", "enrty", obj.ID, "err", err.Error())
		return err
	}

	return nil
}

func (m *manager) MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error) {
	var (
		srcObj    = src.Object()
		parentObj = dstParent.Object()
		err       error
	)
	if src.IsGroup() {
		return nil, types.ErrIsGroup
	}
	if !dstParent.IsGroup() {
		return nil, types.ErrNoGroup
	}

	if src.IsMirror() {
		srcObj, err = m.store.GetObject(ctx, srcObj.RefID)
		if err != nil {
			m.logger.Errorw("query source object error", "entry", srcObj.ID, "srcObj", srcObj.RefID, "err", err.Error())
			return nil, err
		}
		m.logger.Infow("replace source object", "entry", srcObj.ID)
	}

	obj, err := initMirrorEntryObject(srcObj, parentObj, attr)
	if err != nil {
		m.logger.Errorw("create mirror object error", "srcEntry", srcObj.ID, "dstParent", parentObj.ID, "err", err.Error())
		return nil, err
	}

	srcObj.RefCount += 1
	srcObj.ChangedAt = time.Now()

	parentObj.ChangedAt = time.Now()
	parentObj.ModifiedAt = time.Now()
	if err = m.store.MirrorObject(ctx, srcObj, parentObj, obj); err != nil {
		m.logger.Errorw("update dst parent object ref count error", "srcEntry", srcObj.ID, "dstParent", parentObj.ID, "err", err.Error())
		return nil, err
	}
	return BuildEntry(obj, m.store), nil
}

func (m *manager) ChangeEntryParent(ctx context.Context, targetEntry, overwriteEntry, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error {
	if !oldParent.IsGroup() || !newParent.IsGroup() {
		return types.ErrNoGroup
	}

	var (
		oldParentObj = oldParent.Object()
		newParentObj = newParent.Object()
		entryObj     = targetEntry.Object()
	)
	if overwriteEntry != nil {
		if overwriteEntry.IsGroup() {
			children, err := overwriteEntry.Group().ListChildren(ctx)
			if err != nil {
				return err
			}
			if len(children) > 0 {
				return types.ErrIsExist
			}
		}

		if !opt.Replace {
			return types.ErrIsExist
		}

		if opt.Exchange {
			// TODO
			return types.ErrUnsupported
		}

		if err := m.DestroyEntry(ctx, newParent, overwriteEntry); err != nil {
			return err
		}
	}

	entryObj.Name = newName

	oldParentObj.ChangedAt = time.Now()
	oldParentObj.ModifiedAt = time.Now()
	oldParentObj.ChangedAt = time.Now()
	oldParentObj.ModifiedAt = time.Now()
	if entryObj.IsGroup() {
		oldParentObj.RefCount -= 1
		newParentObj.RefCount += 1
	}
	err := m.store.ChangeParent(ctx, oldParentObj, newParentObj, entryObj, types.ChangeParentOption{})
	if err != nil {
		m.logger.Errorw("change object parent failed", "entry", entryObj.ID, "newParent", newParentObj.ID, "newName", newName, "err", err.Error())
		return err
	}
	return nil
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
