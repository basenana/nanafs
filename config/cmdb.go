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

package config

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/utils/logger"
	"sync"
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

	data, ok := f.db[cacheConfigKey{namespace: namespace, group: group, name: namespace}]
	if !ok {
		return "", fmt.Errorf("cmdb: no record")
	}
	return data, nil
}

func (f *memCmdb) SetConfigValue(ctx context.Context, namespace, group, name, value string) error {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.db[cacheConfigKey{namespace: namespace, group: group, name: namespace}] = value
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

var (
	defaultConfigValues []Value = []Value{
		{Group: AdminApiConfigGroup, Name: "enable", Value: "false"},
		{Group: AdminApiConfigGroup, Name: "host", Value: "127.0.0.1"},
		{Group: AdminApiConfigGroup, Name: "port", Value: "7080"},
		{Group: AdminApiConfigGroup, Name: "enable_metric", Value: "true"},
		{Group: AdminApiConfigGroup, Name: "enable_pprof", Value: "true"},
		{Group: FsAPIConfigGroup, Name: "enable", Value: "true"},
		{Group: FsAPIConfigGroup, Name: "host", Value: "127.0.0.1"},
		{Group: FsAPIConfigGroup, Name: "port", Value: "7081"},
		{Group: WorkflowConfigGroup, Name: "job_workdir", Value: ""},
		{Group: PluginConfigGroup, Name: "define_path", Value: ""},
		{Group: WebdavConfigGroup, Name: "enable", Value: "false"},
		{Group: WebdavConfigGroup, Name: "host", Value: "127.0.0.1"},
		{Group: WebdavConfigGroup, Name: "port", Value: "7082"},
		{Group: DocConfigGroup, Name: "index.enable", Value: "false"},
		{Group: DocConfigGroup, Name: "index.local_indexer_dir", Value: ""},
		{Group: DocConfigGroup, Name: "index.jieba_dict_file", Value: ""},
	}
)

func setCMDBDefaultConfigs(db CMDB) error {
	var (
		ctx       context.Context = context.Background()
		cfgLogger                 = logger.NewLogger("defaultConfig")
	)
	for _, defaultVal := range defaultConfigValues {
		_, err := db.GetConfigValue(ctx, "", defaultVal.Group, defaultVal.Name)
		if err == nil {
			continue
		}

		if !isConfigNotFound(err) {
			return fmt.Errorf("get default config %s.%s failed %w", defaultVal.Group, defaultVal.Name, err)
		}

		if err := db.SetConfigValue(ctx, "", defaultVal.Group, defaultVal.Name, defaultVal.Value); err != nil {
			return fmt.Errorf("set default config %s.%s failed %w", defaultVal.Group, defaultVal.Name, err)
		}
		cfgLogger.Infof("set %s.%s=%s", defaultVal.Group, defaultVal.Name, defaultVal.Value)
	}
	return nil
}
