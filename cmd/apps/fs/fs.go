package fs

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.uber.org/zap"
	"syscall"
	"time"
)

const (
	fsName           = "nanafs"
	defaultFsTimeout = time.Second
)

type NanaFS struct {
	controller.Controller

	Path    string
	Dev     uint64
	Display string

	logger *zap.SugaredLogger
	debug  bool
}

func (n *NanaFS) Start(stopCh chan struct{}) error {
	root, err := n.newFsNode(context.Background(), nil, nil)
	if err != nil {
		return err
	}

	var (
		entryTimeout = defaultFsTimeout
		attrTimeout  = defaultFsTimeout
	)
	opt := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: true,
			FsName:     fsName,
			Name:       fsName,
			Options:    []string{fmt.Sprintf("volname=%s", n.Display), "force"},
		},
		EntryTimeout: &entryTimeout,
		AttrTimeout:  &attrTimeout,
		Logger:       logger.NewFuseLogger(),
	}

	rawFs := fs.NewNodeFS(root, opt)
	server, err := fuse.NewServer(rawFs, n.Path, &opt.MountOptions)
	if err != nil {
		return err
	}
	//server.SetDebug(n.debug)

	go server.Serve()

	go func() {
		<-stopCh
		_ = server.Unmount()
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

	node := &NanaNode{
		obj:    obj,
		R:      n,
		logger: n.logger.With(zap.String("obj", obj.ID)),
	}
	if parent != nil {
		parent.NewInode(ctx, node, idFromStat(n.Dev, nanaNode2Stat(node)))
	}

	return node, nil
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
		Dev:        uint64(st.Dev),
		logger:     logger.NewLogger("fs"),
	}

	return root, nil
}
