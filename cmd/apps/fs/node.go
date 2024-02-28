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
	"runtime/trace"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

type nodeOperation interface {
	fs.NodeOnAdder

	fs.NodeAccesser
	fs.NodeStatfser
	fs.NodeGetattrer
	fs.NodeSetattrer
	fs.NodeGetxattrer
	fs.NodeSetxattrer
	fs.NodeRemovexattrer
	fs.NodeOpener
	fs.NodeLookuper
	fs.NodeCreater
	fs.NodeOpendirer
	fs.NodeReaddirer
	fs.NodeMkdirer
	fs.NodeMknoder
	fs.NodeLinker
	fs.NodeSymlinker
	fs.NodeReadlinker
	fs.NodeUnlinker
	fs.NodeRmdirer
	fs.NodeRenamer
	fs.NodeReleaser
}

type NanaNode struct {
	fs.Inode
	R *NanaFS

	entryID int64
	logger  *zap.SugaredLogger
}

var _ nodeOperation = &NanaNode{}

func (n *NanaNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Access").End()
	defer logOperationLatency("entry_access", time.Now())

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	entry, err := n.R.GetEntry(ctx, n.entryID)
	if err != nil {
		return Error2FuseSysError("entry_access", err)
	}

	return Error2FuseSysError("entry_access", dentry.IsAccess(entry.Access, int64(uid), int64(gid), mask))
}

func (n *NanaNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Getattr").End()
	defer logOperationLatency("entry_get_attr", time.Now())
	file, ok := f.(fs.FileGetattrer)
	if ok {
		return file.Getattr(ctx, out)
	}

	entry, err := n.R.GetSourceEntry(ctx, n.entryID)
	if err != nil {
		return Error2FuseSysError("entry_get_attr", err)
	}

	st := nanaNode2Stat(entry)
	updateAttrOut(st, &out.Attr)
	return NoErr
}

func (n *NanaNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Setattr").End()
	defer logOperationLatency("entry_set_attr", time.Now())
	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	var attr types.OpenAttr
	nanaFile, ok := f.(*File)
	if ok {
		attr = nanaFile.file.GetAttr()
	}

	entry, err := n.R.GetEntry(ctx, n.entryID)
	if err != nil {
		return Error2FuseSysError("entry_set_attr", err)
	}
	if err = updateNanaNodeWithAttr(in, entry, int64(uid), int64(gid), attr); err != nil {
		return Error2FuseSysError("entry_set_attr", err)
	}
	if err = n.R.UpdateEntry(ctx, entry); err != nil {
		return Error2FuseSysError("entry_set_attr", err)
	}
	return n.Getattr(ctx, f, out)
}

func (n *NanaNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Getxattr").End()
	defer logOperationLatency("entry_get_xattr", time.Now())
	data, err := n.R.GetEntryExtendField(ctx, n.entryID, attr)
	if err != nil {
		return 0, Error2FuseSysError("entry_get_xattr", err)
	}
	if data == nil {
		return 0, ENOAttr
	}
	if len(data) > len(dest) {
		return uint32(len(data)), syscall.ERANGE
	}

	copy(dest, data)
	return uint32(len(data)), NoErr
}

func (n *NanaNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Setxattr").End()
	defer logOperationLatency("entry_set_xattr", time.Now())
	if err := n.R.SetEntryEncodedExtendField(ctx, n.entryID, attr, data); err != nil {
		return Error2FuseSysError("entry_set_xattr", err)
	}
	return NoErr
}

func (n *NanaNode) Removexattr(ctx context.Context, attr string) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Removexattr").End()
	defer logOperationLatency("entry_remove_xattr", time.Now())
	if err := n.R.RemoveEntryExtendField(ctx, n.entryID, attr); err != nil {
		return syscall.Errno(0x5d)
	}
	return NoErr
}

func (n *NanaNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Open").End()
	defer logOperationLatency("entry_open", time.Now())
	entry, err := n.R.GetSourceEntry(ctx, n.entryID)
	if err != nil {
		return nil, 0, Error2FuseSysError("entry_open", err)
	}
	if types.IsGroup(entry.Kind) {
		return nil, 0, Error2FuseSysError("entry_open", types.ErrIsGroup)
	}
	f, err := n.R.Controller.OpenFile(ctx, n.entryID, openFileAttr(flags))
	return &File{node: n, entry: entry, file: f}, flags, Error2FuseSysError("entry_open", err)
}

func (n *NanaNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Create").End()
	defer logOperationLatency("entry_create", time.Now())

	ch, err := n.R.FindEntry(ctx, n.entryID, name)
	if err != nil && err != types.ErrNotFound {
		return nil, nil, 0, Error2FuseSysError("entry_create", err)
	}
	if ch != nil && flags|syscall.O_EXCL > 0 {
		return nil, nil, 0, syscall.EEXIST
	}

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		dentry.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}
	newCh, err := n.R.CreateEntry(ctx, n.entryID, types.EntryAttr{
		Name:   name,
		Kind:   fileKindFromMode(mode),
		Access: acc,
		Dev:    int64(MountDev),
	})
	if err != nil {
		return nil, nil, 0, Error2FuseSysError("entry_create", err)
	}
	node, err := n.R.newFsNode(ctx, n, newCh)
	if err != nil {
		return nil, nil, 0, Error2FuseSysError("entry_create", err)
	}
	updateAttrOut(nanaNode2Stat(newCh), &out.Attr)
	n.AddChild(name, node.EmbeddedInode(), true)

	f, err := n.R.Controller.OpenFile(ctx, newCh.ID, openFileAttr(flags))
	return node.EmbeddedInode(), &File{node: node, entry: newCh, file: f}, dentry.Access2Mode(newCh.Access), Error2FuseSysError("entry_create", err)
}

func (n *NanaNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Lookup").End()
	defer logOperationLatency("entry_lookup", time.Now())

	ch, err := n.R.FindEntry(ctx, n.entryID, name)
	if err != nil {
		return nil, Error2FuseSysError("entry_lookup", err)
	}

	ch, err = n.R.GetSourceEntry(ctx, ch.ID)
	if err != nil {
		n.logger.Errorw("query source entry failed", "err", err)
		return nil, Error2FuseSysError("entry_lookup", err)
	}

	node, err := n.R.newFsNode(ctx, n, ch)
	if err != nil {
		return nil, Error2FuseSysError("entry_lookup", err)
	}
	updateAttrOut(nanaNode2Stat(ch), &out.Attr)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Opendir(ctx context.Context) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Opendir").End()
	defer logOperationLatency("entry_open_dir", time.Now())
	entry, err := n.R.GetEntry(ctx, n.entryID)
	if err != nil {
		return Error2FuseSysError("entry_open_dir", err)
	}
	if types.IsGroup(entry.Kind) {
		return NoErr
	}
	return syscall.EISDIR
}

func (n *NanaNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Readdir").End()
	defer logOperationLatency("entry_read_dir", time.Now())
	result := make([]fuse.DirEntry, 0)
	children, err := n.R.ListEntryChildren(ctx, n.entryID)
	if err != nil {
		return nil, Error2FuseSysError("entry_read_dir", types.ErrNoGroup)
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

func (n *NanaNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Mkdir").End()
	defer logOperationLatency("entry_mkdir", time.Now())
	ch, err := n.R.FindEntry(ctx, n.entryID, name)
	if err != nil && err != types.ErrNotFound {
		return nil, Error2FuseSysError("entry_mkdir", err)
	}
	if ch != nil {
		return nil, syscall.EEXIST
	}
	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		dentry.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}
	newDir, err := n.R.CreateEntry(ctx, n.entryID, types.EntryAttr{
		Name:   name,
		Kind:   types.GroupKind,
		Access: acc,
		Dev:    int64(MountDev),
	})
	if err != nil {
		return nil, Error2FuseSysError("entry_mkdir", err)
	}

	node, err := n.R.newFsNode(ctx, n, newDir)
	if err != nil {
		return nil, Error2FuseSysError("entry_mkdir", err)
	}
	updateAttrOut(nanaNode2Stat(newDir), &out.Attr)
	n.AddChild(name, node.EmbeddedInode(), true)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Mknod").End()
	defer logOperationLatency("entry_mknod", time.Now())
	ch, err := n.R.FindEntry(ctx, n.entryID, name)
	if err != nil && err != types.ErrNotFound {
		return nil, Error2FuseSysError("entry_mknod", err)
	}
	if ch != nil {
		return nil, syscall.EEXIST
	}

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		dentry.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}
	newCh, err := n.R.CreateEntry(ctx, n.entryID, types.EntryAttr{
		Name:   name,
		Kind:   fileKindFromMode(mode),
		Access: acc,
		Dev:    int64(dev),
	})
	if err != nil {
		return nil, Error2FuseSysError("entry_mknod", err)
	}
	node, err := n.R.newFsNode(ctx, n, newCh)
	if err != nil {
		return nil, Error2FuseSysError("entry_mknod", err)
	}
	updateAttrOut(nanaNode2Stat(newCh), &out.Attr)
	n.AddChild(name, node.EmbeddedInode(), true)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Link").End()
	defer logOperationLatency("entry_link", time.Now())
	targetNode, ok := target.(*NanaNode)
	if !ok {
		return nil, syscall.EIO
	}

	newEntry, err := n.R.MirrorEntry(ctx, targetNode.entryID, n.entryID, types.EntryAttr{Name: name})
	if err != nil {
		return nil, Error2FuseSysError("entry_link", err)
	}

	updateAttrOut(nanaNode2Stat(newEntry), &out.Attr)
	n.AddChild(name, target.EmbeddedInode(), true)
	return target.EmbeddedInode(), NoErr
}

// TODO: improve symlink operation
func (n *NanaNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Symlink").End()
	defer logOperationLatency("entry_symlink", time.Now())
	exist, err := n.R.FindEntry(ctx, n.entryID, name)
	if err != nil {
		if err != types.ErrNotFound {
			return nil, Error2FuseSysError("entry_symlink", err)
		}
	}
	if exist != nil {
		return nil, Error2FuseSysError("entry_symlink", types.ErrIsExist)
	}
	newLink, err := n.R.CreateEntry(ctx, n.entryID, types.EntryAttr{
		Name: name,
		Kind: types.SymLinkKind,
		Dev:  int64(MountDev),
	})
	if err != nil {
		return nil, Error2FuseSysError("entry_symlink", err)
	}

	n.logger.Debugw("create new symlink", "target", target)
	f, err := n.R.OpenFile(ctx, newLink.ID, types.OpenAttr{Write: true, Create: true, Trunc: true})
	if err != nil {
		return nil, Error2FuseSysError("entry_symlink", err)
	}

	_, err = n.R.WriteFile(ctx, f, []byte(target), 0)
	if err != nil {
		return nil, Error2FuseSysError("entry_symlink", err)
	}
	if err = f.Close(ctx); err != nil {
		return nil, Error2FuseSysError("entry_symlink", err)
	}

	newNode, err := n.R.newFsNode(ctx, n, newLink)
	if err != nil {
		return nil, Error2FuseSysError("entry_symlink", err)
	}
	st := nanaNode2Stat(newLink)
	updateAttrOut(st, &out.Attr)
	return newNode.EmbeddedInode(), NoErr
}

func (n *NanaNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Readlink").End()
	defer logOperationLatency("entry_read_link", time.Now())
	entry, err := n.R.GetEntry(ctx, n.entryID)
	if err != nil {
		return nil, Error2FuseSysError("entry_read_link", err)
	}
	f, err := n.R.OpenFile(ctx, n.entryID, types.OpenAttr{Read: true})
	if err != nil {
		return nil, Error2FuseSysError("entry_read_link", err)
	}
	defer n.R.CloseFile(ctx, f)

	buf := make([]byte, entry.Size)
	_, err = n.R.ReadFile(ctx, f, buf, 0)
	if err != nil {
		return nil, Error2FuseSysError("entry_read_link", err)
	}
	return buf, NoErr
}

func (n *NanaNode) Unlink(ctx context.Context, name string) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Unlink").End()
	defer logOperationLatency("entry_unlink", time.Now())
	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	ch, err := n.R.FindEntry(ctx, n.entryID, name)
	if err != nil {
		return Error2FuseSysError("entry_unlink", err)
	}

	if err = n.R.DestroyEntry(ctx, n.entryID, ch.ID, types.DestroyObjectAttr{Uid: int64(uid), Gid: int64(gid)}); err != nil {
		return Error2FuseSysError("entry_unlink", err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Rmdir").End()
	defer logOperationLatency("entry_rmdir", time.Now())
	if name == ".." {
		return Error2FuseSysError("entry_rmdir", types.ErrIsExist)
	}

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	ch, err := n.R.FindEntry(ctx, n.entryID, name)
	if err != nil {
		return Error2FuseSysError("entry_rmdir", err)
	}
	if !types.IsGroup(ch.Kind) {
		return Error2FuseSysError("entry_rmdir", types.ErrNoGroup)
	}

	children, err := n.R.ListEntryChildren(ctx, ch.ID)
	if err != nil {
		return Error2FuseSysError("entry_rmdir", err)
	}
	if len(children) > 0 {
		return Error2FuseSysError("entry_rmdir", types.ErrNotEmpty)
	}

	if err = n.R.DestroyEntry(ctx, n.entryID, ch.ID, types.DestroyObjectAttr{Uid: int64(uid), Gid: int64(gid)}); err != nil {
		return Error2FuseSysError("entry_rmdir", err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Rename").End()
	defer logOperationLatency("entry_rename", time.Now())
	newNode, ok := newParent.(*NanaNode)
	if !ok {
		return syscall.EIO
	}

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}
	opt := types.ChangeParentAttr{Uid: int64(uid), Gid: int64(gid), Replace: true}
	if flags&RenameExchange > 0 {
		opt.Exchange = true
	}
	if flags&RenameNoreplace > 0 {
		opt.Replace = false
	}

	oldEntry, err := n.R.FindEntry(ctx, n.entryID, name)
	if err != nil {
		return Error2FuseSysError("entry_rename", err)
	}
	newParentEntry, err := n.R.GetEntry(ctx, newNode.entryID)
	if err != nil {
		return Error2FuseSysError("entry_rename", err)
	}

	if err = n.R.ChangeEntryParent(ctx, oldEntry.ID, n.entryID, newParentEntry.ID, newName, opt); err != nil {
		return Error2FuseSysError("entry_rename", err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) OnAdd(ctx context.Context) {
	defer trace.StartRegion(ctx, "fs.node.OnAdd").End()
}

func (n *NanaNode) Release(ctx context.Context, f fs.FileHandle) (err syscall.Errno) {
	defer trace.StartRegion(ctx, "fs.node.Release").End()
	closer, ok := f.(fs.FileReleaser)
	if ok {
		err = closer.Release(ctx)
	}

	if err != NoErr {
		return err
	}
	return NoErr
}

func (n *NanaNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defer trace.StartRegion(ctx, "fs.node.Statfs").End()
	defer logOperationLatency("entry_statfs", time.Now())
	info := n.R.FsInfo(ctx)
	fsInfo2StatFs(info, out)
	return NoErr
}
