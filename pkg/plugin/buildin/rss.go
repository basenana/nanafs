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
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hyponet/webpage-packer/packer"
	"github.com/mmcdole/gofeed"
	"go.uber.org/zap"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	RssSourcePluginName    = "rss"
	RssSourcePluginVersion = "1.0"

	archiveFileTypeUrl        = "url"
	archiveFileTypeHtml       = "html"
	archiveFileTypeWebArchive = "webarchive"
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
	spec     types.PluginSpec
	scope    types.PlugScope
	recorder metastore.PluginRecorder
	logger   *zap.SugaredLogger
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
	source, err := r.rssSources(r.scope.Parameters)
	if err != nil {
		r.logger.Errorw("get rss source failed", "err", err)
		return nil, err
	}
	source.EntryId = request.EntryId

	entries, err := r.syncRssSource(ctx, source, request.WorkPath)
	if err != nil {
		r.logger.Warnw("sync rss failed", "source", source.FeedUrl, "err", err)
		return pluginapi.NewFailedResponse(fmt.Sprintf("sync rss failed: %s", err)), nil
	}
	results := []pluginapi.CollectManifest{{BaseEntry: source.EntryId, NewFiles: entries}}

	return pluginapi.NewResponseWithResult(map[string]any{pluginapi.ResCollectManifest: results}), nil
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
		src.FileType = archiveFileTypeUrl
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

func (r *RssSourcePlugin) syncRssSource(ctx context.Context, source rssSource, workdir string) ([]pluginapi.Entry, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(source.FeedUrl)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for k, v := range source.Parameters {
		if strings.HasPrefix(k, "header_") || strings.HasPrefix(k, "HEADER_") {
			headerKey := strings.TrimPrefix(k, "header_")
			headerKey = strings.TrimPrefix(headerKey, "HEADER_")
			headers[k] = v
		}
	}

	newEntries := make([]pluginapi.Entry, 0)
	for _, item := range feed.Items {
		filePath := path.Join(workdir, item.Title)
		switch source.FileType {
		case archiveFileTypeUrl:
			filePath += ".url"
			buf := bytes.Buffer{}
			buf.WriteString("[InternetShortcut]")
			buf.WriteString("\n")
			buf.WriteString(fmt.Sprintf("URL=%s", item.Link))

			err = ioutil.WriteFile(filePath, buf.Bytes(), 0655)
			if err != nil {
				return nil, fmt.Errorf("pack to url file failed: %s", err)
			}

		case archiveFileTypeHtml:
			filePath += ".html"
			p := packer.NewHtmlPacker()
			err = p.Pack(ctx, packer.Option{
				URL:         item.Link,
				FilePath:    filePath,
				Timeout:     source.Timeout,
				ClutterFree: source.ClutterFree,
				Headers:     headers,
			})
			if err != nil {
				return nil, fmt.Errorf("pack to html file failed: %s", err)
			}

		case archiveFileTypeWebArchive:
			filePath += ".webarchive"
			p := packer.NewWebArchivePacker()
			err = p.Pack(ctx, packer.Option{
				URL:         item.Link,
				FilePath:    filePath,
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

		fInfo, err := os.Stat(filePath)
		if err != nil {
			return nil, fmt.Errorf("stat archive file error: %s", err)
		}

		newEntries = append(newEntries, pluginapi.Entry{
			Name: path.Base(filePath),
			Kind: types.RawKind,
			Size: fInfo.Size(),
			Parameters: map[string]string{
				pluginapi.ResEntryDocumentsKey: item.Content,
			},
			IsGroup: false,
		})
	}
	return nil, nil
}

func BuildRssSourcePlugin(ctx context.Context, recorder metastore.PluginRecorder, spec types.PluginSpec, scope types.PlugScope) *RssSourcePlugin {
	return &RssSourcePlugin{spec: spec, scope: scope, recorder: recorder, logger: logger.NewLogger("rssPlugin")}
}
