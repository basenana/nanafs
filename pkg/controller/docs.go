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
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/mmcdole/gofeed"
)

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
			"file_type":    "html",
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
