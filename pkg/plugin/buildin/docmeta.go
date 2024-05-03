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
	"errors"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"strings"
	"time"
)

const (
	DocMetaPluginName    = "docmeta"
	DocMetaPluginVersion = "1.0"
)

type DocMetaPlugin struct {
	spec  types.PluginSpec
	scope types.PlugScope
	svc   Services
	log   *zap.SugaredLogger
}

func (d DocMetaPlugin) Name() string {
	return DocMetaPluginName
}

func (d DocMetaPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (d DocMetaPlugin) Version() string {
	return DocMetaPluginVersion
}

func (d DocMetaPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	if request.EntryId == 0 {
		return nil, fmt.Errorf("entry id is empty")
	}

	for k, v := range request.Parameter {
		if strings.HasPrefix(k, pluginapi.ResWorkflowKeyPrefix) {
			continue
		}

		valStr, ok := v.(string)
		if !ok {
			continue
		}

		if err := d.svc.SetEntryExtendField(ctx, request.EntryId, k, valStr, false); err != nil {
			return pluginapi.NewFailedResponse(fmt.Sprintf("update entry %d extend field %s error: %s", request.EntryId, k, err)), nil
		}

		d.log.Infof("set entey %d extend field %s=%s", request.EntryId, k, valStr)
	}

	doc, err := d.svc.GetDocumentByEntryId(ctx, request.EntryId)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return pluginapi.NewResponseWithResult(nil), nil
		}
		return pluginapi.NewFailedResponse(fmt.Sprintf("get document with entry id %d error: %s", request.EntryId, err)), nil
	}

	// TODO: move to summary plugin
	totalUsage := make(map[string]any)
	if request.ContextResults.IsSet(pluginapi.ResEntryDocSummaryKey) {
		var summaryVal = types.FLlmResult{}
		err = request.ContextResults.Load(pluginapi.ResEntryDocSummaryKey, &summaryVal)
		if err != nil {
			return nil, fmt.Errorf("load document summary error %w", err)
		}
		doc.Summary = summaryVal.Summary
		totalUsage["summary"] = summaryVal.Usage
	}
	if request.ContextResults.IsSet(pluginapi.ResEntryDocKeyWordsKey) {
		var keyWordsVal = types.FLlmResult{}
		err = request.ContextResults.Load(pluginapi.ResEntryDocKeyWordsKey, &keyWordsVal)
		if err != nil {
			return nil, fmt.Errorf("load document keywords error %w", err)
		}
		doc.KeyWords = keyWordsVal.Keywords
		totalUsage["keywords"] = keyWordsVal.Usage
	}

	for k, v := range totalUsage {
		u, ok := v.(map[string]int)
		if !ok {
			continue
		}
		err = d.svc.CreateFridayAccount(ctx, &types.FridayAccount{
			RefID:          doc.ID,
			RefType:        "document",
			Type:           k,
			CompleteTokens: u["completion_tokens"],
			PromptTokens:   u["prompt_tokens"],
			TotalTokens:    u["total_tokens"],
		})
		if err != nil {
			return pluginapi.NewFailedResponse(fmt.Sprintf("create account of %s failed: %s", k, err)), nil
		}
	}

	doc.ChangedAt = time.Now()
	err = d.svc.SaveDocument(ctx, doc)
	if err != nil {
		return pluginapi.NewFailedResponse(fmt.Sprintf("update document %d meta failed: %s", doc.ID, err)), nil
	}
	return pluginapi.NewResponseWithResult(nil), nil
}

func NewDocMetaPlugin(spec types.PluginSpec, scope types.PlugScope, svc Services) (*DocMetaPlugin, error) {
	return &DocMetaPlugin{
		spec:  spec,
		scope: scope,
		svc:   svc,
		log:   logger.NewLogger("docMetaPlugin"),
	}, nil
}
