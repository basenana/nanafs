package packer

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

type htmlPacker struct{}

func (h *htmlPacker) Pack(ctx context.Context, opt Option) error {
	if opt.URL == "" {
		return fmt.Errorf("url is empty")
	}
	if opt.FilePath == "" {
		return fmt.Errorf("file path is empty")
	}

	content, err := h.ReadContent(ctx, opt)
	if err != nil {
		return err
	}

	output, err := os.OpenFile(opt.FilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0655)
	if err != nil {
		return fmt.Errorf("open output file failed: %s", err)
	}
	defer output.Close()

	_, err = output.WriteString(content)
	if err != nil {
		return fmt.Errorf("write output to file failed: %s", err)
	}
	return nil
}

func (h *htmlPacker) ReadContent(ctx context.Context, opt Option) (string, error) {
	var (
		content string
		err     error
	)
	if opt.FilePath != "" {
		f, err := os.OpenFile(opt.FilePath, os.O_RDONLY, 0655)
		if err != nil {
			return "", fmt.Errorf("open %s failed: %s", opt.FilePath, err)
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return "", fmt.Errorf("read %s failed: %s", opt.FilePath, err)
		}
		content = string(data)
	} else {
		content, err = h.readContentByUrl(ctx, opt)
		if err != nil {
			return "", err
		}
	}

	if opt.ClutterFree {
		content, err = htmlContentClutterFree(opt.URL, content)
	}

	return content, err
}

func (h *htmlPacker) readContentByUrl(ctx context.Context, opt Option) (string, error) {
	if opt.URL == "" {
		return "", fmt.Errorf("url is empty")
	}
	cli, headers := newHttpClient(opt)
	urlStr := opt.URL
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("build request with url %s error: %s", urlStr, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := cli.Do(req.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("do request with url %s error: %s", urlStr, err)
	}
	defer func() {
		_, _ = ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("do request with url %s error: status code is %d", urlStr, resp.StatusCode)
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

	data, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		return "", fmt.Errorf("read response body with url %s error: %s", urlStr, err)
	}
	return string(data), nil
}

func NewHtmlPacker() Packer {
	return &htmlPacker{}
}
