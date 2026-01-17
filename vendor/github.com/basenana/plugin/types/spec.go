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

type Plugin interface {
	Name() string
	Type() PluginType
	Version() string
}

// ParameterSpec describes a plugin parameter
type ParameterSpec struct {
	Name        string   `json:"name"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// PluginSpec is Plugin Config File to load a Plugin
type PluginSpec struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	Type           PluginType      `json:"type"`
	RequiredConfig []string        `json:"required_config"` // Config keys required by this plugin
	InitParameters []ParameterSpec `json:"init_parameters"` // Parameters for plugin initialization
	Parameters     []ParameterSpec `json:"parameters"`      // Parameters for plugin execution
}

type PluginCall struct {
	JobID       string            `json:"job_id"`
	Workflow    string            `json:"workflow"`
	Namespace   string            `json:"namespace"`
	WorkingPath string            `json:"working_path"`
	PluginName  string            `json:"plugin_name"`
	Version     string            `json:"version"`
	Params      map[string]string `json:"params"`
	Config      map[string]string `json:"config"` // LLM and other configuration
}
