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
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"strings"
)

type Inbox struct {
	inboxGroup int64
	entry      dentry.Manager
	workflow   workflow.Manager
	logger     *zap.SugaredLogger
}

func (b *Inbox) QuickInbox(ctx context.Context, fileName string, option Option) (*types.Metadata, error) {
	fileName = utils.SafetyFilePathJoin("", fileName)

	if fileName == "" || option.FileType == "" {
		return nil, fmt.Errorf("filename or file type not set")
	}
	if !strings.HasSuffix(fileName, option.FileType) {
		if option.Data == nil {
			fileName = fmt.Sprintf("%s.url", fileName)
		} else {
			fileName = fmt.Sprintf("%s.%s", fileName, option.FileType)
		}
	}

	fileEn, err := b.entry.CreateEntry(ctx, b.inboxGroup,
		types.EntryAttr{Name: fileName, Kind: types.RawKind})
	if err != nil {
		return nil, err
	}

	file, err := b.entry.Open(ctx, fileEn.ID, types.OpenAttr{Write: true, Create: true})
	if err != nil {
		return nil, err
	}
	defer file.Close(ctx)

	if option.Data != nil {
		b.logger.Infow("write data direct", "entry", fileEn.ID, "filename", fileName)
		if _, err = file.WriteAt(ctx, option.Data, 0); err != nil {
			return nil, err
		}
		return fileEn, nil
	}

	err = UrlFile{
		Url:         option.Url,
		FileType:    option.FileType,
		ClutterFree: option.ClutterFree,
	}.Write(ctx, file)
	if err != nil {
		return nil, err
	}
	return fileEn, nil
}

func New(entry dentry.Manager, workflow workflow.Manager) (*Inbox, error) {
	inboxEn, err := initInboxInternalGroup(entry)
	if err != nil {
		return nil, fmt.Errorf("init inbox internal group group failed %w", err)
	}
	return &Inbox{
		inboxGroup: inboxEn.ID,
		entry:      entry,
		workflow:   workflow,
		logger:     logger.NewLogger("inbox"),
	}, nil
}

type Option struct {
	Url         string
	FileType    string
	ClutterFree bool
	Data        []byte
}
