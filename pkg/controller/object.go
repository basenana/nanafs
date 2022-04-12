package controller

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hyponet/eventbus/bus"
)

type ObjectController interface {
	LoadRootObject(ctx context.Context) (*types.Object, error)
	FindObject(ctx context.Context, parent *types.Object, name string) (*types.Object, error)
	CreateObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	SaveObject(ctx context.Context, obj *types.Object) error
	DestroyObject(ctx context.Context, obj *types.Object) error
	MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error)
	ChangeObjectParent(ctx context.Context, old, newParent *types.Object, newName string, opt ChangeParentOpt) error
}

func (c *controller) LoadRootObject(ctx context.Context) (*types.Object, error) {
	c.logger.Info("init root object")
	root, err := c.meta.GetObject(ctx, dentry.RootObjectID)
	if err != nil {
		if err == types.ErrNotFound {
			root = dentry.InitRootObject()
			return root, c.SaveObject(ctx, root)
		}
		return nil, err
	}
	return root, nil
}

func (c *controller) FindObject(ctx context.Context, parent *types.Object, name string) (*types.Object, error) {
	c.logger.Infow("finding child object", "name", name)
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

func (c *controller) CreateObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	c.logger.Infow("creating new obj", "name", attr.Name, "kind", attr.Kind)
	obj, err := types.InitNewObject(parent, attr)
	if err != nil {
		return nil, err
	}
	obj.Access.Permissions = attr.Permissions
	if err = c.SaveObject(ctx, obj); err != nil {
		return nil, err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.create", obj.ID), obj)
	return obj, nil
}

func (c *controller) SaveObject(ctx context.Context, obj *types.Object) error {
	c.logger.Infow("save obj", "name", obj.Name)
	err := c.meta.SaveObject(ctx, obj)
	if err != nil {
		return err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.update", obj.ID), obj)
	return nil
}

func (c *controller) DestroyObject(ctx context.Context, obj *types.Object) (err error) {
	c.logger.Infow("destroy obj", "name", obj.Name)

	defer func() {
		if err == nil {
			bus.Publish(fmt.Sprintf("object.entry.%s.destory", obj.ID), obj)
		}
	}()

	var objects []*types.Object
	if dentry.IsMirrorObject(obj) {
		var srcObj *types.Object
		srcObj, err = c.meta.GetObject(ctx, obj.RefID)
		if err != nil {
			return err
		}
		if err = c.destroyObject(ctx, obj); err != nil {
			return err
		}

		if srcObj.ParentID == "" {
			objects, err = c.meta.ListObjects(ctx, storage.Filter{RefID: srcObj.ID})
			if err != nil {
				return err
			}
			if len(objects) == 0 {
				err = c.destroyObject(ctx, srcObj)
			}
		}
		return
	}

	objects, err = c.meta.ListObjects(ctx, storage.Filter{RefID: obj.ID})
	if err != nil {
		return err
	}
	if len(objects) == 0 {
		err = c.destroyObject(ctx, obj)
		return
	}

	obj.ParentID = ""
	err = c.meta.SaveObject(ctx, obj)
	return
}

func (c *controller) destroyObject(ctx context.Context, obj *types.Object) (err error) {
	if err = c.meta.DestroyObject(ctx, obj); err != nil {
		return err
	}
	return c.storage.Delete(ctx, obj.ID)
}

func (c *controller) MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	c.logger.Infow("mirror obj", "src", src.Name, "dstParent", dstParent.Name)

	var err error
	for dentry.IsMirrorObject(src) {
		src, err = c.meta.GetObject(ctx, src.RefID)
		if err != nil {
			return nil, err
		}
	}

	obj, err := dentry.CreateMirrorObject(src, dstParent, attr)
	if err != nil {
		return nil, err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.mirror", obj.ID), obj)
	return obj, nil
}

func (c *controller) ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	c.logger.Infow("list children obj", "name", obj.Name)
	it, err := c.meta.ListChildren(ctx, obj.ID)
	if err != nil {
		return nil, err
	}
	result := make([]*types.Object, 0)
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

func (c *controller) ChangeObjectParent(ctx context.Context, obj, newParent *types.Object, newName string, opt ChangeParentOpt) error {
	c.logger.Infow("change obj parent", "old", obj.Name, "newParent", newParent.Name, "newName", newName)
	old, err := c.FindObject(ctx, newParent, newName)
	if err != nil {
		if err != types.ErrNotFound {
			return err
		}
	}
	if old != nil {
		if opt.Exchange {
			old.Name = obj.Name
			obj.Name = newName
			if err = c.SaveObject(ctx, old); err != nil {
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
		if err = c.destroyObject(ctx, old); err != nil {
			return err
		}
	}
	obj.Name = newName
	err = c.meta.ChangeParent(ctx, obj, newParent)
	if err != nil {
		return err
	}
	bus.Publish(fmt.Sprintf("object.entry.%s.mv", obj.ID), obj)
	return nil
}
