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
	pgClient, err := postgres.NewPostgresClient(cfg.Logger, cfg.VectorStoreConfig.VectorUrl)
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

func IngestFile(ctx context.Context, entryId, parentId int64, fileName, content string) (map[string]int, error) {
	if fridayClient == nil {
		return nil, fmt.Errorf("fridayClient is nil, can not use it")
	}
	file := models.File{
		Name:     fileName,
		OID:      entryId,
		ParentId: parentId,
		Content:  content,
	}
	result := friday.IngestState{}
	f := fridayClient.WithContext(ctx).File(&file).Ingest(&result)
	return result.Tokens, f.Error
}

func Question(ctx context.Context, q string) (answer string, usage map[string]int, err error) {
	if fridayClient == nil {
		return "", nil, fmt.Errorf("fridayClient is nil, can not use it")
	}
	result := friday.ChatState{}
	f := fridayClient.WithContext(ctx).Question(q).Complete(&result)
	return result.Answer, result.Tokens, f.Error
}

func SummaryFile(ctx context.Context, fileName, content string) (string, map[string]int, error) {
	if fridayClient == nil {
		return "", nil, fmt.Errorf("fridayClient is nil, can not use it")
	}
	file := models.File{
		Name:    fileName,
		Content: content,
	}
	result := friday.SummaryState{}
	f := fridayClient.WithContext(ctx).File(&file).OfType(summary.MapReduce).Summary(&result)
	if f.Error != nil {
		return "", nil, f.Error
	}
	if result.Summary == nil || result.Summary[fileName] == "" {
		return "", nil, fmt.Errorf("fail to summary file %s", fileName)
	}
	return result.Summary[fileName], result.Tokens, nil
}

func Keywords(ctx context.Context, content string) ([]string, map[string]int, error) {
	if fridayClient == nil {
		return nil, nil, fmt.Errorf("fridayClient is nil, can not use it")
	}
	result := friday.KeywordsState{}
	f := fridayClient.WithContext(ctx).Content(content).Keywords(&result)
	return result.Keywords, result.Tokens, f.Error
}

func ChatWithEntry(ctx context.Context, entryId int64, isGroup bool, history []map[string]string, response chan map[string]string) ([]map[string]string, error) {
	if fridayClient == nil {
		return nil, fmt.Errorf("fridayClient is nil, can not use it")
	}
	result := friday.ChatState{Response: response}
	f := fridayClient.WithContext(ctx).History(history)
	if isGroup {
		f = f.SearchIn(&models.DocQuery{ParentId: entryId})
	} else {
		f = f.SearchIn(&models.DocQuery{Oid: entryId})
	}
	f = f.Chat(&result)
	return f.GetRealHistory(), f.Error
}
