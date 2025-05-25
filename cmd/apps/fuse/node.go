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
	"bytes"
	"context"
	"errors"
	"github.com/basenana/nanafs/pkg/core"
	"io"
	"runtime/trace"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.uber.org/zap"

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

	name    string
	entryID int64
	cache   *types.Entry
	logger  *zap.SugaredLogger
}

var _ nodeOperation = &NanaNode{}

func (n *NanaNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Access").End()
	defer logOperationLatency("entry_access", time.Now())

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	en, err := n.R.GetEntry(ctx, n.entryID)
	if err != nil {
		en = n.cache
	}

	return Error2FuseSysError("entry_access", core.IsAccess(en.Access, int64(uid), int64(gid), mask))
}

func (n *NanaNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Getattr").End()
	defer logOperationLatency("entry_get_attr", time.Now())
	file, ok := f.(fs.FileGetattrer)
	if ok {
		return file.Getattr(ctx, out)
	}

	en, err := n.R.GetEntry(ctx, n.entryID)
	if err != nil {
		en = n.cache
		if errors.Is(err, types.ErrNotFound) {
			// An open file will not be immediately freed by unlink
			en.RefCount = 0
		}
	}

	st := nanaNode2Stat(en)
	updateAttrOut(st, &out.Attr)
	return NoErr
}

func (n *NanaNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Setattr").End()
	defer logOperationLatency("entry_set_attr", time.Now())
	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	var attr types.OpenAttr
	nanaFile, ok := f.(*File)
	if ok {
		attr = nanaFile.attr
	}

	en, err := n.R.GetEntry(ctx, n.entryID)
	if err != nil {
		en = n.cache
	}

	update := types.UpdateEntry{}
	if err := updateNanaNodeWithAttr(in, en, &update, int64(uid), int64(gid), attr); err != nil {
		n.logger.Errorw("update nana node with attr failed", "err", err, "name", n.name)
		return Error2FuseSysError("entry_set_attr", err)
	}
	newEN, err := n.R.UpdateEntry(ctx, n.entryID, update)
	if err != nil {
		n.logger.Errorw("update entry with attr failed", "err", err, "name", n.name)
		return Error2FuseSysError("entry_set_attr", err)
	}
	n.cache = newEN
	return n.Getattr(ctx, f, out)
}

func (n *NanaNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Getxattr").End()
	defer logOperationLatency("entry_get_xattr", time.Now())
	data, err := n.R.GetXAttr(ctx, n.entryID, attr)
	if err != nil {
		return 0, ENODATA
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
	defer trace.StartRegion(ctx, "fuse.node.Setxattr").End()
	defer logOperationLatency("entry_set_xattr", time.Now())
	if err := n.R.SetXAttr(ctx, n.entryID, attr, data); err != nil {
		n.logger.Errorw("update entry with xattr failed", "err", err, "name", n.name)
		return Error2FuseSysError("entry_set_xattr", err)
	}
	return NoErr
}

func (n *NanaNode) Removexattr(ctx context.Context, attr string) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Removexattr").End()
	defer logOperationLatency("entry_remove_xattr", time.Now())
	if err := n.R.RemoveXAttr(ctx, n.entryID, attr); err != nil {
		n.logger.Errorw("remove entry xattr failed", "err", err, "name", n.name)
		return syscall.Errno(0x5d)
	}
	return NoErr
}

func (n *NanaNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Open").End()
	defer logOperationLatency("entry_open", time.Now())
	openAttr := openFileAttr(flags)
	file, err := n.R.Open(ctx, n.entryID, openAttr)
	if err != nil {
		n.logger.Errorw("open entry failed", "err", err, "name", n.name)
		return nil, 0, Error2FuseSysError("entry_open", err)
	}
	return &File{node: n, file: file.Raw(), attr: openAttr}, flags, NoErr
}

func (n *NanaNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Create").End()
	defer logOperationLatency("entry_create", time.Now())

	ch, err := n.R.LookUpEntry(ctx, n.entryID, name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		n.logger.Errorw("lookup for create entry failed", "err", err, "name", name)
		return nil, nil, 0, Error2FuseSysError("entry_create", err)
	}
	if ch != nil && flags|syscall.O_EXCL > 0 {
		return nil, nil, 0, syscall.EEXIST
	}

	acc := &types.Access{}
	core.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		core.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}
	newCh, err := n.R.CreateEntry(ctx, n.entryID, types.EntryAttr{
		Name:   name,
		Kind:   fileKindFromMode(mode),
		Access: acc,
		Dev:    int64(MountDev),
	})
	if err != nil {
		n.logger.Errorw("create new entry failed", "err", err, "name", name)
		return nil, nil, 0, Error2FuseSysError("entry_create", err)
	}

	node := n.R.newFsNode(name, newCh)
	updateAttrOut(nanaNode2Stat(newCh), &out.Attr)

	openAttr := openFileAttr(flags)
	f, err := n.R.Open(ctx, newCh.ID, openAttr)
	if err != nil {
		n.logger.Errorw("open new entry failed", "err", err, "entry", newCh.ID, "name", name)
		return nil, nil, 0, Error2FuseSysError("entry_create", err)
	}
	return n.NewInode(ctx, node, fs.StableAttr{Ino: uint64(node.entryID), Mode: out.Mode}), &File{node: node, file: f.Raw(), attr: openAttr}, core.Access2Mode(newCh.Access), NoErr
}

func (n *NanaNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Lookup").End()
	defer logOperationLatency("entry_lookup", time.Now())

	ch, err := n.R.LookUpEntry(ctx, n.entryID, name)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			n.logger.Errorw("lookup entry failed", "err", err, "name", name)
		}
		return nil, Error2FuseSysError("entry_lookup", err)
	}

	node := n.R.newFsNode(name, ch)
	updateAttrOut(nanaNode2Stat(ch), &out.Attr)
	return n.NewInode(ctx, node, fs.StableAttr{Ino: uint64(node.entryID), Mode: out.Mode}), NoErr
}

func (n *NanaNode) Opendir(ctx context.Context) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Opendir").End()
	defer logOperationLatency("entry_open_dir", time.Now())
	if n.cache.IsGroup {
		return NoErr
	}
	return syscall.EISDIR
}

func (n *NanaNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Readdir").End()
	defer logOperationLatency("entry_read_dir", time.Now())
	result := make([]fuse.DirEntry, 0)
	group, err := n.R.OpenDir(ctx, n.entryID)
	if err != nil {
		n.logger.Errorw("open entry dir failed", "err", err, "name", n.name)
		return nil, Error2FuseSysError("entry_read_dir", types.ErrNoGroup)
	}

	for {
		children, err := group.Readdir(-1)
		if errors.Is(err, io.EOF) {
			break
		}

		for i := range children {
			ch := children[i]
			result = append(result, fuse.DirEntry{
				Mode: uint32(ch.Mode()),
				Name: ch.Name(),
				Ino:  uint64(ch.ID()),
			})
		}
	}

	return fs.NewListDirStream(result), NoErr
}

func (n *NanaNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Mkdir").End()
	defer logOperationLatency("entry_mkdir", time.Now())
	ch, err := n.R.LookUpEntry(ctx, n.entryID, name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		n.logger.Errorw("lookup for create new gourp entry failed", "err", err, "name", name)
		return nil, Error2FuseSysError("entry_mkdir", err)
	}
	if ch != nil {
		return nil, syscall.EEXIST
	}
	acc := &types.Access{}
	core.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		core.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}

	n.logger.Debugw("create new group entry", "name", name)
	newDir, err := n.R.CreateEntry(ctx, n.entryID, types.EntryAttr{
		Name:   name,
		Kind:   types.GroupKind,
		Access: acc,
		Dev:    int64(MountDev),
	})
	if err != nil {
		n.logger.Errorw("create new group entry failed", "err", err, "name", name)
		return nil, Error2FuseSysError("entry_mkdir", err)
	}

	node := n.R.newFsNode(name, newDir)
	updateAttrOut(nanaNode2Stat(newDir), &out.Attr)
	return n.NewInode(ctx, node, fs.StableAttr{Ino: uint64(node.entryID), Mode: out.Mode}), NoErr
}

func (n *NanaNode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Mknod").End()
	defer logOperationLatency("entry_mknod", time.Now())
	ch, err := n.R.LookUpEntry(ctx, n.entryID, name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		n.logger.Errorw("lookup for create new device entry failed", "err", err, "name", n.name)
		return nil, Error2FuseSysError("entry_mknod", err)
	}
	if ch != nil {
		return nil, syscall.EEXIST
	}

	acc := &types.Access{}
	core.UpdateAccessWithMode(acc, mode)
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		core.UpdateAccessWithOwnID(acc, int64(fuseCtx.Uid), int64(fuseCtx.Gid))
	}
	n.logger.Debugw("create new device entry", "name", name)
	newCh, err := n.R.CreateEntry(ctx, n.entryID, types.EntryAttr{
		Name:   name,
		Kind:   fileKindFromMode(mode),
		Access: acc,
		Dev:    int64(dev),
	})
	if err != nil {
		n.logger.Errorw("create new device entry failed", "err", err, "name", name)
		return nil, Error2FuseSysError("entry_mknod", err)
	}

	node := n.R.newFsNode(name, newCh)
	updateAttrOut(nanaNode2Stat(newCh), &out.Attr)
	return n.NewInode(ctx, node, fs.StableAttr{Ino: uint64(node.entryID), Mode: out.Mode}), NoErr
}

func (n *NanaNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Link").End()
	defer logOperationLatency("entry_link", time.Now())
	targetNode, ok := target.(*NanaNode)
	if !ok {
		return nil, syscall.EIO
	}

	newEntry, err := n.R.LinkEntry(ctx, targetNode.entryID, n.entryID, types.EntryAttr{Name: name})
	if err != nil {
		n.logger.Errorw("create entry link failed", "err", err, "name", targetNode.name, "newName", name)
		return nil, Error2FuseSysError("entry_link", err)
	}

	targetNode.cache = newEntry
	updateAttrOut(nanaNode2Stat(newEntry), &out.Attr)
	return targetNode.EmbeddedInode(), NoErr
}

// TODO: improve symlink operation
func (n *NanaNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Symlink").End()
	defer logOperationLatency("entry_symlink", time.Now())
	exist, err := n.R.LookUpEntry(ctx, n.entryID, name)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			n.logger.Errorw("lookup for create symlink entry failed", "err", err, "target", target, "name", name)
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
		n.logger.Errorw("create symlink entry failed", "err", err, "target", target, "name", name)
		return nil, Error2FuseSysError("entry_symlink", err)
	}

	n.logger.Debugw("create new symlink", "target", target, "name", name, "newEntry", newLink.ID)
	f, err := n.R.Open(ctx, newLink.ID, types.OpenAttr{Write: true, Create: true, Trunc: true})
	if err != nil {
		n.logger.Errorw("open new symlink entry failed", "err", err, "target", target, "name", name)
		return nil, Error2FuseSysError("entry_symlink", err)
	}

	_, err = f.Write([]byte(target))
	if err != nil {
		n.logger.Errorw("write symlink entry failed", "err", err, "target", target, "name", name)
		return nil, Error2FuseSysError("entry_symlink", err)
	}
	if err = f.Close(); err != nil {
		n.logger.Errorw("save symlink entry failed", "err", err, "target", target, "name", name)
		return nil, Error2FuseSysError("entry_symlink", err)
	}

	newNode := n.R.newFsNode(name, newLink)
	st := nanaNode2Stat(newLink)
	updateAttrOut(st, &out.Attr)
	return n.NewInode(ctx, newNode, fs.StableAttr{Ino: uint64(newNode.entryID), Mode: out.Mode}), NoErr
}

func (n *NanaNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Readlink").End()
	defer logOperationLatency("entry_read_link", time.Now())
	f, err := n.R.Open(ctx, n.entryID, types.OpenAttr{Read: true})
	if err != nil {
		n.logger.Errorw("open symlink entry failed", "err", err, "name", n.name)
		return nil, Error2FuseSysError("entry_read_link", err)
	}
	defer f.Close()

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, Error2FuseSysError("entry_read_link", err)
	}
	return buf.Bytes(), NoErr
}

func (n *NanaNode) Unlink(ctx context.Context, name string) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Unlink").End()
	defer logOperationLatency("entry_unlink", time.Now())
	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	if err := n.R.UnlinkEntry(ctx, n.entryID, name, types.DestroyEntryAttr{Uid: int64(uid), Gid: int64(gid)}); err != nil {
		n.logger.Errorw("unlink entry failed", "err", err, "name", name)
		return Error2FuseSysError("entry_unlink", err)
	}

	en, err := n.R.GetEntry(ctx, n.entryID)
	if err == nil {
		n.cache = en
	}
	return NoErr
}

func (n *NanaNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Rmdir").End()
	defer logOperationLatency("entry_rmdir", time.Now())
	if name == ".." {
		return Error2FuseSysError("entry_rmdir", types.ErrIsExist)
	}

	var uid, gid uint32
	if fuseCtx, ok := ctx.(*fuse.Context); ok {
		uid, gid = fuseCtx.Uid, fuseCtx.Gid
	}

	if err := n.R.RmGroup(ctx, n.entryID, name, types.DestroyEntryAttr{Uid: int64(uid), Gid: int64(gid)}); err != nil {
		n.logger.Errorw("remove group entry failed", "err", err, "name", name)
		return Error2FuseSysError("entry_rmdir", err)
	}
	n.RmChild(name)
	return NoErr
}

func (n *NanaNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Rename").End()
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

	oldEntry, err := n.R.LookUpEntry(ctx, n.entryID, name)
	if err != nil {
		n.logger.Errorw("lookup for rename entry failed", "err", err, "name", name)
		return Error2FuseSysError("entry_rename", err)
	}

	if err = n.R.Rename(ctx, oldEntry.ID, n.entryID, newNode.entryID, name, newName, opt); err != nil {
		n.logger.Errorw("rename entry failed", "err", err, "newParent", newNode.entryID, "name", name, "newName", newName)
		return Error2FuseSysError("entry_rename", err)
	}
	return NoErr
}

func (n *NanaNode) OnAdd(ctx context.Context) {
	defer trace.StartRegion(ctx, "fuse.node.OnAdd").End()
}

func (n *NanaNode) Release(ctx context.Context, f fs.FileHandle) (err syscall.Errno) {
	defer trace.StartRegion(ctx, "fuse.node.Release").End()
	closer, ok := f.(fs.FileReleaser)
	if ok {
		err = closer.Release(ctx)
	}

	if !errors.Is(err, NoErr) {
		return err
	}
	return NoErr
}

func (n *NanaNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defer trace.StartRegion(ctx, "fuse.node.Statfs").End()
	defer logOperationLatency("entry_statfs", time.Now())
	info := n.R.FsInfo(ctx)
	fsInfo2StatFs(info, out)
	return NoErr
}
