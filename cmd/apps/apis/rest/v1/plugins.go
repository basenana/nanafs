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
	plugintypes "github.com/basenana/plugin/types"
)

func (s *ServicesV1) ListPlugins(ctx *gin.Context) {
	plugins := s.workflow.ListPlugins(ctx.Request.Context())

	result := make([]*PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, toPluginInfo(&p))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListPluginsResponse{
		Plugins: result,
	})
}

func toPluginInfo(spec *plugintypes.PluginSpec) *PluginInfo {
	initParams := make([]PluginParameter, 0, len(spec.InitParameters))
	for _, p := range spec.InitParameters {
		initParams = append(initParams, PluginParameter{
			Name:        p.Name,
			Required:    p.Required,
			Default:     p.Default,
			Description: p.Description,
			Options:     p.Options,
		})
	}

	params := make([]PluginParameter, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		params = append(params, PluginParameter{
			Name:        p.Name,
			Required:    p.Required,
			Default:     p.Default,
			Description: p.Description,
			Options:     p.Options,
		})
	}

	return &PluginInfo{
		Name:           spec.Name,
		Version:        spec.Version,
		Type:           string(spec.Type),
		InitParameters: initParams,
		Parameters:     params,
	}
}
