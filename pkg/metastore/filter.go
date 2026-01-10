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

package metastore

import (
	"context"
	"github.com/basenana/nanafs/pkg/metastore/db"
	"github.com/basenana/nanafs/pkg/metastore/filters"
	"github.com/basenana/nanafs/pkg/types"
	"runtime/trace"
)

func (s *sqlMetaStore) FilterEntries(ctx context.Context, namespace string, filter types.Filter) (EntryIterator, error) {
	defer trace.StartRegion(ctx, "metastore.sql.FilterEntries").End()
	requireLock()
	defer releaseLock()

	fctx := filters.NewConvertContext()
	err := filters.Convert(fctx, s.DB, filter.CELPattern)
	if err != nil {
		return nil, err
	}

	var (
		where  = fctx.Buffer.String()
		args   = fctx.Args
		result []db.Entry
	)

	tx := filters.Join(s.WithContext(ctx).Where("entry.namespace = ?", namespace), where)

	if page := types.GetPagination(ctx); page != nil {
		if page.Sort != "" && page.Order != "" {
			tx = tx.Order(page.SortField() + " " + page.SortOrder())
		}
		tx = tx.Offset(page.Offset()).Limit(page.Limit())
	}

	res := tx.Where(where, args...).Find(&result)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	return &simpleIterator{page: func() []*types.Entry {
		entries := make([]*types.Entry, len(result))
		for i, entry := range result {
			entries[i] = entry.ToEntry()
		}
		return entries
	}()}, nil
}
