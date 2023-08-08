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
	cacheStore *metaCache
)

type metaCache struct {
	metastore metastore.ObjectStore
	lfu       *utils.LFUPool
}

func (c *metaCache) getEntry(ctx context.Context, entryID int64) (*types.Metadata, error) {
	enRaw := c.lfu.Get(c.entryKey(entryID))
	if enRaw != nil {
		return enRaw.(*types.Metadata), nil
	}

	obj, err := c.metastore.GetObject(ctx, entryID)
	if err != nil {
		return nil, err
	}
	md := obj.Metadata
	c.putEntry2Cache(&md)
	return &md, nil
}

func (c *metaCache) createEntry(ctx context.Context, newObj *types.Object, parentPatch entryPatch) error {
	parentEn, err := c.getEntry(ctx, parentPatch.entryID)
	if err != nil {
		return err
	}
	parentObj := &types.Object{Metadata: *parentEn}
	parentPatch.handler(parentObj)

	err = c.metastore.SaveObjects(ctx, newObj, parentObj)
	if err != nil {
		c.delEntryCache(newObj.ParentID)
		return err
	}
	c.delEntryCache(newObj.ParentID)
	return nil
}

func (c *metaCache) updateEntry(ctx context.Context, patches ...entryPatch) error {
	err := c.updateEntryNoRetry(ctx, patches...)
	if err == types.ErrConflict {
		return c.updateEntryNoRetry(ctx, patches...)
	}
	return err
}

func (c *metaCache) updateEntryNoRetry(ctx context.Context, patches ...entryPatch) error {
	var (
		objList = make([]*types.Object, len(patches))
		err     error
		en      *types.Metadata
	)

	for i, patch := range patches {
		var obj *types.Object
		en, err = c.getEntry(ctx, patch.entryID)
		if err != nil {
			return err
		}
		obj = &types.Object{Metadata: *en}
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

func (c *metaCache) putEntry2Cache(entry *types.Metadata) {
	c.lfu.Put(c.entryKey(entry.ID), entry)
}

func (c *metaCache) delEntryCache(eid int64) {
	c.lfu.Remove(c.entryKey(eid))
}

func (c *metaCache) entryKey(eid int64) string {
	return fmt.Sprintf("entry_%d", eid)
}

func newCacheStore(metastore metastore.ObjectStore) *metaCache {
	cacheStore = &metaCache{metastore: metastore, lfu: utils.NewLFUPool(8192)}
	return cacheStore
}

type entryPatch struct {
	entryID int64
	handler patchHandler
}
