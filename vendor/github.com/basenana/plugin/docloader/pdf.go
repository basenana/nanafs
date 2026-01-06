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

package docloader

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/basenana/plugin/types"
	"github.com/ledongthuc/pdf"
)

const pdfParser = "pdf"

var pdfDateRegex = regexp.MustCompile(`^D:(\d{4})(\d{2})(\d{2})(\d{2})?(\d{2})?(\d{2})?`)

type PDF struct {
	docPath  string
	password string
}

func NewPDF(docPath string, option map[string]string) Parser {
	return newPDFWithPassword(docPath, option["password"])
}

func newPDFWithPassword(docPath, pass string) Parser {
	return &PDF{docPath: docPath, password: pass}
}

func parsePDFDate(dateStr string) int64 {
	matches := pdfDateRegex.FindStringSubmatch(dateStr)
	if matches == nil {
		return 0
	}
	year, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	day, _ := strconv.Atoi(matches[3])
	hour, minute, second := 0, 0, 0
	if matches[4] != "" {
		hour, _ = strconv.Atoi(matches[4])
	}
	if matches[5] != "" {
		minute, _ = strconv.Atoi(matches[5])
	}
	if matches[6] != "" {
		second, _ = strconv.Atoi(matches[6])
	}
	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC).Unix()
}

func (p *PDF) Load(_ context.Context) (types.Document, error) {
	fInfo, err := os.Stat(p.docPath)
	if err != nil {
		return types.Document{}, err
	}

	f, err := os.Open(p.docPath)
	if err != nil {
		return types.Document{}, err
	}
	defer f.Close()

	var reader *pdf.Reader
	if p.password != "" {
		reader, err = pdf.NewReaderEncrypted(f, fInfo.Size(), p.getAndCleanPassword)
	} else {
		reader, err = pdf.NewReader(f, fInfo.Size())
	}
	if err != nil {
		return types.Document{}, err
	}

	props := extractPDFMetadata(reader)

	if props.PublishAt == 0 {
		props.PublishAt = fInfo.ModTime().Unix()
	}

	fonts := make(map[string]*pdf.Font)
	buf := &bytes.Buffer{}
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		for _, name := range page.Fonts() {
			if _, ok := fonts[name]; !ok {
				font := page.Font(name)
				fonts[name] = &font
			}
		}
		text, err := page.GetPlainText(fonts)
		if err != nil {
			return types.Document{}, err
		}
		buf.WriteString(text)
	}

	return types.Document{
		Content:    buf.String(),
		Properties: props,
	}, nil
}

func (p *PDF) getAndCleanPassword() string {
	pass := p.password
	if pass != "" {
		p.password = ""
	}
	return pass
}

func extractPDFMetadata(reader *pdf.Reader) types.Properties {
	props := types.Properties{}
	if reader == nil {
		return props
	}
	catalog := reader.Trailer().Key("Root")
	if catalog.IsNull() {
		return props
	}
	info := catalog.Key("Info")
	if info.IsNull() {
		return props
	}

	for _, key := range info.Keys() {
		value := info.Key(key)
		if value.IsNull() {
			continue
		}
		textValue := value.Text()
		if textValue == "" {
			continue
		}

		switch key {
		case "Title":
			props.Title = textValue
		case "Author":
			props.Author = textValue
		case "Subject":
			props.Abstract = textValue
		case "Keywords":
			var keywords []string
			for _, k := range strings.Split(textValue, ";") {
				k = strings.TrimSpace(k)
				if k != "" {
					keywords = append(keywords, k)
				}
			}
			props.Keywords = keywords
		case "Creator", "Producer":
			if props.Source == "" {
				props.Source = textValue
			} else {
				props.Source = props.Source + "; " + textValue
			}
		case "CreationDate":
			if props.PublishAt == 0 {
				props.PublishAt = parsePDFDate(textValue)
			}
		}
	}
	return props
}
