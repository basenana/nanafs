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
	EmbeddingType  EmbeddingType `json:"embedding_type"`
	EmbeddingUrl   string        `json:"embedding_url,omitempty"`   // only needed for huggingface
	EmbeddingModel string        `json:"embedding_model,omitempty"` // only needed for huggingface

	// vector store config
	VectorStoreType VectorStoreType `json:"vector_store_type"`
	VectorUrl       string          `json:"vector_url"`
	EmbeddingDim    int             `json:"embedding_dim,omitempty"` // embedding dimension, default is 1536

	// LLM
	LLMType           LLMType `json:"llm_type"`
	LLMUrl            string  `json:"llm_url,omitempty"`              // only needed for glm-6b
	LLMQueryPerMinute int     `json:"llm_query_per_minute,omitempty"` // only needed for openai, qpm, default is 3
	LLMBurst          int     `json:"llm_burst,omitempty"`            // only needed for openai, burst, default is 5

	// text spliter
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
