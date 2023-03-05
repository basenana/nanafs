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

const (
	VersionKey         = "nanafs.version"
	KindKey            = "nanafs.kind"
	LabelPluginNameKey = "nanafs.plugin.name"
)

type Labels struct {
	Labels []Label `json:"labels,omitempty"`
}

func (l Labels) Get(key string) *Label {
	for _, label := range l.Labels {
		if label.Key == key {
			return &label
		}
	}
	return nil
}

type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
