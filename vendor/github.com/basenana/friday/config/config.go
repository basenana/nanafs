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
	"github.com/basenana/friday/pkg/utils/logger"
)

type Config struct {
	Debug  bool `json:"debug,omitempty"`
	Logger logger.Logger

	// llm limit token
	LimitToken int `json:"limit_token,omitempty"`

	// openai key
	OpenAIBaseUrl string `json:"open_ai_base_url,omitempty"` // if openai is used for embedding or llm, it is needed, default is "https://api.openai.com"
	OpenAIKey     string `json:"open_ai_key,omitempty"`      // if openai is used for embedding or llm, it is needed

	// embedding config
	EmbeddingConfig EmbeddingConfig `json:"embedding_config,omitempty"`

	// vector store config
	VectorStoreConfig VectorStoreConfig `json:"vector_store_config,omitempty"`

	// LLM
	LLMConfig LLMConfig `json:"llm_config,omitempty"`

	// text spliter
	TextSpliterConfig TextSpliterConfig `json:"text_spliter_config,omitempty"`
}

type LLMConfig struct {
	LLMType LLMType           `json:"llm_type"`
	Prompts map[string]string `json:"prompts,omitempty"`
	OpenAI  OpenAIConfig      `json:"openai,omitempty"`
	GLM6B   GLM6BConfig       `json:"glm6b,omitempty"`
}

type GLM6BConfig struct {
	Url string `json:"url,omitempty"`
}

type OpenAIConfig struct {
	QueryPerMinute   int      `json:"query_per_minute,omitempty"` // qpm, default is 3
	Burst            int      `json:"burst,omitempty"`            // burst, default is 5
	Model            *string  `json:"model,omitempty"`            // model of openai, default for llm is "gpt-3.5-turbo"; default for embedding is "text-embedding-ada-002"
	MaxReturnToken   *int     `json:"max_return_token,omitempty"`
	FrequencyPenalty *uint    `json:"frequency_penalty,omitempty"`
	PresencePenalty  *uint    `json:"presence_penalty,omitempty"`
	Temperature      *float32 `json:"temperature,omitempty"`
}

type EmbeddingConfig struct {
	EmbeddingType  EmbeddingType `json:"embedding_type"`
	EmbeddingUrl   string        `json:"embedding_url,omitempty"`   // only needed for huggingface
	EmbeddingModel string        `json:"embedding_model,omitempty"` // only needed for huggingface
}

type VectorStoreConfig struct {
	VectorStoreType VectorStoreType `json:"vector_store_type"`
	VectorUrl       string          `json:"vector_url"`
	EmbeddingDim    int             `json:"embedding_dim,omitempty"` // embedding dimension, default is 1536
}

type TextSpliterConfig struct {
	SpliterChunkSize    int    `json:"spliter_chunk_size,omitempty"`    // chunk of files splited to store, default is 4000
	SpliterChunkOverlap int    `json:"spliter_chunk_overlap,omitempty"` // overlap of each chunks, default is 200
	SpliterSeparator    string `json:"spliter_separator,omitempty"`     // separator to split files, default is \n
}

type LLMType string

const (
	LLMGLM6B  LLMType = "glm-6b"
	LLMOpenAI LLMType = "openai"
)

type EmbeddingType string

const (
	EmbeddingOpenAI      EmbeddingType = "openai"
	EmbeddingHuggingFace EmbeddingType = "huggingface"
)

type VectorStoreType string

const (
	VectorStoreRedis    VectorStoreType = "redis"
	VectorStorePostgres VectorStoreType = "postgres"
)
