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

package dentry

import (
	"context"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hyponet/eventbus/bus"
	"go.uber.org/zap"
)

type lifecycle struct {
	mgr    *manager
	logger *zap.SugaredLogger
}

func newLifecycle(mgr *manager) *lifecycle {
	return &lifecycle{
		mgr:    mgr,
		logger: logger.NewLogger("entryLifecycle"),
	}
}

func (l *lifecycle) initHooks() {
	var err error
	_, err = bus.Subscribe("object.entry.*.destroy", l.cleanChunks)
	if err != nil {
		l.logger.Errorw("subscribe object destroy topic failed", "err", err)
	}
}
func (l *lifecycle) cleanChunks(en Entry) {
	if en.IsGroup() {
		return
	}

	md := en.Metadata()
	s, ok := l.mgr.storages[md.Storage]
	if !ok {
		return
	}

	cs, ok := l.mgr.store.(storage.ChunkStore)
	if !ok {
		return
	}

	err := bio.DeleteChunksData(context.TODO(), md, cs, s)
	if err != nil {
		l.logger.Errorw("[cleanChunks] delete chunk data failed", "err", err)
	}
}