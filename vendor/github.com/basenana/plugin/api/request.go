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

package api

import "encoding/json"

type Request struct {
	JobID       string
	Namespace   string
	WorkingPath string

	PluginName string
	Parameter  map[string]any

	Store PersistentStore
	FS    NanaFS
}

func GetStringParameter(key string, r *Request, defaultVal string) string {
	if len(r.Parameter) > 0 {
		val, ok := r.Parameter[key]
		if ok {
			if str, ok := val.(string); ok {
				return str
			}
			if data, err := json.Marshal(val); err == nil {
				return string(data)
			}
		}
	}

	return defaultVal
}

func GetBoolParameter(key string, r *Request, defaultVal bool) bool {
	if len(r.Parameter) > 0 {
		val, ok := r.Parameter[key]
		if ok {
			if b, ok := val.(bool); ok {
				return b
			}
			if str, ok := val.(string); ok {
				return str == "true" || str == "1"
			}
		}
	}
	return defaultVal
}

func NewRequest() *Request {
	return &Request{}
}

type Response struct {
	IsSucceed bool
	Message   string
	Results   map[string]any
}

func NewResponse() *Response {
	return &Response{IsSucceed: true}
}

func NewFailedResponse(msg string) *Response {
	return &Response{IsSucceed: false, Message: msg}
}

func NewResponseWithResult(result map[string]any) *Response {
	return &Response{IsSucceed: true, Results: result}
}
