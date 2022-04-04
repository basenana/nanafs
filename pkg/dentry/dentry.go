package dentry

import (
	"github.com/basenana/nanafs/pkg/object"
)

type Entry struct {
	object.Metadata
}

var _ object.Object = &Entry{}

func (e *Entry) GetExtendData() object.ExtendData {
	return object.ExtendData{}
}

func (e *Entry) GetCustomColumn() object.CustomColumn {
	return object.CustomColumn{}
}

func (e *Entry) IsGroup() bool {
	return e.Kind == object.GroupKind
}

type EntryAttr struct {
	Name string
	Mode uint32
	Kind object.Kind
}

func InitNewEntry(parent *Entry, attr EntryAttr) (*Entry, error) {
	newEntry := &Entry{
		Metadata: object.NewMetadata(attr.Name, attr.Kind),
	}
	if parent != nil {
		newEntry.ParentID = parent.ID
	}
	return newEntry, nil
}
