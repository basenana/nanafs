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

package bio

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"math/rand"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestChunkIO", func() {
	Context("test one page", func() {
		var reader Reader
		var writer Writer
		It("init should be succeed", func() {
			reader = NewChunkReader(fakeObj, chunkStore, dataStore)
			Expect(reader).ShouldNot(BeNil())
			writer = NewChunkWriter(reader)
			Expect(writer).ShouldNot(BeNil())
		})

		data1 := buildRandomData(0, 0.5)
		data2 := buildRandomData(0, 0.9)
		data3 := buildRandomData(1, 0)
		It("write data should be succeed", func() {
			var (
				n   int64
				err error
			)
			n, err = writer.WriteAt(context.TODO(), data3, 0)
			Expect(int(n)).Should(Equal(len(data3)))
			Expect(err).Should(BeNil())
			n, err = writer.WriteAt(context.TODO(), data2, 0)
			Expect(int(n)).Should(Equal(len(data2)))
			Expect(err).Should(BeNil())
			n, err = writer.WriteAt(context.TODO(), data1, 0)
			Expect(int(n)).Should(Equal(len(data1)))
			Expect(err).Should(BeNil())
			Expect(writer.Fsync(context.TODO())).Should(BeNil())
		})
		It("read data1 should be succeed", func() {
			buf := make([]byte, 10)
			n, err := reader.ReadAt(context.TODO(), buf, 0)
			Expect(int(n)).Should(Equal(len(buf)))
			Expect(err).Should(BeNil())
			Expect(buf).Should(Equal(data1[:10]))
		})
		It("read data2 should be succeed", func() {
			buf := make([]byte, 10)
			n, err := reader.ReadAt(context.TODO(), buf, int64(len(data1)))
			Expect(int(n)).Should(Equal(len(buf)))
			Expect(err).Should(BeNil())
			Expect(buf).Should(Equal(data2[len(data1) : len(data1)+10]))
		})
		It("read data3 should be succeed", func() {
			buf := make([]byte, 10)
			n, err := reader.ReadAt(context.TODO(), buf, int64(len(data2)))
			Expect(int(n)).Should(Equal(len(buf)))
			Expect(err).Should(BeNil())
			Expect(buf).Should(Equal(data3[len(data2) : len(data2)+10]))
		})
	})
})

var _ = Describe("TestSegmentTree", func() {
	type defineSegment struct {
		dataStart int64
		dataEnd   int64
		segList   []simpleSegment
	}
	Context("test tree1", func() {
		var st *segTree
		It("init should be succeed", func() {
			seg := defineSegment{
				dataStart: 0,
				dataEnd:   fileChunkSize,
				segList: []simpleSegment{
					{ID: 1, Start: 0, End: fileChunkSize},
					{ID: 2, Start: 0, End: 3072},
					{ID: 3, Start: 0, End: fileChunkSize},
					{ID: 4, Start: 0, End: 4096},
					{ID: 5, Start: 8192, End: 9216},
				},
			}
			segList := make([]types.ChunkSeg, len(seg.segList))
			for i, seg := range seg.segList {
				segList[i] = types.ChunkSeg{ID: seg.ID, Off: seg.Start, Len: seg.End - seg.Start}
			}
			st = buildSegmentTree(seg.dataStart, seg.dataEnd, segList)
			Expect(st).ShouldNot(BeNil())
		})
		It("query 0-1024 should be succeed", func() {
			Expect(
				reflect.DeepEqual(
					st.query(0, 1024),
					[]segment{{id: 4, off: 0, pos: 0, len: 1024}},
				),
			).Should(BeTrue())
		})
		It("query 1024-4096 should be succeed", func() {
			Expect(
				reflect.DeepEqual(
					st.query(1024, 4096),
					[]segment{{id: 4, off: 1024, pos: 1024, len: 3072}},
				),
			).Should(BeTrue())
		})
		It("query 1024-10240 should be succeed", func() {
			Expect(
				reflect.DeepEqual(
					st.query(1024, 10240),
					[]segment{
						{id: 4, off: 1024, pos: 1024, len: 3072},
						{id: 3, off: 4096, pos: 4096, len: 4096},
						{id: 5, off: 8192, pos: 0, len: 1024},
						{id: 3, off: 9216, pos: 9216, len: 1024},
					},
				),
			).Should(BeTrue())
		})
	})
})

type simpleSegment struct {
	ID    int64
	Start int64
	End   int64
}

func buildRandomData(pageNum int, moreData float64) []byte {
	dataSize := pageNum*pageSize + int(moreData*pageSize)
	data := make([]byte, dataSize)
	_, err := rand.Read(data)
	if err != nil {
		panic(fmt.Sprintf("read random test data failed: %v", err))
	}
	return data
}