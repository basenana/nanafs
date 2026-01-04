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
	"go.uber.org/zap"

	"github.com/basenana/nanafs/cmd/apps/apis/rest/common"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow"
)

type ServicesV1 struct {
	meta     metastore.Meta
	core     core.Core
	workflow workflow.Workflow
	notify   *notify.Notify
	cfg      config.Config
	logger   *zap.SugaredLogger
}

func NewServicesV1(engine *gin.Engine, depends *common.Depends) (*ServicesV1, error) {
	s := &ServicesV1{
		meta:     depends.Meta,
		core:     depends.Core,
		workflow: depends.Workflow,
		notify:   depends.Notify,
		cfg:      depends.Config,
		logger:   logger.NewLogger("rest"),
	}

	engine.Use(common.AuthMiddleware())

	return s, nil
}

func (s *ServicesV1) caller(ctx *gin.Context) (*common.CallerInfo, error) {
	caller := common.Caller(ctx.Request.Context())
	if caller == nil {
		return nil, errors.New("unauthorized: missing caller info")
	}
	return caller, nil
}

func (s *ServicesV1) errorResponse(ctx *gin.Context, err error) {
	if err == nil {
		return
	}
	ctx.JSON(http.StatusInternalServerError, gin.H{
		"error": err.Error(),
	})
}

func (s *ServicesV1) successResponse(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}
