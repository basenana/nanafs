package types

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
)

type PluginType string

const (
	PluginTypeMeta    PluginType = "meta"
	PluginTypeSource  PluginType = "source"
	PluginTypeProcess PluginType = "process"
	PluginTypeMirror  PluginType = "mirror"

	PluginExecType   = "goplugin"
	PluginBinType    = "bin"
	PluginScriptType = "script"
)

type Plugin interface {
	Name() string
	Type() PluginType
}

type ProcessPlugin interface {
	Plugin
	Run(ctx context.Context, object *Object) error
}

type MetaPlugin interface {
	ProcessPlugin
}

type SourcePlugin interface {
	Plugin
	Run(ctx context.Context) (<-chan *Object, error)
}

type MirrorPlugin interface {
	Plugin
	LookUp(ctx context.Context, path string) (*Object, error)
	List(ctx context.Context, path string) ([]*Object, error)
	Open(ctx context.Context, path string, attr files.Attr) (files.File, error)
}

// PluginSpec is Plugin Config File to load a Plugin
type PluginSpec struct {
	Type       string            `json:"type"`
	Path       string            `json:"path"`
	Parameters map[string]string `json:"parameters"`
}
