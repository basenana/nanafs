package plugin

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
)

type Plugin interface {
	Name() string
	Type() Type
}

type ProcessPlugin interface {
	Plugin
	Run(ctx context.Context, object *types.Object) error
}

type MetaPlugin interface {
	ProcessPlugin
}

type SourcePlugin interface {
	Plugin
	Run(ctx context.Context) (<-chan *types.Object, error)
}

type MirrorPlugin interface {
	Plugin
	LookUp(ctx context.Context, path string) (*types.Object, error)
	List(ctx context.Context, path string) ([]*types.Object, error)
	Open(ctx context.Context, path string, attr files.Attr) (files.File, error)
}
