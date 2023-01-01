package adaptors

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	ExecTypeBin = "bin"
)

type BinPluginAdaptor struct {
	spec types.PluginSpec
}

func (b BinPluginAdaptor) Name() string {
	//TODO implement me
	panic("implement me")
}

func (b BinPluginAdaptor) Type() types.PluginType {
	//TODO implement me
	panic("implement me")
}

func (b BinPluginAdaptor) Version() string {
	//TODO implement me
	panic("implement me")
}

func NewBinPluginAdaptor(spec types.PluginSpec) (*BinPluginAdaptor, error) {
	return nil, fmt.Errorf("no support")
}
