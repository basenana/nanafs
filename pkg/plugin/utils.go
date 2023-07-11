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

package plugin

import (
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"os"
	"path/filepath"
)

func readPluginSpec(basePath, pluginSpecFile string) (types.PluginSpec, Builder, error) {
	pluginPath := filepath.Join(basePath, pluginSpecFile)
	info, err := os.Stat(pluginPath)
	if err != nil {
		return types.PluginSpec{}, nil, err
	}

	if info.IsDir() {
		return types.PluginSpec{}, nil, fmt.Errorf("%s was dir", pluginPath)
	}

	f, err := os.Open(pluginPath)
	if err != nil {
		return types.PluginSpec{}, nil, err
	}
	defer f.Close()

	spec := types.PluginSpec{}
	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		return types.PluginSpec{}, nil, err
	}

	if spec.Name == "" {
		return types.PluginSpec{}, nil, fmt.Errorf("plugin name was empty")
	}

	if spec.Path != "" {
		_, err = os.Stat(spec.Path)
		if err != nil {
			return types.PluginSpec{}, nil, fmt.Errorf("stat plugin failed: %s", err.Error())
		}
	}

	return spec, nil, nil
}
