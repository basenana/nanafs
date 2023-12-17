package packer

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"howett.net/plist"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type WebArchive struct {
	WebMainResource WebResourceItem   `json:"WebMainResource"`
	WebSubresources []WebResourceItem `json:"WebSubresources"`
}

type WebResourceItem struct {
	WebResourceURL              string `json:"WebResourceURL"`
	WebResourceMIMEType         string `json:"WebResourceMIMEType"`
	WebResourceResponse         []byte `json:"WebResourceResponse,omitempty"`
	WebResourceData             []byte `json:"WebResourceData,omitempty"`
	WebResourceTextEncodingName string `json:"WebResourceTextEncodingName,omitempty"`
}

type webArchiver struct {
	workerQ  chan string
	resource *WebArchive
	seen     map[string]struct{}
	parallel int
	mux      sync.Mutex
}

func (w *webArchiver) Pack(ctx context.Context, opt Option) error {
	if opt.URL == "" {
		return fmt.Errorf("url is empty")
	}
	if opt.FilePath == "" {
		return fmt.Errorf("file path is empty")
	}

	w.workerQ <- opt.URL

	wg := sync.WaitGroup{}
	errCh := make(chan error, 1)
	for i := 0; i < w.parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.workerRun(ctx, opt, errCh)
		}()
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		w.resource.WebMainResource = w.resource.WebSubresources[0]
	}

	if opt.ClutterFree {
		err := makeClutterFree(w.resource)
		if err != nil {
			return fmt.Errorf("make clustter free failed: %s", err)
		}
	}

	output, err := os.OpenFile(opt.FilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0655)
	if err != nil {
		return fmt.Errorf("open output file failed: %s", err)
	}
	defer output.Close()

	encoder := plist.NewBinaryEncoder(output)
	err = encoder.Encode(w.resource)
	if err != nil {
		return fmt.Errorf("encode plist file error: %s", err)
	}

	return nil
}

func (w *webArchiver) workerRun(ctx context.Context, opt Option, errCh chan error) {
	cli, headers := newHttpClient(opt)
	for {
		select {
		case urlStr, ok := <-w.workerQ:
			if !ok {
				return
			}
			w.mux.Lock()
			_, seen := w.seen[urlStr]
			w.mux.Unlock()
			if !seen {
				w.mux.Lock()
				w.seen[urlStr] = struct{}{}
				w.mux.Unlock()
				if err := w.loadWebPageFromUrl(ctx, cli, headers, urlStr, opt); err != nil {
					if len(w.resource.WebSubresources) == 0 {
						select {
						case errCh <- err:
						default:

						}
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *webArchiver) ReadContent(ctx context.Context, opt Option) (string, error) {
	if opt.FilePath != "" {
		f, err := os.OpenFile(opt.FilePath, os.O_RDONLY, 0655)
		if err != nil {
			return "", fmt.Errorf("open %s failed: %s", opt.FilePath, err)
		}
		defer f.Close()

		d := plist.NewDecoder(f)
		err = d.Decode(w.resource)
		if err != nil {
			return "", fmt.Errorf("load webarchive %s failed: %s", opt.FilePath, err)
		}
	} else {
		go func() {
			for _ = range w.workerQ {
				// discard next url
			}
		}()

		cli, headers := newHttpClient(opt)
		if err := w.loadWebPageFromUrl(ctx, cli, headers, opt.URL, opt); err != nil {
			return "", err
		}
		if len(w.resource.WebSubresources) == 0 {
			return "", fmt.Errorf("no web subresource found")
		}
		w.resource.WebMainResource = w.resource.WebSubresources[0]
	}

	content := string(w.resource.WebMainResource.WebResourceData)
	if opt.ClutterFree {
		return htmlContentClutterFree(opt.URL, content)
	}
	return content, nil
}

func (w *webArchiver) loadWebPageFromUrl(ctx context.Context, cli *http.Client, headers map[string]string, urlStr string, opt Option) error {
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return fmt.Errorf("build request with url %s error: %s", urlStr, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	var resp *http.Response
	for i := 0; i < 10; i++ {
		resp, err = cli.Do(req.WithContext(ctx))
		if err == nil {
			break
		}
		time.Sleep(time.Second * 5)
	}
	if err != nil {
		return fmt.Errorf("do request with url %s error: %s", urlStr, err)
	}

	defer func() {
		_, _ = ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("do request with url %s error: status code is %d", urlStr, resp.StatusCode)
	}

	var bodyReader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		bodyReader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("new gzip reader failed: %w", err)
		}
	case "deflate":
		bodyReader = flate.NewReader(resp.Body)
	default:
		bodyReader = resp.Body
	}

	// fix text/html; charset=utf-8
	contentTypeParts := strings.Split(resp.Header.Get("Content-Type"), ";")
	contentType := strings.TrimSpace(contentTypeParts[0])

	if strings.Contains(contentType, MIMEHTML) {
		rawReader := bodyReader
		bodyReader, err = charset.NewReader(rawReader, resp.Header.Get("Content-Type"))
		if err != nil {
			return fmt.Errorf("new charset reader with url %s error: %s", urlStr, err)
		}
	}

	data, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		return fmt.Errorf("read response body with url %s error: %s", urlStr, err)
	}
	item := WebResourceItem{
		WebResourceURL:      urlStr,
		WebResourceMIMEType: contentType,
		WebResourceData:     data,
	}

	w.mux.Lock()
	w.resource.WebSubresources = append(w.resource.WebSubresources, item)
	w.mux.Unlock()

	switch {
	case strings.Contains(contentType, MIMEHTML):
		if opt.ClutterFree {
			clutterFreeData, err := htmlContentClutterFree(urlStr, string(data))
			if err != nil {
				return fmt.Errorf("pre clutter free error: %s", err)
			}
			data = []byte(clutterFreeData)
		}

		query, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("build doc query with url %s error: %s", urlStr, err)
		}

		query.Find("img").Each(func(i int, selection *goquery.Selection) {
			var (
				srcVal    string
				isExisted bool
			)
			srcVal, isExisted = selection.Attr("src")
			if isExisted {
				nextUrl(w.workerQ, urlStr, srcVal)
			}
			srcVal, isExisted = selection.Attr("data-src")
			if isExisted {
				selection.SetAttr("src", srcVal)
				nextUrl(w.workerQ, urlStr, srcVal)
			}
			srcVal, isExisted = selection.Attr("data-src-retina")
			if isExisted {
				selection.SetAttr("src", srcVal)
				nextUrl(w.workerQ, urlStr, srcVal)
			}
		})

		query.Find("script").Each(func(i int, selection *goquery.Selection) {
			srcVal, isExisted := selection.Attr("src")
			if isExisted {
				nextUrl(w.workerQ, urlStr, srcVal)
			}
		})

		query.Find("link").Each(func(i int, selection *goquery.Selection) {
			relVal, isExisted := selection.Attr("rel")
			if !isExisted || relVal != "stylesheet" {
				return
			}
			relVal, isExisted = selection.Attr("href")
			if isExisted {
				nextUrl(w.workerQ, urlStr, relVal)
			}
		})

		patchedHtml, err := query.Html()
		if err != nil {
			return err
		}

		//patchedHtml = xssSanitize(patchedHtml)
		data = []byte(patchedHtml)
		item.WebResourceData = data
		w.resource.WebSubresources[0] = item
		close(w.workerQ)

	case strings.Contains(contentType, MIMECSS):
		// TODO: parse @import url
	}

	return nil
}

func NewWebArchivePacker() Packer {
	return &webArchiver{
		workerQ:  make(chan string, 5),
		resource: &WebArchive{},
		seen:     map[string]struct{}{},
		parallel: 10,
	}
}
