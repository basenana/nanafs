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
	"bytes"
	"go.uber.org/zap"
	"log"
	"strings"
)

const (
	defaultFuseLoggerBuf = 1024
	fuseLogMessageDelim  = '\n'
)

func NewFuseLogger() *log.Logger {
	if root == nil {
		return nil
	}
	adp := &fuseLogAdaptor{
		b: bytes.NewBuffer(make([]byte, defaultFuseLoggerBuf)),
		l: root.Named("fuse"),
	}
	return log.New(adp, "", log.Ldate|log.Lmicroseconds)
}

type fuseLogAdaptor struct {
	b *bytes.Buffer
	l *zap.SugaredLogger
}

func (f *fuseLogAdaptor) Write(p []byte) (n int, err error) {
	n, err = f.b.Write(p)
	if n == 0 {
		return
	}

	if p[n-1] == fuseLogMessageDelim {
		f.logLine()
	}
	return
}

func (f *fuseLogAdaptor) logLine() {
	var (
		line   string
		logErr error
	)
	line, logErr = f.b.ReadString(fuseLogMessageDelim)
	if logErr != nil {
		f.l.Errorf("read log buf failed: %s", logErr.Error())
		return
	}
	f.l.Info(strings.TrimSpace(line))
}
