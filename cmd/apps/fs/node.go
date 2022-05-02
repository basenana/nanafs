package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.uber.org/zap"
	"syscall"
)

type NanaNode struct {
	fs.Inode
	obj    *types.Object
	R      *NanaFS
	logger *zap.SugaredLogger
}

var _ nodeOperation = &NanaNode{}

func (n *NanaNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.access")()
	return Error2FuseSysError(dentry.IsAccess(n.obj.Access, mask))
}

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

func (n *NanaNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.setattr")()
	updateNanaNodeWithAttr(in, n)
	if err := n.R.SaveObject(ctx, n.obj); err != nil {
		return Error2FuseSysError(err)
	}
	return n.Getattr(ctx, f, out)
}

func (n *NanaNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.getxattr")()
	ann := dentry.GetInternalAnnotation(n.obj, attr)
	if ann == nil {
		return 0, syscall.ENOATTR
	}
	raw, err := dentry.AnnotationContent2RawData(ann)
	if err != nil {
		return 0, Error2FuseSysError(err)
	}
	if len(raw) > len(dest) {
		return uint32(len(raw)), syscall.ERANGE
	}

	copy(dest, raw)
	return uint32(len(raw)), NoErr
}

func (n *NanaNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.setxattr")()
	dentry.AddInternalAnnotation(n.obj, attr, dentry.RawData2AnnotationContent(data), true)
	return Error2FuseSysError(n.R.SaveObject(ctx, n.obj))
}

func (n *NanaNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.open")()
	if n.obj.IsGroup() {
		return nil, 0, Error2FuseSysError(types.ErrIsGroup)
	}
	f, err := n.R.Controller.OpenFile(ctx, n.obj, openFileAttr(flags))
	return &File{node: n, file: f}, flags, Error2FuseSysError(err)
}

func (n *NanaNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.create")()
	ch, err := n.R.FindObject(ctx, n.obj, name)
	if err != nil && err != types.ErrNotFound {
		return nil, nil, 0, Error2FuseSysError(err)
	}
	if ch != nil && flags|syscall.O_EXCL > 0 {
		return nil, nil, 0, syscall.EEXIST
	}

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	obj, err := n.R.CreateObject(ctx, n.obj, types.ObjectAttr{
		Name:        name,
		Kind:        types.RawKind,
		Permissions: acc.Permissions,
	})
	if err != nil {
		return nil, nil, 0, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, obj)
	if err != nil {
		return nil, nil, 0, Error2FuseSysError(err)
	}
	f, err := n.R.Controller.OpenFile(ctx, obj, openFileAttr(flags))
	return node.EmbeddedInode(), &File{node: n, file: f}, dentry.Access2Mode(obj.Access), Error2FuseSysError(err)
}

func (n *NanaNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.lookup")()
	ch, err := n.R.FindObject(ctx, n.obj, name)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, ch)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Opendir(ctx context.Context) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.opendir")()
	if n.obj.IsGroup() {
		return NoErr
	}
	return syscall.EISDIR
}

func (n *NanaNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.readdir")()
	if n.IsDir() {
		result := make([]fuse.DirEntry, 0)
		children, err := n.R.ListObjectChildren(ctx, n.obj)
		if err != nil {
			return nil, Error2FuseSysError(types.ErrNoGroup)
		}

		for i := range children {
			ch := children[i]
			node, _ := n.R.newFsNode(ctx, n, ch)
			n.AddChild(ch.Name, node.EmbeddedInode(), false)

			result = append(result, fuse.DirEntry{
				Mode: node.Mode(),
				Name: ch.Name,
				Ino:  node.StableAttr().Ino,
			})
		}
		return fs.NewListDirStream(result), NoErr
	}
	return nil, Error2FuseSysError(types.ErrNoGroup)
}

func (n *NanaNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.mkdir")()
	ch, err := n.R.FindObject(ctx, n.obj, name)
	if err != nil && err != types.ErrNotFound {
		return nil, Error2FuseSysError(err)
	}
	if ch != nil {
		return nil, syscall.EEXIST
	}
	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	obj, err := n.R.CreateObject(ctx, n.obj, types.ObjectAttr{
		Name:        name,
		Kind:        types.GroupKind,
		Permissions: acc.Permissions,
	})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, obj)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	st := nanaNode2Stat(n)
	out.FromStat(st)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.mknod")()
	ch, err := n.R.FindObject(ctx, n.obj, name)
	if err != nil && err != types.ErrNotFound {
		return nil, Error2FuseSysError(err)
	}
	if ch != nil {
		return nil, syscall.EEXIST
	}

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	obj, err := n.R.CreateObject(ctx, n.obj, types.ObjectAttr{
		Name:        name,
		Kind:        types.RawKind,
		Permissions: acc.Permissions,
	})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, obj)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	st := nanaNode2Stat(n)
	out.FromStat(st)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.link")()
	targetNode, ok := target.(*NanaNode)
	if !ok {
		return nil, syscall.EIO
	}

	obj, err := n.R.MirrorObject(ctx, targetNode.obj, n.obj, types.ObjectAttr{Name: name})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	node, err := n.R.newFsNode(ctx, n, obj)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Unlink(ctx context.Context, name string) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.unlink")()
	ch, err := n.R.FindObject(ctx, n.obj, name)
	if err != nil {
		return Error2FuseSysError(err)
	}
	if err = n.R.DestroyObject(ctx, ch); err != nil {
		return Error2FuseSysError(err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.rmdir")()
	ch, err := n.R.FindObject(ctx, n.obj, name)
	if err != nil {
		return Error2FuseSysError(err)
	}
	if !ch.IsGroup() {
		return Error2FuseSysError(types.ErrNoGroup)
	}

	if err = n.R.DestroyObject(ctx, ch); err != nil {
		return Error2FuseSysError(err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.rmname")()
	oldObject, err := n.R.FindObject(ctx, n.obj, name)
	if err != nil {
		return Error2FuseSysError(err)
	}
	newNode, ok := newParent.(*NanaNode)
	if !ok {
		return syscall.EIO
	}
	opt := controller.ChangeParentOpt{Replace: true}
	if flags&RENAME_EXCHANGE > 0 {
		opt.Exchange = true
	}
	if flags&RENAME_NOREPLACE > 0 {
		opt.Replace = false
	}
	if err = n.R.ChangeObjectParent(ctx, oldObject, newNode.obj, newName, opt); err != nil {
		return Error2FuseSysError(err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) OnAdd(ctx context.Context) {
	defer utils.TraceRegion(ctx, "node.onadd")()
	children, err := n.R.ListObjectChildren(ctx, n.obj)
	if err == nil {
		for i := range children {
			obj := children[i]
			node, _ := n.R.newFsNode(ctx, n, obj)
			n.AddChild(obj.Name, node.EmbeddedInode(), false)
		}
	}
}

func (n *NanaNode) Release(ctx context.Context, f fs.FileHandle) (err syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.release")()
	closer, ok := f.(fs.FileReleaser)
	if ok {
		err = closer.Release(ctx)
	}

	if err != NoErr {
		return err
	}

	n.R.releaseFsNode(ctx, n.obj)
	return NoErr
}

func (n *NanaNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.statfs")()
	info := n.R.FsInfo(ctx)
	fsInfo2StatFs(info, out)
	return NoErr
}
