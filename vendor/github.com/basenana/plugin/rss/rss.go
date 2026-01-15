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

package rss

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"github.com/basenana/plugin/web"
	"go.uber.org/zap"

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

type RssSourcePlugin struct {
	logger      *zap.SugaredLogger
	fileRoot    *utils.FileAccess
	fileType    string
	timeout     int
	clutterFree bool
	headers     map[string]string
}

func NewRssPlugin(ps types.PluginCall) types.Plugin {
	fileType := ps.Params[rssParameterFileType]
	if fileType == "" {
		fileType = archiveFileTypeWebArchive
	}

	timeoutStr := ps.Params[rssParameterTimeout]
	timeout := 120
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = t
		}
	}

	clutterFree := true
	if v, ok := ps.Params[rssParameterClutterFree]; ok {
		v = strings.ToLower(v)
		clutterFree = v == "true" || v == "1"
	}

	headers := make(map[string]string)
	for k, v := range ps.Params {
		if strings.HasPrefix(k, "header_") || strings.HasPrefix(k, "HEADER_") {
			headers[k] = v
		}
	}

	return &RssSourcePlugin{
		logger:      logger.NewPluginLogger(RssSourcePluginName, ps.JobID),
		fileRoot:    utils.NewFileAccess(ps.WorkingPath),
		fileType:    fileType,
		timeout:     timeout,
		clutterFree: clutterFree,
		headers:     headers,
	}
}

type Article struct {
	FilePath  string `json:"file_path"`
	Size      int64  `json:"size"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	SiteURL   string `json:"site_url"`
	SiteName  string `json:"site_name"`
	UpdatedAt string `json:"updated_at"`
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

func (r *RssSourcePlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	source, err := r.rssSources(request)
	if err != nil {
		r.logger.Errorw("get rss source failed", "err", err)
		return nil, err
	}
	r.logger.Infow("syncing rss", "feed", source.FeedUrl)

	articles, err := r.syncRssSource(ctx, source)
	if err != nil {
		r.logger.Warnw("sync rss failed", "source", source.FeedUrl, "err", err)
		return api.NewFailedResponse(fmt.Sprintf("sync rss failed: %s", err)), nil
	}

	articleMaps := make([]map[string]interface{}, len(articles))
	for i := range articles {
		articleMaps[i] = utils.MarshalMap(articles[i])
	}

	resp := api.NewResponseWithResult(map[string]any{"articles": articleMaps})
	return resp, nil
}

func (r *RssSourcePlugin) rssSources(request *api.Request) (src rssSource, err error) {
	src.FeedUrl = api.GetStringParameter(rssParameterFeed, request, "")
	if src.FeedUrl == "" {
		err = fmt.Errorf("feed url is empty")
		return
	}

	_, err = url.Parse(src.FeedUrl)
	if err != nil {
		err = fmt.Errorf("parse feed url failed: %s", err)
		return
	}

	src.FileType = r.fileType
	src.Timeout = r.timeout
	src.ClutterFree = r.clutterFree
	src.Headers = r.headers
	src.Store = request.Store
	return
}

func (r *RssSourcePlugin) syncRssSource(ctx context.Context, source rssSource) ([]Article, error) {
	var nowTime = time.Now()
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

	var (
		articles = make([]Article, 0)
		links    []string
	)

	for i, item := range feed.Items {
		if i > rssPostMaxCollect {
			r.logger.Infow("soo many post need to collect, skip", "collectLimit", rssPostMaxCollect)
			break
		}

		item.Link = absoluteURL(siteURL, item.Link)
		if item.Content == "" && item.Description != "" {
			item.Content = item.Description
		}

		if isNew, err := source.isNew(ctx, item.Link); err != nil || !isNew {
			if err != nil {
				r.logger.Errorw("check if feed is new", "feed", source.FeedUrl, "err", err)
			}
			continue
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

			err = r.fileRoot.Write(fileName, buf.Bytes(), 0655)
			if err != nil {
				return nil, fmt.Errorf("pack to url file failed: %s", err)
			}

		case archiveFileTypeHtml:
			fileName += ".html"
			htmlContent := readableHtmlContent(item.Link, item.Title, item.Content)
			err = r.fileRoot.Write(fileName, []byte(htmlContent), 0655)
			if err != nil {
				return nil, fmt.Errorf("pack to html file failed: %s", err)
			}

		case archiveFileTypeRawHtml:
			filePath, err := web.PackFromURL(logger.IntoContext(ctx, r.logger), fileName, item.Link, "html", r.fileRoot.Workdir(), source.ClutterFree, source.toOption())
			if err != nil {
				r.logger.Warnw("pack to raw html file failed", "link", item.Link, "err", err)
				continue
			}
			fileName = path.Base(filePath)

		case archiveFileTypeWebArchive:
			filePath, err := web.PackFromURL(logger.IntoContext(ctx, r.logger), fileName, item.Link, "webarchive", r.fileRoot.Workdir(), source.ClutterFree, source.toOption())
			if err != nil {
				r.logger.Warnw("pack to webarchive failed", "link", item.Link, "err", err)
				continue
			}
			fileName = path.Base(filePath)

		default:
			return nil, fmt.Errorf("unknown rss archive file type %s", source.FileType)
		}

		fInfo, err := r.fileRoot.Stat(fileName)
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
		articles = append(articles, Article{
			FilePath:  path.Base(fileName),
			Size:      fInfo.Size(),
			Title:     item.Title,
			URL:       item.Link,
			SiteURL:   feed.Link,
			SiteName:  feed.Title,
			UpdatedAt: updatedAt.Format(time.RFC3339),
		})
	}

	if err = source.record(ctx, links...); err != nil {
		r.logger.Warnw("record links failed", "err", err)
	}

	r.logger.Infow("sync rss finish", "entries", len(articles))

	return articles, nil
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

type rssSource struct {
	FeedUrl     string
	FileType    string
	ClutterFree bool
	Timeout     int
	Headers     map[string]string

	Store api.PersistentStore
}

func (s *rssSource) isNew(ctx context.Context, linkStr string) (bool, error) {
	var v = make(map[string]any)
	err := s.Store.Load(ctx, RssSourcePluginName, "articles", linkStr, &v)
	if err == nil {
		return false, nil
	}

	if strings.Contains(err.Error(), "no record") {
		return true, nil
	}

	if strings.Contains(err.Error(), "not found") {
		return true, nil
	}

	return false, err
}

func (s *rssSource) record(ctx context.Context, linkList ...string) error {
	for _, linkStr := range linkList {
		v := map[string]string{"link": linkStr, "time": time.Now().Format(time.RFC3339)}
		err := s.Store.Save(ctx, RssSourcePluginName, "articles", linkStr, &v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *rssSource) toOption() web.Option {
	return func(option *packer.Option) {
		option.Timeout = s.Timeout
		option.ClutterFree = s.ClutterFree
		option.Headers = s.Headers
	}
}
