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

package apitool

import (
	"context"
	"fmt"
	"net/http"

	"github.com/basenana/nanafs/pkg/types"
)

const (
	userInfoContextKey = "ctx.user_info"
)

type UserInfo struct {
	UID, GID  int64
	Namespace string
}

func GetUserInfo(ctx context.Context) *UserInfo {
	rawUi := ctx.Value(userInfoContextKey)
	if rawUi == nil {
		return nil
	}
	return rawUi.(*UserInfo)
}

type TokenValidator interface {
	AccessToken(ctx context.Context, token string) (*types.AccessToken, error)
}

func BasicAuthHandler(h http.Handler, validator TokenValidator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, token, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorised.\n"))
			return
		}

		tokenInfo, err := validator.AccessToken(r.Context(), token)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(fmt.Sprintf("%s\n", err)))
			return
		}

		ctx := context.WithValue(r.Context(), userInfoContextKey, &UserInfo{
			UID:       tokenInfo.UID,
			GID:       tokenInfo.GID,
			Namespace: tokenInfo.Namespace,
		})
		ctx = types.WithNamespace(ctx, types.NewNamespace(tokenInfo.Namespace))

		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
