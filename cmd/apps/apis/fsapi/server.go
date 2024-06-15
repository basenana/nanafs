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
	"context"
	"crypto/tls"
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	v1 "github.com/basenana/nanafs/cmd/apps/apis/fsapi/v1"
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/utils/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"

	_ "google.golang.org/grpc/encoding/gzip"
)

type Server struct {
	server   *grpc.Server
	listener net.Listener
	services v1.Services
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

func New(ctrl controller.Controller, pathEntryMgr *pathmgr.PathManager, cfg config.Loader) (*Server, error) {
	rootCaPool, err := common.ReadRootCAs(cfg)
	if err != nil {
		return nil, fmt.Errorf("load root ca error: %w", err)
	}
	clientCaPool, err := common.ReadClientCAs(cfg)
	if err != nil {
		return nil, fmt.Errorf("load client ca error: %w", err)
	}
	certificate, err := common.EnsureServerX509KeyPair(cfg)
	if err != nil {
		return nil, fmt.Errorf("load cert/key file error: %w", err)
	}

	serviceName, err := common.ServiceName(cfg)
	if err != nil {
		return nil, fmt.Errorf("load service name error: %w", err)
	}

	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{*certificate},
		ServerName:   serviceName,
		RootCAs:      rootCaPool,
		ClientCAs:    clientCaPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
	})

	serverHost, err := cfg.GetSystemConfig(context.TODO(), config.FsAPIConfigGroup, "host").String()
	if err != nil {
		return nil, fmt.Errorf("load server host error: %w", err)
	}
	serverPort, err := cfg.GetSystemConfig(context.TODO(), config.FsAPIConfigGroup, "port").Int()
	if err != nil {
		return nil, fmt.Errorf("load server port error: %w", err)
	}

	var opts = []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.MaxRecvMsgSize(1024 * 1024 * 50), // 50M
		common.WithCommonInterceptors(),
		common.WithStreamInterceptors(),
	}
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", serverHost, serverPort))
	if err != nil {
		return nil, fmt.Errorf("")
	}
	s := &Server{
		listener: l,
		server:   grpc.NewServer(opts...),
	}
	s.services, err = v1.InitServices(s.server, ctrl, pathEntryMgr)
	if err != nil {
		return nil, err
	}
	return s, nil
}
