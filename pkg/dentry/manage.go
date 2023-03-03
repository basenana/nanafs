package dentry

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

type Manage interface {
	Root(ctx context.Context) (Entry, error)
	MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error)
	ChangeObjectParent(ctx context.Context, old, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error
}

type manage struct {
}

func (m *manage) Root(ctx context.Context) (Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (m *manage) MirrorEntry(ctx context.Context, src, dstParent Entry, attr EntryAttr) (Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (m *manage) ChangeObjectParent(ctx context.Context, old, oldParent, newParent Entry, newName string, opt ChangeParentAttr) error {
	//TODO implement me
	panic("implement me")
}

type EntryAttr struct {
	Name   string
	Dev    int64
	Kind   types.Kind
	Access types.Access
}

type ChangeParentAttr struct {
	Replace  bool
	Exchange bool
}
