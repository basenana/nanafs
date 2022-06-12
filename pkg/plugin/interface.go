package plugin

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

type Plugin interface {
	Name() string
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
	Run(ctx context.Context) error
}

type MirrorPlugin interface {
	Plugin
	LookUp(ctx context.Context, path string) error
	List(ctx context.Context, path string) error
	Open(ctx context.Context, path string) error
}
