/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package apitool

import "github.com/gin-gonic/gin"

type Response struct {
	Status int         `json:"status"`
	Error  *Error      `json:"error,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ErrorResponse(gCtx *gin.Context, status int, code string, err error) {
	resp := Response{
		Status: status,
		Error: &Error{
			Code:    code,
			Message: err.Error(),
		},
	}
	gCtx.JSON(status, resp)
}

func JsonResponse(gCtx *gin.Context, status int, data interface{}) {
	resp := Response{
		Status: status,
		Data:   data,
	}
	gCtx.JSON(status, resp)
}
