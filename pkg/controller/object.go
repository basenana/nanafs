package controller

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/hyponet/eventbus/bus"
	"time"
)

const (
	objectNameMaxLength = 255
)

type ObjectController interface {
	LoadRootObject(ctx context.Context) (*types.Object, error)
	FindObject(ctx context.Context, parent *types.Object, name string) (*types.Object, error)
	GetObject(ctx context.Context, id string) (*types.Object, error)
	CreateObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	SaveObject(ctx context.Context, parent, obj *types.Object) error
	DestroyObject(ctx context.Context, parent, obj *types.Object, attr types.DestroyObjectAttr) error
	MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error)
	ChangeObjectParent(ctx context.Context, old, oldParent, newParent *types.Object, newName string, opt types.ChangeParentAttr) error
}

func (c *controller) LoadRootObject(ctx context.Context) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "controller.loadroot")()
	c.logger.Info("init root object")
	root, err := c.meta.GetObject(ctx, dentry.RootObjectID)
	if err != nil {
		if err == types.ErrNotFound {
			root = dentry.InitRootObject()
			root.Access.UID = c.cfg.Owner.Uid
			root.Access.GID = c.cfg.Owner.Gid
			return root, c.SaveObject(ctx, nil, root)
		}
		c.logger.Errorw("load root object error", "err", err.Error())
		return nil, err
	}
	return root, nil
}

func (c *controller) FindObject(ctx context.Context, parent *types.Object, name string) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "controller.findobject")()

	if len(name) > objectNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	c.logger.Infow("finding child object", "parent", parent.ID, "name", name)
	if !parent.IsGroup() {
		return nil, types.ErrNoGroup
	}
	children, err := c.ListObjectChildren(ctx, parent)
	if err != nil {
		return nil, err
	}

	for i := range children {
		tgt := children[i]
		if tgt.Name == name {
			return tgt, nil
		}
	}
	return nil, types.ErrNotFound
}

func (c *controller) GetObject(ctx context.Context, id string) (*types.Object, error) {
	obj, err := c.meta.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *controller) CreateObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "controller.createobject")()

	if len(attr.Name) > objectNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	c.logger.Infow("creating new obj", "name", attr.Name, "kind", attr.Kind)
	obj, err := types.InitNewObject(parent, attr)
	if err != nil {
		c.logger.Errorw("create new object error", "parent", parent.ID, "name", attr.Name, "err", err.Error())
		return nil, err
	}
	if obj.IsGroup() {
		parent.RefCount += 1
	}
	parent.ChangedAt = time.Now()
	parent.ModifiedAt = time.Now()
	if err = c.SaveObject(ctx, parent, obj); err != nil {
		return nil, err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.create", obj.ID), obj)
	return obj, nil
}

func (c *controller) SaveObject(ctx context.Context, parent, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "controller.saveobject")()
	c.logger.Infow("save obj", "obj", obj.ID)
	err := c.meta.SaveObject(ctx, parent, obj)
	if err != nil {
		c.logger.Errorw("save object error", "obj", obj.ID, "err", err.Error())
		return err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.update", obj.ID), obj)
	return nil
}

func (c *controller) DestroyObject(ctx context.Context, parent, obj *types.Object, attr types.DestroyObjectAttr) (err error) {
	defer utils.TraceRegion(ctx, "controller.destroyobject")()
	c.logger.Infow("destroy obj", "obj", obj.ID)

	if err = dentry.IsAccess(parent.Access, attr.Uid, attr.Gid, 0x2); err != nil {
		return types.ErrNoAccess
	}
	if attr.Uid != 0 && attr.Uid != obj.Access.UID && attr.Uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoAccess
	}

	defer func() {
		if err == nil {
			bus.Publish(fmt.Sprintf("object.entry.%s.destroy", obj.ID), obj)
			if c.IsStructured(obj) {
				bus.Publish(fmt.Sprintf("object.%s.%s.destroy", obj.Kind, obj.ID), obj)
			}
		}
	}()

	var (
		srcObj *types.Object
	)
	if dentry.IsMirrorObject(obj) {
		c.logger.Infow("object is mirrored, delete ref count", "obj", obj.ID, "ref", obj.RefID)
		srcObj, err = c.meta.GetObject(ctx, obj.RefID)
		if err != nil {
			c.logger.Errorw("query source object from meta server error", "obj", obj.ID, "ref", obj.RefID, "err", err.Error())
			return err
		}
	}

	if err = c.destroyObject(ctx, srcObj, parent, obj); err != nil {
		c.logger.Errorw("update deleted object parent meta error", "err", err.Error())
		return err
	}
	return
}

func (c *controller) destroyObject(ctx context.Context, src, parent, obj *types.Object) (err error) {
	if src != nil {
		src.RefCount -= 1
		src.ChangedAt = time.Now()
	}

	if !obj.IsGroup() && obj.RefCount > 0 {
		c.logger.Infow("object has mirrors, remove parent id", "obj", obj.ID)
		obj.RefCount -= 1
		obj.ParentID = ""
		obj.ChangedAt = time.Now()
	}

	if obj.IsGroup() {
		parent.RefCount -= 1
	}
	parent.ChangedAt = time.Now()
	parent.ModifiedAt = time.Now()

	if err = c.meta.DestroyObject(ctx, src, parent, obj); err != nil {
		c.logger.Errorw("destroy object from meta server error", "obj", obj.ID, "err", err.Error())
		return err
	}

	if obj.RefCount == 0 {
		if err = c.storage.Delete(ctx, obj.ID); err != nil {
			c.logger.Errorw("destroy object from storage server error", "obj", obj.ID, "err", err.Error())
		}
	}
	return nil
}

func (c *controller) MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "controller.mirrorobject")()
	c.logger.Infow("mirror obj", "srcObj", src.ID, "dstParent", dstParent.ID)

	if len(attr.Name) > objectNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	oldEntry, err := c.FindObject(ctx, dstParent, attr.Name)
	if err != nil && err != types.ErrNotFound {
		c.logger.Errorw("check entry error", "obj", src.ID, "srcObj", src.RefID, "err", err.Error())
		return nil, err
	}
	if oldEntry != nil {
		return nil, types.ErrIsExist
	}

	for dentry.IsMirrorObject(src) {
		src, err = c.meta.GetObject(ctx, src.RefID)
		if err != nil {
			c.logger.Errorw("query source object error", "obj", src.ID, "srcObj", src.RefID, "err", err.Error())
			return nil, err
		}
		c.logger.Infow("replace source object", "srcObj", src.ID)
	}

	obj, err := dentry.CreateMirrorObject(src, dstParent, attr)
	if err != nil {
		c.logger.Errorw("create mirror object error", "srcObj", src.ID, "dstParent", dstParent.ID, "err", err.Error())
		return nil, err
	}

	src.RefCount += 1
	src.ChangedAt = time.Now()

	dstParent.ChangedAt = time.Now()
	dstParent.ModifiedAt = time.Now()
	if err = c.meta.MirrorObject(ctx, src, dstParent, obj); err != nil {
		c.logger.Errorw("update dst parent object ref count error", "srcObj", src.ID, "dstParent", dstParent.ID, "err", err.Error())
		return nil, err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.mirror", obj.ID), obj)
	return obj, nil
}

func (c *controller) ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "controller.listchildren")()
	if !obj.IsGroup() {
		return nil, types.ErrNoGroup
	}
	result, err := c.group.ListObjectChildren(ctx, obj)
	if err != nil {
		c.logger.Errorw("list object children failed", "parent", obj.ID, "err", err.Error())
		return nil, err
	}
	return result, err
}

func (c *controller) ChangeObjectParent(ctx context.Context, obj, oldParent, newParent *types.Object, newName string, opt types.ChangeParentAttr) (err error) {
	defer utils.TraceRegion(ctx, "controller.changeparent")()

	if len(newName) > objectNameMaxLength {
		return types.ErrNameTooLong
	}

	c.logger.Infow("change obj parent", "old", obj.ID, "newParent", newParent.ID, "newName", newName)
	// need source dir WRITE
	if err = dentry.IsAccess(oldParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}
	// need dst dir WRITE
	if err = dentry.IsAccess(newParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}

	if opt.Uid != 0 && opt.Uid != oldParent.Access.UID && opt.Uid != obj.Access.UID && oldParent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoPerm
	}

	existObj, err := c.FindObject(ctx, newParent, newName)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("new name verify failed", "old", obj.ID, "newParent", newParent.ID, "newName", newName, "err", err.Error())
			return err
		}
	}
	if existObj != nil {
		if opt.Uid != 0 && opt.Uid != newParent.Access.UID && opt.Uid != existObj.Access.UID && newParent.Access.HasPerm(types.PermSticky) {
			return types.ErrNoPerm
		}

		if existObj.IsGroup() {
			children, err := c.ListObjectChildren(ctx, existObj)
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

		// TODO: need a atomic operation?
		if err = c.DestroyObject(ctx, newParent, existObj, types.DestroyObjectAttr{Uid: opt.Uid, Gid: opt.Gid}); err != nil {
			return err
		}
	}

	obj.Name = newName

	oldParent.ChangedAt = time.Now()
	oldParent.ModifiedAt = time.Now()
	newParent.ChangedAt = time.Now()
	newParent.ModifiedAt = time.Now()
	if obj.IsGroup() {
		oldParent.RefCount -= 1
		newParent.RefCount += 1
	}
	err = c.meta.ChangeParent(ctx, oldParent, newParent, obj, types.ChangeParentOption{})
	if err != nil {
		c.logger.Errorw("change object parent failed", "old", obj.ID, "newParent", newParent.ID, "newName", newName, "err", err.Error())
		return err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.mv", obj.ID), obj)
	return nil
}
