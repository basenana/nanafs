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

const testJWTSecret = "test-secret-key-for-jwt-signing"

func TestNewClaimsWithUser(t *testing.T) {
	claims := NewClaimsWithUser("test-ns", 1000, 1001, "test@example.com", "google-123", time.Hour)

	assert.Equal(t, "test-ns", claims.Namespace)
	assert.Equal(t, int64(1000), claims.UID)
	assert.Equal(t, int64(1001), claims.GID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "google-123", claims.GoogleID)
}

func TestGenerateTokenWithUser(t *testing.T) {
	claims := NewClaimsWithUser("test-ns", 1000, 1001, "test@example.com", "google-123", time.Hour)

	token, err := claims.GenerateToken(testJWTSecret)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := ParseToken(token, testJWTSecret)
	require.NoError(t, err)
	assert.Equal(t, "test-ns", parsed.Namespace)
	assert.Equal(t, int64(1000), parsed.UID)
	assert.Equal(t, "test@example.com", parsed.Email)
	assert.Equal(t, "google-123", parsed.GoogleID)
}

func TestGoogleOAuthConfig(t *testing.T) {
	cfg := &GoogleOAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8080/callback",
	}

	assert.Equal(t, "client-id", cfg.ClientID)
	assert.Equal(t, "client-secret", cfg.ClientSecret)
	assert.Equal(t, "http://localhost:8080/callback", cfg.RedirectURL)
}

func TestGoogleUserInfo(t *testing.T) {
	info := &GoogleUserInfo{
		Sub:     "123456789",
		Name:    "Test User",
		Email:   "test@example.com",
		Picture: "https://example.com/photo.jpg",
	}

	assert.Equal(t, "123456789", info.Sub)
	assert.Equal(t, "Test User", info.Name)
	assert.Equal(t, "test@example.com", info.Email)
	assert.Equal(t, "https://example.com/photo.jpg", info.Picture)
}

func TestErrorDefinitions(t *testing.T) {
	assert.NotNil(t, ErrUserRequiresNamespace)
	assert.NotNil(t, ErrNamespaceAlreadyExists)
	assert.NotNil(t, ErrEmailAlreadyLinked)

	assert.Equal(t, "user requires namespace name", ErrUserRequiresNamespace.Error())
	assert.Equal(t, "namespace already exists", ErrNamespaceAlreadyExists.Error())
	assert.Equal(t, "email already linked to another account", ErrEmailAlreadyLinked.Error())
}
