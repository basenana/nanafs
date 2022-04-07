package types

import (
	"github.com/google/uuid"
	"sync"
	"time"
)

type Metadata struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Aliases      string        `json:"aliases"`
	ParentID     string        `json:"parent_id"`
	RefID        string        `json:"ref_id"`
	Kind         Kind          `json:"kind"`
	Hash         string        `json:"hash"`
	Size         int64         `json:"size"`
	Inode        uint64        `json:"inode"`
	Namespace    string        `json:"namespace"`
	CreatedAt    time.Time     `json:"created_at"`
	AddedAt      time.Time     `json:"added_at"`
	ModifiedAt   time.Time     `json:"modified_at"`
	Labels       Labels        `json:"labels"`
	Access       Access        `json:"access"`
	CustomColumn *CustomColumn `json:"custom_column"`
}

func NewMetadata(name string, kind Kind) Metadata {
	return Metadata{
		ID:         uuid.New().String(),
		Name:       name,
		Kind:       kind,
		CreatedAt:  time.Now(),
		AddedAt:    time.Now(),
		ModifiedAt: time.Now(),
		Labels:     Labels{},
	}
}

type ExtendData struct {
	Properties *Properties `json:"properties"`
	Annotation *Annotation `json:"annotation"`
}

type Properties struct {
	Author   string
	Title    string
	Subject  string
	KeyWords []string
	Comment  string
}

func (p *Properties) copy(newP *Properties) {
	p.Author = newP.Author
	p.Title = newP.Title
	p.Subject = newP.Subject
	p.KeyWords = newP.KeyWords
	p.Comment = newP.Comment
}

type Annotation struct {
	Annotations []AnnotationItem
	Details     string
}

func (a *Annotation) Add(newA *AnnotationItem) {
	for i, ann := range a.Annotations {
		if ann.Type != newA.Type {
			continue
		}
		a.Annotations[i].Content = newA.Content
		return
	}
	a.Annotations = append(a.Annotations, *newA)
}

func (a *Annotation) Get(key string, withInternal bool) *AnnotationItem {
	for i := range a.Annotations {
		ann := a.Annotations[i]
		if ann.Type != key {
			continue
		}
		if ann.IsInternal {
			if !withInternal {
				return nil
			}
		}
		return &ann
	}
	return nil
}

func (a *Annotation) Remove(key string) {
	needDel := -1
	for i, ann := range a.Annotations {
		if ann.Type != key {
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
	Type       string
	Content    string
	IsInternal bool
	Encode     bool
}

type CustomColumn struct {
	Columns []CustomColumnItem `json:"columns"`
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
	return o.Kind == GroupKind
}

type ObjectAttr struct {
	Name string
	Mode uint32
	Kind Kind
}

func InitNewObject(parent *Object, attr ObjectAttr) (*Object, error) {
	newObj := &Object{
		Metadata: NewMetadata(attr.Name, attr.Kind),
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
