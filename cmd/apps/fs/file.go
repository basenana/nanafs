package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/hanwen/go-fuse/v2/fuse"
	"io"
	"syscall"
)

type File struct {
	node *NanaNode
	file *files.File
}

var _ fileOperation = &File{}

func (f *File) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n, err := f.node.R.ReadFile(ctx, f.file, dest, off)
	if err != nil && err != io.EOF {
		return fuse.ReadResultData(dest), Error2FuseSysError(err)
	}
	return fuse.ReadResultData(dest[:n]), NoErr
}

func (f *File) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	cnt, err := f.node.R.WriteFile(ctx, f.file, data, off)
	return uint32(cnt), Error2FuseSysError(err)
}

func (f *File) Flush(ctx context.Context) syscall.Errno {
	return Error2FuseSysError(f.file.Flush(ctx))
}

func (f *File) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	return Error2FuseSysError(f.file.Fsync(ctx))
}

func (f *File) Release(ctx context.Context) syscall.Errno {
	return Error2FuseSysError(f.file.Close(ctx))
}
