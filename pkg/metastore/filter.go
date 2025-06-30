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
	"strings"
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

	tx := s.WithNamespace(ctx, namespace)
	if strings.Contains(where, "child.") {
		tx = tx.Joins("JOIN children ON children.child_id = entry.id")
	}
	if strings.Contains(where, "group.") {
		tx = tx.Joins("JOIN entry_property AS group ON group.entry = entry.id").Where("group.type = ?", types.PropertyTypeGroupAttr)
	}
	if strings.Contains(where, "property.") {
		tx = tx.Joins("JOIN entry_property AS property ON property.entry = entry.id").Where("property.type = ?", types.PropertyTypeProperty)
	}
	if strings.Contains(where, "document.") {
		tx = tx.Joins("JOIN entry_property AS document ON document.entry = entry.id").Where("document.type = ?", types.PropertyTypeDocument)
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
