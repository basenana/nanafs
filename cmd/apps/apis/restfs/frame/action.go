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
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

const (
	/*
		GET:	read, search, inspect, download
		POST:	create, bulk, copy
		PUT:	update, move, rename
		DELETE:	destroy
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

	UrlArgsActionKey = "action"
	UrlArgsFlagsKey  = "flags"
	UrlArgsFieldsKey = "fields"
)

type Action struct {
	Action string   `json:"action"`
	Flags  []string `json:"flags"`
	Fields []string `json:"fields"`
}

func BuildAction(gCtx *gin.Context) Action {
	action := gCtx.Query(UrlArgsActionKey)
	flagsStr := gCtx.Query(UrlArgsFlagsKey)
	fieldsStr := gCtx.Query(UrlArgsFieldsKey)

	if action == "" {
		switch gCtx.Request.Method {
		case http.MethodGet:
			action = ActionRead
		case http.MethodPost:
			action = ActionCreate
		case http.MethodPut:
			action = ActionUpdate
		case http.MethodDelete:
			action = ActionDestroy
		}
	}

	return Action{
		Action: action,
		Flags:  FlagsValidator(action, strings.Split(flagsStr, ",")),
		Fields: strings.Split(fieldsStr, ","),
	}
}

func FlagsValidator(action string, flags []string) []string {
	return flags
}

func HasFlags(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

type Parameters struct {
	Name        string `json:"name"`
	Content     []byte `json:"content"`
	Destination string `json:"destination"`
}
