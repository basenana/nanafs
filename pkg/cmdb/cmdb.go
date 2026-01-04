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

package cmdb

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/basenana/nanafs/utils/logger"
)

const (
	DocConfigGroup      = "document"
	PluginConfigGroup   = "plugin"
	WorkflowConfigGroup = "workflow"
)

type CMDB interface {
	GetConfigValue(ctx context.Context, namespace, group, name string) (string, error)
	SetConfigValue(ctx context.Context, namespace, group, name, value string) error
}

type memCmdb struct {
	db  map[cacheConfigKey]string
	mux sync.Mutex
}

func (f *memCmdb) GetConfigValue(ctx context.Context, namespace, group, name string) (string, error) {
	f.mux.Lock()
	defer f.mux.Unlock()

	data, ok := f.db[cacheConfigKey{namespace: namespace, group: group, name: name}]
	if !ok {
		return "", fmt.Errorf("cmdb: no record")
	}
	return data, nil
}

func (f *memCmdb) SetConfigValue(ctx context.Context, namespace, group, name, value string) error {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.db[cacheConfigKey{namespace: namespace, group: group, name: name}] = value
	return nil
}

func (f *memCmdb) key(group, name string) string {
	return fmt.Sprintf("%s.%s", group, name)
}

func NewMemCmdb() CMDB {
	return &memCmdb{db: map[cacheConfigKey]string{}}
}

type cacheConfigKey struct {
	namespace string
	group     string
	name      string
}

type Value struct {
	Group string
	Name  string
	Value string
	Error error
}

var (
	defaultConfigValues []Value = []Value{}
)

func SetCMDBDefaultConfigs(db CMDB) error {
	var (
		ctx       context.Context = context.Background()
		cfgLogger                 = logger.NewLogger("defaultConfig")
	)
	for _, defaultVal := range defaultConfigValues {
		_, err := db.GetConfigValue(ctx, "", defaultVal.Group, defaultVal.Name)
		if err == nil {
			continue
		}

		if !IsConfigNotFound(err) {
			return fmt.Errorf("get default config %s.%s failed %w", defaultVal.Group, defaultVal.Name, err)
		}

		if err := db.SetConfigValue(ctx, "", defaultVal.Group, defaultVal.Name, defaultVal.Value); err != nil {
			return fmt.Errorf("set default config %s.%s failed %w", defaultVal.Group, defaultVal.Name, err)
		}
		cfgLogger.Infof("set %s.%s=%s", defaultVal.Group, defaultVal.Name, defaultVal.Value)
	}
	return nil
}

func IsConfigNotFound(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "no record") {
		return true
	}
	return false
}
