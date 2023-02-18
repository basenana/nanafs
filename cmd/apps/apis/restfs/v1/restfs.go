package v1

import (
	"bytes"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
	"github.com/basenana/nanafs/cmd/apps/apis/restfs/frame"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"net/http"
	"path"
)

type handleF func(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters)

/*
RestFS is an implement of Brinkbit HTTP Filesystem API
Brinkbit HTTP Filesystem API is a well-thought-out design, thanks a lot for their works
*/
type RestFS struct {
	cfg      config.Config
	ctrl     controller.Controller
	handlers map[string]handleF
	logger   *zap.SugaredLogger
}

func (s *RestFS) register() {
	// GET
	s.handlers[frame.ActionRead] = s.read
	s.handlers[frame.ActionAlias] = s.alias
	s.handlers[frame.ActionSearch] = s.search
	s.handlers[frame.ActionInspect] = s.inspect
	s.handlers[frame.ActionDownload] = s.download

	// POST
	s.handlers[frame.ActionCreate] = s.create
	s.handlers[frame.ActionBulk] = s.bulk
	s.handlers[frame.ActionCopy] = s.copy

	// PUT
	s.handlers[frame.ActionUpdate] = s.update
	s.handlers[frame.ActionMove] = s.move
	s.handlers[frame.ActionRename] = s.rename

	// DELETE
	s.handlers[frame.ActionDestroy] = s.destroy
}

func (s *RestFS) HttpHandle(gCtx *gin.Context) {
	ctx := gCtx.Request.Context()
	defer utils.TraceRegion(ctx, fmt.Sprintf("restfs.%s", gCtx.Request.Method))()

	action := frame.BuildAction(gCtx)
	parent, obj, err := frame.FindObject(ctx, s.ctrl, gCtx.Param("path"), action.Action)
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}

	req := frame.RequestV1{}
	switch gCtx.Request.Method {
	case http.MethodPost:
		if action.Action != frame.ActionBulk {
			if err = gCtx.BindJSON(&req); err != nil && err != io.EOF {
				common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, err)
				return
			}
		}
	case http.MethodPut:
		if err = gCtx.BindJSON(&req); err != nil && err != io.EOF {
			common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, err)
			return
		}
	}

	handler, ok := s.handlers[action.Action]
	if !ok {
		common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, fmt.Errorf("action %s not support", action.Action))
	}

	handler(gCtx, obj, parent, action, req.Parameters)
}

func (s *RestFS) read(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	ctx := gCtx.Request.Context()
	if obj.IsGroup() {
		children, err := s.ctrl.ListObjectChildren(ctx, obj)
		if err != nil {
			common.ErrorResponse(gCtx, err)
			return
		}
		common.JsonResponse(gCtx, http.StatusOK, frame.BuildObjectList(children))
		return
	}

	f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Read: true})
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	defer s.ctrl.CloseFile(ctx, f)
	gCtx.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", obj.Name))
	http.ServeContent(gCtx.Writer, gCtx.Request, obj.Name, obj.ModifiedAt, &file{f: f})
}

func (s *RestFS) alias(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	common.JsonResponse(gCtx, http.StatusOK, map[string]int64{"id": obj.ID})
}

// TODO: search in fs
func (s *RestFS) search(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, fmt.Errorf("search not support yet"))
}

func (s *RestFS) inspect(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	common.JsonResponse(gCtx, http.StatusOK, obj)
}

// TODO: download single file or dir with zip
func (s *RestFS) download(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, fmt.Errorf("download not support yet"))
}

func (s *RestFS) create(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	if obj != nil {
		common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiEntryExisted, fmt.Errorf("object existed"))
		return
	}
	pathStr := gCtx.Param("path")
	if param.Name == "" {
		param.Name = path.Base(pathStr)
	}

	if param.Name == "" {
		common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, fmt.Errorf("object name [%s] invalid", param.Name))
		return
	}

	newObj, err := s.newFile(gCtx, parent, param.Name, action, bytes.NewReader(param.Content))
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	common.JsonResponse(gCtx, http.StatusOK, newObj)
}

func (s *RestFS) bulk(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	if obj == nil {
		var err error
		newDir := path.Base(gCtx.Param("path"))
		obj, err = s.ctrl.CreateObject(gCtx.Request.Context(), parent, types.ObjectAttr{Name: newDir, Kind: types.GroupKind, Access: parent.Access})
		if err != nil {
			common.ErrorResponse(gCtx, err)
			return
			return
		}
	}

	if !obj.IsGroup() {
		common.HttpStatusResponse(gCtx, http.StatusBadRequest, types.ErrIsGroup)
		return
	}

	results := make([]*types.Object, 0)
	mf, err := gCtx.MultipartForm()
	if err != nil {
		common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, err)
		return
	}
	if mf == nil {
		common.JsonResponse(gCtx, http.StatusOK, frame.BuildObjectList(results))
		return
	}

	for _, fH := range mf.File["files"] {
		newF, err := fH.Open()
		if err != nil {
			common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, err)
			return
		}

		newObj, err := s.newFile(gCtx, obj, fH.Filename, action, newF)
		if err != nil {
			_ = newF.Close()
			common.ErrorResponse(gCtx, err)
			return
		}
		_ = newF.Close()
		results = append(results, newObj)
	}
	common.JsonResponse(gCtx, http.StatusOK, frame.BuildObjectList(results))
}

func (s *RestFS) copy(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	ctx := gCtx.Request.Context()
	dstParent, _, err := frame.FindObject(ctx, s.ctrl, param.Destination, frame.ActionCreate)
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	newObjName := path.Base(param.Destination)
	if newObjName == "" {
		common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, fmt.Errorf("object name [%s] invalid", newObjName))
		return
	}

	f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Read: true})
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	defer s.ctrl.CloseFile(ctx, f)

	newObj, err := s.newFile(gCtx, dstParent, newObjName, action, &file{f: f})
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	common.JsonResponse(gCtx, http.StatusOK, newObj)
}

func (s *RestFS) update(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	ctx := gCtx.Request.Context()
	f, err := s.ctrl.OpenFile(ctx, obj, files.Attr{Write: true, Create: true, Trunc: true})
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	defer s.ctrl.CloseFile(ctx, f)
	_, err = s.ctrl.WriteFile(ctx, f, param.Content, 0)
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	common.JsonResponse(gCtx, http.StatusOK, obj)
}

func (s *RestFS) move(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	ctx := gCtx.Request.Context()

	if param.Destination == "" {
		common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, fmt.Errorf("destination is empty"))
		return
	}
	if param.Name == "" {
		param.Name = obj.Name
	}

	_, dstParent, err := frame.FindObject(ctx, s.ctrl, param.Destination, action.Action)
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}

	if !dstParent.IsGroup() {
		common.ApiErrorResponse(gCtx, http.StatusBadRequest, common.ApiArgsError, types.ErrNoGroup)
		return
	}

	err = s.ctrl.ChangeObjectParent(ctx, obj, parent, dstParent, param.Name, types.ChangeParentAttr{})
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}

	obj, err = s.ctrl.FindObject(ctx, dstParent, param.Name)
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	common.JsonResponse(gCtx, http.StatusOK, obj)
}

func (s *RestFS) rename(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	ctx := gCtx.Request.Context()
	err := s.ctrl.ChangeObjectParent(ctx, obj, parent, parent, param.Name, types.ChangeParentAttr{})
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}

	obj, err = s.ctrl.FindObject(ctx, parent, obj.Name)
	if err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	common.JsonResponse(gCtx, http.StatusOK, obj)
}

func (s *RestFS) destroy(gCtx *gin.Context, obj, parent *types.Object, action frame.Action, param frame.Parameters) {
	ctx := gCtx.Request.Context()
	if err := s.ctrl.DestroyObject(ctx, parent, obj, types.DestroyObjectAttr{}); err != nil {
		common.ErrorResponse(gCtx, err)
		return
	}
	common.JsonResponse(gCtx, http.StatusOK, obj)
}

func (s *RestFS) newFile(gCtx *gin.Context, parent *types.Object, name string, action frame.Action, reader io.Reader) (*types.Object, error) {
	ctx := gCtx.Request.Context()
	defer utils.TraceRegion(ctx, "restfs.newfile")()

	if len(name) == 0 {
		return nil, fmt.Errorf("name is empty")
	}

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

	// TODO: async task copy?
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

func NewRestFs(ctrl controller.Controller, cfg config.Config) *RestFS {
	fs := &RestFS{
		cfg:      cfg,
		ctrl:     ctrl,
		handlers: map[string]handleF{},
		logger:   logger.NewLogger("restFs.v1"),
	}
	fs.register()
	return fs
}
