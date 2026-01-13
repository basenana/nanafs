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
	"strings"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/auth"
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

func AuthMiddleware(jwtConfig *config.JWT) gin.HandlerFunc {
	return func(gCtx *gin.Context) {
		caller := parseCallerInfo(gCtx, jwtConfig)
		if caller == nil {
			return
		}

		ctx := WithCallerInfo(gCtx.Request.Context(), caller)
		gCtx.Request = gCtx.Request.WithContext(ctx)

		gCtx.Next()
	}
}

func parseCallerInfo(gCtx *gin.Context, jwtConfig *config.JWT) *CallerInfo {
	if jwtConfig != nil && jwtConfig.SecretKey != "" {
		caller := tryParseJWT(gCtx, jwtConfig.SecretKey)
		if caller != nil {
			return caller
		}
		gCtx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing token"})
		return nil
	}
	return tryParseHeaders(gCtx)
}

func tryParseJWT(gCtx *gin.Context, jwtSecretKey string) *CallerInfo {
	authHeader := gCtx.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.ParseToken(tokenString, jwtSecretKey)
	if err != nil {
		return nil
	}

	return &CallerInfo{
		Namespace: claims.Namespace,
		UID:       claims.UID,
		GID:       claims.GID,
	}
}

func tryParseHeaders(gCtx *gin.Context) *CallerInfo {
	ns := gCtx.GetHeader(headerNamespace)
	if ns == "" {
		gCtx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "missing X-Namespace header or Authorization header",
		})
		return nil
	}

	uidStr := gCtx.GetHeader(headerUID)
	gidStr := gCtx.GetHeader(headerGID)

	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	gid, _ := strconv.ParseInt(gidStr, 10, 64)

	return &CallerInfo{
		Namespace: ns,
		UID:       uid,
		GID:       gid,
	}
}
