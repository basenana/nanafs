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

package workflow

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/flow"
	"github.com/basenana/nanafs/utils"
	"gopkg.in/yaml.v3"
	"path"
	"strings"
	"time"
)

const (
	MirrorPluginName    = "workflow"
	MirrorPluginVersion = "1.0"
	MirrorDirRoot       = "root"
	MirrorDirWorkflows  = "workflows"
	MirrorDirJobs       = "jobs"
	MirrorFileType      = ".yaml"
	MirrorRootDirName   = ".workflow"
)

var mirrorPlugin = types.PluginSpec{
	Name:       MirrorPluginName,
	Version:    MirrorPluginVersion,
	Type:       types.TypeMirror,
	Parameters: map[string]string{},
}

/*
	MirrorPlugin is an implementation of plugin.MirrorPlugin,
	which supports managing workflows using POSIX operations.

	virtual directory structure as follows:
		.
		|--workflows
		  |--<workflow_id>.yaml
		|--jobs
		  |--<workflow_id>
		    |--<job_id>.yaml

*/
type MirrorPlugin struct {
	path string
	fs   *plugin.MemFS
	mgr  Manager

	*dirHandler
	*fileHandler
}

var _ plugin.MirrorPlugin = &MirrorPlugin{}

func (m *MirrorPlugin) Name() string { return MirrorPluginName }

func (m *MirrorPlugin) Type() types.PluginType { return types.TypeMirror }

func (m *MirrorPlugin) Version() string { return MirrorPluginVersion }

func (m *MirrorPlugin) build(ctx context.Context, _ types.PluginSpec, scope types.PlugScope) (plugin.Plugin, error) {
	if scope.Parameters == nil {
		scope.Parameters = map[string]string{}
	}
	enPath := scope.Parameters[types.PlugScopeEntryPath]
	if enPath == "" {
		return nil, fmt.Errorf("path is empty")
	}

	en, err := m.fs.GetEntry(enPath)
	if err != nil {
		return nil, err
	}

	dirKind, wfID, err := parseFilePath(enPath)
	if err != nil {
		return nil, fmt.Errorf("unexcpect dir path %s", dirKind)
	}

	mp := &MirrorPlugin{path: enPath, fs: m.fs, mgr: m.mgr}
	if en.IsGroup {
		mp.dirHandler = &dirHandler{plugin: mp, dirKind: dirKind, wfID: wfID}
	} else {
		mp.fileHandler = &fileHandler{plugin: mp, dirKind: dirKind, wfID: wfID}
	}

	return mp, nil
}

type dirHandler struct {
	plugin  *MirrorPlugin
	dirKind string
	wfID    string
}

func (d *dirHandler) IsGroup(ctx context.Context) (bool, error) {
	en, err := d.plugin.fs.GetEntry(d.plugin.path)
	if err != nil {
		return false, err
	}
	return en.IsGroup, nil
}

func (d *dirHandler) FindEntry(ctx context.Context, name string) (*stub.Entry, error) {
	if d == nil {
		return nil, types.ErrNoGroup
	}

	// memfs cached entry
	en, err := d.plugin.fs.GetEntry(path.Join(d.plugin.path, name))
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	if en != nil {
		return en, nil
	}

	if d.dirKind == MirrorDirRoot {
		switch name {
		case MirrorDirWorkflows, MirrorDirJobs:
			return d.plugin.fs.CreateEntry(d.plugin.path, stub.EntryAttr{Name: name, Kind: types.ExternalGroupKind})
		default:
			return nil, types.ErrNotFound
		}
	}

	if d.dirKind == MirrorDirWorkflows {
		_, err = d.plugin.mgr.GetWorkflow(ctx, mirrorFile2ID(name))
		if err != nil {
			return nil, err
		}
		return d.plugin.fs.CreateEntry(d.plugin.path, stub.EntryAttr{Name: name, Kind: types.RawKind})
	}

	if d.dirKind == MirrorDirJobs {
		if d.wfID == "" {
			_, err := d.plugin.mgr.GetWorkflow(ctx, name)
			if err != nil {
				return nil, err
			}
			return d.plugin.fs.CreateEntry(d.plugin.path, stub.EntryAttr{Name: name, Kind: types.ExternalGroupKind})
		} else {
			jobs, err := d.ListChildren(ctx)
			if err != nil {
				return nil, err
			}
			for i, j := range jobs {
				if j.Name == name {
					return jobs[i], nil
				}
			}
		}
	}
	return nil, types.ErrNotFound
}

func (d *dirHandler) CreateEntry(ctx context.Context, attr stub.EntryAttr) (*stub.Entry, error) {
	if d.dirKind == MirrorDirRoot {
		return nil, types.ErrNoAccess
	}
	if d.dirKind == MirrorDirJobs && d.wfID == "" {
		return nil, types.ErrNoAccess
	}
	if types.IsGroup(attr.Kind) {
		return nil, types.ErrNoAccess
	}
	en, err := d.plugin.fs.CreateEntry(d.plugin.path, attr)
	if err != nil {
		return nil, err
	}
	if en.Parameters == nil {
		en.Parameters = map[string]string{}
	}
	en.Parameters[types.PlugScopeWorkflowID] = d.wfID
	return en, nil
}

func (d *dirHandler) UpdateEntry(ctx context.Context, en *stub.Entry) error {
	return d.plugin.fs.UpdateEntry(d.plugin.path, en)
}

func (d *dirHandler) RemoveEntry(ctx context.Context, en *stub.Entry) error {
	if d == nil {
		return types.ErrNoGroup
	}

	if en.IsGroup {
		return types.ErrNotEmpty
	}

	cachedEn, err := d.plugin.fs.GetEntry(path.Join(d.plugin.path, en.Name))
	if err == nil {
		_ = d.plugin.fs.RemoveEntry(d.plugin.path, cachedEn)
	}

	if d.dirKind == MirrorDirWorkflows {
		wf, err := d.plugin.mgr.GetWorkflow(ctx, mirrorFile2ID(en.Name))
		if err != nil {
			if err == types.ErrNotFound {
				return nil
			}
			return err
		}
		return d.plugin.mgr.DeleteWorkflow(ctx, wf.Id)
	}

	return nil
}

func (d *dirHandler) ListChildren(ctx context.Context) ([]*stub.Entry, error) {
	if d == nil {
		return nil, types.ErrNoGroup
	}
	cachedChild, err := d.plugin.fs.ListChildren(d.plugin.path)
	if err != nil {
		return nil, err
	}

	children := make([]*stub.Entry, 0)
	cachedChildMap := make(map[string]struct{})
	for i, ch := range cachedChild {
		cachedChildMap[ch.Name] = struct{}{}
		children = append(children, cachedChild[i])
	}

	switch {
	case d.dirKind == MirrorDirRoot:

		if _, ok := cachedChildMap[MirrorDirJobs]; !ok {
			child, err := d.plugin.fs.CreateEntry(d.plugin.path, stub.EntryAttr{Name: MirrorDirJobs, Kind: types.ExternalGroupKind})
			if err != nil {
				wfLogger.Errorf("init mirror dir %s error: %s", MirrorDirJobs, err)
				return nil, err
			}
			children = append(children, child)
		}

		if _, ok := cachedChildMap[MirrorDirWorkflows]; !ok {
			child, err := d.plugin.fs.CreateEntry(d.plugin.path, stub.EntryAttr{Name: MirrorDirWorkflows, Kind: types.ExternalGroupKind})
			if err != nil {
				wfLogger.Errorf("init mirror dir %s error: %s", MirrorDirWorkflows, err)
				return nil, err
			}
			children = append(children, child)
		}

	case d.dirKind == MirrorDirWorkflows:
		wfList, err := d.plugin.mgr.ListWorkflows(ctx)
		if err != nil {
			return children, err
		}
		for _, wf := range wfList {
			if _, ok := cachedChildMap[id2MirrorFile(wf.Id)]; !ok {
				child, err := d.plugin.fs.CreateEntry(d.plugin.path, stub.EntryAttr{Name: id2MirrorFile(wf.Id), Kind: types.RawKind})
				if err != nil {
					wfLogger.Errorf("init mirror workflow file %s error: %s", id2MirrorFile(wf.Id), err)
					return nil, err
				}
				children = append(children, child)
			}
		}
	case d.dirKind == MirrorDirJobs && d.wfID == "":
		wfList, err := d.plugin.mgr.ListWorkflows(ctx)
		if err != nil {
			return children, err
		}
		for _, wf := range wfList {
			if _, ok := cachedChildMap[wf.Id]; !ok {
				child, err := d.plugin.fs.CreateEntry(d.plugin.path, stub.EntryAttr{Name: wf.Id, Kind: types.ExternalGroupKind})
				if err != nil {
					wfLogger.Errorf("init mirror jobs workflow group %s error: %s", wf.Id, err)
					return nil, err
				}
				children = append(children, child)
			}
		}
	case d.dirKind == MirrorDirJobs && d.wfID != "":
		jobList, err := d.plugin.mgr.ListJobs(ctx, d.wfID)
		if err != nil {
			return children, err
		}
		for _, j := range jobList {
			if _, ok := cachedChildMap[id2MirrorFile(j.Id)]; !ok {
				child, err := d.plugin.fs.CreateEntry(d.plugin.path, stub.EntryAttr{Name: id2MirrorFile(j.Id), Kind: types.RawKind})
				if err != nil {
					wfLogger.Errorf("init mirror job file %s error: %s", id2MirrorFile(j.Id), err)
					return nil, err
				}
				children = append(children, child)
			}
		}
	}

	return children, nil
}

type fileHandler struct {
	plugin        *MirrorPlugin
	dirKind, wfID string
	err           error
}

func (f *fileHandler) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	return f.plugin.fs.WriteAt(f.plugin.path, data, off)
}

func (f *fileHandler) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	return f.plugin.fs.ReadAt(f.plugin.path, dest, off)
}

func (f *fileHandler) Fsync(ctx context.Context) error {
	return nil
}

func (f *fileHandler) Trunc(ctx context.Context) error {
	return f.plugin.fs.Trunc(f.plugin.path)
}

func (f *fileHandler) Close(ctx context.Context) error {
	if f == nil {
		return types.ErrIsGroup
	}

	en, err := f.plugin.fs.GetEntry(f.plugin.path)
	if err != nil {
		return err
	}

	if strings.HasPrefix(en.Name, ".") {
		return nil
	}

	op := "unknown"
	switch {
	case f.dirKind == MirrorDirWorkflows:
		op = "create or update workflow"
		f.err = f.createOrUpdateWorkflow(ctx, en)
	case f.dirKind == MirrorDirJobs && f.wfID != "":
		op = "update workflow job"
		f.err = f.triggerOrUpdateWorkflowJob(ctx, en)
	}

	if f.err != nil {
		wfLogger.Errorf("%s failed: %s", op, f.err)
		_, _ = f.plugin.fs.WriteAt(f.plugin.path, []byte(fmt.Sprintf("\n# error: %s\n", f.err)), en.Size)
	}

	return nil
}

func (f *fileHandler) createOrUpdateWorkflow(ctx context.Context, en *stub.Entry) error {
	wf := &types.WorkflowSpec{}
	decodeErr := yaml.NewDecoder(&memfsFile{filePath: f.plugin.path, entry: en, memfs: f.plugin.fs}).Decode(wf)
	if decodeErr != nil {
		return decodeErr
	}

	wfID := mirrorFile2ID(en.Name)
	if err := isValidID(wfID); err != nil {
		return err
	}
	wf.Id = wfID

	oldWf, err := f.plugin.mgr.GetWorkflow(ctx, wfID)
	if err != nil && err != types.ErrNotFound {
		return err
	}

	// do create
	if err == types.ErrNotFound {
		wf, err = f.plugin.mgr.CreateWorkflow(ctx, initWorkflow(wf))
		if err != nil {
			return err
		}
		_ = f.plugin.fs.Trunc(f.plugin.path)
		return yaml.NewEncoder(&memfsFile{filePath: f.plugin.path, entry: en, memfs: f.plugin.fs}).Encode(wf)
	}

	// do update
	if wf.Name != "" {
		oldWf.Name = wf.Name
	}
	oldWf.Rule = wf.Rule
	oldWf.Steps = wf.Steps
	oldWf.UpdatedAt = time.Now()

	oldWf, err = f.plugin.mgr.UpdateWorkflow(ctx, oldWf)
	if err != nil {
		return err
	}
	_ = f.plugin.fs.Trunc(f.plugin.path)
	_ = yaml.NewEncoder(&memfsFile{filePath: f.plugin.path, entry: en, memfs: f.plugin.fs}).Encode(oldWf)
	return nil
}

func (f *fileHandler) triggerOrUpdateWorkflowJob(ctx context.Context, en *stub.Entry) error {
	wfJob := &types.WorkflowJob{}
	decodeErr := yaml.NewDecoder(&memfsFile{filePath: f.plugin.path, entry: en, memfs: f.plugin.fs}).Decode(wfJob)
	if decodeErr != nil {
		return decodeErr
	}

	jobID := mirrorFile2ID(en.Name)
	if err := isValidID(jobID); err != nil {
		return err
	}

	jobs, err := f.plugin.mgr.ListJobs(ctx, f.wfID)
	if err != nil {
		return err
	}

	var oldJob *types.WorkflowJob
	for i, j := range jobs {
		if j.Id == jobID {
			oldJob = jobs[i]
		}
	}

	// do update
	if oldJob != nil {
		if wfJob.Status != oldJob.Status && oldJob.FinishAt.IsZero() {
			switch {
			case wfJob.Status == flow.PausedStatus && oldJob.Status == flow.RunningStatus:
				err = f.plugin.mgr.PauseWorkflowJob(ctx, jobID)
			case wfJob.Status == flow.RunningStatus && oldJob.Status == flow.PausedStatus:
				err = f.plugin.mgr.ResumeWorkflowJob(ctx, jobID)
			case wfJob.Status == flow.CanceledStatus:
				err = f.plugin.mgr.CancelWorkflowJob(ctx, jobID)
			default:
				err = fmt.Errorf("the current state is %s and cannot be changed to %s", oldJob.Status, wfJob.Status)
			}
		}
		return err
	}

	// do create
	target := wfJob.Target
	wfJob, err = f.plugin.mgr.TriggerWorkflow(ctx, f.wfID, target.EntryID, JobAttr{JobID: jobID})
	if err != nil {
		return err
	}
	encodeErr := yaml.NewEncoder(&memfsFile{filePath: f.plugin.path, entry: en, memfs: f.plugin.fs}).Encode(wfJob)
	if encodeErr != nil {
		return encodeErr
	}
	return nil
}

type memfsFile struct {
	filePath string
	entry    *stub.Entry
	memfs    *plugin.MemFS
	off      int64
}

func (m *memfsFile) Write(p []byte) (int, error) {
	n64, err := m.memfs.WriteAt(m.filePath, p, m.off)
	m.off += n64
	return int(n64), err
}

func (m *memfsFile) Read(p []byte) (int, error) {
	n64, err := m.memfs.ReadAt(m.filePath, p, m.off)
	m.off += n64
	return int(n64), err
}

func buildWorkflowMirrorPlugin(mgr Manager) plugin.Builder {
	mp := &MirrorPlugin{path: "/", fs: plugin.NewMemFS(), mgr: mgr}
	mp.dirHandler = &dirHandler{plugin: mp, dirKind: MirrorDirRoot}

	_, _ = mp.fs.CreateEntry("/", stub.EntryAttr{
		Name: MirrorDirJobs,
		Kind: types.ExternalGroupKind,
	})

	_, _ = mp.fs.CreateEntry("/", stub.EntryAttr{
		Name: MirrorDirWorkflows,
		Kind: types.ExternalGroupKind,
	})
	return mp.build
}

func initWorkflowMirrorDir(root *types.Metadata, entryMgr dentry.Manager) error {
	rootGrp, err := entryMgr.OpenGroup(context.TODO(), root.ID)
	if err != nil {
		return fmt.Errorf("open root group failed: %s", err)
	}
	oldDir, err := rootGrp.FindEntry(context.Background(), ".workflow")
	if err != nil && err != types.ErrNotFound {
		return err
	}
	if oldDir != nil {
		return nil
	}

	_, err = entryMgr.CreateEntry(context.Background(), root.ID, dentry.EntryAttr{
		Name:   MirrorRootDirName,
		Kind:   types.ExternalGroupKind,
		Access: root.Access,
		PlugScope: &types.PlugScope{
			PluginName: MirrorPluginName,
			Version:    MirrorPluginVersion,
			PluginType: types.TypeMirror,
			Parameters: map[string]string{},
		},
	})
	return err
}

func mirrorFile2ID(fileName string) string {
	if strings.HasSuffix(fileName, MirrorFileType) {
		return strings.TrimSuffix(fileName, MirrorFileType)
	}
	return fileName
}

func id2MirrorFile(idStr string) string {
	if strings.HasSuffix(idStr, MirrorFileType) {
		return idStr
	}
	return idStr + MirrorFileType
}

func parseFilePath(enPath string) (dirKind, wfID string, err error) {
	if enPath == "/" {
		dirKind = MirrorDirRoot
		return
	}
	enPath = strings.TrimPrefix(enPath, utils.PathSeparator)
	pathParts := strings.SplitN(enPath, utils.PathSeparator, 3)

	dirKind = pathParts[0]
	if dirKind != MirrorDirJobs && dirKind != MirrorDirWorkflows {
		err = fmt.Errorf("unknown dir %s", dirKind)
		return
	}

	if len(pathParts) > 1 {
		wfID = mirrorFile2ID(pathParts[1])
	}
	return
}
