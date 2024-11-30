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

import "testing"

func TestContentTrim(t *testing.T) {
	type args struct {
		contentType string
		content     string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				contentType: "html",
				content:     "<h1>abc</h1><div id=\"readability-page-1\" class=\"page\"><p data-check-id=\"619724\">def</p><p data-check-id=\"619724\">xyz</p>",
			},
			want: "abcdef xyz ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContentTrim(tt.args.contentType, tt.args.content); got != tt.want {
				t.Errorf("ContentTrim() = %v, want %v", got, tt.want)
			}
		})
	}
}
