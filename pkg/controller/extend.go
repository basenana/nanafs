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
	"encoding/base64"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"runtime/trace"
	"strings"
)

const (
	attrSourcePluginPrefix = "org.basenana.plugin.source/"
)

func (c *controller) GetEntryExtendField(ctx context.Context, id int64, fKey string) (*string, bool, error) {
	defer trace.StartRegion(ctx, "controller.GetEntryExtendField").End()
	result, err := c.entry.GetEntryExtendField(ctx, id, fKey)
	if err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func (c *controller) SetEntryExtendField(ctx context.Context, id int64, fKey, fVal string, encoded bool) error {
	defer trace.StartRegion(ctx, "controller.SetEntryExtendField").End()
	c.logger.Debugw("set entry extend filed", "entry", id, "key", fKey)

	if strings.HasPrefix(fKey, attrSourcePluginPrefix) {
		scope, err := buildPluginScopeFromAttr(fKey, fVal, encoded)
		if err != nil {
			c.logger.Errorw("build plugin scope from attr failed",
				"entry", id, "key", fKey, "val", fVal, "err", err)
			return types.ErrUnsupported
		}
		return c.ConfigEntrySourcePlugin(ctx, id, scope)
	}

	if fKey == types.LabelKeyGroupFeedID {
		if encoded {
			var err error
			fVal, err = utils.DecodeBase64(fVal)
			if err != nil {
				c.logger.Errorw("set group feed failed: decode base64 error", "val", fVal, "err", err)
				return err
			}
		}
		if err := c.EnableGroupFeed(ctx, id, strings.TrimSpace(fVal)); err != nil {
			c.logger.Errorw("enable group feed failed", "val", fVal, "err", err)
			return types.ErrUnsupported
		}
		return nil
	}

	err := c.entry.SetEntryExtendField(ctx, id, fKey, fVal)
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
	if fKey == types.LabelKeyGroupFeedID {
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

	// TODO: using document manager
	// TODO: check feed id is already existed?
	labels, err := c.entry.GetEntryLabels(ctx, id)
	if err != nil {
		c.logger.Errorw("enable group feed failed when query labels", "entry", id, "err", err)
		return err
	}

	var (
		updated, found bool
	)
	for _, l := range labels.Labels {
		if l.Key == types.LabelKeyGroupFeedID {
			found = true
			if l.Value != feedID {
				l.Value = feedID
				updated = true
			}
			break
		}
	}

	if !found {
		labels.Labels = append(labels.Labels, types.Label{Key: types.LabelKeyGroupFeedID, Value: feedID})
		updated = true
	}

	if updated {
		err = c.entry.UpdateEntryLabels(ctx, id, labels)
		if err != nil {
			c.logger.Errorw("enable group feed failed when write back labels", "entry", id, "err", err)
			return err
		}
	}
	return nil
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

	labels, err := c.entry.GetEntryLabels(ctx, id)
	if err != nil {
		c.logger.Errorw("disable group feed failed when query lables", "entry", id, "err", err)
		return err
	}

	var (
		idx   int
		total = len(labels.Labels)
	)
	for idx = 0; idx < total; idx++ {
		if labels.Labels[idx].Key == types.LabelKeyGroupFeedID {
			break
		}
	}

	if idx == total {
		return nil
	}

	labels.Labels = append(labels.Labels[0:idx], labels.Labels[idx+1:total]...)

	err = c.entry.UpdateEntryLabels(ctx, id, labels)
	if err != nil {
		c.logger.Errorw("disable group feed failed when write back labels", "entry", id, "err", err)
		return err
	}
	return nil
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
			ed.Properties.Fields = map[string]string{}
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

func buildPluginScopeFromAttr(fKey, fVal string, encoded bool) (types.ExtendData, error) {
	if encoded {
		rawVal, err := base64.StdEncoding.DecodeString(fVal)
		if err != nil {
			return types.ExtendData{}, err
		}
		fVal = strings.TrimSpace(string(rawVal))
	}
	sourceCfgKey := strings.TrimPrefix(fKey, attrSourcePluginPrefix)
	switch sourceCfgKey {
	case "rssurl":
		return BuildRssPluginScopeFromURL(fVal)
	}
	return types.ExtendData{}, fmt.Errorf("unknown source attr config key: %s", fKey)
}
