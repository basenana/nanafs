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

package restfs

import (
	"github.com/basenana/nanafs/cmd/apps/apis/restfs/v1"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/gin-gonic/gin"
)

const (
	restFsPath = "/v1/fs/*path"
)

type Handler interface {
	Get(gCtx *gin.Context)
	Post(gCtx *gin.Context)
	Put(gCtx *gin.Context)
	Delete(gCtx *gin.Context)
}

func InitRestFs(ctrl controller.Controller, engine *gin.Engine, cfg config.Config) error {
	if !cfg.ApiConfig.Enable {
		return nil
	}

	v1Handler := v1.NewRestFs(ctrl, cfg)

	engine.Any(restFsPath, v1Handler.HttpHandle)
	return nil
}
