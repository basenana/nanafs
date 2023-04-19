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
	"compress/gzip"
	"context"
	"io"
	"runtime/trace"
)

type nodeUsingInfo struct {
	filename string
	node     CacheNode
	index    int
	updateAt int64
}

type priorityNodeQueue []*nodeUsingInfo

func (pq *priorityNodeQueue) Len() int {
	return len(*pq)
}

func (pq *priorityNodeQueue) Less(i, j int) bool {
	if (*pq)[i].node.freq() == (*pq)[j].node.freq() {
		return (*pq)[i].updateAt < (*pq)[j].updateAt
	}
	return (*pq)[i].node.freq() < (*pq)[j].node.freq()
}

func (pq *priorityNodeQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
	(*pq)[i].index = i
	(*pq)[j].index = j
}

func (pq *priorityNodeQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*nodeUsingInfo)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityNodeQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

func compress(ctx context.Context, in io.Reader, out io.Writer) error {
	defer trace.StartRegion(ctx, "storage.localCache.compress").End()
	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		return err
	}

	if err := gz.Close(); err != nil {
		return err
	}

	return nil
}

func decompress(ctx context.Context, in io.Reader, out io.Writer) error {
	defer trace.StartRegion(ctx, "storage.localCache.decompress").End()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, gz); err != nil {
		_ = gz.Close()
		return err
	}

	return gz.Close()
}

func reverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
