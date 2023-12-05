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
	"context"
	"fmt"

	"github.com/basenana/friday/config"
	"github.com/basenana/friday/pkg/build/withvector"
	"github.com/basenana/friday/pkg/friday"
	"github.com/basenana/friday/pkg/friday/summary"
	"github.com/basenana/friday/pkg/models"
	"github.com/basenana/friday/pkg/vectorstore/postgres"
)

var (
	fridayClient *friday.Friday
)

func InitFriday(cfg *config.Config) (err error) {
	if cfg == nil {
		return nil
	}
	pgClient, err := postgres.NewPostgresClient(cfg.VectorStoreConfig.VectorUrl)
	if err != nil {
		return err
	}
	fridayClient, err = withvector.NewFridayWithVector(cfg, pgClient)
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

func IngestFile(ctx context.Context, fileName, content string) error {
	if fridayClient == nil {
		return fmt.Errorf("fridayClient is nil, can not use it")
	}
	file := models.File{
		Source:  fileName,
		Content: content,
	}
	return fridayClient.IngestFromFile(ctx, file)
}

func Question(ctx context.Context, q string) (answer string, err error) {
	if fridayClient == nil {
		return "", fmt.Errorf("fridayClient is nil, can not use it")
	}
	return fridayClient.Question(ctx, q)
}

func SummaryFile(ctx context.Context, fileName, content string) (string, error) {
	if fridayClient == nil {
		return "", fmt.Errorf("fridayClient is nil, can not use it")
	}
	file := models.File{
		Source:  fileName,
		Content: content,
	}
	result, err := fridayClient.SummaryFromFile(ctx, file, summary.MapReduce)
	if err != nil {
		return "", err
	}
	if result == nil || result[fileName] == "" {
		return "", fmt.Errorf("fail to summary file %s", fileName)
	}
	return result[fileName], nil
}

func Keywords(ctx context.Context, content string) ([]string, error) {
	if fridayClient == nil {
		return nil, fmt.Errorf("fridayClient is nil, can not use it")
	}
	return fridayClient.Keywords(ctx, content)
}
