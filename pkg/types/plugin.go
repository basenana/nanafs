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

	PluginLabelName = "plugin.basenana.org/name"
	PluginAnnPath   = "plugin.basenana.org/path"
)

type Plugin interface {
	Name() string
	Type() PluginType
}

type ProcessPlugin interface {
	Plugin
	Run(ctx context.Context, object *Object, params map[string]string) error
}

type MetaPlugin interface {
	ProcessPlugin
}

type SourcePlugin interface {
	Plugin
	Run(ctx context.Context, parent *Object, params map[string]string) (<-chan SimpleFile, error)
}

type MirrorPlugin interface {
	Plugin
	LookUp(ctx context.Context, path string, params map[string]string) (SimpleFile, error)
	List(ctx context.Context, path string, params map[string]string) ([]SimpleFile, error)
	Open(ctx context.Context, path string, attr files.Attr, params map[string]string) (SimpleFile, error)
}

// PluginSpec is Plugin Config File to load a Plugin
type PluginSpec struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Path       string            `json:"path"`
	Parameters map[string]string `json:"parameters"`
}
