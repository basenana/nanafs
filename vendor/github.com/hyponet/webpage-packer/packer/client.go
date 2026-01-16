package packer

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.dny.dev/ssrf"
)

type WebResource struct {
	Data        []byte
	ContentType string
}

type Client interface {
	ReadMain(ctx context.Context, url string) (*WebResource, error)
	ReadResource(ctx context.Context, url string) (*WebResource, error)
}

type httpClient struct {
	cli     *http.Client
	headers map[string]string
}

func (h *httpClient) ReadMain(ctx context.Context, urlStr string) (*WebResource, error) {
	return h.httpCall(ctx, urlStr)
}

func (h *httpClient) ReadResource(ctx context.Context, urlStr string) (*WebResource, error) {
	return h.httpCall(ctx, urlStr)
}

func (h *httpClient) httpCall(ctx context.Context, urlStr string) (*WebResource, error) {
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("build request with url %s error: %s", urlStr, err)
	}
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}

	return doHttpCallWithRetry(ctx, h.cli, urlStr, req)
}

type browserlessClient struct {
	*httpClient

	endpoint    string
	token       string
	stealthMode bool
	blockAds    bool
	cli         *http.Client
	headers     map[string]string
}

func (b *browserlessClient) ReadMain(ctx context.Context, urlStr string) (*WebResource, error) {
	apiUrl, err := url.Parse(b.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse browerless endpoint error: %w", err)
	}

	v := &url.Values{}
	v.Set("blockAds", fmt.Sprintf("%v", b.blockAds))
	v.Set("launch", fmt.Sprintf(`{"stealth": %v}`, b.stealthMode))
	if b.token != "" {
		v.Set("token", b.token)
	}
	apiUrl.Path = "/content"
	apiUrl.RawQuery = v.Encode()

	data := map[string]any{
		"gotoOptions": map[string]any{
			"timeout":   120000,
			"waitUntil": "load",
		},
		"url":                 urlStr,
		"setExtraHTTPHeaders": b.headers,
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiUrl.String(), bytes.NewBuffer(raw))
	if err != nil {
		return nil, fmt.Errorf("build request with url %s error: %s", urlStr, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")

	return doHttpCallWithRetry(ctx, b.cli, urlStr, req)
}

func newClient(opt Option) Client {
	if opt.Timeout == 0 {
		opt.Timeout = 60
	}

	headers := map[string]string{"Referer": opt.URL}
	for k, v := range defaultHeaders {
		headers[k] = v
	}
	if len(opt.Headers) > 0 {
		for k, v := range opt.Headers {
			headers[k] = v
		}
	}

	hc := &httpClient{
		cli:     initHttpCli(!opt.EnablePrivateNet, time.Second*time.Duration(opt.Timeout)),
		headers: headers,
	}

	if opt.Browserless != nil {
		return &browserlessClient{
			httpClient:  hc,
			endpoint:    opt.Browserless.Endpoint,
			token:       opt.Browserless.Token,
			stealthMode: opt.Browserless.StealthMode,
			blockAds:    opt.Browserless.BlockADS,
			cli:         initHttpCli(false, time.Second*time.Duration(opt.Timeout)),
			headers:     headers,
		}
	}

	return hc
}

func initHttpCli(enableSSRF bool, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	if enableSSRF {
		dialer.Control = ssrf.New().Safe
	}

	cli := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: timeout,
	}

	return cli
}

func doHttpCallWithRetry(ctx context.Context, cli *http.Client, urlStr string, req *http.Request) (*WebResource, error) {
	var (
		resp *http.Response
		err  error
	)
	for i := 0; i < 10; i++ {
		resp, err = cli.Do(req.WithContext(ctx))
		if err == nil {
			break
		}
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			if netErr.Op == "dial" {
				return nil, fmt.Errorf("do request with url %s error: %s", urlStr, err)
			}
		}
		time.Sleep(time.Second * 5)
	}
	if err != nil {
		return nil, fmt.Errorf("do request with url %s error: %s", urlStr, err)
	}

	defer func() {
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("do request with url %s error: status code is %d %s", urlStr, resp.StatusCode, string(raw))
	}

	// for browserless
	if realCode := resp.Header.Get("X-Response-Code"); realCode != "" {
		if !strings.HasPrefix(realCode, "2") {
			return nil, fmt.Errorf("do request with url %s error: status code is %d", urlStr, resp.StatusCode)
		}
	}

	var bodyReader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		bodyReader, err = gzip.NewReader(resp.Body)
	case "deflate":
		bodyReader = flate.NewReader(resp.Body)
	default:
		bodyReader = resp.Body
	}

	// fix text/html; charset=utf-8
	var (
		contentTypeParts = strings.Split(resp.Header.Get("Content-Type"), ";")
		contentType      = resp.Header.Get("Content-Type")
	)

	if len(contentTypeParts) == 0 {
		contentType = MIMEHTML
	} else {
		contentType = strings.TrimSpace(contentTypeParts[0])
	}

	data, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("read response body with url %s error: %s", urlStr, err)
	}

	return &WebResource{Data: data, ContentType: contentType}, nil
}
