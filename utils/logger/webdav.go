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

package logger

import (
	"go.uber.org/zap"
	"io/fs"
	"net/http"
)

type WebdavLogger struct {
	logger *zap.SugaredLogger
}

func (l *WebdavLogger) Handle(req *http.Request, err error) {
	if err != nil && err != fs.ErrNotExist {
		l.logger.Errorw(req.URL.Path, "method", req.Method, "err", err)
		return
	}
	l.logger.Infow(req.URL.Path, "method", req.Method)
}

func InitWebdavLogger() *WebdavLogger {
	return &WebdavLogger{logger: root.Named("webdav")}
}
