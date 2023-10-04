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

package huggingface

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/basenana/friday/pkg/embedding"
	"github.com/basenana/friday/pkg/utils/logger"
)

type HuggingFace struct {
	log     logger.Logger
	baseUri string
	model   string
}

var _ embedding.Embedding = &HuggingFace{}

func NewHuggingFace(baseUri string, model string) embedding.Embedding {
	return &HuggingFace{
		log:     logger.NewLogger("huggingface"),
		baseUri: baseUri,
		model:   model,
	}
}

type vectorResult struct {
	Result []float32
}

type vectorResults struct {
	Result [][]float32
}

func (h HuggingFace) VectorQuery(doc string) ([]float32, map[string]interface{}, error) {
	path := "/embeddings/query"

	model := h.model
	data := map[string]string{
		"model": model,
		"text":  doc,
	}
	postBody, _ := json.Marshal(data)

	respBody, err := h.request(path, "POST", bytes.NewBuffer(postBody))
	if err != nil {
		return nil, nil, err
	}

	var res vectorResult
	err = json.Unmarshal(respBody, &res)
	return res.Result, nil, err
}

func (h HuggingFace) VectorDocs(docs []string) ([][]float32, []map[string]interface{}, error) {
	path := "/embeddings/docs"

	model := h.model
	data := map[string]interface{}{
		"model": model,
		"docs":  docs,
	}
	postBody, _ := json.Marshal(data)

	respBody, err := h.request(path, "POST", bytes.NewBuffer(postBody))
	if err != nil {
		return nil, nil, err
	}

	var res vectorResults
	err = json.Unmarshal(respBody, &res)
	return res.Result, nil, err
}

func (h HuggingFace) request(path string, method string, body io.Reader) ([]byte, error) {
	uri, err := url.JoinPath(h.baseUri, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, uri, body)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fail to call embedding server, status code error: %d", resp.StatusCode)
	}
	return respBody, nil
}
