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

package frame

import (
	"encoding/json"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
)

type RequestV1 struct {
	Parameters Parameters `json:"parameters"`
}

type FsResponse struct {
	Data   interface{}     `json:"data,omitempty"`
	Errors []ErrorResponse `json:"errors,omitempty"`
}

func (r FsResponse) Json() []byte {
	data, _ := json.Marshal(r)
	return data
}

type ErrorResponse struct {
	Code    common.ApiErrorCode `json:"code"`
	Message string              `json:"message"`
}

func NewFsResponse(data interface{}) FsResponse {
	return FsResponse{
		Data: data,
	}
}

func NewErrorResponse(code common.ApiErrorCode, err error) FsResponse {
	return FsResponse{
		Errors: []ErrorResponse{{
			Code:    code,
			Message: err.Error(),
		}},
	}
}
