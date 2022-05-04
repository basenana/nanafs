package config

import (
	"github.com/basenana/nanafs/config"
	"path"
)

const (
	defaultLocalDataDir = "local"
	defaultSqliteFile   = "sqlite.db"
)

func localConfigFilePath(local string) string {
	return path.Join(local, config.DefaultConfigBase)
}

func localDataDirPath(local string) string {
	return path.Join(local, defaultLocalDataDir)
}

func localDbFilePath(local string) string {
	return path.Join(local, defaultSqliteFile)
}
