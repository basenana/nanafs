package packer

import (
	"context"
	"fmt"
	"io"
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

	content, err := h.runPack(ctx, opt)
	if err != nil {
		return err
	}

	output, err := os.OpenFile(opt.FilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
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

func (h *htmlPacker) runPack(ctx context.Context, opt Option) (string, error) {
	content, err := h.readContentByUrl(ctx, opt)
	if err != nil {
		return "", err
	}

	if opt.ClutterFree {
		content, err = htmlContentClutterFree(opt.URL, content)
	}

	return content, err
}

func (h *htmlPacker) ReadContent(ctx context.Context, opt Option) (string, error) {
	var (
		content string
		err     error
	)

	switch {
	case opt.FilePath != "":
		f, err := os.OpenFile(opt.FilePath, os.O_RDONLY, 0644)
		if err != nil {
			return "", fmt.Errorf("open %s failed: %s", opt.FilePath, err)
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return "", fmt.Errorf("read %s failed: %s", opt.FilePath, err)
		}
		content = string(data)
	case opt.Reader != nil:
		defer opt.Reader.Close()
		data, err := io.ReadAll(opt.Reader)
		if err != nil {
			return "", fmt.Errorf("read %s failed: %s", opt.FilePath, err)
		}
		content = string(data)
	default:
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
	cli := newClient(opt)
	res, err := cli.ReadMain(ctx, opt.URL)
	if err != nil {
		return "", fmt.Errorf("read response body with url %s error: %s", opt.URL, err)
	}
	return string(res.Data), nil
}

func NewHtmlPacker() Packer {
	return &htmlPacker{}
}
