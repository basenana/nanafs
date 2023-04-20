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
	"github.com/basenana/nanafs/config"
	"net/http"
)

const (
	userInfoContextKey = "ctx.user_info"
)

type UserInfo struct {
	UID, GID int64
}

func GetUserInfo(ctx context.Context) *UserInfo {
	rawUi := ctx.Value(userInfoContextKey)
	if rawUi == nil {
		return nil
	}
	return rawUi.(*UserInfo)
}

func BasicAuthHandler(h http.Handler, users []config.OverwriteUser) http.Handler {
	userPassMapper := map[string]*config.OverwriteUser{}
	for i := range users {
		u := users[i]
		userPassMapper[u.Username] = &u
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || userPassMapper[username] == nil || userPassMapper[username].Password != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorised.\n"))
			return
		}
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userInfoContextKey, &UserInfo{
			UID: userPassMapper[username].UID,
			GID: userPassMapper[username].GID,
		})))
	})
}
