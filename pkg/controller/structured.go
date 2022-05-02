package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
)

type StructuredController interface {
	ListStructuredObject(ctx context.Context, cType types.Kind, version string) ([]*types.Object, error)
	OpenStructuredObject(ctx context.Context, obj *types.Object, spec interface{}, attr files.Attr) (files.File, error)
	CleanStructuredObject(ctx context.Context, obj *types.Object) error
	CreateStructuredObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr, cType types.Kind, version string) (*types.Object, error)
	LoadStructureObject(ctx context.Context, obj *types.Object, spec interface{}) error
	SaveStructureObject(ctx context.Context, obj *types.Object, spec interface{}) error
}

func (c *controller) LoadStructureObject(ctx context.Context, obj *types.Object, spec interface{}) error {
	err := c.meta.LoadContent(ctx, obj, obj.Kind, obj.Labels.Get(types.VersionKey).Value, spec)
	if err != nil {
		return err
	}
	return nil
}

func (c *controller) SaveStructureObject(ctx context.Context, obj *types.Object, spec interface{}) error {
	err := c.meta.SaveContent(ctx, obj, obj.Kind, obj.Labels.Get(types.VersionKey).Value, spec)
	if err != nil {
		return err
	}
	return nil
}

func (c *controller) ListStructuredObject(ctx context.Context, cType types.Kind, version string) ([]*types.Object, error) {
	f := storage.Filter{
		Kind: cType,
		Label: storage.LabelMatch{Include: []types.Label{{
			types.VersionKey,
			version,
		}, {
			Key:   types.KindKey,
			Value: string(cType),
		}}},
	}
	return c.meta.ListObjects(ctx, f)
}

func (c *controller) OpenStructuredObject(ctx context.Context, obj *types.Object, spec interface{}, attr files.Attr) (files.File, error) {
	return files.OpenStructured(ctx, obj, spec, attr)
}

func (c *controller) CleanStructuredObject(ctx context.Context, obj *types.Object) error {
	return c.meta.DestroyObject(ctx, obj)
}

func (c *controller) CreateStructuredObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr, cType types.Kind, version string) (*types.Object, error) {
	obj, err := types.InitNewObject(parent, attr)
	if err != nil {
		return nil, err
	}
	obj.Kind = cType
	obj.Labels = types.Labels{Labels: []types.Label{{
		Key:   types.VersionKey,
		Value: version,
	}, {
		Key:   types.KindKey,
		Value: string(attr.Kind),
	}}}
	return obj, c.SaveObject(ctx, obj)
}
