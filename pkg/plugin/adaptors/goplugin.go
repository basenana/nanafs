package adaptors

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	goplugin "plugin"
)

const (
	ExecTypeGoPlugin = "goplugin"

	goPluginSymTarget  = "Plugin"
	goVersionSymTarget = "Version"

	currentGoPluginVersion = "1.0"
)

type GoPluginAdaptor struct {
}

func (g GoPluginAdaptor) Name() string {
	//TODO implement me
	panic("implement me")
}

func (g GoPluginAdaptor) Type() types.PluginType {
	//TODO implement me
	panic("implement me")
}

func (g GoPluginAdaptor) Version() string {
	//TODO implement me
	panic("implement me")
}

func NewGoPluginAdaptor(spec types.PluginSpec) (*GoPluginAdaptor, error) {
	p, err := goplugin.Open(spec.Path)
	if err != nil {
		return nil, err
	}

	ver, err := p.Lookup(goVersionSymTarget)
	if err != nil {
		return nil, err
	}
	versionInfo, ok := ver.(string)
	if !ok {
		return nil, fmt.Errorf("no plugin version found")
	}
	if versionInfo != currentGoPluginVersion {
		return nil, fmt.Errorf("plugin version %s too low", versionInfo)
	}

	_, err = p.Lookup(goPluginSymTarget)
	if err != nil {
		return nil, err
	}
	return &GoPluginAdaptor{}, nil
}
