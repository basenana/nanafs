package agentic

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/plugin/utils"
	"github.com/hyponet/webpage-packer/packer"
	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/option"
)

var (
	BrowserlessURL   = os.Getenv("BROWSERLESS_URL")
	BrowserlessToken = os.Getenv("BROWSERLESS_TOKEN")
)

// NewPSEWebSearchTool https://programmablesearchengine.google.com/
func NewPSEWebSearchTool(engineID, apiKey string) []*tools.Tool {
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
			tools.WithToolHandler(pseSearchHandler(engineID, apiKey)),
		),
	}
}

func pseSearchHandler(engineID, apiKey string) func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
	return func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
		query, ok := request.Arguments["query"].(string)
		if !ok || query == "" {
			return tools.NewToolResultError("missing required parameter: query"), nil
		}

		var (
			dateRestrict string
			results      []WebSearchItem
		)

		dateRaw, ok := request.Arguments["time_range"]
		if ok {
			switch dateRaw.(string) {
			case "day":
				dateRestrict = "d1"
			case "week":
				dateRestrict = "w1"
			case "month":
				dateRestrict = "m1"
			case "year":
				dateRestrict = "y1"
			default:
				dateRestrict = "" // anytime
			}
		}

		tp := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
			proxyUrl, err := url.Parse(proxy)
			if err == nil {
				tp.Proxy = http.ProxyURL(proxyUrl)
			}
		}
		cli := &http.Client{
			Transport: &transport.APIKey{Key: apiKey, Transport: tp},
			Timeout:   time.Minute,
		}

		svc, err := customsearch.NewService(ctx, option.WithHTTPClient(cli))
		if err != nil {
			return tools.NewToolResultError(err.Error()), nil
		}

		doQuery := svc.Cse.List().Cx(engineID).Q(query)
		if dateRestrict != "" {
			doQuery = doQuery.DateRestrict(dateRestrict)
		}

		resp, err := doQuery.Num(10).Do()
		if err != nil {
			return tools.NewToolResultError(err.Error()), nil
		}

		for _, item := range resp.Items {
			results = append(results, WebSearchItem{
				Title:   item.Title,
				Content: item.Snippet,
				Site:    item.DisplayLink,
				URL:     item.Link,
			})
		}

		return tools.NewToolResultText(tools.Res2Str(results)), nil
	}
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
		if len([]rune(content.Content)) > 1000 {
			n, err := request.Scratchpad.WriteNote(ctx, &tools.ScratchpadNote{
				ID:      fmt.Sprintf("webpage-%s", utils.ComputeStructHash(content.URL, nil)),
				Title:   fmt.Sprintf("the content of %s", content.URL),
				Content: content.Content,
			})
			if err != nil {
				return nil, err
			}
			content.Content = fmt.Sprintf("The content of web page %s was successfully saved in the scratchpad, note id: %s. "+
				"Use tools to retrieve the original text if needed.", content.URL, n.ID)
		}
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
