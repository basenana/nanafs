package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
	"github.com/basenana/nanafs/cmd/apps/apis/restfs"
	common2 "github.com/basenana/nanafs/cmd/apps/apis/restfs/frame"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"path"
)

import (
	"github.com/basenana/nanafs/config"
	"go.uber.org/zap"
)

*
RestFS is an implement of Brinkbit HTTP Filesystem API
Brinkbit HTTP Filesystem API is a well-thought-out design, thanks a lot for their works
*/
type RestFS struct {
	cfg    config.Config
	ctrl   controller.Controller
	logger *zap.SugaredLogger
}

func (s *RestFS) Get(gCtx *gin.Context) {
	ctx := gCtx.Request.Context()
	defer utils.TraceRegion(ctx, "restfs.get")()
	req := common2.FsRequest{}
	if err := gCtx.ShouldBindJSON(&req); err != nil && err != io.EOF {
		gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, err))
		return
	}

	action := common2.fillDefaultAction(http.MethodGet, req.Data)
	_, obj, err := s.object(gCtx)
	if err != nil {
		if err == types.ErrNotFound {
			gCtx.JSON(http.StatusNotFound, common2.NewErrorResponse(common.ApiNotFoundError, err))
			return
		}
		gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
		return
	}

	switch action.Action {
	case common2.ActionAlias:
		gCtx.JSON(http.StatusOK, common2.NewFsResponse(map[string]string{"id": obj.ID}))
		return
	case common2.ActionInspect:
		gCtx.JSON(http.StatusOK, common2.NewFsResponse(obj))
		return
	case common2.ActionSearch:
		gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, errors.New("search not support")))
		return
	case common2.ActionDownload:
		gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, errors.New("search not support")))
		return
	case common2.ActionRead:
		if obj.IsGroup() {
			children, err := s.ctrl.ListObjectChildren(ctx, obj)
			if err != nil {
				gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
				return
			}
			gCtx.JSON(http.StatusOK, common2.NewFsResponse(children))
			return
		}

		f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Read: true})
		if err != nil {
			gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
			return
		}
		defer s.ctrl.CloseFile(ctx, f)
		gCtx.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", obj.Name))
		http.ServeContent(gCtx.Writer, gCtx.Request, obj.Name, obj.ModifiedAt, &restfs.file{f: f})
	}
}

const (
	bulkActionKey = "action"
	bulkFileKey   = "files"
)

func (s *RestFS) Post(gCtx *gin.Context) {
	var (
		req = common2.FsRequest{}
		ctx = gCtx.Request.Context()
	)
	defer utils.TraceRegion(ctx, "restfs.post")()

	mf, err := gCtx.MultipartForm()
	if err != nil {
		if err = gCtx.BindJSON(&req); err != nil {
			gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, err))
			return
		}
	} else {
		if len(mf.Value[bulkActionKey]) > 0 {
			actionData := mf.Value[bulkActionKey][0]
			if err = json.Unmarshal([]byte(actionData), &req); err != nil {
				gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, err))
				return
			}
		}
	}

	action := common2.fillDefaultAction(http.MethodPost, req.Data)
	parent, obj, err := s.object(gCtx)
	if err != nil && err != types.ErrNotFound {
		gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
		return
	}
	switch action.Action {
	case common2.ActionCreate:
		if err != types.ErrNotFound {
			gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiEntryExisted, fmt.Errorf("entry existed")))
			return
		}
		newObj, err := s.newFile(gCtx, parent, action.Parameters.Name, bytes.NewReader(action.Parameters.Content))
		if err != nil {
			gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
			return
		}
		gCtx.JSON(http.StatusOK, common2.NewFsResponse(newObj))
		return
	case common2.ActionBulk:
		if err != nil && err == types.ErrNotFound {
			newDir := path.Base(gCtx.Param("path"))
			obj, err = s.ctrl.CreateObject(gCtx.Request.Context(), parent, types.ObjectAttr{Name: newDir, Kind: types.GroupKind, Access: parent.Access})
			if err != nil {
				gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiNotGroupError, fmt.Errorf("create group failed: %s", err.Error())))
			}
		}

		if !obj.IsGroup() {
			gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiNotGroupError, fmt.Errorf("%s not a group", gCtx.Param("path"))))
			return
		}
		results := make([]*types.Object, 0)
		if mf == nil {
			gCtx.JSON(http.StatusOK, common2.NewFsResponse(results))
			return
		}

		for _, fH := range mf.File[bulkFileKey] {
			newF, err := fH.Open()
			if err != nil {
				gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, err))
				return
			}

			newObj, err := s.newFile(gCtx, obj, fH.Filename, newF)
			if err != nil {
				_ = newF.Close()
				gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
				return
			}
			_ = newF.Close()
			results = append(results, newObj)
		}
		gCtx.JSON(http.StatusOK, common2.NewFsResponse(results))
	case common2.ActionCopy:
		gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, errors.New("search not support")))
		return
	}
}

func (s *RestFS) newFile(gCtx *gin.Context, parent *types.Object, name string, reader io.Reader) (*types.Object, error) {
	ctx := gCtx.Request.Context()
	defer utils.TraceRegion(ctx, "restfs.newfile")()
	obj, err := s.ctrl.CreateObject(ctx, parent, types.ObjectAttr{
		Name:   name,
		Kind:   types.RawKind,
		Access: parent.Access,
	})
	if err != nil {
		return nil, err
	}
	f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Write: true})
	if err != nil {
		return nil, err
	}
	defer s.ctrl.CloseFile(ctx, f)

	buf := make([]byte, 1024)

	var total int64
	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}

		_, err = s.ctrl.WriteFile(ctx, f, buf[:n], total)
		if err != nil && err != io.EOF {
			return nil, err
		}
		total += int64(n)

		if n == 0 || err == io.EOF {
			break
		}
	}

	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (s *RestFS) Put(gCtx *gin.Context) {
	ctx := gCtx.Request.Context()
	defer utils.TraceRegion(ctx, "restfs.put")()
	req := common2.FsRequest{}
	if err := gCtx.BindJSON(&req); err != nil {
		gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, err))
		return
	}
	action := common2fillDefaultAction(http.MethodPut, req.Data)
	parent, obj, err := s.object(gCtx)
	if err != nil {
		if err == types.ErrNotFound {
			gCtx.JSON(http.StatusNotFound, common2.NewErrorResponse(common.ApiNotFoundError, err))
			return
		}
		gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
		return
	}

	switch action.Action {
	case common2.ActionUpdate:
		f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Write: true, Create: true, Trunc: true})
		if err != nil {
			gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
			return
		}
		defer s.ctrl.CloseFile(ctx, f)
		_, err = s.ctrl.WriteFile(ctx, f, action.Parameters.Content, 0)
		if err != nil {
			gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
			return
		}
		gCtx.JSON(http.StatusOK, common2.NewFsResponse(obj))
		return
	case common2.ActionMove:
		gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiArgsError, errors.New("search not support")))
		return
	case common2.ActionRename:
		oldObj, err := s.ctrl.FindObject(ctx, parent, action.Parameters.Name)
		if err != nil {
			if err == types.ErrNotFound {
				obj.Name = action.Parameters.Name
				if err = s.ctrl.SaveObject(ctx, obj); err != nil {
					gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
					return
				}
				gCtx.JSON(http.StatusOK, common2.NewFsResponse(obj))
				return
			}
			gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
			return
		}
		if oldObj != nil {
			gCtx.JSON(http.StatusBadRequest, common2.NewErrorResponse(common.ApiEntryExisted, fmt.Errorf("entry existed")))
			return
		}
	}
}

func (s *RestFS) Delete(gCtx *gin.Context) {
	ctx := gCtx.Request.Context()
	defer utils.TraceRegion(ctx, "restfs.delete")()
	_, obj, err := s.object(gCtx)
	if err != nil {
		if err == types.ErrNotFound {
			gCtx.JSON(http.StatusNotFound, common2.NewErrorResponse(common.ApiNotFoundError, err))
			return
		}
		gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
		return
	}

	if err := s.ctrl.DestroyObject(ctx, obj); err != nil {
		gCtx.JSON(http.StatusInternalServerError, common2.NewErrorResponse(common.ApiInternalError, err))
		return
	}
	gCtx.JSON(http.StatusOK, common2.NewFsResponse(obj))
}

func (s *RestFS) object(gCtx *gin.Context) (parent, obj *types.Object, err error) {
	ctx := gCtx.Request.Context()
	defer utils.TraceRegion(ctx, "restfs.findobject")()
	pathStr := gCtx.Param("path")
	entries := pathEntries(pathStr)
	obj, err = s.ctrl.LoadRootObject(ctx)
	if err != nil {
		return nil, nil, err
	}

	if len(entries) == 1 && entries[0] == "" {
		return obj, obj, nil
	}

	for _, ent := range entries {
		parent = obj
		obj, err = s.ctrl.FindObject(ctx, obj, ent)
		if err != nil {
			return
		}
	}
	return
}

func NewRestFs(ctrl controller.Controller, cfg config.Config) *RestFS {
	return &RestFS{
		cfg:    cfg,
		ctrl:   ctrl,
		logger: logger.NewLogger("restFs.v1"),
	}
}
