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

package token

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

var (
	testMeta metastore.Meta
	manager  *Manager
)

func TestToken(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Token Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	testMeta = memMeta

	cfgLoader := config.NewMockConfigLoader(config.Bootstrap{})

	manager = NewTokenManager(memMeta, cfgLoader)
	err = manager.InitBuildinCA(context.TODO())
	Expect(err).Should(BeNil())
})
