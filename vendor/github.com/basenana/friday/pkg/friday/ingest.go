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
	"encoding/json"
	"errors"
	"os"

	"github.com/google/uuid"

	"github.com/basenana/friday/pkg/models"
	"github.com/basenana/friday/pkg/utils/files"
)

func (f *Friday) ElementFile(ps *string) *Friday {
	f.statement.elementFile = ps
	return f
}

func (f *Friday) Element(elements []models.Element) *Friday {
	f.statement.elements = elements
	return f
}

func (f *Friday) OriginFile(ps *string) *Friday {
	f.statement.originFile = ps
	return f
}

func (f *Friday) File(file *models.File) *Friday {
	f.statement.file = file
	return f
}

func (f *Friday) Ingest(res *IngestState) *Friday {
	elements := []models.Element{}

	if f.statement.file != nil {
		// split doc
		subDocs := f.Spliter.Split(f.statement.file.Content)
		for i, subDoc := range subDocs {
			e := models.Element{
				Name:     f.statement.file.Name,
				Group:    i,
				OID:      f.statement.file.OID,
				ParentId: f.statement.file.ParentId,
				Content:  subDoc,
			}
			elements = append(elements, e)
		}
		f.statement.elements = elements
	}

	if f.statement.elementFile != nil {
		doc, err := os.ReadFile(*f.statement.elementFile)
		if err != nil {
			f.Error = err
			return f
		}

		if err := json.Unmarshal(doc, &elements); err != nil {
			f.Error = err
			return f
		}
		elements = f.Spliter.Merge(elements)
		f.statement.elements = elements
	}

	if f.statement.originFile != nil {
		fs, err := files.ReadFiles(*f.statement.originFile)
		if err != nil {
			f.Error = err
			return f
		}

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
		f.statement.elements = elements
	}

	// ingest
	return f.ingest(res)
}

func (f *Friday) ingest(res *IngestState) *Friday {
	if res == nil {
		f.Error = errors.New("result can not be nil")
		return f
	}
	totalToken := make(map[string]int)
	f.Log.Debugf("Ingesting %d ...", len(f.statement.elements))
	for i, element := range f.statement.elements {
		exist, err := f.Vector.Get(f.statement.context, element.OID, element.Name, element.Group)
		if err != nil {
			f.Error = err
			return f
		}
		if exist != nil && exist.Content == element.Content {
			f.Log.Debugf("vector %d(th) name(%s) group(%d) exist, skip ...", i, element.Name, element.Group)
			continue
		}

		vectors, m, err := f.Embedding.VectorQuery(f.statement.context, element.Content)
		if err != nil {
			f.Error = err
			return f
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
		if err := f.Vector.Store(f.statement.context, &element, m); err != nil {
			f.Error = err
			return f
		}
	}
	res.Tokens = totalToken
	return f
}
