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
	"encoding/json"
	"fmt"
)

type EmbeddingResult struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

func (g *Gemini) Embedding(ctx context.Context, doc string) (*EmbeddingResult, error) {
	model := "embedding-001"
	path := fmt.Sprintf("v1beta/models/%s:embedContent", model)

	data := map[string]interface{}{
		"model": fmt.Sprintf("models/%s", model),
		"content": map[string]any{
			"parts": []map[string]string{{
				"text": doc,
			}},
		},
	}

	var (
		buf = make(chan []byte)
		err error
	)
	go func() {
		defer close(buf)
		err = g.request(ctx, false, path, "POST", data, buf)
	}()
	if err != nil {
		return nil, err
	}

	var res EmbeddingResult
	respBody := <-buf
	err = json.Unmarshal(respBody, &res)
	return &res, err
}
