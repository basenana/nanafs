package dentry

import (
	"github.com/basenana/nanafs/pkg/object"
)

const (
	RootEntryID = "root"
)

func InitRootEntry() *Entry {
	root, _ := InitNewEntry(nil, EntryAttr{Name: RootEntryID, Kind: object.GroupKind})
	root.ID = RootEntryID
	root.ParentID = root.ID
	return root
}
