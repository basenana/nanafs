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

package storage

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

func NewMetaStorage(metaType string, meta config.Meta) (Meta, error) {
	switch metaType {
	case MemoryMeta:
		return newMemoryMetaStore(), nil
	case SqliteMeta:
		m, err := newSqliteMetaStore(meta)
		if err != nil {
			return nil, err
		}
		return wrapCachedMeta(m), nil
	default:
		return nil, fmt.Errorf("unknow meta store type: %s", metaType)
	}
}

type cachedMetaStorage struct {
	Meta
	cache *utils.LFUCache
}

func wrapCachedMeta(meta Meta) Meta {
	return &cachedMetaStorage{Meta: meta, cache: utils.NewLFUCache(100)}
}

func (c *cachedMetaStorage) GetObject(ctx context.Context, id int64) (*types.Object, error) {
	cached := c.cache.Get(objectCacheKey(id))
	if cached != nil {
		return cached.(*types.Object), nil
	}
	obj, err := c.Meta.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}
	c.cache.Put(objectCacheKey(id), obj)
	return obj, nil
}

func (c *cachedMetaStorage) SaveObject(ctx context.Context, parent, obj *types.Object) error {
	err := c.Meta.SaveObject(ctx, parent, obj)
	if err != nil {
		return err
	}
	if parent != nil {
		c.cache.Put(objectCacheKey(parent.ID), parent)
	}
	c.cache.Put(objectCacheKey(obj.ID), obj)
	return nil
}

func (c *cachedMetaStorage) DestroyObject(ctx context.Context, src, parent, obj *types.Object) error {
	err := c.Meta.DestroyObject(ctx, src, parent, obj)
	if err != nil {
		return err
	}
	c.cache.Remove(objectCacheKey(obj.ID))
	c.cache.Remove(objectCacheKey(obj.ParentID))
	if src != nil {
		c.cache.Remove(objectCacheKey(src.ID))
	}
	return nil
}

func (c *cachedMetaStorage) MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	err := c.Meta.MirrorObject(ctx, srcObj, dstParent, object)
	if err != nil {
		return err
	}
	c.cache.Remove(objectCacheKey(srcObj.ID))
	c.cache.Remove(objectCacheKey(dstParent.ID))
	c.cache.Remove(objectCacheKey(object.ID))
	return err
}

func (c *cachedMetaStorage) ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	err := c.Meta.ChangeParent(ctx, srcParent, dstParent, obj, opt)
	if err != nil {
		return err
	}
	c.cache.Remove(objectCacheKey(srcParent.ID))
	c.cache.Remove(objectCacheKey(dstParent.ID))
	c.cache.Remove(objectCacheKey(obj.ID))
	return nil
}

func (c *cachedMetaStorage) AppendSegments(ctx context.Context, seg types.ChunkSeg, obj *types.Object) error {
	if err := c.Meta.AppendSegments(ctx, seg, obj); err != nil {
		return err
	}
	c.cache.Put(objectCacheKey(obj.ID), obj)
	return nil
}

func objectCacheKey(oid int64) string {
	return fmt.Sprintf("obj_%d", oid)
}
