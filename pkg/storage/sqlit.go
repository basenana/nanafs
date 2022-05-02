package storage

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

type sqlitMetaStore struct {
}

var _ MetaStore = &sqlitMetaStore{}

func (s sqlitMetaStore) GetObject(ctx context.Context, id string) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "sqlit.getobject")()
	return nil, nil
}

func (s sqlitMetaStore) ListObjects(ctx context.Context, filter Filter) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "sqlit.listobject")()
	return nil, nil
}

func (s sqlitMetaStore) SaveObject(ctx context.Context, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "sqlit.saveobject")()
	return nil
}

func (s sqlitMetaStore) DestroyObject(ctx context.Context, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "sqlit.destroyobject")()
	return nil
}

func (s sqlitMetaStore) ListChildren(ctx context.Context, obj *types.Object) (Iterator, error) {
	defer utils.TraceRegion(ctx, "sqlit.listchildren")()
	return nil, nil
}

func (s sqlitMetaStore) ChangeParent(ctx context.Context, old *types.Object, parent *types.Object) error {
	defer utils.TraceRegion(ctx, "sqlit.changeparent")()
	return nil
}

func (s sqlitMetaStore) SaveContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error {
	defer utils.TraceRegion(ctx, "sqlit.savecontent")()
	return nil
}

func (s sqlitMetaStore) LoadContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error {
	defer utils.TraceRegion(ctx, "sqlit.loadcontent")()
	return nil
}

func (s sqlitMetaStore) DeleteContent(ctx context.Context, obj *types.Object, cType types.Kind, version string) error {
	defer utils.TraceRegion(ctx, "sqlit.deletecontent")()
	return nil
}
