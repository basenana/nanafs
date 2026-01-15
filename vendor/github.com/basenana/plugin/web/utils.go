package web

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/utils"
	"github.com/hyponet/webpage-packer/packer"
)

var (
	browserlessURL   = os.Getenv("WebPackerBrowserlessURL")
	browserlessToken = os.Getenv("WebPackerBrowserlessToken")
	enablePrivateNet = os.Getenv("WebPackerEnablePrivateNet") == "true"
)

type Option func(option *packer.Option)

func PackFromURL(ctx context.Context, filename, urlInfo, tgtFileType, outputDir string, clutterFree bool, options ...Option) (string, error) {
	var (
		log = logger.FromContext(ctx)
		bc  *packer.Browserless
		err error
	)

	if urlInfo == "" {
		return "", fmt.Errorf("url is empty")
	}

	if filename == "" {
		filename, err = generateValidFilenameUsingTitle(ctx, urlInfo)
		if err != nil {
			return "", err
		}
	}

	outputFile := filename + "." + tgtFileType
	log.Infof("packing url %s to %s", urlInfo, outputFile)

	if browserlessURL != "" {
		bc = &packer.Browserless{
			Endpoint:    browserlessURL,
			Token:       browserlessToken,
			StealthMode: true,
			BlockADS:    true,
		}
	}

	var (
		filePath = path.Join(outputDir, outputFile)
		p        packer.Packer
		opt      packer.Option
	)
	switch tgtFileType {
	case "webarchive":
		p = packer.NewWebArchivePacker()
	case "html":
		p = packer.NewHtmlPacker()
	default:
		return "", fmt.Errorf("unsupported file type %s", tgtFileType)
	}

	opt = packer.Option{
		URL:              urlInfo,
		FilePath:         filePath,
		Timeout:          60,
		ClutterFree:      clutterFree,
		Headers:          make(map[string]string),
		Browserless:      bc,
		EnablePrivateNet: enablePrivateNet,
	}

	for _, option := range options {
		option(&opt)
	}

	err = p.Pack(ctx, opt)
	if err != nil {
		log.Warnw("pack to web file failed", "link", urlInfo, "err", err)
		return "", fmt.Errorf("pack to web failed: %w", err)
	}

	return filePath, nil
}

func ReadFromFile(ctx context.Context, filePath string) (string, error) {
	var (
		log     = logger.FromContext(ctx)
		ext     = path.Ext(filePath)
		content string
		err     error
	)

	switch ext {
	case ".webarchive":
		p := packer.NewWebArchivePacker()
		content, err = p.ReadContent(ctx, packer.Option{
			FilePath:    filePath,
			ClutterFree: true,
		})
		if err != nil {
			log.Warnw("read webarchive failed", "err", err)
			return "", fmt.Errorf("read webarchive failed: %w", err)
		}
	case ".html", ".htm", ".hts":
		p := packer.NewHtmlPacker()
		content, err = p.ReadContent(ctx, packer.Option{
			FilePath:    filePath,
			ClutterFree: true,
		})
		if err != nil {
			log.Warnw("read raw html file failed", "err", err)
			return "", fmt.Errorf("read html failed: %w", err)
		}
	}

	return content, nil
}

func ParseFromFile(ctx context.Context, filePath string) (string, error) {
	content, err := ReadFromFile(ctx, filePath)
	if err != nil {
		return "", err
	}
	markdown, err := htmltomarkdown.ConvertString(content)
	if err != nil {
		logger.FromContext(ctx).Warnw("convert to markdown failed", "error", err)
		return content, nil
	}

	return markdown, nil
}

func generateValidFilenameUsingTitle(ctx context.Context, urlInfo string) (string, error) {
	p := packer.NewHtmlPacker()
	content, err := p.ReadContent(ctx, packer.Option{
		URL:         urlInfo,
		ClutterFree: false,
	})
	if err != nil {
		return "", fmt.Errorf("read html file failed: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("parse html file failed: %w", err)
	}

	title := doc.Find("title").Text()
	if title == "" {
		return "", fmt.Errorf("parse html file failed: title not found")
	}

	return utils.SanitizeFilename(title), nil
}
