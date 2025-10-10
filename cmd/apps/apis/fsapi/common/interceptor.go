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
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/token"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	authInfoContextKey = "auth.info"
)

func WithCommonInterceptors(tokenMgr *token.Manager, config config.FsApi) grpc.ServerOption {
	ci := commonInterceptors{token: tokenMgr, config: config}
	return grpc.ChainUnaryInterceptor(
		ci.serverLogInterceptor,
		ci.serverAuthInterceptor,
	)
}

type commonInterceptors struct {
	token  *token.Manager
	config config.FsApi
}

func (c *commonInterceptors) serverLogInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	start := time.Now()
	log := logger.NewLogger("fsapi").With(zap.String("method", info.FullMethod))
	resp, err = handler(ctx, req)
	log.Infow("invoked grpc", "duration", time.Since(start).String(), "err", err)
	return
}

func (c *commonInterceptors) serverAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "failed to parse metadata from incoming context")
	}

	if c.config.Noauth {
		ns := getNamespaceFromMetadata(md)
		if ns != "" {
			valueCtx := context.WithValue(ctx, authInfoContextKey, &token.AuthInfo{UID: 0, GID: 0, Namespace: ns})
			return handler(valueCtx, req)
		}
	}

	accessToken, err := getAccessTokenFromMetadata(md)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to parse token from incoming metadata")
	}
	ai, err := c.token.AccessToken(ctx, accessToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to authenticate token")
	}

	valueCtx := context.WithValue(ctx, authInfoContextKey, ai)
	return handler(valueCtx, req)
}

func WithStreamInterceptors(tokenMgr *token.Manager, config config.FsApi) grpc.ServerOption {
	si := streamInterceptors{token: tokenMgr, config: config}
	return grpc.ChainStreamInterceptor(
		si.serverStreamAuthInterceptor,
		si.serverStreamLogInterceptor,
	)
}

type streamInterceptors struct {
	token  *token.Manager
	config config.FsApi
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func (s *streamInterceptors) serverStreamAuthInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	ctx := ss.Context()
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "failed to parse metadata from incoming context")
	}

	if s.config.Noauth {
		ns := getNamespaceFromMetadata(md)
		if ns != "" {
			valueCtx := context.WithValue(ctx, authInfoContextKey, &token.AuthInfo{UID: 0, GID: 0, Namespace: ns})
			wrapped := &wrappedServerStream{
				ServerStream: ss,
				ctx:          valueCtx,
			}
			return handler(srv, wrapped)
		}
	}

	accessToken, err := getAccessTokenFromMetadata(md)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "failed to parse token from incoming metadata")
	}
	ai, err := s.token.AccessToken(ctx, accessToken)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "failed to authenticate token")
	}

	valueCtx := context.WithValue(ctx, authInfoContextKey, ai)
	wrapped := &wrappedServerStream{
		ServerStream: ss,
		ctx:          valueCtx,
	}
	return handler(valueCtx, wrapped)
}

func (s *streamInterceptors) serverStreamLogInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	start := time.Now()
	log := logger.NewLogger("fsapi").With(zap.String("method", info.FullMethod))
	err = handler(srv, ss)
	log.Debugw("stream", "duration", time.Since(start).String(), "err", err)
	return
}
