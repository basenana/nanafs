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
	"os"
	"testing"
	"time"

	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/indexer"
	"github.com/basenana/nanafs/pkg/types"
	pluginapi "github.com/basenana/plugin/api"
	plugintypes "github.com/basenana/plugin/types"
	"go.uber.org/zap"

	testcfg "github.com/onsi/ginkgo/config"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	stopCh    = make(chan struct{})
	tempDir   string
	fsCore    core.Core
	testMeta  metastore.Meta
	mgr       Workflow
	namespace = types.DefaultNamespace

	bootCfg = config.Bootstrap{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
	}
)

func TestWorkflow(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	testcfg.DefaultReporterConfig.SlowSpecThreshold = time.Minute.Seconds()
	RunSpecs(t, "Workflow Suite")
}

var _ = BeforeSuite(func() {
	var err error
	tempDir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-wf-")
	Expect(err).Should(BeNil())
	bootCfg.CacheDir = tempDir
	bootCfg.CacheSize = 0
	bootCfg.Workflow.JobWorkdir = tempDir

	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	testMeta = memMeta

	fsCore, err = core.New(memMeta, bootCfg)
	Expect(err).Should(BeNil())

	err = fsCore.CreateNamespace(context.Background(), namespace)
	Expect(err).Should(BeNil())

	mgr, err = New(fsCore, notify.NewNotify(memMeta), memMeta, indexer.NewMem(), bootCfg.Workflow)
	Expect(err).Should(BeNil())

	mgr.(*manager).plugin.Register(DelayProcessPluginSpec, NewDelayProcessPlugin)

	ctx, canF := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		canF()
	}()

	mgr.Start(ctx)
})

var _ = AfterSuite(func() {
	close(stopCh)
})

const (
	delayPluginName    = "delay"
	delayPluginVersion = "1.0"
)

var DelayProcessPluginSpec = plugintypes.PluginSpec{
	Name:    delayPluginName,
	Version: delayPluginVersion,
	Type:    plugintypes.TypeProcess,
}

type DelayProcessPlugin struct {
	logger *zap.SugaredLogger
}

func NewDelayProcessPlugin(ps plugintypes.PluginCall) plugintypes.Plugin {
	return &DelayProcessPlugin{
		logger: logger.NewLogger(delayPluginName),
	}
}

func (d *DelayProcessPlugin) Name() string {
	return delayPluginName
}

func (d *DelayProcessPlugin) Type() plugintypes.PluginType {
	return plugintypes.TypeProcess
}

func (d *DelayProcessPlugin) Version() string {
	return delayPluginVersion
}

func (d *DelayProcessPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	var (
		until            time.Time
		delayDurationStr = pluginapi.GetStringParameter("delay", request, "")
		untilStr         = pluginapi.GetStringParameter("until", request, "")
	)

	switch {

	case delayDurationStr != "":
		duration, err := time.ParseDuration(delayDurationStr)
		if err != nil {
			d.logger.Warnw("parse delay duration failed", "duration", delayDurationStr, "error", err)
			return nil, fmt.Errorf("parse delay duration [%s] failed: %s", delayDurationStr, err)
		}
		until = time.Now().Add(duration)

	case untilStr != "":
		var err error
		until, err = time.Parse(time.RFC3339, untilStr)
		if err != nil {
			d.logger.Warnw("parse delay until failed", "until", untilStr, "error", err)
			return nil, fmt.Errorf("parse delay until [%s] failed: %s", untilStr, err)
		}

	default:
		return pluginapi.NewFailedResponse(fmt.Sprintf("unknown action")), nil
	}

	d.logger.Infow("delay started", "until", until)

	now := time.Now()
	if now.Before(until) {
		timer := time.NewTimer(until.Sub(now))
		defer timer.Stop()
		select {
		case <-timer.C:
			d.logger.Infow("delay completed")
			return pluginapi.NewResponse(), nil
		case <-ctx.Done():
			return pluginapi.NewFailedResponse(ctx.Err().Error()), nil
		}
	}

	return pluginapi.NewResponse(), nil
}
