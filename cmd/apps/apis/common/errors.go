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
