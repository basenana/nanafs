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
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
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
	if request.EntryId == 0 || request.ParentEntryId == 0 {
		return nil, fmt.Errorf("entry id or parent entry id is empty")
	}

	var action string
	if err := request.ContextResults.Load(pluginapi.ResEntryActionKey, &action); err == nil {
		d.log.Infof("do action %s", action)
		switch action {
		case "cleanup":
			if err := d.svc.RemoveEntry(ctx, request.ParentEntryId, request.EntryId); err != nil {
				return pluginapi.NewFailedResponse(fmt.Sprintf("cleanup entry %d error: %s", request.EntryId, err)), nil
			}
			return pluginapi.NewResponseWithResult(nil), nil
		}
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
