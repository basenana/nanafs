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
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/basenana/nanafs/pkg/metastore/db"
	"github.com/basenana/nanafs/pkg/types"
)

type EntryIterator interface {
	HasNext() bool
	Next() *types.Metadata
}

const entryFetchPageSize = 1000

type transactionEntryIterator struct {
	tx      *gorm.DB
	onePage []*types.Metadata
	remain  int64
	crtPage int32
}

func newTransactionEntryIterator(tx *gorm.DB, orderName string, total int64) EntryIterator {
	if orderName != "" {
		tx = tx.Order(orderName)
	}
	it := &transactionEntryIterator{tx: tx, onePage: make([]*types.Metadata, 0)}
	it.remain = total
	return it
}

func (i *transactionEntryIterator) HasNext() bool {
	if len(i.onePage) > 0 {
		return true
	}

	// fetch next page
	if atomic.LoadInt64(&i.remain) > 0 {
		defer logOperationLatency("transactionEntryIterator.query_one_page", time.Now())
		onePage := make([]db.Object, 0, entryFetchPageSize)
		res := i.tx.Limit(entryFetchPageSize).Offset(entryFetchPageSize * int(i.crtPage)).Find(&onePage)
		if res.Error != nil {
			logOperationError("transactionEntryIterator.query_one_page", res.Error)
			return false
		}

		if len(onePage) == 0 {
			return false
		}

		for _, en := range onePage {
			i.onePage = append(i.onePage, en.ToEntry())
		}

		atomic.AddInt32(&i.crtPage, 1)
		atomic.AddInt64(&i.remain, -1*int64(len(i.onePage)))
	}
	return len(i.onePage) > 0
}

func (i *transactionEntryIterator) Next() *types.Metadata {
	defer logOperationLatency("transactionEntryIterator.next", time.Now())
	if len(i.onePage) > 0 {
		one := i.onePage[0]
		i.onePage = i.onePage[1:]
		return one
	}
	logOperationError("transactionEntryIterator.next", types.ErrNotFound)
	return nil
}
