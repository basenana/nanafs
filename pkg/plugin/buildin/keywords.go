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

package buildin

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	KeywordsPluginName    = "keywords"
	KeywordsPluginVersion = "1.0"
)

type KeywordsPlugin struct {
	spec  types.PluginSpec
	scope types.PlugScope
	log   *zap.SugaredLogger
}

func (i *KeywordsPlugin) Name() string { return KeywordsPluginName }

func (i *KeywordsPlugin) Type() types.PluginType { return types.TypeProcess }

func (i *KeywordsPlugin) Version() string { return KeywordsPluginVersion }

func (i *KeywordsPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	if request.EntryId == 0 {
		return nil, fmt.Errorf("entry id is empty")
	}

	if request.ParentProperties == nil {
		return nil, fmt.Errorf("parent properties is nil")
	}
	if enabled := request.ParentProperties[propertyKeyFridayEnabled]; enabled != "true" {
		return pluginapi.NewResponseWithResult(nil), nil
	}

	rawSummary := request.Parameter[pluginapi.ResEntryDocSummaryKey]
	summery, ok := rawSummary.(string)
	if !ok || len(summery) == 0 {
		return nil, fmt.Errorf("summary of entry %d is empty", request.EntryId)
	}

	i.log.Infow("get summary", "entryId", request.EntryId)
	keywords, err := friday.Keywords(summery)
	if err != nil {
		return pluginapi.NewFailedResponse(fmt.Sprintf("summary documents failed: %s", err)), nil
	}

	return pluginapi.NewResponseWithResult(map[string]any{
		pluginapi.ResEntryDocKeyWordsKey: keywords,
	}), nil
}

func NewKeyWordsPlugin(spec types.PluginSpec, scope types.PlugScope) (*KeywordsPlugin, error) {
	return &KeywordsPlugin{
		spec:  spec,
		scope: scope,
		log:   logger.NewLogger("keywordsPlugin"),
	}, nil
}
