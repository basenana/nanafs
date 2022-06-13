package plugin

const (
	ExecType   = "goplugin"
	BinType    = "bin"
	ScriptType = "script"
)

// Spec is Plugin Config File to load a Plugin
type Spec struct {
	Type       string            `json:"type"`
	Path       string            `json:"path"`
	Parameters map[string]string `json:"parameters"`
}
