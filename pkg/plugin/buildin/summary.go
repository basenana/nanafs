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
	"bytes"
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	SummaryPluginName    = "summary"
	SummaryPluginVersion = "1.0"
)

type SummaryPlugin struct {
	spec  types.PluginSpec
	scope types.PlugScope
	log   *zap.SugaredLogger
}

func (i *SummaryPlugin) Name() string { return SummaryPluginName }

func (i *SummaryPlugin) Type() types.PluginType { return types.TypeProcess }

func (i *SummaryPlugin) Version() string { return SummaryPluginVersion }

func (i *SummaryPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	if request.EntryId == 0 {
		return nil, fmt.Errorf("entry id is empty")
	}

	rawDocs := request.Parameter[pluginapi.ResEntryDocumentsKey]
	docs, ok := rawDocs.([]types.FDocument)
	if !ok || len(docs) == 0 {
		return nil, fmt.Errorf("content of entry %d is empty", request.EntryId)
	}

	buf := bytes.Buffer{}
	for _, doc := range docs {
		buf.WriteString(doc.Content)
		buf.WriteString("\n")
	}

	i.log.Infow("get docs", "length", buf.Len(), "entryId", request.EntryId)
	summary, err := friday.SummaryFile(fmt.Sprintf("entry_%d", request.EntryId), buf.String())
	if err != nil {
		return pluginapi.NewFailedResponse(fmt.Sprintf("summary documents failed: %s", err)), nil
	}

	return pluginapi.NewResponseWithResult(map[string]any{
		pluginapi.ResEntryDocSummaryKey: summary,
	}), nil
}

func NewSummaryPlugin(spec types.PluginSpec, scope types.PlugScope) (*SummaryPlugin, error) {
	return &SummaryPlugin{
		spec:  spec,
		scope: scope,
		log:   logger.NewLogger("summaryPlugin"),
	}, nil
}
