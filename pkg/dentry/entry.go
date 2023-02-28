package dentry

import "github.com/basenana/nanafs/pkg/types"

type Entry struct {
	*types.Object
}

func NewEntry(obj *types.Object) *Entry {
	return &Entry{Object: obj}
}
