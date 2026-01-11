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
	"fmt"
	"os"
	"path"
	"regexp"
)

var (
	storageIDPattern = "^[A-zA-Z][a-zA-Z0-9-_.]{3,31}$"
	storageIDRegexp  = regexp.MustCompile(storageIDPattern)
)

type verifier func(config *Bootstrap) error

var verifiers = []verifier{
	setDefaultValue,
	checkFuseConfig,
	checkMetaConfig,
	checkStorageConfigs,
	checkGlobalEncryptionConfig,
	checkLocalCache,
	checkWorkflow,
}

func setDefaultValue(config *Bootstrap) error {
	if config.FS == nil {
		config.FS = defaultFsConfig()
	}

	if config.Workflow.JobWorkdir == "" {
		config.Workflow.JobWorkdir = os.TempDir()
	}

	return nil
}

func checkFuseConfig(config *Bootstrap) error {
	fCfg := config.FUSE
	if !fCfg.Enable {
		return nil
	}
	_, err := os.Stat(fCfg.RootPath)
	if err != nil {
		return fmt.Errorf("check fuse.root_pata error: %s", err)
	}
	return nil
}

func checkMetaConfig(config *Bootstrap) error {
	m := config.Meta
	switch m.Type {
	case MemoryMeta:
		return nil
	case SqliteMeta:
		if m.Path == "" {
			return fmt.Errorf("path for sqlite db file is empty")
		}
		return nil
	case PostgresMeta:
		if m.DSN == "" {
			return fmt.Errorf("db dsn is empty")
		}
		return nil
	default:
		return fmt.Errorf("unknown meta type %s", m.Type)
	}
}

func checkStorageConfigs(config *Bootstrap) error {
	if len(config.Storages) == 0 {
		return fmt.Errorf("stroage not config")
	}
	for i, s := range config.Storages {
		if err := checkStorageConfig(s); err != nil {
			return fmt.Errorf("storages[%d].%s: %s", i, s.ID, err)
		}
	}
	return nil
}

func checkGlobalEncryptionConfig(config *Bootstrap) error {
	if !config.Encryption.Enable {
		return nil
	}
	enCfg := config.Encryption
	return checkEncryptionConfig(enCfg)
}

func checkEncryptionConfig(enCfg Encryption) error {
	switch enCfg.Method {
	case AESEncryption:
		skLen := len([]byte(enCfg.SecretKey))
		if skLen != 16 && skLen != 32 {
			return fmt.Errorf("the length of the encryption.secret_key needs to be 16 or 32")
		}
	case ChaCha20Encryption:
		return fmt.Errorf("only supports the AESEncryption encryption method")
	default:
		return fmt.Errorf("unsupports encryption method %s", enCfg.Method)
	}

	return nil
}

func checkStorageConfig(sConfig Storage) error {
	if sConfig.ID == "" {
		return fmt.Errorf("storage.id is empty")
	}
	if !storageIDRegexp.MatchString(sConfig.ID) {
		return fmt.Errorf("storage.id must match %s", storageIDPattern)
	}
	switch sConfig.Type {
	case MemoryStorage:
	case LocalStorage:
		if sConfig.LocalDir == "" {
			return fmt.Errorf("local path is empty")
		}
	case S3Storage:
		cfg := sConfig.S3
		if cfg == nil {
			return fmt.Errorf("s3 is nil")
		}
		if cfg.Region == "" {
			return fmt.Errorf("s3 config region is empty")
		}
		if cfg.AccessKeyID == "" {
			return fmt.Errorf("s3 config access_key_id is empty")
		}
		if cfg.SecretAccessKey == "" {
			return fmt.Errorf("s3 config secret_access_key is empty")
		}
		if cfg.BucketName == "" {
			return fmt.Errorf("s3 config bucket_name is empty")
		}
	case MinioStorage:
		cfg := sConfig.MinIO
		if cfg == nil {
			return fmt.Errorf("minio is nil")
		}
		if cfg.Endpoint == "" {
			return fmt.Errorf("minio config endpoint is empty")
		}
		if cfg.AccessKeyID == "" {
			return fmt.Errorf("minio config access_key_id is empty")
		}
		if cfg.SecretAccessKey == "" {
			return fmt.Errorf("minio config secret_access_key is empty")
		}
		if cfg.BucketName == "" {
			return fmt.Errorf("minio config bucket_name is empty")
		}
	case OSSStorage:
		cfg := sConfig.OSS
		if cfg == nil {
			return fmt.Errorf("OSS config is nil")
		}
		if cfg.Endpoint == "" {
			return fmt.Errorf("OSS endpoint is empty")
		}
		if cfg.AccessKeyID == "" {
			return fmt.Errorf("OSS access_key_id is empty")
		}
		if cfg.AccessKeySecret == "" {
			return fmt.Errorf("OSS access_key_secret is empty")
		}
		if cfg.BucketName == "" {
			return fmt.Errorf("OSS config bucket_name is empty")
		}
	case WebdavStorage:
		cfg := sConfig.Webdav
		if cfg == nil {
			return fmt.Errorf("webdav is nil")
		}
		if cfg.ServerURL == "" {
			return fmt.Errorf("webdav config server_url is empty")
		}
		if cfg.Username == "" {
			return fmt.Errorf("webdav config user is empty")
		}
		if cfg.Password == "" {
			return fmt.Errorf("webdav config password is empty")
		}
	default:
		return fmt.Errorf("unknown storage type: %s", sConfig.Type)
	}
	return nil
}

func checkLocalCache(config *Bootstrap) error {
	if config.CacheDir == "" {
		return fmt.Errorf("cache dir is empty")
	}
	if config.CacheSize < 0 {
		config.CacheSize = 0
	}
	return nil
}

func checkWorkflow(config *Bootstrap) error {
	jwd := config.Workflow.JobWorkdir
	if jwd != "" && !path.IsAbs(jwd) {
		return fmt.Errorf("workflow job_workdir must be an absolute path")
	}
	return nil
}

func Verify(cfg *Bootstrap) error {
	for _, f := range verifiers {
		if err := f(cfg); err != nil {
			return err
		}
	}
	return nil
}
