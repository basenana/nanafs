package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/object"
)

type EntryController interface {
	LoadRootEntry(ctx context.Context) (*dentry.Entry, error)
	FindEntry(ctx context.Context, parent *dentry.Entry, name string) (*dentry.Entry, error)
	CreateEntry(ctx context.Context, parent *dentry.Entry, attr dentry.EntryAttr) (*dentry.Entry, error)
	SaveEntry(ctx context.Context, entry *dentry.Entry) error
	DestroyEntry(ctx context.Context, entry *dentry.Entry) error
	MirrorEntry(ctx context.Context, src, dstParent *dentry.Entry, attr dentry.EntryAttr) (*dentry.Entry, error)
	ListEntryChildren(ctx context.Context, entry *dentry.Entry) ([]*dentry.Entry, error)
	ChangeEntryParent(ctx context.Context, old, newParent *dentry.Entry, newName string) error
}

func (c *controller) LoadRootEntry(ctx context.Context) (*dentry.Entry, error) {
	c.logger.Info("init root entry")
	root, err := c.meta.GetEntry(ctx, dentry.RootEntryID)
	if err != nil {
		if err == object.ErrNotFound {
			root = dentry.InitRootEntry()
			return root, c.SaveEntry(ctx, root)
		}
		return nil, err
	}
	return root, nil
}

func (c *controller) FindEntry(ctx context.Context, parent *dentry.Entry, name string) (*dentry.Entry, error) {
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
	return nil, object.ErrNotFound
}

func (c *controller) CreateEntry(ctx context.Context, parent *dentry.Entry, attr dentry.EntryAttr) (*dentry.Entry, error) {
	c.logger.Infow("creating new entry", "name", attr.Name, "kind", attr.Kind)
	entry, err := dentry.InitNewEntry(parent, attr)
	if err != nil {
		return nil, err
	}
	return entry, c.SaveEntry(ctx, entry)
}

func (c *controller) SaveEntry(ctx context.Context, entry *dentry.Entry) error {
	c.logger.Infow("save entry", "name", entry.Name)
	return c.meta.SaveEntry(ctx, entry)
}

func (c *controller) DestroyEntry(ctx context.Context, entry *dentry.Entry) error {
	c.logger.Infow("destroy entry", "name", entry.Name)
	return c.meta.DestroyEntry(ctx, entry)
}

func (c *controller) MirrorEntry(ctx context.Context, src, dstParent *dentry.Entry, attr dentry.EntryAttr) (*dentry.Entry, error) {
	c.logger.Infow("mirror entry", "src", src.Name, "dstParent", dstParent.Name)
	return nil, nil
}

func (c *controller) ListEntryChildren(ctx context.Context, entry *dentry.Entry) ([]*dentry.Entry, error) {
	c.logger.Infow("list children entry", "name", entry.Name)
	it, err := c.meta.ListChildren(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	result := make([]*dentry.Entry, 0)
	for it.HasNext() {
		next := it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, next)
	}
	return result, nil
}

func (c *controller) ChangeEntryParent(ctx context.Context, old, newParent *dentry.Entry, newName string) error {
	c.logger.Infow("change entry parent", "old", old.Name, "newParent", newParent.Name, "newName", newName)
	old.Name = newName
	return c.meta.ChangeParent(ctx, old, newParent)
}
