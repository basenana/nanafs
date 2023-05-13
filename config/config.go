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
	Api    Api     `json:"api"`
	FUSE   FUSE    `json:"fuse"`
	Webdav *Webdav `json:"webdav,omitempty"`

	Meta             Meta       `json:"meta"`
	Storages         []Storage  `json:"storages"`
	GlobalEncryption Encryption `json:"global_encryption"`

	Workflow Workflow `json:"workflow"`
	Plugin   *Plugin  `json:"plugin,omitempty"`
	FS       *FS      `json:"fs,omitempty"`
	Metric   *Metric  `json:"metric,omitempty"`

	CacheDir  string `json:"cache_dir,omitempty"`
	CacheSize int    `json:"cache_size,omitempty"`
	Debug     bool   `json:"debug,omitempty"`
}

type Api struct {
	Enable bool   `json:"enable"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Pprof  bool   `json:"pprof"`
}

type Webdav struct {
	Enable         bool            `json:"enable"`
	Host           string          `json:"host"`
	Port           int             `json:"port"`
	OverwriteUsers []OverwriteUser `json:"overwrite_users"`
}

type FUSE struct {
	Enable       bool     `json:"enable"`
	RootPath     string   `json:"root_path"`
	MountOptions []string `json:"mount_options,omitempty"`
	DisplayName  string   `json:"display_name,omitempty"`
	VerboseLog   bool     `json:"verbose_log,omitempty"`

	EntryTimeout *int `json:"entry_timeout,omitempty"`
	AttrTimeout  *int `json:"attr_timeout,omitempty"`
}

type Encryption struct {
	Enable    bool   `json:"enable"`
	Method    string `json:"method"`
	SecretKey string `json:"secret_key"`
}

type OverwriteUser struct {
	UID      int64  `json:"uid"`
	GID      int64  `json:"gid"`
	Username string `json:"username"`
	Password string `json:"password"`
}
