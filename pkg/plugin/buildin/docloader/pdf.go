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

	"github.com/basenana/nanafs/pkg/types"
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

func (p *PDF) Load(_ context.Context, doc types.DocumentProperties) (*FDocument, error) {
	fInfo, err := os.Stat(p.docPath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(p.docPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader *pdf.Reader
	if p.password != "" {
		reader, err = pdf.NewReaderEncrypted(f, fInfo.Size(), p.getAndCleanPassword)
	} else {
		reader, err = pdf.NewReader(f, fInfo.Size())
	}
	if err != nil {
		return nil, err
	}

	doc = extractPDFMetadata(reader, doc)

	if doc.PublishAt == 0 {
		doc.PublishAt = fInfo.ModTime().Unix()
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
			return nil, err
		}
		buf.WriteString(text)
	}

	return &FDocument{
		Content:            buf.String(),
		DocumentProperties: doc,
	}, nil
}

func (p *PDF) getAndCleanPassword() string {
	pass := p.password
	if pass != "" {
		p.password = ""
	}
	return pass
}

func extractPDFMetadata(reader *pdf.Reader, doc types.DocumentProperties) types.DocumentProperties {
	if reader == nil {
		return doc
	}
	catalog := reader.Trailer().Key("Root")
	if catalog.IsNull() {
		return doc
	}
	info := catalog.Key("Info")
	if info.IsNull() {
		return doc
	}

	mapping := map[string]string{
		"Title":        "Title",
		"Author":       "Author",
		"Subject":      "Abstract",
		"Keywords":     "Keywords",
		"Creator":      "Source",
		"Producer":     "Source",
		"CreationDate": "PublishAt",
	}

	for _, key := range info.Keys() {
		value := info.Key(key)
		if value.IsNull() {
			continue
		}
		fieldName, ok := mapping[key]
		if !ok || fieldName == "" {
			continue
		}
		textValue := value.Text()
		if textValue == "" {
			continue
		}

		switch fieldName {
		case "Title":
			doc.Title = textValue
		case "Author":
			doc.Author = textValue
		case "Abstract":
			if doc.Abstract == "" {
				doc.Abstract = textValue
			}
		case "Keywords":
			for _, k := range strings.Split(textValue, ";") {
				k = strings.TrimSpace(k)
				if k != "" {
					doc.Keywords = append(doc.Keywords, k)
				}
			}
		case "Source":
			if doc.Source == "" {
				doc.Source = textValue
			} else {
				doc.Source = doc.Source + "; " + textValue
			}
		case "PublishAt":
			if doc.PublishAt == 0 {
				doc.PublishAt = parsePDFDate(textValue)
			}
		}
	}
	return doc
}
