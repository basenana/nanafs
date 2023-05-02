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
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sync"
)

var (
	entryLifecycleLock sync.RWMutex
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
	_, err = events.Subscribe(events.EntryActionTopic(events.TopicEntryActionFmt, events.ActionTypeDestroy), l.cleanChunks)
	if err != nil {
		l.logger.Errorw("subscribe object destroy topic failed", "err", err)
	}
	_, err = events.Subscribe(events.EntryActionTopic(events.TopicFileActionFmt, events.ActionTypeClose), l.handleFileClose)
	if err != nil {
		l.logger.Errorw("subscribe object destroy topic failed", "err", err)
	}
}

func (l *lifecycle) handleFileClose(evt types.Event) {
	md := evt.Data.(*types.Metadata)
	if md.ParentID == 0 && md.RefCount == 0 && !IsFileOpened(md.ID) {
		l.logger.Infow("destroy closed and deleted entry", "entry", md.ID)
		l.cleanChunks(evt)
		if err := l.mgr.store.DestroyObject(context.TODO(), nil, nil, &types.Object{Metadata: *md}); err != nil {
			l.logger.Errorw("clean closed and deleted object record failed", "entry", md.ID, "err", err)
		}
		return
	}

	if IsFileOpened(md.ID) {
		s, ok := l.mgr.storages[md.Storage]
		if !ok {
			return
		}
		if err := bio.CompactChunksData(context.TODO(), md, l.mgr.store.(metastore.ChunkStore), s); err != nil {
			l.logger.Errorw("compact chunk data failed", "entry", md.ID, "err", err)
		}
		return
	}
}

func (l *lifecycle) cleanChunks(evt types.Event) {
	md := evt.Data.(*types.Metadata)
	s, ok := l.mgr.storages[md.Storage]
	if !ok {
		return
	}

	cs, ok := l.mgr.store.(metastore.ChunkStore)
	if !ok {
		return
	}

	l.logger.Infow("[cleanChunks] delete chunk data", "entry", md.ID)
	err := bio.DeleteChunksData(context.TODO(), md, cs, s)
	if err != nil {
		l.logger.Errorw("[cleanChunks] delete chunk data failed", "entry", md.ID, "err", err)
	}
}
