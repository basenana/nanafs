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
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore/db"
	"github.com/basenana/nanafs/pkg/types"
	"gorm.io/gorm"
	"sync"
	"time"
)

type EntryIterator interface {
	HasNext() bool
	Next() (*types.Metadata, error)
}

const entryFetchPageSize = 100

type transactionEntryIterator struct {
	tx        *gorm.DB
	onePage   []*types.Metadata
	crtPage   int
	totalPage int
	mux       sync.Mutex
}

func newTransactionEntryIterator(tx *gorm.DB, total int64) EntryIterator {
	it := &transactionEntryIterator{tx: tx, onePage: make([]*types.Metadata, 0)}
	it.totalPage = int(total / entryFetchPageSize)
	if total%entryFetchPageSize > 0 {
		it.totalPage += 1
	}
	return it
}

func (i *transactionEntryIterator) HasNext() bool {
	i.mux.Lock()
	if len(i.onePage) > 0 || i.crtPage < i.totalPage {
		i.mux.Unlock()
		return true
	}
	i.mux.Unlock()
	return false
}

func (i *transactionEntryIterator) Next() (*types.Metadata, error) {
	i.mux.Lock()
	defer i.mux.Unlock()
	defer logOperationLatency("transactionEntryIterator.next", time.Now())
	// fetch next page
	if len(i.onePage) == 0 && i.crtPage < i.totalPage {
		defer logOperationLatency("transactionEntryIterator.query_one_page", time.Now())
		res := i.tx.Order("name DESC").Limit(entryFetchPageSize).Offset(entryFetchPageSize * i.crtPage).Find(&i.onePage)
		if res.Error != nil {
			logOperationError("transactionEntryIterator.query_one_page", res.Error)
			return nil, db.SqlError2Error(res.Error)
		}
		i.crtPage += 1
	}

	if len(i.onePage) > 0 {
		one := i.onePage[0]
		i.onePage = i.onePage[1:]
		return one, nil
	}
	return nil, fmt.Errorf("has no next entry")
}

type Iterator interface {
	HasNext() bool
	Next() *types.Object
}

type iterator struct {
	objects []*types.Object
	mux     sync.Mutex
}

func (i *iterator) HasNext() bool {
	i.mux.Lock()
	defer i.mux.Unlock()
	return len(i.objects) > 0
}

func (i *iterator) Next() *types.Object {
	i.mux.Lock()
	defer i.mux.Unlock()
	obj := i.objects[0]
	i.objects = i.objects[1:]
	return obj
}
