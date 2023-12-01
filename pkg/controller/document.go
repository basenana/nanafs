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
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/basenana/nanafs/pkg/types"
)

func (c *controller) QueryDocuments(ctx context.Context, query string) ([]*types.Document, error) {
	return c.document.QueryDocuments(ctx, query)
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

	docs, err := c.document.ListDocuments(ctx, group.ID)
	if err != nil {
		c.logger.Errorw("get docs by parentId failed", "parentId", group.ID, "err", err)
		return nil, err
	}
	if len(docs) > count {
		docs = docs[:count]
	}

	docFeeds := make([]types.DocumentFeed, len(docs))
	for i, doc := range docs {
		eExtend, err := c.entry.GetEntryExtendData(ctx, doc.OID)
		if err != nil {
			c.logger.Errorw("get entry extendData failed", "feedid", feedId, "entry", doc.OID, "err", err)
			return nil, err
		}
		updatedAt := eExtend.Properties.Fields[rssPostMetaUpdatedAt].Value
		if updatedAt == "" {
			updatedAt = doc.CreatedAt.Format(time.RFC3339)
		}
		docFeeds[i] = types.DocumentFeed{
			ID:        eExtend.Properties.Fields[rssPostMetaID].Value,
			Title:     eExtend.Properties.Fields[rssPostMetaTitle].Value,
			Link:      eExtend.Properties.Fields[rssPostMetaLink].Value,
			UpdatedAt: updatedAt,
			Document:  *doc,
		}
	}
	groupFeed.Documents = docFeeds
	return groupFeed, nil
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
