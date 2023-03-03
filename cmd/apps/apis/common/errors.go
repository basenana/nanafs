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

package common

import (
	"github.com/basenana/nanafs/pkg/types"
	"net/http"
)

type ApiErrorCode string

const (
	ApiArgsError     ApiErrorCode = "ArgsError"
	ApiNoAccess      ApiErrorCode = "NoAccess"
	ApiNoPermits     ApiErrorCode = "NoPermits"
	ApiNotFoundError ApiErrorCode = "NotFound"
	ApiNotGroupError ApiErrorCode = "NotGroup"
	ApiIsGroupError  ApiErrorCode = "IsGroup"
	ApiNotEmptyError ApiErrorCode = "NotEmpty"
	ApiEntryExisted  ApiErrorCode = "EntryExisted"
	ApiInternalError ApiErrorCode = "InternalError"
)

type Error struct {
	Code    ApiErrorCode `json:"code"`
	Message string       `json:"message"`
}

func Error2ApiErrorCode(err error) (int, ApiErrorCode) {
	if err == nil {
		return http.StatusOK, "NoError"
	}
	switch err {
	case types.ErrNotFound:
		return http.StatusNotFound, ApiNotFoundError
	case types.ErrIsExist:
		return http.StatusBadRequest, ApiEntryExisted
	case types.ErrNoGroup:
		return http.StatusBadRequest, ApiNotGroupError
	case types.ErrNotEmpty:
		return http.StatusBadRequest, ApiNotEmptyError
	case types.ErrIsGroup:
		return http.StatusBadRequest, ApiIsGroupError
	case types.ErrNoAccess:
		return http.StatusForbidden, ApiNoAccess
	case types.ErrNoPerm:
		return http.StatusForbidden, ApiNoPermits
	case types.ErrNameTooLong:
		return http.StatusBadRequest, ApiArgsError
	case types.ErrOutOfFS:
		return http.StatusBadRequest, ApiArgsError
	}
	return http.StatusInternalServerError, ApiInternalError
}
