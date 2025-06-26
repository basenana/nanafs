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

package pluginapi

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
)

type Request struct {
	Action    string
	Parameter map[string]any

	WorkPath string

	Entries       []Entry
	ParentEntryId int64

	Namespace      string
	CacheData      *CachedData
	ContextResults Results
}

func GetParameter(key string, r *Request, spec types.PluginSpec, scope types.PluginCall) string {
	if len(r.Parameter) > 0 {
		valRaw, ok := r.Parameter[key]
		if ok {
			str, ok := valRaw.(string)
			if ok {
				return str
			}
			return fmt.Sprintf("%v", valRaw)
		}

	}
	if len(scope.Parameters) > 0 {
		val, ok := scope.Parameters[key]
		if ok {
			return val
		}
	}

	for _, cfg := range spec.Customization {
		if cfg.Key == key {
			return cfg.Default
		}
	}
	return ""
}

func NewRequest() *Request {
	return &Request{}
}

type Response struct {
	IsSucceed  bool
	Message    string
	NewEntries []CollectManifest
	Results    map[string]any
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
