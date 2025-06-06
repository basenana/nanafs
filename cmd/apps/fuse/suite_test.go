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
	"github.com/basenana/nanafs/pkg/core"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
	"testing"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

var (
	cfg = config.Bootstrap{
		FS: &config.FS{Owner: config.FSOwner{Uid: 0, Gid: 0}},
		Storages: []config.Storage{
			{ID: storage.MemoryStorage, Type: storage.MemoryStorage},
		},
	}

	corefs *core.FileSystem
	nfs    *NanaFS
	root   *NanaNode
)

func newCoreFileSystem() *core.FileSystem {
	m, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	tempDir, err := os.MkdirTemp(os.TempDir(), "ut-nanafs-fuse-")
	Expect(err).Should(BeNil())
	cfg.CacheDir = tempDir
	cfg.CacheSize = 0

	c, err := core.New(m, cfg)
	Expect(err).Should(BeNil())
	fs, err := core.NewFileSystem(c, m, types.DefaultNamespace)
	Expect(err).Should(BeNil())

	return fs
}

func newMockNanaFS(fs *core.FileSystem) *NanaFS {
	nfs := &NanaFS{
		FileSystem: fs,
		Path:       "/tmp/test",
		Display:    "NanaFSTest",
		cfg:        cfg.FUSE,
		logger:     logger.NewLogger("fuse.test"),
	}
	return nfs
}

var _ = BeforeSuite(func() {
	var err error
	corefs = newCoreFileSystem()
	nfs = newMockNanaFS(corefs)

	root, err = nfs.rootNode()
	Expect(err).Should(BeNil())
})

var _ = AfterSuite(func() {

})

func TestFs(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fs Suite")
}
