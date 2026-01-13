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

package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecretKey = "test-secret-key-for-jwt-signing"

func TestGenerateAndParseToken(t *testing.T) {
	claims := NewClaims("test-namespace", 1000, 1000, time.Hour)

	token, err := claims.GenerateToken(testSecretKey)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := ParseToken(token, testSecretKey)
	require.NoError(t, err)
	assert.Equal(t, "test-namespace", parsed.Namespace)
	assert.Equal(t, int64(1000), parsed.UID)
	assert.Equal(t, int64(1000), parsed.GID)
}

func TestExpiredToken(t *testing.T) {
	claims := NewClaims("test-namespace", 1000, 1000, -time.Hour)

	token, err := claims.GenerateToken(testSecretKey)
	require.NoError(t, err)

	_, err = ParseToken(token, testSecretKey)
	assert.ErrorIs(t, err, ErrExpiredToken)
}

func TestInvalidToken(t *testing.T) {
	_, err := ParseToken("invalid-token", testSecretKey)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestWrongSecretKey(t *testing.T) {
	claims := NewClaims("test-namespace", 1000, 1000, time.Hour)

	token, err := claims.GenerateToken(testSecretKey)
	require.NoError(t, err)

	_, err = ParseToken(token, "wrong-secret-key")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestDifferentNamespaceAndUIDGID(t *testing.T) {
	testCases := []struct {
		namespace string
		uid       int64
		gid       int64
	}{
		{"personal", 0, 0},
		{"default", 1001, 1001},
		{"my-namespace", 65534, 65534},
	}

	for _, tc := range testCases {
		t.Run(tc.namespace, func(t *testing.T) {
			claims := NewClaims(tc.namespace, tc.uid, tc.gid, time.Hour)

			token, err := claims.GenerateToken(testSecretKey)
			require.NoError(t, err)

			parsed, err := ParseToken(token, testSecretKey)
			require.NoError(t, err)

			assert.Equal(t, tc.namespace, parsed.Namespace)
			assert.Equal(t, tc.uid, parsed.UID)
			assert.Equal(t, tc.gid, parsed.GID)
		})
	}
}
