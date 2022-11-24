package types

import (
	"github.com/google/uuid"
	"sync"
	"time"
)

const (
	objectNameMaxLength = 255
)

type Metadata struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Aliases    string    `json:"aliases,omitempty"`
	ParentID   string    `json:"parent_id"`
	RefID      string    `json:"ref_id,omitempty"`
	RefCount   int       `json:"ref_count,omitempty"`
	Kind       Kind      `json:"kind"`
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	Inode      uint64    `json:"inode"`
	Dev        int64     `json:"dev"`
	Namespace  string    `json:"namespace,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ChangedAt  time.Time `json:"changed_at"`
	ModifiedAt time.Time `json:"modified_at"`
	AccessAt   time.Time `json:"access_at"`
	Labels     Labels    `json:"labels"`
	Access     Access    `json:"access"`
}

func NewMetadata(name string, kind Kind) Metadata {
	result := Metadata{
		ID:         uuid.New().String(),
		Name:       name,
		Kind:       kind,
		RefCount:   1,
		CreatedAt:  time.Now(),
		AccessAt:   time.Now(),
		ChangedAt:  time.Now(),
		ModifiedAt: time.Now(),
		Labels:     Labels{},
	}

	if IsGroup(kind) {
		// as dir, default has self and '..' two links
		result.RefCount = 2
	}
	return result
}

type ExtendData struct {
	Properties  *Properties `json:"properties,omitempty"`
	Annotation  *Annotation `json:"annotation,omitempty"`
	GroupFilter *Rule       `json:"group_filter,omitempty"`
	PlugScope   *PlugScope  `json:"plug_scope,omitempty"`
}

type Properties struct {
	Author   string   `json:"author,omitempty"`
	Title    string   `json:"title,omitempty"`
	Subject  string   `json:"subject,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
	Comment  string   `json:"comment,omitempty"`
}

func (p *Properties) copy(newP *Properties) {
	p.Author = newP.Author
	p.Title = newP.Title
	p.Subject = newP.Subject
	p.Keywords = newP.Keywords
	p.Comment = newP.Comment
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

type CustomColumn struct {
	Columns []CustomColumnItem `json:"columns,omitempty"`
}

type CustomColumnItem struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Object struct {
	Metadata
	ExtendData   ExtendData   `json:"extend_data"`
	CustomColumn CustomColumn `json:"custom_column"`

	mux sync.Mutex
}

func (o *Object) IsGroup() bool {
	return IsGroup(o.Kind)
}

func (o *Object) IsSmartGroup() bool {
	return o.Kind == SmartGroupKind
}

type PluginType string

type PlugScope struct {
	PluginName string            `json:"plugin_name"`
	PluginType PluginType        `json:"plugin_type"`
	Parameters map[string]string `json:"parameters"`
}

func InitNewObject(parent *Object, attr ObjectAttr) (*Object, error) {
	if len(attr.Name) > objectNameMaxLength {
		return nil, ErrNameTooLong
	}

	md := NewMetadata(attr.Name, attr.Kind)
	md.Access = attr.Access
	md.Dev = attr.Dev

	newObj := &Object{
		Metadata: md,
		ExtendData: ExtendData{
			Properties: &Properties{},
			Annotation: &Annotation{},
		},
	}
	if parent != nil {
		newObj.ParentID = parent.ID
	}
	return newObj, nil
}
