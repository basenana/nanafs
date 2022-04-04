package object

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
