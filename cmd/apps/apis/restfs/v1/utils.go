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

package v1

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"io"
)

type file struct {
	f      dentry.File
	offset int64
}

func (f *file) Read(p []byte) (n int, err error) {
	n64, err := f.f.ReadAt(context.Background(), p, f.offset)
	f.offset += n64
	if f.offset == f.f.Metadata().Size {
		err = io.EOF
	}
	n = int(n64)
	return
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		f.offset = f.f.Metadata().Size + int64(whence)
	}
	return f.offset, nil
}

func defaultAccess() []types.Permission {
	return []types.Permission{
		types.PermOwnerRead,
		types.PermOwnerWrite,
		types.PermGroupRead,
		types.PermGroupWrite,
		types.PermOthersRead,
	}
}
