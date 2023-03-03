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

type verifier func(config *Config) error

var verifiers = []verifier{
	setDefaultValue,
}

func setDefaultValue(config *Config) error {
	if config.Owner == nil {
		config.Owner = defaultOwner()
	}
	return nil
}

func Verify(cfg *Config) error {
	for _, f := range verifiers {
		if err := f(cfg); err != nil {
			return err
		}
	}
	return nil
}
