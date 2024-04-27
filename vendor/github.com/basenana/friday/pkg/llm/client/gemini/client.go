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
	defaultQueryPerMinute = 30
	defaultBurst          = 10
	defaultModel          = "gemini-pro"
)

type Gemini struct {
	log     logger.Logger
	baseUri string
	key     string
	conf    config.GeminiConfig
	limiter *rate.Limiter
}

var _ llm.LLM = &Gemini{}

func NewGemini(log logger.Logger, baseUrl, key string, conf config.GeminiConfig) *Gemini {
	if baseUrl == "" {
		baseUrl = "https://generativelanguage.googleapis.com"
	}
	defaultMl := defaultModel
	if conf.Model == nil {
		conf.Model = &defaultMl
	}
	if conf.QueryPerMinute <= 0 {
		conf.QueryPerMinute = defaultQueryPerMinute
	}
	if conf.Burst <= 0 {
		conf.Burst = defaultBurst
	}
	limiter := rate.NewLimiter(rate.Limit(conf.QueryPerMinute), conf.Burst*60)
	return &Gemini{
		log:     log,
		baseUri: baseUrl,
		key:     key,
		conf:    conf,
		limiter: limiter,
	}
}

func (g *Gemini) request(ctx context.Context, stream bool, path string, method string, data map[string]any, res chan<- []byte) error {
	defer close(res)
	jsonData, _ := json.Marshal(data)

	maxRetry := 100
	for i := 0; i < maxRetry; i++ {
		body := bytes.NewBuffer(jsonData)
		select {
		case <-ctx.Done():
			return fmt.Errorf("gemini request context cancelled")
		default:
			err := g.limiter.WaitN(ctx, 60)
			if err != nil {
				return err
			}

			uri, err := url.JoinPath(g.baseUri, path)
			if err != nil {
				return err
			}
			url := fmt.Sprintf("%s?key=%s", uri, g.key)
			req, err := http.NewRequest(method, url, body)
			req.Header.Set("Content-Type", "application/json; charset=utf-8")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				g.log.Debugf("fail to call gemini, sleep 1s and retry. err: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if resp.StatusCode != 200 {
				// Read Response Body
				respBody, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				g.log.Debugf("fail to call gemini, sleep 1s and retry. status code error: %d, resp body: %s", resp.StatusCode, string(respBody))
				time.Sleep(time.Second)
				continue
			}

			reader := bufio.NewReader(resp.Body)
			if stream {
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil && err != io.EOF {
						g.log.Error(err)
						return nil
					}
					if err == io.EOF {
						break
					}
					select {
					case <-ctx.Done():
						return fmt.Errorf("context timeout in gemini stream client")
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
				return fmt.Errorf("context timeout in gemini client")
			case res <- respBody:
				return nil
			}
		}
	}
	return fmt.Errorf("fail to call gemini after retry 100 times")
}
