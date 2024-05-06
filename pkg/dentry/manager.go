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
	"errors"
	"fmt"
	"io"
	"path"
	"runtime/trace"
	"strings"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/plugin"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	Root(ctx context.Context) (*types.Metadata, error)

	GetEntry(ctx context.Context, id int64) (*types.Metadata, error)
	GetEntryByUri(ctx context.Context, uri string) (*types.Metadata, error)
	CreateEntry(ctx context.Context, parentId int64, attr types.EntryAttr) (*types.Metadata, error)
	RemoveEntry(ctx context.Context, parentId, entryId int64) error
	DestroyEntry(ctx context.Context, entryId int64) error
	CleanEntryData(ctx context.Context, entryId int64) error
	MirrorEntry(ctx context.Context, srcId, dstParentId int64, attr types.EntryAttr) (*types.Metadata, error)
	ChangeEntryParent(ctx context.Context, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error

	GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error)
	UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error
	GetEntryExtendField(ctx context.Context, id int64, fKey string) (*string, bool, error)
	SetEntryExtendField(ctx context.Context, id int64, fKey, fVal string, encoded bool) error
	RemoveEntryExtendField(ctx context.Context, id int64, fKey string) error
	GetEntryLabels(ctx context.Context, id int64) (types.Labels, error)
	UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error

	Open(ctx context.Context, entryId int64, attr types.OpenAttr) (File, error)
	OpenGroup(ctx context.Context, entryID int64) (Group, error)
	ChunkCompact(ctx context.Context, entryId int64) error

	MustCloseAll()
}

func NewManager(store metastore.Meta, cfg config.Bootstrap) (Manager, error) {
	var (
		defaultStorage storage.Storage
		storages       = make(map[string]storage.Storage)
		err            error
	)
	for i := range cfg.Storages {
		storages[cfg.Storages[i].ID], err = storage.NewStorage(cfg.Storages[i].ID, cfg.Storages[i].Type, cfg.Storages[i])
		if err != nil {
			return nil, err
		}
	}
	defaultStorage = storages[cfg.Storages[0].ID]

	mgr := &manager{
		store:          store,
		metastore:      store,
		extIndexer:     NewExtIndexer(),
		defaultStorage: defaultStorage,
		storages:       storages,
		eventQ:         make(chan *entryEvent, 8),
		fsOwnerUid:     cfg.FS.Owner.Uid,
		fsOwnerGid:     cfg.FS.Owner.Gid,
		fsWriteback:    cfg.FS.Writeback,
		logger:         logger.NewLogger("entryManager"),
	}

	go mgr.entryActionEventHandler()
	fileEntryLogger = mgr.logger.Named("files")
	return mgr, err
}

type manager struct {
	store          metastore.DEntry
	metastore      metastore.Meta
	extIndexer     *ExtIndexer
	defaultStorage storage.Storage
	storages       map[string]storage.Storage
	eventQ         chan *entryEvent
	cfgLoader      config.Loader
	fsOwnerUid     int64
	fsOwnerGid     int64
	fsWriteback    bool
	logger         *zap.SugaredLogger
}

var _ Manager = &manager{}

func (m *manager) Root(ctx context.Context) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.Root").End()
	root, err := m.GetEntry(ctx, RootEntryID)
	if err == nil {
		return root, nil
	}
	if !errors.Is(err, types.ErrNotFound) {
		m.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	root = initRootEntry()
	root.Access.UID = m.fsOwnerUid
	root.Access.GID = m.fsOwnerGid
	root.Storage = m.defaultStorage.ID()

	err = m.store.CreateEntry(ctx, 0, root)
	if err != nil {
		m.logger.Errorw("create root entry failed", "err", err)
		return nil, err
	}
	return root, nil
}

func (m *manager) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryMetadata").End()
	if externalIDPrefix == id>>entryIDPrefixMask {
		stubEn, err := m.extIndexer.GetStubEntry(id)
		if err != nil {
			m.logger.Warnw("query external entry with id failed", "entry", id, "err", err)
			return nil, err
		}
		return stubEn.toEntry(), nil
	}

	en, err := m.store.GetEntry(ctx, id)
	if err != nil {
		return nil, err
	}

	if en.Storage == externalStorage {
		if err = m.registerStubRoot(ctx, en); err != nil {
			m.logger.Errorw("fetching ext entry but register stub root failed", "entry", en.ID, "err", err)
		}
		return en, nil
	}
	return en, nil
}

func (m *manager) GetEntryByUri(ctx context.Context, uri string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryUri").End()
	if uri == "/" {
		return m.store.GetEntry(ctx, RootEntryID)
	}
	uri = strings.TrimSuffix(uri, "/")
	entryUri, err := m.store.GetEntryUri(ctx, uri)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			return nil, err
		}
		parent, base := path.Split(uri)
		parentEntry, err := m.GetEntryByUri(ctx, parent)
		if err != nil {
			return nil, err
		}

		grp, err := m.OpenGroup(ctx, parentEntry.ID)
		if err != nil {
			return nil, err
		}

		entry, err := grp.FindEntry(ctx, base)
		if err != nil {
			return nil, err
		}

		if entry.Storage != externalStorage {
			entryUri = &types.EntryUri{ID: entry.ID, Uri: uri}
			if err = m.store.SaveEntryUri(ctx, entryUri); err != nil {
				return nil, err
			}
		}
	}
	return m.GetEntry(ctx, entryUri.ID)
}

func (m *manager) GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryExtendData").End()
	return m.store.GetEntryExtendData(ctx, id)
}

func (m *manager) UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error {
	defer trace.StartRegion(ctx, "dentry.manager.UpdateEntryExtendData").End()
	if externalIDPrefix == id>>entryIDPrefixMask {
		return types.ErrUnsupported
	}
	return m.store.UpdateEntryExtendData(ctx, id, ed)
}

func (m *manager) GetEntryExtendField(ctx context.Context, id int64, fKey string) (*string, bool, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryExtendField").End()
	ed, err := m.GetEntryExtendData(ctx, id)
	if err != nil {
		return nil, false, err
	}
	if ed.Properties.Fields == nil {
		return nil, false, nil
	}
	item, ok := ed.Properties.Fields[fKey]
	if !ok {
		return nil, false, nil
	}
	return &item.Value, item.Encoded, nil
}

func (m *manager) SetEntryExtendField(ctx context.Context, id int64, fKey, fVal string, encoded bool) error {
	defer trace.StartRegion(ctx, "dentry.manager.SetEntryExtendField").End()
	ed, err := m.GetEntryExtendData(ctx, id)
	if err != nil {
		return err
	}

	if ed.Properties.Fields == nil {
		ed.Properties.Fields = map[string]types.PropertyItem{}
	}
	ed.Properties.Fields[fKey] = types.PropertyItem{Value: fVal, Encoded: encoded}

	err = m.UpdateEntryExtendData(ctx, id, ed)
	if err != nil {
		return err
	}
	m.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeUpdate, id)
	return nil
}

func (m *manager) RemoveEntryExtendField(ctx context.Context, id int64, fKey string) error {
	defer trace.StartRegion(ctx, "dentry.manager.RemoveEntryExtendField").End()
	ed, err := m.GetEntryExtendData(ctx, id)
	if err != nil {
		return err
	}

	if ed.Properties.Fields == nil {
		ed.Properties.Fields = map[string]types.PropertyItem{}
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

func (m *manager) GetEntryLabels(ctx context.Context, id int64) (types.Labels, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryLabels").End()
	return m.store.GetEntryLabels(ctx, id)
}

func (m *manager) UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error {
	defer trace.StartRegion(ctx, "dentry.manager.UpdateEntryLabels").End()
	if externalIDPrefix == id>>entryIDPrefixMask {
		return types.ErrUnsupported
	}
	return m.store.UpdateEntryLabels(ctx, id, labels)
}

func (m *manager) CreateEntry(ctx context.Context, parentId int64, attr types.EntryAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.CreateEntry").End()
	grp, err := m.OpenGroup(ctx, parentId)
	if err != nil {
		return nil, err
	}
	en, err := grp.CreateEntry(ctx, attr)
	if err != nil {
		return nil, err
	}
	if en.Storage == externalStorage {
		return en, m.registerStubRoot(ctx, en)
	}
	return en, nil
}

func (m *manager) RemoveEntry(ctx context.Context, parentId, entryId int64) error {
	defer trace.StartRegion(ctx, "dentry.manager.RemoveEntry").End()
	parentGrp, err := m.OpenGroup(ctx, parentId)
	if err != nil {
		return err
	}

	err = parentGrp.RemoveEntry(ctx, entryId)
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) DestroyEntry(ctx context.Context, entryID int64) error {
	defer trace.StartRegion(ctx, "dentry.manager.DestroyEntry").End()

	err := m.store.DeleteRemovedEntry(ctx, entryID)
	if err != nil {
		m.logger.Errorw("destroy entry failed", "err", err)
		return err
	}
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

func (m *manager) MirrorEntry(ctx context.Context, srcId, dstParentId int64, attr types.EntryAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.MirrorEntry").End()

	src, err := m.GetEntry(ctx, srcId)
	if err != nil {
		return nil, err
	}

	parent, err := m.GetEntry(ctx, dstParentId)
	if err != nil {
		return nil, err
	}
	if src.IsGroup {
		return nil, types.ErrIsGroup
	}
	if !parent.IsGroup {
		return nil, types.ErrNoGroup
	}

	if src.Storage == externalStorage || parent.Storage == externalStorage {
		return nil, types.ErrUnsupported
	}

	if types.IsMirrored(src) {
		m.logger.Warnw("source entry is mirrored", "entry", srcId)
		return nil, fmt.Errorf("source entry is mirrored")
	}

	en, err := initMirrorEntry(src, parent, attr)
	if err != nil {
		m.logger.Errorw("create mirror object error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}

	if err = m.store.MirrorEntry(ctx, en); err != nil {
		m.logger.Errorw("update dst parent object ref count error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}
	m.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeMirror, en.ID)
	return en, nil
}

func (m *manager) ChangeEntryParent(ctx context.Context, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error {
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
		if overwriteEntry.IsGroup {
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

		if err = m.RemoveEntry(ctx, newParentId, *overwriteEntryId); err != nil {
			m.logger.Errorw("remove entry failed when overwrite old one", "err", err)
			return err
		}
		m.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeDestroy, overwriteEntry.ID)
	}

	if oldParent.Storage == externalStorage || newParent.Storage == externalStorage || oldParent.Storage != newParent.Storage {
		err = m.changeEntryParentByFileCopy(ctx, target, oldParent, newParent, newName, opt)
		if err != nil {
			m.logger.Errorw("change entry parent by file copy failed", "err", err)
			return err
		}
		m.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeChangeParent, target.ID)
		return nil
	}

	err = m.store.ChangeEntryParent(ctx, targetEntryId, newParentId, newName, opt)
	if err != nil {
		m.logger.Errorw("change object parent failed", "entry", target.ID, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	m.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeChangeParent, target.ID)
	return nil
}

func (m *manager) changeEntryParentByFileCopy(ctx context.Context, targetEntry, oldParent, newParent *types.Metadata, newName string, _ types.ChangeParentAttr) error {
	newParentEd, err := m.GetEntryExtendData(ctx, newParent.ID)
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, query new parent extend data failed", "err", err)
		return err
	}

	if targetEntry.IsGroup {
		if oldParent.ID == newParent.ID {
			// only rename
			targetEntry.Name = newName
			err = m.store.UpdateEntryMetadata(ctx, targetEntry)
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
	attr := types.EntryAttr{
		Name:      newName,
		Kind:      targetEntry.Kind,
		Access:    &targetEntry.Access,
		PlugScope: newParentEd.PlugScope,
	}
	en, err := m.CreateEntry(ctx, newParent.ID, attr)
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, create new entry failed", "err", err)
		return err
	}

	// step 2: copy old to new file
	oldFileReader, err := m.Open(ctx, targetEntry.ID, types.OpenAttr{Read: true})
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, open old file failed", "err", err)
		return err
	}
	newFileWriter, err := m.Open(ctx, en.ID, types.OpenAttr{Write: true})
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

func (m *manager) Open(ctx context.Context, entryId int64, attr types.OpenAttr) (f File, err error) {
	defer trace.StartRegion(ctx, "dentry.manager.Open").End()
	attr.EntryID = entryId

	if externalIDPrefix == entryId>>entryIDPrefixMask {
		stubEntry, err := m.extIndexer.GetStubEntry(entryId)
		if err != nil {
			return nil, err
		}
		f, err = openExternalFile(ctx, stubEntry, stubEntry.root.mirror, attr)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	entry, err := m.store.Open(ctx, entryId, attr)
	if err != nil {
		return nil, err
	}
	if attr.Trunc {
		if err := m.CleanEntryData(ctx, entryId); err != nil {
			m.logger.Errorw("clean entry with trunc error", "entry", entryId, "err", err)
		}
		m.publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeTrunc, entryId)
	}

	switch entry.Kind {
	case types.SymLinkKind:
		f, err = openSymlink(m, entry, attr)
	default:
		attr.FsWriteback = m.fsWriteback
		f, err = openFile(entry, attr, m.metastore, m.storages[entry.Storage])
	}
	if err != nil {
		return nil, err
	}
	m.publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeOpen, entryId)
	return instrumentalFile{file: f}, nil
}

func (m *manager) OpenGroup(ctx context.Context, groupId int64) (Group, error) {
	defer trace.StartRegion(ctx, "dentry.manager.OpenGroup").End()
	entry, err := m.GetEntry(ctx, groupId)
	if err != nil {
		return nil, err
	}
	if !entry.IsGroup {
		return nil, types.ErrNoGroup
	}
	var (
		stdGrp       = &stdGroup{entryID: entry.ID, name: entry.Name, mgr: m, store: m.store}
		grp    Group = stdGrp
	)
	switch entry.Kind {
	case types.SmartGroupKind:
		ed, err := m.GetEntryExtendData(ctx, groupId)
		if err != nil {
			m.logger.Errorw("query dynamic group extend data failed", "err", err)
			return nil, err
		}
		if ed.GroupFilter != nil {
			grp = &dynamicGroup{
				std:       stdGrp,
				rule:      *ed.GroupFilter,
				baseEntry: groupId,
				logger:    logger.NewLogger("dynamicGroup").With(zap.Int64("group", groupId)),
			}
		} else {
			m.logger.Warnw("dynamic group not filter config", "entry", entry.ID)
			grp = emptyGroup{}
		}
	case types.ExternalGroupKind:
		stubEntry, _ := m.extIndexer.GetStubEntry(groupId)
		if stubEntry != nil {
			grp = &extGroup{
				entry:  stubEntry,
				mirror: stubEntry.root.mirror,
				logger: logger.NewLogger("extGroup").With(zap.Int64("group", groupId)),
			}
		} else {
			m.logger.Warnw("external group not indexed", "entry", entry.ID)
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

func (m *manager) getOrSaveEntryUri(ctx context.Context, entry *types.Metadata) (uri string, err error) {
	if entry.ID == RootEntryID {
		return "/", nil
	}
	var (
		entryUri    = &types.EntryUri{}
		parentEntry = &types.Metadata{}
		parentUri   = ""
	)
	entryUri, err = m.store.GetEntryUriById(ctx, entry.ID)
	if err != nil {
		if err == types.ErrNotFound {
			parentEntry, err = m.store.GetEntry(ctx, entry.ParentID)
			if err != nil {
				return
			}
			parentUri, err = m.getOrSaveEntryUri(ctx, parentEntry)
			if err != nil {
				return
			}

			uri = path.Join(parentUri, entry.Name)
			err = m.store.SaveEntryUri(ctx, &types.EntryUri{ID: entry.ID, Uri: uri})
			return
		}
		return
	}
	uri = entryUri.Uri
	return
}

func (m *manager) registerStubRoot(ctx context.Context, en *types.Metadata) error {
	stubEntry, _ := m.extIndexer.GetStubEntry(en.ID)
	if stubEntry != nil {
		return nil
	}

	ed, err := m.store.GetEntryExtendData(ctx, en.ID)
	if err != nil {
		return err
	}
	if ed.PlugScope != nil {
		mirror, err := plugin.NewMirrorPlugin(ctx, *ed.PlugScope)
		if err != nil {
			return err
		}
		m.extIndexer.AddStubRoot(en, mirror)
	} else {
		m.logger.Errorw("external root has no plug scope define")
	}
	return nil
}
