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

package core

import (
	"github.com/basenana/nanafs/pkg/types"
	"github.com/bluele/gcache"
)

type cache struct {
	entryCache gcache.Cache
	childCache gcache.Cache
}

func (c *cache) getEntry(namespace string, id int64) (*types.Entry, error) {
	k := ik{namespace: namespace, id: id}
	cached, err := c.entryCache.Get(k)
	if err != nil {
		return nil, err
	}

	if cached != nil {
		return cached.(*types.Entry), nil
	}

	return nil, types.ErrNotFound
}

func (c *cache) setEntry(entry *types.Entry) {
	k := ik{namespace: entry.Namespace, id: entry.ID}
	_ = c.entryCache.Set(k, entry)
}

func (c *cache) invalidEntry(namespace string, idList ...int64) {
	k := ik{namespace: namespace}
	for _, id := range idList {
		k.id = id
		c.entryCache.Remove(k)
	}
}

func (c *cache) findChild(namespace string, parent int64, name string) (*types.Child, error) {
	k := dk{namespace: namespace, parentID: parent, name: name}
	cached, err := c.childCache.Get(k)
	if err != nil {
		return nil, err
	}

	if cached != nil {
		return cached.(*types.Child), nil
	}

	return nil, types.ErrNotFound
}

func (c *cache) setChild(child *types.Child) {
	k := dk{namespace: child.Namespace, parentID: child.ParentID, name: child.Name}
	_ = c.childCache.Set(k, child)
}

func (c *cache) invalidChild(namespace string, parent int64, name string) {
	k := dk{namespace: namespace, parentID: parent, name: name}
	c.childCache.Remove(k)
}

func newCache() *cache {
	return &cache{
		entryCache: gcache.New(defaultLFUCacheSize).LFU().
			Expiration(defaultLFUCacheExpire).Build(),
		childCache: gcache.New(defaultLFUCacheSize).LRU().
			Expiration(defaultLFUCacheExpire).Build(),
	}
}

type ik struct {
	namespace string
	id        int64
}

type dk struct {
	namespace string
	parentID  int64
	name      string
}
