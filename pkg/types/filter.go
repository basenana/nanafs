package types

import "time"

type Filter struct {
	ID        string
	ParentID  string
	RefID     string
	Kind      Kind
	Namespace string
	Label     LabelMatch
}

func FilterObjectMapper(f Filter) map[string]interface{} {
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
	Include []Label  `json:"include"`
	Exclude []string `json:"exclude"`
}

func IsObjectFiltered(obj *Object, filter Filter) bool {
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

type WorkflowFilter struct {
	ID        string
	Synced    bool
	UpdatedAt *time.Time
}

type ObjectMapper struct {
	Key       string
	Value     interface{}
	Operation string
}

func WorkflowFilterObjectMapper(f WorkflowFilter) []ObjectMapper {
	result := []ObjectMapper{}
	if f.ID != "" {
		result = append(result, ObjectMapper{
			Key:       "id",
			Value:     f.ID,
			Operation: "=",
		})
	}
	if !f.Synced {
		result = append(result, ObjectMapper{
			Key:       "synced",
			Value:     f.Synced,
			Operation: "=",
		})
	}
	if f.UpdatedAt != nil {
		result = append(result, ObjectMapper{
			Key:       "updated_at",
			Value:     f.UpdatedAt,
			Operation: "<",
		})
	}
	return result
}
