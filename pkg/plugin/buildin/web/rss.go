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

package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/basenana/nanafs/utils"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hyponet/webpage-packer/packer"
	"github.com/mmcdole/gofeed"
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
	Name:    RssSourcePluginName,
	Version: RssSourcePluginVersion,
	Type:    types.TypeSource,
}

type RssSourcePlugin struct{}

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
	logger := pluginapi.Log(request, RssSourcePluginName)
	source, err := r.rssSources(request, logger)
	if err != nil {
		logger.Errorw("get rss source failed", "err", err)
		return nil, err
	}
	logger.Infow("syncing rss", "feed", source.FeedUrl)

	result, err := r.syncRssSource(ctx, source, request.WorkingPath, logger)
	if err != nil {
		logger.Warnw("sync rss failed", "source", source.FeedUrl, "err", err)
		return pluginapi.NewFailedResponse(fmt.Sprintf("sync rss failed: %s", err)), nil
	}

	resp := pluginapi.NewResponseWithResult(result)
	return resp, nil
}

func (r *RssSourcePlugin) rssSources(request *pluginapi.Request, logger *zap.SugaredLogger) (src rssSource, err error) {
	src.FeedUrl = request.Parameter["feed"]
	if src.FeedUrl == "" {
		err = fmt.Errorf("feed url is empty")
		return
	}

	_, err = url.Parse(src.FeedUrl)
	if err != nil {
		err = fmt.Errorf("parse feed url failed: %s", err)
		return
	}

	src.FileType = request.Parameter["file_type"]
	if src.FileType == "" {
		src.FileType = archiveFileTypeWebArchive
	}

	timeoutStr := request.Parameter["timeout"]
	if timeoutStr == "" {
		timeoutStr = "120"
	}
	src.Timeout, err = strconv.Atoi(timeoutStr)
	if err != nil {
		logger.Warnf("parse timeout error: %s", err)
		src.Timeout = 120
	}

	src.ClutterFree = request.Parameter["clutter_free"] == "true" || request.Parameter["clutter_free"] == ""
	src.Headers = make(map[string]string)

	for k, vstr := range request.Parameter {
		if strings.HasPrefix(k, "header_") || strings.HasPrefix(k, "HEADER_") {
			headerKey := strings.TrimPrefix(k, "header_")
			headerKey = strings.TrimPrefix(headerKey, "HEADER_")
			src.Headers[k] = vstr
		}
	}
	src.Store = request.ContextStore
	src.Namespace = request.Namespace
	return
}

func (r *RssSourcePlugin) syncRssSource(ctx context.Context, source rssSource, workdir string, logger *zap.SugaredLogger) (map[string]any, error) {
	var nowTime = time.Now()
	siteURL, err := parseSiteURL(source.FeedUrl)
	if err != nil {
		logger.Errorw("parse rss site url failed", "feed", source.FeedUrl, "err", err)
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
		logger.Infow("using html file for RSSHub source")
		source.FileType = archiveFileTypeHtml
	}

	var (
		articles = make([]map[string]any, 0)
		links    []string
	)

	for i, item := range feed.Items {
		if i > rssPostMaxCollect {
			logger.Infow("soo many post need to collect, skip", "collectLimit", rssPostMaxCollect)
			break
		}

		item.Link = absoluteURL(siteURL, item.Link)
		if item.Content == "" && item.Description != "" {
			item.Content = item.Description
		}

		if isNew, err := source.isNew(ctx, item.Link); err != nil || !isNew {
			if err != nil {
				logger.Errorw("check if feed is new", "feed", source.FeedUrl, "err", err)
			}
			continue
		}

		logger.Infow("parse rss post", "link", item.Link)

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
				logger.Warnw("pack to raw html file failed", "link", item.Link, "err", err)
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
				logger.Warnw("pack to webarchive failed", "link", item.Link, "err", err)
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

		updatedAtSelect := []*time.Time{item.UpdatedParsed, item.PublishedParsed}
		var updatedAt *time.Time
		for i := range updatedAtSelect {
			if updatedAt = updatedAtSelect[i]; updatedAt != nil {
				break
			}
		}

		if updatedAt == nil {
			updatedAt = &nowTime
		}

		links = append(links, item.Link)
		articles = append(articles, map[string]any{
			"file_path":  path.Base(filePath),
			"size":       fInfo.Size(),
			"title":      item.Title,
			"url":        item.Link,
			"updated_at": updatedAt.Format(time.RFC3339),
		})
	}

	if err := source.record(ctx, links...); err != nil {
		logger.Warnw("record links failed", "err", err)
	}

	logger.Infow("sync rss finish", "entries", len(articles))

	return map[string]any{"articles": articles}, nil
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

func BuildRssSourcePlugin() *RssSourcePlugin {
	return &RssSourcePlugin{}
}

type rssSource struct {
	FeedUrl     string
	FileType    string
	ClutterFree bool
	Timeout     int
	Headers     map[string]string

	Namespace string
	Store     pluginapi.ContextStore
}

func (s *rssSource) isNew(ctx context.Context, linkStr string) (bool, error) {
	var (
		k = string(fnv.New64a().Sum([]byte(linkStr)))
		v = make(map[string]string)
	)
	err := s.Store.LoadWorkflowContext(ctx, s.Namespace, RssSourcePluginName, "source", k, &v)
	if err == nil {
		return false, nil
	}

	if !errors.Is(err, types.ErrNotFound) {
		return false, err
	}

	v["time"] = time.Now().Format(time.RFC3339)
	err = s.Store.SaveWorkflowContext(ctx, s.Namespace, RssSourcePluginName, "source", k, &v)
	if err == nil {
		return false, nil
	}
	return true, nil
}

func (s *rssSource) record(ctx context.Context, linkList ...string) error {
	for _, linkStr := range linkList {
		var (
			k   = string(fnv.New64a().Sum([]byte(linkStr)))
			v   = map[string]string{"time": time.Now().Format(time.RFC3339)}
			err error
		)
		err = s.Store.SaveWorkflowContext(ctx, s.Namespace, RssSourcePluginName, "source", k, &v)
		if err != nil {
			return err
		}
	}
	return nil
}
