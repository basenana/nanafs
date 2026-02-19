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

package friday

import (
	"fmt"
	"sync"

	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/indexer"
)

type Factory func(namespace string) (*core.FileSystem, indexer.Indexer, error)

type Manager struct {
	mu      sync.RWMutex
	fridays map[string]*Friday
	llm     openai.Client
	factory Factory
	config  config.Friday
}

func NewFridayManager(llm openai.Client, factory Factory, cfg config.Friday) *Manager {
	return &Manager{
		fridays: make(map[string]*Friday),
		llm:     llm,
		factory: factory,
		config:  cfg,
	}
}

func (m *Manager) GetFriday(namespace string) (*Friday, error) {
	m.mu.RLock()
	f, ok := m.fridays[namespace]
	m.mu.RUnlock()

	if ok {
		return f, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if f, ok = m.fridays[namespace]; ok {
		return f, nil
	}

	fs, idx, err := m.factory(namespace)
	if err != nil {
		return nil, fmt.Errorf("create filesystem for namespace %s: %w", namespace, err)
	}

	store := NewFileSessionStore(fs, namespace)
	f = NewFriday(fs, m.llm, idx, store)
	m.fridays[namespace] = f
	return f, nil
}
