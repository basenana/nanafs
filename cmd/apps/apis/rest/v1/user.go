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

package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/pkg/auth"
	"github.com/basenana/nanafs/pkg/types"
)

type GoogleLoginRequest struct {
	Code string `json:"code" binding:"required"`
}

type GoogleLoginResponse struct {
	Token     string             `json:"token"`
	User      *UserResponse      `json:"user"`
	Namespace *NamespaceResponse `json:"namespace,omitempty"`
}

type GoogleAuthURLResponse struct {
	URL string `json:"url"`
}

type CreateNamespaceRequest struct {
	Name string `json:"name" binding:"required"`
}

type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Namespace string `json:"namespace"`
}

type NamespaceResponse struct {
	Name    string `json:"name"`
	OwnerID int64  `json:"owner_id"`
}

func (s *ServicesV1) GetGoogleAuthURL(ctx *gin.Context) {
	if s.googleAuth == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "NOT_CONFIGURED", errors.New("Google OAuth is not configured"))
		return
	}

	state := ctx.Query("state")
	url := s.googleAuth.GetAuthURL(state)
	ctx.JSON(http.StatusOK, GoogleAuthURLResponse{URL: url})
}

func (s *ServicesV1) GoogleLogin(ctx *gin.Context) {
	if s.googleAuth == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "NOT_CONFIGURED", errors.New("Google OAuth is not configured"))
		return
	}

	var req GoogleLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	userInfo, err := s.googleAuth.ExchangeCode(ctx.Request.Context(), req.Code)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "AUTH_FAILED", err)
		return
	}

	claims, user, err := s.googleAuth.LoginOrRegister(ctx.Request.Context(), userInfo)
	if err != nil {
		if err == auth.ErrUserRequiresNamespace {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "REQUIRE_NAMESPACE", errors.New("namespace name is required"))
			return
		}
		if err == auth.ErrEmailAlreadyLinked {
			apitool.ErrorResponse(ctx, http.StatusConflict, "EMAIL_LINKED", errors.New("email already linked to another account"))
			return
		}
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "AUTH_FAILED", err)
		return
	}

	token, err := s.googleAuth.GenerateToken(claims)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "TOKEN_FAILED", err)
		return
	}

	var ns *types.Namespace
	if user.Namespace != "" {
		ns, _ = s.googleAuth.GetNamespace(ctx.Request.Context(), user.Namespace)
	}

	ctx.JSON(http.StatusOK, GoogleLoginResponse{
		Token:     token,
		User:      toUserResponse(user),
		Namespace: toNamespaceResponse(ns),
	})
}

func (s *ServicesV1) GoogleLoginWithNamespace(ctx *gin.Context) {
	if s.googleAuth == nil {
		apitool.ErrorResponse(ctx, http.StatusServiceUnavailable, "NOT_CONFIGURED", errors.New("Google OAuth is not configured"))
		return
	}

	var req struct {
		Code      string `json:"code" binding:"required"`
		Namespace string `json:"namespace" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	userInfo, err := s.googleAuth.ExchangeCode(ctx.Request.Context(), req.Code)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "AUTH_FAILED", err)
		return
	}

	claims, user, err := s.googleAuth.CreateUserWithNamespace(ctx.Request.Context(), userInfo, req.Namespace)
	if err != nil {
		if err == auth.ErrNamespaceAlreadyExists {
			apitool.ErrorResponse(ctx, http.StatusConflict, "NAMESPACE_EXISTS", errors.New("namespace already exists"))
			return
		}
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "CREATE_FAILED", err)
		return
	}

	token, err := s.googleAuth.GenerateToken(claims)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "TOKEN_FAILED", err)
		return
	}

	ns, _ := s.googleAuth.GetNamespace(ctx.Request.Context(), user.Namespace)

	ctx.JSON(http.StatusOK, GoogleLoginResponse{
		Token:     token,
		User:      toUserResponse(user),
		Namespace: toNamespaceResponse(ns),
	})
}

func (s *ServicesV1) ListMyNamespace(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	if caller.Namespace == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "NO_NAMESPACE", errors.New("user has no namespace"))
		return
	}

	ns, err := s.googleAuth.GetNamespace(ctx.Request.Context(), caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", errors.New("namespace not found"))
		return
	}

	ctx.JSON(http.StatusOK, toNamespaceResponse(ns))
}

func (s *ServicesV1) CreateNamespace(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	if caller.Namespace != "" {
		apitool.ErrorResponse(ctx, http.StatusForbidden, "ALREADY_EXISTS", errors.New("user already has a namespace"))
		return
	}

	var req CreateNamespaceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	ns := &types.Namespace{
		Name:    req.Name,
		OwnerID: caller.UID,
	}

	if err := s.meta.CreateNamespace(ctx.Request.Context(), ns); err != nil {
		apitool.ErrorResponse(ctx, http.StatusConflict, "NAMESPACE_EXISTS", err)
		return
	}

	user, err := s.googleAuth.GetUser(ctx.Request.Context(), caller.UID)
	if err == nil && user != nil {
		user.Namespace = ns.Name
		s.meta.UpdateUser(ctx.Request.Context(), user)
	}

	ctx.JSON(http.StatusOK, toNamespaceResponse(ns))
}

func (s *ServicesV1) GetNamespace(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	name := ctx.Param("name")
	if name != caller.Namespace {
		apitool.ErrorResponse(ctx, http.StatusForbidden, "FORBIDDEN", errors.New("cannot access other user's namespace"))
		return
	}

	ns, err := s.googleAuth.GetNamespace(ctx.Request.Context(), name)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", errors.New("namespace not found"))
		return
	}

	ctx.JSON(http.StatusOK, toNamespaceResponse(ns))
}

func (s *ServicesV1) DeleteNamespace(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	name := ctx.Param("name")
	if name != caller.Namespace {
		apitool.ErrorResponse(ctx, http.StatusForbidden, "FORBIDDEN", errors.New("cannot delete other user's namespace"))
		return
	}

	if err := s.googleAuth.DeleteNamespace(ctx.Request.Context(), name); err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "DELETE_FAILED", err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "namespace deleted"})
}

func (s *ServicesV1) GetCurrentUser(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	user, err := s.googleAuth.GetUser(ctx.Request.Context(), caller.UID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", errors.New("user not found"))
		return
	}

	ctx.JSON(http.StatusOK, toUserResponse(user))
}

func toUserResponse(user *types.User) *UserResponse {
	if user == nil {
		return nil
	}
	return &UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		Namespace: user.Namespace,
	}
}

func toNamespaceResponse(ns *types.Namespace) *NamespaceResponse {
	if ns == nil {
		return nil
	}
	return &NamespaceResponse{
		Name:    ns.Name,
		OwnerID: ns.OwnerID,
	}
}
