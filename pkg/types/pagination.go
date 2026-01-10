/*
  Copyright 2024 NanaFS Authors.

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

package types

import (
	"context"
	"strings"
)

const (
	PageKey     = "page"
	PageSizeKey = "pageSize"
	SortKey     = "sort"
	OrderKey    = "order"
)

type Pagination struct {
	Page     int64
	PageSize int64
	Sort     string
	Order    string
}

func (p *Pagination) Limit() int {
	return int(p.PageSize)
}

func (p *Pagination) Offset() int {
	return int((p.Page - 1) * p.PageSize)
}

func (p *Pagination) SortField() string {
	sort := strings.ToLower(p.Sort)
	switch sort {
	case "created_at":
		return "created_at"
	case "changed_at":
		return "changed_at"
	case "name":
		fallthrough
	default:
		return "name"
	}
}

func (p *Pagination) SortOrder() string {
	order := strings.ToLower(p.Order)
	if order == "desc" {
		return "desc"
	}
	return "asc"
}

func NewPagination(page, pageSize int64) *Pagination {
	return NewPaginationWithSort(page, pageSize, "", "")
}

func NewPaginationWithSort(page, pageSize int64, sort, order string) *Pagination {
	if page < 0 {
		return nil
	}

	p := &Pagination{
		Page:     page,
		PageSize: pageSize,
		Sort:     sort,
		Order:    order,
	}

	if p.Page == 0 {
		p.Page = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 50
	}
	if p.Sort == "" {
		p.Sort = "name"
	}
	if p.Order == "" {
		p.Order = "asc"
	}
	return p
}

func GetPagination(ctx context.Context) *Pagination {
	if ctx.Value(PageKey) != nil && ctx.Value(PageSizeKey) != nil {
		var sort, order string
		if ctx.Value(SortKey) != nil {
			sort = ctx.Value(SortKey).(string)
		}
		if ctx.Value(OrderKey) != nil {
			order = ctx.Value(OrderKey).(string)
		}
		return &Pagination{
			Page:     ctx.Value(PageKey).(int64),
			PageSize: ctx.Value(PageSizeKey).(int64),
			Sort:     sort,
			Order:    order,
		}
	}
	return nil
}

func WithPagination(ctx context.Context, page *Pagination) context.Context {
	if page != nil {
		ctx = context.WithValue(ctx, PageKey, page.Page)
		ctx = context.WithValue(ctx, PageSizeKey, page.PageSize)
		ctx = context.WithValue(ctx, SortKey, page.Sort)
		ctx = context.WithValue(ctx, OrderKey, page.Order)
	}
	return ctx
}
