package types

type PluginType string

const (
	TypeSource  PluginType = "source"
	TypeProcess PluginType = "process"
	TypeMirror  PluginType = "mirror"
)

// PluginSpec is Plugin Config File to load a Plugin
type PluginSpec struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Type       PluginType        `json:"type"`
	Path       string            `json:"path,omitempty"`
	Parameters map[string]string `json:"parameters"`
}
