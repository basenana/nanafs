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
	"errors"
	"runtime/trace"
	"time"

	"github.com/basenana/nanafs/pkg/dialogue"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/inbox"
	"github.com/basenana/nanafs/pkg/token"
	"github.com/basenana/nanafs/workflow"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	entryNameMaxLength = 255
)

type Controller interface {
	AccessToken(ctx context.Context, ak, sk string) (*types.AccessToken, error)
	CreateNamespace(ctx context.Context, namespace string) (*types.Namespace, error)

	CreateEntry(ctx context.Context, parentId int64, attr types.EntryAttr) (*types.Entry, error)
	UpdateEntry(ctx context.Context, entry *types.Entry) error
	DestroyEntry(ctx context.Context, parentId, entryId int64, attr types.DestroyObjectAttr) error
	MirrorEntry(ctx context.Context, srcEntryId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error)
	ChangeEntryParent(ctx context.Context, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error
	SetEntryEncodedProperty(ctx context.Context, id int64, fKey string, fVal []byte) error
	SetEntryProperty(ctx context.Context, id int64, fKey, fVal string) error
	RemoveEntryProperty(ctx context.Context, id int64, fKey string) error

	LoadRootEntry(ctx context.Context) (*types.Entry, error)
	GetGroupTree(ctx context.Context) (*types.GroupEntry, error)
	FindEntry(ctx context.Context, parentId int64, name string) (*types.Entry, error)
	GetEntry(ctx context.Context, id int64) (*types.Entry, error)
	GetEntryByURI(ctx context.Context, uri string) (*types.Entry, error)
	ListEntryChildren(ctx context.Context, entryId int64, order *types.EntryOrder, filters ...types.Filter) ([]*types.Entry, error)
	ListEntryProperties(ctx context.Context, id int64) (map[string]types.PropertyItem, error)
	GetEntryProperty(ctx context.Context, id int64, fKey string) ([]byte, error)

	ConfigEntrySourcePlugin(ctx context.Context, id int64, scope types.ExtendData) error
	CleanupEntrySourcePlugin(ctx context.Context, id int64) error

	ListDocuments(ctx context.Context, filter types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error)
	ListDocumentGroups(ctx context.Context, parentId int64, filter types.DocFilter) ([]*types.Entry, error)
	GetDocumentsByEntryId(ctx context.Context, entryId int64) (*types.Document, error)
	GetDocument(ctx context.Context, documentId int64) (*types.Document, error)
	QueryDocuments(ctx context.Context, query string) ([]*types.Document, error)
	UpdateDocument(ctx context.Context, doc *types.Document) error

	ListRooms(ctx context.Context, entryId int64) ([]*types.Room, error)
	CreateRoom(ctx context.Context, entryId int64, prompt string) (*types.Room, error)
	GetRoom(ctx context.Context, id int64) (*types.Room, error)
	FindRoom(ctx context.Context, entryId int64) (*types.Room, error)
	UpdateRoom(ctx context.Context, roomId int64, prompt string) error
	DeleteRoom(ctx context.Context, id int64) error
	ClearRoom(ctx context.Context, id int64) error
	ChatInRoom(ctx context.Context, roomId int64, newMsg string, reply chan types.ReplyChannel) (err error)
	CreateRoomMessage(ctx context.Context, roomID int64, sender, msg string, sendAt time.Time) (*types.RoomMessage, error)

	OpenFile(ctx context.Context, entryId int64, attr types.OpenAttr) (dentry.File, error)
	ReadFile(ctx context.Context, file dentry.File, data []byte, offset int64) (n int64, err error)
	WriteFile(ctx context.Context, file dentry.File, data []byte, offset int64) (n int64, err error)
	CloseFile(ctx context.Context, file dentry.File) error

	FsInfo(ctx context.Context) Info
	StartBackendTask(stopCh chan struct{})
	SetupShutdownHandler(stopCh chan struct{}) chan struct{}
}

type controller struct {
	*notify.Notify

	meta      metastore.Meta
	cfgLoader config.Loader

	entry    dentry.Manager
	notify   *notify.Notify
	workflow workflow.Workflow
	document document.Manager
	dialogue dialogue.Manager
	token    *token.Manager

	logger *zap.SugaredLogger
}

var _ Controller = &controller{}

func (c *controller) StartBackendTask(stopCh chan struct{}) {
}

func (c *controller) CreateNamespace(ctx context.Context, namespace string) (*types.Namespace, error) {
	defer trace.StartRegion(ctx, "controller.CreateNamespace").End()
	c.logger.Infow("init entry of namespace", "namespace", namespace)
	ns := types.NewNamespace(namespace)
	nsCtx := types.WithNamespace(ctx, ns)

	// init namespace entry
	err := c.entry.CreateNamespace(ctx, ns)
	if err != nil {
		c.logger.Errorw("init namespace root object error", "namespace", namespace, "err", err.Error())
		return nil, err
	}
	// init inbox group
	err = inbox.InitInboxInternalGroup(nsCtx, c.entry)
	if err != nil {
		c.logger.Errorw("init internal inbox group failed", "err", err)
		return nil, err
	}
	return ns, nil
}

func (c *controller) LoadRootEntry(ctx context.Context) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "controller.LoadRootEntry").End()
	c.logger.Info("init root entry")
	rootEntry, err := c.entry.Root(ctx)
	if err != nil {
		c.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	return rootEntry, nil
}

func (c *controller) GetGroupTree(ctx context.Context) (*types.GroupEntry, error) {
	defer trace.StartRegion(ctx, "controller.GetGroupTree").End()
	root, err := c.entry.Root(ctx)
	if err != nil {
		return nil, err
	}
	return buildGroupEntry(ctx, c.entry, root, false)
}

func (c *controller) FindEntry(ctx context.Context, parentId int64, name string) (*types.Entry, error) {
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

func (c *controller) GetEntry(ctx context.Context, id int64) (*types.Entry, error) {
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

func (c *controller) GetEntryByURI(ctx context.Context, uri string) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "controller.GetEntryByURI").End()
	result, err := c.entry.GetEntryByUri(ctx, uri)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("get entry error", "entryURI", uri, "err", err)
		}
		return nil, err
	}
	return result, nil
}

func (c *controller) CreateEntry(ctx context.Context, parentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "controller.CreateEntry").End()

	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	c.logger.Debugw("create entry", "parent", parentId, "entryName", attr.Name)
	entry, err := c.entry.CreateEntry(ctx, parentId, attr)
	if err != nil {
		c.logger.Errorw("create entry error", "parent", parentId, "entryName", attr.Name, "err", err)
		return nil, err
	}
	return entry, nil
}

func (c *controller) UpdateEntry(ctx context.Context, entry *types.Entry) error {
	entryID := entry.ID
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

	if err = parent.UpdateEntry(ctx, entry); err != nil {
		c.logger.Errorw("save entry error", "entry", entryID, "err", err)
		return err
	}
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
	return nil
}

func (c *controller) MirrorEntry(ctx context.Context, srcId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error) {
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

	entry, err := c.entry.MirrorEntry(ctx, srcId, dstParentId, attr)
	if err != nil {
		c.logger.Errorw("mirror entry failed", "src", srcId, "err", err.Error())
		return nil, err
	}
	c.logger.Debugw("mirror entry", "src", srcId, "dstParent", dstParentId, "entry", entry.ID)
	return entry, nil
}

func (c *controller) ListEntryChildren(ctx context.Context, parentId int64, order *types.EntryOrder, filters ...types.Filter) ([]*types.Entry, error) {
	defer trace.StartRegion(ctx, "controller.ListEntryChildren").End()
	parent, err := c.entry.OpenGroup(ctx, parentId)
	if err != nil {
		return nil, err
	}
	result, err := parent.ListChildren(ctx, order, filters...)
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

	//nextParentID := newParentId
	//for nextParentID != dentry.RootEntryID {
	//	en, err := c.GetEntry(ctx, newParentId)
	//	if err != nil {
	//		if errors.Is(err, types.ErrNotFound) {
	//			break
	//		}
	//		c.logger.Errorw("check in loop failed", "old", targetId, "newParent", newParentId, "newName", newName, "err", err)
	//		return err
	//	}
	//	if en.ID == targetId {
	//		return types.ErrNotEmpty
	//	}
	//	nextParentID = en.ParentID
	//}

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

func New(loader config.Loader, meta metastore.Meta, fridayClient friday.Friday) (Controller, error) {
	ctl := &controller{
		meta:      meta,
		cfgLoader: loader,
		logger:    logger.NewLogger("controller"),
	}
	bCfg, err := loader.GetBootstrapConfig()
	if err != nil {
		return nil, err
	}
	ctl.token = token.NewTokenManager(meta, loader)
	if tokenErr := ctl.token.InitBuildinCA(context.Background()); tokenErr != nil {
		ctl.logger.Warnw("init build-in ca failed", "err", tokenErr)
	}

	ctl.Notify = notify.NewNotify(meta)

	ctl.entry, err = dentry.NewManager(meta, bCfg)
	if err != nil {
		return nil, err
	}

	ctl.document, err = document.NewManager(meta, ctl.entry, loader, fridayClient)
	if err != nil {
		return nil, err
	}

	ctl.dialogue, err = dialogue.NewManager(meta, ctl.entry)
	if err != nil {
		return nil, err
	}

	return ctl, nil
}
