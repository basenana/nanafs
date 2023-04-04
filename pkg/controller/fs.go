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
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"math"
)

const (
	defaultFsMaxSize = 8796093022208
)

type Info struct {
	Objects     uint64
	FileCount   uint64
	AvailInodes uint64
	MaxSize     uint64
	UsageSize   uint64
}

func (c *controller) FsInfo(ctx context.Context) Info {
	defer utils.TraceRegion(ctx, "controller.fsinfo")()

	info := Info{
		AvailInodes: math.MaxUint32,
		MaxSize:     defaultFsMaxSize,
	}

	objects, err := c.meta.ListObjects(ctx, types.Filter{})
	if err != nil {
		return info
	}

	for _, obj := range objects {
		switch obj.Kind {
		case types.GroupKind:
		default:
			info.FileCount += 1
		}
		info.Objects += 1
		info.UsageSize += uint64(obj.Size)
	}
	return info
}
func (c *controller) SetupShutdownHandler(stopCh chan struct{}) chan struct{} {
	shutdownSafe := make(chan struct{})
	go func() {
		<-stopCh
		c.logger.Warn("waiting all entry closed")
		c.entry.MustCloseAll()
		close(shutdownSafe)
	}()
	return shutdownSafe
}
