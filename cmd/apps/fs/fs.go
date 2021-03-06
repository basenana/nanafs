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
	Dev       uint64
	Display   string
	MountOpts []string

	cfg    config.Fs
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
	server.SetDebug(n.debug)

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

func (n *NanaFS) newFsNode(ctx context.Context, parent *NanaNode, obj *types.Object) (*NanaNode, error) {
	if parent == nil {
		var err error
		obj, err = n.LoadRootObject(ctx)
		if err != nil {
			return nil, err
		}
	}

	if obj.Dev == 0 {
		obj.Dev = int64(n.Dev)
	}

	node := &NanaNode{
		oid:    obj.ID,
		R:      n,
		logger: n.logger.With(zap.String("obj", obj.ID)),
	}
	if parent != nil {
		parent.NewInode(ctx, node, idFromStat(n.Dev, nanaNode2Stat(obj)))
	}

	return node, nil
}

func (n *NanaFS) umount(server *fuse.Server) {
	go func() {
		_ = server.Unmount()
	}()

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
}

func (n *NanaFS) GetObject(ctx context.Context, id string) (*types.Object, error) {
	return n.Controller.GetObject(ctx, id)
}

func (n *NanaFS) releaseFsNode(ctx context.Context, obj *types.Object) {
}

func NewNanaFsRoot(cfg config.Fs, controller controller.Controller) (*NanaFS, error) {
	var st syscall.Stat_t
	err := syscall.Stat(cfg.RootPath, &st)
	if err != nil {
		return nil, err
	}

	if cfg.DisplayName == "" {
		cfg.DisplayName = fsName
	}

	root := &NanaFS{
		Controller: controller,
		Path:       cfg.RootPath,
		Display:    cfg.DisplayName,
		MountOpts:  cfg.MountOptions,
		cfg:        cfg,
		Dev:        uint64(st.Dev),
		logger:     logger.NewLogger("fs"),
	}

	return root, nil
}
