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
	"sync"
)

const (
	memoryBlockSize = 1 << 20 // 1M
)

var (
	memoryBlockPools = map[int]*sync.Pool{
		2:  {New: func() any { return make([]byte, memoryBlockSize*2) }},
		4:  {New: func() any { return make([]byte, memoryBlockSize*4) }},
		8:  {New: func() any { return make([]byte, memoryBlockSize*8) }},
		16: {New: func() any { return make([]byte, memoryBlockSize*16) }},
	}
)

func NewMemoryBlock(blkSize int64) []byte {
	needBs := int((blkSize / memoryBlockSize) + 1)
	for bs, pool := range memoryBlockPools {
		if bs >= needBs {
			return pool.Get().([]byte)
		}
	}
	return nil
}

func ReleaseMemoryBlock(blk []byte) {
	if blk == nil {
		return
	}
	for i := range blk {
		blk[i] = 0
	}
	for bs, pool := range memoryBlockPools {
		if bs == len(blk) {
			pool.Put(blk)
			break
		}
	}
}

func ExtendMemoryBlock(blk []byte, expectedSize int64) []byte {
	blkSize := int64(len(blk))
	if blkSize >= expectedSize {
		return blk
	}

	var newBlk []byte
	for bs, pool := range memoryBlockPools {
		if int64(bs)*memoryBlockSize >= expectedSize {
			newBlk = pool.Get().([]byte)
			break
		}
	}

	if newBlk == nil {
		return nil
	}
	copy(newBlk, blk)
	ReleaseMemoryBlock(blk)
	return newBlk
}
