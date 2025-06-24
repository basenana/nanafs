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

package common

import (
	"context"
	"github.com/basenana/nanafs/pkg/token"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"time"

	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	authInfoContextKey = "auth.info"
)

func WithCommonInterceptors(tokenMgr *token.Manager) grpc.ServerOption {
	ci := commonInterceptors{token: tokenMgr}
	return grpc.ChainUnaryInterceptor(
		ci.serverLogInterceptor,
		ci.serverAuthInterceptor,
	)
}

type commonInterceptors struct {
	token *token.Manager
}

func (c *commonInterceptors) serverLogInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	start := time.Now()
	log := logger.NewLogger("fsapi").With(zap.String("method", info.FullMethod))
	resp, err = handler(ctx, req)
	log.Infow("invoked grpc", "duration", time.Since(start).String(), "err", err)
	return
}

func (c *commonInterceptors) serverAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	var valueCtx context.Context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "failed to parse metadata from incoming context")
	}
	accessToken, err := getAccessTokenFromMetadata(md)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to parse token from incoming metadata")
	}
	ai, err := c.token.AccessToken(ctx, accessToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to authenticate token")
	}

	valueCtx = context.WithValue(ctx, authInfoContextKey, ai)
	return handler(valueCtx, req)
}

func WithStreamInterceptors(tokenMgr *token.Manager) grpc.ServerOption {
	si := streamInterceptors{token: tokenMgr}
	return grpc.ChainStreamInterceptor(
		si.serverStreamAuthInterceptor,
		si.serverStreamLogInterceptor,
	)
}

type streamInterceptors struct {
	token *token.Manager
}

func (s *streamInterceptors) serverStreamAuthInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	err = handler(srv, ss)
	return
}

func (s *streamInterceptors) serverStreamLogInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	start := time.Now()
	log := logger.NewLogger("fsapi").With(zap.String("method", info.FullMethod))
	err = handler(srv, ss)
	log.Debugw("stream", "duration", time.Since(start).String(), "err", err)
	return
}
