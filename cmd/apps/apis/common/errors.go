package common

type ApiErrorCode string

const (
	ApiArgsError     ApiErrorCode = "ArgsError"
	ApiNoPermits     ApiErrorCode = "NoPermits"
	ApiNotFoundError ApiErrorCode = "NotFound"
	ApiNotGroupError ApiErrorCode = "NotGroup"
	ApiEntryExisted  ApiErrorCode = "EntryExisted"
	ApiMethodError   ApiErrorCode = "MethodNotAllowed"
	ApiInternalError ApiErrorCode = "InternalError"
)