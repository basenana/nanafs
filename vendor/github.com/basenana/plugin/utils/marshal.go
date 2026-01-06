package utils

import "encoding/json"

func MarshalMap(obj any) map[string]interface{} {
	if obj == nil {
		return nil
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func UnmarshalMap(data map[string]interface{}, obj any) {
	if data == nil || obj == nil {
		return
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	_ = json.Unmarshal(dataBytes, obj)
}
