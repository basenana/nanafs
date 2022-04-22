package restfs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/schema"
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

	var response schema.FsResponse
	ctx := request.Context()
	action := schema.Action{}
	if err := json.NewDecoder(request.Body).Decode(&action); err != nil {
		s.logger.Errorw("decode request action error", "path", request.URL.Path, "err", err.Error())
	}
	schema.FillDefaultAction(request.Method, action)

	obj, err := s.object(ctx, request.URL.Path)
	if err != nil {
		if err == types.ErrNotFound {
			response = schema.NewErrorResponse(schema.ApiNotFoundError, err)
			response.Status = http.StatusMethodNotAllowed
			s.response(ctx, writer, response)
			return
		}
		response = schema.NewErrorResponse(schema.ApiInternalError, err)
		response.Status = http.StatusInternalServerError
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
		response = schema.NewErrorResponse(schema.ApiMethodError, fmt.Errorf("method %s not allowed", request.Method))
		response.Status = http.StatusMethodNotAllowed
	}
	s.response(ctx, writer, response)
}

func (s *RestFS) get(ctx context.Context, obj *types.Object, action schema.Action) (result schema.FsResponse) {
	return
}

func (s *RestFS) post(ctx context.Context, obj *types.Object, action schema.Action) (result schema.FsResponse) {
	return
}

func (s *RestFS) put(ctx context.Context, obj *types.Object, action schema.Action) (result schema.FsResponse) {
	return
}

func (s *RestFS) delete(ctx context.Context, obj *types.Object, action schema.Action) (result schema.FsResponse) {
	return
}

func (s *RestFS) object(ctx context.Context, path string) (obj *types.Object, err error) {
	return
}

func (s *RestFS) response(ctx context.Context, writer http.ResponseWriter, response schema.FsResponse) {
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
