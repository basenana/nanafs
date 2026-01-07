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
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/basenana/plugin/types"
)

type CSV struct {
	docPath string
}

func NewCSV(docPath string, option map[string]string) CSV {
	return CSV{docPath: docPath}
}

func (c CSV) Load(_ context.Context) (types.Document, error) {
	f, err := os.Open(c.docPath)
	if err != nil {
		return types.Document{}, err
	}
	defer f.Close()

	rd := csv.NewReader(f)
	buf := bytes.Buffer{}

	var header []string
	for {
		row, err := rd.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return types.Document{}, err
		}

		if len(header) == 0 {
			header = row
			continue
		}

		var content []string
		for i, value := range row {
			if i < len(header) {
				content = append(content, fmt.Sprintf("%s: %s", header[i], value))
			}
		}
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(strings.Join(content, "\t"))
	}

	props := extractFileNameMetadata(c.docPath)

	if props.PublishAt == 0 {
		if info, err := os.Stat(c.docPath); err == nil {
			props.PublishAt = info.ModTime().Unix()
		}
	}

	if props.Abstract == "" {
		props.Abstract = fmt.Sprintf("CSV file with %d columns", len(header))
	}

	return types.Document{
		Content:    buf.String(),
		Properties: props,
	}, nil
}
