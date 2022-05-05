// +build darwin

package fs

import (
	"context"
	"github.com/basenana/nanafs/utils"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"syscall"
)

func (n *NanaNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.getattr")()
	file, ok := f.(fs.FileGetattrer)
	if ok {
		return file.Getattr(ctx, out)
	}
	st := nanaNode2Stat(n)
	out.FromStat(st)

	// macos
	out.Crtime_ = uint64(st.Ctimespec.Sec)
	out.Crtimensec_ = uint32(st.Ctimespec.Nsec)
	return NoErr
}
