package packer

import (
	"bytes"
	"fmt"
	"github.com/yosssi/gohtml"
	"net/url"
	"strings"

	"github.com/go-shiori/go-readability"
)

const readableHtmlTpl = `
<head>
<title>{TITLE}</title>
<meta charset='UTF-8' />
<meta name='viewport' content='width=device-width, initial-scale=1.0, user-scalable=yes'>
<meta name='packer' content='webpage-packer, clutter-free=true'>
<style type='text/css'>body, table { width: 95%; margin: 0 auto; background-color: #FFF; color:#333; font-family: arial, sans-serif; font-weight: 100; font-size: 12pt; margin:2em 2em 2em 2em; }
p, li { line-height: 150%; }
h1, h2, h3 { color: #333; }
a { color: #3366cc; border-bottom: 1px dotted #3366cc; text-decoration: none; }
a:hover { color: #2647a3; border-bottom-color: color: #66ccff; }
img { max-width: 50%; height: auto; margin: 10px auto; }
pre { overflow: auto; }
blockquote { color: #888888; padding: 10px; }
figure { width: 100%; margin: 0px; }
figure figcaption { display: none; }
iframe { height: auto; width: auto; max-width: 95%; max-height: 100%; }</style>
</head>
<body>
<div> <a href="{URL}" target="_blank">{HOST}</a> <h1>{TITLE}</h1> </div>
{CONTENT}
</body>
`

func htmlContentClutterFree(urlStr, htmlContent string) (string, error) {
	if strings.Contains(htmlContent, "clutter-free=true") {
		return htmlContent, nil
	}

	resUrl := &url.URL{}
	if urlStr != "" {
		var err error
		resUrl, err = url.Parse(urlStr)
		if err != nil {
			return "", fmt.Errorf("parse main resource url failed: %s", err)
		}
	}

	article, err := readability.FromReader(bytes.NewReader([]byte(htmlContent)), resUrl)
	if err != nil {
		return "", fmt.Errorf("parse html resource failed: %s", err)
	}

	patched := strings.ReplaceAll(readableHtmlTpl, "{TITLE}", article.Title)
	patched = strings.ReplaceAll(patched, "{HOST}", resUrl.Host)
	patched = strings.ReplaceAll(patched, "{URL}", resUrl.String())
	patched = strings.ReplaceAll(patched, "{CONTENT}", article.Content)

	return gohtml.Format(patched), nil
}
