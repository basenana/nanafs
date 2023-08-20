package requester

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tickstep/library-go/requester/rio"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// HttpGet 简单实现 http 访问 GET 请求
func HttpGet(urlStr string) (body []byte, err error) {
	resp, err := DefaultClient.Get(urlStr)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}

// HttpPost 简单的HTTP POST方法
func HttpPost(urlStr string, postData interface{}) (body []byte, err error) {
	return Fetch("POST", urlStr, postData, nil)
}

// Req 参见 *HTTPClient.Req, 使用默认 http 客户端
func Req(method string, urlStr string, post interface{}, header map[string]string) (resp *http.Response, err error) {
	return DefaultClient.Req(method, urlStr, post, header)
}

// Fetch 参见 *HTTPClient.Fetch, 使用默认 http 客户端
func Fetch(method string, urlStr string, post interface{}, header map[string]string) (body []byte, err error) {
	return DefaultClient.Fetch(method, urlStr, post, header)
}

// Req 实现 http／https 访问，
// 根据给定的 method (GET, POST, HEAD, PUT 等等), urlStr (网址),
// post (post 数据), header (header 请求头数据), 进行网站访问。
// 返回值分别为 *http.Response, 错误信息
func (h *HTTPClient) Req(method string, urlStr string, post interface{}, header map[string]string) (resp *http.Response, err error) {
	h.lazyInit()
	var (
		req           *http.Request
		obody         io.Reader
		contentLength int64
		contentType   string
	)

	if post != nil {
		isJson := false
		if header != nil {
			if ct, ok := header["Content-Type"]; ok {
				if strings.Contains(strings.ToLower(ct), "application/json") {
					isJson = true
				}
			}
			if ct, ok := header["content-type"]; ok {
				if strings.Contains(strings.ToLower(ct), "application/json") {
					isJson = true
				}
			}
		}
		if isJson {
			switch value := post.(type) {
			case io.Reader:
				obody = value
			case map[string]string:
				paramJson, _ := json.Marshal(value)
				obody = strings.NewReader(string(paramJson))
			case map[string]interface{}:
				paramJson, _ := json.Marshal(value)
				obody = strings.NewReader(string(paramJson))
			case map[interface{}]interface{}:
				paramJson, _ := json.Marshal(value)
				obody = strings.NewReader(string(paramJson))
			case string:
				obody = strings.NewReader(value)
			case []byte:
				obody = bytes.NewReader(value[:])
			default:
				paramJson, _ := json.Marshal(value)
				obody = strings.NewReader(string(paramJson))
			}
		} else {
			switch value := post.(type) {
			case io.Reader:
				obody = value
			case map[string]string:
				query := url.Values{}
				for k := range value {
					query.Set(k, value[k])
				}
				obody = strings.NewReader(query.Encode())
			case map[string]interface{}:
				query := url.Values{}
				for k := range value {
					query.Set(k, fmt.Sprint(value[k]))
				}
				obody = strings.NewReader(query.Encode())
			case map[interface{}]interface{}:
				query := url.Values{}
				for k := range value {
					query.Set(fmt.Sprint(k), fmt.Sprint(value[k]))
				}
				obody = strings.NewReader(query.Encode())
			case string:
				obody = strings.NewReader(value)
			case []byte:
				obody = bytes.NewReader(value[:])
			default:
				return nil, fmt.Errorf("requester.Req: unknown post type: %s", value)
			}
		}

		switch value := post.(type) {
		case ContentLengther:
			contentLength = value.ContentLength()
		case rio.Lener:
			contentLength = int64(value.Len())
		case rio.Lener64:
			contentLength = value.Len()
		}

		switch value := post.(type) {
		case ContentTyper:
			contentType = value.ContentType()
		}
	}
	req, err = http.NewRequest(method, urlStr, obody)
	if err != nil {
		return nil, err
	}

	if req.ContentLength <= 0 && contentLength != 0 {
		req.ContentLength = contentLength
	}

	// 设置浏览器标识
	req.Header.Set("User-Agent", h.UserAgent)

	// 设置Content-Type
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if header != nil {
		// 处理Host
		if host, ok := header["Host"]; ok {
			req.Host = host
		}

		for key := range header {
			req.Header.Set(key, header[key])
		}
	}

	return h.Client.Do(req)
}

// Fetch 实现 http／https 访问，
// 根据给定的 method (GET, POST, HEAD, PUT 等等), urlStr (网址),
// post (post 数据), header (header 请求头数据), 进行网站访问。
// 返回值分别为 网站主体, 错误信息
func (h *HTTPClient) Fetch(method string, urlStr string, post interface{}, header map[string]string) (body []byte, err error) {
	h.lazyInit()
	resp, err := h.Req(method, urlStr, post, header)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(resp.Body)
}

func (h *HTTPClient) DoGet(urlStr string) (body []byte, err error) {
	return h.Fetch("GET", urlStr, nil, nil)
}

func (h *HTTPClient) DoPost(urlStr string, post interface{}) (body []byte, err error) {
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
	}
	return h.Fetch("POST", urlStr, post, headers)
}