package plugin

import "github.com/basenana/nanafs/pkg/types"

const (
	PluginTypeSource  types.PluginType = "source"
	PluginTypeProcess types.PluginType = "process"
	PluginTypeMirror  types.PluginType = "mirror"

	PluginLibType    = "goplugin"
	PluginBinType    = "bin"
	PluginScriptType = "script"

	PluginLabelName = "plugin.basenana.org/name"
	PluginAnnPath   = "plugin.basenana.org/path"
)

// Spec is Plugin Config File to load a Plugin
type Spec struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Type       string            `json:"type"`
	Path       string            `json:"path"`
	Parameters map[string]string `json:"parameters"`
}

type Version struct {
}

func ParsePluginVersion(versionContent string) (Version, error) {
	return Version{}, nil
}
