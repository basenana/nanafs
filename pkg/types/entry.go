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

import (
	"time"

	"github.com/basenana/nanafs/utils"
)

const (
	entryNameMaxLength = 255

	LabelKeyPluginPrefix = "org.basenana.internal.plugin/"
	LabelKeyPluginKind   = LabelKeyPluginPrefix + "kind"
	LabelKeyPluginName   = LabelKeyPluginPrefix + "name"

	PropertyWebSiteName     = "org.basenana.web.site_name"
	PropertyWebSiteURL      = "org.basenana.web.site_url"
	PropertyWebPageURL      = "org.basenana.web.url"
	PropertyWebPageUpdateAt = "org.basenana.web.updated_at"
	PropertyWebPageTitle    = "org.basenana.web.title"
)

type SystemInfo struct {
	FilesystemID  string `json:"filesystem_id"`
	MaxSegmentID  int64  `json:"max_segment_id"`
	ObjectCount   int64  `json:"object_count"`
	FileSizeTotal int64  `json:"file_size_total"`
}

type Entry struct {
	ID         int64     `json:"id"`
	Aliases    string    `json:"aliases,omitempty"`
	RefCount   int       `json:"ref_count,omitempty"`
	Kind       Kind      `json:"kind"`
	KindMap    int64     `json:"kind_map"`
	IsGroup    bool      `json:"is_group"`
	Size       int64     `json:"size"`
	Version    int64     `json:"version"`
	Dev        int64     `json:"dev"`
	Namespace  string    `json:"namespace,omitempty"`
	Storage    string    `json:"storage"`
	CreatedAt  time.Time `json:"created_at"`
	ChangedAt  time.Time `json:"changed_at"`
	ModifiedAt time.Time `json:"modified_at"`
	AccessAt   time.Time `json:"access_at"`
	Access     Access    `json:"access"`

	// Deprecated
	Name string `json:"name"`
	// Deprecated
	ParentID int64 `json:"parent_id"`
}

type Child struct {
	ParentID  int64  `json:"parent_id"`
	ChildID   int64  `json:"child_id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Marker    string `json:"marker"`
}

func NewEntry(name string, kind Kind) Entry {
	result := Entry{
		ID:         utils.GenerateNewID(),
		Name:       name,
		Kind:       FileKind(name, kind),
		KindMap:    0,
		IsGroup:    IsGroup(kind),
		Version:    1,
		RefCount:   1,
		CreatedAt:  time.Now(),
		AccessAt:   time.Now(),
		ChangedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}
	if result.IsGroup {
		// as dir, default has self and '..' two links
		result.RefCount = 2
		result.Kind = kind
	}
	return result
}

type ExtendData struct {
	Symlink     string     `json:"symlink,omitempty"`
	GroupFilter *Rule      `json:"group_filter,omitempty"`
	PlugScope   *PlugScope `json:"plug_scope,omitempty"`
}

const (
	PlugScopeEntryName     = "entry.name"
	PlugScopeEntryPath     = "entry.path" // relative path
	PlugScopeWorkflowID    = "workflow.id"
	PlugScopeWorkflowJobID = "workflow.job.id"
)

type PlugScope struct {
	PluginName string            `json:"plugin_name"`
	Version    string            `json:"version"`
	PluginType PluginType        `json:"plugin_type,omitempty"`
	Action     string            `json:"action,omitempty"`
	Parameters map[string]string `json:"parameters"`
}

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

type Properties struct {
	Fields map[string]PropertyItem `json:"fields,omitempty"`
}

type PropertyItem struct {
	Value   string `json:"value"`
	Encoded bool   `json:"encoded"`
}

func InitNewEntry(parent *Entry, attr EntryAttr) (*Entry, error) {
	if len(attr.Name) > entryNameMaxLength {
		return nil, ErrNameTooLong
	}

	md := NewEntry(attr.Name, attr.Kind)
	md.Dev = attr.Dev

	if parent != nil {
		md.ParentID = parent.ID
		md.Storage = parent.Storage
		md.Access = parent.Access
		md.Namespace = parent.Namespace
	}

	if attr.Access != nil {
		md.Access = Access{
			Permissions: attr.Access.Permissions,
			UID:         attr.Access.UID,
			GID:         attr.Access.GID,
		}
	}
	return &md, nil
}

type ChunkSeg struct {
	ID      int64
	ChunkID int64
	EntryID int64
	Off     int64
	Len     int64
	State   int16
}
