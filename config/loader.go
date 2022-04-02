package config

import (
	"encoding/json"
	"fmt"
	"os"
)

var FilePath string

type Loader interface {
	GetConfig() (Config, error)
}

type localLoader struct{}

func (l localLoader) GetConfig() (Config, error) {
	result := Config{}

	if FilePath == "" {
		return result, fmt.Errorf("--config not set")
	}

	_, err := os.Stat(FilePath)
	if err != nil {
		return result, fmt.Errorf("open config file failed: %s", err.Error())
	}

	f, err := os.Open(FilePath)
	if err != nil {
		return result, fmt.Errorf("open config file failed: %s", err.Error())
	}
	defer f.Close()

	jd := json.NewDecoder(f)
	if err = jd.Decode(&result); err != nil {
		return result, fmt.Errorf("parse config failed: %s", err.Error())
	}
	return result, nil
}

func NewConfigLoader() Loader {
	return localLoader{}
}
