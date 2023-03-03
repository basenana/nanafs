/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package types

import "time"

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
