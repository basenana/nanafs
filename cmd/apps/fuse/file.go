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

package fuse

import (
	"context"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"io"
	"runtime/trace"
	"syscall"
	"time"
)

type fileOperation interface {
	fs.FileGetattrer
	fs.FileReader
	fs.FileWriter
	fs.FileFlusher
	fs.FileFsyncer
	fs.FileReleaser
}

type File struct {
	node *NanaNode
	file core.RawFile
	attr types.OpenAttr
}

var _ fileOperation = &File{}

func (f *File) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.file.Getattr").End()
	defer logOperationLatency("file_get_attr", time.Now())

	return f.node.Getattr(ctx, nil, out)
}

func (f *File) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.file.Read").End()
	defer logOperationLatency("file_read", time.Now())
	n, err := f.file.ReadAt(ctx, dest, off)
	if err != nil && err != io.EOF {
		return fuse.ReadResultData(dest), Error2FuseSysError("file_read", err)
	}
	return fuse.ReadResultData(dest[:n]), NoErr
}

func (f *File) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.file.Write").End()
	defer logOperationLatency("file_write", time.Now())
	cnt, err := f.file.WriteAt(ctx, data, off)
	return uint32(cnt), Error2FuseSysError("file_write", err)
}

func (f *File) Flush(ctx context.Context) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.file.Flush").End()
	defer logOperationLatency("file_flush", time.Now())
	return Error2FuseSysError("file_flush", f.file.Flush(ctx))
}

func (f *File) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.file.Fsync").End()
	defer logOperationLatency("file_fsync", time.Now())
	return Error2FuseSysError("file_fsync", f.file.Fsync(ctx))
}

func (f *File) Release(ctx context.Context) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.file.Release").End()
	defer logOperationLatency("file_release", time.Now())
	return Error2FuseSysError("file_release", f.file.Close(ctx))
}
