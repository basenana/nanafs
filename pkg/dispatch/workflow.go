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

package dispatch

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
)

type workflowAction struct {
	entry    dentry.Manager
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

func (w workflowAction) handleEvent(ctx context.Context, evt *types.Event) error {
	//TODO implement me
	panic("implement me")
}

func (w workflowAction) execute(ctx context.Context, task *types.ScheduledTask) error {
	//TODO implement me
	panic("implement me")
}
