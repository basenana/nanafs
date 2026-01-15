package packer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"howett.net/plist"
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
	workerQ    chan string
	hasMainRes bool
	resource   *WebArchive
	seen       map[string]struct{}
	parallel   int
	mux        sync.Mutex
}

func (w *webArchiver) Pack(ctx context.Context, opt Option) error {
	if opt.URL == "" {
		return fmt.Errorf("url is empty")
	}
	if opt.FilePath == "" {
		return fmt.Errorf("file path is empty")
	}

	w.workerQ <- opt.URL
	ctx, canF := context.WithCancel(ctx)
	defer canF()

	var (
		workerDone = make(chan struct{})
		errCh      = make(chan error, 1)
		wg         = sync.WaitGroup{}
	)

	for i := 0; i < w.parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.workerRun(ctx, opt, errCh)
		}()
	}

	go func() {
		wg.Wait()
		close(workerDone)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-workerDone:
		if len(w.resource.WebSubresources) == 0 {
			return fmt.Errorf("no resource packed")
		}
		w.resource.WebMainResource = w.resource.WebSubresources[0]
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
	cli := newClient(opt)
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
				if err := w.loadWebPageFromUrl(ctx, cli, urlStr, opt); err != nil {
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
	switch {

	case opt.FilePath != "":
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
	case opt.Reader != nil:
		defer opt.Reader.Close()

		data, err := io.ReadAll(opt.Reader)
		if err != nil {
			return "", fmt.Errorf("read %s failed: %s", opt.FilePath, err)
		}

		d := plist.NewDecoder(bytes.NewReader(data))
		err = d.Decode(w.resource)
		if err != nil {
			return "", fmt.Errorf("load webarchive %s failed: %s", opt.FilePath, err)
		}
	default:
		go func() {
			for _ = range w.workerQ {
				// discard next url
			}
		}()

		if err := w.loadWebPageFromUrl(ctx, newClient(opt), opt.URL, opt); err != nil {
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

func (w *webArchiver) loadWebPageFromUrl(ctx context.Context, cli Client, urlStr string, opt Option) error {
	var (
		res *WebResource
		err error
	)
	if !w.hasMainRes {
		res, err = cli.ReadMain(ctx, urlStr)
	} else {
		res, err = cli.ReadResource(ctx, urlStr)
	}
	if err != nil {
		return err
	}

	// fix text/html; charset=utf-8
	contentTypeParts := strings.Split(res.ContentType, ";")
	contentType := strings.TrimSpace(contentTypeParts[0])

	var bodyReader io.Reader = bytes.NewReader(res.Data)
	if strings.Contains(contentType, MIMEHTML) {
		rawReader := bodyReader
		bodyReader, err = charset.NewReader(rawReader, res.ContentType)
		if err != nil {
			return fmt.Errorf("new charset reader with url %s error: %s", urlStr, err)
		}
	}

	data, err := io.ReadAll(bodyReader)
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

		w.mux.Lock()
		// only one page
		if w.hasMainRes {
			w.mux.Unlock()
			break
		}
		w.hasMainRes = true
		w.mux.Unlock()

		if opt.ClutterFree {
			clutterFreeContent, err := htmlContentClutterFree(urlStr, string(data))
			if err != nil {
				return fmt.Errorf("make clustter free failed: %s", err)
			}
			data = []byte(clutterFreeContent)
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
			srcVal, isExisted = selection.Attr("data-original")
			if isExisted {
				selection.SetAttr("src", srcVal)
				nextUrl(w.workerQ, urlStr, srcVal)
			}
			// remove srcset for display issues
			selection.RemoveAttr("srcset")
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
		workerQ:  make(chan string, 1024),
		resource: &WebArchive{},
		seen:     map[string]struct{}{},
		parallel: 10,
	}
}
