package packer

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-shiori/go-readability"
)

const readableHtmlTpl = `
<head>
<title>{TITLE}</title>
<meta charset='UTF-8' />
<meta name='viewport' content='width=device-width, initial-scale=1.0, user-scalable=yes'>
<style type='text/css'>body, table { width: 95%; margin: 0 auto; background-color: #FFF; color:#333; font-family: arial, sans-serif; font-weight: 100; font-size: 12pt; margin:2em 2em 2em 2em; }
p, li { line-height: 150%; }
h1, h2, h3 { color: #333; }
a { color: #3366cc; border-bottom: 1px dotted #3366cc; text-decoration: none; }
a:hover { color: #2647a3; border-bottom-color: color: #66ccff; }
img { max-width: 90%; margin: 0 auto; }
pre { overflow: auto; }
blockquote { color: #888888; padding: 10px; }
figure { width: 100%; margin: 0px; }
figure figcaption { display: none; }
iframe { height: auto; width: auto; max-width: 95%; max-height: 100%; }</style>
<body>
<div> <a href="{URL}" target="_blank">{HOST}</a> <h1>{TITLE}</h1> </div>
{CONTENT}
</body>
`

func makeClutterFree(wa *WebArchive) error {
	res := wa.WebMainResource
	resUrl, err := url.Parse(res.WebResourceURL)
	if err != nil {
		return fmt.Errorf("parse main resource url failed: %s", err)
	}

	article, err := readability.FromReader(bytes.NewReader(res.WebResourceData), resUrl)
	if err != nil {
		return fmt.Errorf("parse main resource failed: %s", err)
	}

	patched := strings.ReplaceAll(readableHtmlTpl, "{TITLE}", article.Title)
	patched = strings.ReplaceAll(patched, "{HOST}", resUrl.Host)
	patched = strings.ReplaceAll(patched, "{URL}", resUrl.String())
	patched = strings.ReplaceAll(patched, "{CONTENT}", article.Content)

	res.WebResourceData = []byte(patched)
	wa.WebMainResource = res
	wa.WebSubresources[0] = res

	return nil
}

func htmlContentClutterFree(urlStr, htmlContent string) (string, error) {
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
		return "", fmt.Errorf("parse main resource failed: %s", err)
	}

	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("<h1>%s</h1>\n", article.Title))
	buf.WriteString(article.Content)

	return buf.String(), nil
}
