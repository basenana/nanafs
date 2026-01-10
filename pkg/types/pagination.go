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
)

const (
	PageKey     = "page"
	PageSizeKey = "pageSize"
)

type Pagination struct {
	Page     int64
	PageSize int64
}

func (p *Pagination) Limit() int {
	return int(p.PageSize)
}

func (p *Pagination) Offset() int {
	return int((p.Page - 1) * p.PageSize)
}

func NewPagination(page, pageSize int64) *Pagination {
	if page < 0 {
		return nil
	}

	p := &Pagination{
		Page:     page,
		PageSize: pageSize,
	}

	if p.Page == 0 {
		p.Page = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 50
	}
	return p
}

func GetPagination(ctx context.Context) *Pagination {
	if ctx.Value(PageKey) != nil && ctx.Value(PageSizeKey) != nil {
		return &Pagination{
			Page:     ctx.Value(PageKey).(int64),
			PageSize: ctx.Value(PageSizeKey).(int64),
		}
	}
	return nil
}

func WithPagination(ctx context.Context, page *Pagination) context.Context {
	if page != nil {
		ctx = context.WithValue(ctx, PageKey, page.Page)
		ctx = context.WithValue(ctx, PageSizeKey, page.PageSize)
	}
	return ctx
}
