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
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"runtime/trace"
	"strings"
)

const (
	entryNameMaxLength = 255
	attrPrefix         = "org.basenana"
)

type Commander interface {
	FSRoot(ctx context.Context) (*types.Entry, error)
	InitNamespace(ctx context.Context, namespace string) error
	CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error)
	UpdateEntry(ctx context.Context, namespace string, entry *types.Entry) error
	DestroyEntry(ctx context.Context, namespace string, parentId, entryId int64, attr types.DestroyObjectAttr) error
	MirrorEntry(ctx context.Context, namespace string, srcEntryId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error)
	ChangeEntryParent(ctx context.Context, namespace string, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error
	SetEntryEncodedProperty(ctx context.Context, namespace string, id int64, fKey string, fVal []byte) error
	SetEntryProperty(ctx context.Context, namespace string, id int64, fKey, fVal string) error
	RemoveEntryProperty(ctx context.Context, namespace string, id int64, fKey string) error
}

type commandService struct {
	meta           metastore.Meta
	entry          dentry.Manager
	notify         *notify.Notify
	query          Query
	fsOwnerUid     int64
	fsOwnerGid     int64
	fsWriteback    bool
	defaultStorage string
	cfgLoader      config.Loader
	logger         *zap.SugaredLogger
}

func newCommander(depends Depends, query Query) (Commander, error) {
	return &commandService{
		meta:   depends.Meta,
		notify: depends.Notify,
		query:  query,
		// TODO
		fsOwnerUid:     0,
		fsOwnerGid:     0,
		fsWriteback:    false,
		defaultStorage: "",
		cfgLoader:      depends.ConfigLoader,
		logger:         logger.NewLogger("fsCommander"),
	}, nil
}

func (c *commandService) FSRoot(ctx context.Context) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.commander.FSRoot").End()
	c.logger.Info("get root entry")

	var (
		root *types.Entry
		err  error
	)

	root, err = c.meta.GetEntry(ctx, dentry.RootEntryID)
	if err == nil {
		return root, nil
	}

	if !errors.Is(err, types.ErrNotFound) {
		c.logger.Errorw("load root object error", "err", err)
		return nil, err
	}

	root = initRootEntry()
	root.Access.UID = c.fsOwnerUid
	root.Access.GID = c.fsOwnerGid
	root.Storage = c.defaultStorage

	err = c.meta.CreateEntry(ctx, 0, root, nil)
	if err != nil {
		c.logger.Errorw("create root entry failed", "err", err)
		return nil, err
	}
	return root, nil
}

func (c *commandService) InitNamespace(ctx context.Context, namespace string) error {
	defer trace.StartRegion(ctx, "fs.commander.InitNamespace").End()
	c.logger.Infow("init entry of namespace", "namespace", namespace)

	root, err := c.FSRoot(ctx)
	if err != nil {
		c.logger.Errorw("load root object error", "err", err)
		return err
	}
	// init root entry of namespace
	nsRoot := initNamespaceRootEntry(root, namespace)
	nsRoot.Access.UID = c.fsOwnerUid
	nsRoot.Access.GID = c.fsOwnerGid
	nsRoot.Storage = c.defaultStorage

	err = c.meta.CreateEntry(ctx, dentry.RootEntryID, nsRoot, nil)
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

func (c *commandService) CreateEntry(ctx context.Context, namespace string, parentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.commander.CreateEntry").End()

	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	c.logger.Debugw("create entry", "parent", parentId, "entryName", attr.Name)
	grp, err := c.entry.OpenGroup(ctx, parentId)
	if err != nil {
		return nil, err
	}
	en, err := grp.CreateEntry(ctx, attr)
	if err != nil {
		return nil, err
	}
	return en, nil
}

func (c *commandService) UpdateEntry(ctx context.Context, namespace string, entry *types.Entry) error {
	entryID := entry.ID
	defer trace.StartRegion(ctx, "fs.commander.UpdateEntry").End()
	en, err := c.query.GetEntryInfo(ctx, namespace, entryID)
	if err != nil {
		return err
	}

	c.logger.Debugw("update entry", "entry", entryID)
	parent, err := c.entry.OpenGroup(ctx, en.ParentID)
	if err != nil {
		c.logger.Errorw("open group error", "parent", en.ParentID, "entry", entryID, "err", err)
		return err
	}

	if err = parent.UpdateEntry(ctx, entry); err != nil {
		c.logger.Errorw("save entry error", "entry", entryID, "err", err)
		return err
	}
	return nil
}

func (c *commandService) DestroyEntry(ctx context.Context, namespace string, parentId, entryId int64, attr types.DestroyObjectAttr) error {
	defer trace.StartRegion(ctx, "fs.commander.DestroyEntry").End()
	parent, err := c.query.GetEntryInfo(ctx, namespace, parentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(parent.Access, attr.Uid, attr.Gid, 0x2); err != nil {
		return types.ErrNoAccess
	}

	en, err := c.query.GetEntryInfo(ctx, namespace, entryId)
	if err != nil {
		return err
	}
	if attr.Uid != 0 && attr.Uid != en.Access.UID && attr.Uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoAccess
	}

	c.logger.Debugw("destroy entry", "parent", parentId, "entry", entryId)
	if en.IsGroup {
		children, err := c.meta.ListEntryChildren(ctx, entryId, nil, types.Filter{})
		if err != nil {
			return err
		}

		for children.HasNext() {
			next := children.Next()
			if next.ID == next.ParentID {
				continue
			}
			if err = c.DestroyEntry(ctx, namespace, entryId, next.ID, attr); err != nil {
				return err
			}
		}

	}
	parentGrp, err := c.entry.OpenGroup(ctx, parentId)
	if err != nil {
		return err
	}

	err = parentGrp.RemoveEntry(ctx, entryId)
	if err != nil {
		return err
	}
	err = c.entry.RemoveEntry(ctx, parentId, entryId)
	if err != nil {
		c.logger.Errorw("delete entry failed", "entry", entryId, "err", err.Error())
		return err
	}
	return nil
}

func (c *commandService) MirrorEntry(ctx context.Context, namespace string, srcEntryId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.commander.MirrorEntry").End()
	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	oldEntry, err := c.query.FindEntryInfo(ctx, namespace, dstParentId, attr.Name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		c.logger.Errorw("check entry error", "srcEntry", srcEntryId, "err", err.Error())
		return nil, err
	}
	if oldEntry != nil {
		return nil, types.ErrIsExist
	}

	entry, err := c.entry.MirrorEntry(ctx, srcEntryId, dstParentId, attr)
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
	oldParent, err := c.query.GetEntryInfo(ctx, namespace, oldParentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(oldParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}
	// need dst dir WRITE
	newParent, err := c.query.GetEntryInfo(ctx, namespace, newParentId)
	if err != nil {
		return err
	}
	if err = dentry.IsAccess(newParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}

	target, err := c.query.GetEntryInfo(ctx, namespace, targetId)
	if err != nil {
		return err
	}
	if opt.Uid != 0 && opt.Uid != oldParent.Access.UID && opt.Uid != target.Access.UID && oldParent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoPerm
	}

	var existObjId *int64
	existObj, err := c.query.FindEntryInfo(ctx, namespace, newParentId, newName)
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
	err = c.entry.ChangeEntryParent(ctx, targetId, existObjId, oldParentId, newParentId, newName, types.ChangeParentAttr{
		Replace:  opt.Replace,
		Exchange: opt.Exchange,
	})
	if err != nil {
		c.logger.Errorw("change object parent failed", "target", targetId, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	return nil
}

func (c *commandService) SetEntryEncodedProperty(ctx context.Context, namespace string, id int64, fKey string, fVal []byte) error {
	defer trace.StartRegion(ctx, "fs.commander.SetEntryEncodedProperty").End()
	c.logger.Debugw("set entry extend filed", "entry", id, "key", fKey)

	if strings.HasPrefix(fKey, attrPrefix) {
		return c.SetEntryProperty(ctx, namespace, id, fKey, string(fVal))
	}

	if err := c.meta.AddEntryProperty(ctx, id, fKey, types.PropertyItem{Value: utils.EncodeBase64(fVal), Encoded: true}); err != nil {
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
