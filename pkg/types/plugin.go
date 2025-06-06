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

package types

type PluginType string

const (
	TypeSource  PluginType = "source"
	TypeProcess PluginType = "process"
)

// PluginSpec is Plugin Config File to load a Plugin
type PluginSpec struct {
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Type          PluginType        `json:"type"`
	Parameters    map[string]string `json:"parameters"`
	Customization []PluginConfig    `json:"customization"`
}

type PluginConfig struct {
	Key     string `json:"key"`
	Default string `json:"default"`
}
