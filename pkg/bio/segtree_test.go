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
	"github.com/basenana/nanafs/pkg/types"
	"reflect"
	"testing"
)

type simpleSegment struct {
	ID    int64
	Start int64
	End   int64
}

type wantedSegmentTree struct {
	Start    int64
	End      int64
	segments []segment
}

func Test_buildSegmentTree(t *testing.T) {
	type args struct {
		dataStart int64
		dataEnd   int64
		segList   []simpleSegment
	}
	tests := []struct {
		name string
		args args
		want []wantedSegmentTree
	}{
		{
			name: "test-one-full-chunk",
			args: args{
				dataStart: 0,
				dataEnd:   fileChunkSize,
				segList: []simpleSegment{
					{ID: 1, Start: 0, End: fileChunkSize},
					{ID: 2, Start: 0, End: 3072},
					{ID: 3, Start: 0, End: fileChunkSize},
					{ID: 4, Start: 0, End: 4096},
					{ID: 5, Start: 8192, End: 9216},
				},
			},
			want: []wantedSegmentTree{
				{Start: 0, End: 1024, segments: []segment{{id: 4, off: 0, pos: 0, len: 1024}}},
				{Start: 1024, End: 4096, segments: []segment{{id: 4, off: 1024, pos: 1024, len: 3072}}},
				{Start: 1024, End: 10240, segments: []segment{
					{id: 4, off: 1024, pos: 1024, len: 3072},
					{id: 3, off: 4096, pos: 4096, len: 4096},
					{id: 5, off: 8192, pos: 0, len: 1024},
					{id: 3, off: 9216, pos: 9216, len: 1024},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segList := make([]types.ChunkSeg, len(tt.args.segList))
			for i, seg := range tt.args.segList {
				segList[i] = types.ChunkSeg{ID: seg.ID, Off: seg.Start, Len: seg.End - seg.Start}
			}
			st := buildSegmentTree(tt.args.dataStart, tt.args.dataEnd, segList)

			for _, testCase := range tt.want {
				marchedSegment := st.query(testCase.Start, testCase.End)
				if !reflect.DeepEqual(marchedSegment, testCase.segments) {
					t.Errorf("query [%d-%d], want: %v, got: %v", testCase.Start, testCase.End, testCase.segments, marchedSegment)
				}
			}
		})
	}
}
