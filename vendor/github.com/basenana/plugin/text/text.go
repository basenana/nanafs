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

package text

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"go.uber.org/zap"
)

const (
	pluginName    = "text"
	pluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:    pluginName,
	Version: pluginVersion,
	Type:    types.TypeProcess,
}

type TextPlugin struct {
	logger *zap.SugaredLogger
}

func NewTextPlugin(ps types.PluginCall) types.Plugin {
	return &TextPlugin{
		logger: logger.NewPluginLogger(pluginName, ps.JobID),
	}
}

func (p *TextPlugin) Name() string {
	return pluginName
}

func (p *TextPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (p *TextPlugin) Version() string {
	return pluginVersion
}

func (p *TextPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	action := api.GetStringParameter("action", request, "")
	content := api.GetStringParameter("content", request, "")

	if action == "" {
		return api.NewFailedResponse("action is required"), nil
	}

	if content == "" && action != "join" {
		return api.NewFailedResponse("content is required"), nil
	}

	p.logger.Infow("text started", "action", action)

	resultKey := api.GetStringParameter("result_key", request, "result")
	var result any
	var err error

	switch action {
	case "search":
		result, err = actionSearch(content, request)
	case "replace":
		result, err = actionReplace(content, request)
	case "regex":
		result, err = actionRegex(content, request)
	case "split":
		result, err = actionSplit(content, request)
	case "join":
		result, err = actionJoin(request)
	default:
		return api.NewFailedResponse(fmt.Sprintf("unknown action: %s", action)), nil
	}

	if err != nil {
		p.logger.Warnw("text action failed", "action", action, "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	p.logger.Infow("text completed", "action", action)

	results := map[string]any{
		resultKey: result,
	}

	return api.NewResponseWithResult(results), nil
}

func actionSearch(content string, request *api.Request) (any, error) {
	pattern := api.GetStringParameter("pattern", request, "")
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required for search action")
	}

	return strings.Contains(content, pattern), nil
}

func actionReplace(content string, request *api.Request) (any, error) {
	pattern := api.GetStringParameter("pattern", request, "")
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required for replace action")
	}

	replacement := api.GetStringParameter("replacement", request, "")
	if replacement == "" {
		return nil, fmt.Errorf("replacement is required for replace action")
	}

	countStr := api.GetStringParameter("count", request, "-1")
	count := -1
	fmt.Sscanf(countStr, "%d", &count)

	if count < 0 {
		return strings.ReplaceAll(content, pattern, replacement), nil
	}

	n := count
	for i := 0; i < n; i++ {
		idx := strings.Index(content, pattern)
		if idx == -1 {
			break
		}
		content = content[:idx] + replacement + content[idx+len(pattern):]
	}

	return content, nil
}

func actionRegex(content string, request *api.Request) (any, error) {
	pattern := api.GetStringParameter("pattern", request, "")
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required for regex action")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	result := re.FindString(content)
	return result, nil
}

func actionSplit(content string, request *api.Request) (any, error) {
	delimiter := api.GetStringParameter("delimiter", request, "")
	if delimiter == "" {
		delimiter = api.GetStringParameter("pattern", request, "")
	}
	if delimiter == "" {
		return nil, fmt.Errorf("delimiter or pattern is required for split action")
	}

	parts := strings.Split(content, delimiter)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result, nil
}

func actionJoin(request *api.Request) (any, error) {
	delimiter := api.GetStringParameter("delimiter", request, "")
	if delimiter == "" {
		return nil, fmt.Errorf("delimiter is required for join action")
	}

	itemsParam := api.GetStringParameter("items", request, "")
	if itemsParam == "" {
		return "", nil
	}

	// Split items by comma (default separator for list input)
	items := strings.Split(itemsParam, ",")
	// Trim spaces from each item
	for i, item := range items {
		items[i] = strings.TrimSpace(item)
	}
	return strings.Join(items, delimiter), nil
}
