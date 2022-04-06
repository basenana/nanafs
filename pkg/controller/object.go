package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

type ObjectController interface {
	LoadRootObject(ctx context.Context) (*types.Object, error)
	FindObject(ctx context.Context, parent *types.Object, name string) (*types.Object, error)
	CreateObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	SaveObject(ctx context.Context, obj *types.Object) error
	DestroyObject(ctx context.Context, obj *types.Object) error
	MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error)
	ChangeObjectParent(ctx context.Context, old, newParent *types.Object, newName string) error
}

func (c *controller) LoadRootObject(ctx context.Context) (*types.Object, error) {
	c.logger.Info("init root object")
	root, err := c.meta.GetObject(ctx, types.RootObjectID)
	if err != nil {
		if err == types.ErrNotFound {
			root = types.InitRootObject()
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
	return obj, c.SaveObject(ctx, obj)
}

func (c *controller) SaveObject(ctx context.Context, obj *types.Object) error {
	c.logger.Infow("save obj", "name", obj.Name)
	return c.meta.SaveObject(ctx, obj)
}

func (c *controller) DestroyObject(ctx context.Context, obj *types.Object) error {
	c.logger.Infow("destroy obj", "name", obj.Name)
	return c.meta.DestroyObject(ctx, obj)
}

func (c *controller) MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	c.logger.Infow("mirror obj", "src", src.Name, "dstParent", dstParent.Name)
	return nil, nil
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

func (c *controller) ChangeObjectParent(ctx context.Context, old, newParent *types.Object, newName string) error {
	c.logger.Infow("change obj parent", "old", old.Name, "newParent", newParent.Name, "newName", newName)
	old.Name = newName
	return c.meta.ChangeParent(ctx, old, newParent)
}
