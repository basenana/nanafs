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
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"os"
)

const (
	textParser = "text"
)

type Text struct {
	docPath string
}

func NewText(docPath string, option map[string]string) Parser { return Text{docPath: docPath} }

func (l Text) Load(_ context.Context, doc types.DocumentProperties) (*FDocument, error) {
	f, err := os.Open(l.docPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, err
	}

	return &FDocument{
		Content:            buf.String(),
		DocumentProperties: doc,
	}, nil
}
