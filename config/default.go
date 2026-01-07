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
	"crypto/rand"
	"fmt"
	"path"

	"github.com/basenana/nanafs/utils"
)

func DefaultConfig(workdir string) (Bootstrap, error) {
	var (
		dataPath  = path.Join(workdir, "local-data")
		cachePath = path.Join(workdir, "cache")
		err       error
	)

	secretKey := generateSecretKey(16)
	bcfg := Bootstrap{
		FUSE:   FUSE{Enable: false},
		Webdav: Webdav{Enable: false},
		API: FsApi{
			Enable: true,
			Host:   "127.0.0.1",
			Port:   17086,
		},
		Meta: Meta{
			Type: SqliteMeta,
			Path: fmt.Sprintf("%s/nanafs.db", workdir),
		},
		Storages: []Storage{
			{
				ID:       "local-data",
				Type:     LocalStorage,
				LocalDir: dataPath,
			},
		},
		Encryption: Encryption{
			Enable:    true,
			Method:    AESEncryption,
			SecretKey: secretKey,
		},
		FS:        defaultFsConfig(),
		CacheDir:  cachePath,
		CacheSize: 2,
		Debug:     false,
		Workflow: Workflow{
			JobWorkdir: "/tmp",
		},
	}

	err = utils.Mkdir(dataPath)
	if err != nil {
		return bcfg, err
	}

	err = utils.Mkdir(cachePath)
	if err != nil {
		return bcfg, err
	}

	return bcfg, nil
}

func generateSecretKey(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	key := make([]byte, length)
	_, _ = rand.Read(key)
	for i := range key {
		key[i] = charset[int(key[i])%len(charset)]
	}
	return string(key)
}
