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
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
)

const (
	RssSourcePluginName    = "rss"
	RssSourcePluginVersion = "1.0"
)

type rssSource struct {
	obj        *types.Object
	parameters map[string]string
}

type RssSourcePlugin struct {
	meta   storage.Meta
	logger *zap.SugaredLogger
}

func (r *RssSourcePlugin) Name() string {
	return RssSourcePluginName
}

func (r *RssSourcePlugin) Type() types.PluginType {
	return types.TypeSource
}

func (r *RssSourcePlugin) Version() string {
	return RssSourcePluginVersion
}

func (r *RssSourcePlugin) listRssSources(ctx context.Context) ([]rssSource, error) {
	objects, err := r.meta.ListObjects(ctx, types.Filter{
		Label: types.LabelMatch{Include: []types.Label{{Key: types.LabelPluginNameKey, Value: RssSourcePluginName}}},
	})
	if err != nil {
		return nil, err
	}

	result := make([]rssSource, 0, len(objects))
	for i, obj := range objects {
		if !obj.IsGroup() {
			r.logger.Warnf("object %s not a group, skip", obj.ID)
			continue
		}
		plugScope := obj.PlugScope
		if plugScope == nil {
			r.logger.Warnf("object %s has no plugscope data", obj.ID)
			continue
		}

		result = append(result, rssSource{obj: objects[i], parameters: plugScope.Parameters})
	}
	return result, nil
}

func (r *RssSourcePlugin) Run(ctx context.Context, request *common.Request, params map[string]string) (*common.Response, error) {
	rssSourceList, err := r.listRssSources(ctx)
	if err != nil {
		r.logger.Errorw("list rss source failed", "err", err)
		return nil, err
	}

	for i := range rssSourceList {
		source := rssSourceList[i]
		r.syncRssSource(ctx, source, params)
	}

	resp := &common.Response{
		IsSucceed: true,
	}
	return resp, nil
}

func (r *RssSourcePlugin) syncRssSource(ctx context.Context, source rssSource, params map[string]string) {
}

func InitRssSourcePlugin() *RssSourcePlugin {
	return nil
}
