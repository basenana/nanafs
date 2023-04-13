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

type Config struct {
	Meta      Meta      `json:"meta"`
	Storages  []Storage `json:"storages"`
	Owner     *FsOwner  `json:"owner,omitempty"`
	Plugin    Plugin    `json:"plugin"`
	CacheDir  string    `json:"cache_dir,omitempty"`
	CacheSize int       `json:"cache_size,omitempty"`
	Debug     bool      `json:"debug"`

	ApiConfig Api `json:"api"`
	FsConfig  Fs  `json:"fs"`
}

type Meta struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
	DSN  string `json:"dsn,omitempty"`
}

type Storage struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	LocalDir string       `json:"local_dir,omitempty"`
	MinIO    *MinIOConfig `json:"minio,omitempty"`
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

type Api struct {
	Enable bool   `json:"enable"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Pprof  bool   `json:"pprof"`
}

type Fs struct {
	Enable       bool     `json:"enable"`
	RootPath     string   `json:"root_path"`
	MountOptions []string `json:"mount_options,omitempty"`
	DisplayName  string   `json:"display_name,omitempty"`
	VerboseLog   bool     `json:"verbose_log,omitempty"`

	EntryTimeout *int `json:"entry_timeout,omitempty"`
	AttrTimeout  *int `json:"attr_timeout,omitempty"`
}

type Plugin struct {
	BasePath     string `json:"base_path"`
	DummyPlugins bool   `json:"dummy_plugins"`
}
