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
	"context"
	"encoding/json"
	"os"

	"github.com/google/uuid"

	"github.com/basenana/friday/pkg/models"
	"github.com/basenana/friday/pkg/utils/files"
)

// IngestFromFile ingest a whole file providing models.File
func (f *Friday) IngestFromFile(ctx context.Context, file models.File) (map[string]int, error) {
	elements := []models.Element{}
	// split doc
	subDocs := f.Spliter.Split(file.Content)
	for i, subDoc := range subDocs {
		e := models.Element{
			Name:     file.Name,
			Group:    i,
			OID:      file.OID,
			ParentId: file.ParentId,
			Content:  subDoc,
		}
		elements = append(elements, e)
	}
	// ingest
	return f.Ingest(ctx, elements)
}

// Ingest ingest elements of a file
func (f *Friday) Ingest(ctx context.Context, elements []models.Element) (map[string]int, error) {
	totalToken := make(map[string]int)
	f.Log.Debugf("Ingesting %d ...", len(elements))
	for i, element := range elements {
		exist, err := f.Vector.Get(ctx, element.OID, element.Name, element.Group)
		if err != nil {
			return nil, err
		}
		if exist != nil && exist.Content == element.Content {
			f.Log.Debugf("vector %d(th) name(%s) group(%d) exist, skip ...", i, element.Name, element.Group)
			continue
		}

		vectors, m, err := f.Embedding.VectorQuery(ctx, element.Content)
		if err != nil {
			return nil, err
		}
		for k, v := range m {
			totalToken[k] = v.(int)
		}

		if exist != nil {
			element.ID = exist.ID
			element.OID = exist.OID
			element.ParentId = exist.ParentId
		} else {
			element.ID = uuid.New().String()
		}
		element.Vector = vectors

		f.Log.Debugf("store %d(th) vector name(%s) group(%d)  ...", i, element.Name, element.Group)
		if err := f.Vector.Store(ctx, &element, m); err != nil {
			return nil, err
		}
	}
	return totalToken, nil
}

// IngestFromElementFile ingest a whole file given an element-style origin file
func (f *Friday) IngestFromElementFile(ctx context.Context, ps string) (map[string]int, error) {
	doc, err := os.ReadFile(ps)
	if err != nil {
		return nil, err
	}
	elements := []models.Element{}

	if err := json.Unmarshal(doc, &elements); err != nil {
		return nil, err
	}
	merged := f.Spliter.Merge(elements)
	return f.Ingest(ctx, merged)
}

// IngestFromOriginFile ingest a whole file given an origin file
func (f *Friday) IngestFromOriginFile(ctx context.Context, ps string) (map[string]int, error) {
	fs, err := files.ReadFiles(ps)
	if err != nil {
		return nil, err
	}

	elements := []models.Element{}
	for n, file := range fs {
		subDocs := f.Spliter.Split(file)
		for i, subDoc := range subDocs {
			e := models.Element{
				Content: subDoc,
				Name:    n,
				Group:   i,
			}
			elements = append(elements, e)
		}
	}

	return f.Ingest(ctx, elements)
}
