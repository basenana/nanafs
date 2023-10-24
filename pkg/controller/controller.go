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
	"runtime/trace"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	entryNameMaxLength = 255
)

type Controller interface {
	LoadRootEntry(ctx context.Context) (*types.Metadata, error)
	FindEntry(ctx context.Context, parentId int64, name string) (*types.Metadata, error)
	GetEntry(ctx context.Context, id int64) (*types.Metadata, error)
	CreateEntry(ctx context.Context, parentId int64, attr types.ObjectAttr) (*types.Metadata, error)
	UpdateEntry(ctx context.Context, entryId int64, patch *types.Metadata) error
	DestroyEntry(ctx context.Context, parentId, entryId int64, attr types.DestroyObjectAttr) error
	MirrorEntry(ctx context.Context, srcEntryId, dstParentId int64, attr types.ObjectAttr) (*types.Metadata, error)
	ListEntryChildren(ctx context.Context, entryId int64) ([]*types.Metadata, error)
	ChangeEntryParent(ctx context.Context, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error

	GetEntryExtendField(ctx context.Context, id int64, fKey string) (*string, error)
	SetEntryExtendField(ctx context.Context, id int64, fKey, fVal string) error
	RemoveEntryExtendField(ctx context.Context, id int64, fKey string) error

	OpenFile(ctx context.Context, entryId int64, attr dentry.Attr) (dentry.File, error)
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

	entry    dentry.Manager
	notify   *notify.Notify
	workflow workflow.Manager
	document document.Manager

	logger *zap.SugaredLogger
}

var _ Controller = &controller{}

func (c *controller) LoadRootEntry(ctx context.Context) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "controller.LoadRootEntry").End()
	c.logger.Info("init root object")
	rootEntry, err := c.entry.Root(ctx)
	if err != nil {
		c.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	return rootEntry, nil
}

func (c *controller) FindEntry(ctx context.Context, parentId int64, name string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "controller.FindEntry").End()
	if len(name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}
	group, err := c.entry.OpenGroup(ctx, parentId)
	if err != nil {
		return nil, err
	}
	result, err := group.FindEntry(ctx, name)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("find entry error", "parent", parentId, "entryName", name, "err", err.Error())
		}
		return nil, err
	}
	return result, nil
}

func (c *controller) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "controller.GetEntry").End()
	result, err := c.entry.GetEntry(ctx, id)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("get entry error", "entry", id, "err", err.Error())
		}
		return nil, err
	}
	return result, nil
}

func (c *controller) CreateEntry(ctx context.Context, parentId int64, attr types.ObjectAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "controller.CreateEntry").End()

	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	c.logger.Debugw("create entry", "parent", parentId, "entryName", attr.Name)
	entry, err := c.entry.CreateEntry(ctx, parentId, dentry.EntryAttr{
		Name:   attr.Name,
		Kind:   attr.Kind,
		Access: attr.Access,
		Dev:    attr.Dev,
	})
	if err != nil {
		c.logger.Errorw("create entry error", "parent", parentId, "entryName", attr.Name, "err", err)
		return nil, err
	}
	dentry.PublicEntryActionEvent(events.ActionTypeCreate, entry)
	return entry, nil
}

func (c *controller) UpdateEntry(ctx context.Context, entryID int64, newContent *types.Metadata) error {
	defer trace.StartRegion(ctx, "controller.UpdateEntry").End()
	en, err := c.GetEntry(ctx, entryID)
	if err != nil {
		return err
	}

	c.logger.Debugw("update entry", "entry", entryID)
	parent, err := c.entry.OpenGroup(ctx, en.ParentID)
	if err != nil {
		c.logger.Errorw("open group error", "parent", en.ParentID, "entry", entryID, "err", err)
		return err
	}

	if err = parent.UpdateEntry(ctx, entryID, newContent); err != nil {
		c.logger.Errorw("save entry error", "entry", entryID, "err", err)
		return err
	}
	dentry.PublicEntryActionEvent(events.ActionTypeUpdate, en)
	return nil
}

func (c *controller) DestroyEntry(ctx context.Context, parentId, entryId int64, attr types.DestroyObjectAttr) error {
	defer trace.StartRegion(ctx, "controller.DestroyEntry").End()
	parent, err := c.GetEntry(ctx, parentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(parent.Access, attr.Uid, attr.Gid, 0x2); err != nil {
		return types.ErrNoAccess
	}

	en, err := c.GetEntry(ctx, entryId)
	if err != nil {
		return err
	}
	if attr.Uid != 0 && attr.Uid != en.Access.UID && attr.Uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoAccess
	}

	c.logger.Debugw("destroy entry", "parent", parentId, "entry", entryId)
	err = c.entry.RemoveEntry(ctx, parentId, entryId)
	if err != nil {
		c.logger.Errorw("delete entry failed", "entry", entryId, "err", err.Error())
		return err
	}
	dentry.PublicEntryActionEvent(events.ActionTypeDestroy, en)
	return nil
}

func (c *controller) MirrorEntry(ctx context.Context, srcId, dstParentId int64, attr types.ObjectAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "controller.MirrorEntry").End()
	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	oldEntry, err := c.FindEntry(ctx, dstParentId, attr.Name)
	if err != nil && err != types.ErrNotFound {
		c.logger.Errorw("check entry error", "srcEntry", srcId, "err", err.Error())
		return nil, err
	}
	if oldEntry != nil {
		return nil, types.ErrIsExist
	}

	entry, err := c.entry.MirrorEntry(ctx, srcId, dstParentId, dentry.EntryAttr{
		Name:   attr.Name,
		Kind:   attr.Kind,
		Access: attr.Access,
	})
	if err != nil {
		c.logger.Errorw("mirror entry failed", "src", srcId, "err", err.Error())
		return nil, err
	}
	c.logger.Debugw("mirror entry", "src", srcId, "dstParent", dstParentId, "entry", entry.ID)

	events.Publish(events.EntryActionTopic(events.TopicEntryActionFmt, events.ActionTypeMirror),
		dentry.BuildEntryEvent(events.ActionTypeMirror, entry))
	return entry, nil
}

func (c *controller) ListEntryChildren(ctx context.Context, parentId int64) ([]*types.Metadata, error) {
	defer trace.StartRegion(ctx, "controller.ListEntryChildren").End()
	parent, err := c.entry.OpenGroup(ctx, parentId)
	if err != nil {
		return nil, err
	}
	result, err := parent.ListChildren(ctx)
	if err != nil {
		c.logger.Errorw("list entry children failed", "parent", parentId, "err", err)
		return nil, err
	}
	return result, err
}

func (c *controller) ChangeEntryParent(ctx context.Context, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) (err error) {
	defer trace.StartRegion(ctx, "controller.ChangeEntryParent").End()
	if len(newName) > entryNameMaxLength {
		return types.ErrNameTooLong
	}

	// need source dir WRITE
	oldParent, err := c.GetEntry(ctx, oldParentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(oldParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}
	// need dst dir WRITE
	newParent, err := c.GetEntry(ctx, newParentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(newParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}

	target, err := c.GetEntry(ctx, targetId)
	if err != nil {
		return err
	}
	if opt.Uid != 0 && opt.Uid != oldParent.Access.UID && opt.Uid != target.Access.UID && oldParent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoPerm
	}

	var existObjId *int64
	existObj, err := c.FindEntry(ctx, newParentId, newName)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("new name verify failed", "old", targetId, "newParent", newParentId, "newName", newName, "err", err)
			return err
		}
	}

	if existObj != nil {
		if opt.Uid != 0 && opt.Uid != newParent.Access.UID && opt.Uid != existObj.Access.UID && newParent.Access.HasPerm(types.PermSticky) {
			return types.ErrNoPerm
		}
		eid := existObj.ID
		existObjId = &eid
	}

	c.logger.Debugw("change entry parent", "target", targetId, "existObj", existObjId, "oldParent", oldParentId, "newParent", newParentId, "newName", newName)
	err = c.entry.ChangeEntryParent(ctx, targetId, existObjId, oldParentId, newParentId, newName, dentry.ChangeParentAttr{
		Replace:  opt.Replace,
		Exchange: opt.Exchange,
	})
	if err != nil {
		c.logger.Errorw("change object parent failed", "target", targetId, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	dentry.PublicEntryActionEvent(events.ActionTypeChangeParent, target)
	return nil
}

func (c *controller) GetEntryExtendField(ctx context.Context, id int64, fKey string) (*string, error) {
	defer trace.StartRegion(ctx, "controller.GetEntryExtendField").End()
	return c.entry.GetEntryExtendField(ctx, id, fKey)
}

func (c *controller) SetEntryExtendField(ctx context.Context, id int64, fKey, fVal string) error {
	defer trace.StartRegion(ctx, "controller.SetEntryExtendField").End()
	c.logger.Debugw("set entry extend filed", "entry", id, "key", fKey)
	return c.entry.SetEntryExtendField(ctx, id, fKey, fVal)
}

func (c *controller) RemoveEntryExtendField(ctx context.Context, id int64, fKey string) error {
	defer trace.StartRegion(ctx, "controller.RemoveEntryExtendField").End()
	c.logger.Debugw("remove entry extend filed", "entry", id, "key", fKey)
	return c.entry.RemoveEntryExtendField(ctx, id, fKey)
}

func New(loader config.Loader, meta metastore.Meta) (Controller, error) {
	cfg, _ := loader.GetConfig()

	ctl := &controller{
		meta:      meta,
		cfg:       cfg,
		cfgLoader: loader,
		logger:    logger.NewLogger("controller"),
	}
	var err error
	ctl.entry, err = dentry.NewManager(meta, cfg)
	if err != nil {
		return nil, err
	}

	ctl.document, err = document.NewManager(meta)
	if err != nil {
		return nil, err
	}

	ctl.notify = notify.NewNotify(meta)
	ctl.workflow, err = workflow.NewManager(ctl.entry, ctl.document, ctl.notify, meta, cfg.Workflow, cfg.FUSE)
	if err != nil {
		return nil, err
	}

	return ctl, nil
}
