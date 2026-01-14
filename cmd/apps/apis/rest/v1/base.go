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

package v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/cmd/apps/apis/rest/common"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow"
)

type ServicesV1 struct {
	meta     metastore.Meta
	core     core.Core
	workflow workflow.Workflow
	notify   *notify.Notify
	cfg      config.Config
	logger   *zap.SugaredLogger
}

func NewServicesV1(engine *gin.Engine, depends *common.Depends) (*ServicesV1, error) {
	s := &ServicesV1{
		meta:     depends.Meta,
		core:     depends.Core,
		workflow: depends.Workflow,
		notify:   depends.Notify,
		cfg:      depends.Config,
		logger:   logger.NewLogger("rest"),
	}

	return s, nil
}

func (s *ServicesV1) caller(ctx *gin.Context) (*common.CallerInfo, error) {
	caller := common.Caller(ctx.Request.Context())
	if caller == nil {
		return nil, errors.New("unauthorized: missing caller info")
	}
	return caller, nil
}

// requireCaller extracts caller info from context and writes error response if missing.
func (s *ServicesV1) requireCaller(ctx *gin.Context) *common.CallerInfo {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return nil
	}
	return caller
}

// requireEntryWithPermission gets entry by uri or id query parameter and checks permissions.
func (s *ServicesV1) requireEntryWithPermission(ctx *gin.Context, caller *common.CallerInfo, perms ...types.Permission) (*types.Entry, string) {
	uri := decodeMagicURI(ctx.Query("uri"))
	idStr := ctx.Query("id")

	var en *types.Entry
	var err error

	if uri != "" {
		_, en, err = s.core.GetEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	} else if idStr != "" {
		id, parseErr := strconv.ParseInt(idStr, 10, 64)
		if parseErr != nil {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("invalid id format"))
			return nil, ""
		}
		en, err = s.core.GetEntry(ctx.Request.Context(), caller.Namespace, id)
	} else {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("missing uri or id parameter"))
		return nil, ""
	}

	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			apitool.ErrorResponse(ctx, http.StatusNotFound, "NOT_FOUND", err)
		} else {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		}
		return nil, ""
	}

	if !s.checkPermissionEntry(ctx, caller, en, perms...) {
		return nil, ""
	}

	if uri == "" {
		uri, err = core.ProbableEntryPath(ctx.Request.Context(), s.core, en)
		if err != nil {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("unable to deduce the URI of entry: %w", err))
		}
	}

	return en, uri
}

// checkPermissionEntry checks if caller has required permissions on entry.
func (s *ServicesV1) checkPermissionEntry(ctx *gin.Context, caller *common.CallerInfo, en *types.Entry, perms ...types.Permission) bool {
	err := core.HasAllPermissions(en.Access, caller.UID, caller.GID, perms...)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return false
	}
	return true
}

// listChildren lists children of a group entry, sorted by name.
func (s *ServicesV1) listChildren(ctx context.Context, namespace string, entryID int64) ([]*types.Entry, error) {
	grp, err := s.core.OpenGroup(ctx, namespace, entryID)
	if err != nil {
		return nil, err
	}
	return grp.ListChildren(ctx)
}

// listGroupChildren lists only group children of a group entry, sorted by name.
func (s *ServicesV1) listGroupChildren(ctx context.Context, namespace string, entryID int64) ([]*types.Entry, error) {
	grp, err := s.core.OpenGroup(ctx, namespace, entryID)
	if err != nil {
		return nil, err
	}
	return grp.ListGroupChildren(ctx)
}

// getEntryByPath resolves uri to parent and entry IDs.
func (s *ServicesV1) getEntryByPath(ctx context.Context, namespace, uri string) (int64, int64, error) {
	if uri == "" {
		return 0, 0, errors.New("invalid uri")
	}
	par, e, err := s.core.GetEntryByPath(ctx, namespace, uri)
	if err != nil {
		return 0, 0, errors.New("get entry failed: " + err.Error())
	}
	var pid, eid int64
	if par != nil {
		pid = par.ID
	}
	if e != nil {
		eid = e.ID
	}
	return pid, eid, nil
}

// deleteEntry deletes an entry with permission checks.
func (s *ServicesV1) deleteEntry(ctx context.Context, namespace string, uid int64, entryURI string) (*types.Entry, error) {
	parent, en, err := s.core.GetEntryByPath(ctx, namespace, entryURI)
	if err != nil {
		return nil, errors.New("query entry failed")
	}

	err = core.HasAllPermissions(en.Access, uid, 0, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, err
	}

	err = core.HasAllPermissions(parent.Access, uid, 0, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, err
	}

	var gid int64
	if err = core.IsAccess(parent.Access, uid, gid, 0x2); err != nil {
		return nil, types.ErrNoAccess
	}

	if uid != 0 && uid != en.Access.UID && uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return nil, types.ErrNoAccess
	}

	return en, s.core.RemoveEntry(ctx, namespace, entryURI, types.DeleteEntry{DeleteAll: true})
}

// getDocumentProperty retrieves document properties for an entry.
func (s *ServicesV1) getDocumentProperty(ctx context.Context, namespace string, en *types.Entry) *types.DocumentProperties {
	if en.IsGroup {
		return nil
	}
	doc := &types.DocumentProperties{}
	if err := s.meta.GetEntryProperties(ctx, namespace, types.PropertyTypeDocument, en.ID, doc); err != nil {
		return nil
	}
	return doc
}

// getEntryDetails retrieves full entry details including properties.
func (s *ServicesV1) getEntryDetails(ctx context.Context, namespace, uri, name string, id int64) (*EntryDetail, error) {
	en, err := s.core.GetEntry(ctx, namespace, id)
	if err != nil {
		return nil, err
	}

	doc := types.DocumentProperties{}
	err = s.meta.GetEntryProperties(ctx, namespace, types.PropertyTypeDocument, id, &doc)
	if err != nil {
		return nil, err
	}

	properties := &types.Properties{}
	err = s.meta.GetEntryProperties(ctx, namespace, types.PropertyTypeProperty, id, properties)
	if err != nil {
		return nil, err
	}

	return toEntryDetail(uri, name, en, doc, &Property{
		Tags:       properties.Tags,
		Properties: properties.Properties,
	}), nil
}

// pdKind2EntryKind converts string to entry kind.
func (s *ServicesV1) pdKind2EntryKind(k string) types.Kind {
	kindMap := map[types.Kind]struct{}{
		types.RawKind:           {},
		types.GroupKind:         {},
		types.SmartGroupKind:    {},
		types.FIFOKind:          {},
		types.SocketKind:        {},
		types.SymLinkKind:       {},
		types.BlkDevKind:        {},
		types.CharDevKind:       {},
		types.ExternalGroupKind: {},
	}
	_, ok := kindMap[types.Kind(k)]
	if !ok {
		return types.RawKind
	}
	return types.Kind(k)
}

// setupRssConfig configures RSS source for a group.
func (s *ServicesV1) setupRssConfig(config *RssConfig, attr *types.EntryAttr) {
	if config == nil || config.Feed == "" {
		return
	}

	fileType := config.FileType
	if fileType == "" {
		fileType = "html"
	}

	attr.GroupProperties = &types.GroupProperties{
		Source: "rss",
		RSS: &types.GroupRSS{
			Feed:     config.Feed,
			SiteName: config.SiteName,
			SiteURL:  config.SiteURL,
			FileType: fileType,
		},
	}

	if attr.Properties == nil {
		attr.Properties = &types.Properties{}
	}

	attr.Properties.SiteName = config.SiteName
	attr.Properties.URL = config.SiteURL
}

// setupGroupFilterConfig configures filter for a smart group.
func (s *ServicesV1) setupGroupFilterConfig(config *FilterConfig, attr *types.EntryAttr) {
	attr.GroupProperties = &types.GroupProperties{
		Filter: &types.Filter{
			CELPattern: config.CELPattern,
		},
	}
}

func (s *ServicesV1) Meta() metastore.Meta {
	return s.meta
}

func (s *ServicesV1) Core() core.Core {
	return s.core
}

func (s *ServicesV1) Workflow() workflow.Workflow {
	return s.workflow
}

func (s *ServicesV1) Notify() *notify.Notify {
	return s.notify
}

func (s *ServicesV1) Config() config.Config {
	return s.cfg
}

func (s *ServicesV1) Logger() *zap.SugaredLogger {
	return s.logger
}

func decodeMagicURI(uri string) string {
	// URLQueryItem does not encode + (it has special meaning in query strings).
	// We manually encode + to %2B, and let the backend handle other encodings.
	return strings.ReplaceAll(uri, "%2B", "+")
}
