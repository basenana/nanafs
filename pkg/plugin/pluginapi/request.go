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

type Request struct {
	JobID       string
	Namespace   string
	WorkingPath string

	PluginName string
	Parameter  map[string]string

	ContextStore ContextStore
}

func GetParameter(key string, r *Request, defaultVal string) string {
	if len(r.Parameter) > 0 {
		str, ok := r.Parameter[key]
		if ok {
			return str
		}

	}

	return defaultVal
}

func NewRequest() *Request {
	return &Request{}
}

type Response struct {
	IsSucceed     bool
	Message       string
	ModifyEntries []Entry
	Results       map[string]any

	// Deprecated
	NewEntries []CollectManifest
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
