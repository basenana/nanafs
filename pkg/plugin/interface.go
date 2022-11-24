package plugin

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

type Plugin interface {
	Name() string
	Type() types.PluginType
	Version() Version
}

type ProcessPlugin interface {
	Plugin
	Run(ctx context.Context, object *types.Object, params map[string]string) error
}

type SourcePlugin interface {
	Plugin
	Run(ctx context.Context, parent *types.Object, params map[string]string) (<-chan File, error)
}

type MirrorPlugin interface {
	Plugin
	LookUp(ctx context.Context, path string, params map[string]string) (File, error)
	List(ctx context.Context, path string, params map[string]string) ([]File, error)
	Open(ctx context.Context, path string, attr types.OpenAttr, params map[string]string) (File, error)
}
