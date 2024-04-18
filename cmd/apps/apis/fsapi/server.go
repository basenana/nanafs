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
	v1 "github.com/basenana/nanafs/cmd/apps/apis/fsapi/v1"
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/utils/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net"
)

type Server struct {
	server   *grpc.Server
	listener net.Listener
	services v1.Services
	cfg      config.FsApi
}

func (s *Server) Run(stopCh chan struct{}) {
	if !s.cfg.Enable {
		return
	}

	go func() {
		<-stopCh
		s.server.GracefulStop()
	}()

	if err := s.server.Serve(s.listener); err != nil {
		logger.NewLogger("fsapi").Fatalf("start server failed: %s", err)
	}
}

func New(ctrl controller.Controller, pathEntryMgr *pathmgr.PathManager, cfg config.FsApi) (*Server, error) {
	if !cfg.Enable {
		return nil, fmt.Errorf("fsapi not enabled")
	}

	//certPool := x509.NewCertPool()
	//ca, err := os.ReadFile(cfg.CaFile)
	//if err != nil {
	//	return nil, fmt.Errorf("open ca file error: %s", err)
	//}
	//if ok := certPool.AppendCertsFromPEM(ca); !ok {
	//	log.Fatal("failed to append ca certs")
	//}
	//
	//certificate, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	//if err != nil {
	//	return nil, fmt.Errorf("open cert/key file error: %s", err)
	//}
	//creds := credentials.NewTLS(&tls.Config{
	//	Certificates: []tls.Certificate{certificate},
	//	ServerName:   cfg.ServerName, // NOTE: this is required!
	//	RootCAs:      certPool,
	//})
	//
	var opts = []grpc.ServerOption{
		//grpc.Creds(creds),
		grpc.Creds(insecure.NewCredentials()),
	}
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("")
	}
	s := &Server{
		listener: l,
		server:   grpc.NewServer(opts...),
		cfg:      cfg,
	}
	s.services, err = v1.InitServices(s.server, ctrl, pathEntryMgr)
	if err != nil {
		return nil, err
	}
	return s, nil
}
