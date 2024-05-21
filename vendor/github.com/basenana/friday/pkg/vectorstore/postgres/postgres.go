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
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/cdipaolo/goml/base"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/basenana/friday/pkg/models"
	"github.com/basenana/friday/pkg/utils/logger"
	"github.com/basenana/friday/pkg/vectorstore"
	"github.com/basenana/friday/pkg/vectorstore/db"
)

const defaultNamespace = "global"

type PostgresClient struct {
	log     logger.Logger
	dEntity *db.Entity
}

func NewPostgresClient(log logger.Logger, postgresUrl string) (*PostgresClient, error) {
	if log == nil {
		log = logger.NewLogger("database")
	}
	dbObj, err := gorm.Open(postgres.Open(postgresUrl), &gorm.Config{Logger: logger.NewDbLogger(log)})
	if err != nil {
		panic(err)
	}

	dbConn, err := dbObj.DB()
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxIdleConns(5)
	dbConn.SetMaxOpenConns(50)
	dbConn.SetConnMaxLifetime(time.Hour)

	if err = dbConn.Ping(); err != nil {
		return nil, err
	}

	dbEnt, err := db.NewDbEntity(dbObj, Migrate)
	if err != nil {
		return nil, err
	}

	return &PostgresClient{
		log:     log,
		dEntity: dbEnt,
	}, nil
}

func (p *PostgresClient) Store(ctx context.Context, element *models.Element, extra map[string]any) error {
	namespace := ctx.Value("namespace")
	if namespace == nil {
		namespace = defaultNamespace
	}
	return p.dEntity.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if extra == nil {
			extra = make(map[string]interface{})
		}
		extra["name"] = element.Name
		extra["group"] = element.Group

		b, err := json.Marshal(extra)
		if err != nil {
			return err
		}

		var v *Index
		v, err = v.From(element)
		if err != nil {
			return err
		}

		v.Extra = string(b)
		v.Namespace = namespace.(string)

		vModel := &Index{}
		res := tx.Where("namespace = ? AND name = ? AND idx_group = ?", namespace, element.Name, element.Group).First(vModel)
		if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
			return res.Error
		}

		if res.Error == gorm.ErrRecordNotFound {
			v.CreatedAt = time.Now().UnixNano()
			v.ChangedAt = time.Now().UnixNano()
			res = tx.Create(v)
			if res.Error != nil {
				return res.Error
			}
			return nil
		}

		vModel.Update(v)
		res = tx.Where("namespace = ? AND name = ? AND idx_group = ?", namespace, element.Name, element.Group).Updates(vModel)
		if res.Error != nil || res.RowsAffected == 0 {
			if res.RowsAffected == 0 {
				return errors.New("operation conflict")
			}
			return res.Error
		}
		return nil
	})
}

func (p *PostgresClient) Search(ctx context.Context, query models.DocQuery, vectors []float32, k int) ([]*models.Doc, error) {
	namespace := ctx.Value("namespace")
	if namespace == nil {
		namespace = defaultNamespace
	}
	vectors64 := make([]float64, 0)
	for _, v := range vectors {
		vectors64 = append(vectors64, float64(v))
	}
	// query from db
	existIndexes := make([]Index, 0)
	var res *gorm.DB

	res = p.dEntity.WithContext(ctx)
	res = res.Where("namespace = ?", namespace)
	if query.ParentId != 0 {
		res = res.Where("parent_entry_id = ?", query.ParentId)
	}
	if query.Oid != 0 {
		res = res.Where("oid = ?", query.Oid)
	}
	res = res.Find(&existIndexes)
	if res.Error != nil {
		return nil, res.Error
	}

	// knn search
	dists := distances{}
	for _, index := range existIndexes {
		var vector []float64
		err := json.Unmarshal([]byte(index.Vector), &vector)
		if err != nil {
			return nil, err
		}

		dists = append(dists, distance{
			Index: index,
			dist:  base.EuclideanDistance(vector, vectors64),
		})
	}

	sort.Sort(dists)

	minKIndexes := dists
	if k < len(dists) {
		minKIndexes = dists[0:k]
	}
	results := make([]*models.Doc, 0)
	for _, index := range minKIndexes {
		results = append(results, index.ToDoc())
	}

	return results, nil
}

func (p *PostgresClient) Get(ctx context.Context, oid int64, name string, group int) (*models.Element, error) {
	namespace := ctx.Value("namespace")
	if namespace == nil {
		namespace = defaultNamespace
	}
	vModel := &Index{}
	var res *gorm.DB
	if oid == 0 {
		res = p.dEntity.WithContext(ctx).Where("namespace = ? AND name = ? AND idx_group = ?", namespace, name, group).First(vModel)
	} else {
		res = p.dEntity.WithContext(ctx).Where("namespace = ? AND name = ? AND oid = ? AND idx_group = ?", namespace, name, oid, group).First(vModel)
	}
	if res.Error != nil {
		if res.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, res.Error
	}
	return vModel.To()
}

var _ vectorstore.VectorStore = &PostgresClient{}

func (p *PostgresClient) Inited(ctx context.Context) (bool, error) {
	var count int64
	res := p.dEntity.WithContext(ctx).Model(&BleveKV{}).Count(&count)
	if res.Error != nil {
		return false, res.Error
	}

	return count > 0, nil
}

type distance struct {
	Index
	dist float64
}

type distances []distance

func (d distances) Len() int {
	return len(d)
}

func (d distances) Less(i, j int) bool {
	return d[i].dist < d[j].dist
}

func (d distances) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
