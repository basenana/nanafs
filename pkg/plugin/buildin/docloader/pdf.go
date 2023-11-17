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
	"context"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/ledongthuc/pdf"
	"os"
)

const (
	pdfParser = "pdf"
)

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

func (p *PDF) Load(_ context.Context) ([]types.FDocument, error) {
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
		if err != nil {
			return nil, err
		}
	} else {
		reader, err = pdf.NewReader(f, fInfo.Size())
		if err != nil {
			return nil, err
		}
	}

	var (
		numPages = reader.NumPage()
		result   = make([]types.FDocument, 0)
	)

	fonts := make(map[string]*pdf.Font)
	for i := 1; i < numPages+1; i++ {
		page := reader.Page(i)
		for _, name := range page.Fonts() {
			if _, ok := fonts[name]; !ok {
				f := page.Font(name)
				fonts[name] = &f
			}
		}
		text, err := page.GetPlainText(fonts)
		if err != nil {
			return nil, err
		}

		// TODO: using HTML fmt?
		result = append(result, types.FDocument{
			Content: text,
			Metadata: map[string]any{
				"type":        "pdf",
				"page":        i,
				"total_pages": numPages,
			},
		})
	}

	return result, nil
}

func (p *PDF) getAndCleanPassword() string {
	pass := p.password
	if pass != "" {
		// set password empty to stop retry
		p.password = ""
	}
	return pass
}
