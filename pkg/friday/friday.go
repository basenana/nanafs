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

package friday

import (
	"github.com/basenana/friday/config"
	"github.com/basenana/friday/pkg/friday"
	"github.com/basenana/friday/pkg/llm/prompts"
	"github.com/basenana/friday/pkg/models"
)

var (
	fridayClient *friday.Friday
)

func InitFriday(cfg *config.Config) (err error) {
	fridayClient, err = friday.NewFriday(cfg)
	return
}

func InitFridayFromConfig() (err error) {
	loader := config.NewConfigLoader()
	cfg, err := loader.GetConfig()
	if err != nil {
		return err
	}

	return InitFriday(&cfg)
}

func IngestFile(fileName, content string) error {
	file := models.File{
		Source:  fileName,
		Content: content,
	}
	return fridayClient.IngestFromFile(file)
}

func Question(q string) (answer string, err error) {
	p := prompts.NewQuestionPrompt()
	return fridayClient.Question(p, q)
}
