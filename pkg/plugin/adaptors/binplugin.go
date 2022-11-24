package adaptors

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin"
)

func NewBinPluginAdaptor(spec plugin.Spec) (plugin.Plugin, error) {
	return nil, fmt.Errorf("no support")
}
