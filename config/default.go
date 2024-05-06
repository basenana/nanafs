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
	"errors"
	"fmt"
	"github.com/basenana/nanafs/utils/logger"
	"os"
	"path"
	"strings"
)

const (
	DefaultConfigBase   = "nanafs.conf"
	defaultWorkDir      = ".nana"
	defaultSysLocalPath = "/var/lib/nanafs"
)

func LocalUserPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return defaultSysLocalPath
	}
	return path.Join(homeDir, defaultWorkDir)
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
		{Group: WorkflowConfigGroup, Name: "enable", Value: "false"},
		{Group: WorkflowConfigGroup, Name: "job_workdir", Value: ""},
		{Group: PluginConfigGroup, Name: "define_path", Value: ""},
		{Group: WebdavConfigGroup, Name: "enable", Value: "false"},
		{Group: WebdavConfigGroup, Name: "host", Value: "127.0.0.1"},
		{Group: WebdavConfigGroup, Name: "port", Value: "7082"},
	}
)

func setDefaultConfigs(l Loader) error {
	var (
		ctx       context.Context
		configVal Value
		cfgLogger = logger.NewLogger("defaultConfig")
	)
	for _, defaultVal := range defaultConfigValues {
		configVal = l.GetSystemConfig(ctx, defaultVal.Group, defaultVal.Name)
		if configVal.Error == nil {
			continue
		}

		if !errors.Is(configVal.Error, ErrNotConfigured) {
			return fmt.Errorf("get default config %s.%s failed %w", defaultVal.Group, defaultVal.Name, configVal.Error)
		}

		if err := l.SetSystemConfig(ctx, defaultVal.Group, defaultVal.Name, defaultVal.Value); err != nil {
			return fmt.Errorf("set default config %s.%s failed %w", defaultVal.Group, defaultVal.Name, err)
		}
		cfgLogger.Infof("set %s.%s=%s", defaultVal.Group, defaultVal.Name, defaultVal.Value)
	}
	return nil
}

func isConfigNotFound(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "no record") {
		return true
	}
	return false
}
