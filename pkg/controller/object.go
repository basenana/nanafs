package controller

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/storage"
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
	SaveObject(ctx context.Context, obj *types.Object) error
	DestroyObject(ctx context.Context, obj *types.Object) error
	MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error)
	ChangeObjectParent(ctx context.Context, old, oldParent, newParent *types.Object, newName string, opt ChangeParentOpt) error
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
			return root, c.SaveObject(ctx, root)
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
	c.logger.Infow("get object", "id", id)
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
	if parent.Labels.Get(types.KindKey) != nil && parent.Labels.Get(types.KindKey).Value != "" {
		return c.CreateStructuredObject(ctx, parent, attr, types.Kind(parent.Labels.Get(types.KindKey).Value), parent.Labels.Get(types.VersionKey).Value)
	}

	obj, err := types.InitNewObject(parent, attr)
	if err != nil {
		c.logger.Errorw("create new object error", "parent", parent.ID, "name", attr.Name, "err", err.Error())
		return nil, err
	}
	if err = c.SaveObject(ctx, obj); err != nil {
		return nil, err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.create", obj.ID), obj)

	parent.ChangedAt = time.Now()
	parent.ModifiedAt = time.Now()
	if err = c.SaveObject(ctx, parent); err != nil {
		c.logger.Errorw("update parent object error", "parent", parent.ID, "name", attr.Name, "err", err.Error())
		return nil, err
	}
	return obj, nil
}

func (c *controller) SaveObject(ctx context.Context, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "controller.saveobject")()
	c.logger.Infow("save obj", "obj", obj.ID)
	err := c.meta.SaveObject(ctx, obj)
	if err != nil {
		c.logger.Errorw("save object error", "obj", obj.ID, "err", err.Error())
		return err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.update", obj.ID), obj)
	return nil
}

func (c *controller) DestroyObject(ctx context.Context, obj *types.Object) (err error) {
	defer utils.TraceRegion(ctx, "controller.destroyobject")()
	c.logger.Infow("destroy obj", "obj", obj.ID)

	defer func() {
		if err == nil {
			bus.Publish(fmt.Sprintf("object.entry.%s.destroy", obj.ID), obj)
			if c.IsStructured(obj) {
				bus.Publish(fmt.Sprintf("object.%s.%s.destroy", obj.Kind, obj.ID), obj)
			}
		}
	}()

	var objects []*types.Object
	if dentry.IsMirrorObject(obj) {
		c.logger.Infow("object is mirrored, delete ref count", "obj", obj.ID, "ref", obj.RefID)
		var srcObj *types.Object
		srcObj, err = c.meta.GetObject(ctx, obj.RefID)
		if err != nil {
			c.logger.Errorw("query source object from meta server error", "obj", obj.ID, "ref", obj.RefID, "err", err.Error())
			return err
		}
		if err = c.destroyObject(ctx, obj); err != nil {
			c.logger.Errorw("delete mirrored object from meta server error", "obj", obj.ID, "err", err.Error())
			return err
		}
		srcObj.RefCount -= 1

		if srcObj.RefCount == 0 && srcObj.ParentID == "" {
			c.logger.Infow("source object has no parent", "obj", obj.ID, "ref", obj.RefID)
			objects, err = c.meta.ListObjects(ctx, storage.Filter{RefID: srcObj.ID})
			if err != nil {
				c.logger.Errorw("query source object mirrors from meta server error", "srcObj", srcObj.ID, "err", err.Error())
				return err
			}

			if len(objects) == 0 {
				c.logger.Infow("source object has no mirrors any more, destroy source", "srcObj", srcObj.ID)
				err = c.destroyObject(ctx, srcObj)
			}
		} else {
			srcObj.ChangedAt = time.Now()
			err = c.SaveObject(ctx, srcObj)
		}
		return
	}

	if obj.ID != "" {
		objects, err = c.meta.ListObjects(ctx, storage.Filter{RefID: obj.ID})
		if err != nil {
			c.logger.Errorw("query object mirrors from meta server error", "obj", obj.ID, "err", err.Error())
			return err
		}
		if len(objects) != 0 {
			c.logger.Infow("object has mirrors, remove parent id", "obj", obj.ID)
			obj.ParentID = ""
			obj.RefCount -= 1
			obj.ChangedAt = time.Now()
			err = c.meta.SaveObject(ctx, obj)
			return
		}
	}

	err = c.destroyObject(ctx, obj)
	return
}

func (c *controller) destroyObject(ctx context.Context, obj *types.Object) (err error) {
	if err = c.meta.DestroyObject(ctx, obj); err != nil {
		c.logger.Errorw("destroy object from meta server error", "obj", obj.ID, "err", err.Error())
		return err
	}
	if err = c.storage.Delete(ctx, obj.ID); err != nil {
		c.logger.Errorw("destroy object from storage server error", "obj", obj.ID, "err", err.Error())
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
	if err = c.SaveObject(ctx, obj); err != nil {
		c.logger.Errorw("save mirror object error", "srcObj", src.ID, "dstParent", dstParent.ID, "err", err.Error())
		return nil, err
	}

	src.RefCount += 1
	src.ChangedAt = time.Now()
	if err = c.SaveObject(ctx, src); err != nil {
		c.logger.Errorw("update src object ref count error", "srcObj", src.ID, "dstParent", dstParent.ID, "err", err.Error())
		_ = c.DestroyObject(ctx, obj)
		return nil, err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.mirror", obj.ID), obj)

	dstParent.ChangedAt = time.Now()
	dstParent.ModifiedAt = time.Now()
	if err = c.SaveObject(ctx, dstParent); err != nil {
		c.logger.Errorw("update dst parent object ref count error", "srcObj", src.ID, "dstParent", dstParent.ID, "err", err.Error())
		return nil, err
	}
	return obj, nil
}

func (c *controller) ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "controller.listchildren")()
	if !obj.IsGroup() {
		return nil, types.ErrNoGroup
	}
	c.logger.Infow("list children obj", "name", obj.Name)
	result := make([]*types.Object, 0)
	it, err := c.meta.ListChildren(ctx, obj)
	if err != nil {
		c.logger.Errorw("load object children error", "obj", obj.ID, "err", err.Error())
		return nil, err
	}
	for it.HasNext() {
		next := it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, next)
	}
	return result, nil
}

type ChangeParentOpt struct {
	Replace  bool
	Exchange bool
}

func (c *controller) ChangeObjectParent(ctx context.Context, obj, oldParent, newParent *types.Object, newName string, opt ChangeParentOpt) error {
	defer utils.TraceRegion(ctx, "controller.changeparent")()

	if len(newName) > objectNameMaxLength {
		return types.ErrNameTooLong
	}

	c.logger.Infow("change obj parent", "old", obj.ID, "newParent", newParent.ID, "newName", newName)
	existObj, err := c.FindObject(ctx, newParent, newName)
	if err != nil {
		if err != types.ErrNotFound {
			c.logger.Errorw("new name verify failed", "old", obj.ID, "newParent", newParent.ID, "newName", newName, "err", err.Error())
			return err
		}
	}
	if existObj != nil {
		if existObj.IsGroup() {
			children, err := c.ListObjectChildren(ctx, existObj)
			if err != nil {
				return err
			}
			if len(children) > 0 {
				return types.ErrIsExist
			}
		}

		if opt.Exchange {
			existObj.Name = obj.Name
			obj.Name = newName
			if err = c.SaveObject(ctx, existObj); err != nil {
				return err
			}
			if err = c.SaveObject(ctx, obj); err != nil {
				return err
			}
			return nil
		}

		if !opt.Replace {
			return types.ErrIsExist
		}
		if err = c.DestroyObject(ctx, existObj); err != nil {
			return err
		}
	}

	obj.Name = newName
	err = c.meta.ChangeParent(ctx, obj, newParent)
	if err != nil {
		c.logger.Errorw("change object parent failed", "old", obj.ID, "newParent", newParent.ID, "newName", newName, "err", err.Error())
		return err
	}

	oldParent.ChangedAt = time.Now()
	oldParent.ModifiedAt = time.Now()
	newParent.ChangedAt = time.Now()
	newParent.ModifiedAt = time.Now()
	if obj.IsGroup() {
		oldParent.RefCount -= 1
		newParent.RefCount += 1
	}
	if err = c.SaveObject(ctx, oldParent); err != nil {
		return err
	}
	if err = c.SaveObject(ctx, newParent); err != nil {
		return err
	}

	bus.Publish(fmt.Sprintf("object.entry.%s.mv", obj.ID), obj)
	return nil
}
