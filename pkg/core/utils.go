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

package core

import (
	"context"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"github.com/hyponet/eventbus"
	"sync"
	"syscall"
	"time"
)

const (
	RootEntryID           = 1
	RootEntryName         = "root"
	defaultLFUCacheExpire = time.Minute * 10
	defaultLFUCacheSize   = 1 << 15
)

var (
	fsInfoCache       *Info
	fsInfoNextFetchAt time.Time
)

type ik struct {
	namespace string
	id        int64
}

func initRootEntry() *types.Entry {
	acc := &types.Access{
		Permissions: []types.Permission{
			types.PermOwnerRead,
			types.PermOwnerWrite,
			types.PermOwnerExec,
			types.PermGroupRead,
			types.PermGroupWrite,
			types.PermOthersRead,
		},
	}
	root, _ := types.InitNewEntry(nil, types.EntryAttr{Name: RootEntryName, Kind: types.GroupKind, Access: acc})
	root.ID = RootEntryID
	root.ParentID = root.ID
	root.Namespace = types.DefaultNamespace
	return root
}

func initNamespaceRootEntry(root *types.Entry, ns string) *types.Entry {
	acc := &types.Access{
		Permissions: []types.Permission{
			types.PermOwnerRead,
			types.PermOwnerWrite,
			types.PermOwnerExec,
			types.PermGroupRead,
			types.PermGroupWrite,
			types.PermOthersRead,
		},
	}
	nsRoot, _ := types.InitNewEntry(root, types.EntryAttr{Name: ns, Kind: types.GroupKind, Access: acc})
	nsRoot.Namespace = ns
	nsRoot.ParentID = root.ID
	return nsRoot
}

func modeFromFileKind(kind types.Kind) uint32 {
	switch kind {
	case types.RawKind:
		return syscall.S_IFREG
	case types.GroupKind, types.ExternalGroupKind:
		return syscall.S_IFDIR
	case types.SymLinkKind:
		return syscall.S_IFLNK
	case types.FIFOKind:
		return syscall.S_IFIFO
	case types.SocketKind:
		return syscall.S_IFSOCK
	case types.BlkDevKind:
		return syscall.S_IFBLK
	case types.CharDevKind:
		return syscall.S_IFCHR
	default:
		return syscall.S_IFREG
	}
}

type entryEvent struct {
	namespace  string
	entryID    int64
	topicNS    string
	actionType string
}

var eventQ = make(chan *entryEvent, 8)

func publicEntryActionEvent(topicNS, actionType, namespace string, entryID int64) {
	eventQ <- &entryEvent{namespace: namespace, entryID: entryID, topicNS: topicNS, actionType: actionType}
}

func (c *core) entryActionEventHandler() {
	c.logger.Debugw("start entryActionEventHandler")
	for evt := range eventQ {
		if evt.entryID == 0 {
			c.logger.Errorw("handle entry event error: entry id is empty", "entry", evt.entryID, "action", evt.actionType)
			continue
		}
		en, err := c.store.GetEntry(context.Background(), evt.namespace, evt.entryID)
		if err != nil {
			c.logger.Errorw("encounter error when handle entry event", "namespace", evt.namespace, "entry", evt.entryID, "action", evt.actionType, "err", err)
			continue
		}
		eventbus.Publish(events.NamespacedTopic(evt.topicNS, evt.actionType), BuildEntryEvent(evt.actionType, en))
	}
}

func BuildEntryEvent(actionType string, entry *types.Entry) *types.Event {
	return &types.Event{
		Id:              uuid.New().String(),
		Namespace:       entry.Namespace,
		Type:            actionType,
		Source:          "fsCore",
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "entry",
		RefID:           entry.ID,
		DataContentType: "application/json",
		Data:            types.NewEventDataFromEntry(entry),
	}
}

func SetupShutdownHandler(stopCh chan struct{}) chan struct{} {
	shutdownSafe := make(chan struct{})
	go func() {
		<-stopCh
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer wg.Done()
			MustCloseAll()
		}()

		wg.Wait()
		close(shutdownSafe)
	}()
	return shutdownSafe
}
