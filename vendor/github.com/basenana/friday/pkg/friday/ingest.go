/*
 * Copyright 2023 friday
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package friday

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/basenana/friday/pkg/models"
	"github.com/basenana/friday/pkg/utils/files"
)

// IngestFromFile ingest a whole file providing models.File
func (f *Friday) IngestFromFile(file models.File) error {
	elements := []models.Element{}
	parentDir := filepath.Dir(file.Source)
	// split doc
	subDocs := f.Spliter.Split(file.Content)
	for i, subDoc := range subDocs {
		e := models.Element{
			Content: subDoc,
			Metadata: models.Metadata{
				Source:    file.Source,
				Group:     strconv.Itoa(i),
				ParentDir: parentDir,
			},
		}
		elements = append(elements, e)
	}
	// ingest
	return f.Ingest(elements)
}

// Ingest ingest elements of a file
func (f *Friday) Ingest(elements []models.Element) error {
	f.Log.Debugf("Ingesting %d ...", len(elements))
	for i, element := range elements {
		// id: sha256(source)-group
		h := sha256.New()
		h.Write([]byte(element.Metadata.Source))
		val := hex.EncodeToString(h.Sum(nil))[:64]
		id := fmt.Sprintf("%s-%s", val, element.Metadata.Group)
		if exist, err := f.Vector.Exist(id); err != nil {
			return err
		} else if exist {
			f.Log.Debugf("vector %d(th) id(%s) source(%s) exist, skip ...", i, id, element.Metadata.Source)
			continue
		}

		vectors, m, err := f.Embedding.VectorQuery(element.Content)
		if err != nil {
			return err
		}

		t := strings.TrimSpace(element.Content)

		f.Log.Debugf("store %d(th) vector id (%s) source(%s) ...", i, id, element.Metadata.Source)
		if err := f.Vector.Store(id, t, element.Metadata, m, vectors); err != nil {
			return err
		}
	}
	return nil
}

// IngestFromElementFile ingest a whole file given an element-style origin file
func (f *Friday) IngestFromElementFile(ps string) error {
	doc, err := os.ReadFile(ps)
	if err != nil {
		return err
	}
	elements := []models.Element{}
	if err := json.Unmarshal(doc, &elements); err != nil {
		return err
	}
	merged := f.Spliter.Merge(elements)
	return f.Ingest(merged)
}

// IngestFromOriginFile ingest a whole file given an origin file
func (f *Friday) IngestFromOriginFile(ps string) error {
	fs, err := files.ReadFiles(ps)
	if err != nil {
		return err
	}

	elements := []models.Element{}
	for n, file := range fs {
		parentDir := filepath.Dir(n)
		subDocs := f.Spliter.Split(file)
		for i, subDoc := range subDocs {
			e := models.Element{
				Content: subDoc,
				Metadata: models.Metadata{
					Source:    n,
					Group:     strconv.Itoa(i),
					ParentDir: parentDir,
				},
			}
			elements = append(elements, e)
		}
	}

	return f.Ingest(elements)
}
