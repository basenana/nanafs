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
	"github.com/hyponet/webpage-packer/packer"
	"github.com/mmcdole/gofeed"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path"
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
	SourceID    string            `json:"source_id"`
	EntryId     int64             `json:"entry_id"`
	FeedUrl     string            `json:"feed_url"`
	FileType    string            `json:"file_type"`
	ClutterFree bool              `json:"clutter_free"`
	Timeout     int               `json:"timeout"`
	Parameters  map[string]string `json:"parameters"`
}

type RssSourcePlugin struct {
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

func (r *RssSourcePlugin) Run(ctx context.Context, request *pluginapi.Request, pluginParams map[string]string) (*pluginapi.Response, error) {
	rssSourceList, err := r.listRssSources(ctx)
	if err != nil {
		r.logger.Errorw("list rss source failed", "err", err)
		return nil, err
	}

	results := make([]pluginapi.CollectManifest, 0)
	for i := range rssSourceList {
		source := rssSourceList[i]
		entries, err := r.syncRssSource(ctx, source, request.WorkPath, pluginParams)
		if err != nil {
			r.logger.Warnw("sync rss failed", "source", source.FeedUrl, "err", err)
			continue
		}
		results = append(results, pluginapi.CollectManifest{BaseEntry: source.EntryId, NewFiles: entries})
	}

	return pluginapi.NewResponseWithResult(map[string]any{pluginapi.ResCollectManifest: results}), nil
}

func (r *RssSourcePlugin) listRssSources(ctx context.Context) ([]rssSource, error) {
	sourceConfigs, err := r.recorder.ListRecords(ctx, "source")
	if err != nil {
		return nil, fmt.Errorf("list source configs failed: %s", err)
	}

	result := make([]rssSource, 0, len(sourceConfigs))
	for _, srcID := range sourceConfigs {
		rs := rssSource{}
		err = r.recorder.GetRecord(ctx, srcID, &rs)
		if err != nil {
			r.logger.Errorw("get source config failed", "recordID", srcID, "err", err)
			continue
		}

		rs.SourceID = srcID
		if rs.FileType == "" {
			rs.FileType = archiveFileTypeUrl
		}
		if rs.Timeout == 0 {
			rs.Timeout = 120
		}

		if rs.FeedUrl == "" {
			r.logger.Warnw("invalid source config", "recordID", srcID, "err", "feed_url is empty")
			continue
		}

		result = append(result, rs)
	}
	return result, nil
}

func (r *RssSourcePlugin) syncRssSource(ctx context.Context, source rssSource, workdir string, params map[string]string) ([]pluginapi.Entry, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(source.FeedUrl)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for k, v := range params {
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
			Name:    path.Base(filePath),
			Kind:    types.RawKind,
			Size:    fInfo.Size(),
			IsGroup: false,
		})
	}
	return nil, nil
}

func InitRssSourcePlugin(recorder metastore.PluginRecorder) *RssSourcePlugin {
	return nil
}
