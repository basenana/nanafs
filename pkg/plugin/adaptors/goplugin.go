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
