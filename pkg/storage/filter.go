package storage

import "github.com/basenana/nanafs/pkg/object"

type Filter struct {
	ID        string
	ParentID  string
	Kind      object.Kind
	Namespace string
	Label     LabelMatch
}

type LabelMatch struct {
	Include []object.Label
	Exclude []object.Label
}

func isObjectFiltered(obj object.Object, filter Filter) bool {
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
