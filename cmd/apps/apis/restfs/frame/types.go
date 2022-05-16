package frame

import (
	"encoding/json"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
)

type FsRequest struct {
	Data Action `json:"data"`
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
