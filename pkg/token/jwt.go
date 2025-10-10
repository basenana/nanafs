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

package token

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

const (
	Issuer                  = "nanafs"
	KeyID                   = "v1"
	AccessTokenAudienceName = "user.access-token"
	AccessTokenDuration     = 7 * 24 * time.Hour
)

type CallerAuthGetter func(ctx context.Context) AuthInfo

type AuthInfo struct {
	UID       int64  `json:"uid"`
	GID       int64  `json:"gid"`
	Namespace string `json:"namespace"`
	jwt.RegisteredClaims
}

func generateJWT(namespace string, uid, gid int64, audience string, expirationTime time.Time, secret []byte) (string, error) {
	registeredClaims := jwt.RegisteredClaims{
		Issuer:   Issuer,
		Audience: jwt.ClaimStrings{audience},
		IssuedAt: jwt.NewNumericDate(time.Now()),
		Subject:  fmt.Sprintf("/namespace/%s/group/%d/user%d", namespace, gid, uid),
	}
	if !expirationTime.IsZero() {
		registeredClaims.ExpiresAt = jwt.NewNumericDate(expirationTime)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &AuthInfo{
		UID:              uid,
		GID:              gid,
		Namespace:        namespace,
		RegisteredClaims: registeredClaims,
	})
	token.Header["kid"] = KeyID

	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func authenticateByJWT(accessToken string, secret []byte) (*AuthInfo, error) {
	ai := &AuthInfo{}
	_, err := jwt.ParseWithClaims(accessToken, ai, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Name {
			return nil, fmt.Errorf("unexpected access token signing method=%v, expect %v", t.Header["alg"], jwt.SigningMethodHS256)
		}
		if kid, ok := t.Header["kid"].(string); ok {
			if kid == "v1" {
				return []byte(secret), nil
			}
		}
		return nil, fmt.Errorf("unexpected access token kid=%v", t.Header["kid"])
	})
	if err != nil {
		return nil, fmt.Errorf("invalid or expired access token")
	}

	return ai, nil
}
