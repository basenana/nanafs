/*
 Copyright 2023 NanaFS Authors.

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

package friday

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/basenana/friday/core/agents"
	"github.com/basenana/friday/core/agents/research"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/nanafs/config"
	"github.com/hyponet/webpage-packer/packer"
)

var (
	BrowserlessURL   = os.Getenv("WebPackerBrowserlessURL")
	BrowserlessToken = os.Getenv("WebPackerBrowserlessToken")
)

func NewResearch(llm openai.Client, fcfg config.Friday) agents.Agent {
	return research.New(llm, research.Option{
		ResearchTools: initResearchTools(fcfg),
	})
}

func initResearchTools(fcfg config.Friday) []*tools.Tool {
	return []*tools.Tool{
		tools.NewTool(
			"crawl_webpages",
			tools.WithDescription("You can get the content of a webpage just by knowing its URL."),
			tools.WithArray("url_list",
				tools.Required(),
				tools.Items(map[string]interface{}{"type": "string", "description": "The exact url address you want to view，Do not make up addresses."}),
				tools.Description("The urls need to be crawled, If you don't know the exact address, use a search engine FIRST."),
			),
			tools.WithToolHandler(crawlWebpagesHandler),
		),
		tools.NewTool(
			"web_search",
			tools.WithDescription("Use this tool to search the Internet. Inappropriate searches will yield a vast amount of useless information; carefully craft your queries and use the fewest possible searches possible."),
			tools.WithString("query",
				tools.Required(),
				tools.Description("The search query, should employ search techniques and adhere to search engine syntax"),
			),
			tools.WithString("time_range",
				tools.Required(),
				tools.Enum("day", "week", "month", "year", "anytime"),
				tools.Description("The time range you want to search, (this) day/week/month/year, default: anytime"),
			),
			tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
				query, ok := request.Arguments["query"].(string)
				if !ok || query == "" {
					return tools.NewToolResultError("missing required parameter: query"), nil
				}

				// time_range is parsed but not used as Jina Search API doesn't support date filtering
				_, _ = request.Arguments["time_range"]

				if fcfg.JinaAPIKey == "" {
					return tools.NewToolResultError("Jina API key not configured"), nil
				}

				results, err := jinaSearch(ctx, query, fcfg.JinaAPIKey, 10)
				if err != nil {
					return tools.NewToolResultError(err.Error()), nil
				}

				return tools.NewToolResultText(tools.Res2Str(results)), nil
			}),
		),
	}
}

type jinaSearchResponse struct {
	Code   int              `json:"code"`
	Status int              `json:"status"`
	Data   []jinaSearchItem `json:"data"`
}

type jinaSearchItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Content     string `json:"content"`
}

func jinaSearch(ctx context.Context, query, apiKey string, num int) ([]WebSearchItem, error) {
	reqBody := map[string]interface{}{
		"q":   query,
		"num": num,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://s.jina.ai/", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jinaResp jinaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&jinaResp); err != nil {
		return nil, err
	}

	results := make([]WebSearchItem, 0, len(jinaResp.Data))
	for _, item := range jinaResp.Data {
		results = append(results, WebSearchItem{
			Title:   item.Title,
			Content: item.Description,
			Site:    item.URL,
			URL:     item.URL,
		})
	}

	return results, nil
}

func crawlWebpagesHandler(ctx context.Context, request *tools.Request) (*tools.Result, error) {
	urlList, ok := request.Arguments["url_list"].([]any)
	if !ok || len(urlList) == 0 {
		return tools.NewToolResultError("missing required parameter: url_list"), nil
	}

	var (
		result = make(chan WebContent, len(urlList))
		wg     = sync.WaitGroup{}
		p      = packer.NewHtmlPacker()
		bc     *packer.Browserless
	)

	if BrowserlessURL != "" {
		bc = &packer.Browserless{
			Endpoint:    BrowserlessURL,
			Token:       BrowserlessToken,
			StealthMode: true,
			BlockADS:    true,
		}
	}

	for _, urlItem := range urlList {
		wg.Add(1)

		go func(u string) {
			defer wg.Done()
			content, err := p.ReadContent(ctx, packer.Option{
				URL:              u,
				Timeout:          60,
				ClutterFree:      true,
				Browserless:      bc,
				EnablePrivateNet: true,
			})

			if err != nil {
				result <- WebContent{URL: u, Error: err.Error()}
				return
			}

			markdown, err := htmltomarkdown.ConvertString(content)
			if err != nil {
				result <- WebContent{URL: u, Content: content}
				return
			}
			result <- WebContent{URL: u, Content: markdown}

		}(urlItem.(string))

	}
	wg.Wait()
	close(result)

	contents := make([]WebContent, 0, len(urlList))
	for content := range result {
		contents = append(contents, content)
	}

	return tools.NewToolResultText(tools.Res2Str(contents)), nil
}

type WebContent struct {
	URL     string `json:"url"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

type WebSearchItem struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Site    string `json:"site,omitempty"`
	URL     string `json:"url"`
}
