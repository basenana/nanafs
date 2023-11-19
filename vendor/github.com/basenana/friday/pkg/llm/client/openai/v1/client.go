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
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/basenana/friday/pkg/llm"
	"github.com/basenana/friday/pkg/utils/logger"
)

const defaultRateLimit = 60

type OpenAIV1 struct {
	log logger.Logger

	baseUri   string
	key       string
	rateLimit int
}

func NewOpenAIV1(baseUrl, key string, rateLimit int) *OpenAIV1 {
	if rateLimit <= 0 {
		rateLimit = defaultRateLimit
	}
	return &OpenAIV1{
		log:       logger.NewLogger("openai"),
		baseUri:   baseUrl,
		key:       key,
		rateLimit: rateLimit,
	}
}

var _ llm.LLM = &OpenAIV1{}

func (o *OpenAIV1) request(path string, method string, body io.Reader) ([]byte, error) {
	uri, err := url.JoinPath(o.baseUri, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, uri, body)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.key))

	o.log.Debugf("request: %s", uri)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read Response Body
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fail to call openai, status code error: %d, resp body: %s", resp.StatusCode, string(respBody))
	}
	//o.log.Debugf("response: %s", respBody)
	return respBody, nil
}
