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
	rssParameterParentURI   = "parent_uri"

	rssPostMaxCollect = 50
)

var RssSourcePluginSpec = types.PluginSpec{
	Name:    RssSourcePluginName,
	Version: RssSourcePluginVersion,
	Type:    types.TypeSource,
	InitParameters: []types.ParameterSpec{
		{
			Name:        rssParameterFileType,
			Required:    false,
			Default:     "webarchive",
			Description: "Archive format: url, html, rawhtml, webarchive",
			Options:     []string{"url", "html", "rawhtml", "webarchive"},
		},
		{
			Name:        rssParameterTimeout,
			Required:    false,
			Default:     "120",
			Description: "Download timeout (seconds)",
		},
		{
			Name:        rssParameterClutterFree,
			Required:    false,
			Default:     "true",
			Description: "Enable clutter-free mode",
			Options:     []string{"true", "false"},
		},
	},
	Parameters: []types.ParameterSpec{
		{
			Name:        rssParameterFeed,
			Required:    true,
			Description: "RSS/Atom feed URL",
		},
		{
			Name:        rssParameterParentURI,
			Required:    true,
			Description: "The Parent URI where RSS from",
		},
	},
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
	r.logger.Infow("syncing rss", "feed", source.FeedUrl, "fileType", source.FileType)

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
	src.ParentURI = api.GetStringParameter(rssParameterParentURI, request, "")
	if src.ParentURI == "" {
		err = fmt.Errorf("parent_uri is empty")
		return
	}
	src.FS = request.FS
	if src.FS == nil {
		err = fmt.Errorf("fs is empty")
		return
	}
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

	var (
		articles  = make([]Article, 0)
		collected int
	)
	for _, item := range feed.Items {
		if collected > rssPostMaxCollect {
			r.logger.Infow("soo many post need to collect, skip", "collectLimit", rssPostMaxCollect)
			break
		}

		item.Link = absoluteURL(siteURL, item.Link)
		if item.Content == "" && item.Description != "" {
			item.Content = item.Description
		}

		article, err := r.processItem(ctx, &source, item, nowTime, feed)
		if err != nil {
			r.logger.Errorw("process item failed", "link", item.Link, "err", err)
			continue
		}
		if article == nil {
			continue // collected
		}

		collected++
		articles = append(articles, *article)
	}

	r.logger.Infow("sync rss finish", "entries", len(articles))
	return articles, nil
}

func (r *RssSourcePlugin) processItem(ctx context.Context, source *rssSource, item *gofeed.Item, nowTime time.Time, feed *gofeed.Feed) (*Article, error) {
	switch source.FileType {
	case archiveFileTypeUrl:
		return r.processUrlType(ctx, source, item, nowTime, feed)
	case archiveFileTypeHtml:
		return r.processHtmlType(ctx, source, item, nowTime, feed)
	case archiveFileTypeRawHtml:
		return r.processRawHtmlType(ctx, source, item, nowTime, feed)
	case archiveFileTypeWebArchive:
		return r.processWebArchiveType(ctx, source, item, nowTime, feed)
	default:
		return nil, fmt.Errorf("unknown file type: %s", source.FileType)
	}
}

func (r *RssSourcePlugin) processUrlType(ctx context.Context, source *rssSource, item *gofeed.Item, nowTime time.Time, feed *gofeed.Feed) (*Article, error) {
	fileName := utils.SanitizeFilename(item.Title) + ".url"
	entryURI := path.Join(source.ParentURI, fileName)
	if isNew, err := source.isNew(ctx, entryURI); err != nil || !isNew {
		if err != nil {
			return nil, err
		}
		r.logger.Infow("entry already exists, skip", "entryURI", entryURI)
		return nil, nil
	}

	buf := bytes.Buffer{}
	buf.WriteString("[InternetShortcut]")
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("URL=%s", item.Link))

	if err := r.fileRoot.Write(fileName, buf.Bytes(), 0655); err != nil {
		return nil, fmt.Errorf("pack to url file failed: %s", err)
	}

	return r.buildArticle(item, fileName, nowTime, feed)
}

func (r *RssSourcePlugin) processHtmlType(ctx context.Context, source *rssSource, item *gofeed.Item, nowTime time.Time, feed *gofeed.Feed) (*Article, error) {
	fileName := utils.SanitizeFilename(item.Title) + ".html"
	entryURI := path.Join(source.ParentURI, fileName)
	if isNew, err := source.isNew(ctx, entryURI); err != nil || !isNew {
		if err != nil {
			return nil, err
		}
		r.logger.Infow("entry already exists, skip", "entryURI", entryURI)
		return nil, nil
	}

	htmlContent := readableHtmlContent(item.Link, item.Title, item.Content)
	if err := r.fileRoot.Write(fileName, []byte(htmlContent), 0655); err != nil {
		return nil, fmt.Errorf("pack to html file failed: %s", err)
	}

	return r.buildArticle(item, fileName, nowTime, feed)
}

func (r *RssSourcePlugin) processRawHtmlType(ctx context.Context, source *rssSource, item *gofeed.Item, nowTime time.Time, feed *gofeed.Feed) (*Article, error) {
	fileName := utils.SanitizeFilename(item.Title)
	entryURI := path.Join(source.ParentURI, fileName+".html")
	if isNew, err := source.isNew(ctx, entryURI); err != nil || !isNew {
		if err != nil {
			return nil, err
		}
		r.logger.Infow("entry already exists, skip", "entryURI", entryURI)
		return nil, nil
	}

	filePath, err := web.PackFromURL(logger.IntoContext(ctx, r.logger), fileName, item.Link, "html", r.fileRoot.Workdir(), source.ClutterFree, source.toOption())
	if err != nil {
		r.logger.Warnw("pack to raw html file failed", "link", item.Link, "err", err)
		return nil, err
	}

	return r.buildArticle(item, path.Base(filePath), nowTime, feed)
}

func (r *RssSourcePlugin) processWebArchiveType(ctx context.Context, source *rssSource, item *gofeed.Item, nowTime time.Time, feed *gofeed.Feed) (*Article, error) {
	fileName := utils.SanitizeFilename(item.Title)
	entryURI := path.Join(source.ParentURI, fileName+".webarchive")
	if isNew, err := source.isNew(ctx, entryURI); err != nil || !isNew {
		if err != nil {
			return nil, err
		}
		r.logger.Infow("entry already exists, skip", "entryURI", entryURI)
		return nil, nil
	}

	filePath, err := web.PackFromURL(logger.IntoContext(ctx, r.logger), fileName, item.Link, "webarchive", r.fileRoot.Workdir(), source.ClutterFree, source.toOption())
	if err != nil {
		r.logger.Warnw("pack to webarchive failed", "link", item.Link, "err", err)
		return nil, err
	}

	return r.buildArticle(item, path.Base(filePath), nowTime, feed)
}

func (r *RssSourcePlugin) buildArticle(item *gofeed.Item, fileName string, nowTime time.Time, feed *gofeed.Feed) (*Article, error) {
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

	return &Article{
		FilePath:  fileName,
		Size:      fInfo.Size(),
		Title:     item.Title,
		URL:       item.Link,
		SiteURL:   feed.Link,
		SiteName:  feed.Title,
		UpdatedAt: updatedAt.Format(time.RFC3339),
	}, nil
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
	ParentURI   string
	FS          api.NanaFS
}

func (s *rssSource) isNew(ctx context.Context, entryURI string) (bool, error) {
	props, err := s.FS.GetEntryProperties(ctx, entryURI)
	if err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "no record") ||
			strings.Contains(err.Error(), "no entry") {
			return true, nil
		}
		return false, err
	}
	if props != nil && props.Title != "" {
		return false, nil
	}
	return true, nil
}

func (s *rssSource) toOption() web.Option {
	return func(option *packer.Option) {
		option.Timeout = s.Timeout
		option.ClutterFree = s.ClutterFree
		option.Headers = s.Headers
	}
}
