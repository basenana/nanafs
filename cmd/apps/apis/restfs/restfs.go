package restfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/common"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
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

func (s *RestFS) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.logger.Debugw("", "path", request.URL.Path, "method", request.Method)

	var response FsResponse
	ctx := request.Context()
	action := Action{}
	if err := json.NewDecoder(request.Body).Decode(&action); err != nil {
		s.logger.Errorw("decode request action error", "path", request.URL.Path, "err", err.Error())
	}
	FillDefaultAction(request.Method, action)

	obj, err := s.object(ctx, request.URL.Path)
	if err != nil {
		if err == types.ErrNotFound {
			response = NewErrorResponse(http.StatusMethodNotAllowed, common.ApiNotFoundError, err)
			s.response(ctx, writer, response)
			return
		}
		response = NewErrorResponse(http.StatusInternalServerError, common.ApiInternalError, err)
		s.response(ctx, writer, response)
		return
	}

	switch request.Method {
	case http.MethodGet:
		response = s.get(ctx, obj, action)
	case http.MethodPost:
		response = s.post(ctx, obj, action)
	case http.MethodPut:
		response = s.put(ctx, obj, action)
	case http.MethodDelete:
		response = s.delete(ctx, obj, action)
	default:
		response = NewErrorResponse(http.StatusMethodNotAllowed, common.ApiMethodError, fmt.Errorf("method %s not allowed", request.Method))
	}
	s.response(ctx, writer, response)
}

func (s *RestFS) get(ctx context.Context, obj *types.Object, action Action) (result FsResponse) {
	switch action.Action {
	case ActionAlias:
		result = NewFsResponse(map[string]string{
			"id": obj.ID,
		})
		return
	case ActionInspect:
		result = NewFsResponse(obj)
		return
	case ActionSearch:
		result = NewErrorResponse(http.StatusBadRequest, common.ApiArgsError, errors.New("search not support"))
		return
	case ActionDownload:
		result = NewErrorResponse(http.StatusBadRequest, common.ApiArgsError, errors.New("download not support"))
		return
	case ActionRead:
		if obj.IsGroup() {
			children, err := s.ctrl.ListObjectChildren(ctx, obj)
			if err != nil {
				result = NewErrorResponse(http.StatusInternalServerError, common.ApiInternalError, err)
				return
			}
			result = NewFsResponse(children)
			return
		}

		// TODO: return file raw data
	}
	return
}

func (s *RestFS) post(ctx context.Context, obj *types.Object, action Action) (result FsResponse) {
	switch action.Action {
	case ActionCreate:
	case ActionBulk:
	case ActionCopy:
	}
	return
}

func (s *RestFS) put(ctx context.Context, obj *types.Object, action Action) (result FsResponse) {
	switch action.Action {
	case ActionUpdate:
	case ActionMove:
	case ActionRename:
	}
	return
}

func (s *RestFS) delete(ctx context.Context, obj *types.Object, action Action) (result FsResponse) {
	switch action.Action {
	case ActionDestroy:
		if err := s.ctrl.DestroyObject(ctx, obj); err != nil {
			result = NewErrorResponse(http.StatusInternalServerError, common.ApiInternalError, err)
			return
		}
		result = NewFsResponse(obj)
	}
	return
}

func (s *RestFS) object(ctx context.Context, path string) (obj *types.Object, err error) {
	entries := pathEntries(path)
	obj, err = s.ctrl.LoadRootObject(ctx)
	if err != nil {
		return nil, err
	}
	for _, ent := range entries {
		obj, err = s.ctrl.FindObject(ctx, obj, ent)
		if err != nil {
			return nil, err
		}
	}
	return
}

func (s *RestFS) response(ctx context.Context, writer http.ResponseWriter, response FsResponse) {
	writer.WriteHeader(response.Status)
	if _, err := writer.Write(response.Json()); err != nil {
		s.logger.Errorw("write response back failed", "err", err.Error())
	}
}

func InitRestFs(cfg config.Api, route map[string]http.HandlerFunc) error {
	if !cfg.Enable {
		return nil
	}
	s := &RestFS{
		cfg:    cfg,
		logger: logger.NewLogger("HttpServer"),
	}
	route["/fs/"] = s.ServeHTTP
	return nil
}
