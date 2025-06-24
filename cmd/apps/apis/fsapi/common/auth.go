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
	"errors"
	"github.com/basenana/nanafs/pkg/token"
	"google.golang.org/grpc/metadata"
	"strings"
)

func getAccessTokenFromMetadata(md metadata.MD) (string, error) {
	authorizationHeaders := md.Get("Authorization")
	if len(authorizationHeaders) == 0 {
		return "", errors.New("authorization header not found")
	}
	authHeaderParts := strings.Fields(authorizationHeaders[0])
	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		return "", errors.New("authorization header format must be Bearer {token}")
	}
	return authHeaderParts[1], nil
}

func Auth(ctx context.Context) *token.AuthInfo {
	raw := ctx.Value(authInfoContextKey)
	if raw == nil {
		return nil
	}
	return raw.(*token.AuthInfo)
}
