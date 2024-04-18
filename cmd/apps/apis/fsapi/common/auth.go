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
	"fmt"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"strconv"
	"strings"
)

type AuthInfo struct {
	Authenticated bool
	AccessToken   string
	UID           int64
	GID           int64
	Namespace     string
}

type CallerAuthGetter func(ctx context.Context) AuthInfo

func CallerAuth(ctx context.Context) AuthInfo {
	ai := AuthInfo{Authenticated: false, UID: -1}
	p, ok := peer.FromContext(ctx)
	if ok && p.AuthInfo != nil {
		tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
		if !ok {
			return ai
		}
		if len(tlsInfo.State.VerifiedChains) > 0 &&
			len(tlsInfo.State.VerifiedChains[0]) > 0 {
			subject := tlsInfo.State.VerifiedChains[0][0].Subject

			if len(subject.Organization) == 0 {
				return ai
			}
			ai.Namespace = subject.Organization[0]

			if len(subject.OrganizationalUnit) == 0 {
				return ai
			}
			ai.AccessToken = subject.OrganizationalUnit[0]

			var err error
			ai.UID, ai.GID, err = parseUidAndGid(subject.CommonName)
			if err != nil {
				return ai
			}
			ai.Authenticated = true
		}
	}
	return ai
}

func parseUidAndGid(cn string) (uid, gid int64, err error) {
	cnParts := strings.Split(cn, ",")
	if len(cnParts) < 2 {
		err = fmt.Errorf("unexpect common name")
		return
	}
	uid, err = strconv.ParseInt(cnParts[0], 10, 64)
	if err != nil {
		return
	}
	gid, err = strconv.ParseInt(cnParts[1], 10, 64)
	if err != nil {
		return
	}
	return
}
