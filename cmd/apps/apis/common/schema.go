package common

import "github.com/gin-gonic/gin"

type Response struct {
	Status int         `json:"status"`
	Error  *Error      `json:"error,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

func ErrorResponse(gCtx *gin.Context, status int, code ApiErrorCode, err error) {
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
