package frame

import (
	"context"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"strings"
)

func FindObject(ctx context.Context, ctrl controller.Controller, path, action string) (parent, obj *types.Object, err error) {
	defer utils.TraceRegion(ctx, "restfs.findobject")()
	entries := pathEntries(path)
	obj, err = ctrl.LoadRootObject(ctx)
	if err != nil {
		return
	}

	if len(entries) == 1 && entries[0] == "" {
		return obj, obj, nil
	}

	for _, ent := range entries {
		parent = obj
		obj, err = ctrl.FindObject(ctx, obj, ent)
		if err != nil {
			return
		}
	}
	return
}

func pathEntries(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}
