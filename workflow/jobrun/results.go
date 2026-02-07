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

package jobrun

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
)

type Results interface {
	Set(key string, val any) error
	Data() map[string]any
}

type memoryBasedResults struct {
	results map[string]any
	mux     sync.Mutex
}

func (m *memoryBasedResults) Set(key string, val any) error {
	m.mux.Lock()
	m.results[key] = val
	m.mux.Unlock()
	return nil
}

func (m *memoryBasedResults) Data() map[string]any {
	m.mux.Lock()
	defer m.mux.Unlock()
	// Return a copy to prevent external mutation
	data := make(map[string]any)
	for k, v := range m.results {
		data[k] = deepCopy(v)
	}
	return data
}

func NewMemBasedResults() Results {
	return &memoryBasedResults{results: map[string]any{}}
}

const (
	resultsFilename = "results.json"
)

func ResultsFilePath(basePath string) string {
	return path.Join(basePath, resultsFilename)
}

type JSONFileResults struct {
	filePath string
	data     map[string]any
	mux      sync.Mutex
}

func (r *JSONFileResults) Set(key string, val any) error {
	r.mux.Lock()
	r.data[key] = deepCopy(val)
	r.mux.Unlock()
	return r.flush()
}

func (r *JSONFileResults) Data() map[string]any {
	r.mux.Lock()
	defer r.mux.Unlock()
	result := make(map[string]any)
	for k, v := range r.data {
		result[k] = deepCopy(v)
	}
	return result
}

func (r *JSONFileResults) flush() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	data, err := json.Marshal(r.data)
	if err != nil {
		return fmt.Errorf("marshal results error: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmpPath := r.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write temp results file error: %w", err)
	}
	if err := os.Rename(tmpPath, r.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename results file error: %w", err)
	}
	return nil
}

func NewJSONFileResults(filePath string) (Results, error) {
	r := &JSONFileResults{
		filePath: filePath,
		data:     make(map[string]any),
	}

	// Try to load existing results
	f, err := os.Open(filePath)
	if err == nil {
		defer f.Close()
		_ = json.NewDecoder(f).Decode(&r.data)
	}
	return r, nil
}

func deepCopy(val any) any {
	data, _ := json.Marshal(val)
	var result any
	_ = json.Unmarshal(data, &result)
	return result
}
