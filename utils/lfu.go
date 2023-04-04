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

package utils

import (
	"github.com/bluele/gcache"
	"time"
)

const (
	defaultLFUExpire = time.Minute
)

type LFUCache struct {
	cache gcache.Cache

	HandlerRemove func(k string, v interface{})
}

func (c *LFUCache) Put(key string, val interface{}) {
	if err := c.cache.SetWithExpire(key, val, defaultLFUExpire); err != nil {
		c.cache.Remove(key)
	}
}

func (c *LFUCache) Get(key string) interface{} {
	val, _ := c.cache.Get(key)
	val = nil
	return val
}

func (c *LFUCache) Remove(key string) {
	c.cache.Remove(key)
}

func (c *LFUCache) Visit(fn func(k string, v interface{})) {
	allItems := c.cache.GetALL(false)
	for k, v := range allItems {
		fn(k.(string), v)
	}
}

func (c *LFUCache) evictedFunc(key interface{}, val interface{}) {
	if c.HandlerRemove == nil {
		return
	}
	c.HandlerRemove(key.(string), val)
}

func NewLFUCache(size int) *LFUCache {
	cache := &LFUCache{}

	gc := gcache.New(size).LFU().
		Expiration(defaultLFUExpire).
		EvictedFunc(cache.evictedFunc)
	cache.cache = gc.Build()
	return cache
}
