package packer

import (
	"net/url"
	"path"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// Content-Type MIME of the most common data formats.
const (
	MIMEHTML = "text/html"
	MIMECSS  = "text/css"
)

func xssSanitize(bodyContent string) string {
	sanitized := bluemonday.UGCPolicy().Sanitize(bodyContent)
	return strings.TrimSpace(sanitized)
}

func nextUrl(workQ chan string, topUrl, nextUrl string) {
	if nextUrl == "" {
		return
	}

	if strings.HasPrefix(nextUrl, "data:") {
		return
	}

	topParsedUrl, err := url.Parse(topUrl)
	if err != nil {
		return
	}

	nextParsedUrl, err := url.Parse(nextUrl)
	if err != nil {
		u, err := url.Parse(topUrl)
		if err != nil {
			return
		}
		u.Path = path.Join(u.Path, nextUrl)
		nextParsedUrl = u
	}
	nextParsedUrl.Fragment = ""
	if nextParsedUrl.Scheme == "" {
		nextParsedUrl.Scheme = topParsedUrl.Scheme
	}
	workQ <- nextParsedUrl.String()
}
