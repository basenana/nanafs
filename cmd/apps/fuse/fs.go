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

package fuse

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
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
	*core.FileSystem

	Path      string
	Display   string
	MountOpts []string

	cfg    config.FUSE
	logger *zap.SugaredLogger

	// debug will enable debug log and SingleThreaded
	debug bool
}

func (n *NanaFS) Start(stopCh chan struct{}) error {
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

	root, err := n.rootNode()
	if err != nil {
		return err
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
				n.logger.Infow("fuse mounted")
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

func (n *NanaFS) rootNode() (*NanaNode, error) {
	root, err := n.FileSystem.Root(context.Background())
	if err != nil {
		return nil, err
	}
	return n.newFsNode(root.Name, root), nil
}

func (n *NanaFS) newFsNode(name string, entry *types.Entry) *NanaNode {
	node := &NanaNode{
		name:    name,
		entryID: entry.ID,
		entry:   entry,
		R:       n,
		logger:  n.logger.With(zap.Int64("entry", entry.ID)),
	}
	return node
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

func NewNanaFsRoot(fs *core.FileSystem, cfg config.FUSE) (*NanaFS, error) {
	var st syscall.Stat_t
	err := syscall.Stat(cfg.RootPath, &st)
	if err != nil {
		return nil, err
	}

	if cfg.DisplayName == "" {
		cfg.DisplayName = fsName
	}

	MountDev = uint64(st.Dev)
	nfs := &NanaFS{
		FileSystem: fs,
		Path:       cfg.RootPath,
		Display:    cfg.DisplayName,
		MountOpts:  cfg.MountOptions,
		cfg:        cfg,
		logger:     logger.NewLogger("fuse"),
	}

	return nfs, nil
}

func Run(stopCh chan struct{}, fs *core.FileSystem, cfg config.FUSE, debug bool) error {
	if !cfg.Enable {
		return nil
	}
	fsServer, err := NewNanaFsRoot(fs, cfg)
	if err != nil {
		panic(err)
	}
	fsServer.SetDebug(debug)
	err = fsServer.Start(stopCh)
	if err != nil {
		panic(err)
	}
	return nil
}
