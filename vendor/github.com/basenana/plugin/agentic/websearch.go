package agentic

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"os"
	"path"
	"sync"
	"time"

	"github.com/basenana/friday/core/tools"
	"github.com/basenana/plugin/web"
	"go.uber.org/zap"
	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/option"
)

// NewPSEWebSearchTool https://programmablesearchengine.google.com/
func NewPSEWebSearchTool(engineID, apiKey string, wc *WebCitations, toolLogger *zap.SugaredLogger) []*tools.Tool {
	return []*tools.Tool{
		tools.NewTool(
			"crawl_webpages",
			tools.WithDescription("You can get the content of a webpage just by knowing its URL."),
			tools.WithArray("url_list",
				tools.Required(),
				tools.Items(map[string]interface{}{"type": "string", "description": "The exact url address you want to view，Do not make up addresses."}),
				tools.Description("The urls need to be crawled, If you don't know the exact address, use a search engine FIRST."),
			),
			tools.WithToolHandler(crawlWebpagesHandler(wc, toolLogger)),
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
			tools.WithToolHandler(pseSearchHandler(toolLogger, engineID, apiKey)),
		),
	}
}

func pseSearchHandler(toolLogger *zap.SugaredLogger, engineID, apiKey string) func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
	return func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
		query, ok := request.Arguments["query"].(string)
		if !ok || query == "" {
			toolLogger.Warnw("missing required parameter: query")
			return tools.NewToolResultError("missing required parameter: query"), nil
		}

		var (
			dateRestrict string
			results      []WebSearchItem
		)

		dateRaw, ok := request.Arguments["time_range"]
		if ok {
			dateStr, ok := dateRaw.(string)
			if ok {
				switch dateStr {
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
		}

		toolLogger.Infow("web_search started", "query", query, "time_range", dateRaw)

		tp := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		if proxy := os.Getenv("GOOGLE_PROXY"); proxy != "" {
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
			toolLogger.Warnw("create search service failed", "error", err)
			return tools.NewToolResultError(err.Error()), nil
		}

		doQuery := svc.Cse.List().Cx(engineID).Q(query)
		if dateRestrict != "" {
			doQuery = doQuery.DateRestrict(dateRestrict)
		}

		resp, err := doQuery.Num(10).Do()
		if err != nil {
			toolLogger.Warnw("search query failed", "error", err)
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

		toolLogger.Infow("web_search completed", "results_count", len(results))
		return tools.NewToolResultText(tools.Res2Str(results)), nil
	}
}

func crawlWebpagesHandler(wc *WebCitations, toolLogger *zap.SugaredLogger) func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
	return func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
		urlList, ok := request.Arguments["url_list"].([]any)
		if !ok || len(urlList) == 0 {
			toolLogger.Warnw("missing required parameter: url_list")
			return tools.NewToolResultError("missing required parameter: url_list"), nil
		}

		toolLogger.Infow("crawl_webpages started", "url_count", len(urlList))

		var (
			result       = make(chan WebContent, len(urlList))
			wg           = sync.WaitGroup{}
			successCount = 0
			errorCount   = 0
		)

		for _, urlItem := range urlList {
			urlStr, ok := urlItem.(string)
			if !ok {
				errorCount++
				continue
			}
			wg.Add(1)

			go func(u string) {
				defer wg.Done()
				toolLogger.Debugw("crawling url", "url", u)

				filePath, err := web.PackFromURL(ctx, "", u, "html", wc.workdir, true)
				if err != nil {
					toolLogger.Warnw("crawl url failed", "url", u, "error", err)
					result <- WebContent{URL: u, Error: err.Error()}
					return
				}

				toolLogger.Debugw("crawl url completed", "url", u, "file", path.Base(filePath))
				result <- WebContent{URL: u, FilePath: filePath}

			}(urlStr)
		}
		wg.Wait()
		close(result)

		contents := make([]WebContent, 0, len(urlList))
		for content := range result {
			if content.Error != "" {
				errorCount++
			} else {
				wc.files = append(wc.files, WebFile{
					Filepath: content.FilePath,
					URL:      content.URL,
				})
				content.FilePath = path.Base(content.FilePath) // remove workdir
				successCount++
			}
			contents = append(contents, content)
		}

		toolLogger.Infow("crawl_webpages completed", "success_count", successCount, "error_count", errorCount)
		return tools.NewToolResultText(tools.Res2Str(contents)), nil
	}
}

type WebContent struct {
	URL      string `json:"url"`
	FilePath string `json:"file_path"`
	Error    string `json:"error,omitempty"`
}

type WebSearchItem struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Site    string `json:"site,omitempty"`
	URL     string `json:"url"`
}

type WebCitations struct {
	workdir string
	files   []WebFile
}

func newWebCitations(workdir string) *WebCitations {
	return &WebCitations{workdir: workdir, files: make([]WebFile, 0)}
}

type WebFile struct {
	Filepath string `json:"file_path"`
	URL      string `json:"url"`
}
