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

package dentry

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

type patchHandler func(old *types.Object)

var (
	cacheStore *lfuCache
)

type lfuCache struct {
	metastore metastore.ObjectStore
	lfu       *utils.LFUPool
}

func (c *lfuCache) getEntry(ctx context.Context, entryID int64) (Entry, error) {
	enRaw := c.lfu.Get(c.entryKey(entryID))
	if enRaw != nil {
		return enRaw.(Entry), nil
	}

	obj, err := c.metastore.GetObject(ctx, entryID)
	if err != nil {
		return nil, err
	}
	en := buildEntry(obj, c.metastore)
	if err != nil {
		return nil, err
	}
	c.putEntry2Cache(en)
	return en, nil
}

func (c *lfuCache) createEntry(ctx context.Context, newObj *types.Object, parentPatch entryPatch) error {
	parentEn, err := c.getEntry(ctx, parentPatch.entryID)
	if err != nil {
		return err
	}
	parentObj := &types.Object{Metadata: *parentEn.Metadata()}
	parentPatch.handler(parentObj)

	err = c.metastore.SaveObjects(ctx, newObj, parentObj)
	if err != nil {
		c.delEntryCache(newObj.ParentID)
		return err
	}
	c.delEntryCache(newObj.ParentID)
	return nil
}

func (c *lfuCache) updateEntry(ctx context.Context, patches ...entryPatch) error {
	err := c.updateEntryNoRetry(ctx, patches...)
	if err == types.ErrConflict {
		return c.updateEntryNoRetry(ctx, patches...)
	}
	return err
}

func (c *lfuCache) updateEntryNoRetry(ctx context.Context, patches ...entryPatch) error {
	var (
		objList = make([]*types.Object, len(patches))
		err     error
		en      Entry
	)

	for i, patch := range patches {
		var obj *types.Object
		en, err = c.getEntry(ctx, patch.entryID)
		if err != nil {
			return err
		}
		obj = &types.Object{Metadata: *en.Metadata()}
		patch.handler(obj)
		objList[i] = obj
	}

	err = c.metastore.SaveObjects(ctx, objList...)
	if err != nil {
		return err
	}

	for _, patch := range patches {
		enRaw := c.lfu.Get(c.entryKey(patch.entryID))
		if enRaw == nil {
			continue
		}
		patch.handler(&types.Object{Metadata: *enRaw.(Entry).Metadata()})
	}
	return nil
}

func (c *lfuCache) putEntry2Cache(entry Entry) {
	c.lfu.Put(c.entryKey(entry.Metadata().ID), entry)
}

func (c *lfuCache) delEntryCache(eid int64) {
	c.lfu.Remove(c.entryKey(eid))
}

func (c *lfuCache) entryKey(eid int64) string {
	return fmt.Sprintf("entry_%d", eid)
}

func newCacheStore(metastore metastore.ObjectStore) *lfuCache {
	cacheStore = &lfuCache{metastore: metastore, lfu: utils.NewLFUPool(8192)}
	return cacheStore
}

type entryPatch struct {
	entryID int64
	handler patchHandler
}
