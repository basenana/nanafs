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

func filterMapper(f Filter) map[string]interface{} {
	result := make(map[string]interface{})
	if f.ID != "" {
		result["id"] = f.ID
	}
	if f.ParentID != "" {
		result["parent_id"] = f.ParentID
	}
	if f.RefID != "" {
		result["ref_id"] = f.RefID
	}
	if f.Kind != "" {
		result["kind"] = string(f.Kind)
	}
	if f.Namespace != "" {
		result["namespace"] = string(f.Namespace)
	}
	return result
}

type LabelMatch struct {
	Include []types.Label
	Exclude []string
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
