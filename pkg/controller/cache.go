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

package controller

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/utils"
)

type entryCache struct {
	lfu *utils.LFUPool
}

func (c *entryCache) ResetEntry(entry dentry.Entry) {
	c.lfu.Remove(c.entryKey(entry.Metadata().ID))
}

func (c *entryCache) putEntry(entry dentry.Entry) {
	c.lfu.Put(c.entryKey(entry.Metadata().ID), entry)
}

func (c *entryCache) getEntry(eid int64) dentry.Entry {
	enRaw := c.lfu.Get(c.entryKey(eid))
	if enRaw == nil {
		return nil
	}
	return enRaw.(dentry.Entry)
}

func (c *entryCache) delEntry(eid int64) {
	c.lfu.Remove(c.entryKey(eid))
}

func (c *entryCache) entryKey(eid int64) string {
	return fmt.Sprintf("entry_%d", eid)
}

func initEntryCache() *entryCache {
	return &entryCache{lfu: utils.NewLFUPool(1024)}
}
