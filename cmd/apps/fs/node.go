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
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

type NanaNode struct {
	fs.Inode
	oid    int64
	R      *NanaFS
	logger *zap.SugaredLogger
}

var _ nodeOperation = &NanaNode{}

func (n *NanaNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.access")()

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}

	return Error2FuseSysError(dentry.IsAccess(entry.Metadata().Access, int64(uid), int64(gid), mask))
}

func (n *NanaNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.getattr")()
	file, ok := f.(fs.FileGetattrer)
	if ok {
		return file.Getattr(ctx, out)
	}

	entry, err := n.R.GetSourceEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}

	st := nanaNode2Stat(entry)
	updateAttrOut(st, &out.Attr)
	return NoErr
}

func (n *NanaNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.setattr")()
	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	var attr dentry.Attr
	nanaFile, ok := f.(*File)
	if ok {
		attr = nanaFile.file.GetAttr()
	}

	entry, err := n.R.GetSourceEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}
	if err := updateNanaNodeWithAttr(in, entry.Metadata(), int64(uid), int64(gid), attr); err != nil {
		return Error2FuseSysError(err)
	}

	entry.Metadata().ChangedAt = time.Now()
	if err = n.R.SaveEntry(ctx, nil, entry); err != nil {
		return Error2FuseSysError(err)
	}
	return n.Getattr(ctx, f, out)
}

func (n *NanaNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.getxattr")()

	entry, err := n.R.GetSourceEntry(ctx, n.oid)
	if err != nil {
		return 0, Error2FuseSysError(err)
	}

	encodedData := entry.GetExtendField(attr)
	if encodedData == nil {
		return 0, syscall.Errno(0x5d)
	}
	raw, err := xattrContent2RawData(*encodedData)
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
	entry, err := n.R.GetSourceEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}
	entry.SetExtendField(attr, xattrRawData2Content(data))
	entry.Metadata().ChangedAt = time.Now()
	return Error2FuseSysError(n.R.SaveEntry(ctx, nil, entry))
}

func (n *NanaNode) Removexattr(ctx context.Context, attr string) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.removexattr")()
	entry, err := n.R.GetSourceEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}
	if err = entry.RemoveExtendField(attr); err != nil {
		return syscall.Errno(0x5d)
	}
	entry.Metadata().ChangedAt = time.Now()
	return Error2FuseSysError(n.R.SaveEntry(ctx, nil, entry))
}

func (n *NanaNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.open")()
	entry, err := n.R.GetSourceEntry(ctx, n.oid)
	if err != nil {
		return nil, 0, Error2FuseSysError(err)
	}
	if entry.IsGroup() {
		return nil, 0, Error2FuseSysError(types.ErrIsGroup)
	}
	f, err := n.R.Controller.OpenFile(ctx, entry, openFileAttr(flags))
	return &File{node: n, entry: entry, file: f}, flags, Error2FuseSysError(err)
}

func (n *NanaNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.create")()
	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return nil, nil, 0, Error2FuseSysError(err)
	}

	ch, err := n.R.FindEntry(ctx, entry, name)
	if err != nil && err != types.ErrNotFound {
		return nil, nil, 0, Error2FuseSysError(err)
	}
	if ch != nil && flags|syscall.O_EXCL > 0 {
		return nil, nil, 0, syscall.EEXIST
	}

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		dentry.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}
	newCh, err := n.R.CreateEntry(ctx, entry, types.ObjectAttr{
		Name:   name,
		Kind:   fileKindFromMode(mode),
		Access: *acc,
	})
	if err != nil {
		return nil, nil, 0, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, newCh)
	if err != nil {
		return nil, nil, 0, Error2FuseSysError(err)
	}
	updateAttrOut(nanaNode2Stat(newCh), &out.Attr)
	n.AddChild(name, node.EmbeddedInode(), true)

	f, err := n.R.Controller.OpenFile(ctx, newCh, openFileAttr(flags))
	return node.EmbeddedInode(), &File{node: node, entry: entry, file: f}, dentry.Access2Mode(newCh.Metadata().Access), Error2FuseSysError(err)
}

func (n *NanaNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.lookup")()
	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	ch, err := n.R.FindEntry(ctx, entry, name)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	ch, err = n.R.GetSourceEntry(ctx, ch.Metadata().ID)
	if err != nil {
		n.logger.Errorw("query source entry failed", "", "err", err.Error())
		return nil, Error2FuseSysError(err)
	}

	node, err := n.R.newFsNode(ctx, n, ch)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	updateAttrOut(nanaNode2Stat(ch), &out.Attr)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Opendir(ctx context.Context) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.opendir")()
	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}
	if entry.IsGroup() {
		return NoErr
	}
	return syscall.EISDIR
}

func (n *NanaNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.readdir")()
	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	if !entry.IsGroup() {
		return nil, Error2FuseSysError(types.ErrNoGroup)
	}
	result := make([]fuse.DirEntry, 0)
	children, err := n.R.ListEntryChildren(ctx, entry)
	if err != nil {
		return nil, Error2FuseSysError(types.ErrNoGroup)
	}

	for i := range children {
		ch := children[i]
		node, _ := n.R.newFsNode(ctx, n, ch)
		n.AddChild(ch.Metadata().Name, node.EmbeddedInode(), false)

		result = append(result, fuse.DirEntry{
			Mode: node.Mode(),
			Name: ch.Metadata().Name,
			Ino:  node.StableAttr().Ino,
		})
	}
	return fs.NewListDirStream(result), NoErr
}

func (n *NanaNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.mkdir")()
	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	ch, err := n.R.FindEntry(ctx, entry, name)
	if err != nil && err != types.ErrNotFound {
		return nil, Error2FuseSysError(err)
	}
	if ch != nil {
		return nil, syscall.EEXIST
	}
	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		dentry.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}
	newDir, err := n.R.CreateEntry(ctx, entry, types.ObjectAttr{
		Name:   name,
		Kind:   fileKindFromMode(mode),
		Access: *acc,
	})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	node, err := n.R.newFsNode(ctx, n, newDir)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	updateAttrOut(nanaNode2Stat(newDir), &out.Attr)
	n.AddChild(name, node.EmbeddedInode(), true)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.mknod")()
	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	ch, err := n.R.FindEntry(ctx, entry, name)
	if err != nil && err != types.ErrNotFound {
		return nil, Error2FuseSysError(err)
	}
	if ch != nil {
		return nil, syscall.EEXIST
	}

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		dentry.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}
	newCh, err := n.R.CreateEntry(ctx, entry, types.ObjectAttr{
		Name:   name,
		Dev:    int64(dev),
		Kind:   fileKindFromMode(mode),
		Access: *acc,
	})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	node, err := n.R.newFsNode(ctx, n, newCh)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	updateAttrOut(nanaNode2Stat(newCh), &out.Attr)
	n.AddChild(name, node.EmbeddedInode(), true)
	return node.EmbeddedInode(), NoErr
}

func (n *NanaNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer utils.TraceRegion(ctx, "node.link")()
	targetNode, ok := target.(*NanaNode)
	if !ok {
		return nil, syscall.EIO
	}

	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	targetEntry, err := n.R.GetSourceEntry(ctx, targetNode.oid)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	_, err = n.R.MirrorEntry(ctx, targetEntry, entry, types.ObjectAttr{Name: name})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	updateAttrOut(nanaNode2Stat(targetEntry), &out.Attr)
	n.AddChild(name, target.EmbeddedInode(), true)
	return target.EmbeddedInode(), NoErr
}

// TODO: improve symlink operation
func (n *NanaNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	exist, err := n.R.FindEntry(ctx, entry, name)
	if err != nil {
		if err != types.ErrNotFound {
			return nil, Error2FuseSysError(err)
		}
	}
	if exist != nil {
		return nil, Error2FuseSysError(types.ErrIsExist)
	}
	newLink, err := n.R.CreateEntry(ctx, entry, types.ObjectAttr{
		Name:   name,
		Kind:   types.SymLinkKind,
		Access: entry.Metadata().Access,
	})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	n.logger.Debugw("create new symlink", "target", target)
	f, err := n.R.OpenFile(ctx, newLink, dentry.Attr{Write: true, Create: true, Trunc: true})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}

	_, err = n.R.WriteFile(ctx, f, []byte(target), 0)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	if err = f.Close(ctx); err != nil {
		return nil, Error2FuseSysError(err)
	}
	if err = n.R.SaveEntry(ctx, entry, newLink); err != nil {
		return nil, Error2FuseSysError(err)
	}

	newNode, err := n.R.newFsNode(ctx, n, newLink)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	st := nanaNode2Stat(newLink)
	updateAttrOut(st, &out.Attr)
	return newNode.EmbeddedInode(), NoErr
}

func (n *NanaNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	f, err := n.R.OpenFile(ctx, entry, dentry.Attr{Read: true})
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	defer n.R.CloseFile(ctx, f)

	buf := make([]byte, entry.Metadata().Size)
	_, err = n.R.ReadFile(ctx, f, buf, 0)
	if err != nil {
		return nil, Error2FuseSysError(err)
	}
	return buf, NoErr
}

func (n *NanaNode) Unlink(ctx context.Context, name string) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.unlink")()

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}

	ch, err := n.R.FindEntry(ctx, entry, name)
	if err != nil {
		return Error2FuseSysError(err)
	}

	if err = n.R.DestroyEntry(ctx, entry, ch, types.DestroyObjectAttr{Uid: int64(uid), Gid: int64(gid)}); err != nil {
		return Error2FuseSysError(err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.rmdir")()

	if name == ".." {
		return Error2FuseSysError(types.ErrIsExist)
	}

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}

	ch, err := n.R.FindEntry(ctx, entry, name)
	if err != nil {
		return Error2FuseSysError(err)
	}
	if !ch.IsGroup() {
		return Error2FuseSysError(types.ErrNoGroup)
	}

	children, err := n.R.ListEntryChildren(ctx, ch)
	if err != nil {
		return Error2FuseSysError(err)
	}
	if len(children) > 0 {
		return Error2FuseSysError(types.ErrNotEmpty)
	}

	if err = n.R.DestroyEntry(ctx, entry, ch, types.DestroyObjectAttr{Uid: int64(uid), Gid: int64(gid)}); err != nil {
		return Error2FuseSysError(err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.rmname")()
	newNode, ok := newParent.(*NanaNode)
	if !ok {
		return syscall.EIO
	}

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}
	opt := types.ChangeParentAttr{Uid: int64(uid), Gid: int64(gid), Replace: true}
	if flags&RENAME_EXCHANGE > 0 {
		opt.Exchange = true
	}
	if flags&RENAME_NOREPLACE > 0 {
		opt.Replace = false
	}

	entry, err := n.R.GetEntry(ctx, n.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}
	oldEntry, err := n.R.FindEntry(ctx, entry, name)
	if err != nil {
		return Error2FuseSysError(err)
	}
	newParentEntry, err := n.R.GetEntry(ctx, newNode.oid)
	if err != nil {
		return Error2FuseSysError(err)
	}

	if err = n.R.ChangeEntryParent(ctx, oldEntry, entry, newParentEntry, newName, opt); err != nil {
		return Error2FuseSysError(err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) OnAdd(ctx context.Context) {
	defer utils.TraceRegion(ctx, "node.onadd")()
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
	return NoErr
}

func (n *NanaNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defer utils.TraceRegion(ctx, "node.statfs")()
	info := n.R.FsInfo(ctx)
	fsInfo2StatFs(info, out)
	return NoErr
}
