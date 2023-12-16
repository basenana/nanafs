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

	"github.com/basenana/friday/pkg/utils/files"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	SummaryPluginName    = "summary"
	SummaryPluginVersion = "1.0"
	DefaultSummaryToken  = 300
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

	if err := maxAITaskParallel.Acquire(ctx); err != nil {
		return nil, err
	}
	defer maxAITaskParallel.Release()

	if request.ParentProperties == nil {
		return nil, fmt.Errorf("parent properties is nil")
	}
	if enabled := request.ParentProperties[propertyKeyFridayEnabled]; enabled != "true" {
		return pluginapi.NewResponseWithResult(nil), nil
	}

	rawDocs := request.Parameter[pluginapi.ResEntryDocumentsKey]
	docs, ok := rawDocs.([]types.FDocument)
	if !ok || len(docs) == 0 {
		return nil, fmt.Errorf("content of entry %d is empty", request.EntryId)
	}

	buf := bytes.Buffer{}
	var docType string
	for _, doc := range docs {
		if doc.Metadata != nil {
			rawType, hasType := doc.Metadata["type"]
			if hasType {
				docType, _ = rawType.(string)
			}
		}
		buf.WriteString(doc.Content)
		buf.WriteString("\n")
	}

	trimmedContent := utils.ContentTrim(docType, buf.String())
	length := files.Length(trimmedContent)
	if length <= DefaultSummaryToken {
		i.logger(ctx).Infow("skip summary, length less than default summary token.", "length", length, "default token", DefaultSummaryToken, "entryId", request.EntryId)
		return pluginapi.NewResponseWithResult(map[string]any{}), nil
	}
	i.logger(ctx).Infow("get docs", "length", length, "entryId", request.EntryId)
	summary, usage, err := friday.SummaryFile(ctx, fmt.Sprintf("entry_%d", request.EntryId), trimmedContent)
	if err != nil {
		return pluginapi.NewFailedResponse(fmt.Sprintf("summary documents failed: %s", err)), nil
	}

	return pluginapi.NewResponseWithResult(map[string]any{
		pluginapi.ResEntryDocSummaryKey: map[string]any{
			"summary": summary,
			"usage":   usage,
		},
	}), nil
}
func (i *SummaryPlugin) logger(ctx context.Context) *zap.SugaredLogger {
	return utils.WorkflowJobLogger(ctx, i.log)
}

func NewSummaryPlugin(spec types.PluginSpec, scope types.PlugScope) (*SummaryPlugin, error) {
	return &SummaryPlugin{
		spec:  spec,
		scope: scope,
		log:   logger.NewLogger("summaryPlugin"),
	}, nil
}
