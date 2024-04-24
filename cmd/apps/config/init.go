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
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/utils"
	"github.com/spf13/cobra"
	"os"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate local default configuration",
	Run: func(cmd *cobra.Command, args []string) {
		initDefaultConfig()
	},
}

func initDefaultConfig() {
	configPath := localConfigFilePath(WorkSpace)

	fmt.Printf("Workspace: %s\n", WorkSpace)
	fmt.Printf("Configuration File: %s\n", configPath)

	_, err := os.Stat(configPath)
	if err == nil {
		fmt.Println("the configuration file already exists.")
		return
	}

	if err = utils.Mkdir(WorkSpace); err != nil {
		fmt.Printf("init workspace failed: %s\n", err.Error())
		return
	}

	dataDir := localDataDirPath(WorkSpace)
	fmt.Printf("Workspace Data Dir: %s\n", dataDir)
	if err = utils.Mkdir(dataDir); err != nil {
		fmt.Printf("init workspace data dir failed: %s\n", err.Error())
		return
	}

	cacheDir := localCacheDirPath(WorkSpace)
	fmt.Printf("Workspace Local Cache Dir: %s\n", cacheDir)
	if err := utils.Mkdir(cacheDir); err != nil {
		fmt.Printf("init workspace local cache dir failed: %s\n", err.Error())
		return
	}

	dbFile := localDbFilePath(WorkSpace)
	fmt.Printf("Workspace Database File: %s\n", dbFile)

	sk, err := utils.RandString(16)
	if err != nil {
		fmt.Printf("gen encryption secret key failed: %s\n", err.Error())
		return
	}

	conf := config.Bootstrap{
		Meta: config.Meta{
			Type: metastore.SqliteMeta,
			Path: dbFile,
		},
		Storages: []config.Storage{
			{
				ID:       "local-0",
				Type:     config.LocalStorage,
				LocalDir: dataDir,
			},
		},
		GlobalEncryption: config.Encryption{
			Enable:    false,
			Method:    config.AESEncryption,
			SecretKey: sk,
		},
		CacheDir:  cacheDir,
		CacheSize: 10,
		Debug:     false,
		Webdav: &config.Webdav{
			Enable:         false,
			Host:           "127.0.0.1",
			Port:           7082,
			OverwriteUsers: []config.OverwriteUser{{Username: "admin", Password: "changeme"}},
		},
		FUSE: config.FUSE{
			Enable:      false,
			RootPath:    "/your/path/to/mount",
			DisplayName: "nanafs",
		},
	}
	fmt.Printf("Workspace Config: %s\n", configPath)
	raw, _ := json.MarshalIndent(conf, "", "    ")
	if err := os.WriteFile(configPath, raw, 0755); err != nil {
		fmt.Printf("wirteback config file failed: %s\n", err.Error())
		return
	}
	fmt.Println("Generate local configuration succeed")
}
