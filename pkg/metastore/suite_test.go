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

package metastore

import (
	"github.com/basenana/nanafs/utils/logger"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var workdir string

func TestMetaStore(t *testing.T) {
	RegisterFailHandler(Fail)

	logger.InitLogger()
	defer logger.Sync()

	var err error
	workdir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-storage-")
	Expect(err).Should(BeNil())
	t.Logf("unit test workdir on: %s", workdir)

	RunSpecs(t, "MetaStore Suite")
}
