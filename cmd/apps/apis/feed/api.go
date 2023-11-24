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

package feed

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/pkg/controller"
)

type FeedServer struct {
	ctrl controller.Controller
}

func NewFeedServer(ctrl controller.Controller) *FeedServer {
	return &FeedServer{ctrl: ctrl}
}

func (f *FeedServer) Atom(gCtx *gin.Context) {
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
				sumContents := []string{fmt.Sprintf("<p><b>Summary</b>: %s</p>", summaries[0])}
				for _, sum := range summaries[1:] {
					sumContents = append(sumContents, fmt.Sprintf("<p>%s</p>", sum))
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

	feed := Feed{
		Title: docFeed.SiteName,
		Link:  &Link{Href: docFeed.SiteUrl},
		Id:    docFeed.SiteUrl,
		Items: items,
	}

	x, err := ToXML(NewAtomGenerator(), feed)
	if err != nil {
		gCtx.String(500, "Internal error")
		return
	}

	gCtx.String(200, x)
}
