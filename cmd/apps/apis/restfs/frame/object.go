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

package frame

import (
	"context"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"sort"
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

	for i, ent := range entries {
		parent = obj
		obj, err = ctrl.FindObject(ctx, obj, ent)
		if err != nil {
			if err == types.ErrNotFound {
				if i == len(entries)-1 && action == ActionCreate {
					// ignore NotFoundError for create object
					return parent, nil, nil
				}
			}
			return
		}
	}
	return
}

type ObjectList struct {
	Objects []*types.Object
	Total   int
}

func BuildObjectList(objList []*types.Object) ObjectList {
	sort.Slice(objList, func(i, j int) bool {
		return objList[i].Name < objList[j].Name
	})
	return ObjectList{Objects: objList, Total: len(objList)}
}

func pathEntries(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}
