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

package bio

import (
	"context"
	"github.com/basenana/nanafs/utils/logger"
	"sync"
)

type Reader interface {
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Close()
}

type Writer interface {
	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	Flush(ctx context.Context) error
	Fsync(ctx context.Context) error
	Close()
}

var (
	fileChunkReaders = make(map[int64]Reader)
	fileChunkWriters = make(map[int64]Writer)
	fileChunkMux     sync.Mutex
)

func CloseAll() {
	log := logger.NewLogger("Closing")

	allReaders := make(map[int64]Reader)
	allWriters := make(map[int64]Writer)
	fileChunkMux.Lock()
	for fid := range fileChunkWriters {
		allWriters[fid] = fileChunkWriters[fid]
	}
	for fid := range fileChunkReaders {
		allReaders[fid] = fileChunkReaders[fid]
	}
	fileChunkMux.Unlock()

	wg := sync.WaitGroup{}
	wg.Add(len(allWriters))
	for i := range allWriters {
		go func(w Writer) {
			defer wg.Done()
			w.Close()
		}(allWriters[i])
	}
	wg.Add(len(allReaders))
	for i := range allReaders {
		go func(r Reader) {
			defer wg.Done()
			r.Close()
		}(allReaders[i])
	}
	wg.Wait()
	log.Infow("all opened file closed")
}
