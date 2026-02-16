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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
)

var (
	ErrUserRequiresNamespace  = errors.New("user requires namespace name")
	ErrNamespaceAlreadyExists = errors.New("namespace already exists")
	ErrEmailAlreadyLinked     = errors.New("email already linked to another account")
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type GoogleUserInfo struct {
	Sub     string `json:"sub"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Picture string `json:"picture"`
}

type GoogleAuthService struct {
	config    *oauth2.Config
	jwtSecret string
	metaStore metastore.Meta
}

func NewGoogleAuthService(cfg *GoogleOAuthConfig, jwtSecret string, metaStore metastore.Meta) *GoogleAuthService {
	return &GoogleAuthService{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
			Endpoint:     google.Endpoint,
		},
		jwtSecret: jwtSecret,
		metaStore: metaStore,
	}
}

func (s *GoogleAuthService) GetAuthURL(state string) string {
	return s.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *GoogleAuthService) ExchangeCode(ctx context.Context, code string) (*GoogleUserInfo, error) {
	token, err := s.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	client := s.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}
	return &userInfo, nil
}

func (s *GoogleAuthService) LoginOrRegister(ctx context.Context, userInfo *GoogleUserInfo) (*Claims, *types.User, error) {
	user, err := s.metaStore.GetUserByGoogleID(ctx, userInfo.Sub)
	if err == nil && user != nil {
		claims := NewClaimsWithUser(user.Namespace, user.ID, 0, user.Email, user.GoogleID, 24*time.Hour)
		return claims, user, nil
	}

	existingUser, _ := s.metaStore.GetUserByEmail(ctx, userInfo.Email)
	if existingUser != nil {
		return nil, nil, ErrEmailAlreadyLinked
	}

	return nil, nil, ErrUserRequiresNamespace
}

func (s *GoogleAuthService) CreateUserWithNamespace(ctx context.Context, userInfo *GoogleUserInfo, namespaceName string) (*Claims, *types.User, error) {
	exists, err := s.metaStore.NamespaceExists(ctx, namespaceName)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, ErrNamespaceAlreadyExists
	}

	now := time.Now()
	user := &types.User{
		GoogleID:  userInfo.Sub,
		Email:     userInfo.Email,
		Name:      userInfo.Name,
		AvatarURL: userInfo.Picture,
		Namespace: namespaceName,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.metaStore.CreateUser(ctx, user); err != nil {
		return nil, nil, err
	}

	ns := &types.Namespace{
		Name:      namespaceName,
		OwnerID:   user.ID,
		CreatedAt: now,
	}
	if err := s.metaStore.CreateNamespace(ctx, ns); err != nil {
		return nil, nil, err
	}

	claims := NewClaimsWithUser(namespaceName, user.ID, 0, user.Email, user.GoogleID, 24*time.Hour)
	return claims, user, nil
}

func (s *GoogleAuthService) GenerateToken(claims *Claims) (string, error) {
	return claims.GenerateToken(s.jwtSecret)
}

func (s *GoogleAuthService) GetUser(ctx context.Context, userID int64) (*types.User, error) {
	return s.metaStore.GetUserByID(ctx, userID)
}

func (s *GoogleAuthService) GetUserByNamespace(ctx context.Context, namespace string) (*types.User, error) {
	return s.metaStore.GetUserByNamespace(ctx, namespace)
}

func (s *GoogleAuthService) GetNamespace(ctx context.Context, name string) (*types.Namespace, error) {
	return s.metaStore.GetNamespace(ctx, name)
}

func (s *GoogleAuthService) ListNamespaces(ctx context.Context) ([]*types.Namespace, error) {
	return s.metaStore.ListNamespaces(ctx)
}

func (s *GoogleAuthService) DeleteNamespace(ctx context.Context, name string) error {
	return s.metaStore.DeleteNamespace(ctx, name)
}
