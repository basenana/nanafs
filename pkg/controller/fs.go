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

package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/dispatch"
	"github.com/basenana/nanafs/pkg/plugin"
	"math"
	"runtime/trace"
	"sync"
	"time"
)

const (
	defaultFsMaxSize = 1125899906842624
)

type Info struct {
	Objects     uint64
	FileCount   uint64
	AvailInodes uint64
	MaxSize     uint64
	UsageSize   uint64
}

var (
	fsInfoCache       *Info
	fsInfoNextFetchAt time.Time
)

func (c *controller) FsInfo(ctx context.Context) Info {
	defer trace.StartRegion(ctx, "controller.FsInfo").End()

	nowTime := time.Now()
	if fsInfoCache != nil && nowTime.Before(fsInfoNextFetchAt) {
		return *fsInfoCache
	}

	info := Info{
		AvailInodes: math.MaxUint32,
		MaxSize:     defaultFsMaxSize,
	}

	sysInfo, err := c.meta.SystemInfo(ctx)
	if err != nil {
		return info
	}

	info.Objects = uint64(sysInfo.ObjectCount)
	info.UsageSize = uint64(sysInfo.FileSizeTotal)

	fsInfoCache = &info
	fsInfoNextFetchAt.Add(time.Minute * 5)
	return info
}

func (c *controller) StartBackendTask(stopCh chan struct{}) {
	st, err := dispatch.Init(c.entry, c.notify, c.meta)
	if err != nil {
		c.logger.Panicf("start backend task failed: %s", err)
	}
	go st.Run(stopCh)
	go c.workflow.StartCron(stopCh)
}

func (c *controller) SetupShutdownHandler(stopCh chan struct{}) chan struct{} {
	shutdownSafe := make(chan struct{})
	go func() {
		<-stopCh
		wg := sync.WaitGroup{}
		wg.Add(2)

		go func() {
			defer wg.Done()
			c.logger.Warn("waiting all entry closed")
			c.entry.MustCloseAll()
		}()
		go func() {
			defer wg.Done()
			c.logger.Warn("waiting all plugin shutdown")
			plugin.MustShutdown()
		}()

		wg.Wait()
		close(shutdownSafe)
	}()
	return shutdownSafe
}
