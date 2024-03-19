/*
 Copyright 2023 Friday Author.

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

package gemini

import (
	"context"

	"github.com/basenana/friday/config"
	"github.com/basenana/friday/pkg/embedding"
	"github.com/basenana/friday/pkg/llm/client/gemini"
	"github.com/basenana/friday/pkg/utils/logger"
)

type GeminiEmbedding struct {
	*gemini.Gemini
}

func NewGeminiEmbedding(log logger.Logger, baseUrl, key string, conf config.GeminiConfig) embedding.Embedding {
	return &GeminiEmbedding{
		Gemini: gemini.NewGemini(log, baseUrl, key, conf),
	}
}

func (g *GeminiEmbedding) VectorQuery(ctx context.Context, doc string) ([]float32, map[string]interface{}, error) {
	res, err := g.Embedding(ctx, doc)
	if err != nil {
		return nil, nil, err
	}

	return res.Embedding.Values, nil, nil
}

func (g *GeminiEmbedding) VectorDocs(ctx context.Context, docs []string) ([][]float32, []map[string]interface{}, error) {
	res := make([][]float32, len(docs))

	for i, doc := range docs {
		r, err := g.Embedding(ctx, doc)
		if err != nil {
			return nil, nil, err
		}
		res[i] = r.Embedding.Values
	}

	return res, nil, nil
}
