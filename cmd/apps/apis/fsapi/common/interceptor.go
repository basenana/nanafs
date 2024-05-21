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
	"google.golang.org/grpc/status"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

func WithCommonInterceptors() grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(
		serverLogInterceptor,
		serverAuthInterceptor,
	)
}

func WithStreamInterceptors() grpc.ServerOption {
	return grpc.ChainStreamInterceptor(
		serverStreamLogInterceptor,
	)
}

func serverLogInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	start := time.Now()
	log := logger.NewLogger("fsapi").With(zap.String("method", info.FullMethod))
	resp, err = handler(ctx, req)
	log.Infow("invoked grpc", "duration", time.Since(start).String(), "err", err)
	return
}

func serverStreamLogInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	start := time.Now()
	log := logger.NewLogger("fsapi").With(zap.String("method", info.FullMethod))
	err = handler(srv, ss)
	log.Debugw("stream", "duration", time.Since(start).String(), "err", err)
	return
}

func serverAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	var valueCtx context.Context
	valueCtx, err = withCallerAuthentication(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}
	return handler(valueCtx, req)
}

var (
	insecureMethods = map[string]bool{
		"/api.v1.Auth/AccessToken": true,
	}
)

func withCallerAuthentication(ctx context.Context, fullMethod string) (context.Context, error) {
	if !insecureMethods[fullMethod] {
		caller := CallerAuth(ctx)
		if !caller.Authenticated {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		valueCtx := context.WithValue(ctx, types.NamespaceKey, caller.Namespace)
		return valueCtx, nil
	}
	return ctx, nil
}
