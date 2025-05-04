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

type LFUPool struct {
	cache gcache.Cache

	HandlerRemove func(k interface{}, v interface{})
}

func (c *LFUPool) Put(key interface{}, val interface{}) {
	if err := c.cache.SetWithExpire(key, val, defaultLFUExpire); err != nil {
		c.cache.Remove(key)
	}
}

func (c *LFUPool) Get(key interface{}) interface{} {
	val, _ := c.cache.Get(key)
	return val
}

func (c *LFUPool) Remove(key interface{}) {
	c.cache.Remove(key)
}

func (c *LFUPool) Visit(fn func(k interface{}, v interface{})) {
	allItems := c.cache.GetALL(false)
	for k, v := range allItems {
		fn(k.(string), v)
	}
}

func (c *LFUPool) evictedFunc(key interface{}, val interface{}) {
	if c.HandlerRemove == nil {
		return
	}
	c.HandlerRemove(key, val)
}

func NewLFUPool(size int) *LFUPool {
	cache := &LFUPool{}

	gc := gcache.New(size).LFU().
		Expiration(defaultLFUExpire).
		EvictedFunc(cache.evictedFunc)
	cache.cache = gc.Build()
	return cache
}
