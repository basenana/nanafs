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

package controller

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	ctrl       Controller
	entryStore metastore.DEntry
)

func TestController(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	entryStore = memMeta

	ctrl, err = New(mockConfig{}, memMeta)
	Expect(err).Should(BeNil())

	// init plugin
	err = plugin.Init(buildin.Services{}, &config.Plugin{})
	Expect(err).Should(BeNil())
})

type mockConfig struct{}

var _ config.Loader = mockConfig{}

func (m mockConfig) GetBootstrapConfig() (config.Bootstrap, error) {
	var cfg = config.Bootstrap{
		FS:       &config.FS{Owner: config.FSOwner{Uid: 0, Gid: 0}, Writeback: false},
		Meta:     config.Meta{Type: metastore.MemoryMeta},
		Storages: []config.Storage{{ID: "test-memory-0", Type: storage.MemoryStorage}},
	}
	return cfg, nil
}
