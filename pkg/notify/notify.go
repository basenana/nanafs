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

package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

type Notify struct {
	store metastore.NotificationRecorder
}

func (n *Notify) ListNotifications(ctx context.Context) ([]types.Notification, error) {
	return n.store.ListNotifications(ctx)
}

func (n *Notify) RecordInfo(ctx context.Context, title, message, source string) error {
	nid := fmt.Sprintf("%d-info-%s", time.Now().UnixNano(), utils.MustRandString(8))
	ns := types.GetNamespace(ctx).String()
	no := types.Notification{
		ID:        nid,
		Namespace: ns,
		Title:     title,
		Message:   message,
		Type:      types.NotificationInfo,
		Source:    source,
		Status:    types.NotificationUnread,
		Time:      time.Now(),
	}
	return n.store.RecordNotification(ctx, nid, no)
}

func (n *Notify) RecordWarn(ctx context.Context, title, message, source string) error {
	nid := fmt.Sprintf("%d-warn-%s", time.Now().UnixNano(), utils.MustRandString(8))
	ns := types.GetNamespace(ctx).String()
	no := types.Notification{
		ID:        nid,
		Namespace: ns,
		Title:     title,
		Message:   message,
		Type:      types.NotificationWarn,
		Source:    source,
		Status:    types.NotificationUnread,
		Time:      time.Now(),
	}
	return n.store.RecordNotification(ctx, nid, no)
}

func (n *Notify) MarkRead(ctx context.Context, nid string) error {
	return n.store.UpdateNotificationStatus(ctx, nid, types.NotificationRead)
}

func NewNotify(s metastore.NotificationRecorder) *Notify {
	n := &Notify{store: s}
	registerEventHandle(n)
	return n
}
