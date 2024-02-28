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

package inbox

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

type Inbox struct {
	entry    dentry.Manager
	workflow workflow.Manager
	logger   *zap.SugaredLogger
}

func (i *Inbox) QuickInbox(ctx context.Context, option Option) (*types.Metadata, error) {
	return nil, nil
}

func New(entry dentry.Manager, workflow workflow.Manager) *Inbox {
	return &Inbox{
		entry:    entry,
		workflow: workflow,
		logger:   logger.NewLogger("inbox"),
	}
}

type Option struct {
	Url         string
	FileType    string
	ClutterFree bool
}
