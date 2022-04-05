package plugin

import (
	"github.com/basenana/nanafs/pkg/object"
	goplugin "plugin"
)

type Plugin interface {
	Name() string
	Run(object object.Object) error
}

func NewPlugin(pluginPath string) (Plugin, error) {
	p, err := goplugin.Open(pluginPath)
	if err != nil {
		return nil, err
	}
	pl, err := p.Lookup("Plugin")
	if err != nil {
		return nil, err
	}
	i, ok := pl.(Plugin)
	if !ok {
		return nil, err
	}
	return i, nil
}
