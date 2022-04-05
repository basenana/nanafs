package types

import (
	"github.com/google/uuid"
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

func (d *Metadata) GetObjectMeta() *Metadata {
	return d
}

type Labels struct {
	Labels []Label `json:"labels"`
}

type Label struct {
	Key string
}

// Access TODO
type Access struct{}

func NewMetadata(name string, kind Kind) *Metadata {
	return &Metadata{
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
	Properties
	Annotation
	Storage
}

func (e *ExtendData) GetProperties() Properties {
	return e.Properties
}

func (e *ExtendData) GetAnnotation() Annotation {
	return e.Annotation
}

func (e *ExtendData) GetStorage() Storage {
	return e.Storage
}

func (e *ExtendData) SetProperties(properties *Properties) {
	e.Properties.copy(properties)
}

func (e *ExtendData) AddAnnotation(annotation *AnnotationItem) {
	e.Annotation.add(annotation)
}

func (e *ExtendData) RemoveAnnotation(annotation *AnnotationItem) {
	e.Annotation.remove(annotation)
}

func (e *ExtendData) SetStorage(storage *Storage) {
	e.Storage.copy(storage)
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

func (a *Annotation) add(newA *AnnotationItem) {
	for i, ann := range a.Annotations {
		if ann.Type != newA.Type {
			continue
		}
		a.Annotations[i].Content = newA.Content
		return
	}
	a.Annotations = append(a.Annotations, *newA)
}

func (a *Annotation) remove(oldA *AnnotationItem) {
	needDel := -1
	for i, ann := range a.Annotations {
		if ann.Type != oldA.Type {
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
	Type    string
	Content string
}

type Storage struct {
	Type string
	Key  string
	Opts map[string]string
}

func (s *Storage) copy(newS *Storage) {
	if s.Type != newS.Type {
		return
	}

	s.Key = newS.Key
	s.Opts = newS.Opts
}

type CustomColumn struct {
	Columns []CustomColumnItem `json:"columns"`
}

type CustomColumnItem struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}
