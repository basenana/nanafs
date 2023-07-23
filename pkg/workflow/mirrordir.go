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
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"gopkg.in/yaml.v3"
	"io"
	ospath "path"
	"strings"
	"sync"
	"time"
)

const (
	MirrorPluginName          = "workflow"
	MirrorPluginVersion       = "1.0"
	MirrorDirRoot             = "root"
	MirrorDirWorkflows        = "workflows"
	MirrorDirJobs             = "jobs"
	MirrorFileType            = ".yaml"
	MirrorFileMaxSize   int64 = 1 << 22 // 4M
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
	*dirHandler
	*fileHandler
}

var _ plugin.MirrorPlugin = &MirrorPlugin{}

func (m *MirrorPlugin) Name() string {
	return MirrorPluginName
}

func (m *MirrorPlugin) Type() types.PluginType {
	return types.TypeMirror
}

func (m *MirrorPlugin) Version() string {
	return MirrorPluginVersion
}

func (m *MirrorPlugin) build(ctx context.Context, _ types.PluginSpec, scope types.PlugScope) (plugin.Plugin, error) {
	defer m.memfs.release()

	return nil, nil
}

type dirHandler struct {
	mgr     Manager
	path    string
	dirKind string
	wfID    string
	memfs   *memFS
}

func (d *dirHandler) IsGroup(ctx context.Context) (bool, error) {
	if d == nil {
		return false, nil
	}
	return true, nil
}

func (d *dirHandler) FindEntry(ctx context.Context, name string) (*stub.Entry, error) {
	if d == nil {
		return nil, types.ErrNoGroup
	}

	if d.dirKind == MirrorDirRoot {
		switch name {
		case MirrorDirWorkflows, MirrorDirJobs:
			return &stub.Entry{Name: name, Kind: types.ExternalGroupKind, IsGroup: true}, nil
		default:
			return nil, types.ErrNotFound
		}
	}

	if d.memfs.Exist(ospath.Join(d.path, name)) {
		return &stub.Entry{Name: name, Kind: types.RawKind, IsGroup: false}, nil
	}

	if d.dirKind == MirrorDirWorkflows {
		_, err := d.mgr.GetWorkflow(ctx, mirrorFile2ID(name))
		if err != nil {
			return nil, err
		}
		return &stub.Entry{Name: name, Kind: types.RawKind, IsGroup: false}, nil
	}

	if d.dirKind == MirrorDirJobs {
		if d.wfID == "" {
			_, err := d.mgr.GetWorkflow(ctx, name)
			if err != nil {
				return nil, err
			}
			return &stub.Entry{Name: name, Kind: types.RawKind, IsGroup: false}, nil
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
	if d == nil {
		return nil, types.ErrNoGroup
	}
	if d.dirKind == MirrorDirRoot {
		return nil, types.ErrNoAccess
	}
	if d.dirKind == MirrorDirJobs && d.wfID == "" {
		return nil, types.ErrNoAccess
	}

	return &stub.Entry{Name: attr.Name, Kind: attr.Kind, IsGroup: false}, nil
}

func (d *dirHandler) UpdateEntry(ctx context.Context, en *stub.Entry) error {
	if d == nil {
		return types.ErrNoGroup
	}
	return nil
}

func (d *dirHandler) RemoveEntry(ctx context.Context, en *stub.Entry) error {
	if d == nil {
		return types.ErrNoGroup
	}

	path := ospath.Join(d.path, en.Name)
	if d.memfs.Exist(path) {
		d.memfs.Remove(path)
	}

	if d.dirKind == MirrorDirWorkflows {
		wf, err := d.mgr.GetWorkflow(ctx, mirrorFile2ID(en.Name))
		if err != nil {
			return err
		}
		return d.mgr.DeleteWorkflow(ctx, wf.Id)
	}

	return types.ErrNoAccess
}

func (d *dirHandler) ListChildren(ctx context.Context) ([]*stub.Entry, error) {
	if d == nil {
		return nil, types.ErrNoGroup
	}
	children := make([]*stub.Entry, 0)
	switch {
	case d.dirKind == MirrorDirRoot:
		children = append(children,
			&stub.Entry{Name: MirrorDirJobs, Kind: types.ExternalGroupKind, IsGroup: true},
			&stub.Entry{Name: MirrorDirWorkflows, Kind: types.ExternalGroupKind, IsGroup: true})

	case d.dirKind == MirrorDirWorkflows:
		wfList, err := d.mgr.ListWorkflows(ctx)
		if err != nil {
			return children, err
		}
		for _, wf := range wfList {
			children = append(children, &stub.Entry{Name: id2MirrorFile(wf.Id), Kind: types.RawKind, IsGroup: false})
		}
	case d.dirKind == MirrorDirJobs && d.wfID == "":
		wfList, err := d.mgr.ListWorkflows(ctx)
		if err != nil {
			return children, err
		}
		for _, wf := range wfList {
			children = append(children, &stub.Entry{Name: wf.Id, Kind: types.ExternalGroupKind, IsGroup: true})
		}
	case d.dirKind == MirrorDirJobs && d.wfID != "":
		jobList, err := d.mgr.ListJobs(ctx, d.wfID)
		if err != nil {
			return children, err
		}
		for _, j := range jobList {
			children = append(children, &stub.Entry{Name: id2MirrorFile(j.Id), Kind: types.ExternalGroupKind, IsGroup: true})
		}
	}

	for _, f := range d.memfs.ListPrefix(d.path) {
		children = append(children, &stub.Entry{Name: f.name, Kind: types.RawKind, IsGroup: false})
	}

	return children, nil
}

type fileHandler struct {
	mgr           Manager
	path          string
	file          *memFile
	err           error
	dirKind, wfID string
}

func (f *fileHandler) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	if f == nil {
		return 0, types.ErrIsGroup
	}
	n, err := f.file.WriteAt(data, off)
	return int64(n), err
}

func (f *fileHandler) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	if f == nil {
		return 0, types.ErrIsGroup
	}
	n, err := f.file.ReadAt(dest, off)
	return int64(n), err
}

func (f *fileHandler) Close(ctx context.Context) error {
	if f == nil {
		return types.ErrIsGroup
	}
	if strings.HasPrefix(f.file.name, ".") {
		return nil
	}

	switch {
	case f.dirKind == MirrorDirWorkflows:
		f.err = f.createOrUpdateWorkflow(ctx)
	case f.dirKind == MirrorDirJobs && f.wfID != "":
		f.err = f.updateWorkflowJob(ctx)
	}

	if f.err != nil {
		_, _ = fmt.Fprintf(utils.NewWriterWithOffset(f.file, f.file.off), "\n// error: %s\n", f.err)
	}

	return nil
}

func (f *fileHandler) createOrUpdateWorkflow(ctx context.Context) error {
	wf := &types.WorkflowSpec{}
	decodeErr := yaml.NewDecoder(utils.NewReader(f.file)).Decode(wf)
	if decodeErr == nil {
		return decodeErr
	}

	wfID := mirrorFile2ID(f.file.name)
	if err := isValidID(wfID); err != nil {
		return err
	}
	wf.Id = wfID

	oldWf, err := f.mgr.GetWorkflow(ctx, wfID)
	if err != nil && err != types.ErrNotFound {
		return err
	}

	// do create
	if err == types.ErrNotFound {
		wf, err = f.mgr.CreateWorkflow(ctx, initWorkflow(wf))
		if err != nil {
			return err
		}
		f.file.reset()
		_ = yaml.NewEncoder(utils.NewWriter(f.file)).Encode(wf)
		return nil
	}

	// do update
	if wf.Name != "" {
		oldWf.Name = wf.Name
	}
	oldWf.Rule = wf.Rule
	oldWf.Steps = wf.Steps
	oldWf.UpdatedAt = time.Now()

	oldWf, err = f.mgr.UpdateWorkflow(ctx, oldWf)
	if err != nil {
		return err
	}
	f.file.reset()
	_ = yaml.NewEncoder(utils.NewWriter(f.file)).Encode(oldWf)
	return nil
}

func (f *fileHandler) updateWorkflowJob(ctx context.Context) error {
	wfJob := &types.WorkflowJob{}
	decodeErr := yaml.NewDecoder(utils.NewReader(f.file)).Decode(wfJob)
	if decodeErr == nil {
		return decodeErr
	}

	jobID := mirrorFile2ID(f.file.name)
	if err := isValidID(jobID); err != nil {
		return err
	}

	jobs, err := f.mgr.ListJobs(ctx, f.wfID)
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
		if wfJob.Status != oldJob.Status && !oldJob.FinishAt.IsZero() {
			switch {
			case wfJob.Status == flow.PausedStatus && oldJob.Status == flow.RunningStatus:
				err = f.mgr.PauseWorkflowJob(ctx, jobID)
			case wfJob.Status == flow.RunningStatus && oldJob.Status == flow.PausedStatus:
				err = f.mgr.ResumeWorkflowJob(ctx, jobID)
			case wfJob.Status == flow.CanceledStatus:
				err = f.mgr.CancelWorkflowJob(ctx, jobID)
			default:
				err = fmt.Errorf("the current state is %s and cannot be changed to %s", oldJob.Status, wfJob.Status)
			}
		}
		return err
	}

	// do create
	target := wfJob.Target
	if target.EntryID != nil {
		wfJob, err = f.mgr.TriggerWorkflow(ctx, f.wfID, *target.EntryID)
		if err != nil {
			return err
		}
		encodeErr := yaml.NewEncoder(utils.NewWriter(f.file)).Encode(wfJob)
		if encodeErr != nil {
			return encodeErr
		}
		return nil
	}
	return nil
}

func (f *fileHandler) Fsync(ctx context.Context) error {
	if f == nil {
		return types.ErrIsGroup
	}
	return nil
}

func (f *fileHandler) Flush(ctx context.Context) error {
	if f == nil {
		return types.ErrIsGroup
	}
	return nil
}

func buildWorkflowMirrorPlugin(mgr Manager) plugin.Builder {
	mp := &MirrorPlugin{dirHandler: &dirHandler{mgr: mgr, path: "/", dirKind: MirrorDirRoot}}
	return mp.build
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

type memFS struct {
	files  map[string]*memFile
	mux    sync.Mutex
	scanAt time.Time
}

func (m *memFS) Exist(path string) bool {
	m.mux.Lock()
	defer m.mux.Unlock()
	_, ok := m.files[path]
	return ok
}

func (m *memFS) Open(path string) *memFile {
	m.mux.Lock()
	defer m.mux.Unlock()
	f, ok := m.files[path]
	if !ok {
		f = &memFile{name: ospath.Base(path), data: utils.NewMemoryBlock(MirrorFileMaxSize / 4), cachedAt: time.Now()}
	}
	return f
}

func (m *memFS) ListPrefix(path string) []*memFile {
	m.mux.Lock()
	defer m.mux.Unlock()

	result := make([]*memFile, 0)
	for p := range m.files {
		if ospath.Dir(p) == path {
			result = append(result, m.files[p])
		}
	}
	return result
}

func (m *memFS) Remove(path string) {
	m.mux.Lock()
	defer m.mux.Unlock()

	f, ok := m.files[path]
	if !ok {
		return
	}

	utils.ReleaseMemoryBlock(f.data)
	f.data = nil
	delete(m.files, path)
}

func (m *memFS) release() {
	m.mux.Lock()
	defer m.mux.Unlock()

	if time.Since(m.scanAt) < time.Hour {
		return
	}
	m.scanAt = time.Now()

	for k, f := range m.files {
		if time.Since(f.cachedAt) < time.Hour {
			continue
		}
		utils.ReleaseMemoryBlock(f.data)
		f.data = nil
		delete(m.files, k)
	}
}

type memFile struct {
	name     string
	data     []byte
	off      int64
	cachedAt time.Time
}

func (m *memFile) WriteAt(data []byte, off int64) (n int, err error) {
	if m.data == nil {
		err = types.ErrNotFound
		return
	}
	if off+int64(len(data)) > int64(len(data)) {
		if off+int64(len(data)) > MirrorFileMaxSize {
			return 0, io.ErrShortBuffer
		}
		blk := utils.NewMemoryBlock(off + int64(len(data)) + 1)
		copy(blk, m.data[:m.off])
		utils.ReleaseMemoryBlock(m.data)
		m.data = blk
	}
	return copy(m.data[off:], data), nil
}

func (m *memFile) ReadAt(dest []byte, off int64) (n int, err error) {
	if m.data == nil {
		err = types.ErrNotFound
		return
	}
	if off >= m.off {
		return 0, io.EOF
	}
	return copy(dest, m.data[off:]), nil
}

func (m *memFile) reset() {
	m.off = 0
}

func initMemFS() *memFS {
	return &memFS{files: map[string]*memFile{}}
}
