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
	"github.com/basenana/nanafs/pkg/plugin"
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
	Root(ctx context.Context) (*types.Metadata, error)

	GetEntry(ctx context.Context, id int64) (*types.Metadata, error)
	CreateEntry(ctx context.Context, parentId int64, attr EntryAttr) (*types.Metadata, error)
	RemoveEntry(ctx context.Context, parentId, entryId int64) error
	DestroyEntry(ctx context.Context, entryId int64) error
	CleanEntryData(ctx context.Context, entryId int64) error
	MirrorEntry(ctx context.Context, srcId, dstParentId int64, attr EntryAttr) (*types.Metadata, error)
	ChangeEntryParent(ctx context.Context, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, newName string, opt ChangeParentAttr) error

	GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error)
	UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error
	GetEntryExtendField(ctx context.Context, id int64, fKey string) (*string, error)
	SetEntryExtendField(ctx context.Context, id int64, fKey, fVal string) error
	RemoveEntryExtendField(ctx context.Context, id int64, fKey string) error

	Open(ctx context.Context, entryId int64, attr Attr) (File, error)
	OpenGroup(ctx context.Context, entryID int64) (Group, error)
	ChunkCompact(ctx context.Context, entryId int64) error

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
		metastore: store,
		storages:  storages,
		cache:     newCacheStore(store),
		cfg:       cfg,
		logger:    logger.NewLogger("entryManager"),
	}
	fileEntryLogger = mgr.logger.Named("files")
	return mgr, nil
}

var entryLifecycleLock sync.RWMutex

type manager struct {
	metastore metastore.ObjectStore
	storages  map[string]storage.Storage
	cache     *metaCache
	cfg       config.Config
	logger    *zap.SugaredLogger
}

var _ Manager = &manager{}

func (m *manager) Root(ctx context.Context) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.Root").End()
	root, err := m.GetEntry(ctx, RootEntryID)
	if err == nil {
		return root, nil
	}
	if err != types.ErrNotFound {
		m.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	rootObj := initRootEntryObject()
	rootObj.Access.UID = m.cfg.FS.Owner.Uid
	rootObj.Access.GID = m.cfg.FS.Owner.Gid
	rootObj.Storage = m.cfg.Storages[0].ID
	err = m.cache.createEntry(ctx, rootObj, nil)
	if err != nil {
		return nil, err
	}
	return m.GetEntry(ctx, RootEntryID)
}

func (m *manager) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryMetadata").End()
	return m.cache.getEntry(ctx, id)
}

func (m *manager) GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryExtendData").End()
	entry, err := m.GetEntry(ctx, id)
	if err != nil {
		return types.ExtendData{}, err
	}
	obj := &types.Object{Metadata: *entry}
	err = m.metastore.GetObjectExtendData(ctx, obj)
	if err != nil {
		return types.ExtendData{}, err
	}
	return *obj.ExtendData, nil
}

func (m *manager) UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error {
	defer trace.StartRegion(ctx, "dentry.manager.UpdateEntryExtendData").End()
	entry, err := m.GetEntry(ctx, id)
	if err != nil {
		return err
	}
	defer m.cache.delEntryCache(id)
	entry.ChangedAt = time.Now()
	err = m.metastore.SaveObjects(ctx, &types.Object{Metadata: *entry, ExtendData: &ed, ExtendDataChanged: true})
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) GetEntryExtendField(ctx context.Context, id int64, fKey string) (*string, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryExtendField").End()
	ed, err := m.GetEntryExtendData(ctx, id)
	if err != nil {
		return nil, err
	}
	if ed.Properties.Fields == nil {
		return nil, nil
	}
	fVal, ok := ed.Properties.Fields[fKey]
	if !ok {
		return nil, nil
	}
	return &fVal, nil
}

func (m *manager) SetEntryExtendField(ctx context.Context, id int64, fKey, fVal string) error {
	defer trace.StartRegion(ctx, "dentry.manager.SetEntryExtendField").End()
	ed, err := m.GetEntryExtendData(ctx, id)
	if err != nil {
		return err
	}

	if ed.Properties.Fields == nil {
		ed.Properties.Fields = map[string]string{}
	}
	ed.Properties.Fields[fKey] = fVal

	return m.UpdateEntryExtendData(ctx, id, ed)
}

func (m *manager) RemoveEntryExtendField(ctx context.Context, id int64, fKey string) error {
	defer trace.StartRegion(ctx, "dentry.manager.RemoveEntryExtendField").End()
	ed, err := m.GetEntryExtendData(ctx, id)
	if err != nil {
		return err
	}

	if ed.Properties.Fields == nil {
		ed.Properties.Fields = map[string]string{}
	}
	_, ok := ed.Properties.Fields[fKey]
	if ok {
		delete(ed.Properties.Fields, fKey)
	}
	if !ok {
		return types.ErrNotFound
	}
	return m.UpdateEntryExtendData(ctx, id, ed)
}

func (m *manager) CreateEntry(ctx context.Context, parentId int64, attr EntryAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.CreateEntry").End()
	grp, err := m.OpenGroup(ctx, parentId)
	if err != nil {
		return nil, err
	}
	return grp.CreateEntry(ctx, attr)
}

func (m *manager) RemoveEntry(ctx context.Context, parentId, entryId int64) error {
	defer trace.StartRegion(ctx, "dentry.manager.RemoveEntry").End()
	parentGrp, err := m.OpenGroup(ctx, parentId)
	if err != nil {
		return err
	}

	entry, err := m.GetEntry(ctx, entryId)
	if err != nil {
		return err
	}

	var src *types.Metadata
	if types.IsMirrored(entry) {
		m.logger.Debugw("entry is mirrored, delete ref count", "entry", entry.ID, "ref", entry.RefID)
		src, err = m.GetEntry(ctx, entry.RefID)
		if err != nil {
			m.logger.Errorw("query source object from meta server error", "entry", entry.ID, "ref", entry.RefID, "err", err)
			return err
		}
	}

	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()

	if src == nil && ((types.IsGroup(entry.Kind) && entry.RefCount == 2) || (!types.IsGroup(entry.Kind) && entry.RefCount == 1)) {
		err = parentGrp.RemoveEntry(ctx, entryId)
		return err
	}

	var nowTime = time.Now()
	parent, err := m.GetEntry(ctx, parentId)
	if err != nil {
		return err
	}
	parent.ModifiedAt = nowTime
	if types.IsGroup(entry.Kind) {
		parent.RefCount -= 1
	}

	entry.ParentID = 0
	entry.RefCount -= 1
	entry.ModifiedAt = nowTime

	needUpdate := []*types.Metadata{parent, entry}
	if src != nil {
		src.RefCount -= 1
		src.ModifiedAt = nowTime
		needUpdate = append(needUpdate, src)
	}

	if err = m.cache.updateEntries(ctx, needUpdate...); err != nil {
		m.logger.Errorw("destroy object from meta server error", "entry", entry.ID, "err", err)
		return err
	}
	return nil
}

func (m *manager) DestroyEntry(ctx context.Context, entryID int64) error {
	defer trace.StartRegion(ctx, "dentry.manager.DestroyEntry").End()

	entry, err := m.GetEntry(ctx, entryID)
	if err != nil {
		return err
	}

	var srcObj *types.Object
	if types.IsMirrored(entry) {
		srcObj, err = m.metastore.GetObject(ctx, entry.RefID)
		if err != nil {
			m.logger.Warnw("query source object from meta server error", "entry", entry.ID, "ref", entry.RefID, "err", err)
		}
	}

	if err = m.metastore.DestroyObject(ctx, srcObj, &types.Object{Metadata: *entry}); err != nil {
		m.logger.Errorw("destroy object from meta server error", "entry", entry.ID, "err", err.Error())
		return err
	}
	if srcObj != nil {
		m.cache.delEntryCache(srcObj.ID)
	}
	m.cache.delEntryCache(entry.ID)
	return nil
}

func (m *manager) CleanEntryData(ctx context.Context, entryId int64) error {
	entry, err := m.GetEntry(ctx, entryId)
	if err != nil {
		return err
	}
	if entry.Storage == externalStorage {
		return nil
	}

	s, ok := m.storages[entry.Storage]
	if !ok {
		return fmt.Errorf("storage %s not register", entry.Storage)
	}

	cs, ok := m.metastore.(metastore.ChunkStore)
	if !ok {
		return nil
	}

	defer logger.CostLog(m.logger.With(zap.Int64("entry", entry.ID)), "clean entry data")()
	err = bio.DeleteChunksData(ctx, entry, cs, s)
	if err != nil {
		m.logger.Errorw("delete chunk data failed", "entry", entry.ID, "err", err)
		return err
	}
	return nil
}

func (m *manager) MirrorEntry(ctx context.Context, srcId, dstParentId int64, attr EntryAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.MirrorEntry").End()

	src, err := m.GetEntry(ctx, srcId)
	if err != nil {
		return nil, err
	}

	parent, err := m.GetEntry(ctx, dstParentId)
	if err != nil {
		return nil, err
	}
	if types.IsGroup(src.Kind) {
		return nil, types.ErrIsGroup
	}
	if !types.IsGroup(parent.Kind) {
		return nil, types.ErrNoGroup
	}

	if types.IsMirrored(src) {
		m.logger.Warnw("source entry is mirrored", "entry", srcId)
		return nil, fmt.Errorf("source entry is mirrored")
	}

	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()
	obj, err := initMirrorEntryObject(src, parent, attr)
	if err != nil {
		m.logger.Errorw("create mirror object error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}

	if err = m.metastore.MirrorObject(ctx, &types.Object{Metadata: *src}, &types.Object{Metadata: *parent}, obj); err != nil {
		m.logger.Errorw("update dst parent object ref count error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}
	m.cache.delEntryCache(src.ID)
	m.cache.delEntryCache(parent.ID)
	m.cache.delEntryCache(obj.ID)
	return &obj.Metadata, nil
}

func (m *manager) ChangeEntryParent(ctx context.Context, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, newName string, opt ChangeParentAttr) error {
	defer trace.StartRegion(ctx, "dentry.manager.ChangeEntryParent").End()

	oldParent, err := m.GetEntry(ctx, oldParentId)
	if err != nil {
		return err
	}
	newParent, err := m.GetEntry(ctx, newParentId)
	if err != nil {
		return err
	}
	target, err := m.GetEntry(ctx, targetEntryId)
	if err != nil {
		return err
	}

	// TODO delete overwrite entry on outside
	if overwriteEntryId != nil {
		overwriteEntry, err := m.GetEntry(ctx, *overwriteEntryId)
		if err != nil {
			return err
		}
		if types.IsGroup(overwriteEntry.Kind) {
			overwriteGrp, err := m.OpenGroup(ctx, *overwriteEntryId)
			if err != nil {
				return err
			}
			children, err := overwriteGrp.ListChildren(ctx)
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

		if err := m.RemoveEntry(ctx, newParentId, *overwriteEntryId); err != nil {
			return err
		}
		PublicEntryActionEvent(events.ActionTypeDestroy, overwriteEntry)
	}

	if oldParent.Kind == externalStorage || newParent.Kind == externalStorage || oldParent.Storage != newParent.Storage {
		return m.changeEntryParentByFileCopy(ctx, target, oldParent, newParent, newName, opt)
	}

	entryLifecycleLock.Lock()
	defer entryLifecycleLock.Unlock()
	err = m.metastore.ChangeParent(ctx, &types.Object{Metadata: *oldParent}, &types.Object{Metadata: *newParent}, &types.Object{Metadata: *target}, types.ChangeParentOption{Name: newName})
	if err != nil {
		m.logger.Errorw("change object parent failed", "entry", target.ID, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	m.cache.delEntryCache(oldParent.ID)
	m.cache.delEntryCache(newParent.ID)
	m.cache.delEntryCache(target.ID)
	return nil
}

func (m *manager) changeEntryParentByFileCopy(ctx context.Context, targetEntry, oldParent, newParent *types.Metadata, newName string, _ ChangeParentAttr) error {
	newParentEd, err := m.GetEntryExtendData(ctx, newParent.ID)
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, query new parent extend data failed", "err", err)
		return err
	}

	if types.IsGroup(targetEntry.Kind) {
		if oldParent.ID == newParent.ID {
			// only rename
			targetEntry.Name = newName
			err = m.cache.updateEntries(ctx, targetEntry)
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
		Kind:      targetEntry.Kind,
		Access:    targetEntry.Access,
		PlugScope: newParentEd.PlugScope,
	}
	en, err := m.CreateEntry(ctx, newParent.ID, attr)
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, create new entry failed", "err", err)
		return err
	}

	// step 2: copy old to new file
	oldFileReader, err := m.Open(ctx, targetEntry.ID, Attr{Read: true})
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, open old file failed", "err", err)
		return err
	}
	newFileWriter, err := m.Open(ctx, en.ID, Attr{Write: true})
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, open new file failed", "err", err)
		return err
	}
	_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, newFileWriter), utils.NewReaderWithContextReaderAt(ctx, oldFileReader))
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, copy file content failed", "err", err)
		return err
	}

	// step 3: delete old file
	if err = m.RemoveEntry(ctx, oldParent.ID, targetEntry.ID); err != nil {
		m.logger.Errorw("change entry parent by file copy error, clean up old file failed", "err", err)
		return err
	}
	return nil
}

func (m *manager) Open(ctx context.Context, entryId int64, attr Attr) (File, error) {
	defer trace.StartRegion(ctx, "dentry.manager.Open").End()
	entry, err := m.GetEntry(ctx, entryId)
	if err != nil {
		return nil, err
	}

	attr.EntryID = entryId
	// do not update ctime
	entry.AccessAt = time.Now()
	if attr.Write {
		entry.ModifiedAt = entry.AccessAt
	}
	if attr.Trunc && entry.Storage != externalStorage {
		if err := m.CleanEntryData(ctx, entryId); err != nil {
			m.logger.Errorw("clean entry with trunc error", "entry", entryId, "err", err)
		}
		entry.Size = 0
		PublicFileActionEvent(events.ActionTypeTrunc, entry)
	}

	defer m.cache.delEntryCache(entryId)
	if err = m.metastore.SaveObjects(ctx, &types.Object{Metadata: *entry}); err != nil {
		m.logger.Errorw("update entry size to zero error", "entry", entryId, "err", err)
	}

	var f File
	if entry.Storage == externalStorage {
		var ed types.ExtendData
		ed, err = m.GetEntryExtendData(ctx, entryId)
		if err != nil {
			m.logger.Errorw("get entry extend data failed", "entry", entryId, "err", err)
			return nil, err
		}
		f, err = openExternalFile(entry, ed.PlugScope, attr, m.cache, m.cfg.FS)
	} else {
		switch entry.Kind {
		case types.SymLinkKind:
			f, err = openSymlink(m, entry, attr)
		default:
			f, err = openFile(entry, attr, m.metastore, m.cache, m.storages[entry.Storage], m.cfg.FS)
		}
	}
	if err != nil {
		return nil, err
	}
	PublicFileActionEvent(events.ActionTypeOpen, entry)
	return instrumentalFile{file: f}, nil
}
func (m *manager) OpenGroup(ctx context.Context, groupId int64) (Group, error) {
	defer trace.StartRegion(ctx, "dentry.manager.OpenGroup").End()
	entry, err := m.GetEntry(ctx, groupId)
	if err != nil {
		return nil, err
	}
	if !types.IsGroup(entry.Kind) {
		return nil, types.ErrNoGroup
	}
	var (
		stdGrp       = &stdGroup{entryID: entry.ID, name: entry.Name, store: m.metastore, cacheStore: m.cache}
		grp    Group = stdGrp
	)
	switch entry.Kind {
	case types.SmartGroupKind:
		grp = &dynamicGroup{stdGroup: stdGrp}
	case types.ExternalGroupKind:
		obj := &types.Object{Metadata: *entry}
		if err = m.metastore.GetObjectExtendData(ctx, obj); err != nil {
			return nil, err
		}
		if obj.ExtendData != nil && obj.PlugScope != nil {
			mirror, err := plugin.NewMirrorPlugin(ctx, *obj.ExtendData.PlugScope)
			if err != nil {
				return nil, err
			}
			grp = &extGroup{mgr: m, stdGroup: stdGrp, mirror: mirror}
		} else {
			grp = emptyGroup{}
		}
	}
	return instrumentalGroup{grp: grp}, nil
}

func (m *manager) ChunkCompact(ctx context.Context, entryId int64) error {
	defer trace.StartRegion(ctx, "dentry.manager.ChunkCompact").End()
	entry, err := m.GetEntry(ctx, entryId)
	if err != nil {
		return err
	}
	chunkStore, ok := m.metastore.(metastore.ChunkStore)
	if !ok {
		return fmt.Errorf("not chunk store")
	}
	dataStorage, ok := m.storages[entry.Storage]
	if !ok {
		return fmt.Errorf("storage %s not registered", entry.Storage)
	}
	return bio.CompactChunksData(ctx, entry, chunkStore, dataStorage)
}

func (m *manager) MustCloseAll() {
	bio.CloseAll()
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
