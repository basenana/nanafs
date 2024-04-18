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
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
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
	err = checkCallerAuthentication(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

var (
	insecureMethods = map[string]bool{}
)

func checkCallerAuthentication(ctx context.Context, fullMethod string) error {
	if !insecureMethods[fullMethod] {
		caller := CallerAuth(ctx)
		if !caller.Authenticated {
			return status.Error(codes.Unauthenticated, "unauthenticated")
		}
	}
	return nil
}
