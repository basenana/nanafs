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
	"os/user"
	"strconv"
)

const (
	MemoryMeta         = "memory"
	SqliteMeta         = "sqlite"
	PostgresMeta       = "postgres"
	S3Storage          = "s3"
	OSSStorage         = "oss"
	MinioStorage       = "minio"
	WebdavStorage      = "webdav"
	LocalStorage       = "local"
	MemoryStorage      = "memory"
	AESEncryption      = "AES"
	ChaCha20Encryption = "ChaCha20"
)

type FS struct {
	Owner     FSOwner `json:"owner,omitempty"`
	Writeback bool    `json:"writeback,omitempty"`
	PageSize  int     `json:"page_size,omitempty"`
}

type FSOwner struct {
	Uid int64 `json:"uid"`
	Gid int64 `json:"gid"`
}

type Meta struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
	DSN  string `json:"dsn,omitempty"`
}

type Storage struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	LocalDir   string               `json:"local_dir,omitempty"`
	S3         *S3Config            `json:"s3,omitempty"`
	MinIO      *MinIOConfig         `json:"minio,omitempty"`
	OSS        *OSSConfig           `json:"oss,omitempty"`
	Webdav     *WebdavStorageConfig `json:"webdav,omitempty"`
	Encryption *Encryption          `json:"encryption,omitempty"`
}

type S3Config struct {
}

type MinIOConfig struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	BucketName      string `json:"bucket_name"`
	Location        string `json:"location"`
	Token           string `json:"token"`
	UseSSL          bool   `json:"use_ssl"`
}

type OSSConfig struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	BucketName      string `json:"bucket_name"`
}

type WebdavStorageConfig struct {
	ServerURL string `json:"server_url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Insecure  bool   `json:"insecure,omitempty"`
}

func defaultFsConfig() *FS {
	u, err := user.Current()
	if err != nil {
		return nil
	}
	owner := FSOwner{}
	owner.Uid, _ = strconv.ParseInt(u.Uid, 10, 64)
	owner.Gid, _ = strconv.ParseInt(u.Gid, 10, 64)
	return &FS{Owner: owner}
}
