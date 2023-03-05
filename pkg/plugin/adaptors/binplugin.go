/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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
