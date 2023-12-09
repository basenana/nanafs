/*
 Copyright 2023 Friday Author.

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

package postgres

import (
	"encoding/json"
	"time"

	"github.com/basenana/friday/pkg/models"
)

type Index struct {
	ID        string `gorm:"column:id;type:varchar(256);primaryKey"`
	Name      string `gorm:"column:name;index:index_name"`
	OID       int64  `gorm:"column:oid;index:index_oid"`
	Group     int    `gorm:"column:idx_group;index:index_group"`
	ParentID  int64  `gorm:"column:parent_entry_id;index:index_parent_id"`
	Content   string `gorm:"column:content"`
	Vector    string `gorm:"column:vector;type:json"`
	Extra     string `gorm:"column:extra"`
	CreatedAt int64  `gorm:"column:created_at"`
	ChangedAt int64  `gorm:"column:changed_at"`
}

func (v *Index) TableName() string {
	return "friday_idx"
}

func (v *Index) Update(vector *Index) {
	v.ID = vector.ID
	v.Name = vector.Name
	v.OID = vector.OID
	v.Group = vector.Group
	v.ParentID = vector.ParentID
	v.Content = vector.Content
	v.Extra = vector.Extra
	v.Vector = vector.Vector
	v.ChangedAt = time.Now().UnixNano()
}

func (v *Index) From(element *models.Element) (*Index, error) {
	i := &Index{
		ID:       element.ID,
		Name:     element.Name,
		OID:      element.OID,
		Group:    element.Group,
		ParentID: element.ParentId,
		Content:  element.Content,
	}
	vector, err := json.Marshal(element.Vector)
	if err != nil {
		return nil, err
	}
	i.Vector = string(vector)

	return i, nil
}

func (v *Index) To() (*models.Element, error) {
	res := &models.Element{
		ID:       v.ID,
		Name:     v.Name,
		Group:    v.Group,
		OID:      v.OID,
		ParentId: v.ParentID,
		Content:  v.Content,
	}
	var vector []float32
	err := json.Unmarshal([]byte(v.Vector), &vector)
	if err != nil {
		return nil, err
	}
	res.Vector = vector

	return res, nil
}

func (v *Index) ToDoc() *models.Doc {
	res := &models.Doc{
		Id:       v.ID,
		OID:      v.OID,
		Name:     v.Name,
		Group:    v.Group,
		Content:  v.Content,
		ParentId: v.ParentID,
	}

	return res
}

type BleveKV struct {
	ID    string `gorm:"column:id;primaryKey"`
	Key   []byte `gorm:"column:key"`
	Value []byte `gorm:"column:value"`
}

func (v *BleveKV) TableName() string {
	return "friday_blevekv"
}
