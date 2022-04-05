package storage

import (
	"github.com/basenana/nanafs/pkg/types"
)

type Filter struct {
	ID        string
	ParentID  string
	Kind      types.Kind
	Namespace string
	Label     LabelMatch
}

type LabelMatch struct {
	Include []types.Label
	Exclude []types.Label
}

func isObjectFiltered(obj *types.Object, filter Filter) bool {
	md := obj.GetObjectMeta()
	if filter.ID != "" {
		return md.ID == filter.ID
	}
	if filter.ParentID != "" {
		return md.ParentID == filter.ParentID
	}
	if filter.Kind != "" {
		return md.Kind == filter.Kind
	}
	if filter.Namespace != "" {
		return md.Namespace == filter.Namespace
	}
	return false
}
