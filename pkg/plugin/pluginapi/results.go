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

package pluginapi

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"os"
	"path"
	"sync"
)

type Results interface {
	Set(key string, val any) error
	SetAll(data map[string]any) error
	IsSet(key string) bool
	Load(key string, val any) error
}

type baseMap struct {
	results map[string][]byte
	mux     sync.Mutex
}

func (b *baseMap) Set(key string, val any) error {
	buf := bytes.Buffer{}
	err := gob.NewEncoder(&buf).Encode(val)
	if err != nil {
		return err
	}

	b.mux.Lock()
	b.results[key] = buf.Bytes()
	b.mux.Unlock()
	return nil
}

func (b *baseMap) SetAll(data map[string]any) error {
	if len(data) == 0 {
		return nil
	}
	for k := range data {
		v := data[k]
		if err := b.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (b *baseMap) IsSet(key string) bool {
	b.mux.Lock()
	_, ok := b.results[key]
	b.mux.Unlock()
	return ok
}

func (b *baseMap) Load(key string, val any) error {
	b.mux.Lock()
	raw, ok := b.results[key]
	b.mux.Unlock()
	if !ok {
		return types.ErrNotFound
	}
	buf := bytes.NewReader(raw)
	return gob.NewDecoder(buf).Decode(val)
}

type memoryBasedResults struct{ baseMap }

func NewMemBasedResults() Results {
	return &memoryBasedResults{baseMap{results: map[string][]byte{}}}
}

const (
	defaultFileBasedFilename = ".workflowcontext.gob"
)

func ResultFilePath(basePath string) string {
	return path.Join(basePath, defaultFileBasedFilename)
}

type fileBasedResults struct {
	baseMap
	filePath string
}

func (f *fileBasedResults) Set(key string, val any) error {
	err := f.baseMap.Set(key, val)
	if err != nil {
		return err
	}
	return f.flush()
}

func (f *fileBasedResults) SetAll(data map[string]any) error {
	err := f.baseMap.SetAll(data)
	if err != nil {
		return err
	}
	return f.flush()
}

func (f *fileBasedResults) flush() error {
	file, err := os.OpenFile(f.filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open result file error %w", err)
	}
	return gob.NewEncoder(file).Encode(f.results)
}

func NewFileBasedResults(filePath string) (Results, error) {
	r := &fileBasedResults{baseMap: baseMap{results: map[string][]byte{}}, filePath: filePath}
	f, err := os.Open(filePath)
	if err == nil {
		defer f.Close()
		err = gob.NewDecoder(f).Decode(&(r.results))
		if err != nil {
			return nil, fmt.Errorf("load existed result filed error %w", err)
		}
	}
	return r, nil
}
