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

package storage

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
	"path"
)

var _ = Describe("TestTemporaryNode", func() {
	cacheDir := path.Join(tempDir, "cache-TestTemporaryNode")
	Expect(os.Mkdir(cacheDir, 0755)).Should(BeNil())
	storageDir := path.Join(tempDir, "storage-TestTemporaryNode")
	Expect(os.Mkdir(storageDir, 0755)).Should(BeNil())
	sk, err := utils.RandString(32)
	Expect(err).Should(BeNil())

	cfg := config.Config{
		Storages: []config.Storage{
			{
				ID:       "test-local-1",
				Type:     LocalStorage,
				LocalDir: storageDir,
			},
			{
				ID:       "test-local-2",
				Type:     LocalStorage,
				LocalDir: storageDir,
				Encryption: &config.Encryption{
					Enable:    true,
					Method:    config.AESEncryption,
					SecretKey: sk,
				},
			},
		},
		CacheDir:         cacheDir,
		GlobalEncryption: config.Encryption{},
		Debug:            false,
	}
	InitLocalCache(cfg)

	Context("test open temp node", func() {
		var (
			lc *LocalCache
			no CacheNode
		)

		data, err := utils.RandString(1024)
		Expect(err).Should(BeNil())

		It("init local cache", func() {
			s, err := NewStorage(cfg.Storages[0].ID, LocalStorage, cfg.Storages[0])
			Expect(err).Should(BeNil())
			lc = NewLocalCache(s)
		})

		It("open should be succeed", func() {
			no, err = lc.OpenTemporaryNode(context.Background(), 0, 0)
			Expect(err).Should(BeNil())
		})

		It("write should be succeed", func() {
			n, err := no.WriteAt([]byte(data), 0)
			Expect(err).Should(BeNil())
			Expect(n).Should(Equal(len(data)))
		})

		It("commit should be succeed", func() {
			err = lc.CommitTemporaryNode(context.Background(), 1, 1, no)
			Expect(err).Should(BeNil())
			Expect(no.Close()).Should(BeNil())
		})

		It("verify should be succeed", func() {
			no, err = lc.openDirectNode(context.Background(), 1, 1)
			Expect(err).Should(BeNil())

			loadData, err := ioutil.ReadAll(utils.NewReader(no))
			Expect(err).Should(BeNil())
			Expect(no.Close()).Should(BeNil())

			Expect(loadData).Should(Equal([]byte(data)))
		})
	})

	Context("test encrypt node to commit", func() {
		var (
			lc *LocalCache
			no CacheNode
		)

		data, err := utils.RandString(cacheNodeBaseSize * 8)
		Expect(err).Should(BeNil())

		It("init local cache", func() {
			s, err := NewStorage(cfg.Storages[1].ID, LocalStorage, cfg.Storages[1])
			Expect(err).Should(BeNil())
			lc = NewLocalCache(s)
		})

		It("open should be succeed", func() {
			no, err = lc.OpenTemporaryNode(context.Background(), 0, 0)
			Expect(err).Should(BeNil())
		})

		It("write should be succeed", func() {
			n, err := no.WriteAt([]byte(data), 0)
			Expect(err).Should(BeNil())
			Expect(n).Should(Equal(len(data)))
		})

		It("commit should be succeed", func() {
			err = lc.CommitTemporaryNode(context.Background(), 1, 1, no)
			Expect(err).Should(BeNil())
			Expect(no.Close()).Should(BeNil())
		})

		It("verify should be succeed", func() {
			no, err = lc.openDirectNode(context.Background(), 1, 1)
			Expect(err).Should(BeNil())

			loadData, err := ioutil.ReadAll(utils.NewReader(no))
			Expect(err).Should(BeNil())
			Expect(no.Close()).Should(BeNil())

			Expect(loadData).Should(Equal([]byte(data)))
		})
	})
})

var _ = Describe("TestCacheNode", func() {
	cacheDir := path.Join(tempDir, "cache-TestCacheNode")
	Expect(os.Mkdir(cacheDir, 0755)).Should(BeNil())

	sk, err := utils.RandString(32)
	Expect(err).Should(BeNil())

	cfg := config.Config{
		Storages: []config.Storage{
			{
				ID:   "test-local-1",
				Type: MemoryStorage,
			},
		},
		CacheDir:  cacheDir,
		CacheSize: 1,
		GlobalEncryption: config.Encryption{
			Enable:    true,
			Method:    config.AESEncryption,
			SecretKey: sk,
		},
		Debug: false,
	}
	InitLocalCache(cfg)

	Context("test open temp node", func() {
		var (
			lc *LocalCache
			no CacheNode
		)

		data, err := utils.RandString(cacheNodeBaseSize)
		Expect(err).Should(BeNil())

		It("init local cache", func() {
			s, err := NewStorage(cfg.Storages[0].ID, cfg.Storages[0].Type, cfg.Storages[0])
			Expect(err).Should(BeNil())
			lc = NewLocalCache(s)
		})

		It("open should be succeed", func() {
			no, err = lc.OpenTemporaryNode(context.Background(), 0, 0)
			Expect(err).Should(BeNil())
		})

		It("write should be succeed", func() {
			n, err := no.WriteAt([]byte(data), 0)
			Expect(err).Should(BeNil())
			Expect(n).Should(Equal(len(data)))
		})

		It("commit should be succeed", func() {
			err = lc.CommitTemporaryNode(context.Background(), 1, 1, no)
			Expect(err).Should(BeNil())
			Expect(no.Close()).Should(BeNil())
		})

		It("verify should be succeed", func() {
			no, err = lc.OpenCacheNode(context.Background(), 1, 1)
			Expect(err).Should(BeNil())

			loadData, err := ioutil.ReadAll(utils.NewReader(no))
			Expect(err).Should(BeNil())
			Expect(no.Close()).Should(BeNil())

			Expect(loadData).Should(Equal([]byte(data)))
		})

		It("read again should be succeed", func() {
			no, err = lc.OpenCacheNode(context.Background(), 1, 1)
			Expect(err).Should(BeNil())

			loadData, err := ioutil.ReadAll(utils.NewReader(no))
			Expect(err).Should(BeNil())
			Expect(no.Close()).Should(BeNil())

			Expect(loadData).Should(Equal([]byte(data)))
		})

		It("read again should be succeed", func() {
			no1, err := lc.OpenCacheNode(context.Background(), 1, 1)
			Expect(err).Should(BeNil())

			no2, err := lc.OpenCacheNode(context.Background(), 1, 1)
			Expect(err).Should(BeNil())

			loadData, err := ioutil.ReadAll(utils.NewReader(no2))
			Expect(err).Should(BeNil())

			Expect(no1.Close()).Should(BeNil())
			Expect(no2.Close()).Should(BeNil())

			Expect(loadData).Should(Equal([]byte(data)))
		})
	})
})
