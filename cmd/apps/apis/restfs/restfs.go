package restfs

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

import (
	"github.com/basenana/nanafs/config"
	"go.uber.org/zap"
)

/*
	RestFS is an implement of Brinkbit HTTP Filesystem API
	Brinkbit HTTP Filesystem API is a well-thought-out design, thanks a lot for their works
	the details can be found in https://github.com/Brinkbit/http-fs-api
*/
type RestFS struct {
	cfg    config.Api
	ctrl   controller.Controller
	logger *zap.SugaredLogger
}

func (s *RestFS) Get(gCtx *gin.Context) {
	req := FsRequest{}
	if err := gCtx.ShouldBindJSON(&req); err != nil && err != io.EOF {
		gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, err))
		return
	}

	ctx := gCtx.Request.Context()
	action := fillDefaultAction(http.MethodGet, req.Data)
	_, obj, err := s.object(gCtx)
	if err != nil {
		if err == types.ErrNotFound {
			gCtx.JSON(http.StatusNotFound, NewErrorResponse(common.ApiNotFoundError, err))
			return
		}
		gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
		return
	}

	switch action.Action {
	case ActionAlias:
		gCtx.JSON(http.StatusOK, NewFsResponse(map[string]string{"id": obj.ID}))
		return
	case ActionInspect:
		gCtx.JSON(http.StatusOK, NewFsResponse(obj))
		return
	case ActionSearch:
		gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, errors.New("search not support")))
		return
	case ActionDownload:
		gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, errors.New("search not support")))
		return
	case ActionRead:
		if obj.IsGroup() {
			children, err := s.ctrl.ListObjectChildren(ctx, obj)
			if err != nil {
				gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
				return
			}
			gCtx.JSON(http.StatusOK, NewFsResponse(children))
			return
		}

		f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Read: true})
		if err != nil {
			gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
			return
		}
		defer s.ctrl.CloseFile(ctx, f)
		gCtx.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", obj.Name))
		http.ServeContent(gCtx.Writer, gCtx.Request, obj.Name, obj.ModifiedAt, &file{f: f})
	}
}

func (s *RestFS) Post(gCtx *gin.Context) {
	req := FsRequest{}
	if err := gCtx.BindJSON(&req); err != nil {
		gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, err))
		return
	}
	action := fillDefaultAction(http.MethodGet, req.Data)
	parent, obj, err := s.object(gCtx)
	if err != nil && err != types.ErrNotFound {
		gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
		return
	}
	switch action.Action {
	case ActionCreate:
		if err != types.ErrNotFound {
			gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiEntryExisted, err))
			return
		}
		s.newFile(gCtx, parent, action.Parameters.Name, bytes.NewReader(action.Parameters.Content))
		return
	case ActionBulk:
		if !obj.IsGroup() {
			gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiNotGroupError, fmt.Errorf("%s not a group", gCtx.Param("path"))))
			return
		}

		parts, err := gCtx.MultipartForm()
		if err != nil {
			gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, err))
			return
		}

		for fName := range parts.File {
			fH, err := gCtx.FormFile(fName)
			if err != nil {
				gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, err))
				return
			}

			newF, err := fH.Open()
			if err != nil {
				gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, err))
				return
			}
			s.newFile(gCtx, obj, fName, newF)
			_ = newF.Close()
		}
	case ActionCopy:
		gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, errors.New("search not support")))
		return
	}
}

func (s *RestFS) newFile(gCtx *gin.Context, parent *types.Object, name string, reader io.Reader) {
	ctx := gCtx.Request.Context()
	obj, err := s.ctrl.CreateObject(ctx, parent, types.ObjectAttr{
		Name:        name,
		Kind:        types.RawKind,
		Permissions: defaultAccess(),
	})
	if err != nil {
		gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
		return
	}
	f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Write: true})
	if err != nil {
		gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
		return
	}
	defer s.ctrl.CloseFile(ctx, f)

	buf := make([]byte, 1024)

	var total int64
	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
			return
		}

		_, err = s.ctrl.WriteFile(ctx, f, buf[:n], total)
		if err != nil && err != io.EOF {
			gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
			return
		}
		total += int64(n)

		if n == 0 || err == io.EOF {
			break
		}
	}

	if err != nil {
		gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
		return
	}
	gCtx.JSON(http.StatusOK, NewFsResponse(obj))
}

func (s *RestFS) Put(gCtx *gin.Context) {
	req := FsRequest{}
	if err := gCtx.BindJSON(&req); err != nil {
		gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, err))
		return
	}
	ctx := gCtx.Request.Context()
	action := fillDefaultAction(http.MethodGet, req.Data)
	parent, obj, err := s.object(gCtx)
	if err != nil {
		if err == types.ErrNotFound {
			gCtx.JSON(http.StatusNotFound, NewErrorResponse(common.ApiNotFoundError, err))
			return
		}
		gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
		return
	}

	switch action.Action {
	case ActionUpdate:
		f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Write: true, Create: true, Trunc: true})
		if err != nil {
			gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
			return
		}
		defer s.ctrl.CloseFile(ctx, f)
		_, err = s.ctrl.WriteFile(ctx, f, action.Parameters.Content, 0)
		if err != nil {
			gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
			return
		}
		gCtx.JSON(http.StatusOK, NewFsResponse(obj))
		return
	case ActionMove:
		gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiArgsError, errors.New("search not support")))
		return
	case ActionRename:
		oldObj, err := s.ctrl.FindObject(ctx, parent, action.Parameters.Name)
		if err != nil {
			if err == types.ErrNotFound {
				obj.Name = action.Parameters.Name
				if err = s.ctrl.SaveObject(ctx, oldObj); err != nil {
					gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
					return
				}
				gCtx.JSON(http.StatusOK, NewFsResponse(obj))
				return
			}
			gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
			return
		}
		if oldObj != nil {
			gCtx.JSON(http.StatusBadRequest, NewErrorResponse(common.ApiEntryExisted, fmt.Errorf("entry existed")))
			return
		}
	}
}

func (s *RestFS) Delete(gCtx *gin.Context) {
	ctx := gCtx.Request.Context()
	_, obj, err := s.object(gCtx)
	if err != nil {
		if err == types.ErrNotFound {
			gCtx.JSON(http.StatusNotFound, NewErrorResponse(common.ApiNotFoundError, err))
			return
		}
		gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
		return
	}

	if err := s.ctrl.DestroyObject(ctx, obj); err != nil {
		gCtx.JSON(http.StatusInternalServerError, NewErrorResponse(common.ApiInternalError, err))
		return
	}
	gCtx.JSON(http.StatusOK, NewFsResponse(obj))
}

func (s *RestFS) object(gCtx *gin.Context) (parent, obj *types.Object, err error) {
	ctx := gCtx.Request.Context()
	path := gCtx.Param("path")
	entries := pathEntries(path)
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
			return nil, nil, err
		}
	}
	return
}
