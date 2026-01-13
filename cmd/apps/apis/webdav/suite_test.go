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

package webdav

import (
	"context"
	"os"
	"testing"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	testMeta   metastore.Meta
	testCore   core.Core
	testFs     *core.FileSystem
	testLogger *zap.SugaredLogger
	namespace  = types.DefaultNamespace
	bootCfg    config.Bootstrap
	workdir    string
)

func TestWebdav(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webdav Suite")
}

var _ = BeforeSuite(func() {
	var err error
	workdir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-webdav-")
	Expect(err).Should(BeNil())

	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	testMeta = memMeta

	bootCfg = config.Bootstrap{
		FS:       &config.FS{Owner: config.FSOwner{Uid: 0, Gid: 0}, Writeback: false},
		Storages: []config.Storage{{ID: "test-memory-0", Type: storage.MemoryStorage}},
		CacheDir: workdir,
		CacheSize: 0,
	}

	testCore, err = core.New(testMeta, bootCfg)
	Expect(err).Should(BeNil())

	testFs, err = core.NewFileSystem(testCore, testMeta, namespace)
	Expect(err).Should(BeNil())
})

var _ = AfterSuite(func() {
	if workdir != "" {
		os.RemoveAll(workdir)
	}
})

var _ = AfterEach(func() {
	// Cleanup is handled by individual tests using unique entry names
})

func withUserContext(ctx context.Context, uid, gid int64) context.Context {
	return context.WithValue(ctx, userInfoContextKey, &UserInfo{
		UID:       uid,
		GID:       gid,
		Namespace: namespace,
	})
}
