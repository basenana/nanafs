package schema

type ApiErrorCode string

const (
	ApiArgsError     ApiErrorCode = "ArgsError"
	ApiNoPermits     ApiErrorCode = "NoPermits"
	ApiNotFoundError ApiErrorCode = "NotFound"
	ApiMethodError   ApiErrorCode = "MethodNotAllowed"
	ApiInternalError ApiErrorCode = "InternalError"
)
