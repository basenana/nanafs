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
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/dentry"
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

func entryURIByEntryID(ctx context.Context, entryID int64, entryMgr dentry.Manager) (string, error) {
	var (
		uriPart, reversedURI []string
	)

	root, err := entryMgr.Root(ctx)
	if err != nil {
		return "", err
	}

	if entryID == root.ID {
		return utils.SafetyFilePathJoin(utils.PathSeparator, root.Name), nil
	}
	reversedURI = append(uriPart, root.Name)

	en, err := entryMgr.GetEntry(ctx, entryID)
	if err != nil {
		return "", err
	}
	reversedURI = append(uriPart, en.Name)

	for en.ID != root.ID {
		en, err = entryMgr.GetEntry(ctx, en.ParentID)
		if err != nil {
			return "", err
		}

		if en.ID == root.ID {
			break
		}

		reversedURI = append(reversedURI, en.Name)
	}

	for i := len(reversedURI) - 1; i >= 0; i-- {
		uriPart = append(uriPart, reversedURI[i])
	}

	return strings.Join([]string{"/", strings.Join(uriPart, utils.PathSeparator)}, ""), nil
}

func entryWorkdirInit(ctx context.Context, entryID int64, entryMgr dentry.Manager, workdir string) (string, error) {
	entry, err := entryMgr.GetEntry(ctx, entryID)
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

	f, err := entryMgr.Open(ctx, entry.ID, types.OpenAttr{Read: true})
	if err != nil {
		return "", fmt.Errorf("open entry failed: %s", err)
	}
	defer f.Close(ctx)

	if err = copyEntryToJobWorkDir(ctx, entryPath, entry, f); err != nil {
		return "", fmt.Errorf("copy entry file failed: %s", err)
	}
	return entryPath, nil
}

func copyEntryToJobWorkDir(ctx context.Context, entryPath string, entry *types.Metadata, file dentry.File) error {
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

func collectFile2BaseEntry(ctx context.Context, entryMgr dentry.Manager, baseEntryId int64, workdir string, entry *pluginapi.Entry) error {
	isNeedCreate := entry.ID == 0

	grp, err := entryMgr.OpenGroup(ctx, baseEntryId)
	if err != nil {
		return fmt.Errorf("open base entry group failed: %s", err)
	}

	_, err = grp.FindEntry(ctx, entry.Name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return fmt.Errorf("check new file existed error: %s", err)
	}

	if isNeedCreate && err == nil {
		// file already existed
		return nil
	}

	tmpFile, err := os.Open(path.Join(workdir, entry.Name))
	if err != nil {
		return fmt.Errorf("read temporary file failed: %s", err)
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
		newEn, err := grp.CreateEntry(ctx, types.EntryAttr{Name: entry.Name, Kind: entry.Kind, Properties: properties})
		if err != nil {
			return fmt.Errorf("create new entry failed: %s", err)
		}
		entry.ID = newEn.ID
	}

	f, err := entryMgr.Open(ctx, entry.ID, types.OpenAttr{Write: true, Trunc: true})
	if err != nil {
		return fmt.Errorf("open entry file failed: %s", err)
	}
	defer f.Close(ctx)

	_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, f), tmpFile)
	if err != nil {
		return fmt.Errorf("copy temporary file to entry file failed: %s", err)
	}

	return nil
}

func collectFile2Document(ctx context.Context, docMgr document.Manager, entryMgr dentry.Manager, entryId int64, document *pluginapi.Document) error {
	baseEn, err := entryMgr.GetEntry(ctx, entryId)
	if err != nil {
		return fmt.Errorf("query entry failed: %s", err)
	}
	unread := true
	doc := &types.Document{
		EntryId:       entryId,
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
	err = docMgr.CreateDocument(ctx, doc)
	if err != nil {
		return fmt.Errorf("create new entry failed: %s", err)
	}

	return nil
}
