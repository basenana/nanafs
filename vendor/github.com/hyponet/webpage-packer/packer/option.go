package packer

import "time"

var (
	defaultTimeout = time.Minute
	defaultHeaders = map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Safari/605.1.15",
		"Accept-Language": "en-us",
		"Accept-Encoding": "gzip, deflate",
	}
)

type Option struct {
	URL         string
	FilePath    string
	Timeout     int
	ClutterFree bool
	Headers     map[string]string

	EnablePrivateNet bool
}
