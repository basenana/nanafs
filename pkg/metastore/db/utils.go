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
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
	"time"
)

func SqlError2Error(err error) error {
	switch err {
	case gorm.ErrRecordNotFound:
		return types.ErrNotFound
	default:
		return err
	}
}

func queryFilter(tx *gorm.DB, filter types.Filter) *gorm.DB {
	if filter.ID != 0 {
		tx = tx.Where("id = ?", filter.ID)
	}
	if filter.ParentID != 0 {
		tx = tx.Where("parent_id = ?", filter.ParentID)
	}
	if filter.RefID != 0 {
		tx = tx.Where("ref_id = ?", filter.RefID)
	}
	if filter.Name != "" {
		tx = tx.Where("name = ?", filter.Name)
	}
	if filter.Namespace != "" {
		tx = tx.Where("namespace = ?", filter.Namespace)
	}
	if filter.Kind != "" {
		tx = tx.Where("kind = ?", filter.Kind)
	}
	return tx
}

func labelSearchKey(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}

type Logger struct {
	*zap.SugaredLogger
}

func (l *Logger) LogMode(level glogger.LogLevel) glogger.Interface {
	return l
}

func (l *Logger) Info(ctx context.Context, s string, i ...interface{}) {
	l.Infof(s, i...)
}

func (l *Logger) Warn(ctx context.Context, s string, i ...interface{}) {
	l.Warnf(s, i...)
}

func (l *Logger) Error(ctx context.Context, s string, i ...interface{}) {
	l.Errorf(s, i...)
}

func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	switch {
	case err != nil && err != gorm.ErrRecordNotFound:
		sqlContent, rows := fc()
		l.Warnw("trace error", "sql", sqlContent, "rows", rows, "err", err)
	case time.Since(begin) > time.Second:
		sqlContent, rows := fc()
		l.Infow("slow sql", "sql", sqlContent, "rows", rows, "err", err)
	}
}

func NewDbLogger() *Logger {
	return &Logger{SugaredLogger: logger.NewLogger("database")}
}

var accessMapping map[types.Permission]int64

func init() {
	accessMapping = map[types.Permission]int64{
		types.PermOthersExec:  1 << 0,
		types.PermOthersRead:  1 << 1,
		types.PermOthersWrite: 1 << 2,
		types.PermOwnerExec:   1 << 3,
		types.PermGroupRead:   1 << 4,
		types.PermGroupWrite:  1 << 5,
		types.PermGroupExec:   1 << 6,
		types.PermOwnerRead:   1 << 7,
		types.PermOwnerWrite:  1 << 8,
		types.PermSetUid:      1 << 9,
		types.PermSetGid:      1 << 10,
		types.PermSticky:      1 << 11,
	}
}

func buildObjectAccess(perm, uid, gid int64) types.Access {
	acc := types.Access{UID: uid, GID: gid}
	for name, permVal := range accessMapping {
		if perm&permVal > 0 {
			acc.Permissions = append(acc.Permissions, name)
		}
	}
	return acc
}

func updateObjectPermission(acc types.Access) int64 {
	var perm int64
	for _, p := range acc.Permissions {
		perm |= accessMapping[p]
	}
	return perm
}