package plugin

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"sync"
	"time"
)

const (
	pluginExecutionInterval = time.Minute * 15
)

type run struct {
	id     string
	p      types.Plugin
	obj    *types.Object
	params map[string]string
	stopCh chan struct{}
}

type runtime struct {
	meta         storage.MetaStore
	pluginRuns   map[string]*run
	globalStopCh chan struct{}
	mux          sync.Mutex
	logger       *zap.SugaredLogger
}

func (r *runtime) run() {
	ticker := time.NewTicker(pluginExecutionInterval)

	for {
		select {
		case <-ticker.C:
			if !pluginRegistry.init {
				continue
			}
			r.scanObjects()
		case <-r.globalStopCh:
			r.logger.Infow("closed")
			return
		}
	}
}

func (r *runtime) scanObjects() {
	objectList, err := r.meta.ListObjects(context.Background(), types.Filter{Kind: types.SmartGroupKind})
	if err != nil {
		r.logger.Errorw("list smt groups failed", "err", err.Error())
		return
	}

	for i := range objectList {
		obj := objectList[i]
		if obj.ExtendData.PlugScope == nil {
			continue
		}

		plu, err := LoadPlugin(obj.ExtendData.PlugScope.PluginName)
		if err != nil {
			r.logger.Errorw("load group plugin failed", "oid", obj.ID, "plugin", obj.ExtendData.PlugScope.PluginName, "err", err.Error())
			continue
		}

		r.trigger(obj, plu)
	}
}

func (r *runtime) trigger(obj *types.Object, p types.Plugin) {
	rid := fmt.Sprintf("%s-%s", obj.ID, p.Name())
	r.mux.Lock()
	defer r.mux.Unlock()

	if _, ok := r.pluginRuns[rid]; ok {
		return
	}

	pRun := run{
		id:     rid,
		obj:    obj,
		p:      p,
		params: obj.ExtendData.PlugScope.Parameters,
		stopCh: make(chan struct{}),
	}
	r.pluginRuns[pRun.id] = &pRun
	go r.runOnePlugin(&pRun)
}

func (r *runtime) runOnePlugin(pRun *run) {
	defer func() {
		r.mux.Lock()
		defer r.mux.Unlock()
		close(pRun.stopCh)
		delete(r.pluginRuns, pRun.id)
	}()

	ctx, canF := context.WithCancel(context.Background())
	go func() {
		<-pRun.stopCh
		canF()
	}()

	switch pRun.p.Type() {
	case types.PluginTypeSource:
		plug, ok := pRun.p.(types.SourcePlugin)
		if !ok {
			r.logger.Warnw("not source plugin", "runId", pRun.id)
			return
		}

		fileCh, err := plug.Run(ctx, pRun.obj, pRun.params)
		if err != nil {
			r.logger.Warnw("run plugin failed", "runId", pRun.id, "err", err.Error())
			return
		}

		for newFile := range fileCh {
			obj, err := r.meta.GetObject(ctx, pRun.obj.ID)
			if err != nil {
				if err == types.ErrNotFound {
					r.logger.Infow("object not found, close", "runId", pRun.id)
					return
				}
				r.logger.Warnw("load fresh object failed", "runId", pRun.id, "err", err.Error())
				continue
			}

			child, err := types.InitNewObject(pRun.obj, types.ObjectAttr{
				Name:   newFile.Name,
				Kind:   types.RawKind,
				Access: pRun.obj.Access,
			})
			if err != nil {
				r.logger.Warnw("build new object failed", "runId", pRun.id, "err", err.Error())
				_ = newFile.Close()
				continue
			}
			updatePlugLabels(child)
			obj.ChangedAt = time.Now()
			obj.ModifiedAt = time.Now()
			if err = r.meta.SaveObject(ctx, obj, child); err != nil {
				r.logger.Warnw("save new object failed", "runId", pRun.id, "err", err.Error())
				_ = newFile.Close()
				continue
			}
			pRun.obj = obj

			if err = copyNewFileContent(ctx, child, newFile); err != nil {
				r.logger.Warnw("save new object content failed", "runId", pRun.id, "err", err.Error())
			}
		}
	}
}

func newPluginRuntime(meta storage.MetaStore, stopCh chan struct{}) *runtime {
	return &runtime{
		meta:         meta,
		pluginRuns:   map[string]*run{},
		globalStopCh: stopCh,
		logger:       logger.NewLogger("pluginRuntime"),
	}
}

func copyNewFileContent(ctx context.Context, newObj *types.Object, newFile types.SimpleFile) error {
	defer newFile.Close()
	f, err := files.Open(ctx, newObj, files.Attr{
		Write:  true,
		Create: true,
		Trunc:  true,
	})
	if err != nil {
		return fmt.Errorf("open file failed: %s", err.Error())
	}
	defer f.Close(ctx)

	buf := make([]byte, 1024)
	var count int64
	for {
		n, err := newFile.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("read content failed: %s", err.Error())
		}
		if 0 == n {
			break
		}
		_, err = f.Write(ctx, buf[:n], count)
		if err != nil {
			return fmt.Errorf("copy content failed: %s", err.Error())
		}
		count += int64(n)
	}
	return nil
}

// TODO
func updatePlugLabels(obj *types.Object) {
}
