package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
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

	Path string
	Dev  uint64

	nodes map[string]*NanaNode
	debug bool
	mux   sync.Mutex
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
			Debug:      n.debug,
			//Options:    []string{"default_permissions"},
		},
		EntryTimeout: &entryTimeout,
		AttrTimeout:  &attrTimeout,
		Logger:       nil,
	}
	server, err := fs.Mount(n.Path, root, opt)
	if err != nil {
		return err
	}
	server.SetDebug(n.debug)
	go func() {
		<-stopCh
		_ = server.Unmount()
	}()
	return server.WaitMount()
}

func (n *NanaFS) SetDebug(debug bool) {
	n.debug = debug
}

func (n *NanaFS) newFsNode(ctx context.Context, parent *NanaNode, entry *dentry.Entry) (*NanaNode, error) {
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
			entry: entry,
			R:     n,
		}
		if parent != nil {
			parent.NewInode(ctx, node, idFromStat(n.Dev, nanaNode2Stat(node)))
		}
		n.nodes[entry.ID] = node
	}
	n.mux.Unlock()

	return node, nil
}

func (n *NanaFS) releaseFsNode(ctx context.Context, entry *dentry.Entry) {
	n.mux.Lock()
	_, ok := n.nodes[entry.ID]
	if ok {
		delete(n.nodes, entry.ID)
	}
	n.mux.Unlock()
}

func NewNanaFsRoot(rootPath string, controller controller.Controller) (*NanaFS, error) {
	var st syscall.Stat_t
	err := syscall.Stat(rootPath, &st)
	if err != nil {
		return nil, err
	}

	root := &NanaFS{
		Controller: controller,
		Path:       rootPath,
		Dev:        uint64(st.Dev),
		nodes:      map[string]*NanaNode{},
	}

	return root, nil
}
