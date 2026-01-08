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

type Core interface {
	FSRoot(ctx context.Context) (*types.Entry, error)
	NamespaceRoot(ctx context.Context, namespace string) (*types.Entry, error)
	CreateNamespace(ctx context.Context, namespace string) error

	GetEntry(ctx context.Context, namespace string, id int64) (*types.Entry, error)
	GetEntryByPath(ctx context.Context, namespace string, path string) (*types.Entry, *types.Entry, error)
	CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error)
	UpdateEntry(ctx context.Context, namespace string, id int64, update types.UpdateEntry) (*types.Entry, error)
	RemoveEntry(ctx context.Context, namespace string, parentId, entryId int64, entryName string, attr types.DeleteEntry) error

	MirrorEntry(ctx context.Context, namespace string, srcId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error)
	ChangeEntryParent(ctx context.Context, namespace string, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, oldName, newName string, opt types.ChangeParentAttr) error
	FindEntry(ctx context.Context, namespace string, parentId int64, name string) (*types.Child, error)
	ListChildren(ctx context.Context, namespace string, parentId int64) ([]*types.Child, error)
	ListParents(ctx context.Context, namespace string, childId int64) ([]*types.Child, error)

	OpenGroup(ctx context.Context, namespace string, entryID int64) (Group, error)
	Open(ctx context.Context, namespace string, entryId int64, attr types.OpenAttr) (RawFile, error)

	DestroyEntry(ctx context.Context, namespace string, entryId int64) error
	CleanEntryData(ctx context.Context, namespace string, entryId int64) error
	ChunkCompact(ctx context.Context, namespace string, entryId int64) error

	NextSegmentID(ctx context.Context) (int64, error)
	ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error)
	AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Entry, error)
	DeleteSegment(ctx context.Context, segID int64) error
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
	if len(storages) == 0 {
		return nil, errors.New("no storage configed")
	}
	defaultStorage = storages[cfg.Storages[0].ID]

	c := &core{
		store:          store,
		metastore:      store,
		defaultStorage: defaultStorage,
		storages:       storages,
		cache:          newCache(),
		fsOwnerUid:     cfg.FS.Owner.Uid,
		fsOwnerGid:     cfg.FS.Owner.Gid,
		fsWriteback:    cfg.FS.Writeback,
		logger:         logger.NewLogger("fsCore"),
	}

	bio.InitPageCache(cfg.FS.PageSize)
	storage.InitLocalCache(cfg)

	go c.entryActionEventHandler()
	fileEntryLogger = c.logger.Named("files")
	return c, nil
}

type core struct {
	store          metastore.EntryStore
	metastore      metastore.Meta
	defaultStorage storage.Storage
	storages       map[string]storage.Storage
	cfg            config.Config
	fsOwnerUid     int64
	fsOwnerGid     int64
	fsWriteback    bool
	cache          *cache
	root           *types.Entry
	logger         *zap.SugaredLogger
}

var _ Core = &core{}

func (c *core) FSRoot(ctx context.Context) (*types.Entry, error) {
	if c.root != nil {
		return c.root, nil
	}

	defer trace.StartRegion(ctx, "fs.core.FSRoot").End()
	var (
		root *types.Entry
		err  error
	)
	root, err = c.getEntry(ctx, types.DefaultNamespace, RootEntryID)
	if err == nil {
		c.root = root
		return c.root, nil
	}

	if !errors.Is(err, types.ErrNotFound) {
		c.logger.Errorw("load root entry error", "err", err.Error())
		return nil, err
	}
	root = initRootEntry()
	root.Access.UID = c.fsOwnerUid
	root.Access.GID = c.fsOwnerGid
	root.Storage = c.defaultStorage.ID()

	err = c.store.CreateEntry(ctx, types.DefaultNamespace, 0, root)
	if err != nil {
		c.logger.Errorw("create root entry failed", "err", err)
		return nil, err
	}
	c.root = root
	return c.root, nil
}

func (c *core) NamespaceRoot(ctx context.Context, namespace string) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.NamespaceRoot").End()

	root, err := c.FSRoot(ctx)
	if err != nil {
		c.logger.Errorw("get fs root error", "err", err)
		return nil, err
	}

	var (
		nsChild *types.Child
		nsRoot  *types.Entry
	)
	nsChild, err = c.store.FindEntry(ctx, namespace, root.ID, namespace)
	if err != nil {
		c.logger.Errorw("load ns child entry error", "namespace", namespace, "err", err)
		return nil, err
	}

	nsRoot, err = c.getEntry(ctx, namespace, nsChild.ChildID)
	if err != nil {
		c.logger.Errorw("load ns root entry error", "namespace", namespace, "err", err)
		return nil, err
	}

	if nsRoot.Namespace != namespace {
		c.logger.Errorw("find ns root entry error", "err", "namespace not match")
		return nil, types.ErrNotFound
	}
	return nsRoot, nil
}

func (c *core) CreateNamespace(ctx context.Context, namespace string) error {
	defer trace.StartRegion(ctx, "fs.core.CreateNamespace").End()

	_, err := c.NamespaceRoot(ctx, namespace)
	if err == nil {
		return nil
	}

	root, err := c.getEntry(ctx, types.DefaultNamespace, RootEntryID)
	if err != nil {
		c.logger.Errorw("load root entry error", "err", err.Error())
		return err
	}

	// init root entry of namespace
	nsRoot := initNamespaceRootEntry(root, namespace)
	nsRoot.Access.UID = c.fsOwnerUid
	nsRoot.Access.GID = c.fsOwnerGid
	nsRoot.Storage = c.defaultStorage.ID()

	err = c.store.CreateEntry(ctx, namespace, RootEntryID, nsRoot)
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

func (c *core) getEntry(ctx context.Context, namespace string, id int64) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.GetEntryMetadata").End()

	cached, err := c.cache.getEntry(namespace, id)
	if err == nil {
		return cached, nil
	}

	en, err := c.store.GetEntry(ctx, namespace, id)
	if err != nil {
		return nil, err
	}

	if en.Namespace != namespace {
		return nil, types.ErrNotFound
	}

	c.cache.setEntry(en)
	return en, nil
}

func (c *core) GetEntry(ctx context.Context, namespace string, id int64) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.GetEntryMetadata").End()

	en, err := c.getEntry(ctx, namespace, id)
	if err != nil {
		return nil, err
	}

	if en.RefCount == 0 {
		return nil, types.ErrNotFound
	}

	return en, nil
}

func (c *core) GetEntryByPath(ctx context.Context, namespace string, path string) (*types.Entry, *types.Entry, error) {
	var (
		crt, parent *types.Entry
		err         error
	)
	parent, err = c.NamespaceRoot(ctx, namespace)
	if err != nil {
		return nil, nil, err
	}

	if path == "/" {
		return parent, parent, nil
	}

	entries := strings.Split(path, "/")
	for _, entryName := range entries {
		if entryName == "" {
			continue
		}

		if len(entryName) > fileNameMaxLength {
			return nil, nil, types.ErrNameTooLong
		}

		if crt != nil {
			parent = crt
		}

		child, err := c.FindEntry(ctx, namespace, parent.ID, entryName)
		if err != nil {
			return nil, nil, err
		}
		crt, err = c.GetEntry(ctx, namespace, child.ChildID)
		if err != nil {
			return nil, nil, err
		}
	}

	return parent, crt, nil
}

func (c *core) CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.CreateEntry").End()
	existed, err := c.store.FindEntry(ctx, namespace, parentId, attr.Name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, err
	}
	if existed != nil {
		return nil, types.ErrIsExist
	}

	group, err := c.getEntry(ctx, namespace, parentId)
	if err != nil {
		return nil, err
	}

	entry, err := types.InitNewEntry(group, attr)
	if err != nil {
		return nil, err
	}

	err = c.store.CreateEntry(ctx, namespace, parentId, entry)
	if err != nil {
		return nil, err
	}
	defer c.cache.invalidEntry(namespace, parentId)

	if attr.Properties != nil {
		err = c.store.UpdateEntryProperties(ctx, namespace, types.PropertyTypeProperty, entry.ID, attr.Properties)
		if err != nil {
			c.logger.Errorw("update entry properties error", "err", err)
			return nil, err
		}
	}

	if entry.IsGroup && attr.GroupProperties != nil {
		err = c.store.UpdateEntryProperties(ctx, namespace, types.PropertyTypeGroupAttr, entry.ID, attr.GroupProperties)
		if err != nil {
			c.logger.Errorw("update group properties error", "err", err)
			return nil, err
		}
	}

	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeCreate, entry.Namespace, entry.ID)
	return entry, nil
}

func (c *core) UpdateEntry(ctx context.Context, namespace string, id int64, update types.UpdateEntry) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.UpdateEntry").End()

	en, err := c.getEntry(ctx, namespace, id)
	if err != nil {
		return nil, err
	}

	if update.Name != nil {
		en.Name = *update.Name
	}
	if update.Aliases != nil {
		en.Aliases = *update.Aliases
	}

	if update.Size != nil {
		en.Size = *update.Size
	}
	if update.ModifiedAt != nil {
		en.ModifiedAt = *update.ModifiedAt
	}
	if update.AccessAt != nil {
		en.AccessAt = *update.AccessAt
	}
	if update.ChangedAt != nil {
		en.ChangedAt = *update.ChangedAt
	}

	if len(update.Permissions) > 0 {
		en.Access.Permissions = update.Permissions
	}
	if update.UID != nil {
		en.Access.UID = *update.UID
	}
	if update.GID != nil {
		en.Access.GID = *update.GID
	}
	if err = c.store.UpdateEntry(ctx, namespace, en); err != nil {
		return nil, err
	}
	c.cache.invalidEntry(namespace, id)
	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeUpdate, en.Namespace, en.ID)
	return c.getEntry(ctx, namespace, id)
}

func (c *core) RemoveEntry(ctx context.Context, namespace string, parentId, entryId int64, entryName string, attr types.DeleteEntry) error {
	defer trace.StartRegion(ctx, "fs.core.RemoveEntry").End()
	children, err := c.store.ListChildren(ctx, namespace, entryId)
	if err != nil {
		return err
	}

	if len(children) > 0 && !attr.DeleteAll {
		return types.ErrNotEmpty
	}

	for _, child := range children {
		if err = c.RemoveEntry(ctx, namespace, entryId, child.ChildID, child.Name, types.DeleteEntry{DeleteAll: true}); err != nil {
			return err
		}
	}

	err = c.store.RemoveEntry(ctx, namespace, parentId, entryId, entryName, attr)
	if err != nil {
		return err
	}
	c.cache.invalidEntry(namespace, entryId, parentId)
	c.cache.invalidChild(namespace, parentId, entryName)
	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeDestroy, namespace, entryId)
	return nil
}

func (c *core) DestroyEntry(ctx context.Context, namespace string, entryId int64) error {
	defer trace.StartRegion(ctx, "fs.core.DestroyEntry").End()

	err := c.store.DeleteRemovedEntry(ctx, namespace, entryId)
	if err != nil {
		c.logger.Errorw("destroy entry failed", "err", err)
		return err
	}
	c.cache.invalidEntry(namespace, entryId)
	return nil
}

func (c *core) CleanEntryData(ctx context.Context, namespace string, entryId int64) error {
	entry, err := c.getEntry(ctx, namespace, entryId)
	if err != nil {
		return err
	}

	s, ok := c.storages[entry.Storage]
	if !ok {
		return fmt.Errorf("storage %s not register", entry.Storage)
	}

	defer logger.CostLog(c.logger.With(zap.Int64("entry", entry.ID)), "clean entry data")()
	err = bio.DeleteChunksData(ctx, entry.ID, c, s)
	if err != nil {
		c.logger.Errorw("delete chunk data failed", "entry", entry.ID, "err", err)
		return err
	}
	return nil
}

func (c *core) MirrorEntry(ctx context.Context, namespace string, srcId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.core.MirrorEntry").End()

	src, err := c.getEntry(ctx, namespace, srcId)
	if err != nil {
		return nil, err
	}
	if src.IsGroup {
		return nil, types.ErrIsGroup
	}

	parent, err := c.getEntry(ctx, namespace, dstParentId)
	if err != nil {
		return nil, err
	}
	if !parent.IsGroup {
		return nil, types.ErrNoGroup
	}

	name := src.Name
	if attr.Name != "" {
		name = attr.Name
	}

	if err = c.store.MirrorEntry(ctx, namespace, srcId, name, dstParentId, attr); err != nil {
		c.logger.Errorw("update dst parent entry ref count error", "srcEntry", srcId, "dstParent", dstParentId, "err", err.Error())
		return nil, err
	}
	c.cache.invalidEntry(namespace, srcId, dstParentId)
	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeMirror, src.Namespace, src.ID)
	return c.getEntry(ctx, namespace, src.ID)
}

func (c *core) ChangeEntryParent(ctx context.Context, namespace string, targetEntryId int64, overwriteEntryId *int64, oldParentId, newParentId int64, oldName, newName string, opt types.ChangeParentAttr) error {
	defer trace.StartRegion(ctx, "fs.core.ChangeEntryParent").End()

	target, err := c.getEntry(ctx, namespace, targetEntryId)
	if err != nil {
		return err
	}

	if newName == "" {
		newName = oldName
	}

	// TODO delete overwrite entry on outside
	if overwriteEntryId != nil {
		overwriteEntry, err := c.getEntry(ctx, namespace, *overwriteEntryId)
		if err != nil {
			return err
		}
		if overwriteEntry.IsGroup {
			overwriteGrp, err := c.OpenGroup(ctx, namespace, *overwriteEntryId)
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

		if err = c.RemoveEntry(ctx, namespace, newParentId, *overwriteEntryId, newName, types.DeleteEntry{}); err != nil {
			c.logger.Errorw("remove entry failed when overwrite old one", "err", err)
			return err
		}
		c.cache.invalidEntry(namespace, *overwriteEntryId)
		c.cache.invalidChild(namespace, newParentId, newName)
		publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeDestroy, overwriteEntry.Namespace, overwriteEntry.ID)
	}

	err = c.store.ChangeEntryParent(ctx, namespace, targetEntryId, oldParentId, newParentId, oldName, newName, opt)
	if err != nil {
		c.logger.Errorw("change entry parent failed", "entry", target.ID, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	c.cache.invalidChild(namespace, oldParentId, oldName)
	c.cache.invalidEntry(namespace, targetEntryId, newParentId, oldParentId)
	publicEntryActionEvent(events.TopicNamespaceEntry, events.ActionTypeChangeParent, target.Namespace, target.ID)
	return nil
}

func (c *core) Open(ctx context.Context, namespace string, entryId int64, attr types.OpenAttr) (f RawFile, err error) {
	defer trace.StartRegion(ctx, "fs.core.Open").End()
	attr.EntryID = entryId

	entry, err := c.store.Open(ctx, namespace, entryId, attr)
	if err != nil {
		return nil, err
	}
	if attr.Trunc {
		if err := c.CleanEntryData(ctx, namespace, entryId); err != nil {
			c.logger.Errorw("clean entry with trunc error", "entry", entryId, "err", err)
		}
		publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeTrunc, namespace, entryId)
	}
	c.cache.invalidEntry(namespace, entryId)

	switch entry.Kind {
	case types.SymLinkKind:
		f, err = openSymlink(c.metastore, entry, attr)
	default:
		attr.FsWriteback = c.fsWriteback
		f, err = openFile(entry, attr, c, c.storages[entry.Storage])
	}
	if err != nil {
		return nil, err
	}
	publicEntryActionEvent(events.TopicNamespaceFile, events.ActionTypeOpen, namespace, entryId)
	return f, nil
}

func (c *core) FindEntry(ctx context.Context, namespace string, parentId int64, name string) (*types.Child, error) {
	defer trace.StartRegion(ctx, "fs.core.ListChildren").End()
	cached, err := c.cache.findChild(namespace, parentId, name)
	if err == nil {
		return cached, nil
	}

	ch, err := c.store.FindEntry(ctx, namespace, parentId, name)
	if err != nil {
		return nil, err
	}
	c.cache.setChild(ch)
	return ch, nil
}

func (c *core) ListChildren(ctx context.Context, namespace string, parentId int64) ([]*types.Child, error) {
	defer trace.StartRegion(ctx, "fs.core.ListChildren").End()
	return c.store.ListChildren(ctx, namespace, parentId)
}

func (c *core) ListParents(ctx context.Context, namespace string, childID int64) ([]*types.Child, error) {
	defer trace.StartRegion(ctx, "fs.core.ListParents").End()
	return c.store.ListParents(ctx, namespace, childID)
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
		stdGrp       = &stdGroup{entryID: entry.ID, name: entry.Name, namespace: namespace, core: c, store: c.store}
		grp    Group = stdGrp
	)

	switch entry.Kind {
	case types.SmartGroupKind:
		gattr := &types.GroupProperties{}
		err = c.store.GetEntryProperties(ctx, namespace, types.PropertyTypeGroupAttr, groupId, gattr)
		if err != nil {
			c.logger.Errorw("query dynamic group extend data failed", "err", err)
			return nil, err
		}

		if gattr.Filter != nil {
			grp = &dynamicGroup{
				std:       stdGrp,
				filter:    *gattr.Filter,
				baseEntry: groupId,
				logger:    logger.NewLogger("dynamicGroup").With(zap.Int64("group", groupId)),
			}
		} else {
			c.logger.Warnw("dynamic group not filter config", "entry", entry.ID)
			grp = emptyGroup{}
		}
	}
	return grp, nil
}

func (c *core) ChunkCompact(ctx context.Context, namespace string, entryId int64) error {
	defer trace.StartRegion(ctx, "fs.core.ChunkCompact").End()
	entry, err := c.getEntry(ctx, namespace, entryId)
	if err != nil {
		return err
	}
	dataStorage, ok := c.storages[entry.Storage]
	if !ok {
		return fmt.Errorf("storage %s not registered", entry.Storage)
	}
	return bio.CompactChunksData(ctx, entry.ID, entry.Size, c, dataStorage)
}

func (c *core) NextSegmentID(ctx context.Context) (int64, error) {
	return c.store.NextSegmentID(ctx)
}

func (c *core) ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error) {
	return c.store.ListSegments(ctx, oid, chunkID, allChunk)
}

func (c *core) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Entry, error) {
	en, err := c.store.AppendSegments(ctx, seg)
	if err != nil {
		return nil, err
	}
	c.cache.setEntry(en)
	return en, nil
}

func (c *core) DeleteSegment(ctx context.Context, segID int64) error {
	return c.store.DeleteSegment(ctx, segID)
}

// FIXME: call this before shutdown
func MustCloseAll() {
	bio.CloseAll()
}
