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

package fs

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.uber.org/zap"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

const (
	fsName = "nanafs"
)

type NanaFS struct {
	controller.Controller

	Path      string
	Display   string
	MountOpts []string

	cfg    config.FUSE
	logger *zap.SugaredLogger

	// debug will enable debug log and SingleThreaded
	debug bool
}

func (n *NanaFS) Start(stopCh chan struct{}) error {
	root, err := n.newFsNode(context.Background(), nil, nil)
	if err != nil {
		return err
	}

	opt := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther:     true,
			FsName:         fsName,
			Name:           fsName,
			Options:        fsMountOptions(n.Display, n.MountOpts),
			SingleThreaded: n.debug,
		},
		Logger: logger.NewFuseLogger(),
	}

	if n.cfg.EntryTimeout != nil {
		entryTimeout := time.Duration(*n.cfg.EntryTimeout) * time.Second
		opt.EntryTimeout = &entryTimeout
	}
	if n.cfg.AttrTimeout != nil {
		attrTimeout := time.Duration(*n.cfg.AttrTimeout) * time.Second
		opt.EntryTimeout = &attrTimeout
	}

	rawFs := fs.NewNodeFS(root, opt)
	server, err := fuse.NewServer(rawFs, n.Path, &opt.MountOptions)
	if err != nil {
		return err
	}
	server.SetDebug(n.cfg.VerboseLog)

	go server.Serve()

	go func() {
		<-stopCh
		n.umount(server)
	}()

	waitMount := func() error {
		var (
			timeout = time.NewTimer(time.Minute)
			finish  = make(chan struct{})
		)
		defer timeout.Stop()
		go func() {
			n.logger.Infow("waiting mount finish")
			select {
			case <-timeout.C:
				if err = server.Unmount(); err != nil {
					n.logger.Errorw("mount timeout and clean mount point failed", "err", err.Error())
				}
				n.logger.Panicw("wait mount timeout")
			case <-finish:
				n.logger.Infow("fs mounted")
				return
			}
		}()
		if err := server.WaitMount(); err != nil {
			return err
		}
		close(finish)
		return nil
	}
	return waitMount()
}

func (n *NanaFS) SetDebug(debug bool) {
	n.logger.Warn("enable debug mode")
	n.debug = debug
}

func (n *NanaFS) newFsNode(ctx context.Context, parent *NanaNode, entry *types.Metadata) (*NanaNode, error) {
	if parent == nil {
		var err error
		entry, err = n.LoadRootEntry(ctx)
		if err != nil {
			return nil, err
		}
	}

	node := &NanaNode{
		oid:    entry.ID,
		R:      n,
		logger: n.logger.With(zap.Int64("entry", entry.ID)),
	}
	if parent != nil {
		parent.NewInode(ctx, node, idFromStat(nanaNode2Stat(entry)))
	}

	return node, nil
}

func (n *NanaFS) umount(server *fuse.Server) {
	n.logger.Infof("umount %s", n.Path)
	err := server.Unmount()
	if err == nil {
		return
	}

	n.logger.Errorw("umount failed, try again ", "err", err)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("umount", "-f", n.Path)
	case "linux":
		cmd = exec.Command("umount", "-l", n.Path)
	}

	if err := cmd.Run(); err != nil {
		n.logger.Errorw("umount failed", "err", err.Error())
	}
	n.logger.Info("umount finish")
}

func (n *NanaFS) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	return n.Controller.GetEntry(ctx, id)
}

func (n *NanaFS) GetSourceEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	var (
		entry   *types.Metadata
		err     error
		entryId = id
	)
	for {
		entry, err = n.Controller.GetEntry(ctx, entryId)
		if err != nil {
			return nil, err
		}
		if entry.RefID == 0 || entry.RefID == entry.ID {
			return entry, nil
		}
		entryId = entry.RefID
	}
}

func (n *NanaFS) releaseFsNode(ctx context.Context, entryId int64) {
}

func NewNanaFsRoot(cfg config.FUSE, controller controller.Controller) (*NanaFS, error) {
	var st syscall.Stat_t
	err := syscall.Stat(cfg.RootPath, &st)
	if err != nil {
		return nil, err
	}

	if cfg.DisplayName == "" {
		cfg.DisplayName = fsName
	}

	MountDev = uint64(st.Dev)
	root := &NanaFS{
		Controller: controller,
		Path:       cfg.RootPath,
		Display:    cfg.DisplayName,
		MountOpts:  cfg.MountOptions,
		cfg:        cfg,
		logger:     logger.NewLogger("fs"),
	}

	return root, nil
}
