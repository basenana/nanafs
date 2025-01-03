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
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("TestAccess", func() {
	var (
		node *NanaNode
		root *NanaNode
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		entry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   "file.txt",
			Kind:   types.RawKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		node, err = nfs.newFsNode(context.Background(), root, entry)
		Expect(err).Should(BeNil())
		root.AddChild(entry.Name, node.EmbeddedInode(), false)
	})

	Describe("", func() {
		Context("access root dir", func() {
			It("should be ok", func() {
				Expect(root.Access(context.Background(), 0)).To(Equal(syscall.Errno(0)))
			})
		})
		Context("access a file", func() {
			It("should be ok", func() {
				Expect(node.Access(context.Background(), 0)).To(Equal(syscall.Errno(0)))
			})
		})
	})
})

var _ = Describe("TestGetattr", func() {
	var (
		node *NanaNode
		root *NanaNode
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		entry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   "file.txt",
			Kind:   types.RawKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		node, err = nfs.newFsNode(context.Background(), root, entry)
		Expect(err).Should(BeNil())
		root.AddChild(entry.Name, node.EmbeddedInode(), false)
	})

	Describe("", func() {
		Context("get a file attr", func() {
			It("should be ok", func() {
				out := &fuse.AttrOut{}
				Expect(node.Getattr(context.Background(), nil, out)).To(Equal(syscall.Errno(0)))
			})
		})
	})
})

var _ = Describe("TestOpen", func() {
	var (
		node *NanaNode
		root *NanaNode
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		entry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   "file.txt",
			Kind:   types.RawKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		node, err = nfs.newFsNode(context.Background(), root, entry)
		Expect(err).Should(BeNil())
		root.AddChild(entry.Name, node.EmbeddedInode(), false)
	})

	Describe("", func() {
		Context("open a file", func() {
			It("should be ok", func() {
				_, _, err := node.Open(context.Background(), uint32(os.O_RDWR))
				Expect(err).To(Equal(syscall.Errno(0)))
			})
		})
		Context("open a dir", func() {
			It("should be failed", func() {
				_, _, err := root.Open(context.Background(), uint32(os.O_RDWR))
				Expect(err).To(Equal(syscall.EISDIR))
			})
		})
	})
})

var _ = Describe("TestCreate", func() {
	var (
		root        *NanaNode
		ctl         controller.Controller
		newFileName = "file.txt"
	)

	BeforeEach(func() {
		ctl = NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)
	})

	Describe("", func() {
		It("create file", func() {
			Context("create new file", func() {
				out := &fuse.EntryOut{}
				inode, _, _, errNo := root.Create(context.Background(), newFileName, 0, 0755, out)
				Expect(errNo).To(Equal(syscall.Errno(0)))
				root.AddChild(newFileName, inode, false)
				children, err := ctl.ListEntryChildren(context.Background(), mustGetNanaEntry(root, ctl).ID, nil, types.Filter{})
				Expect(err).To(BeNil())

				found := false
				for _, ch := range children {
					if ch.Name == newFileName {
						found = true
					}
				}
				Expect(found).To(BeTrue())
			})
			When("when file already existed", func() {
				Context("create dup file", func() {
					out := &fuse.EntryOut{}
					_, _, _, err := root.Create(context.Background(), "file.txt", 0, 0755, out)
					Expect(err).To(Equal(syscall.EEXIST))
				})
			})
		})
	})
})

var _ = Describe("TestLookup", func() {
	var (
		root     *NanaNode
		node     *NanaNode
		fileName = "file.txt"
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		entry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   fileName,
			Kind:   types.RawKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		node, err = nfs.newFsNode(context.Background(), root, entry)
		Expect(err).Should(BeNil())
		root.AddChild(entry.Name, node.EmbeddedInode(), false)
	})

	Describe("", func() {
		Context("lookup a file", func() {
			It("should be ok", func() {
				out := &fuse.EntryOut{}
				_, err := root.Lookup(context.Background(), fileName, out)
				Expect(err).To(Equal(syscall.Errno(0)))
			})
		})
		Context("lookup a not found file", func() {
			It("should be failed", func() {
				out := &fuse.EntryOut{}
				_, err := root.Lookup(context.Background(), "nofile.txt", out)
				Expect(err).To(Equal(syscall.ENOENT))
			})
		})
	})
})

var _ = Describe("TestOpendir", func() {
	var (
		fileNode *NanaNode
		dirNode  *NanaNode
		root     *NanaNode
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		fileEntry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   "file.txt",
			Kind:   types.RawKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		dirEntry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   "dir",
			Kind:   types.GroupKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		fileNode, err = nfs.newFsNode(context.Background(), root, fileEntry)
		Expect(err).Should(BeNil())

		dirNode, err = nfs.newFsNode(context.Background(), root, dirEntry)
		Expect(err).Should(BeNil())

		root.AddChild(fileEntry.Name, fileNode.EmbeddedInode(), false)
		root.AddChild(dirEntry.Name, dirNode.EmbeddedInode(), false)
	})

	Describe("", func() {
		Context("open a dir", func() {
			It("should be ok", func() {
				Expect(dirNode.Opendir(context.Background())).To(Equal(syscall.Errno(0)))
			})
		})
		Context("open a file", func() {
			It("should be failed", func() {
				Expect(fileNode.Opendir(context.Background())).To(Equal(syscall.EISDIR))
			})
		})
	})
})

var _ = Describe("TestReaddir", func() {
	var (
		node *NanaNode
		root *NanaNode
		nfs  *NanaFS
		ctl  controller.Controller
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl = NewMockController()
		nfs = &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		entry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   "files",
			Kind:   types.GroupKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		node, err = nfs.newFsNode(context.Background(), root, entry)
		Expect(err).Should(BeNil())
		root.AddChild(entry.Name, node.EmbeddedInode(), false)
	})

	Describe("", func() {
		addFileName := "file.txt"
		It("normal file test", func() {
			Context("read empty dir", func() {
				ds, err := node.Readdir(context.Background())
				Expect(err).To(Equal(syscall.Errno(0)))
				Expect(ds.HasNext()).To(BeFalse())
				ds.Close()
			})
			Context("add file to dir", func() {
				newEntry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(node, ctl).ID, types.EntryAttr{
					Name:   addFileName,
					Kind:   types.RawKind,
					Access: acc,
				})
				Expect(err).Should(BeNil())

				var newNode *NanaNode
				newNode, err = nfs.newFsNode(context.Background(), node, newEntry)
				Expect(err).Should(BeNil())
				node.AddChild(addFileName, newNode.EmbeddedInode(), false)
			})
			Context("read dir", func() {
				ds, err := node.Readdir(context.Background())
				Expect(err).To(Equal(syscall.Errno(0)))

				Expect(ds.HasNext()).To(BeTrue())
				newFile, err := ds.Next()
				Expect(err).To(Equal(syscall.Errno(0)))
				Expect(newFile.Name).Should(Equal(addFileName))
				Expect(ds.HasNext()).To(BeFalse())
			})
		})
	})
})

var _ = Describe("TestMkdir", func() {
	var (
		root *NanaNode
	)

	BeforeEach(func() {
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)
	})

	Describe("", func() {
		var (
			dirName = "files"
			out     *fuse.EntryOut
			err     error
		)
		It("test make dir dup", func() {
			Context("make a dir", func() {
				out = &fuse.EntryOut{}

				var newDir *fs.Inode
				newDir, err = root.Mkdir(context.Background(), dirName, 0, out)
				Expect(err).To(Equal(syscall.Errno(0)))
				root.AddChild(dirName, newDir, false)
			})
			When("dir already existed", func() {
				Context("make a dir again", func() {
					out = &fuse.EntryOut{}
					_, err = root.Mkdir(context.Background(), dirName, 0, out)
					Expect(err).To(Equal(syscall.EEXIST))
				})
			})
		})
	})
})

var _ = Describe("TestMknod", func() {
	var (
		root *NanaNode
	)

	BeforeEach(func() {
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)
	})

	Describe("", func() {
		Context("mknode a new file", func() {
			It("should be ok", func() {
				out := &fuse.EntryOut{}
				_, err := root.Mknod(context.Background(), "file.txt", 0, 0, out)
				Expect(err).To(Equal(syscall.Errno(0)))
			})
		})
	})
})

var _ = Describe("TestLink", func() {
	var (
		node *NanaNode
		root *NanaNode
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		entry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   "file.txt",
			Kind:   types.RawKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		node, err = nfs.newFsNode(context.Background(), root, entry)
		Expect(err).Should(BeNil())
		root.AddChild(entry.Name, node.EmbeddedInode(), false)
	})

	Describe("", func() {
		Context("link new file", func() {
			It("should be ok", func() {
			})
		})
		Context("unlink file", func() {
			It("should be ok", func() {
			})
		})
	})
})

var _ = Describe("TestRmdir", func() {
	var (
		root    *NanaNode
		node    *NanaNode
		dirName = "files"
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		entry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   dirName,
			Kind:   types.GroupKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		node, err = nfs.newFsNode(context.Background(), root, entry)
		Expect(err).Should(BeNil())
		root.AddChild(entry.Name, node.EmbeddedInode(), false)
	})

	Describe("", func() {
		It("test remove", func() {
			Describe("remove a dir", func() {
				Expect(root.Rmdir(context.Background(), dirName)).To(Equal(syscall.Errno(0)))
				root.RmChild(dirName)
			})
			When("dir removed", func() {
				Describe("remove a dir again", func() {
					Expect(root.Rmdir(context.Background(), dirName)).To(Equal(syscall.ENOENT))
				})

				Describe("can not see old dir", func() {
					ds, err := root.Readdir(context.Background())
					Expect(err).To(Equal(syscall.Errno(0)))
					found := false
					for ds.HasNext() {
						ch, err := ds.Next()
						Expect(err).To(Equal(syscall.Errno(0)))
						if ch.Name == dirName {
							found = true
						}
					}
					Expect(found).To(BeFalse())
					ds.Close()
				})
			})
		})
	})
})

var _ = Describe("TestRename", func() {
	var (
		node *NanaNode
		root *NanaNode
	)

	acc := &types.Access{}
	dentry.UpdateAccessWithMode(acc, 0655)
	dentry.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

	BeforeEach(func() {
		var err error
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)

		entry, err := nfs.CreateEntry(context.Background(), mustGetNanaEntry(root, ctl).ID, types.EntryAttr{
			Name:   "file.txt",
			Kind:   types.RawKind,
			Access: acc,
		})
		Expect(err).Should(BeNil())

		node, err = nfs.newFsNode(context.Background(), root, entry)
		Expect(err).Should(BeNil())
		root.AddChild(entry.Name, node.EmbeddedInode(), false)
	})

	Describe("", func() {
		Context("remove a file", func() {
			It("should be ok", func() {
			})
		})
	})
})

var _ = Describe("TestStatfs", func() {
	var (
		root *NanaNode
	)

	BeforeEach(func() {
		ctl := NewMockController()
		nfs := &NanaFS{
			Controller: ctl,
			Path:       "/tmp/test",
		}
		root = initFsBridge(nfs)
	})

	Describe("", func() {
		Context("stat fs", func() {
			It("should be ok", func() {
				out := &fuse.StatfsOut{}
				Expect(root.Statfs(context.Background(), out)).To(Equal(syscall.Errno(0)))
			})
		})
	})
})
