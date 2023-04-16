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

package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hanwen/go-fuse/v2/fuse"
	"io"
	"runtime/trace"
	"syscall"
)

type File struct {
	node *NanaNode
	file dentry.File
}

var _ fileOperation = &File{}

func (f *File) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.file.Getattr").End()
	entry, err := f.node.R.GetEntry(ctx, f.file.Metadata().ID)
	if err != nil {
		if err == types.ErrNotFound && f.file != nil {
			f.file.Metadata().RefCount = 0
			st := nanaNode2Stat(f.file)
			updateAttrOut(st, &out.Attr)
			return NoErr
		}
		return Error2FuseSysError(err)
	}

	st := nanaNode2Stat(entry)
	cachedSt := nanaNode2Stat(f.file)
	st.Size = cachedSt.Size
	st.Mtimespec = cachedSt.Mtimespec
	updateAttrOut(st, &out.Attr)
	return NoErr
}

func (f *File) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.file.Read").End()
	n, err := f.node.R.ReadFile(ctx, f.file, dest, off)
	if err != nil && err != io.EOF {
		return fuse.ReadResultData(dest), Error2FuseSysError(err)
	}
	return fuse.ReadResultData(dest[:n]), NoErr
}

func (f *File) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.file.Write").End()
	cnt, err := f.node.R.WriteFile(ctx, f.file, data, off)
	return uint32(cnt), Error2FuseSysError(err)
}

func (f *File) Flush(ctx context.Context) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.file.Flush").End()
	return Error2FuseSysError(f.file.Flush(ctx))
}

func (f *File) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.file.Fsync").End()
	return Error2FuseSysError(f.file.Fsync(ctx))
}

func (f *File) Release(ctx context.Context) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.file.Release").End()
	return Error2FuseSysError(f.node.R.CloseFile(ctx, f.file))
}
