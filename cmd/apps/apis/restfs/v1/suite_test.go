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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/gin-gonic/gin"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	testRestFsAddr = "127.0.0.1:8001"
)

var (
	server *restFsServer
	ctrl   controller.Controller
	cfg    config.Config
)

type mockConfig struct{}

func (m mockConfig) GetConfig() (config.Config, error) {
	cfg := config.Config{ApiConfig: config.Api{Enable: true}}
	_ = config.Verify(&cfg)
	return cfg, nil
}

var _ config.Loader = mockConfig{}

func NewControllerForTest() controller.Controller {
	m, _ := storage.NewMetaStorage("memory", config.Meta{})
	s, _ := storage.NewStorage("memory", config.Storage{})

	files.InitFileIoChain(config.Config{}, s, make(chan struct{}))
	return controller.New(mockConfig{}, m, s)
}

type restFsServer struct {
	engine     *gin.Engine
	httpServer *http.Server
}

func (s *restFsServer) Run() error {
	s.httpServer = &http.Server{
		Addr:         testRestFsAddr,
		Handler:      s.engine,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	}

	go func() {
		_ = s.httpServer.ListenAndServe()
	}()
	return nil
}

func (s *restFsServer) Shutdown() error {
	ctx, canF := context.WithTimeout(context.Background(), time.Second)
	defer canF()
	return s.httpServer.Shutdown(ctx)
}

func newRestFsServer() *restFsServer {
	engine := gin.New()
	v1Handler := NewRestFs(ctrl, cfg)
	engine.Any("/v1/fs/*path", v1Handler.HttpHandle)
	return &restFsServer{engine: engine}
}

func defaultAccessForTest() types.Access {
	return types.Access{
		Permissions: defaultAccess(),
		UID:         cfg.Owner.Uid,
		GID:         cfg.Owner.Gid,
	}
}

var _ = BeforeSuite(func() {
	ctrl = NewControllerForTest()
	server = newRestFsServer()
	Expect(server.Run()).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(server.Shutdown()).NotTo(HaveOccurred())
})

func TestFile(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()

	cfg = config.Config{ApiConfig: config.Api{Enable: true}}
	_ = config.Verify(&cfg)

	RegisterFailHandler(Fail)
	RunSpecs(t, "RestFS Suite")
}
