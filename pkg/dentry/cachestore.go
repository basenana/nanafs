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

type metaCache struct {
	metastore  metastore.ObjectStore
	lfu        *utils.LFUPool
	childCache *utils.LFUPool
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

func (c *metaCache) lookupChild(ctx context.Context, parentID int64, name string) (*types.Metadata, error) {
	childID := c.lfu.Get(c.entryChildKey(parentID, name))
	if childID != nil {
		return c.getEntry(ctx, childID.(int64))
	}

	objs, err := c.metastore.ListObjects(ctx, types.Filter{Name: name, ParentID: parentID})
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, types.ErrNotFound
	}

	md := objs[0].Metadata
	c.putChildId2Cache(parentID, md.ID, name)
	return &md, nil
}

func (c *metaCache) createEntry(ctx context.Context, newObj *types.Object, parent *types.Metadata) error {
	objects := make([]*types.Object, 1, 2)
	objects[0] = newObj
	if parent != nil {
		parent.ChangedAt = time.Now()
		objects = append(objects, &types.Object{Metadata: *parent})
		defer c.delEntryCache(parent.ID)
	}
	err := c.metastore.SaveObjects(ctx, objects...)
	if err != nil {
		return err
	}
	return nil
}

func (c *metaCache) updateEntries(ctx context.Context, entries ...*types.Metadata) error {
	var (
		objList = make([]*types.Object, len(entries))
		err     error
	)

	defer func() {
		for _, en := range entries {
			c.delEntryCache(en.ID)
			c.delChildCache(en.ParentID, en.Name)
		}
	}()

	nowAt := time.Now()
	for i := range entries {
		en := entries[i]
		en.ChangedAt = nowAt
		objList[i] = &types.Object{Metadata: *en}
	}

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
	return fmt.Sprintf("e_%d", eid)
}

func (c *metaCache) putChildId2Cache(parentID, childID int64, name string) {
	c.lfu.Put(c.entryChildKey(parentID, name), childID)
}

func (c *metaCache) delChildCache(parentID int64, name string) {
	c.lfu.Remove(c.entryChildKey(parentID, name))
}

func (c *metaCache) entryChildKey(eid int64, name string) string {
	return fmt.Sprintf("e_%d_c_%s", eid, name)
}

func newCacheStore(metastore metastore.ObjectStore) *metaCache {
	cacheStore := &metaCache{
		metastore:  metastore,
		lfu:        utils.NewLFUPool(8192),
		childCache: utils.NewLFUPool(32768),
	}
	return cacheStore
}