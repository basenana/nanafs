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
)

var FilePath string

var (
	ErrNotConfigured = fmt.Errorf("no configured")
)

type Loader interface {
	GetBootstrapConfig() (Bootstrap, error)
	InitCMDB(cmdb CMDB) error
	SetSystemConfig(ctx context.Context, group, name string, value any) error
	GetSystemConfig(ctx context.Context, group, name string) Value
}

type loader struct {
	cmdb CMDB
}

func (l *loader) GetBootstrapConfig() (Bootstrap, error) {
	result := Bootstrap{}

	if FilePath == "" {
		return result, fmt.Errorf("--config not set")
	}

	_, err := os.Stat(FilePath)
	if err != nil {
		return result, fmt.Errorf("open config file failed: %w", err)
	}

	f, err := os.Open(FilePath)
	if err != nil {
		return result, fmt.Errorf("open config file failed: %w", err)
	}
	defer f.Close()

	jd := json.NewDecoder(f)
	if err = jd.Decode(&result); err != nil {
		return result, fmt.Errorf("parse config failed: %w", err)
	}

	if err = Verify(&result); err != nil {
		return result, err
	}

	return result, nil
}

func (l *loader) InitCMDB(cmdb CMDB) error {
	l.cmdb = cmdb
	return setDefaultConfigs(l)
}

func (l *loader) SetSystemConfig(ctx context.Context, group, name string, value any) error {
	if l.cmdb == nil {
		return fmt.Errorf("cmdb not init")
	}

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

	return l.cmdb.SetConfigValue(ctx, group, name, record.Value)
}

func (l *loader) GetSystemConfig(ctx context.Context, group, name string) Value {
	var record = Value{Group: group, Name: name}
	if l.cmdb == nil {
		record.Error = fmt.Errorf("cmdb not init")
		return record
	}
	record.Value, record.Error = l.cmdb.GetConfigValue(ctx, group, name)
	if record.Error != nil && isConfigNotFound(record.Error) {
		record.Error = ErrNotConfigured
	}
	return record
}

func NewConfigLoader() Loader {
	return &cacheLoader{
		loader: &loader{},
		cached: make(map[string]*Value),
	}
}

type cacheLoader struct {
	loader *loader
	cached map[string]*Value
	mux    sync.Mutex
}

func (c *cacheLoader) GetBootstrapConfig() (Bootstrap, error) {
	return c.loader.GetBootstrapConfig()
}

func (c *cacheLoader) InitCMDB(cmdb CMDB) error {
	return c.loader.InitCMDB(cmdb)
}

func (c *cacheLoader) SetSystemConfig(ctx context.Context, group, name string, value any) error {
	err := c.loader.SetSystemConfig(ctx, group, name, value)
	if err != nil {
		return err
	}
	c.mux.Lock()
	delete(c.cached, c.cacheKey(group, name))
	c.mux.Unlock()
	return nil
}

func (c *cacheLoader) GetSystemConfig(ctx context.Context, group, name string) Value {
	keyStr := c.cacheKey(group, name)
	c.mux.Lock()
	cachedRecord := c.cached[keyStr]
	c.mux.Unlock()

	if cachedRecord != nil &&
		(cachedRecord.Expiration == nil || time.Now().Before(*cachedRecord.Expiration)) {
		return *cachedRecord
	}

	record := c.loader.GetSystemConfig(ctx, group, name)
	if record.Error != nil {
		return record
	}
	record.Expiration = c.initExpiration()
	c.mux.Lock()
	c.cached[keyStr] = &record
	c.mux.Unlock()

	return record
}

func (c *cacheLoader) cacheKey(group, name string) string {
	return fmt.Sprintf("%s.%s", group, name)
}

func (c *cacheLoader) initExpiration() *time.Time {
	expiration := time.Now().Add(time.Minute * 15)
	return &expiration
}

type fakeLoader struct {
	*loader
	b Bootstrap
}

func (f *fakeLoader) GetBootstrapConfig() (Bootstrap, error) {
	return f.b, nil
}

func NewFakeConfigLoader(b Bootstrap) Loader {
	l := &fakeLoader{loader: &loader{}, b: b}
	_ = l.InitCMDB(NewFakeCmdb())
	return l
}

type Value struct {
	Group string
	Name  string
	Value string
	Error error

	Expiration *time.Time
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
