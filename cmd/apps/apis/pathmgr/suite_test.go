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

package pathmgr

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

var cfg = config.Config{
	FS:       &config.FS{OwnerUid: 0, OwnerGid: 0, Writeback: false},
	Meta:     config.Meta{Type: metastore.MemoryMeta},
	Storages: []config.Storage{{ID: "test-memory-0", Type: storage.MemoryStorage}},
}

type mockConfig struct{}

func (m mockConfig) GetConfig() (config.Config, error) {
	return cfg, nil
}

func NewMockController() controller.Controller {
	m, _ := metastore.NewMetaStorage(metastore.MemoryMeta, cfg.Meta)
	ctrl, _ := controller.New(mockConfig{}, m)
	return ctrl
}

func TestPathMgr(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()

	RegisterFailHandler(Fail)
	RunSpecs(t, "PathMgr Suite")
}