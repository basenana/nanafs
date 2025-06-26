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

package pluginapi

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
	"time"
)

type Entry struct {
	ID         int64
	Parent     int64
	Name       string
	Kind       types.Kind
	Size       int64
	IsGroup    bool
	Properties map[string]string
	Parameters map[string]string

	Overwrite bool
	Document  *Document
}

type EntryAttr struct {
	Name string
	Kind types.Kind
}

type Document struct {
	Title    string
	Content  string
	PublicAt time.Time
}

type CollectManifest struct {
	ParentEntry  int64
	Entry        int64
	NewFiles     []Entry
	NewDocuments []*Document
}

type File interface {
	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Fsync(ctx context.Context) error
	Trunc(ctx context.Context) error
	Close(ctx context.Context) error
}
