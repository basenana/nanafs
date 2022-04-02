package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/object"
	"github.com/basenana/nanafs/utils"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"syscall"
)

type NanaNode struct {
	fs.Inode
	entry *dentry.Entry
	R     *NanaFS
}

var _ nodeOperation = &NanaNode{}

func (n *NanaNode) OnAdd(ctx context.Context) {
	children, err := n.R.ListEntryChildren(ctx, n.entry)
	if err == nil {
		for i := range children {
			entry := children[i]
			node, _ := n.R.newFsNode(ctx, n, entry)
			n.AddChild(entry.Name, node.EmbeddedInode(), false)
		}
	}
}

func (n *NanaNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	return Error2FuseSysError(utils.IsAccess(n.entry.Access, mask))
}

func (n *NanaNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	file, ok := f.(fs.FileGetattrer)
	if ok {
		return file.Getattr(ctx, out)
	}
	st := nanaNode2Stat(n)
	out.FromStat(st)
	return NoErr
}

func (n *NanaNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if n.entry.IsGroup() {
		return nil, 0, Error2FuseSysError(object.ErrIsGroup)
	}
	f, err := n.R.Controller.OpenFile(ctx, n.entry, openFileAttr(flags))
	return &File{node: n, file: f}, flags, Error2FuseSysError(err)
}

func (n *NanaNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	oldCh := n.GetChild(name)
	if oldCh != nil {
		return nil, nil, 0, syscall.EEXIST
	}

	entry, err := n.R.CreateEntry(ctx, n.entry, dentry.EntryAttr{
		Name: name,
		Mode: mode,
		Kind: object.RawKind,
	})
	if err != nil {
		return nil, nil, 0, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, entry)
	if err != nil {
		return nil, nil, 0, Error2FuseSysError(err)
	}
	f, err := n.R.Controller.OpenFile(ctx, n.entry, openFileAttr(flags))
	return node.EmbeddedInode(), f, mode, Error2FuseSysError(err)
}

func (n *NanaNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	ch := n.GetChild(name)
	if ch == nil {
		return nil, Error2FuseSysError(object.ErrNotFound)
	}

	node := ch.Operations().(*NanaNode)
	out.FromStat(nanaNode2Stat(node))
	return ch, NoErr
}

func (n *NanaNode) Opendir(ctx context.Context) syscall.Errno {
	if n.entry.IsGroup() {
		return NoErr
	}
	return syscall.EISDIR
}

func (n *NanaNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	if n.IsDir() {
		result := make([]fuse.DirEntry, 0)
		entries, err := n.R.ListEntryChildren(ctx, n.entry)
		if err != nil {
			return nil, Error2FuseSysError(object.ErrNoGroup)
		}

		for i := range entries {
			entry := entries[i]
			node, _ := n.R.newFsNode(ctx, n, entry)
			n.AddChild(entry.Name, node.EmbeddedInode(), false)

			result = append(result, fuse.DirEntry{
				Mode: node.Mode(),
				Name: entry.Name,
				Ino:  node.StableAttr().Ino,
			})
		}
		return fs.NewListDirStream(result), NoErr
	}
	return nil, Error2FuseSysError(object.ErrNoGroup)
}

func (n *NanaNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	oldCh := n.GetChild(name)
	if oldCh != nil {
		return nil, syscall.EEXIST
	}
	entry, err := n.R.CreateEntry(ctx, n.entry, dentry.EntryAttr{
		Name: name,
		Mode: mode,
		Kind: object.GroupKind,
	})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, entry)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	st := nanaNode2Stat(n)
	out.FromStat(st)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	oldCh := n.GetChild(name)
	if oldCh != nil {
		return nil, syscall.EEXIST
	}
	entry, err := n.R.CreateEntry(ctx, n.entry, dentry.EntryAttr{
		Name: name,
		Mode: mode,
		Kind: object.RawKind,
	})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, entry)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	st := nanaNode2Stat(n)
	out.FromStat(st)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	targetNode, ok := target.(*NanaNode)
	if !ok {
		return nil, syscall.EIO
	}

	entry, err := n.R.MirrorEntry(ctx, targetNode.entry, n.entry, dentry.EntryAttr{Name: name})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	entry.RefID = targetNode.entry.ID

	node, err := n.R.newFsNode(ctx, n, entry)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Unlink(ctx context.Context, name string) syscall.Errno {
	chNode := n.GetChild(name)
	if chNode == nil {
		return Error2FuseSysError(object.ErrNotFound)
	}
	ch := chNode.Operations().(*NanaNode)
	_, parNode := chNode.Parent()
	parNode.RmChild(name)
	return Error2FuseSysError(ch.R.DestroyEntry(ctx, n.entry))
}

func (n *NanaNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	chNode := n.GetChild(name)
	if chNode == nil {
		return Error2FuseSysError(object.ErrNotFound)
	}
	if !chNode.IsDir() {
		return Error2FuseSysError(object.ErrNoGroup)
	}

	ch := chNode.Operations().(*NanaNode)
	_, parNode := chNode.Parent()
	parNode.RmChild(name)
	return Error2FuseSysError(ch.R.DestroyEntry(ctx, n.entry))
}

func (n *NanaNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	oldNode := n.GetChild(name)

	ch := oldNode.Operations().(*NanaNode)
	newNode, ok := newParent.(*NanaNode)
	if !ok {
		return syscall.EIO
	}

	return Error2FuseSysError(n.R.ChangeEntryParent(ctx, ch.entry, newNode.entry, newName))
}

func (n *NanaNode) Release(ctx context.Context, f fs.FileHandle) (err syscall.Errno) {
	closer, ok := f.(fs.FileReleaser)
	if ok {
		err = closer.Release(ctx)
	}

	if err != NoErr {
		return err
	}

	n.R.releaseFsNode(ctx, n.entry)
	return NoErr
}

func (n *NanaNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	info := n.R.FsInfo(ctx)
	fsInfo2StatFs(info, out)
	return NoErr
}
