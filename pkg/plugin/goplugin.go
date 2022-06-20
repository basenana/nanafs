package plugin

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	goplugin "plugin"
)

const (
	goPluginSymTarget  = "Plugin"
	goVersionSymTarget = "Version"

	currentGoPluginVersion = "1.0"
)

func NewGoPlugin(spec types.PluginSpec) (types.Plugin, error) {
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

	pl, err := p.Lookup(goPluginSymTarget)
	if err != nil {
		return nil, err
	}
	i, ok := pl.(types.Plugin)
	if !ok {
		return nil, fmt.Errorf("plugin not implemented types.Plugin")
	}
	return i, nil
}
