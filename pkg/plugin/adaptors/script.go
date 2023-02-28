package adaptors

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	ExecTypeScript = "script"
)

type ScriptPluginAdaptor struct {
}

func (s ScriptPluginAdaptor) Name() string {
	//TODO implement me
	panic("implement me")
}

func (s ScriptPluginAdaptor) Type() types.PluginType {
	//TODO implement me
	panic("implement me")
}

func (s ScriptPluginAdaptor) Version() string {
	//TODO implement me
	panic("implement me")
}

func NewScriptAdaptor(spec types.PluginSpec) (*ScriptPluginAdaptor, error) {
	return nil, fmt.Errorf("no support")
}
