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

	"github.com/basenana/nanafs/utils/logger"
)

func WithCommonInterceptors() grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(
		serverLogInterceptor,
		serverAuthInterceptor,
	)
}

func serverLogInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	start := time.Now()
	log := logger.NewLogger("fsapi").With(zap.String("method", info.FullMethod))
	resp, err = handler(ctx, req)
	log.Infow("invoked grpc", "duration", time.Since(start).String(), "err", err)
	return
}

func serverAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	ns := NamespaceFromMetadata(md)
	if ns == "" {
		return nil, status.Errorf(codes.Unauthenticated, "missing X-Namespace header")
	}

	caller := &CallerInfo{
		Namespace: ns,
		UID:       UIDFromMetadata(md),
		GID:       GIDFromMetadata(md),
	}
	valueCtx := WithCallerInfo(ctx, caller)
	return handler(valueCtx, req)
}

func WithStreamInterceptors() grpc.ServerOption {
	return grpc.ChainStreamInterceptor(
		serverStreamAuthInterceptor,
		serverStreamLogInterceptor,
	)
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func serverStreamAuthInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	ctx := ss.Context()
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	ns := NamespaceFromMetadata(md)
	if ns == "" {
		return status.Errorf(codes.Unauthenticated, "missing X-Namespace header")
	}

	caller := &CallerInfo{
		Namespace: ns,
		UID:       UIDFromMetadata(md),
		GID:       GIDFromMetadata(md),
	}
	valueCtx := WithCallerInfo(ctx, caller)
	wrapped := &wrappedServerStream{
		ServerStream: ss,
		ctx:          valueCtx,
	}
	return handler(srv, wrapped)
}

func serverStreamLogInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	start := time.Now()
	log := logger.NewLogger("fsapi").With(zap.String("method", info.FullMethod))
	err = handler(srv, ss)
	log.Debugw("stream", "duration", time.Since(start).String(), "err", err)
	return
}
