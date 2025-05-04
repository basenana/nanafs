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

package jobrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/pkg/core"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

func initWorkdir(ctx context.Context, workdir string, job *types.WorkflowJob) error {
	enInfo, err := os.Stat(workdir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if enInfo != nil {
		if enInfo.IsDir() {
			return nil
		}
		return fmt.Errorf("job workdir is existed and not a dir")
	}

	if err = utils.Mkdir(workdir); err != nil {
		return err
	}

	// workdir info
	infoFile := path.Join(workdir, ".workflowinfo.json")
	info := map[string]string{
		"workflow": job.Workflow,
		"job":      job.Id,
		"init_at":  time.Now().Format(time.RFC3339),
	}
	infoRaw, _ := json.Marshal(info)
	_ = os.WriteFile(infoFile, infoRaw, 0655)

	return nil
}

func cleanupWorkdir(ctx context.Context, workdir string) error {
	if workdir == "" {
		return nil
	}
	info, err := os.Stat(workdir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("workdir %s not a dir", workdir)
	}
	return utils.Rmdir(workdir)
}

func entryWorkdirInit(ctx context.Context, namespace string, entryID int64, fsCore core.Core, workdir string) (string, error) {
	entry, err := fsCore.GetEntry(ctx, namespace, entryID)
	if err != nil {
		return "", fmt.Errorf("load entry failed: %s", err)
	}
	if entry.IsGroup {
		return "", fmt.Errorf("entry is a group")
	}

	entryPath := path.Join(workdir, entry.Name)
	enInfo, err := os.Stat(entryPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if enInfo != nil {
		return entryPath, nil
	}

	f, err := fsCore.Open(ctx, namespace, entry.ID, types.OpenAttr{Read: true})
	if err != nil {
		return "", fmt.Errorf("open entry failed: %s", err)
	}
	defer f.Close(ctx)

	if err = copyEntryToJobWorkDir(ctx, entryPath, entry, f); err != nil {
		return "", fmt.Errorf("copy entry file failed: %s", err)
	}
	return entryPath, nil
}

func copyEntryToJobWorkDir(ctx context.Context, entryPath string, entry *types.Entry, file core.RawFile) error {
	if entryPath == "" {
		entryPath = entry.Name
	}
	f, err := os.OpenFile(entryPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := io.Copy(f, utils.NewReaderWithContextReaderAt(ctx, file))
	fmt.Println(n)
	return err
}

func collectFile2BaseEntry(ctx context.Context, namespace string, fsCore core.Core, baseEntryId int64, workdir string, entry *pluginapi.Entry) (*types.Entry, error) {
	isNeedCreate := entry.ID == 0

	if !isNeedCreate && !entry.Overwrite {
		return nil, fmt.Errorf("file %s already exists", entry.Name)
	}

	var (
		result *types.Entry
		err    error
	)
	result, err = fsCore.FindEntry(ctx, namespace, baseEntryId, entry.Name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, fmt.Errorf("check new file existed error: %s", err)
	}

	if isNeedCreate && err == nil {
		// file already existed
		return result, nil
	}

	tmpFile, err := os.Open(path.Join(workdir, entry.Name))
	if err != nil {
		return nil, fmt.Errorf("read temporary file failed: %s", err)
	}
	defer tmpFile.Close()

	var properties = types.Properties{Fields: make(map[string]types.PropertyItem)}
	for k, v := range entry.Parameters {
		if strings.HasPrefix(k, pluginapi.ResWorkflowKeyPrefix) {
			continue
		}
		properties.Fields[k] = types.PropertyItem{Value: v}
	}

	if isNeedCreate {
		result, err = fsCore.CreateEntry(ctx, namespace, baseEntryId, types.EntryAttr{Name: entry.Name, Kind: entry.Kind, Properties: properties})
		if err != nil {
			return nil, fmt.Errorf("create new entry failed: %s", err)
		}
		entry.ID = result.ID
	}

	f, err := fsCore.Open(ctx, namespace, entry.ID, types.OpenAttr{Write: true, Trunc: true})
	if err != nil {
		return nil, fmt.Errorf("open entry file failed: %s", err)
	}
	defer f.Close(ctx)

	_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, f), tmpFile)
	if err != nil {
		return nil, fmt.Errorf("copy temporary file to entry file failed: %s", err)
	}

	return result, nil
}

func collectFile2Document(ctx context.Context, docMgr document.Manager, baseEn *types.Entry, document *pluginapi.Document) error {
	unread := true
	doc := &types.Document{
		EntryId:       baseEn.ID,
		Name:          document.Title,
		Namespace:     baseEn.Namespace,
		ParentEntryID: baseEn.ParentID,
		Source:        "collect",
		Content:       document.Content,
		Unread:        &unread,
		CreatedAt:     baseEn.CreatedAt,
		ChangedAt:     baseEn.ChangedAt,
	}
	if !document.PublicAt.IsZero() {
		doc.CreatedAt = document.PublicAt
		doc.ChangedAt = document.PublicAt
	}
	err := docMgr.CreateDocument(ctx, baseEn.Namespace, doc)
	if err != nil {
		return fmt.Errorf("create new entry failed: %s", err)
	}

	return nil
}
