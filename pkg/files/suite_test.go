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

package files

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	fileChunk1 []byte
	fileChunk2 []byte
	fileChunk3 []byte
	fileChunk4 []byte
)

func resetFileChunk() {
	fileChunk1 = make([]byte, fileChunkSize)
	fileChunk2 = make([]byte, fileChunkSize)
	fileChunk3 = make([]byte, fileChunkSize)
	fileChunk4 = make([]byte, fileChunkSize)
	copy(fileChunk1, []byte("testdata-1"))
	copy(fileChunk2, []byte("          "))
	copy(fileChunk3, []byte("testdata-3"))
	copy(fileChunk4, []byte(""))
}

func NewMockStorage() storage.Storage {
	s, _ := storage.NewStorage("memory", config.Storage{})
	InitFileIoChain(config.Config{}, s, make(chan struct{}))
	return s
}

func newMockObject(name string, inode int64) *types.Object {
	meta := types.NewMetadata(name, types.RawKind)
	meta.Size = fileChunkSize * 4
	meta.ID = inode
	return &types.Object{
		Metadata: meta,
	}
}

func TestFile(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "File Suite")
}
