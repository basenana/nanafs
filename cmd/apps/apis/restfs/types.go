package restfs

import (
	"encoding/json"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
	"net/http"
)

type FsRequest struct {
	Data Action `json:"data"`
}

const (
	/*
		GET: read, search, inspect, download
		POST: create, bulk, copy
		PUT: update, move, rename
		DELETE: destroy
	*/
	ActionRead     = "read"
	ActionAlias    = "alias"
	ActionSearch   = "search"
	ActionInspect  = "inspect"
	ActionDownload = "download"

	ActionCreate = "create"
	ActionBulk   = "bulk"
	ActionCopy   = "copy"

	ActionUpdate = "update"
	ActionMove   = "move"
	ActionRename = "rename"

	ActionDestroy = "destroy"
)

type Action struct {
	Action     string `json:"action"`
	Parameters struct {
		Name   string   `json:"name"`
		Flags  []string `json:"flags"`
		Fields []string `json:"fields"`
	} `json:"parameters"`
}

func FillDefaultAction(method string, action Action) Action {
	switch method {
	case http.MethodGet:
		if action.Action == "" {
			action.Action = ActionRead
			return action
		}
	case http.MethodPost:
		if action.Action == "" {
			action.Action = ActionCreate
			return action
		}
	case http.MethodPut:
		if action.Action == "" {
			action.Action = ActionUpdate
			return action
		}
	case http.MethodDelete:
		if action.Action == "" {
			action.Action = ActionDestroy
			return action
		}
	}
	return action
}

type FsResponse struct {
	Status int             `json:"status"`
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
		Status: http.StatusOK,
		Data:   data,
	}
}

func NewErrorResponse(status int, code common.ApiErrorCode, err error) FsResponse {
	return FsResponse{
		Status: status,
		Errors: []ErrorResponse{{
			Code:    code,
			Message: err.Error(),
		}},
	}
}
