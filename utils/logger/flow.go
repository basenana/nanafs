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
	"github.com/basenana/go-flow/utils"
	"go.uber.org/zap"
)

func NewFlowLogger(l *zap.SugaredLogger) utils.Logger {
	fLogger := FlowLogger{logger: l}
	utils.SetLogger(fLogger)
	return fLogger
}

type FlowLogger struct {
	logger *zap.SugaredLogger
}

func (f FlowLogger) Error(i interface{}) {
	f.logger.Error(i)
}

func (f FlowLogger) Errorf(s string, i ...interface{}) {
	f.logger.Errorf(s, i...)
}

func (f FlowLogger) Warn(i interface{}) {
	f.logger.Warn(i)
}

func (f FlowLogger) Warnf(s string, i ...interface{}) {
	f.logger.Warnf(s, i...)
}

func (f FlowLogger) Info(i interface{}) {
	f.logger.Info(i)
}

func (f FlowLogger) Infof(s string, i ...interface{}) {
	f.logger.Infof(s, i...)
}

func (f FlowLogger) Debug(i interface{}) {
	f.logger.Debug(i)
}

func (f FlowLogger) Debugf(s string, i ...interface{}) {
	f.logger.Debugf(s, i...)
}

func (f FlowLogger) With(s string) utils.Logger {
	return FlowLogger{logger: f.logger.Named(s)}
}
