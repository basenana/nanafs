package storage

import (
	"github.com/basenana/nanafs/pkg/types"
)

type Filter struct {
	ID        string
	ParentID  string
	RefID     string
	Kind      types.Kind
	Namespace string
	Label     LabelMatch
}

type LabelMatch struct {
	Include []types.Label
	Exclude []types.Label
}

func isObjectFiltered(obj *types.Object, filter Filter) bool {
	if filter.ID != "" {
		return obj.ID == filter.ID
	}
	if filter.ParentID != "" {
		return obj.ParentID == filter.ParentID
	}
	if filter.Kind != "" {
		return obj.Kind == filter.Kind
	}
	if filter.RefID != "" {
		return obj.RefID == filter.RefID
	}
	if filter.Namespace != "" {
		return obj.Namespace == filter.Namespace
	}
	return false
}
