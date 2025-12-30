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

package fsapi

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"

	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	v1 "github.com/basenana/nanafs/cmd/apps/apis/fsapi/v1"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils/logger"
)

type Server struct {
	server   *grpc.Server
	listener net.Listener
	services v1.ServicesV1
}

func (s *Server) Run(stopCh chan struct{}) {
	log := logger.NewLogger("fsAPI")
	go func() {
		<-stopCh
		s.server.GracefulStop()
		log.Infow("shutdown")
	}()

	log.Infof("listen on %s", s.listener.Addr().String())
	if err := s.server.Serve(s.listener); err != nil {
		logger.NewLogger("fsapi").Fatalf("start server failed: %s", err)
	}
}

func New(depends *common.Depends, cfg config.Config) (*Server, error) {
	apiCfg := cfg.GetBootstrapConfig().API
	if !apiCfg.Enable {
		return nil, fmt.Errorf("no api enabled")
	}

	var opts = []grpc.ServerOption{
		grpc.MaxRecvMsgSize(1024 * 1024 * 50), // 50M
		common.WithCommonInterceptors(),
		common.WithStreamInterceptors(),
	}
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", apiCfg.Host, apiCfg.Port))
	if err != nil {
		return nil, fmt.Errorf("")
	}
	s := &Server{
		listener: l,
		server:   grpc.NewServer(opts...),
	}
	s.services, err = v1.InitServicesV1(s.server, depends)
	if err != nil {
		return nil, err
	}
	return s, nil
}
