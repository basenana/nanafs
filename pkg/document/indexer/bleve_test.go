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
	"context"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("testDocumentManage", func() {
	var (
		ctx             = context.TODO()
		indexer         *Bleve
		localIndexerDir = path.Join(workdir, "index")
	)
	Context("init document indexer", func() {
		It("init should be succeed", func() {
			var err error
			indexer, err = NewBleveIndexer(recorder, map[string]string{
				"localIndexerDir": localIndexerDir,
				"jiebaDictFile":   "",
			})
			Expect(err).Should(BeNil())
		})
		It("insert one document should be succeed", func() {
			Expect(indexer).ShouldNot(BeNil())
			err := recorder.SaveDocument(ctx, needIndexedDoc)
			Expect(err).Should(BeNil())
			err = indexer.Index(ctx, needIndexedDoc)
			Expect(err).Should(BeNil())
		})
		It("search document should be succeed", func() {
			Expect(indexer).ShouldNot(BeNil())
			ctx = types.WithNamespace(ctx, types.NewNamespace("testns"))
			docList, err := indexer.Query(ctx, "butter", QueryDialectBleve)
			Expect(err).Should(BeNil())

			Expect(len(docList)).Should(Equal(1))
		})
		It("search document in other ns should be succeed", func() {
			Expect(indexer).ShouldNot(BeNil())
			ctx = types.WithNamespace(ctx, types.NewNamespace("none"))
			docList, err := indexer.Query(ctx, "butter", QueryDialectBleve)
			Expect(err).Should(BeNil())

			Expect(len(docList)).Should(Equal(0))
		})
	})
})

var (
	needIndexedDoc = &types.Document{
		ID:            1727330397221748736,
		OID:           1000,
		Name:          "Hello World!",
		Namespace:     "testns",
		ParentEntryID: 1,
		Source:        "unittest",
		KeyWords:      make([]string, 0),
		Content: `<content>Betty bought some butter, but the butter was bitter. 
So Betty bought some better butter to make the bitter butter better.</content>`,
		CreatedAt: time.Now(),
		ChangedAt: time.Now(),
	}
)
