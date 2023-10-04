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

package plugin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	ingestPluginName    = "ingest"
	ingestPluginVersion = "1.0"
)

type IngestPlugin struct {
}

var _ ProcessPlugin = &IngestPlugin{}

func (i *IngestPlugin) Name() string { return ingestPluginName }

func (i *IngestPlugin) Type() types.PluginType { return types.TypeIngest }

func (i *IngestPlugin) Version() string { return ingestPluginVersion }

func (i *IngestPlugin) Run(ctx context.Context, request *pluginapi.Request, params map[string]string) (*pluginapi.Response, error) {
	fileName := request.EntryId
	content := request.Parameter["content"]
	if content == "" {
		return nil, fmt.Errorf("content of %d is empty", fileName)
	}
	err := friday.IngestFile(strconv.FormatInt(fileName, 10), content.(string))
	if err != nil {
		return &pluginapi.Response{
			IsSucceed: false,
			Message:   err.Error(),
		}, err
	}
	return &pluginapi.Response{IsSucceed: true}, nil
}

func registerIngestPlugin(r *registry) {
	r.Register(
		ingestPluginName,
		types.PluginSpec{Name: ingestPluginName, Version: ingestPluginVersion, Type: types.TypeIngest, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return &IngestPlugin{}, nil
		},
	)
}
