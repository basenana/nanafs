package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"math"
)

const (
	defaultFsMaxSize = 8796093022208
)

type FsController interface {
	FsInfo(ctx context.Context) Info
}

type Info struct {
	Objects     uint64
	FileCount   uint64
	AvailInodes uint64
	MaxSize     uint64
	UsageSize   uint64
}

func (c *controller) FsInfo(ctx context.Context) Info {
	info := Info{
		AvailInodes: math.MaxUint32,
		MaxSize:     defaultFsMaxSize,
	}

	objects, err := c.meta.ListObjects(ctx, storage.Filter{})
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
