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
	"path"
	"runtime/trace"
	"strings"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	Root(ctx context.Context) (*types.Metadata, error)
	CreateNamespace(ctx context.Context, namespace *types.Namespace) error

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

	ListEntryProperty(ctx context.Context, id int64) (types.Properties, error)
	GetEntryProperty(ctx context.Context, id int64, fKey string) (*string, bool, error)
	SetEntryProperty(ctx context.Context, id int64, fKey, fVal string, encoded bool) error
	RemoveEntryProperty(ctx context.Context, id int64, fKey string) error
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
	var (
		root   *types.Metadata
		nsRoot *types.Metadata
		err    error
	)
	ns := types.GetNamespace(ctx)
	if ns.String() != types.DefaultNamespaceValue {
		nsRoot, err = m.store.FindEntry(ctx, RootEntryID, ns.String())
		if err != nil {
			m.logger.Errorw("load ns root object error", "namespace", ns.String(), "err", err)
			return nsRoot, err
		}
		if nsRoot.Namespace != ns.String() {
			m.logger.Errorw("find ns root object error", "err", "namespace not match")
			return nil, types.ErrNotFound
		}
		return nsRoot, nil
	}

	root, err = m.GetEntry(ctx, RootEntryID)
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

	err = m.store.CreateEntry(ctx, 0, root, nil)
	if err != nil {
		m.logger.Errorw("create root entry failed", "err", err)
		return nil, err
	}
	return root, nil
}

func (m *manager) CreateNamespace(ctx context.Context, namespace *types.Namespace) error {
	defer trace.StartRegion(ctx, "dentry.manager.CreateNamespace").End()
	root, err := m.Root(ctx)
	if err != nil {
		m.logger.Errorw("load root object error", "err", err.Error())
		return err
	}
	// init root entry of namespace
	nsRoot := initNamespaceRootEntry(root, namespace)
	nsRoot.Access.UID = m.fsOwnerUid
	nsRoot.Access.GID = m.fsOwnerGid
	nsRoot.Storage = m.defaultStorage.ID()

	err = m.store.CreateEntry(ctx, RootEntryID, nsRoot, nil)
	if err != nil {
		m.logger.Errorw("create root entry failed", "err", err)
		return err
	}
	return nil
}

func (m *manager) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryMetadata").End()

	en, err := m.store.GetEntry(ctx, id)
	if err != nil {
		return nil, err
	}

	return en, nil
}

func (m *manager) GetEntryByUri(ctx context.Context, uri string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryUri").End()
	if uri == "/" {
		return m.Root(ctx)
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

		entryUri = &types.EntryUri{ID: entry.ID, Namespace: entry.Namespace, Uri: uri}
		if err = m.store.SaveEntryUri(ctx, entryUri); err != nil {
			return nil, err
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
	return m.store.UpdateEntryExtendData(ctx, id, ed)
}

func (m *manager) ListEntryProperty(ctx context.Context, id int64) (types.Properties, error) {
	defer trace.StartRegion(ctx, "dentry.manager.ListEntryProperty").End()
	return m.store.ListEntryProperties(ctx, id)
}

func (m *manager) GetEntryProperty(ctx context.Context, id int64, fKey string) (*string, bool, error) {
	defer trace.StartRegion(ctx, "dentry.manager.GetEntryProperty").End()
	p, err := m.store.GetEntryProperty(ctx, id, fKey)
	if err != nil {
		return nil, false, err
	}
	return &p.Value, p.Encoded, nil
}

func (m *manager) SetEntryProperty(ctx context.Context, id int64, fKey, fVal string, encoded bool) error {
	defer trace.StartRegion(ctx, "dentry.manager.SetEntryProperty").End()
	if err := m.store.AddEntryProperty(ctx, id, fKey, types.PropertyItem{Value: fVal, Encoded: encoded}); err != nil {
		return err
	}
	m.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeUpdate, id)
	return nil
}

func (m *manager) RemoveEntryProperty(ctx context.Context, id int64, fKey string) error {
	defer trace.StartRegion(ctx, "dentry.manager.RemoveEntryProperty").End()

	if err := m.store.RemoveEntryProperty(ctx, id, fKey); err != nil {
		return err
	}
	m.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeUpdate, id)
	return nil
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
	en, err := grp.CreateEntry(ctx, attr)
	if err != nil {
		return nil, err
	}
	return en, nil
}

func (m *manager) RemoveEntry(ctx context.Context, parentId, entryId int64) error {
	defer trace.StartRegion(ctx, "dentry.manager.RemoveEntry").End()
	children, err := m.store.ListEntryChildren(ctx, entryId, nil, types.Filter{})
	if err != nil {
		return err
	}

	for children.HasNext() {
		next := children.Next()
		if next.ID == next.ParentID {
			continue
		}
		if err = m.RemoveEntry(ctx, entryId, next.ID); err != nil {
			return err
		}
	}

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
			children, err := overwriteGrp.ListChildren(ctx, nil, types.Filter{})
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

	err = m.store.ChangeEntryParent(ctx, targetEntryId, newParentId, newName, opt)
	if err != nil {
		m.logger.Errorw("change object parent failed", "entry", target.ID, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	m.publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeChangeParent, target.ID)
	return nil
}

func (m *manager) Open(ctx context.Context, entryId int64, attr types.OpenAttr) (f File, err error) {
	defer trace.StartRegion(ctx, "dentry.manager.Open").End()
	attr.EntryID = entryId

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
