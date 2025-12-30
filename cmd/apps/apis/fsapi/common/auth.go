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
	"strconv"

	"google.golang.org/grpc/metadata"
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

func getFromMetadata(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func getInt64FromMetadata(md metadata.MD, key string) int64 {
	val := getFromMetadata(md, key)
	if val == "" {
		return 0
	}
	v, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func NamespaceFromMetadata(md metadata.MD) string {
	return getFromMetadata(md, headerNamespace)
}

func UIDFromMetadata(md metadata.MD) int64 {
	return getInt64FromMetadata(md, headerUID)
}

func GIDFromMetadata(md metadata.MD) int64 {
	return getInt64FromMetadata(md, headerGID)
}

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
