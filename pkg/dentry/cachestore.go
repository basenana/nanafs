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
	"time"
)

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

func (c *metaCache) createEntry(ctx context.Context, newObj *types.Object, parentPatch *types.Metadata) error {
	objects := make([]*types.Object, 1, 2)
	objects[0] = newObj
	if parentPatch != nil {
		objects[1] = &types.Object{Metadata: *parentPatch}
	}
	err := c.metastore.SaveObjects(ctx, objects...)
	if err != nil {
		return err
	}

	c.putEntry2Cache(&newObj.Metadata)
	if parentPatch != nil {
		c.delEntryCache(newObj.ParentID)
	}
	return nil
}

func (c *metaCache) patchEntryMeta(ctx context.Context, patches ...*types.Metadata) error {
	err := c.updateEntryNoRetry(ctx, patches...)
	if err == types.ErrConflict {
		return c.updateEntryNoRetry(ctx, patches...)
	}
	return err
}

func (c *metaCache) updateEntryNoRetry(ctx context.Context, patches ...*types.Metadata) error {
	var (
		objList = make([]*types.Object, len(patches))
		err     error
		en      *types.Metadata
	)

	for i, patch := range patches {
		if patch.ID == 0 {
			return types.ErrNotFound
		}
		en, err = c.getEntry(ctx, patch.ID)
		if err != nil {
			return err
		}
		patch.Version = en.Version
		obj := &types.Object{Metadata: *patch}
		obj.ChangedAt = time.Now()
		objList[i] = obj
	}

	defer func() {
		for _, patch := range patches {
			c.delEntryCache(patch.ID)
		}
	}()

	err = c.metastore.SaveObjects(ctx, objList...)
	if err != nil {
		return err
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
