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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"strconv"
)

const (
	// TODO: delete thsi
	insecure = true
)

type AuthInfo struct {
	Authenticated bool
	UID           int64
	GID           int64
	Namespace     []string
}

type CallerAuthGetter func(ctx context.Context) AuthInfo

func CallerAuth(ctx context.Context) AuthInfo {
	ai := AuthInfo{Authenticated: false, UID: -1}
	if insecure {
		ai.Authenticated = true
		ai.UID = 0
		ai.GID = 0
		return ai
	}
	p, ok := peer.FromContext(ctx)
	if ok && p.AuthInfo != nil {
		tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
		if !ok {
			return ai
		}
		if len(tlsInfo.State.VerifiedChains) > 0 &&
			len(tlsInfo.State.VerifiedChains[0]) > 0 {
			subject := tlsInfo.State.VerifiedChains[0][0].Subject

			var (
				tmpNum int64
				err    error
			)
			tmpNum, err = strconv.ParseInt(subject.CommonName, 10, 64)
			if err != nil {
				return ai
			}
			ai.UID = tmpNum
			ai.Namespace = subject.Organization
			ai.Authenticated = true
		}
	}
	return ai
}
