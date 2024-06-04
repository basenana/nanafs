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

	"github.com/mmcdole/gofeed"

	"github.com/basenana/nanafs/pkg/types"
)

func (c *controller) QueryDocuments(ctx context.Context, query string) ([]*types.Document, error) {
	return c.document.QueryDocuments(ctx, query)
}

func (c *controller) ListDocuments(ctx context.Context, filter types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error) {
	result, err := c.document.ListDocuments(ctx, filter, order)
	if err != nil {
		c.logger.Errorw("list documents failed", "err", err)
		return nil, err
	}
	return result, nil
}

func (c *controller) ListDocumentGroups(ctx context.Context, parentId int64, filter types.DocFilter) ([]*types.Metadata, error) {
	defer trace.StartRegion(ctx, "controller.ListDocumentGroups").End()
	result, err := c.document.ListDocumentGroups(ctx, parentId, filter)
	if err != nil {
		c.logger.Errorw("list document parents failed", "parent", parentId, "err", err)
		return nil, err
	}
	return result, err
}

func (c *controller) GetDocument(ctx context.Context, id int64) (*types.Document, error) {
	return c.document.GetDocument(ctx, id)
}

func (c *controller) GetDocumentsByFeed(ctx context.Context, feedId string, count int) (*types.FeedResult, error) {
	result, err := c.document.GetDocsByFeedId(ctx, feedId, count)
	if err != nil {
		c.logger.Errorw("get document by feed failed", "feedid", feedId, "err", err)
		return nil, err
	}
	return result, nil
}

func (c *controller) GetDocumentsByEntryId(ctx context.Context, entryId int64) (*types.Document, error) {
	return c.document.GetDocumentByEntryId(ctx, entryId)
}

func (c *controller) UpdateDocument(ctx context.Context, doc *types.Document) error {
	return c.document.SaveDocument(ctx, doc)
}

func BuildRssPluginScopeFromURL(url string) (types.ExtendData, error) {
	siteName, siteURL, err := parseRssUrl(url)
	if err != nil {
		return types.ExtendData{}, fmt.Errorf("parse feed url %s failed: %s", url, err)
	}

	ps := types.PlugScope{
		PluginName: "rss",
		Version:    "1.0",
		PluginType: types.TypeSource,
		Parameters: map[string]string{
			"feed":         url,
			"file_type":    "webarchive",
			"clutter_free": "true",
		},
	}
	return types.ExtendData{
		Properties: types.Properties{
			Fields: map[string]types.PropertyItem{
				"site_name": {Value: siteName},
				"site_url":  {Value: siteURL},
				"feed_url":  {Value: url},
			},
		},
		PlugScope: &ps,
	}, nil
}

var (
	parseRssUrl = func(url string) (string, string, error) {
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(url)
		if err != nil {
			return "", "", err
		}
		return feed.Title, feed.Link, nil
	}
)
