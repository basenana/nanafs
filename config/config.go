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
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/basenana/nanafs/pkg/cmdb"
)

var FilePath string

var (
	ErrNotConfigured = fmt.Errorf("no configured")
)

type Config interface {
	GetBootstrapConfig() Bootstrap
	RegisterCMDB(cmdb cmdb.CMDB) error
	SetSystemConfig(ctx context.Context, group, name string, value any) error
	GetSystemConfig(ctx context.Context, group, name string) Value
	GetNamespacedConfig(ctx context.Context, namespace, group, name string) Value
	SetNamespacedConfig(ctx context.Context, namespace, group, name string, value any) error
}

type configWrapper struct {
	cmdb   cmdb.CMDB
	cached map[cmdbCacheKey]*Value
	bCfg   *Bootstrap
	mux    sync.RWMutex
}

type cmdbCacheKey struct {
	namespace string
	group     string
	name      string
}

func (c *configWrapper) initBootstrapConfig() error {
	if c.bCfg != nil {
		return nil
	}

	result := Bootstrap{}

	if FilePath == "" {
		return fmt.Errorf("--config not set")
	}

	_, err := os.Stat(FilePath)
	if err != nil {
		return fmt.Errorf("open config file failed: %w", err)
	}

	f, err := os.Open(FilePath)
	if err != nil {
		return fmt.Errorf("open config file failed: %w", err)
	}
	defer f.Close()

	jd := json.NewDecoder(f)
	if err = jd.Decode(&result); err != nil {
		return fmt.Errorf("parse config failed: %w", err)
	}

	if err = Verify(&result); err != nil {
		return err
	}

	c.bCfg = &result
	return nil
}

func (c *configWrapper) GetBootstrapConfig() Bootstrap {
	return *c.bCfg
}

func (c *configWrapper) RegisterCMDB(m cmdb.CMDB) error {
	c.cmdb = m
	return cmdb.SetCMDBDefaultConfigs(m)
}

func (c *configWrapper) SetSystemConfig(ctx context.Context, group, name string, value any) error {
	return c.SetNamespacedConfig(ctx, "", group, name, value)
}

func (c *configWrapper) GetSystemConfig(ctx context.Context, group, name string) Value {
	return c.GetNamespacedConfig(ctx, "", group, name)
}

func (c *configWrapper) SetNamespacedConfig(ctx context.Context, namespace, group, name string, value any) error {
	var record = Value{Group: group, Name: name}
	switch fmtVal := value.(type) {
	case string:
		record.Value = fmtVal
	case int, int64:
		record.Value = fmt.Sprintf("%d", fmtVal)
	case bool:
		if fmtVal {
			record.Value = "true"
		} else {
			record.Value = "false"
		}
	default:
		bData, err := json.Marshal(value)
		if err != nil {
			return err
		}
		record.Value = string(bData)
	}

	c.mux.Lock()
	delete(c.cached, cmdbCacheKey{namespace: namespace, group: group, name: name})
	c.mux.Unlock()

	return c.cmdb.SetConfigValue(ctx, namespace, group, name, record.Value)
}

func (c *configWrapper) GetNamespacedConfig(ctx context.Context, namespace, group, name string) Value {
	c.mux.RLock()
	cachedRecord := c.cached[cmdbCacheKey{namespace: namespace, group: group, name: name}]
	c.mux.RUnlock()

	if cachedRecord != nil &&
		(cachedRecord.expiration == nil || time.Now().Before(*cachedRecord.expiration)) {
		return *cachedRecord
	}

	var record = Value{Group: group, Name: name}
	if c.cmdb == nil {
		record.Error = fmt.Errorf("cmdb not init")
		return record
	}
	record.Value, record.Error = c.cmdb.GetConfigValue(ctx, namespace, group, name)
	if record.Error != nil && cmdb.IsConfigNotFound(record.Error) {
		record.Error = ErrNotConfigured
	}

	if record.Error != nil {
		return record
	}

	exp := time.Now().Add(time.Minute * 15)
	record.expiration = &exp
	c.mux.Lock()
	c.cached[cmdbCacheKey{namespace: namespace, group: group, name: name}] = &record
	c.mux.Unlock()

	return record
}

func NewConfig() (Config, error) {
	cfg := &configWrapper{cmdb: cmdb.NewMemCmdb(), cached: make(map[cmdbCacheKey]*Value)}
	_ = cmdb.SetCMDBDefaultConfigs(cfg.cmdb)
	return cfg, cfg.initBootstrapConfig()
}

func NewMockConfig(b Bootstrap) Config {
	cfg := &configWrapper{cmdb: cmdb.NewMemCmdb(), cached: make(map[cmdbCacheKey]*Value), bCfg: &b}
	_ = cmdb.SetCMDBDefaultConfigs(cfg.cmdb)
	return cfg
}

type Value struct {
	Group string
	Name  string
	Value string
	Error error

	expiration *time.Time
}

func (v Value) Int() (int, error) {
	if v.Error != nil {
		return 0, v.Error
	}
	return strconv.Atoi(v.Value)
}

func (v Value) Int64() (int64, error) {
	if v.Error != nil {
		return 0, v.Error
	}
	return strconv.ParseInt(v.Value, 10, 64)
}

func (v Value) Bool() (bool, error) {
	if v.Error != nil {
		return false, v.Error
	}
	switch strings.ToLower(v.Value) {
	case "yes", "y", "true", "t":
		return true, nil
	case "no", "n", "false", "f":
		return false, nil
	default:
		return false, fmt.Errorf("unknown bool config value %s", v.Value)
	}
}

func (v Value) String() (string, error) {
	return v.Value, v.Error
}

func (v Value) Unmarshal(data any) error {
	if v.Error != nil {
		return v.Error
	}
	if reflect.TypeOf(data).Kind() != reflect.Pointer {
		return fmt.Errorf("not a pointer")
	}
	return json.Unmarshal([]byte(v.Value), data)
}
