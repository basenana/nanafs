/*
 * Copyright 2023 friday
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

	fp := os.Getenv("FRIDAY_CONFIG")
	if fp != "" {
		FilePath = fp
	}
	if FilePath == "" {
		return result, fmt.Errorf("--config or FRIDAY_CONFIG not set")
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
