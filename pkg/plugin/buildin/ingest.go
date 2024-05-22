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

	"github.com/basenana/nanafs/utils"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	IngestPluginName    = "ingest"
	IngestPluginVersion = "1.0"
)

type IngestPlugin struct {
	spec  types.PluginSpec
	scope types.PlugScope
	svc   Services
	log   *zap.SugaredLogger
}

func (i *IngestPlugin) Name() string { return IngestPluginName }

func (i *IngestPlugin) Type() types.PluginType { return types.TypeProcess }

func (i *IngestPlugin) Version() string { return IngestPluginVersion }

func (i *IngestPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	if request.EntryId == 0 {
		return nil, fmt.Errorf("entry id is empty")
	}

	if request.ParentProperties == nil {
		return nil, fmt.Errorf("parent properties is nil")
	}

	var docs []types.FDocument
	if err := request.ContextResults.Load(pluginapi.ResEntryDocumentsKey, &docs); err != nil {
		return nil, fmt.Errorf("content of entry %d is empty", request.EntryId)
	}

	buf := bytes.Buffer{}
	var docType string
	for _, doc := range docs {
		if doc.Metadata != nil {
			docType = doc.Metadata["type"]
		}
		buf.WriteString(doc.Content)
		buf.WriteString("\n")
	}

	content := buf.String()
	trimmedContent := utils.ContentTrim(docType, content)

	i.logger(ctx).Infow("get docs", "length", buf.Len(), "entryId", request.EntryId)
	usage, err := friday.IngestFile(ctx, request.EntryId, request.ParentEntryId, fmt.Sprintf("entry_%d", request.EntryId), trimmedContent)
	if err != nil {
		return pluginapi.NewFailedResponse(fmt.Sprintf("ingest documents failed: %s", err)), nil
	}

	err = i.svc.CreateFridayAccount(ctx, &types.FridayAccount{
		RefID:          request.EntryId,
		RefType:        "entry",
		Type:           "ingest",
		Namespace:      request.Namespace,
		CompleteTokens: usage["completion_tokens"],
		PromptTokens:   usage["prompt_tokens"],
		TotalTokens:    usage["total_tokens"],
	})
	if err != nil {
		return pluginapi.NewFailedResponse(fmt.Sprintf("create account of ingest failed: %s", err)), nil
	}
	return pluginapi.NewResponseWithResult(map[string]any{
		pluginapi.ResEntryDocIngestKey: types.FLlmResult{Usage: usage},
	}), nil
}

func (i *IngestPlugin) logger(ctx context.Context) *zap.SugaredLogger {
	return utils.WorkflowJobLogger(ctx, i.log)
}

func NewIngestPlugin(spec types.PluginSpec, scope types.PlugScope, svc Services) (*IngestPlugin, error) {
	return &IngestPlugin{
		spec:  spec,
		scope: scope,
		svc:   svc,
		log:   logger.NewLogger("ingestPlugin"),
	}, nil
}
