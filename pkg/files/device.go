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

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	symLinkAnnotationKey = "nanafs.symlink"
)

type Symlink struct {
	data []byte
	obj  *types.Object
	attr Attr
}

func (s *Symlink) GetObject() *types.Object {
	return s.obj
}
func (s *Symlink) GetAttr() Attr {
	return s.attr
}

func (s *Symlink) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	size := offset + int64(len(data))

	if size > int64(len(s.data)) {
		newData := make([]byte, size)
		copy(newData, s.data)
		s.data = newData
	}

	n = int64(copy(s.data[offset:], data))
	s.obj.Size = size
	_ = s.Flush(ctx)
	return
}

func (s *Symlink) Read(ctx context.Context, data []byte, offset int64) (int, error) {
	n := copy(data, s.data[offset:])
	return n, nil
}

func (s *Symlink) Fsync(ctx context.Context) error {
	return s.Flush(ctx)
}

func (s *Symlink) Flush(ctx context.Context) (err error) {
	s.obj.Annotation.Add(&types.AnnotationItem{
		Key:     symLinkAnnotationKey,
		Content: string(s.data),
	})
	return nil
}

func (s *Symlink) Close(ctx context.Context) (err error) {
	return s.Flush(ctx)
}

func openSymlink(obj *types.Object, attr Attr) (*Symlink, error) {
	if obj.Kind != types.SymLinkKind {
		return nil, fmt.Errorf("not symlink")
	}

	var raw []byte
	ann := dentry.GetInternalAnnotation(obj, symLinkAnnotationKey)
	if ann != nil {
		raw = []byte(ann.Content)
	}

	if raw == nil {
		raw = make([]byte, 0, 512)
	}

	return &Symlink{obj: obj, data: raw, attr: attr}, nil
}
