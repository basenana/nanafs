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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"

	"github.com/basenana/friday/config"
	"github.com/basenana/friday/pkg/llm"
	"github.com/basenana/friday/pkg/utils/logger"
)

const (
	defaultQueryPerMinute           = 3
	defaultBurst                    = 5
	defaultTemperature      float32 = 0.7
	defaultFrequencyPenalty uint    = 0
	defaultPresencePenalty  uint    = 0
	defaultMaxReturnToken           = 1024
	defaultModel                    = "gpt-3.5-turbo"
)

type OpenAIV1 struct {
	log logger.Logger

	baseUri string
	key     string

	limiter *rate.Limiter

	conf config.OpenAIConfig
}

func NewOpenAIV1(log logger.Logger, baseUrl, key string, conf config.OpenAIConfig) *OpenAIV1 {
	if conf.QueryPerMinute <= 0 {
		conf.QueryPerMinute = defaultQueryPerMinute
	}
	if conf.Burst <= 0 {
		conf.Burst = defaultBurst
	}
	defaultTemp := defaultTemperature
	if conf.Temperature == nil {
		conf.Temperature = &defaultTemp
	}
	defaultMRT := defaultMaxReturnToken
	if conf.MaxReturnToken == nil {
		conf.MaxReturnToken = &defaultMRT
	}
	defaultMl := defaultModel
	if conf.Model == nil {
		conf.Model = &defaultMl
	}
	defaultFreqPenalty := defaultFrequencyPenalty
	if conf.FrequencyPenalty == nil {
		conf.FrequencyPenalty = &defaultFreqPenalty
	}
	defaultPrePenalty := defaultPresencePenalty
	if conf.PresencePenalty == nil {
		conf.PresencePenalty = &defaultPrePenalty
	}

	limiter := rate.NewLimiter(rate.Limit(conf.QueryPerMinute), conf.Burst*60)

	return &OpenAIV1{
		log:     log,
		baseUri: baseUrl,
		key:     key,
		limiter: limiter,
		conf:    conf,
	}
}

var _ llm.LLM = &OpenAIV1{}

func (o *OpenAIV1) request(ctx context.Context, stream bool, path string, method string, data map[string]any, res chan<- []byte) error {
	jsonData, _ := json.Marshal(data)

	maxRetry := 100
	for i := 0; i < maxRetry; i++ {
		body := bytes.NewBuffer(jsonData)
		select {
		case <-ctx.Done():
			return fmt.Errorf("openai request context cancelled")
		default:
			err := o.limiter.WaitN(ctx, 60)
			if err != nil {
				return err
			}

			uri, err := url.JoinPath(o.baseUri, path)
			if err != nil {
				return err
			}
			req, err := http.NewRequest(method, uri, body)
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.key))

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				o.log.Debugf("fail to call openai, sleep 30s and retry. err: %v", err)
				time.Sleep(time.Second * 30)
				continue
			}

			if resp.StatusCode != 200 {
				respBody, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()

				o.log.Debugf("fail to call openai, sleep 30s and retry. status code error: %d, resp body: %s", resp.StatusCode, string(respBody))
				time.Sleep(time.Second * 30)
				continue
			}

			reader := bufio.NewReader(resp.Body)
			if stream {
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil && err != io.EOF {
						o.log.Error(err)
						return err
					}
					if err == io.EOF {
						break
					}
					select {
					case <-ctx.Done():
						return fmt.Errorf("context timeout in openai stream client")
					case res <- line:
						continue
					}
				}
				return nil
			}

			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			select {
			case <-ctx.Done():
				return fmt.Errorf("context timeout in openai client")
			case res <- respBody:
				return nil
			}
		}
	}
	return fmt.Errorf("fail to call openai after retry 100 times")
}
