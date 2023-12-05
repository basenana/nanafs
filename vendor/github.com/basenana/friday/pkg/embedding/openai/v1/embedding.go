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

package v1

import (
	"context"

	"github.com/basenana/friday/config"
	"github.com/basenana/friday/pkg/embedding"
	"github.com/basenana/friday/pkg/llm/client/openai/v1"
	"github.com/basenana/friday/pkg/utils/logger"
)

type OpenAIEmbedding struct {
	*v1.OpenAIV1
}

var _ embedding.Embedding = &OpenAIEmbedding{}

func NewOpenAIEmbedding(log logger.Logger, baseUrl, key string, conf config.OpenAIConfig) embedding.Embedding {
	return &OpenAIEmbedding{
		OpenAIV1: v1.NewOpenAIV1(log, baseUrl, key, conf),
	}
}

func (o *OpenAIEmbedding) VectorQuery(ctx context.Context, doc string) ([]float32, map[string]interface{}, error) {
	res, err := o.Embedding(ctx, doc)
	if err != nil {
		return nil, nil, err
	}
	usage := res.Usage
	metadata := make(map[string]interface{})
	metadata["prompt_tokens"] = usage.PromptTokens
	metadata["total_tokens"] = usage.TotalTokens

	return res.Data[0].Embedding, metadata, nil
}

func (o *OpenAIEmbedding) VectorDocs(ctx context.Context, docs []string) ([][]float32, []map[string]interface{}, error) {
	res := make([][]float32, len(docs))
	metadata := make([]map[string]interface{}, len(docs))

	for i, doc := range docs {
		r, err := o.Embedding(ctx, doc)
		if err != nil {
			return nil, nil, err
		}
		res[i] = r.Data[0].Embedding
		metadata[i] = map[string]interface{}{
			"prompt_tokens": r.Usage.PromptTokens,
			"total_tokens":  r.Usage.TotalTokens,
		}
	}
	return res, metadata, nil
}
