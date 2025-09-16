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
	PropertyTypeDocument  PropertyType = "D"
)

type Properties struct {
	Tags []string `json:"tags"`

	// web
	URL      string `json:"url,omitempty"`
	SiteName string `json:"site,omitempty"`

	Properties map[string]string `json:"properties,omitempty"`
}

type AttrProperties map[string]string

type SymlinkProperties struct {
	Symlink string `json:"symlink"`
}

type GroupProperties struct {
	Filter *Filter `json:"filter,omitempty"`

	// source configs
	Source string    `json:"source,omitempty"` // rss
	RSS    *GroupRSS `json:"rss,omitempty"`
}

type GroupRSS struct {
	Feed     string `json:"feed"`
	SiteName string `json:"siteName"`
	SiteURL  string `json:"siteUrl"`
	FileType string `json:"fileType"`
}

type DocumentProperties struct {
	Title string `json:"title"`

	// papers
	Author string `json:"author,omitempty"`
	Year   string `json:"year,omitempty"`
	Source string `json:"source,omitempty"`

	// content
	Abstract string   `json:"abstract,omitempty"`
	Notes    string   `json:"notes,omitempty"`
	Keywords []string `json:"keywords,omitempty"`

	// web
	URL         string `json:"url,omitempty"`
	HeaderImage string `json:"headerImage,omitempty"`

	Unread    bool  `json:"unread"`
	Marked    bool  `json:"marked"`
	PublishAt int64 `json:"publishAt,omitempty"`
}

type PropertyItem struct {
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}
