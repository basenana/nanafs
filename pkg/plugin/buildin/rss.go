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
	"github.com/basenana/nanafs/utils"
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

	rssParameterFeed        = "feed"
	rssParameterFileType    = "file_type"
	rssParameterTimeout     = "timeout"
	rssParameterClutterFree = "clutter_free"

	rssPostMaxCollect = 50
)

var RssSourcePluginSpec = types.PluginSpec{
	Name:       RssSourcePluginName,
	Version:    RssSourcePluginVersion,
	Type:       types.TypeSource,
	Parameters: make(map[string]string),
	Customization: []types.PluginConfig{
		{Key: rssParameterFeed, Default: ""},
		{Key: rssParameterFileType, Default: "webarchive"},
		{Key: rssParameterTimeout, Default: "120"},
		{Key: rssParameterClutterFree, Default: "true"},
	},
}

type RssSourcePlugin struct {
	job    *types.WorkflowJob
	scope  types.PluginCall
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

func (r *RssSourcePlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	if request.ParentEntryId <= 0 {
		return nil, fmt.Errorf("invalid parent entry id: %d", request.ParentEntryId)
	}
	source, err := r.rssSources(request)
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
	results := []pluginapi.CollectManifest{{ParentEntry: source.EntryId, NewFiles: entries}}
	r.logger.Infow("sync rss finish", "baseEntry", source.EntryId, "entries", len(entries))

	resp := pluginapi.NewResponse()
	resp.NewEntries = results
	return resp, nil
}

func (r *RssSourcePlugin) rssSources(request *pluginapi.Request) (src rssSource, err error) {
	src.FeedUrl = pluginapi.GetParameter(rssParameterFeed, request, RssSourcePluginSpec, r.scope)
	if src.FeedUrl == "" {
		err = fmt.Errorf("feed url is empty")
		return
	}

	_, err = url.Parse(src.FeedUrl)
	if err != nil {
		err = fmt.Errorf("parse feed url failed: %s", err)
		return
	}

	src.FileType = pluginapi.GetParameter(rssParameterFileType, request, RssSourcePluginSpec, r.scope)
	if src.FileType == "" {
		src.FileType = archiveFileTypeHtml
	}

	timeoutStr := pluginapi.GetParameter(rssParameterTimeout, request, RssSourcePluginSpec, r.scope)
	src.Timeout, err = strconv.Atoi(timeoutStr)
	if err != nil {
		r.logger.Warnf("parse timeout error: %s", err)
		src.Timeout = 120
	}

	src.ClutterFree = pluginapi.GetParameter(rssParameterClutterFree, request, RssSourcePluginSpec, r.scope) == "true"
	src.Headers = make(map[string]string)

	for k, v := range r.scope.Parameters {
		if strings.HasPrefix(k, "header_") || strings.HasPrefix(k, "HEADER_") {
			headerKey := strings.TrimPrefix(k, "header_")
			headerKey = strings.TrimPrefix(headerKey, "HEADER_")
			src.Headers[k] = v
		}
	}
	for k, v := range request.Parameter {
		if vstr, ok := v.(string); ok {
			if strings.HasPrefix(k, "header_") || strings.HasPrefix(k, "HEADER_") {
				headerKey := strings.TrimPrefix(k, "header_")
				headerKey = strings.TrimPrefix(headerKey, "HEADER_")
				src.Headers[k] = vstr
			}
		}
	}
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

	r.logger.Infow("parse rss source", "feed", source.FeedUrl)
	siteURL, err := parseSiteURL(source.FeedUrl)
	if err != nil {
		r.logger.Errorw("parse rss site url failed", "feed", source.FeedUrl, "err", err)
		return nil, err
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

	syncCachedRecords := listCachedSyncRecords(cachedData)
	newEntries := make([]pluginapi.Entry, 0)
	for i, item := range feed.Items {
		if i > rssPostMaxCollect {
			r.logger.Infow("soo many post need to collect, skip", "collectLimit", rssPostMaxCollect)
			break
		}

		item.Link = absoluteURL(siteURL, item.Link)
		basicID := basicPostID(item.Link)

		// TODO: check last post time
		if _, ok := syncCachedRecords[basicID]; ok {
			continue
		}

		if item.Content == "" && item.Description != "" {
			item.Content = item.Description
		}

		r.logger.Infow("parse rss post", "link", item.Link)

		fileName := item.Title
		switch source.FileType {
		case archiveFileTypeUrl:
			fileName += ".url"
			buf := bytes.Buffer{}
			buf.WriteString("[InternetShortcut]")
			buf.WriteString("\n")
			buf.WriteString(fmt.Sprintf("URL=%s", item.Link))

			err = os.WriteFile(utils.SafetyFilePathJoin(workdir, fileName), buf.Bytes(), 0655)
			if err != nil {
				return nil, fmt.Errorf("pack to url file failed: %s", err)
			}

		case archiveFileTypeHtml:
			fileName += ".html"
			htmlContent := readableHtmlContent(item.Link, item.Title, item.Content)
			err = os.WriteFile(utils.SafetyFilePathJoin(workdir, fileName), []byte(htmlContent), 0655)
			if err != nil {
				return nil, fmt.Errorf("pack to html file failed: %s", err)
			}

		case archiveFileTypeRawHtml:
			fileName += ".html"
			p := packer.NewHtmlPacker()
			err = p.Pack(ctx, packer.Option{
				URL:              item.Link,
				FilePath:         utils.SafetyFilePathJoin(workdir, fileName),
				Timeout:          source.Timeout,
				ClutterFree:      source.ClutterFree,
				Headers:          source.Headers,
				EnablePrivateNet: enablePrivateNet,
			})
			if err != nil {
				r.logger.Warnw("pack to raw html file failed", "link", item.Link, "err", err)
				continue
			}

		case archiveFileTypeWebArchive:
			fileName += ".webarchive"
			p := packer.NewWebArchivePacker()
			err = p.Pack(ctx, packer.Option{
				URL:              item.Link,
				FilePath:         utils.SafetyFilePathJoin(workdir, fileName),
				Timeout:          source.Timeout,
				ClutterFree:      source.ClutterFree,
				Headers:          source.Headers,
				EnablePrivateNet: enablePrivateNet,
			})
			if err != nil {
				r.logger.Warnw("pack to webarchive failed", "link", item.Link, "err", err)
				continue
			}

		default:
			return nil, fmt.Errorf("unknown rss archive file type %s", source.FileType)
		}

		filePath := utils.SafetyFilePathJoin(workdir, fileName)
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
				types.PropertyWebPageTitle:    item.Title,
				types.PropertyWebPageURL:      item.Link,
				types.PropertyWebPageUpdateAt: updatedAt.Format(time.RFC3339),
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

func parseSiteURL(feed string) (string, error) {
	sURL, err := url.Parse(feed)
	if err != nil {
		return "", err
	}
	sURL.Path = ""
	return sURL.String(), nil
}

func absoluteURL(sitURL, link string) string {
	if strings.HasPrefix(link, "/") {
		return sitURL + link
	}
	return link
}

func BuildRssSourcePlugin(job *types.WorkflowJob, scope types.PluginCall) *RssSourcePlugin {
	return &RssSourcePlugin{job: job, scope: scope,
		logger: logger.NewLogger("rssPlugin").With(zap.String("job", job.Id))}
}

type rssSource struct {
	EntryId     int64             `json:"entry_id"`
	FeedUrl     string            `json:"feed_url"`
	FileType    string            `json:"file_type"`
	ClutterFree bool              `json:"clutter_free"`
	Timeout     int               `json:"timeout"`
	Headers     map[string]string `json:"headers"`
}
