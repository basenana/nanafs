package types

type Filter struct {
	ID        int64
	ParentID  int64
	RefID     int64
	Kind      Kind
	Namespace string
	Label     LabelMatch
}

func FilterObjectMapper(f Filter) map[string]interface{} {
	result := make(map[string]interface{})
	if f.ID != 0 {
		result["id"] = f.ID
	}
	if f.ParentID != 0 {
		result["parent_id"] = f.ParentID
	}
	if f.RefID != 0 {
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
	Include []Label  `json:"include"`
	Exclude []string `json:"exclude"`
}

func IsObjectFiltered(obj *Object, filter Filter) bool {
	if filter.ID != 0 {
		return obj.ID == filter.ID
	}
	if filter.ParentID != 0 {
		return obj.ParentID == filter.ParentID
	}
	if filter.Kind != "" {
		return obj.Kind == filter.Kind
	}
	if filter.RefID != 0 {
		return obj.RefID == filter.RefID
	}
	if filter.Namespace != "" {
		return obj.Namespace == filter.Namespace
	}
	return false
}
