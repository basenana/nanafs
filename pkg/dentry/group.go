package dentry

import (
	"context"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

type Group interface {
	FindEntry(ctx context.Context, name string) (Entry, error)
	GetEntry(ctx context.Context, id int64) (Entry, error)
	CreateEntry(ctx context.Context, attr types.ObjectAttr) (Entry, error)
	UpdateEntry(ctx context.Context, obj Entry) error
	DestroyEntry(ctx context.Context, obj Entry) error
	ListChildren(ctx context.Context) ([]Entry, error)
}

type stdGroup struct {
	Entry
	store storage.ObjectStore
}

func (g *stdGroup) FindEntry(ctx context.Context, name string) (Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (g *stdGroup) GetEntry(ctx context.Context, id int64) (Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (g *stdGroup) CreateEntry(ctx context.Context, attr types.ObjectAttr) (Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (g *stdGroup) UpdateEntry(ctx context.Context, obj Entry) error {
	//TODO implement me
	panic("implement me")
}

func (g *stdGroup) DestroyEntry(ctx context.Context, obj Entry) error {
	//TODO implement me
	panic("implement me")
}

func (g *stdGroup) ListChildren(ctx context.Context) ([]Entry, error) {
	defer utils.TraceRegion(ctx, "stdGroup.list")()
	it, err := g.store.ListChildren(ctx, g.Object())
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0)
	for it.HasNext() {
		next := it.Next()
		if next.ID == next.ParentID {
			continue
		}
		result = append(result, BuildEntry(next))
	}
	return result, nil
}

type dynamicGroup struct {
	*stdGroup
}

type mirroredGroup struct {
	*stdGroup
}
