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

type PropertyType string

const (
	PropertyTypeProperty  PropertyType = "P"
	PropertyTypeAttr      PropertyType = "A"
	PropertyTypeSymlink   PropertyType = "S"
	PropertyTypeGroupAttr PropertyType = "G"
)

type Properties map[string]PropertyItem

type AttrProperties map[string]PropertyItem

type SymlinkProperties struct {
	Symlink string `json:"symlink"`
}

type GroupProperties struct {
	Filter *Rule     `json:"filter,omitempty"`
	Source string    `json:"source,omitempty"` // rss
	RSS    *GroupRSS `json:"rss,omitempty"`
}

type GroupRSS struct {
	Feed     string `json:"feed"`
	SiteName string `json:"siteName"`
	SiteURL  string `json:"siteUrl"`
	FileType string `json:"fileType"`
}

type PropertyItem struct {
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}
