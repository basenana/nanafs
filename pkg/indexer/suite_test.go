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

package indexer

import (
	"os"
	"path"
	"testing"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	workdir   string
	namespace = types.DefaultNamespace
	testMeta  metastore.Meta
)

func TestDispatch(t *testing.T) {
	RegisterFailHandler(Fail)

	logger.InitLogger()
	defer logger.Sync()
	logger.SetDebug(true)

	RunSpecs(t, "Indexer Suite")
}

var _ = BeforeEach(func() {
	var err error
	workdir, err = os.MkdirTemp(os.TempDir(), "nanafs-indexer-")
	Expect(err).Should(BeNil())

	mdb, err := metastore.NewMetaStorage(metastore.SqliteMeta, config.Meta{Type: config.SqliteMeta, Path: path.Join(workdir, "meta.db")})
	Expect(err).Should(BeNil())
	testMeta = mdb
})
