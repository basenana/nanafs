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
	"github.com/basenana/nanafs/utils"
	"io"
	"runtime/trace"
	"sync"
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
	RemoveEntry(ctx context.Context, parent, en Entry) error
	DestroyEntry(ctx context.Context, en Entry) error
	CleanEntryData(ctx context.Context, en Entry) error
	MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error)
	ChangeEntryParent(ctx context.Context, targetEntry, overwriteEntry, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error
	Open(ctx context.Context, en Entry, attr Attr) (File, error)
	ChunkCompact(ctx context.Context, en Entry) error
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
	fileEntryLogger = mgr.logger.Named("files")
	return mgr, nil
}

var entryLifecycleLock sync.RWMutex

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
	root.Access.UID = m.cfg.FS.Owner.Uid
	root.Access.GID = m.cfg.FS.Owner.Gid
	root.Storage = m.cfg.Storages[0].ID
	return buildEntry(root, m.store), m.store.SaveObjects(ctx, root)
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

func (m *manager) RemoveEntry(ctx context.Context, parent, en Entry) error {
	defer trace.StartRegion(ctx, "dentry.manager.RemoveEntry").End()
	if parent == nil || !parent.IsGroup() {
		return types.ErrNoGroup
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
			return err
		}
	}

	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()

	if srcObj == nil && ((en.IsGroup() && md.RefCount == 2) || (!en.IsGroup() && md.RefCount == 1)) {
		err = parentGrp.RemoveEntry(ctx, en)
		return err
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

	md.ParentID = 0
	md.RefCount -= 1
	md.ChangedAt = time.Now()
	if err = m.store.SaveObjects(ctx, srcObj, &types.Object{Metadata: *parentMd}, &types.Object{Metadata: *md}); err != nil {
		m.logger.Errorw("destroy object from meta server error", "entry", md.ID, "err", err.Error())
		return err
	}

	return nil
}

func (m *manager) DestroyEntry(ctx context.Context, en Entry) error {
	defer trace.StartRegion(ctx, "dentry.manager.DestroyEntry").End()
	var (
		md     = en.Metadata()
		srcObj *types.Object
		err    error
	)
	if en.IsMirror() {
		srcObj, err = m.store.GetObject(ctx, en.Metadata().RefID)
		if err != nil {
			m.logger.Warnw("query source object from meta server error", "entry", md.ID, "ref", md.RefID, "err", err.Error())
		}
	}

	if err = m.store.DestroyObject(ctx, srcObj, &types.Object{Metadata: *md}); err != nil {
		m.logger.Errorw("destroy object from meta server error", "entry", md.ID, "err", err.Error())
		return err
	}
	return nil
}

func (m *manager) CleanEntryData(ctx context.Context, en Entry) error {
	md := en.Metadata()
	if md.Storage == externalStorage {
		return nil
	}

	s, ok := m.storages[md.Storage]
	if !ok {
		return fmt.Errorf("storage %s not register", md.Storage)
	}

	cs, ok := m.store.(metastore.ChunkStore)
	if !ok {
		return nil
	}

	defer logger.CostLog(m.logger.With(zap.Int64("entry", md.ID)), "clean entry data")()
	err := bio.DeleteChunksData(ctx, md, cs, s)
	if err != nil {
		m.logger.Errorw("delete chunk data failed", "entry", md.ID, "err", err)
		return err
	}
	return nil
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
	// TODO delete overwrite entry on outside
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

		if err := m.RemoveEntry(ctx, newParent, overwriteEntry); err != nil {
			return err
		}
		PublicEntryActionEvent(events.ActionTypeDestroy, overwriteEntry)
	}

	if oldParentMd.Kind == types.ExternalGroupKind || newParentMd.Kind == types.ExternalGroupKind {
		return m.changeEntryParentByFileCopy(ctx, targetEntry, oldParent, newParent, newName, opt)
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

func (m *manager) changeEntryParentByFileCopy(ctx context.Context, targetEntry, oldParent, newParent Entry, newName string, _ ChangeParentAttr) error {
	var (
		targetMd = targetEntry.Metadata()
	)

	newParentEd, err := newParent.GetExtendData(ctx)
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, query new parent extend data failed", "err", err)
		return err
	}

	if targetEntry.IsGroup() {
		if oldParent.Metadata().ID == newParent.Metadata().ID {
			// only rename
			targetMd.Name = newName
			err = m.store.SaveObjects(ctx, &types.Object{Metadata: *targetMd})
			if err != nil {
				m.logger.Errorw("change entry parent by file copy error, rename dir failed", "err", err)
				return err
			}
			return nil
		}
		// TODO: move file with scheduled task
		return types.ErrUnsupported
	}

	// step 1: create new file
	attr := EntryAttr{
		Name:      newName,
		Kind:      targetMd.Kind,
		Access:    targetMd.Access,
		PlugScope: newParentEd.PlugScope,
	}
	en, err := m.CreateEntry(ctx, newParent, attr)
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, create new entry failed", "err", err)
		return err
	}

	// step 2: copy old to new file
	oldFileReader, err := m.Open(ctx, targetEntry, Attr{Read: true})
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, open old file failed", "err", err)
		return err
	}
	newFileWriter, err := m.Open(ctx, en, Attr{Write: true})
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, open new file failed", "err", err)
		return err
	}
	_, err = io.Copy(utils.NewWriterWithContextWriter(newFileWriter), utils.NewReaderWithContextReaderAt(oldFileReader))
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, copy file content failed", "err", err)
		return err
	}

	// step 3: delete old file
	if err = m.RemoveEntry(ctx, oldParent, targetEntry); err != nil {
		m.logger.Errorw("change entry parent by file copy error, clean up old file failed", "err", err)
		return err
	}
	return nil
}

func (m *manager) Open(ctx context.Context, en Entry, attr Attr) (File, error) {
	defer trace.StartRegion(ctx, "dentry.manager.Open").End()
	md := en.Metadata()
	if attr.Trunc && md.Storage != externalStorage {
		if err := m.CleanEntryData(ctx, en); err != nil {
			m.logger.Errorw("clean entry with trunc error", "entry", en.Metadata().ID, "err", err)
		}
		en.Metadata().Size = 0
		if err := m.store.SaveObjects(ctx, &types.Object{Metadata: *md}); err != nil {
			m.logger.Errorw("update entry size to zero error", "entry", en.Metadata().ID, "err", err)
		}
		PublicFileActionEvent(events.ActionTypeTrunc, en)
	}

	var (
		f   File
		err error
	)
	if md.Storage == externalStorage {
		var ed types.ExtendData
		ed, err = en.GetExtendData(ctx)
		if err != nil {
			m.logger.Errorw("get entry extend data failed", "entry", en.Metadata().ID, "err", err)
			return nil, err
		}
		f, err = openExternalFile(en, ed.PlugScope, attr, m.store, m.cfg.FS)
	} else {
		switch md.Kind {
		case types.SymLinkKind:
			f, err = openSymlink(en, attr)
		default:
			f, err = openFile(en, attr, m.store, m.storages[en.Metadata().Storage], m.cacheReset, m.cfg.FS)
		}
	}
	if err != nil {
		return nil, err
	}
	PublicFileActionEvent(events.ActionTypeOpen, en)
	return instrumentalFile{Entry: en, file: f}, nil
}

func (m *manager) ChunkCompact(ctx context.Context, en Entry) error {
	chunkStore, ok := m.store.(metastore.ChunkStore)
	if !ok {
		return fmt.Errorf("not chunk store")
	}
	dataStorage, ok := m.storages[en.Metadata().Storage]
	if !ok {
		return fmt.Errorf("storage %s not registered", en.Metadata().Storage)
	}
	return bio.CompactChunksData(ctx, en.Metadata(), chunkStore, dataStorage)
}

func (m *manager) MustCloseAll() {
	bio.CloseAll()
}

func (m *manager) SetCacheResetter(r CacheResetter) {
	m.cacheReset = r
}

type EntryAttr struct {
	Name        string
	Kind        types.Kind
	Access      types.Access
	Dev         int64
	PlugScope   *types.PlugScope
	GroupFilter *types.Rule
}

type ChangeParentAttr struct {
	Replace  bool
	Exchange bool
}

type CacheResetter interface {
	ResetEntry(entry Entry)
}
