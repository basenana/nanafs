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

package core

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"runtime/trace"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Core interface {
	FSRoot(ctx context.Context) (*types.Entry, error)
	NamespaceRoot(ctx context.Context, namespace string) (*types.Entry, error)
	CreateNamespace(ctx context.Context, namespace string) error

	GetEntry(ctx context.Context, namespace string, id int64) (*types.Entry, error)
	CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error)
	RemoveEntry(ctx context.Context, namespace string, parentId, entryId int64) error
	MirrorEntry(ctx context.Context, namespace string, srcId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error)
	ChangeEntryParent(ctx context.Context, namespace string, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error

	Open(ctx context.Context, namespace string, entryId int64, attr types.OpenAttr) (File, error)
	OpenGroup(ctx context.Context, namespace string, entryID int64) (Group, error)

	DestroyEntry(ctx context.Context, entryId int64) error
	CleanEntryData(ctx context.Context, entryId int64) error
	ChunkCompact(ctx context.Context, entryId int64) error
}

func New(store metastore.Meta, cfg config.Bootstrap) (Core, error) {
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

	c := &core{
		store:          store,
		metastore:      store,
		defaultStorage: defaultStorage,
		storages:       storages,
		fsOwnerUid:     cfg.FS.Owner.Uid,
		fsOwnerGid:     cfg.FS.Owner.Gid,
		fsWriteback:    cfg.FS.Writeback,
		logger:         logger.NewLogger("fsCore"),
	}

	go c.entryActionEventHandler()
	fileEntryLogger = c.logger.Named("files")
	return c, err
}

type core struct {
	store          metastore.EntryStore
	metastore      metastore.Meta
	defaultStorage storage.Storage
	storages       map[string]storage.Storage
	cfgLoader      config.Loader
	fsOwnerUid     int64
	fsOwnerGid     int64
	fsWriteback    bool
	logger         *zap.SugaredLogger
}

var _ Core = &core{}

func (c *core) FSRoot(ctx context.Context) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.FSRoot").End()
	var (
		root *types.Entry
		err  error
	)
	root, err = c.getEntry(ctx, RootEntryID)
	if err == nil {
		return root, nil
	}
	if !errors.Is(err, types.ErrNotFound) {
		c.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	root = initRootEntry()
	root.Access.UID = c.fsOwnerUid
	root.Access.GID = c.fsOwnerGid
	root.Storage = c.defaultStorage.ID()

	err = c.store.CreateEntry(ctx, 0, root, nil)
	if err != nil {
		c.logger.Errorw("create root entry failed", "err", err)
		return nil, err
	}
	return root, nil
}

func (c *core) NamespaceRoot(ctx context.Context, namespace string) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.NamespaceRoot").End()
	var (
		nsRoot *types.Entry
		err    error
	)
	nsRoot, err = c.store.FindEntry(ctx, RootEntryID, namespace)
	if err != nil {
		c.logger.Errorw("load ns root object error", "namespace", namespace, "err", err)
		return nsRoot, err
	}
	if nsRoot.Namespace != namespace {
		c.logger.Errorw("find ns root object error", "err", "namespace not match")
		return nil, types.ErrNotFound
	}
	return nsRoot, nil

}

func (c *core) CreateNamespace(ctx context.Context, namespace string) error {
	defer trace.StartRegion(ctx, "fs.core.CreateNamespace").End()
	root, err := c.FSRoot(ctx)
	if err != nil {
		c.logger.Errorw("load root object error", "err", err.Error())
		return err
	}
	// init root entry of namespace
	nsRoot := initNamespaceRootEntry(root, namespace)
	nsRoot.Access.UID = c.fsOwnerUid
	nsRoot.Access.GID = c.fsOwnerGid
	nsRoot.Storage = c.defaultStorage.ID()

	err = c.store.CreateEntry(ctx, RootEntryID, nsRoot, nil)
	if err != nil {
		c.logger.Errorw("create root entry failed", "err", err)
		return err
	}

	buildInGroups := []string{
		".inbox",
	}

	for _, buildInGroupName := range buildInGroups {
		_, err = c.CreateEntry(ctx, namespace, nsRoot.ID, types.EntryAttr{
			Name: buildInGroupName,
			Kind: types.GroupKind,
		})
		if err != nil {
			return fmt.Errorf("init build-in ns group %s failed: %w", buildInGroupName, err)
		}
	}
	return nil
}

func (c *core) getEntry(ctx context.Context, id int64) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.GetEntryMetadata").End()

	en, err := c.store.GetEntry(ctx, id)
	if err != nil {
		return nil, err
	}

	return en, nil
}

func (c *core) GetEntry(ctx context.Context, namespace string, id int64) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.GetEntryMetadata").End()

	en, err := c.getEntry(ctx, id)
	if err != nil {
		return nil, err
	}

	if en.Namespace != namespace {
		return nil, types.ErrNotFound
	}

	return en, nil
}

func (c *core) CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.CreateEntry").End()
	grp, err := c.OpenGroup(ctx, namespace, parentId)
	if err != nil {
		return nil, err
	}
	en, err := grp.CreateEntry(ctx, attr)
	if err != nil {
		return nil, err
	}
	return en, nil
}

func (c *core) RemoveEntry(ctx context.Context, namespace string, parentId, entryId int64) error {
	defer trace.StartRegion(ctx, "fs.core.RemoveEntry").End()
	children, err := c.store.ListEntryChildren(ctx, entryId, nil, types.Filter{})
	if err != nil {
		return err
	}

	for children.HasNext() {
		next := children.Next()
		if next.ID == next.ParentID {
			continue
		}
		if err = c.RemoveEntry(ctx, namespace, entryId, next.ID); err != nil {
			return err
		}
	}

	parentGrp, err := c.OpenGroup(ctx, namespace, parentId)
	if err != nil {
		return err
	}

	err = parentGrp.RemoveEntry(ctx, entryId)
	if err != nil {
		return err
	}
	return nil
}

func (c *core) DestroyEntry(ctx context.Context, entryID int64) error {
	defer trace.StartRegion(ctx, "fs.core.DestroyEntry").End()

	err := c.store.DeleteRemovedEntry(ctx, entryID)
	if err != nil {
		c.logger.Errorw("destroy entry failed", "err", err)
		return err
	}
	return nil
}

func (c *core) CleanEntryData(ctx context.Context, entryId int64) error {
	entry, err := c.getEntry(ctx, entryId)
	if err != nil {
		return err
	}

	s, ok := c.storages[entry.Storage]
	if !ok {
		return fmt.Errorf("storage %s not register", entry.Storage)
	}

	cs, ok := c.metastore.(metastore.ChunkStore)
	if !ok {
		return nil
	}

	defer logger.CostLog(c.logger.With(zap.Int64("entry", entry.ID)), "clean entry data")()
	err = bio.DeleteChunksData(ctx, entry, cs, s)
	if err != nil {
		c.logger.Errorw("delete chunk data failed", "entry", entry.ID, "err", err)
		return err
	}
	return nil
}

func (c *core) MirrorEntry(ctx context.Context, namespace string, srcId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.MirrorEntry").End()

	src, err := c.GetEntry(ctx, namespace, srcId)
	if err != nil {
		return nil, err
	}

	parent, err := c.GetEntry(ctx, namespace, dstParentId)
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
		c.logger.Warnw("source entry is mirrored", "entry", srcId)
		return nil, fmt.Errorf("source entry is mirrored")
	}

	en, err := initMirrorEntry(src, parent, attr)
	if err != nil {
		c.logger.Errorw("create mirror object error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}

	if err = c.store.MirrorEntry(ctx, en); err != nil {
		c.logger.Errorw("update dst parent object ref count error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}
	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeMirror, en.ID)
	return en, nil
}

func (c *core) ChangeEntryParent(ctx context.Context, namespace string, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error {
	defer trace.StartRegion(ctx, "fs.core.ChangeEntryParent").End()

	target, err := c.GetEntry(ctx, namespace, targetEntryId)
	if err != nil {
		return err
	}

	// TODO delete overwrite entry on outside
	if overwriteEntryId != nil {
		overwriteEntry, err := c.GetEntry(ctx, namespace, *overwriteEntryId)
		if err != nil {
			return err
		}
		if overwriteEntry.IsGroup {
			overwriteGrp, err := c.OpenGroup(ctx, namespace, *overwriteEntryId)
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

		if err = c.RemoveEntry(ctx, namespace, newParentId, *overwriteEntryId); err != nil {
			c.logger.Errorw("remove entry failed when overwrite old one", "err", err)
			return err
		}
		publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeDestroy, overwriteEntry.ID)
	}

	err = c.store.ChangeEntryParent(ctx, targetEntryId, newParentId, newName, opt)
	if err != nil {
		c.logger.Errorw("change object parent failed", "entry", target.ID, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeChangeParent, target.ID)
	return nil
}

func (c *core) Open(ctx context.Context, namespace string, entryId int64, attr types.OpenAttr) (f File, err error) {
	defer trace.StartRegion(ctx, "fs.core.Open").End()
	attr.EntryID = entryId

	entry, err := c.store.Open(ctx, entryId, attr)
	if err != nil {
		return nil, err
	}
	if attr.Trunc {
		if err := c.CleanEntryData(ctx, entryId); err != nil {
			c.logger.Errorw("clean entry with trunc error", "entry", entryId, "err", err)
		}
		publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeTrunc, entryId)
	}

	switch entry.Kind {
	case types.SymLinkKind:
		f, err = openSymlink(c.metastore, entry, attr)
	default:
		attr.FsWriteback = c.fsWriteback
		f, err = openFile(entry, attr, c.metastore, c.storages[entry.Storage])
	}
	if err != nil {
		return nil, err
	}
	publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeOpen, entryId)
	return instrumentalFile{file: f}, nil
}

func (c *core) OpenGroup(ctx context.Context, namespace string, groupId int64) (Group, error) {
	defer trace.StartRegion(ctx, "fs.core.OpenGroup").End()
	entry, err := c.GetEntry(ctx, namespace, groupId)
	if err != nil {
		return nil, err
	}
	if !entry.IsGroup {
		return nil, types.ErrNoGroup
	}
	var (
		stdGrp       = &stdGroup{entryID: entry.ID, name: entry.Name, namespace: namespace, store: c.store}
		grp    Group = stdGrp
	)
	switch entry.Kind {
	case types.SmartGroupKind:
		ed, err := c.store.GetEntryExtendData(ctx, groupId)
		if err != nil {
			c.logger.Errorw("query dynamic group extend data failed", "err", err)
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
			c.logger.Warnw("dynamic group not filter config", "entry", entry.ID)
			grp = emptyGroup{}
		}
	}
	return instrumentalGroup{grp: grp}, nil
}

func (c *core) ChunkCompact(ctx context.Context, entryId int64) error {
	defer trace.StartRegion(ctx, "fs.core.ChunkCompact").End()
	entry, err := c.getEntry(ctx, entryId)
	if err != nil {
		return err
	}
	chunkStore, ok := c.metastore.(metastore.ChunkStore)
	if !ok {
		return fmt.Errorf("not chunk store")
	}
	dataStorage, ok := c.storages[entry.Storage]
	if !ok {
		return fmt.Errorf("storage %s not registered", entry.Storage)
	}
	return bio.CompactChunksData(ctx, entry, chunkStore, dataStorage)
}

func MustCloseAll() {
	bio.CloseAll()
}
