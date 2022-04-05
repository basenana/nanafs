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
	"sync"
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

	nodes  map[string]*NanaNode
	logger *zap.SugaredLogger
	debug  bool
	mux    sync.Mutex
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
			Options:    []string{fmt.Sprintf("volname=%s", n.Display)},
		},
		EntryTimeout: &entryTimeout,
		AttrTimeout:  &attrTimeout,
		Logger:       logger.NewFuseLogger(),
	}
	server, err := fs.Mount(n.Path, root, opt)
	if err != nil {
		return err
	}
	//server.SetDebug(n.debug)
	go func() {
		<-stopCh
		_ = server.Unmount()
	}()
	return server.WaitMount()
}

func (n *NanaFS) SetDebug(debug bool) {
	n.debug = debug
}

func (n *NanaFS) newFsNode(ctx context.Context, parent *NanaNode, entry *types.Object) (*NanaNode, error) {
	if parent == nil {
		var err error
		entry, err = n.LoadRootEntry(ctx)
		if err != nil {
			return nil, err
		}
	}

	n.mux.Lock()
	node, ok := n.nodes[entry.ID]
	if !ok {
		node = &NanaNode{
			entry:  entry,
			R:      n,
			logger: n.logger.With(zap.String("obj", entry.ID)),
		}
		if parent != nil {
			parent.NewInode(ctx, node, idFromStat(n.Dev, nanaNode2Stat(node)))
		}
		n.nodes[entry.ID] = node
	}
	n.mux.Unlock()

	return node, nil
}

func (n *NanaFS) releaseFsNode(ctx context.Context, entry *types.Object) {
	n.mux.Lock()
	_, ok := n.nodes[entry.ID]
	if ok {
		delete(n.nodes, entry.ID)
	}
	n.mux.Unlock()
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
		nodes:      map[string]*NanaNode{},
		logger:     logger.NewLogger("fs"),
	}

	return root, nil
}
