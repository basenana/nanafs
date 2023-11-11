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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"runtime/trace"
)

type Manager interface {
	Root(ctx context.Context) (*types.Metadata, error)

	GetEntry(ctx context.Context, id int64) (*types.Metadata, error)
	CreateEntry(ctx context.Context, parentId int64, attr types.EntryAttr) (*types.Metadata, error)
	RemoveEntry(ctx context.Context, parentId, entryId int64) error
	DestroyEntry(ctx context.Context, entryId int64) error
	CleanEntryData(ctx context.Context, entryId int64) error
	MirrorEntry(ctx context.Context, srcId, dstParentId int64, attr types.EntryAttr) (*types.Metadata, error)
	ChangeEntryParent(ctx context.Context, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error

	GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error)
	UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error
	GetEntryExtendField(ctx context.Context, id int64, fKey string) (*string, error)
	SetEntryExtendField(ctx context.Context, id int64, fKey, fVal string) error
	RemoveEntryExtendField(ctx context.Context, id int64, fKey string) error
	GetEntryLabels(ctx context.Context, id int64) (types.Labels, error)
	UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error

	Open(ctx context.Context, entryId int64, attr types.OpenAttr) (File, error)
	OpenGroup(ctx context.Context, entryID int64) (Group, error)
	ChunkCompact(ctx context.Context, entryId int64) error

	MustCloseAll()
}

func NewManager(store metastore.Meta, cfg config.Config) (Manager, error) {
	storages := make(map[string]storage.Storage)
	var err error
	for i := range cfg.Storages {
		storages[cfg.Storages[i].ID], err = storage.NewStorage(cfg.Storages[i].ID, cfg.Storages[i].Type, cfg.Storages[i])
		if err != nil {
			return nil, err
		}
	}
	mgr := &manager{
		store:     store,
		metastore: store,
		storages:  storages,
		cfg:       cfg,
		logger:    logger.NewLogger("entryManager"),
	}
	fileEntryLogger = mgr.logger.Named("files")
	return mgr, nil
}

type manager struct {
	store     metastore.DEntry
	metastore metastore.Meta
	storages  map[string]storage.Storage
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
	root = initRootEntry()
	root.Access.UID = m.cfg.FS.Owner.Uid
	root.Access.GID = m.cfg.FS.Owner.Gid
	root.Storage = m.cfg.Storages[0].ID
	err = m.store.CreateEntry(ctx, 0, root)
	if err != nil {
		m.logger.Errorw("create root entry failed", "err", err)
		return nil, err
	}
	return root, nil
}

func (m *manager) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryMetadata").End()
	return m.store.GetEntry(ctx, id)
}

func (m *manager) GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryExtendData").End()
	return m.store.GetEntryExtendData(ctx, id)
}

func (m *manager) UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error {
	defer trace.StartRegion(ctx, "dentry.manager.UpdateEntryExtendData").End()
	return m.store.UpdateEntryExtendData(ctx, id, ed)
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

func (m *manager) GetEntryLabels(ctx context.Context, id int64) (types.Labels, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryLabels").End()
	return m.store.GetEntryLabels(ctx, id)
}

func (m *manager) UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error {
	defer trace.StartRegion(ctx, "dentry.manager.UpdateEntryLabels").End()
	return m.store.UpdateEntryLabels(ctx, id, labels)
}

func (m *manager) CreateEntry(ctx context.Context, parentId int64, attr types.EntryAttr) (*types.Metadata, error) {
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

	en, err := initMirrorEntry(src, parent, attr)
	if err != nil {
		m.logger.Errorw("create mirror object error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}

	if err = m.store.MirrorEntry(ctx, en); err != nil {
		m.logger.Errorw("update dst parent object ref count error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}

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

		if err = m.RemoveEntry(ctx, newParentId, *overwriteEntryId); err != nil {
			m.logger.Errorw("remove entry failed when overwrite old one", "err", err)
			return err
		}
		PublicEntryActionEvent(events.ActionTypeDestroy, overwriteEntry)
	}

	if oldParent.Storage == externalStorage || newParent.Storage == externalStorage || oldParent.Storage != newParent.Storage {
		return m.changeEntryParentByFileCopy(ctx, target, oldParent, newParent, newName, opt)
	}

	err = m.store.ChangeEntryParent(ctx, targetEntryId, newParentId, newName, opt)
	if err != nil {
		m.logger.Errorw("change object parent failed", "entry", target.ID, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	return nil
}

func (m *manager) changeEntryParentByFileCopy(ctx context.Context, targetEntry, oldParent, newParent *types.Metadata, newName string, _ types.ChangeParentAttr) error {
	newParentEd, err := m.GetEntryExtendData(ctx, newParent.ID)
	if err != nil {
		m.logger.Errorw("change entry parent by file copy error, query new parent extend data failed", "err", err)
		return err
	}

	if types.IsGroup(targetEntry.Kind) {
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
		Access:    targetEntry.Access,
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

func (m *manager) Open(ctx context.Context, entryId int64, attr types.OpenAttr) (File, error) {
	defer trace.StartRegion(ctx, "dentry.manager.Open").End()
	attr.EntryID = entryId
	entry, err := m.store.Open(ctx, entryId, attr)
	if err != nil {
		return nil, err
	}
	if attr.Trunc && entry.Storage != externalStorage {
		if err := m.CleanEntryData(ctx, entryId); err != nil {
			m.logger.Errorw("clean entry with trunc error", "entry", entryId, "err", err)
		}
		PublicFileActionEvent(events.ActionTypeTrunc, entry)
	}

	var f File
	if entry.Storage == externalStorage {
		var ed types.ExtendData
		ed, err = m.GetEntryExtendData(ctx, entryId)
		if err != nil {
			m.logger.Errorw("get entry extend data failed", "entry", entryId, "err", err)
			return nil, err
		}
		f, err = openExternalFile(entry, ed.PlugScope, attr, m.store, m.cfg.FS)
	} else {
		switch entry.Kind {
		case types.SymLinkKind:
			f, err = openSymlink(m, entry, attr)
		default:
			f, err = openFile(entry, attr, m.metastore, m.storages[entry.Storage], m.cfg.FS)
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
		stdGrp       = &stdGroup{entryID: entry.ID, name: entry.Name, store: m.store}
		grp    Group = stdGrp
	)
	switch entry.Kind {
	case types.SmartGroupKind:
		grp = &dynamicGroup{stdGroup: stdGrp}
	case types.ExternalGroupKind:
		var ed types.ExtendData
		if ed, err = m.store.GetEntryExtendData(ctx, entry.ID); err != nil {
			return nil, err
		}
		if ed.PlugScope != nil {
			mirror, err := plugin.NewMirrorPlugin(ctx, *ed.PlugScope)
			if err != nil {
				return nil, err
			}
			grp = &extGroup{mgr: m, stdGroup: stdGrp, mirror: mirror,
				logger: logger.NewLogger("extLogger").With(zap.Int64("group", groupId))}
		} else {
			m.logger.Warnw("external group without plug scope", "entry", entry.ID)
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
