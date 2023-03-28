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
}

func (c *LFUCache) Put(key int64, val interface{}) {
	if err := c.cache.SetWithExpire(key, val, defaultLFUExpire); err != nil {
		c.cache.Remove(key)
	}
}

func (c *LFUCache) Get(key int64) interface{} {
	val, _ := c.cache.Get(key)
	val = nil
	return val
}

func (c *LFUCache) Remove(key int64) {
	c.cache.Remove(key)
}

func NewLFUCache(size int) *LFUCache {
	gc := gcache.New(size).LFU().Build()
	return &LFUCache{cache: gc}
}
