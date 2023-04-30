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
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"runtime/trace"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	Root(ctx context.Context) (Entry, error)
	GetEntry(ctx context.Context, id int64) (Entry, error)
	CreateEntry(ctx context.Context, parent Entry, attr EntryAttr) (Entry, error)
	DestroyEntry(ctx context.Context, parent, en Entry) (bool, error)
	MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error)
	ChangeEntryParent(ctx context.Context, targetEntry, overwriteEntry, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error
	Open(ctx context.Context, en Entry, attr Attr) (File, error)
	SetCacheResetter(r CacheResetter)
	MustCloseAll()
}

func NewManager(store metastore.ObjectStore, cfg config.Config) (Manager, error) {
	storages := make(map[string]storage.Storage)
	var err error
	for i := range cfg.Storages {
		storages[cfg.Storages[i].ID], err = storage.NewStorage(cfg.Storages[i].ID, cfg.Storages[i].Type, cfg.Storages[i])
		if err != nil {
			return nil, err
		}
	}
	mgr := &manager{
		store:    store,
		cfg:      cfg,
		storages: storages,
		logger:   logger.NewLogger("entryManager"),
	}
	newLifecycle(mgr).initHooks()
	fileEntryLogger = mgr.logger.Named("files")
	return mgr, nil
}

type manager struct {
	store      metastore.ObjectStore
	storages   map[string]storage.Storage
	cacheReset CacheResetter
	cfg        config.Config
	logger     *zap.SugaredLogger
}

var _ Manager = &manager{}

func (m *manager) Root(ctx context.Context) (Entry, error) {
	defer trace.StartRegion(ctx, "dentry.manager.Root").End()
	root, err := m.store.GetObject(ctx, RootEntryID)
	if err == nil {
		return buildEntry(root, m.store), nil
	}
	if err != types.ErrNotFound {
		m.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	root = initRootEntryObject()
	root.Access.UID = m.cfg.FS.OwnerUid
	root.Access.GID = m.cfg.FS.OwnerGid
	root.Storage = m.cfg.Storages[0].ID
	return buildEntry(root, m.store), m.store.SaveObject(ctx, nil, root)
}

func (m *manager) GetEntry(ctx context.Context, id int64) (Entry, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntry").End()
	obj, err := m.store.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildEntry(obj, m.store), nil
}

func (m *manager) CreateEntry(ctx context.Context, parent Entry, attr EntryAttr) (Entry, error) {
	defer trace.StartRegion(ctx, "dentry.manager.CreateEntry").End()
	if !parent.IsGroup() {
		return nil, types.ErrNoGroup
	}
	grp := parent.Group()
	return grp.CreateEntry(ctx, attr)
}

func (m *manager) DestroyEntry(ctx context.Context, parent, en Entry) (bool, error) {
	defer trace.StartRegion(ctx, "dentry.manager.DestroyEntry").End()
	if !parent.IsGroup() {
		return false, types.ErrNoGroup
	}
	var (
		parentGrp = parent.Group()
		parentMd  = parent.Metadata()
		md        = en.Metadata()
		srcObj    *types.Object
		err       error
	)
	if en.IsMirror() {
		m.logger.Infow("entry is mirrored, delete ref count", "entry", md.ID, "ref", md.RefID)
		srcObj, err = m.store.GetObject(ctx, en.Metadata().RefID)
		if err != nil {
			m.logger.Errorw("query source object from meta server error", "entry", md.ID, "ref", md.RefID, "err", err.Error())
			return false, err
		}
	}

	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()
	if !en.IsGroup() && (md.RefCount > 1 || (md.RefCount == 1 && isFileOpened(md.ID))) {
		m.logger.Infow("remove object parent id", "entry", md.ID, "ref", md.RefID)
		md.RefCount -= 1
		md.ParentID = 0
		md.ChangedAt = time.Now()
		if err = m.store.SaveObject(ctx, &types.Object{Metadata: *parentMd}, &types.Object{Metadata: *md}); err != nil {
			return false, err
		}
		return false, nil
	}

	if srcObj == nil && ((en.IsGroup() && md.RefCount == 2) || (!en.IsGroup() && md.RefCount == 1)) {
		err = parentGrp.DestroyEntry(ctx, en)
		return err == nil, err
	}

	if srcObj != nil {
		srcObj.RefCount -= 1
		srcObj.ChangedAt = time.Now()
		srcObj.ModifiedAt = time.Now()
	}

	if en.IsGroup() {
		parentMd.RefCount -= 1
	}
	parentMd.ChangedAt = time.Now()
	parentMd.ModifiedAt = time.Now()

	if err = m.store.DestroyObject(ctx, srcObj, &types.Object{Metadata: *parentMd}, &types.Object{Metadata: *md}); err != nil {
		m.logger.Errorw("destroy object from meta server error", "entry", md.ID, "err", err.Error())
		return false, err
	}

	return true, nil
}

func (m *manager) MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error) {
	defer trace.StartRegion(ctx, "dentry.manager.MirrorEntry").End()
	var (
		srcMd    = src.Metadata()
		parentMd = dstParent.Metadata()
		err      error
	)
	if src.IsGroup() {
		return nil, types.ErrIsGroup
	}
	if !dstParent.IsGroup() {
		return nil, types.ErrNoGroup
	}

	if src.IsMirror() {
		m.logger.Warnw("source entry is mirrored", "entry", srcMd.RefID)
		return nil, fmt.Errorf("source entry is mirrored")
	}

	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()
	obj, err := initMirrorEntryObject(srcMd, parentMd, attr)
	if err != nil {
		m.logger.Errorw("create mirror object error", "srcEntry", srcMd.ID, "dstParent", parentMd.ID, "err", err.Error())
		return nil, err
	}

	srcMd.RefCount += 1
	srcMd.ChangedAt = time.Now()

	parentMd.ChangedAt = time.Now()
	parentMd.ModifiedAt = time.Now()
	if err = m.store.MirrorObject(ctx, &types.Object{Metadata: *srcMd}, &types.Object{Metadata: *parentMd}, obj); err != nil {
		m.logger.Errorw("update dst parent object ref count error", "srcEntry", srcMd.ID, "dstParent", parentMd.ID, "err", err.Error())
		return nil, err
	}
	return buildEntry(obj, m.store), nil
}

func (m *manager) ChangeEntryParent(ctx context.Context, targetEntry, overwriteEntry, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error {
	defer trace.StartRegion(ctx, "dentry.manager.ChangeEntryParent").End()
	if !oldParent.IsGroup() || !newParent.IsGroup() {
		return types.ErrNoGroup
	}

	var (
		oldParentMd = oldParent.Metadata()
		newParentMd = newParent.Metadata()
		entryMd     = targetEntry.Metadata()
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

		if _, err := m.DestroyEntry(ctx, newParent, overwriteEntry); err != nil {
			return err
		}
	}

	entryMd.Name = newName

	oldParentMd.ChangedAt = time.Now()
	oldParentMd.ModifiedAt = time.Now()
	oldParentMd.ChangedAt = time.Now()
	oldParentMd.ModifiedAt = time.Now()
	if targetEntry.IsGroup() {
		oldParentMd.RefCount -= 1
		newParentMd.RefCount += 1
	}
	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()
	err := m.store.ChangeParent(ctx, &types.Object{Metadata: *oldParentMd}, &types.Object{Metadata: *newParentMd}, &types.Object{Metadata: *entryMd}, types.ChangeParentOption{})
	if err != nil {
		m.logger.Errorw("change object parent failed", "entry", entryMd.ID, "newParent", newParentMd.ID, "newName", newName, "err", err.Error())
		return err
	}
	return nil
}

func (m *manager) Open(ctx context.Context, en Entry, attr Attr) (File, error) {
	defer trace.StartRegion(ctx, "dentry.manager.Open").End()
	if attr.Trunc {
		en.Metadata().Size = 0
		// TODO clean old data
		PublicFileActionEvent(events.ActionTypeTrunc, en)
	}

	var (
		f   File
		err error
	)
	switch en.Metadata().Kind {
	case types.SymLinkKind:
		f, err = openSymlink(en, attr)
	default:
		f, err = openFile(en, attr, m.store, m.storages[en.Metadata().Storage], m.cacheReset, m.cfg.FS)
	}
	if err != nil {
		return nil, err
	}
	PublicFileActionEvent(events.ActionTypeOpen, en)
	return &instrumentalFile{file: f}, nil
}

func (m *manager) MustCloseAll() {
	bio.CloseAll()
}

func (m *manager) SetCacheResetter(r CacheResetter) {
	m.cacheReset = r
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

type CacheResetter interface {
	ResetEntry(entry Entry)
}
