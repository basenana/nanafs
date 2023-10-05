/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package buildin

import (
	"net/url"
	"strings"
)

func readableHtmlContent(urlStr, title, content string) string {
	var hostStr string
	u, err := url.Parse(urlStr)
	if err == nil {
		hostStr = u.Host
	} else {
		hostStr = urlStr
	}
	patched := strings.ReplaceAll(readableHtmlTpl, "{TITLE}", title)
	patched = strings.ReplaceAll(patched, "{HOST}", hostStr)
	patched = strings.ReplaceAll(patched, "{URL}", urlStr)
	patched = strings.ReplaceAll(patched, "{CONTENT}", content)

	return patched
}

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
