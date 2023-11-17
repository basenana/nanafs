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

package pluginapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"time"
)

const (
	CachedDataFile = ".data.plugin.json"
)

/*
CachedData is the cached data stored in the source plugin directory,
which supports sharing cached data across different workflows.
*/
type CachedData struct {
	Items     []cachedItem `json:"items"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`

	changed bool
}

func (d *CachedData) ListItems(group string) (result []string) {
	for _, it := range d.Items {
		if it.Group != group {
			continue
		}

		decodedVal, err := base64.StdEncoding.DecodeString(it.EncodedVal)
		if err != nil {
			continue
		}
		result = append(result, string(decodedVal))
	}
	return
}

func (d *CachedData) SetItem(group, cachedKey, itemVal string) {
	encodedVal := base64.StdEncoding.EncodeToString([]byte(itemVal))
	for i, it := range d.Items {
		if it.Group == group && it.Key == cachedKey {
			d.Items[i].EncodedVal = encodedVal
			d.changed = true
			return
		}
	}

	d.Items = append(d.Items, cachedItem{Key: cachedKey, Group: group, EncodedVal: encodedVal})
	d.changed = true
}

func (d *CachedData) NeedReCache() bool {
	return d.changed
}

func (d *CachedData) Reader() (io.Reader, error) {
	d.UpdatedAt = time.Now()

	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(d)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func InitCacheData() *CachedData {
	return &CachedData{
		Items:     make([]cachedItem, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		changed:   true,
	}
}

func OpenCacheData(reader io.Reader) (*CachedData, error) {
	cd := &CachedData{}
	err := json.NewDecoder(reader).Decode(cd)
	if err != nil {
		return nil, err
	}
	return cd, nil
}

type cachedItem struct {
	Key        string `json:"key"`
	Group      string `json:"group"`
	EncodedVal string `json:"encoded_val"`
}
