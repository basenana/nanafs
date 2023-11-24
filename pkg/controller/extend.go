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

package controller

import (
	"context"
	"fmt"
	"runtime/trace"
	"strings"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

const (
	attrSourcePluginPrefix = "org.basenana.plugin.source/"
	attrFeedId             = "org.basenana.document/feed"

	rssPostMetaID        = "org.basenana.plugin.rss/id"
	rssPostMetaLink      = "org.basenana.plugin.rss/link"
	rssPostMetaTitle     = "org.basenana.plugin.rss/title"
	rssPostMetaUpdatedAt = "org.basenana.plugin.rss/updated_at"
)

func (c *controller) GetEntryExtendField(ctx context.Context, id int64, fKey string) ([]byte, error) {
	defer trace.StartRegion(ctx, "controller.GetEntryExtendField").End()
	str, encoded, err := c.entry.GetEntryExtendField(ctx, id, fKey)
	if err != nil {
		return nil, err
	}

	if str == nil {
		return nil, nil
	}

	var result []byte
	if encoded {
		result, err = utils.DecodeBase64(*str)
		if err != nil {
			c.logger.Errorw("decode entry extend property error", "entry", id, "key", fKey)
			return nil, err
		}
	} else {
		result = []byte(*str)
	}

	return result, nil
}

func (c *controller) SetEntryExtendField(ctx context.Context, id int64, fKey string, fVal []byte) error {
	defer trace.StartRegion(ctx, "controller.SetEntryExtendField").End()
	c.logger.Debugw("set entry extend filed", "entry", id, "key", fKey)

	if strings.HasPrefix(fKey, attrSourcePluginPrefix) {
		scope, err := buildPluginScopeFromAttr(fKey, string(fVal))
		if err != nil {
			c.logger.Errorw("build plugin scope from attr failed",
				"entry", id, "key", fKey, "val", fVal, "err", err)
			return types.ErrUnsupported
		}
		return c.ConfigEntrySourcePlugin(ctx, id, scope)
	}

	if fKey == attrFeedId {
		if err := c.EnableGroupFeed(ctx, id, strings.TrimSpace(string(fVal))); err != nil {
			c.logger.Errorw("enable group feed failed", "val", fVal, "err", err)
			return types.ErrUnsupported
		}
		return nil
	}

	err := c.entry.SetEntryExtendField(ctx, id, fKey, utils.EncodeBase64(fVal), true)
	if err != nil {
		c.logger.Errorw("set entry extend filed failed", "entry", id, "key", fKey, "err", err)
		return err
	}
	return nil
}

func (c *controller) RemoveEntryExtendField(ctx context.Context, id int64, fKey string) error {
	defer trace.StartRegion(ctx, "controller.RemoveEntryExtendField").End()
	c.logger.Debugw("remove entry extend filed", "entry", id, "key", fKey)
	if strings.HasPrefix(fKey, attrSourcePluginPrefix) {
		return c.CleanupEntrySourcePlugin(ctx, id)
	}
	if fKey == attrFeedId {
		return c.DisableGroupFeed(ctx, id)
	}
	err := c.entry.RemoveEntryExtendField(ctx, id, fKey)
	if err != nil {
		c.logger.Errorw("remove entry extend filed failed", "entry", id, "key", fKey, "err", err)
		return err
	}
	return nil
}

func (c *controller) EnableGroupFeed(ctx context.Context, id int64, feedID string) error {
	defer trace.StartRegion(ctx, "controller.EnableGroupFeed").End()
	en, err := c.entry.GetEntry(ctx, id)
	if err != nil {
		c.logger.Errorw("enable group feed failed", "entry", id, "err", err)
		return err
	}
	if !types.IsGroup(en.Kind) {
		c.logger.Errorw("enable group feed failed", "entry", id, "err", types.ErrNoGroup)
		return types.ErrNoGroup
	}

	if len(feedID) == 0 {
		return fmt.Errorf("feed id is empty")
	}

	return c.document.EnableGroupFeed(ctx, id, feedID)
}

func (c *controller) DisableGroupFeed(ctx context.Context, id int64) error {
	defer trace.StartRegion(ctx, "controller.DisableGroupFeed").End()
	en, err := c.entry.GetEntry(ctx, id)
	if err != nil {
		c.logger.Errorw("disable group feed failed", "entry", id, "err", err)
		return err
	}
	if !types.IsGroup(en.Kind) {
		c.logger.Errorw("disable group feed failed", "entry", id, "err", types.ErrNoGroup)
		return err
	}

	return c.document.DisableGroupFeed(ctx, id)
}

func (c *controller) GetDocumentsByFeed(ctx context.Context, feedId string, count int) (*types.Feed, error) {
	group, err := c.document.GetGroupByFeedId(ctx, feedId)
	if err != nil {
		c.logger.Errorw("get group by feed failed", "feedid", feedId, "err", err)
		return nil, err
	}
	gExtend, err := c.entry.GetEntryExtendData(ctx, group.ID)
	if err != nil {
		c.logger.Errorw("get group extendData failed", "feedid", feedId, "entry", group.ID, "err", err)
		return nil, err
	}

	groupFeed := &types.Feed{
		FeedId:    feedId,
		GroupName: group.Name,
		SiteUrl:   gExtend.Properties.Fields[attrSourcePluginPrefix+"site_url"].Value,
		SiteName:  gExtend.Properties.Fields[attrSourcePluginPrefix+"site_name"].Value,
		FeedUrl:   gExtend.Properties.Fields[attrSourcePluginPrefix+"feed_url"].Value,
	}

	docs, err := c.document.GetDocsByFeedId(ctx, feedId, count)
	if err != nil {
		c.logger.Errorw("get docs by feed failed", "feedid", feedId, "err", err)
		return nil, err
	}

	docFeeds := make([]types.DocumentFeed, len(docs))
	for i, doc := range docs {
		eExtend, err := c.entry.GetEntryExtendData(ctx, doc.OID)
		if err != nil {
			c.logger.Errorw("get entry extendData failed", "feedid", feedId, "entry", doc.OID, "err", err)
			return nil, err
		}
		docFeeds[i] = types.DocumentFeed{
			ID:        eExtend.Properties.Fields[rssPostMetaID].Value,
			Title:     eExtend.Properties.Fields[rssPostMetaTitle].Value,
			Link:      eExtend.Properties.Fields[rssPostMetaLink].Value,
			UpdatedAt: eExtend.Properties.Fields[rssPostMetaUpdatedAt].Value,
			Document:  *doc,
		}
	}
	groupFeed.Documents = docFeeds
	return groupFeed, nil
}

func (c *controller) ConfigEntrySourcePlugin(ctx context.Context, id int64, patch types.ExtendData) error {
	defer trace.StartRegion(ctx, "controller.ConfigEntrySourcePlugin").End()
	c.logger.Infow("setup entry source plugin config", "entry", id)
	// todo: check group entry
	ed, err := c.entry.GetEntryExtendData(ctx, id)
	if err != nil {
		c.logger.Errorw("config entry source plugin encounter query entry extend data failed", "entry", id, "err", err)
		return err
	}

	ed.PlugScope = patch.PlugScope
	if len(patch.Properties.Fields) > 0 {
		if ed.Properties.Fields == nil {
			ed.Properties.Fields = map[string]types.PropertyItem{}
		}
		for k, v := range patch.Properties.Fields {
			ed.Properties.Fields[attrSourcePluginPrefix+k] = v
		}
	}

	err = c.entry.UpdateEntryExtendData(ctx, id, ed)
	if err != nil {
		c.logger.Errorw("config entry source plugin encounter write-back failed", "entry", id, "err", err)
		return err
	}

	var labels types.Labels
	labels, err = c.entry.GetEntryLabels(ctx, id)
	if err != nil {
		c.logger.Errorw("config entry source plugin encounter query entry labels failed", "entry", id, "err", err)
		return err
	}

	needAddLabels := map[string]string{
		types.LabelKeyPluginKind: string(ed.PlugScope.PluginType),
		types.LabelKeyPluginName: ed.PlugScope.PluginName,
	}
	for i, l := range labels.Labels {
		if val, ok := needAddLabels[l.Key]; ok {
			labels.Labels[i].Value = val
			delete(needAddLabels, l.Key)
		}
	}
	for k, v := range needAddLabels {
		labels.Labels = append(labels.Labels, types.Label{Key: k, Value: v})
	}
	err = c.entry.UpdateEntryLabels(ctx, id, labels)
	if err != nil {
		c.logger.Errorw("config entry source plugin encounter write-back entry labels failed", "entry", id, "err", err)
		return err
	}
	return nil
}

func (c *controller) CleanupEntrySourcePlugin(ctx context.Context, id int64) error {
	defer trace.StartRegion(ctx, "controller.CleanupEntrySourcePlugin").End()
	ed, err := c.entry.GetEntryExtendData(ctx, id)
	if err != nil {
		c.logger.Errorw("cleanup entry source plugin encounter query entry extend data failed", "entry", id, "err", err)
		return err
	}
	if ed.PlugScope == nil {
		return nil
	}
	if ed.PlugScope.PluginType != types.TypeSource {
		c.logger.Warnw("cleanup source plugin with non-source-plugin entry", "entry", id, "err", err)
		return nil
	}

	ed.PlugScope = nil
	if len(ed.Properties.Fields) > 0 {
		for k := range ed.Properties.Fields {
			if strings.HasPrefix(k, attrSourcePluginPrefix) {
				delete(ed.Properties.Fields, k)
			}
		}
	}
	err = c.entry.UpdateEntryExtendData(ctx, id, ed)
	if err != nil {
		c.logger.Errorw("cleanup entry source plugin encounter write-back failed", "entry", id, "err", err)
		return err
	}

	var labels, keepLabels types.Labels
	labels, err = c.entry.GetEntryLabels(ctx, id)
	if err != nil {
		c.logger.Errorw("cleanup entry source plugin encounter query entry labels failed", "entry", id, "err", err)
		return err
	}

	for i, l := range labels.Labels {
		if strings.HasPrefix(l.Key, types.LabelKeyPluginPrefix) {
			continue
		}
		keepLabels.Labels = append(keepLabels.Labels, labels.Labels[i])
	}
	err = c.entry.UpdateEntryLabels(ctx, id, keepLabels)
	if err != nil {
		c.logger.Errorw("cleanup entry source plugin encounter write-back entry labels failed", "entry", id, "err", err)
		return err
	}
	return nil
}

func buildPluginScopeFromAttr(fKey, fVal string) (types.ExtendData, error) {
	fVal = strings.TrimSpace(fVal)
	sourceCfgKey := strings.TrimPrefix(fKey, attrSourcePluginPrefix)
	switch sourceCfgKey {
	case "rssurl":
		return BuildRssPluginScopeFromURL(fVal)
	}
	return types.ExtendData{}, fmt.Errorf("unknown source attr config key: %s", fKey)
}
