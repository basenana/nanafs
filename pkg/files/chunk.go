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

package files

const (
	fileChunkSize  = 1 << 22 // 4MB
	pageSize       = 1 << 12 // 4k
	pageCacheLimit = 1 << 24 // 16MB
	bufQueueLen    = 256
)

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}

func computePageIndex(chunkIndex, off int64) (idx int64, pos int64) {
	off += chunkIndex * fileChunkSize
	idx = off / pageSize
	pos = off % pageSize
	return
}

type cRange struct {
	index  int64
	offset int64
	data   []byte
}
