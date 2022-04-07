package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

type EntryController interface {
	LoadRootEntry(ctx context.Context) (*types.Object, error)
	FindEntry(ctx context.Context, parent *types.Object, name string) (*types.Object, error)
	CreateEntry(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	SaveEntry(ctx context.Context, entry *types.Object) error
	DestroyEntry(ctx context.Context, entry *types.Object) error
	MirrorEntry(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error)
	ListEntryChildren(ctx context.Context, entry *types.Object) ([]*types.Object, error)
	ChangeEntryParent(ctx context.Context, old, newParent *types.Object, newName string) error
}

func (c *controller) LoadRootEntry(ctx context.Context) (*types.Object, error) {
	c.logger.Info("init root entry")
	root, err := c.meta.GetEntry(ctx, types.RootEntryID)
	if err != nil {
		if err == types.ErrNotFound {
			root = types.InitRootEntry()
			return root, c.SaveEntry(ctx, root)
		}
		return nil, err
	}
	return root, nil
}

func (c *controller) FindEntry(ctx context.Context, parent *types.Object, name string) (*types.Object, error) {
	c.logger.Infow("finding child entry", "name", name)
	children, err := c.ListEntryChildren(ctx, parent)
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

func (c *controller) CreateEntry(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	c.logger.Infow("creating new entry", "name", attr.Name, "kind", attr.Kind)
	entry, err := types.InitNewEntry(parent, attr)
	if err != nil {
		return nil, err
	}
	return entry, c.SaveEntry(ctx, entry)
}

func (c *controller) SaveEntry(ctx context.Context, entry *types.Object) error {
	c.logger.Infow("save entry", "name", entry.Name)
	return c.meta.SaveEntry(ctx, entry)
}

func (c *controller) DestroyEntry(ctx context.Context, entry *types.Object) error {
	c.logger.Infow("destroy entry", "name", entry.Name)
	return c.meta.DestroyEntry(ctx, entry)
}

func (c *controller) MirrorEntry(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	c.logger.Infow("mirror entry", "src", src.Name, "dstParent", dstParent.Name)
	return nil, nil
}

func (c *controller) ListEntryChildren(ctx context.Context, entry *types.Object) ([]*types.Object, error) {
	c.logger.Infow("list children entry", "name", entry.Name)
	it, err := c.meta.ListChildren(ctx, entry.ID)
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

func (c *controller) ChangeEntryParent(ctx context.Context, old, newParent *types.Object, newName string) error {
	c.logger.Infow("change entry parent", "old", old.Name, "newParent", newParent.Name, "newName", newName)
	old.Name = newName
	return c.meta.ChangeParent(ctx, old, newParent)
}
