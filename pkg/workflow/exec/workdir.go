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

package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

func initWorkdir(ctx context.Context, jobWorkdir string, job *types.WorkflowJob) (string, error) {
	if job.Id == "" {
		return "", fmt.Errorf("job id is empty")
	}
	_, err := os.Stat(jobWorkdir)
	if err != nil {
		return "", fmt.Errorf("base job workdir %s: %s", jobWorkdir, err)
	}

	workdir := path.Join(jobWorkdir, fmt.Sprintf("job-%s", job.Id))
	enInfo, err := os.Stat(workdir)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if enInfo != nil {
		if enInfo.IsDir() {
			return workdir, nil
		}
		return workdir, fmt.Errorf("job workdir is existed and not a dir")
	}

	if err = utils.Mkdir(workdir); err != nil {
		return "", err
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

	return workdir, nil
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

	if entryID == dentry.RootEntryID {
		return utils.PathSeparator, nil
	}

	en, err := entryMgr.GetEntry(ctx, entryID)
	if err != nil {
		return "", err
	}
	reversedURI = append(uriPart, en.Name)

	for en.ID != dentry.RootEntryID {
		en, err = entryMgr.GetEntry(ctx, en.ParentID)
		if err != nil {
			return "", err
		}

		if en.ID == dentry.RootEntryID {
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
	if types.IsGroup(entry.Kind) {
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

func collectFile2BaseEntry(ctx context.Context, entryMgr dentry.Manager, baseEntryId int64, workdir string, tmpEn pluginapi.Entry) error {
	grp, err := entryMgr.OpenGroup(ctx, baseEntryId)
	if err != nil {
		return fmt.Errorf("open base entry group failed: %s", err)
	}

	_, err = grp.FindEntry(ctx, tmpEn.Name)
	if err != nil && err != types.ErrNotFound {
		return fmt.Errorf("check new file existed error: %s", err)
	}

	// TODO: overwrite existed file
	if err == nil {
		return nil
	}

	tmpFile, err := os.Open(path.Join(workdir, tmpEn.Name))
	if err != nil {
		return fmt.Errorf("read temporary file failed: %s", err)
	}
	defer tmpFile.Close()

	var ed *types.ExtendData
	if len(tmpEn.Parameters) > 0 {
		ed = &types.ExtendData{}
		ed.Properties.Fields = map[string]types.PropertyItem{}
		for k, v := range tmpEn.Parameters {
			if strings.HasPrefix(k, pluginapi.ResWorkflowKeyPrefix) {
				continue
			}
			ed.Properties.Fields[k] = types.PropertyItem{Value: v}
		}
	}
	newEn, err := grp.CreateEntry(ctx, types.EntryAttr{Name: tmpEn.Name, Kind: tmpEn.Kind})
	if err != nil {
		return fmt.Errorf("create new entry failed: %s", err)
	}

	if ed != nil {
		// update entry properties
		err = entryMgr.UpdateEntryExtendData(ctx, newEn.ID, *ed)
		if err != nil {
			_ = grp.RemoveEntry(ctx, newEn.ID)
			return fmt.Errorf("update entry extend data failed: %s", err)
		}
	}

	f, err := entryMgr.Open(ctx, newEn.ID, types.OpenAttr{Write: true})
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

func collectFile2Document(ctx context.Context, docMgr document.Manager, entryMgr dentry.Manager, entryId int64, content bytes.Buffer) error {
	baseEn, err := entryMgr.GetEntry(ctx, entryId)
	if err != nil {
		return fmt.Errorf("query entry failed: %s", err)
	}
	doc := &types.Document{
		OID:           baseEn.ID,
		Name:          trimFileExtension(baseEn.Name),
		ParentEntryID: baseEn.ParentID,
		Source:        "collect",
		Content:       content.String(),
	}
	err = docMgr.SaveDocument(ctx, doc)
	if err != nil {
		return fmt.Errorf("create new entry failed: %s", err)
	}

	return nil
}

func trimFileExtension(fileName string) string {
	fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	return strings.TrimSpace(fileName)
}
