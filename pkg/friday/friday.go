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

package friday

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Client struct {
	log     *zap.SugaredLogger
	baseurl string
}

var _ Friday = &Client{}

func NewFridayClient(conf config.FridayConfig) Friday {
	return &Client{
		baseurl: conf.HttpAddr,
		log:     logger.NewLogger("friday"),
	}
}

func (c *Client) request(ctx context.Context, method, uri string, data []byte) ([]byte, error) {
	c.log.Debugf("request friday %s %s", method, uri)
	body := bytes.NewBuffer(data)
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return nil, types.ErrNotFound
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("request failed, status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return respBody, nil
}

func (c *Client) CreateDocument(ctx context.Context, doc *types.Document) error {
	if doc.Namespace == "" {
		ns := types.GetNamespace(ctx)
		doc.Namespace = ns.String()
	}
	docReq := &DocRequest{}
	docReq.FromType(doc)

	data, _ := json.Marshal(docReq)
	uri := fmt.Sprintf("%s/api/namespace/%s/docs/entry/%d", c.baseurl, doc.Namespace, doc.EntryId)
	resp, err := c.request(ctx, "POST", uri, data)
	if err != nil {
		return err
	}

	docResp := &Document{}
	if err := json.Unmarshal(resp, docResp); err != nil {
		return err
	}
	doc.HeaderImage = docResp.HeaderImage
	doc.SubContent = docResp.SubContent
	doc.Summary = docResp.Summary

	return nil
}

func (c *Client) UpdateDocument(ctx context.Context, doc *types.Document) error {
	if doc.Namespace == "" {
		ns := types.GetNamespace(ctx)
		doc.Namespace = ns.String()
	}
	docReq := &DocAttrRequest{}
	docReq.FromType(doc)

	data, _ := json.Marshal(docReq)
	uri := fmt.Sprintf("%s/api/namespace/%s/docs/entry/%d", c.baseurl, doc.Namespace, doc.EntryId)
	_, err := c.request(ctx, "PUT", uri, data)
	return err
}

func (c *Client) GetDocument(ctx context.Context, entryId int64) (*types.Document, error) {
	ns := types.GetNamespace(ctx)

	uri := fmt.Sprintf("%s/api/namespace/%s/docs/entry/%d", c.baseurl, ns.String(), entryId)
	resp, err := c.request(ctx, "GET", uri, nil)
	if err != nil {
		return nil, err
	}

	docResp := &DocumentWithAttr{}
	if err := json.Unmarshal(resp, docResp); err != nil {
		return nil, err
	}
	doc := docResp.ToType()
	return doc, nil
}

func (c *Client) DeleteDocument(ctx context.Context, entryId int64) error {
	ns := types.GetNamespace(ctx)
	uri := fmt.Sprintf("%s/api/namespace/%s/docs/entry/%d", c.baseurl, ns.String(), entryId)
	_, err := c.request(ctx, "DELETE", uri, nil)
	return err
}

func (c *Client) FilterDocuments(ctx context.Context, query *types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error) {
	ns := types.GetNamespace(ctx)
	uri := fmt.Sprintf("%s/api/namespace/%s/docs/filter", c.baseurl, ns.String())

	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if query.FuzzyName != "" {
		q.Set("fuzzyName", query.FuzzyName)
	}
	if query.Search != "" {
		q.Set("search", query.Search)
	}
	if query.ParentID != 0 {
		q.Set("parentID", fmt.Sprintf("%d", query.ParentID))
	}
	if query.Marked != nil {
		q.Set("mark", fmt.Sprintf("%t", *query.Marked))
	}
	if query.Unread != nil {
		q.Set("unRead", fmt.Sprintf("%t", *query.Unread))
	}
	if query.CreatedAtStart != nil {
		q.Set("createdAtStart", fmt.Sprintf("%d", query.CreatedAtStart.Unix()))
	}
	if query.CreatedAtEnd != nil {
		q.Set("createdAtEnd", fmt.Sprintf("%d", query.CreatedAtEnd.Unix()))
	}
	if query.ChangedAtStart != nil {
		q.Set("changedAtStart", fmt.Sprintf("%d", query.ChangedAtStart.Unix()))
	}
	if query.ChangedAtEnd != nil {
		q.Set("changedAtEnd", fmt.Sprintf("%d", query.ChangedAtEnd.Unix()))
	}

	page := types.GetPagination(ctx)
	if page.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", page.Page))
	}
	if page.PageSize > 0 {
		q.Set("pageSize", fmt.Sprintf("%d", page.PageSize))
	}

	if order != nil {
		q.Set("sort", order.Order.String())
		q.Set("desc", fmt.Sprintf("%t", order.Desc))
	}

	u.RawQuery = q.Encode()

	resp, err := c.request(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	var docs []*types.Document
	var docResp []DocumentWithAttr
	if err := json.Unmarshal(resp, &docResp); err != nil {
		return nil, err
	}
	for _, d := range docResp {
		docs = append(docs, d.ToType())
	}
	return docs, nil
}
