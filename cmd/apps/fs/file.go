package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/hanwen/go-fuse/v2/fuse"
	"io"
	"syscall"
)

type File struct {
	node *NanaNode
	obj  *types.Object
	file files.File
}

var _ fileOperation = &File{}

func (f *File) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	obj, err := f.node.R.GetObject(ctx, f.obj.ID)
	if err != nil {
		if f != nil && err == types.ErrNotFound && f.obj != nil {
			f.obj.RefCount = 0
			st := nanaNode2Stat(f.obj)
			updateAttrOut(st, &out.Attr)
			return NoErr
		}
		return Error2FuseSysError(err)
	}

	st := nanaNode2Stat(obj)
	updateAttrOut(st, &out.Attr)
	return NoErr
}

func (f *File) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	defer utils.TraceRegion(ctx, "files.read")()
	n, err := f.node.R.ReadFile(ctx, f.file, dest, off)
	if err != nil && err != io.EOF {
		return fuse.ReadResultData(dest), Error2FuseSysError(err)
	}
	return fuse.ReadResultData(dest[:n]), NoErr
}

func (f *File) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	defer utils.TraceRegion(ctx, "files.write")()
	cnt, err := f.node.R.WriteFile(ctx, f.file, data, off)
	return uint32(cnt), Error2FuseSysError(err)
}

func (f *File) Flush(ctx context.Context) syscall.Errno {
	defer utils.TraceRegion(ctx, "files.flush")()
	return Error2FuseSysError(f.file.Flush(ctx))
}

func (f *File) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	defer utils.TraceRegion(ctx, "files.fsync")()
	return Error2FuseSysError(f.file.Fsync(ctx))
}

func (f *File) Release(ctx context.Context) syscall.Errno {
	defer utils.TraceRegion(ctx, "files.release")()
	return Error2FuseSysError(f.node.R.CloseFile(ctx, f.file))
}
