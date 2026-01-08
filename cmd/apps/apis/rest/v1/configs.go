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
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
)

// GetConfig retrieves a single config
func (s *ServicesV1) GetConfig(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	group := ctx.Param("group")
	name := ctx.Param("name")

	value, err := s.meta.GetConfigValue(ctx.Request.Context(), caller.Namespace, group, name)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusNotFound, "CONFIG_NOT_FOUND", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ConfigResponse{
		Group: group,
		Name:  name,
		Value: value,
	})
}

// SetConfig sets a config value
func (s *ServicesV1) SetConfig(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	group := ctx.Param("group")
	name := ctx.Param("name")

	var req SetConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if err := s.meta.SetConfigValue(ctx.Request.Context(), caller.Namespace, group, name, req.Value); err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "SET_CONFIG_FAILED", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &SetConfigResponse{
		Group: group,
		Name:  name,
		Value: req.Value,
	})
}

// ListConfig lists configs by group
func (s *ServicesV1) ListConfig(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	group := ctx.Param("group")

	items, err := s.meta.ListConfigValues(ctx.Request.Context(), caller.Namespace, group)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "LIST_CONFIG_FAILED", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListConfigResponse{Items: items})
}

// DeleteConfig deletes a config
func (s *ServicesV1) DeleteConfig(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	group := ctx.Param("group")
	name := ctx.Param("name")

	if err := s.meta.DeleteConfigValue(ctx.Request.Context(), caller.Namespace, group, name); err != nil {
		apitool.ErrorResponse(ctx, http.StatusInternalServerError, "DELETE_CONFIG_FAILED", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &DeleteConfigResponse{
		Group:   group,
		Name:    name,
		Deleted: true,
	})
}
