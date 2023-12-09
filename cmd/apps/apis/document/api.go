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

package document

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/pkg/controller"
)

type Server struct {
	ctrl   controller.Controller
	logger *zap.SugaredLogger
}

func NewDocumentAPIServer(ctrl controller.Controller) *Server {
	return &Server{ctrl: ctrl, logger: logger.NewLogger("documentAPI")}
}

func (f *Server) Query(gCtx *gin.Context) {
	queryStr := gCtx.Query("q")
	docList, err := f.ctrl.QueryDocuments(gCtx, queryStr)
	if err != nil {
		gCtx.String(400, "Invalid parameter")
		return
	}

	for _, doc := range docList {
		f.logger.Infof("hit: %s", doc.Name)
	}

	gCtx.String(200, "OK")
}

func (f *Server) Summary(gCtx *gin.Context) {
	force := gCtx.Query("force")
	entryIdStr := gCtx.Query("entryId")
	entryId, err := strconv.ParseInt(entryIdStr, 10, 64)
	if err != nil {
		gCtx.String(400, "Invalid parameter: %s", err)
		return
	}
	doc, err := f.ctrl.GetDocumentsByEntryId(gCtx, entryId)
	if err != nil {
		gCtx.String(400, "Invalid parameter: %s", err)
		return
	}
	entry, err := f.ctrl.GetEntry(gCtx, entryId)
	if err != nil {
		gCtx.String(400, "Invalid parameter: %s", err)
		return
	}
	sum := doc.Summary
	usage := make(map[string]int)
	if sum == "" || force == "true" {
		f.logger.Infow("summary of doc is none, call friday to summary", "entryId", doc.OID, "docId", doc.ID)
		s := strings.Split(entry.Name, ".")
		if len(s) == 0 {
			gCtx.String(400, "Invalid parameter: can not get doc type")
			return
		}
		docType := s[len(s)-1]
		content := utils.ContentTrim(docType, doc.Content)
		sum, usage, err = friday.SummaryFile(gCtx, doc.Name, content)
		if err != nil {
			gCtx.String(500, "Internal Error: %s", err)
			return
		}
	}
	res := map[string]any{
		"summary": sum,
	}
	for k, v := range usage {
		res[k] = v
	}
	gCtx.JSON(200, res)
}

func (f *Server) Atom(gCtx *gin.Context) {
	feedId := gCtx.Param("feedId")
	countStr := gCtx.Query("count")
	count := 20
	var err error
	if countStr != "" {
		count, err = strconv.Atoi(countStr)
		if err != nil {
			gCtx.String(400, "Invalid parameter")
			return
		}
	}
	docFeed, err := f.ctrl.GetDocumentsByFeed(gCtx, feedId, count)
	if err != nil {
		if err == types.ErrNotFound {
			gCtx.String(404, "Not found")
			return
		}
		gCtx.String(500, "Internal error")
		return
	}

	items := make([]*Item, len(docFeed.Documents))
	for i, doc := range docFeed.Documents {
		content := doc.Document.Content
		if doc.Document.Summary != "" || len(doc.Document.KeyWords) != 0 {
			contents := make([]string, 0)
			if doc.Document.Summary != "" {
				summaries := strings.Split(doc.Document.Summary, "\n")
				sumContents := make([]string, 0)
				for j, sum := range summaries {
					s := strings.TrimSpace(sum)
					if s == "" {
						continue
					}
					if j == 0 {
						sumContents = append(sumContents, fmt.Sprintf("<p><b>Summary</b>: %s</p>", s))
						continue
					}
					sumContents = append(sumContents, fmt.Sprintf("<p>%s</p>", s))
				}
				contents = append(contents, strings.Join(sumContents, ""))
			}
			if len(doc.Document.KeyWords) != 0 {
				keyWords := strings.Join(doc.Document.KeyWords, ", ")
				contents = append(contents, fmt.Sprintf("<p><b>Key Words</b>: %s</p>", keyWords))
			}
			contents = append(contents, doc.Document.Content)
			content = strings.Join(contents, "\n")
		}

		items[i] = &Item{
			Title:       doc.Title,
			Link:        &Link{Href: doc.Link},
			Description: doc.Document.Summary,
			Id:          doc.ID,
			Updated:     doc.UpdatedAt,
			Content:     content,
		}
	}

	sort.Slice(items, func(i, j int) bool {
		it1, it2 := items[i], items[j]
		var itTime1, itTime2 time.Time
		if it1.Updated != "" {
			itTime1, _ = time.Parse(time.RFC3339, it1.Updated)
		}
		if it2.Updated != "" {
			itTime2, _ = time.Parse(time.RFC3339, it2.Updated)
		}
		return itTime2.Before(itTime1)
	})

	feed := Feed{
		Title: docFeed.SiteName,
		Link:  &Link{Href: docFeed.SiteUrl},
		Id:    docFeed.SiteUrl,
		Items: items,
		Generator: &Generator{
			Value: "NanaFS",
			URI:   "https://github.com/basenana/nanafs",
		},
		Updated: time.Now(),
	}

	x, err := ToXML(NewAtomGenerator(), feed)
	if err != nil {
		gCtx.String(500, "Internal error")
		return
	}

	gCtx.Data(200, "application/xml; charset=utf-8", []byte(x))
}
