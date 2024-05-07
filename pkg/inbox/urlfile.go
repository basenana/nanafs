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

package inbox

import (
	"bytes"
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
)

type UrlFile struct {
	Url         string
	FileType    string
	ClutterFree bool
}

func (f UrlFile) Write(ctx context.Context, file dentry.File) error {
	content := genUrlFileContent(f)
	_, err := file.WriteAt(ctx, []byte(content), 0)
	return err
}

func genUrlFileContent(uf UrlFile) string {
	buf := &bytes.Buffer{}
	buf.WriteString("[InternetShortcut]\n")
	buf.WriteString(fmt.Sprintf("URL=%s\n", uf.Url))

	buf.WriteString("[NanaFSSection]\n")
	buf.WriteString(fmt.Sprintf("ArchiveType=%s\n", uf.FileType))

	if uf.ClutterFree {
		buf.WriteString("ClutterFree=true\n")
	}

	buf.WriteString("Cleanup=true\n")

	return buf.String()
}
