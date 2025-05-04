/*
 Copyright 2024 NanaFS Authors.

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

package fs

import (
	"context"
	"errors"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"runtime/trace"
)

const (
	entryNameMaxLength = 255
)

type Commander interface {
	FSRoot(ctx context.Context) (*types.Entry, error)
	InitNamespace(ctx context.Context, namespace string) error
	CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error)
	UpdateEntry(ctx context.Context, namespace string, entryID int64, update *UpdateEntry) error
	DestroyEntry(ctx context.Context, namespace string, parentId, entryId int64, attr types.DestroyObjectAttr) error
	MirrorEntry(ctx context.Context, namespace string, srcEntryId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error)
	ChangeEntryParent(ctx context.Context, namespace string, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error
	SetEntryProperty(ctx context.Context, namespace string, id int64, fKey, fVal string) error
	RemoveEntryProperty(ctx context.Context, namespace string, id int64, fKey string) error
	OpenFile(ctx context.Context, namespace string, id int64, attr types.OpenAttr) (core.File, error)
}

type commandService struct {
	meta      metastore.Meta
	core      core.Core
	notify    *notify.Notify
	cfgLoader config.Loader
	logger    *zap.SugaredLogger
}

func newCommander(depends *Depends) (Commander, error) {
	return &commandService{
		meta:      depends.Meta,
		notify:    depends.Notify,
		cfgLoader: depends.ConfigLoader,
		logger:    logger.NewLogger("fsCommander"),
	}, nil
}

func (c *commandService) FSRoot(ctx context.Context) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.commander.FSRoot").End()
	c.logger.Info("get root entry")

	return c.core.FSRoot(ctx)
}

func (c *commandService) InitNamespace(ctx context.Context, namespace string) error {
	defer trace.StartRegion(ctx, "fs.commander.InitNamespace").End()
	c.logger.Infow("init entry of namespace", "namespace", namespace)

	if len(namespace) > entryNameMaxLength {
		return types.ErrNameTooLong
	}

	return c.core.CreateNamespace(ctx, namespace)
}

func (c *commandService) CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.commander.CreateEntry").End()

	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	c.logger.Debugw("create entry", "parent", parentId, "entryName", attr.Name)
	en, err := c.core.CreateEntry(ctx, namespace, parentId, attr)
	if err != nil {
		return nil, err
	}
	return en, nil
}

func (c *commandService) UpdateEntry(ctx context.Context, namespace string, entryID int64, update *UpdateEntry) error {
	defer trace.StartRegion(ctx, "fs.commander.UpdateEntry").End()
	en, err := c.meta.GetEntry(ctx, entryID)
	if err != nil {
		return err
	}

	c.logger.Debugw("update entry", "entry", entryID)
	parent, err := c.core.OpenGroup(ctx, namespace, en.ParentID)
	if err != nil {
		c.logger.Errorw("open group error", "parent", en.ParentID, "entry", entryID, "err", err)
		return err
	}

	if update.Name != nil {
		en.Name = *update.Name
	}
	if update.Aliases != nil {
		en.Aliases = *update.Aliases
	}

	if err = parent.UpdateEntry(ctx, en); err != nil {
		c.logger.Errorw("save entry error", "entry", entryID, "err", err)
		return err
	}
	return nil
}

func (c *commandService) DestroyEntry(ctx context.Context, namespace string, parentId, entryId int64, attr types.DestroyObjectAttr) error {
	defer trace.StartRegion(ctx, "fs.commander.DestroyEntry").End()
	parent, err := c.meta.GetEntry(ctx, parentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(parent.Access, attr.Uid, attr.Gid, 0x2); err != nil {
		return types.ErrNoAccess
	}

	en, err := c.meta.GetEntry(ctx, entryId)
	if err != nil {
		return err
	}
	if attr.Uid != 0 && attr.Uid != en.Access.UID && attr.Uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoAccess
	}

	c.logger.Debugw("destroy entry", "parent", parentId, "entry", entryId)
	return c.core.RemoveEntry(ctx, namespace, parentId, entryId)
}

func (c *commandService) MirrorEntry(ctx context.Context, namespace string, srcEntryId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.commander.MirrorEntry").End()
	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	oldEntry, err := c.meta.FindEntry(ctx, dstParentId, attr.Name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		c.logger.Errorw("check entry error", "srcEntry", srcEntryId, "err", err.Error())
		return nil, err
	}
	if oldEntry != nil {
		return nil, types.ErrIsExist
	}

	entry, err := c.core.MirrorEntry(ctx, namespace, srcEntryId, dstParentId, attr)
	if err != nil {
		c.logger.Errorw("mirror entry failed", "src", srcEntryId, "err", err.Error())
		return nil, err
	}
	c.logger.Debugw("mirror entry", "src", srcEntryId, "dstParent", dstParentId, "entry", entry.ID)

	return entry, nil
}

func (c *commandService) ChangeEntryParent(ctx context.Context, namespace string, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error {
	defer trace.StartRegion(ctx, "fs.commander.ChangeEntryParent").End()
	if len(newName) > entryNameMaxLength {
		return types.ErrNameTooLong
	}

	// need source dir WRITE
	oldParent, err := c.meta.GetEntry(ctx, oldParentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(oldParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}
	// need dst dir WRITE
	newParent, err := c.meta.GetEntry(ctx, newParentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(newParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}

	target, err := c.meta.GetEntry(ctx, targetId)
	if err != nil {
		return err
	}
	if opt.Uid != 0 && opt.Uid != oldParent.Access.UID && opt.Uid != target.Access.UID && oldParent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoPerm
	}

	var existObjId *int64
	existObj, err := c.meta.FindEntry(ctx, newParentId, newName)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
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
	err = c.core.ChangeEntryParent(ctx, namespace, targetId, existObjId, oldParentId, newParentId, newName, types.ChangeParentAttr{
		Replace:  opt.Replace,
		Exchange: opt.Exchange,
	})
	if err != nil {
		c.logger.Errorw("change object parent failed", "target", targetId, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	return nil
}

func (c *commandService) SetEntryProperty(ctx context.Context, namespace string, id int64, fKey, fVal string) error {
	defer trace.StartRegion(ctx, "fs.commander.SetEntryProperty").End()
	if err := c.meta.AddEntryProperty(ctx, id, fKey, types.PropertyItem{Value: fVal, Encoded: false}); err != nil {
		return err
	}
	return nil
}

func (c *commandService) RemoveEntryProperty(ctx context.Context, namespace string, id int64, fKey string) error {
	defer trace.StartRegion(ctx, "fs.commander.RemoveEntryProperty").End()
	if err := c.meta.RemoveEntryProperty(ctx, id, fKey); err != nil {
		return err
	}
	return nil
}

func (c *commandService) OpenFile(ctx context.Context, namespace string, id int64, attr types.OpenAttr) (core.File, error) {
	defer trace.StartRegion(ctx, "fs.commander.OpenFile").End()
	return c.core.Open(ctx, namespace, id, attr)
}
