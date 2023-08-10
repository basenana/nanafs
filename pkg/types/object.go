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
	"github.com/basenana/nanafs/utils"
	"sync"
	"time"
)

const (
	objectNameMaxLength    = 255
	objectDefaultNamespace = "personal"

	VersionKey         = "nanafs.version"
	KindKey            = "nanafs.kind"
	LabelPluginNameKey = "nanafs.plugin.name"
)

type SystemInfo struct {
	FilesystemID  string `json:"filesystem_id"`
	MaxSegmentID  int64  `json:"max_segment_id"`
	ObjectCount   int64  `json:"object_count"`
	FileSizeTotal int64  `json:"file_size_total"`
}

type Metadata struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Aliases    string    `json:"aliases,omitempty"`
	ParentID   int64     `json:"parent_id"`
	RefID      int64     `json:"ref_id,omitempty"`
	RefCount   int       `json:"ref_count,omitempty"`
	Kind       Kind      `json:"kind"`
	KindMap    int64     `json:"kind_map"`
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
}

func NewMetadata(name string, kind Kind) Metadata {
	result := Metadata{
		ID:         utils.GenerateNewID(),
		Name:       name,
		Namespace:  objectDefaultNamespace,
		Kind:       kind,
		KindMap:    KindMap(kind),
		Version:    1,
		RefCount:   1,
		CreatedAt:  time.Now(),
		AccessAt:   time.Now(),
		ChangedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}
	if IsGroup(kind) {
		// as dir, default has self and '..' two links
		result.RefCount = 2
	}
	return result
}

type ExtendData struct {
	Properties  Properties `json:"properties"`
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
	PluginType PluginType        `json:"plugin_type"`
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
	Fields map[string]string `json:"fields,omitempty"`
}

type Annotation struct {
	Annotations []AnnotationItem `json:"annotations,omitempty"`
	Details     string           `json:"details,omitempty"`
}

func (a *Annotation) Add(newA *AnnotationItem) {
	for i, ann := range a.Annotations {
		if ann.Key != newA.Key {
			continue
		}
		a.Annotations[i].Content = newA.Content
		return
	}
	a.Annotations = append(a.Annotations, *newA)
}

func (a *Annotation) Get(key string) *AnnotationItem {
	for i := range a.Annotations {
		ann := a.Annotations[i]
		if ann.Key != key {
			continue
		}
		return &ann
	}
	return nil
}

func (a *Annotation) Remove(key string) {
	needDel := -1
	for i, ann := range a.Annotations {
		if ann.Key != key {
			continue
		}
		needDel = i
	}
	if needDel < 0 {
		return
	}
	a.Annotations = append(a.Annotations[0:needDel], a.Annotations[needDel+1:]...)
}

type AnnotationItem struct {
	Key     string `json:"key"`
	Content string `json:"content"`
	Encode  bool   `json:"encode"`
}

type Object struct {
	Metadata
	*ExtendData
	*Labels

	ExtendDataChanged bool `json:"-"`
	LabelsChanged     bool `json:"-"`

	L sync.Mutex `json:"-"`
}

func (o *Object) SetExtendData(ex ExtendData) {
	o.ExtendData = &ex
	o.ExtendDataChanged = true
}

func (o *Object) SetLabels(labels Labels) {
	o.Labels = &labels
	o.LabelsChanged = true
}

func (o *Object) IsGroup() bool {
	return IsGroup(o.Kind)
}

func (o *Object) IsSmartGroup() bool {
	return o.Kind == SmartGroupKind
}

func InitNewObject(parent *Metadata, attr ObjectAttr) (*Object, error) {
	if len(attr.Name) > objectNameMaxLength {
		return nil, ErrNameTooLong
	}

	md := NewMetadata(attr.Name, attr.Kind)
	md.Access = Access{
		Permissions: attr.Access.Permissions,
		UID:         attr.Access.UID,
		GID:         attr.Access.GID,
	}

	newObj := &Object{
		Metadata: md,
		ExtendData: &ExtendData{
			Properties: Properties{Fields: map[string]string{}},
		},
		ExtendDataChanged: true,
	}
	if parent != nil {
		newObj.ParentID = parent.ID
		newObj.Storage = parent.Storage
	}
	return newObj, nil
}

type ChunkSeg struct {
	ID       int64
	ChunkID  int64
	ObjectID int64
	Off      int64
	Len      int64
	State    int16
}

func IsMirrored(entry *Metadata) bool {
	return !IsGroup(entry.Kind) && entry.RefID != 0 && entry.RefID != entry.ID
}
