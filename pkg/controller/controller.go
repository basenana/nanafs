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

package controller

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"runtime/trace"
)

const (
	entryNameMaxLength = 255
)

type Controller interface {
	LoadRootEntry(ctx context.Context) (dentry.Entry, error)
	FindEntry(ctx context.Context, parent dentry.Entry, name string) (dentry.Entry, error)
	GetEntry(ctx context.Context, id int64) (dentry.Entry, error)
	CreateEntry(ctx context.Context, parent dentry.Entry, attr types.ObjectAttr) (dentry.Entry, error)
	SaveEntry(ctx context.Context, parent, en dentry.Entry) error
	DestroyEntry(ctx context.Context, parent, en dentry.Entry, attr types.DestroyObjectAttr) error
	MirrorEntry(ctx context.Context, src, dstParent dentry.Entry, attr types.ObjectAttr) (dentry.Entry, error)
	ListEntryChildren(ctx context.Context, en dentry.Entry) ([]dentry.Entry, error)
	ChangeEntryParent(ctx context.Context, target, oldParent, newParent dentry.Entry, newName string, opt types.ChangeParentAttr) error

	OpenFile(ctx context.Context, en dentry.Entry, attr dentry.Attr) (dentry.File, error)
	ReadFile(ctx context.Context, file dentry.File, data []byte, offset int64) (n int64, err error)
	WriteFile(ctx context.Context, file dentry.File, data []byte, offset int64) (n int64, err error)
	CloseFile(ctx context.Context, file dentry.File) error

	FsInfo(ctx context.Context) Info
	StartBackendTask(stopCh chan struct{})
	SetupShutdownHandler(stopCh chan struct{}) chan struct{}
}

type controller struct {
	meta      metastore.Meta
	cfg       config.Config
	cfgLoader config.Loader

	entry dentry.Manager
	cache *entryCache

	logger *zap.SugaredLogger
}

var _ Controller = &controller{}

func (c *controller) LoadRootEntry(ctx context.Context) (dentry.Entry, error) {
	defer trace.StartRegion(ctx, "controller.LoadRootEntry").End()
	c.logger.Info("init root object")
	rootEntry, err := c.entry.Root(ctx)
	if err != nil {
		c.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	c.cache.putEntry(rootEntry)
	return rootEntry, nil
}

func (c *controller) FindEntry(ctx context.Context, parent dentry.Entry, name string) (dentry.Entry, error) {
	defer trace.StartRegion(ctx, "controller.FindEntry").End()
	if len(name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}
	if !parent.IsGroup() {
		return nil, types.ErrNoGroup
	}
	result, err := parent.Group().FindEntry(ctx, name)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("find entry error", "parent", parent.Metadata().ID, "entryName", name, "err", err.Error())
		}
		return nil, err
	}
	return result, nil
}

func (c *controller) GetEntry(ctx context.Context, id int64) (dentry.Entry, error) {
	defer trace.StartRegion(ctx, "controller.GetEntry").End()
	cached := c.cache.getEntry(id)
	if cached != nil {
		return cached, nil
	}
	result, err := c.entry.GetEntry(ctx, id)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("get entry error", "entry", id, "err", err.Error())
		}
		return nil, err
	}
	c.cache.putEntry(result)
	return result, nil
}

func (c *controller) CreateEntry(ctx context.Context, parent dentry.Entry, attr types.ObjectAttr) (dentry.Entry, error) {
	defer trace.StartRegion(ctx, "controller.CreateEntry").End()

	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	if parent == nil {
		c.logger.Errorw("create entry failed, parent is nil", "entryName", attr.Name)
		return nil, types.ErrNotFound
	}

	entry, err := c.entry.CreateEntry(ctx, parent, dentry.EntryAttr{
		Name:   attr.Name,
		Dev:    attr.Dev,
		Kind:   attr.Kind,
		Access: attr.Access,
	})
	if err != nil {
		c.logger.Errorw("create entry error", "parent", parent.Metadata().ID, "entryName", attr.Name, "err", err.Error())
		return nil, err
	}
	c.cache.putEntry(entry)
	dentry.PublicEntryActionEvent(events.ActionTypeCreate, entry)
	return entry, nil
}

func (c *controller) SaveEntry(ctx context.Context, parent, entry dentry.Entry) error {
	defer trace.StartRegion(ctx, "controller.SaveEntry").End()
	var err error
	if parent == nil {
		parent, err = c.GetEntry(ctx, entry.Metadata().ParentID)
		if err != nil {
			c.logger.Errorw("save entry error: query parent entry failed", "entry", entry.Metadata().ID, "err", err.Error())
			return err
		}
	}

	if !parent.IsGroup() {
		return types.ErrNoGroup
	}
	if err = parent.Group().UpdateEntry(ctx, entry); err != nil {
		c.logger.Errorw("save entry error", "entry", entry.Metadata().ID, "err", err.Error())
		return err
	}
	c.cache.putEntry(parent)
	c.cache.putEntry(entry)
	dentry.PublicEntryActionEvent(events.ActionTypeUpdate, entry)
	return nil
}

func (c *controller) DestroyEntry(ctx context.Context, parent, en dentry.Entry, attr types.DestroyObjectAttr) (err error) {
	defer trace.StartRegion(ctx, "controller.DestroyEntry").End()
	if err = dentry.IsAccess(parent.Metadata().Access, attr.Uid, attr.Gid, 0x2); err != nil {
		return types.ErrNoAccess
	}
	if attr.Uid != 0 && attr.Uid != en.Metadata().Access.UID && attr.Uid != parent.Metadata().Access.UID && parent.Metadata().Access.HasPerm(types.PermSticky) {
		return types.ErrNoAccess
	}

	err = c.entry.RemoveEntry(ctx, parent, en)
	if err != nil {
		c.logger.Errorw("delete entry failed", "entry", en.Metadata().ID, "err", err.Error())
		return err
	}
	if en.IsMirror() {
		c.cache.delEntry(en.Metadata().RefID)
	}
	c.cache.putEntry(parent)
	c.cache.delEntry(en.Metadata().ID)
	dentry.PublicEntryActionEvent(events.ActionTypeDestroy, en)
	return
}

func (c *controller) MirrorEntry(ctx context.Context, src, dstParent dentry.Entry, attr types.ObjectAttr) (dentry.Entry, error) {
	defer trace.StartRegion(ctx, "controller.MirrorEntry").End()
	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	oldEntry, err := c.FindEntry(ctx, dstParent, attr.Name)
	if err != nil && err != types.ErrNotFound {
		c.logger.Errorw("check entry error", "srcEntry", src.Metadata().ID, "err", err.Error())
		return nil, err
	}
	if oldEntry != nil {
		return nil, types.ErrIsExist
	}

	entry, err := c.entry.MirrorEntry(ctx, src, dstParent, dentry.EntryAttr{
		Name:   attr.Name,
		Dev:    attr.Dev,
		Kind:   attr.Kind,
		Access: attr.Access,
	})

	c.cache.delEntry(src.Metadata().ID)
	c.cache.delEntry(entry.Metadata().RefID)
	c.cache.delEntry(dstParent.Metadata().ID)

	events.Publish(events.EntryActionTopic(events.TopicEntryActionFmt, events.ActionTypeMirror),
		dentry.BuildEntryEvent(events.ActionTypeMirror, entry))
	return entry, nil
}

func (c *controller) ListEntryChildren(ctx context.Context, parent dentry.Entry) ([]dentry.Entry, error) {
	defer trace.StartRegion(ctx, "controller.ListEntryChildren").End()
	if !parent.IsGroup() {
		return nil, types.ErrNoGroup
	}
	result, err := parent.Group().ListChildren(ctx)
	if err != nil {
		c.logger.Errorw("list entry children failed", "parent", parent.Metadata().ID, "err", err.Error())
		return nil, err
	}
	return result, err
}

func (c *controller) ChangeEntryParent(ctx context.Context, target, oldParent, newParent dentry.Entry, newName string, opt types.ChangeParentAttr) (err error) {
	defer trace.StartRegion(ctx, "controller.ChangeEntryParent").End()
	if len(newName) > entryNameMaxLength {
		return types.ErrNameTooLong
	}

	// need source dir WRITE
	if err = dentry.IsAccess(oldParent.Metadata().Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}
	// need dst dir WRITE
	if err = dentry.IsAccess(newParent.Metadata().Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}

	if opt.Uid != 0 && opt.Uid != oldParent.Metadata().Access.UID && opt.Uid != target.Metadata().Access.UID && oldParent.Metadata().Access.HasPerm(types.PermSticky) {
		return types.ErrNoPerm
	}

	existObj, err := c.FindEntry(ctx, newParent, newName)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("new name verify failed", "old", target.Metadata().ID, "newParent", newParent.Metadata().ID, "newName", newName, "err", err.Error())
			return err
		}
	}
	if existObj != nil {
		if opt.Uid != 0 && opt.Uid != newParent.Metadata().Access.UID && opt.Uid != existObj.Metadata().Access.UID && newParent.Metadata().Access.HasPerm(types.PermSticky) {
			return types.ErrNoPerm
		}
	}

	err = c.entry.ChangeEntryParent(ctx, target, existObj, oldParent, newParent, newName, dentry.ChangeParentAttr{
		Replace:  opt.Replace,
		Exchange: opt.Exchange,
	})
	if err != nil {
		c.logger.Errorw("change object parent failed", "target", target.Metadata().ID, "newParent", newParent.Metadata().ID, "newName", newName, "err", err.Error())
		return err
	}
	if existObj != nil {
		c.cache.delEntry(existObj.Metadata().ID)
	}
	c.cache.putEntry(target)
	c.cache.putEntry(oldParent)
	c.cache.putEntry(newParent)
	dentry.PublicEntryActionEvent(events.ActionTypeChangeParent, target)
	return nil
}

func New(loader config.Loader, meta metastore.Meta) (Controller, error) {
	cfg, _ := loader.GetConfig()

	ctl := &controller{
		meta:      meta,
		cfg:       cfg,
		cfgLoader: loader,
		cache:     initEntryCache(),
		logger:    logger.NewLogger("controller"),
	}
	var err error
	ctl.entry, err = dentry.NewManager(meta, cfg)
	if err != nil {
		return nil, err
	}
	ctl.entry.SetCacheResetter(ctl.cache)
	return ctl, nil
}
