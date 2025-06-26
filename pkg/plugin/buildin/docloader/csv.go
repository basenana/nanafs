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
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"os"
	"strings"
)

const (
	csvLoader = "csv"
)

type CSV struct {
	docPath string
}

func NewCSV(docPath string, option map[string]string) CSV {
	return CSV{docPath: docPath}
}

func (c CSV) Load(_ context.Context, doc types.DocumentProperties) (*FDocument, error) {
	f, err := os.Open(c.docPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var header []string
	var rown int

	rd := csv.NewReader(f)
	buf := bytes.Buffer{}
	for {
		row, err := rd.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(header) == 0 {
			header = append(header, row...)
			continue
		}

		var content []string
		for i, value := range row {
			line := fmt.Sprintf("%s: %s", header[i], value)
			content = append(content, line)
		}

		rown++
		buf.WriteString(strings.Join(content, "\t"))
		// TODO: using HTML fmt?
	}

	return &FDocument{
		Content:            buf.String(),
		DocumentProperties: doc,
	}, nil
}
