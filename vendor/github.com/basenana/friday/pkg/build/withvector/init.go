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

package withvector

import (
	"context"

	"github.com/basenana/friday/config"
	"github.com/basenana/friday/pkg/embedding"
	geminiembedding "github.com/basenana/friday/pkg/embedding/gemini"
	huggingfaceembedding "github.com/basenana/friday/pkg/embedding/huggingface"
	openaiembedding "github.com/basenana/friday/pkg/embedding/openai/v1"
	"github.com/basenana/friday/pkg/friday"
	"github.com/basenana/friday/pkg/llm"
	"github.com/basenana/friday/pkg/llm/client/gemini"
	glm_6b "github.com/basenana/friday/pkg/llm/client/glm-6b"
	openaiv1 "github.com/basenana/friday/pkg/llm/client/openai/v1"
	"github.com/basenana/friday/pkg/spliter"
	"github.com/basenana/friday/pkg/utils/logger"
	"github.com/basenana/friday/pkg/vectorstore"
)

func NewFridayWithVector(conf *config.Config, vectorClient vectorstore.VectorStore) (f *friday.Friday, err error) {
	log := conf.Logger
	if conf.Logger == nil {
		log = logger.NewLogger("friday")
	}
	log.SetDebug(conf.Debug)

	var (
		llmClient      llm.LLM
		embeddingModel embedding.Embedding
		prompts        = make(map[string]string)
	)
	// init LLM client
	if conf.LLMConfig.LLMType == config.LLMOpenAI {
		if conf.OpenAIBaseUrl == "" {
			conf.OpenAIBaseUrl = "https://api.openai.com"
		}
		llmClient = openaiv1.NewOpenAIV1(log, conf.OpenAIBaseUrl, conf.OpenAIKey, conf.LLMConfig.OpenAI)
	}
	if conf.LLMConfig.LLMType == config.LLMGLM6B {
		llmClient = glm_6b.NewGLM(log, conf.LLMConfig.GLM6B.Url)
	}
	if conf.LLMConfig.LLMType == config.LLMGemini {
		llmClient = gemini.NewGemini(log, conf.GeminiBaseUri, conf.GeminiKey, conf.LLMConfig.Gemini)
	}

	if conf.LLMConfig.Prompts != nil {
		prompts = conf.LLMConfig.Prompts
	}

	// init embedding client
	if conf.EmbeddingConfig.EmbeddingType == config.EmbeddingOpenAI {
		if conf.OpenAIBaseUrl == "" {
			conf.OpenAIBaseUrl = "https://api.openai.com"
		}
		embeddingModel = openaiembedding.NewOpenAIEmbedding(log, conf.OpenAIBaseUrl, conf.OpenAIKey, conf.EmbeddingConfig.OpenAI)
	}
	if conf.EmbeddingConfig.EmbeddingType == config.EmbeddingHuggingFace {
		embeddingModel = huggingfaceembedding.NewHuggingFace(log, conf.EmbeddingConfig.HuggingFace)
		testEmbed, _, err := embeddingModel.VectorQuery(context.TODO(), "test")
		if err != nil {
			return nil, err
		}
		conf.VectorStoreConfig.EmbeddingDim = len(testEmbed)
	}
	if conf.EmbeddingConfig.EmbeddingType == config.EmbeddingGemini {
		embeddingModel = geminiembedding.NewGeminiEmbedding(log, conf.GeminiBaseUri, conf.GeminiKey, conf.EmbeddingConfig.Gemini)
	}

	defaultVectorTopK := friday.DefaultTopK
	if conf.VectorStoreConfig.TopK == nil {
		conf.VectorStoreConfig.TopK = &defaultVectorTopK
	}

	// init text spliter
	chunkSize := spliter.DefaultChunkSize
	overlapSize := spliter.DefaultChunkOverlap
	separator := "\n"
	if conf.TextSpliterConfig.SpliterChunkSize > 0 {
		chunkSize = conf.TextSpliterConfig.SpliterChunkSize
	}
	if conf.TextSpliterConfig.SpliterChunkOverlap > 0 {
		overlapSize = conf.TextSpliterConfig.SpliterChunkOverlap
	}
	if conf.TextSpliterConfig.SpliterSeparator != "" {
		separator = conf.TextSpliterConfig.SpliterSeparator
	}
	textSpliter := spliter.NewTextSpliter(log, chunkSize, overlapSize, separator)

	f = &friday.Friday{
		Log:        log,
		LimitToken: conf.LimitToken,
		LLM:        llmClient,
		Prompts:    prompts,
		Embedding:  embeddingModel,
		Vector:     vectorClient,
		VectorTopK: conf.VectorStoreConfig.TopK,
		Spliter:    textSpliter,
	}
	return
}
