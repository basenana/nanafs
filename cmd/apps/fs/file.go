package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/hanwen/go-fuse/v2/fuse"
	"syscall"
)

type File struct {
	node *NanaNode
	file *files.File
}

var _ fileOperation = &File{}

func (f *File) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	_, err := f.file.Read(ctx, dest, off)
	return fuse.ReadResultData(dest), Error2FuseSysError(err)
}

func (f *File) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	cnt, err := f.file.Write(ctx, data, off)
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
