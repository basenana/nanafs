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

package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/basenana/nanafs/pkg/types"
)

type Entity struct {
	*gorm.DB
}

func NewDbEntity(db *gorm.DB) (*Entity, error) {
	ent := &Entity{DB: db}

	ctx, canF := context.WithTimeout(context.TODO(), time.Second*10)
	defer canF()

	if err := Migrate(db); err != nil {
		return nil, err
	}

	_, err := ent.SystemInfo(ctx)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	if err != nil {
		sysInfo := &SystemInfo{FsID: uuid.New().String()}
		if res := db.WithContext(ctx).Create(sysInfo); res.Error != nil {
			return nil, res.Error
		}
	}

	return ent, nil
}

func (e *Entity) GetObjectByID(ctx context.Context, oid int64) (*types.Object, error) {
	var (
		objMod = &Object{ID: oid}
	)

	if err := e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.First(objMod)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}); err != nil {
		return nil, err
	}

	obj := objMod.Object()

	return obj, nil
}

func (e *Entity) GetObjectExtendData(ctx context.Context, obj *types.Object) error {
	var (
		ext      = &ObjectExtend{}
		property = make([]ObjectProperty, 0)
	)
	if err := e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", obj.ID).First(ext)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("oid = ?", obj.ID).Find(&property)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}); err != nil {
		return err
	}

	ed := ext.ToExtData()
	for _, p := range property {
		ed.Properties.Fields[p.Name] = p.Value
	}

	obj.ExtendData = &ed
	return nil
}

func (e *Entity) GetLabels(ctx context.Context, refType string, refID int64) (*types.Labels, error) {
	ol := make([]Label, 0)
	res := e.WithContext(ctx).Where("ref_type = ? AND ref_id = ?", refType, refID).Find(&ol)
	if res.Error != nil {
		return nil, res.Error
	}

	label := &types.Labels{}
	for _, kv := range ol {
		label.Labels = append(label.Labels, types.Label{Key: kv.Key, Value: kv.Value})
	}
	return label, nil
}

func (e *Entity) ListObjectChildren(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	if len(filter.Label.Include) > 0 || len(filter.Label.Exclude) > 0 {
		return listObjectWithLabelMatcher(ctx, e.DB, filter.Label)
	}

	objectList := make([]Object, 0)
	var result []*types.Object
	if err := e.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := queryFilter(tx, filter).Find(&objectList)
		if res.Error != nil {
			return res.Error
		}

		result = make([]*types.Object, len(objectList))
		for i, objMod := range objectList {
			o := objMod.Object()
			result[i] = o
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (e *Entity) SaveObject(ctx context.Context, parent, object *types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if parent != nil {
			if err := saveRawObject(tx, parent); err != nil {
				return err
			}
		}
		if err := saveRawObject(tx, object); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) SaveMirroredObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveRawObject(tx, srcObj); err != nil {
			return err
		}
		if err := saveRawObject(tx, dstParent); err != nil {
			return err
		}
		if err := saveRawObject(tx, object); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) SaveChangeParentObject(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveRawObject(tx, srcParent); err != nil {
			return err
		}
		if err := saveRawObject(tx, dstParent); err != nil {
			return err
		}
		if err := saveRawObject(tx, obj); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) DeleteObject(ctx context.Context, srcObj, dstParent, obj *types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if srcObj != nil {
			if srcObj.RefCount == 0 {
				if err := deleteRawObject(tx, srcObj); err != nil {
					return err
				}
			} else {
				if err := saveRawObject(tx, srcObj); err != nil {
					return err
				}
			}
		}
		if err := saveRawObject(tx, dstParent); err != nil {
			return err
		}
		if err := deleteRawObject(tx, obj); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) GetPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		RecordId:   rid,
	}
	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		return res.Error
	}
	return json.Unmarshal([]byte(pd.Content), record)
}

func (e *Entity) ListPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error) {
	result := make([]string, 0)
	res := e.DB.WithContext(ctx).Model(&PluginData{}).Select("record_id").Where("plugin_name = ? AND group_id = ?", plugin.PluginName, groupId).Find(&result)
	return result, res.Error
}

func (e *Entity) SavePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		GroupId:    groupId,
		RecordId:   rid,
	}

	needCreate := false
	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		if res.Error != gorm.ErrRecordNotFound {
			return res.Error
		}
		needCreate = true
	}

	rawContent, err := json.Marshal(record)
	if err != nil {
		return err
	}

	pd.Version = plugin.Version
	pd.Type = plugin.PluginType
	pd.Content = string(rawContent)

	if needCreate {
		res = e.DB.WithContext(ctx).Create(&pd)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}

	res = e.DB.WithContext(ctx).Save(pd)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (e *Entity) DeletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		RecordId:   rid,
	}

	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		return res.Error
	}

	res = e.DB.WithContext(ctx).Delete(pd)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (e *Entity) NextSegmentID(ctx context.Context) (nextID int64, err error) {
	err = e.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nextID, err = availableChunkSegID(tx)
		return err
	})
	return
}

func (e *Entity) InsertChunkSegment(ctx context.Context, seg types.ChunkSeg) (*types.Object, error) {
	var (
		obj    *types.Object
		objMod = &Object{ID: seg.ObjectID}
		err    error
	)
	err = e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.First(objMod)
		if res.Error != nil {
			return res.Error
		}
		obj = objMod.Object()

		res = tx.Create(&ObjectChunk{
			ID:       seg.ID,
			OID:      seg.ObjectID,
			ChunkID:  seg.ChunkID,
			Off:      seg.Off,
			Len:      seg.Len,
			State:    seg.State,
			AppendAt: time.Now().UnixNano(),
		})
		if res.Error != nil {
			return res.Error
		}
		if seg.Off+seg.Len > obj.Size {
			obj.Size = seg.Off + seg.Len
		}
		obj.ModifiedAt = time.Now()

		if writeBackErr := saveRawObject(tx, obj); writeBackErr != nil {
			return writeBackErr
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (e *Entity) DeleteChunkSegment(ctx context.Context, segID int64) error {
	res := e.WithContext(ctx).Delete(&ObjectChunk{ID: segID})
	return res.Error
}

func (e *Entity) ListChunkSegments(ctx context.Context, oid, chunkID int64) ([]types.ChunkSeg, error) {
	segments := make([]ObjectChunk, 0)
	res := e.DB.WithContext(ctx).Where("oid = ? AND chunk_id = ?", oid, chunkID).Order("append_at").Find(&segments)
	if res.Error != nil {
		return nil, res.Error
	}
	result := make([]types.ChunkSeg, len(segments))
	for i, seg := range segments {
		result[i] = types.ChunkSeg{
			ID:       seg.ID,
			ChunkID:  seg.ChunkID,
			ObjectID: seg.OID,
			Off:      seg.Off,
			Len:      seg.Len,
			State:    seg.State,
		}

	}
	return result, nil
}

func (e *Entity) SystemInfo(ctx context.Context) (*SystemInfo, error) {
	info := &SystemInfo{}
	res := e.WithContext(ctx).First(info)
	if res.Error != nil {
		return nil, res.Error
	}
	return info, nil
}

func saveRawObject(tx *gorm.DB, obj *types.Object) error {
	var (
		objModel = &Object{ID: obj.ID}
		extModel = &ObjectExtend{ID: obj.ID}
	)
	res := tx.First(objModel)
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return res.Error
	}

	if res.Error == gorm.ErrRecordNotFound {
		objModel.Update(obj)
		res = tx.Create(objModel)
		if res.Error != nil {
			return res.Error
		}

		extModel.Update(obj)
		res = tx.Create(extModel)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}

	objModel.Update(obj)
	res = tx.Save(objModel)
	if res.Error != nil {
		return res.Error
	}

	if obj.ExtendDataChanged {
		if err := updateObjectExtendData(tx, obj); err != nil {
			return err
		}
	}
	if obj.LabelsChanged {
		if err := updateObjectLabels(tx, obj); err != nil {
			return err
		}
	}
	return nil
}

func deleteRawObject(tx *gorm.DB, obj *types.Object) error {
	var (
		objModel = &Object{ID: obj.ID}
		extModel = &ObjectExtend{ID: obj.ID}
	)
	res := tx.First(objModel)
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return res.Error
	}

	res = tx.Delete(objModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Delete(extModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Where("ref_type = 'object' AND ref_id = ?", obj.ID).Delete(&Label{})
	if res.Error != nil {
		return res.Error
	}

	return nil
}

func availableChunkSegID(tx *gorm.DB) (int64, error) {
	info := &SystemInfo{}
	res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(info)
	if res.Error != nil {
		return 0, res.Error
	}

	info.ChunkSeg += 1
	res = tx.Save(info)
	if res.Error != nil {
		return 0, res.Error
	}

	return info.ChunkSeg, nil
}

func updateObjectExtendData(tx *gorm.DB, obj *types.Object) error {
	var (
		extModel                   = &ObjectExtend{ID: obj.ID}
		objectProperties           = make([]ObjectProperty, 0)
		needCreatePropertiesModels = make([]ObjectProperty, 0)
		needUpdatePropertiesModels = make([]ObjectProperty, 0)
		needDeletePropertiesModels = make([]ObjectProperty, 0)
	)

	extModel.Update(obj)
	res := tx.Save(extModel)
	if res.Error != nil {
		return res.Error
	}

	res = tx.Where("oid = ?", obj.ID).Find(&objectProperties)
	if res.Error != nil {
		return res.Error
	}

	propertiesMap := map[string]string{}
	for k, v := range obj.Properties.Fields {
		propertiesMap[k] = v
	}

	for i := range objectProperties {
		oldKv := objectProperties[i]
		if newV, ok := propertiesMap[oldKv.Name]; ok {
			if oldKv.Value != newV {
				oldKv.Value = newV
				needUpdatePropertiesModels = append(needUpdatePropertiesModels, oldKv)
			}
			delete(propertiesMap, oldKv.Name)
			continue
		}
		needDeletePropertiesModels = append(needDeletePropertiesModels, oldKv)
	}

	for k, v := range propertiesMap {
		needCreatePropertiesModels = append(needCreatePropertiesModels, ObjectProperty{OID: obj.ID, Name: k, Value: v})
	}

	if len(needCreatePropertiesModels) > 0 {
		for i := range needCreatePropertiesModels {
			property := needCreatePropertiesModels[i]
			res = tx.Create(&property)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	if len(needUpdatePropertiesModels) > 0 {
		for i := range needUpdatePropertiesModels {
			property := needUpdatePropertiesModels[i]
			res = tx.Save(&property)
			if res.Error != nil {
				return res.Error
			}
		}
	}

	if len(needDeletePropertiesModels) > 0 {
		for i := range needDeletePropertiesModels {
			property := needDeletePropertiesModels[i]
			res = tx.Delete(&property)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	return nil
}

func updateObjectLabels(tx *gorm.DB, obj *types.Object) error {
	labelModels := make([]Label, 0)
	res := tx.Where("oid = ?", obj.ID).Find(&labelModels)
	if res.Error != nil {
		return res.Error
	}

	labelsMap := map[string]string{}
	for _, kv := range obj.Labels.Labels {
		labelsMap[kv.Key] = kv.Value
	}

	needCreateLabelModels := make([]Label, 0)
	needUpdateLabelModels := make([]Label, 0)
	needDeleteLabelModels := make([]Label, 0)
	for i := range labelModels {
		oldKv := labelModels[i]
		if newV, ok := labelsMap[oldKv.Key]; ok {
			if oldKv.Value != newV {
				oldKv.Value = newV
				needUpdateLabelModels = append(needUpdateLabelModels, oldKv)
			}
			delete(labelsMap, oldKv.Key)
			continue
		}
		needDeleteLabelModels = append(needDeleteLabelModels, oldKv)
	}

	for k, v := range labelsMap {
		needCreateLabelModels = append(needCreateLabelModels, Label{RefType: "object", RefID: obj.ID, Key: k, Value: v, SearchKey: labelSearchKey(k, v)})
	}

	if len(needCreateLabelModels) > 0 {
		for i := range needCreateLabelModels {
			label := needCreateLabelModels[i]
			res = tx.Create(label)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	if len(needUpdateLabelModels) > 0 {
		for i := range needUpdateLabelModels {
			label := needUpdateLabelModels[i]
			res = tx.Save(label)
			if res.Error != nil {
				return res.Error
			}
		}
	}

	if len(needDeleteLabelModels) > 0 {
		for i := range needDeleteLabelModels {
			label := needDeleteLabelModels[i]
			res = tx.Delete(label)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	return nil
}

func listObjectWithLabelMatcher(ctx context.Context, db *gorm.DB, labelMatch types.LabelMatch) ([]*types.Object, error) {
	db = db.WithContext(ctx)
	includeSearchKeys := make([]string, len(labelMatch.Include))
	for i, inKey := range labelMatch.Include {
		includeSearchKeys[i] = labelSearchKey(inKey.Key, inKey.Value)
	}

	var includeLabels []Label
	res := db.Select("oid").Where("search_key IN ?", includeSearchKeys).Find(&includeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	var excludeLabels []Label
	res = db.Select("oid").Where("key IN ?", labelMatch.Exclude).Find(&excludeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	targetObjects := make(map[int64]struct{})
	for _, in := range includeLabels {
		targetObjects[in.RefID] = struct{}{}
	}
	for _, ex := range excludeLabels {
		delete(targetObjects, ex.RefID)
	}

	idList := make([]int64, 0, len(targetObjects))
	for i := range targetObjects {
		idList = append(idList, i)
	}

	result := make([]*types.Object, 0, len(targetObjects))
	res = db.Where("id IN ?", idList).Find(&result)
	if res.Error != nil {
		return nil, res.Error
	}
	return result, nil
}
