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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/hyponet/webpage-packer/packer"
	"github.com/mmcdole/gofeed"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	RssSourcePluginName    = "rss"
	RssSourcePluginVersion = "1.0"

	archiveFileTypeUrl        = "url"
	archiveFileTypeHtml       = "html"
	archiveFileTypeRawHtml    = "rawhtml"
	archiveFileTypeWebArchive = "webarchive"

	rssPostMetaID        = "org.basenana.plugin.rss/id"
	rssPostMetaLink      = "org.basenana.plugin.rss/link"
	rssPostMetaTitle     = "org.basenana.plugin.rss/title"
	rssPostMetaUpdatedAt = "org.basenana.plugin.rss/updated_at"
)

type rssSource struct {
	EntryId     int64             `json:"entry_id"`
	FeedUrl     string            `json:"feed_url"`
	FileType    string            `json:"file_type"`
	ClutterFree bool              `json:"clutter_free"`
	Timeout     int               `json:"timeout"`
	Parameters  map[string]string `json:"parameters"`
}

type RssSourcePlugin struct {
	spec   types.PluginSpec
	scope  types.PlugScope
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

func (r *RssSourcePlugin) SourceInfo() (string, error) {
	source, err := r.rssSources(r.scope.Parameters)
	if err != nil {
		r.logger.Errorw("get source info from rss plugin params failed", "err", err)
		return "", err
	}
	return source.FeedUrl, nil
}

func (r *RssSourcePlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	if request.ParentEntryId <= 0 {
		return nil, fmt.Errorf("invalid parent entry id: %d", request.ParentEntryId)
	}
	source, err := r.rssSources(r.scope.Parameters)
	if err != nil {
		r.logger.Errorw("get rss source failed", "err", err)
		return nil, err
	}
	source.EntryId = request.ParentEntryId

	entries, err := r.syncRssSource(ctx, source, request)
	if err != nil {
		r.logger.Warnw("sync rss failed", "source", source.FeedUrl, "err", err)
		return pluginapi.NewFailedResponse(fmt.Sprintf("sync rss failed: %s", err)), nil
	}
	results := []pluginapi.CollectManifest{{BaseEntry: source.EntryId, NewFiles: entries}}
	r.logger.Infow("sync rss finish", "baseEntry", source.EntryId, "entries", len(entries))

	return pluginapi.NewResponseWithResult(map[string]any{pluginapi.ResCollectManifests: results}), nil
}

func (r *RssSourcePlugin) rssSources(pluginParams map[string]string) (src rssSource, err error) {
	src.FeedUrl = pluginParams["feed"]
	if src.FeedUrl == "" {
		err = fmt.Errorf("feed url is empty")
		return
	}

	_, err = url.Parse(src.FeedUrl)
	if err != nil {
		err = fmt.Errorf("parse feed url failed: %s", err)
		return
	}

	src.FileType = pluginParams["file_type"]
	if src.FileType == "" {
		src.FileType = archiveFileTypeHtml
	}

	src.Timeout = 120
	if timeoutStr, ok := pluginParams["timeout"]; ok {
		src.Timeout, err = strconv.Atoi(timeoutStr)
		if err != nil {
			return src, fmt.Errorf("parse timeout error: %s", err)
		}
	}
	if clutterFreeStr, ok := pluginParams["clutter_free"]; ok {
		switch clutterFreeStr {
		case "yes", "true":
			src.ClutterFree = true
		default:
			src.ClutterFree = false
		}
	}

	src.Parameters = pluginParams
	return
}

func (r *RssSourcePlugin) syncRssSource(ctx context.Context, source rssSource, request *pluginapi.Request) ([]pluginapi.Entry, error) {
	var (
		workdir    = request.WorkPath
		cachedData = request.CacheData
		nowTime    = time.Now()
	)

	if cachedData == nil {
		return nil, fmt.Errorf("cached data is nil")
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(source.FeedUrl, ctx)
	if err != nil {
		return nil, err
	}

	// using html file when using RSSHub
	// https://github.com/DIYgod/RSSHub
	if feed.Generator == "RSSHub" {
		r.logger.Infow("using html file for RSSHub source")
		source.FileType = archiveFileTypeHtml
	}

	headers := make(map[string]string)
	for k, v := range source.Parameters {
		if strings.HasPrefix(k, "header_") || strings.HasPrefix(k, "HEADER_") {
			headerKey := strings.TrimPrefix(k, "header_")
			headerKey = strings.TrimPrefix(headerKey, "HEADER_")
			headers[k] = v
		}
	}

	syncCachedRecords := listCachedSyncRecords(cachedData)
	newEntries := make([]pluginapi.Entry, 0)
	for _, item := range feed.Items {
		basicID := basicPostID(item.Link)

		// TODO: check last post time
		if _, ok := syncCachedRecords[basicID]; ok {
			continue
		}

		if item.Content == "" && item.Description != "" {
			item.Content = item.Description
		}

		fileName := item.Title
		switch source.FileType {
		case archiveFileTypeUrl:
			fileName += ".url"
			buf := bytes.Buffer{}
			buf.WriteString("[InternetShortcut]")
			buf.WriteString("\n")
			buf.WriteString(fmt.Sprintf("URL=%s", item.Link))

			err = os.WriteFile(safetyFilePathJoin(workdir, fileName), buf.Bytes(), 0655)
			if err != nil {
				return nil, fmt.Errorf("pack to url file failed: %s", err)
			}

		case archiveFileTypeHtml:
			fileName += ".html"
			htmlContent := readableHtmlContent(item.Link, item.Title, item.Content)
			err = os.WriteFile(safetyFilePathJoin(workdir, fileName), []byte(htmlContent), 0655)
			if err != nil {
				return nil, fmt.Errorf("pack to html file failed: %s", err)
			}

		case archiveFileTypeRawHtml:
			fileName += ".html"
			p := packer.NewHtmlPacker()
			err = p.Pack(ctx, packer.Option{
				URL:         item.Link,
				FilePath:    safetyFilePathJoin(workdir, fileName),
				Timeout:     source.Timeout,
				ClutterFree: source.ClutterFree,
				Headers:     headers,
			})
			if err != nil {
				return nil, fmt.Errorf("pack to raw html file failed: %s", err)
			}

		case archiveFileTypeWebArchive:
			fileName += ".webarchive"
			p := packer.NewWebArchivePacker()
			err = p.Pack(ctx, packer.Option{
				URL:         item.Link,
				FilePath:    safetyFilePathJoin(workdir, fileName),
				Timeout:     source.Timeout,
				ClutterFree: source.ClutterFree,
				Headers:     headers,
			})
			if err != nil {
				return nil, fmt.Errorf("pack to webarchive failed: %s", err)
			}

		default:
			return nil, fmt.Errorf("unknown rss archive file type %s", source.FileType)
		}

		filePath := safetyFilePathJoin(workdir, fileName)
		fInfo, err := os.Stat(filePath)
		if err != nil {
			return nil, fmt.Errorf("stat archive file error: %s", err)
		}

		updatedAtSelect := []*time.Time{item.UpdatedParsed, item.PublishedParsed, &nowTime}
		var updatedAt *time.Time
		for i := range updatedAtSelect {
			if updatedAt = updatedAtSelect[i]; updatedAt != nil {
				break
			}
		}

		newEntries = append(newEntries, pluginapi.Entry{
			Name: path.Base(filePath),
			Kind: types.RawKind,
			Size: fInfo.Size(),
			Parameters: map[string]string{
				rssPostMetaID:        item.GUID,
				rssPostMetaTitle:     item.Title,
				rssPostMetaLink:      item.Link,
				rssPostMetaUpdatedAt: updatedAt.Format(time.RFC3339),
			},
			IsGroup: false,
		})
		setCachedSyncRecord(cachedData, basicID)
	}
	return newEntries, nil
}

type rssSyncRecord struct {
	ID     string
	SyncAt time.Time
}

func listCachedSyncRecords(data *pluginapi.CachedData) map[string]rssSyncRecord {
	result := make(map[string]rssSyncRecord)
	rawDataList := data.ListItems("posts")
	for _, rawData := range rawDataList {
		r := rssSyncRecord{}
		if err := json.Unmarshal([]byte(rawData), &r); err != nil {
			continue
		}
		result[r.ID] = r
	}
	return result
}

func setCachedSyncRecord(data *pluginapi.CachedData, postID string) {
	record := &rssSyncRecord{ID: postID, SyncAt: time.Now()}
	rawData, _ := json.Marshal(record)
	data.SetItem("posts", postID, string(rawData))
}

func basicPostID(postURL string) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(postURL)))
}

func BuildRssSourcePlugin(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) *RssSourcePlugin {
	return &RssSourcePlugin{spec: spec, scope: scope, logger: logger.NewLogger("rssPlugin")}
}
