package config

type Config struct {
	Meta      Meta      `json:"meta"`
	Storages  []Storage `json:"storages"`
	Owner     *FsOwner  `json:"owner,omitempty"`
	CacheDir  string    `json:"cache_dir,omitempty"`
	CacheSize int64     `json:"cache_size,omitempty"`
	Debug     bool      `json:"debug"`

	ApiConfig Api `json:"api"`
	FsConfig  Fs  `json:"fs"`
}

type Meta struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type Storage struct {
	ID        string `json:"id"`
	LocalDir  string `json:"local_dir,omitempty"`
	CacheDir  string `json:"cache_dir,omitempty"`
	CacheSize int64  `json:"cache_size,omitempty"`
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
}
