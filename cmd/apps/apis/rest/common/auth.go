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
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CallerInfo struct {
	Namespace string
	UID       int64
	GID       int64
}

const (
	headerNamespace = "X-Namespace"
	headerUID       = "X-Uid"
	headerGID       = "X-Gid"
)

const callerInfoContextKey = "caller.info"

func WithCallerInfo(ctx context.Context, info *CallerInfo) context.Context {
	return context.WithValue(ctx, callerInfoContextKey, info)
}

func Caller(ctx context.Context) *CallerInfo {
	raw := ctx.Value(callerInfoContextKey)
	if raw == nil {
		return nil
	}
	return raw.(*CallerInfo)
}

func AuthMiddleware() gin.HandlerFunc {
	return func(gCtx *gin.Context) {
		ns := gCtx.GetHeader(headerNamespace)
		if ns == "" {
			gCtx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing X-Namespace header",
			})
			return
		}

		uidStr := gCtx.GetHeader(headerUID)
		gidStr := gCtx.GetHeader(headerGID)

		uid, _ := strconv.ParseInt(uidStr, 10, 64)
		gid, _ := strconv.ParseInt(gidStr, 10, 64)

		caller := &CallerInfo{
			Namespace: ns,
			UID:       uid,
			GID:       gid,
		}

		ctx := WithCallerInfo(gCtx.Request.Context(), caller)
		gCtx.Request = gCtx.Request.WithContext(ctx)

		gCtx.Next()
	}
}
