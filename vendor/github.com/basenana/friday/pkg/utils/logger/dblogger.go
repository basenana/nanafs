/*
 * Copyright 2023 Friday Author.
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

package logger

import (
	"context"
	"time"

	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

type DBLogger struct {
	Logger
}

func (l *DBLogger) LogMode(level glogger.LogLevel) glogger.Interface {
	return l
}

func (l *DBLogger) Info(ctx context.Context, s string, i ...interface{}) {
	l.Infof(s, i...)
}

func (l *DBLogger) Warn(ctx context.Context, s string, i ...interface{}) {
	l.Warnf(s, i...)
}

func (l *DBLogger) Error(ctx context.Context, s string, i ...interface{}) {
	l.Errorf(s, i...)
}

func (l *DBLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	sqlContent, rows := fc()
	l.Debugf("trace sql: %s\nrows: %s, err: %v", sqlContent, rows, err)
	switch {
	case err != nil && err != gorm.ErrRecordNotFound && err != context.Canceled:
		l.Debugf("trace error, sql: %s\nrows: %s, err: %v", sqlContent, rows, err)
	case time.Since(begin) > time.Second:
		l.Infof("slow sql, sql: %s\nrows: %s, err: %v", sqlContent, rows, err)
	}
}

func NewDbLogger() *DBLogger {
	return &DBLogger{NewLogger("database")}
}
