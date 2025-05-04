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
	"context"
	"github.com/basenana/nanafs/pkg/core"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("TestAccess", func() {
	var (
		ctx  = context.TODO()
		node *NanaNode
	)

	Describe("", func() {
		Context("create a file", func() {
			It("should be ok", func() {
				acc := &types.Access{}
				core.UpdateAccessWithMode(acc, 0655)
				core.UpdateAccessWithOwnID(acc, cfg.FS.Owner.Uid, cfg.FS.Owner.Gid)

				entry, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name:   "test_access_file.txt",
					Kind:   types.RawKind,
					Access: acc,
				})
				Expect(err).Should(BeNil())
				node = nfs.newFsNode(entry)
			})
		})
		Context("access root dir", func() {
			It("should be ok", func() {
				Expect(root.Access(ctx, 0)).To(Equal(syscall.Errno(0)))
			})
		})
		Context("access a file", func() {
			It("should be ok", func() {
				Expect(node.Access(ctx, 0)).To(Equal(syscall.Errno(0)))
			})
		})
	})
})

var _ = Describe("TestGetattr", func() {
	var (
		ctx  = context.TODO()
		node *NanaNode
	)

	Describe("", func() {
		Context("create a file", func() {
			It("should be ok", func() {
				entry, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: "test_getattr_file.txt",
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
				node = nfs.newFsNode(entry)
			})
		})
		Context("get a file attr", func() {
			It("should be ok", func() {
				out := &fuse.AttrOut{}
				Expect(node.Getattr(ctx, nil, out)).To(Equal(syscall.Errno(0)))
			})
		})
	})
})

var _ = Describe("TestOpen", func() {
	var (
		ctx      = context.TODO()
		fileNode *NanaNode
		dirNode  *NanaNode
	)

	Describe("", func() {
		Context("create a file", func() {
			It("should be ok", func() {
				entry, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: "test_open_file.txt",
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
				fileNode = nfs.newFsNode(entry)
			})
		})
		Context("create a dir", func() {
			It("should be ok", func() {
				entry, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: "test_open_dir",
					Kind: types.GroupKind,
				})
				Expect(err).Should(BeNil())
				dirNode = nfs.newFsNode(entry)
			})
		})
		Context("open a file", func() {
			It("should be ok", func() {
				_, _, err := fileNode.Open(ctx, uint32(os.O_RDWR))
				Expect(err).To(Equal(syscall.Errno(0)))
			})
		})
		Context("open a dir", func() {
			It("should be failed", func() {
				_, _, err := dirNode.Open(ctx, uint32(os.O_RDWR))
				Expect(err).To(Equal(syscall.EISDIR))
			})
		})
	})
})

var _ = Describe("TestCreate", func() {
	var (
		ctx         = context.TODO()
		newFileName = "test_create_file1.txt"
	)

	Describe("", func() {
		It("create file", func() {
			Context("create new file", func() {
				out := &fuse.EntryOut{}
				_, _, _, errNo := root.Create(ctx, newFileName, 0, 0755, out)
				Expect(errNo).To(Equal(syscall.Errno(0)))

				children, err := nfs.ListChildren(ctx, root.entry.ID)
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
					_, _, _, err := root.Create(ctx, newFileName, 0, 0755, out)
					Expect(err).To(Equal(syscall.EEXIST))
				})
			})
		})
	})
})

var _ = Describe("TestLookup", func() {
	var (
		ctx      = context.TODO()
		fileName = "test_lookup_file.txt"
	)

	Describe("", func() {
		Context("create a file", func() {
			It("should be ok", func() {
				_, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: fileName,
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
			})
		})
		Context("lookup a file", func() {
			It("should be ok", func() {
				out := &fuse.EntryOut{}
				_, err := root.Lookup(ctx, fileName, out)
				Expect(err).To(Equal(syscall.Errno(0)))
			})
		})
		Context("lookup a not found file", func() {
			It("should be failed", func() {
				out := &fuse.EntryOut{}
				_, err := root.Lookup(ctx, "nofile.txt", out)
				Expect(err).To(Equal(syscall.ENOENT))
			})
		})
	})
})

var _ = Describe("TestOpendir", func() {
	var (
		ctx      = context.TODO()
		fileNode *NanaNode
		dirNode  *NanaNode
	)

	Describe("", func() {
		Context("create a file", func() {
			It("should be ok", func() {
				fileEntry, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: "test_opendir_file.txt",
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
				fileNode = nfs.newFsNode(fileEntry)
			})
		})
		Context("create a dir", func() {
			It("should be ok", func() {
				dirEntry, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: "test_opendir_dir",
					Kind: types.GroupKind,
				})
				Expect(err).Should(BeNil())
				dirNode = nfs.newFsNode(dirEntry)
			})
		})
		Context("open a dir", func() {
			It("should be ok", func() {
				Expect(dirNode.Opendir(ctx)).To(Equal(syscall.Errno(0)))
			})
		})
		Context("open a file", func() {
			It("should be failed", func() {
				Expect(fileNode.Opendir(ctx)).To(Equal(syscall.EISDIR))
			})
		})
	})
})

var _ = Describe("TestReaddir", func() {
	var (
		ctx         = context.TODO()
		node        *NanaNode
		dirName     = "test_readdir_dir"
		addFileName = "test_readdir_file.txt"
	)

	Describe("", func() {
		Context("create a dir", func() {
			It("should be ok", func() {
				entry, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: dirName,
					Kind: types.GroupKind,
				})
				Expect(err).Should(BeNil())
				node = nfs.newFsNode(entry)
			})
		})
		It("normal file test", func() {
			Context("read empty dir", func() {
				ds, err := node.Readdir(ctx)
				Expect(err).To(Equal(syscall.Errno(0)))
				Expect(ds.HasNext()).To(BeFalse())
				ds.Close()
			})
			Context("add file to dir", func() {
				_, err := nfs.CreateEntry(ctx, node.entry.ID, types.EntryAttr{
					Name: addFileName,
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
			})
			Context("read dir", func() {
				ds, err := node.Readdir(ctx)
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
		ctx     = context.TODO()
		dirName = "test_mkdir_dir"
		out     *fuse.EntryOut
		err     error
	)

	Describe("", func() {
		It("test make dir dup", func() {
			Context("make a dir", func() {
				out = &fuse.EntryOut{}

				_, err = root.Mkdir(ctx, dirName, 0, out)
				Expect(err).To(Equal(syscall.Errno(0)))
			})
			When("dir already existed", func() {
				Context("make a dir again", func() {
					out = &fuse.EntryOut{}
					_, err = root.Mkdir(ctx, dirName, 0, out)
					Expect(err).To(Equal(syscall.EEXIST))
				})
			})
		})
	})
})

var _ = Describe("TestMknod", func() {
	var ctx = context.TODO()
	Describe("", func() {
		Context("mknode a new file", func() {
			It("should be ok", func() {
				out := &fuse.EntryOut{}
				_, err := root.Mknod(ctx, "test_mknod_file.txt", 0, 0, out)
				Expect(err).To(Equal(syscall.Errno(0)))
			})
		})
	})
})

var _ = Describe("TestLink", func() {
	var ctx = context.TODO()
	Describe("", func() {
		Context("create a file", func() {
			It("should be ok", func() {
				_, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: "test_link_file.txt",
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
			})
		})
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
		ctx     = context.TODO()
		dirName = "test_rmdir_dir"
	)

	Describe("", func() {
		Context("create a dir", func() {
			It("should be ok", func() {
				_, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: dirName,
					Kind: types.GroupKind,
				})
				Expect(err).Should(BeNil())
			})
		})
		It("test remove", func() {
			Describe("remove a dir", func() {
				Expect(root.Rmdir(ctx, dirName)).To(Equal(syscall.Errno(0)))
			})
			When("dir removed", func() {
				Describe("remove a dir again", func() {
					Expect(root.Rmdir(ctx, dirName)).To(Equal(syscall.ENOENT))
				})

				Describe("can not see old dir", func() {
					ds, err := root.Readdir(ctx)
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
		ctx         = context.TODO()
		filename    = "test_rename_file.txt"
		filenamenew = "test_rename_file_new.txt"
		dstDir      = "test_rename_dir"
		dirNode     *NanaNode
	)

	Describe("", func() {
		Context("create a file", func() {
			It("should be ok", func() {
				_, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: filename,
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
			})
		})
		Context("create a file", func() {
			It("should be ok", func() {
				dst, err := nfs.CreateEntry(ctx, root.entry.ID, types.EntryAttr{
					Name: dstDir,
					Kind: types.GroupKind,
				})
				Expect(err).Should(BeNil())
				dirNode = nfs.newFsNode(dst)
			})
		})
		Context("rename a file", func() {
			It("should be ok", func() {
				eno := root.Rename(ctx, filename, root, filenamenew, 0)
				Expect(eno).To(Equal(NoErr))
			})
			It("should be using new name", func() {
				isFound := false
				dir, eno := root.Readdir(ctx)
				Expect(eno).To(Equal(NoErr))
				for dir.HasNext() {
					ch, eno := dir.Next()
					Expect(eno).To(Equal(NoErr))
					if ch.Name == filenamenew {
						isFound = true
					}
				}
				Expect(isFound).To(BeTrue())
			})
		})
		Context("move a file", func() {
			It("should be ok", func() {
				eno := root.Rename(ctx, filenamenew, dirNode, filename, 0)
				Expect(eno).To(Equal(NoErr))
			})
			It("should be found in new dir", func() {
				isFound := false
				dir, eno := dirNode.Readdir(ctx)
				Expect(eno).To(Equal(NoErr))
				for dir.HasNext() {
					ch, eno := dir.Next()
					Expect(eno).To(Equal(NoErr))
					if ch.Name == filename {
						isFound = true
					}
				}
				Expect(isFound).To(BeTrue())
			})
		})
	})
})

var _ = Describe("TestStatfs", func() {
	var ctx = context.TODO()
	Describe("", func() {
		Context("stat fs", func() {
			It("should be ok", func() {
				out := &fuse.StatfsOut{}
				Expect(root.Statfs(ctx, out)).To(Equal(syscall.Errno(0)))
			})
		})
	})
})
