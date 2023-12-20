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
	conf    config.GeminiConfig
	limiter *rate.Limiter
}

func NewGemini(log logger.Logger, conf config.GeminiConfig) *Gemini {
	baseUrl := "https://generativelanguage.googleapis.com"
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
		conf:    conf,
		limiter: limiter,
	}
}

func (g *Gemini) request(ctx context.Context, path string, method string, data map[string]any) ([]byte, error) {
	jsonData, _ := json.Marshal(data)

	maxRetry := 100
	for i := 0; i < maxRetry; i++ {
		body := bytes.NewBuffer(jsonData)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("gemini request context cancelled")
		default:
			err := g.limiter.WaitN(ctx, 60)
			if err != nil {
				return nil, err
			}

			uri, err := url.JoinPath(g.baseUri, path)
			if err != nil {
				return nil, err
			}
			url := fmt.Sprintf("%s?key=%s", uri, g.conf.Key)
			req, err := http.NewRequest(method, url, body)
			req.Header.Set("Content-Type", "application/json; charset=utf-8")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				g.log.Debugf("fail to call gemini, sleep 1s and retry. err: %v", err)
				time.Sleep(time.Second)
				continue
			}

			// Read Response Body
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode != 200 {
				g.log.Debugf("fail to call gemini, sleep 1s and retry. status code error: %d, resp body: %s", resp.StatusCode, string(respBody))
				time.Sleep(time.Second)
				continue
			}
			g.log.Debugf("gemini response: %s", respBody)
			return respBody, nil
		}
	}
	return nil, fmt.Errorf("fail to call gemini after retry 100 times")
}
