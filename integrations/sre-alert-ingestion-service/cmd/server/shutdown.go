// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// drainer is the part of the server shutdown needs; *server.Server implements it.
type drainer interface {
	StartDraining()
}

// closer is the allocator's shutdown; *allocator.Allocator implements it.
type closer interface {
	Close(ctx context.Context) error
}

// budget splits server.shutdown_grace between the shutdown steps.
type budget struct {
	DrainDelay     time.Duration
	RequestWait    time.Duration
	AllocatorDrain time.Duration
}

// shutdown drains /healthz, in-flight requests, then the allocator, in that order, within server.shutdown_grace.
func shutdown(ctx context.Context, logger *slog.Logger, srv drainer, httpSrv *http.Server, alloc closer, b budget, after ...func(context.Context)) {
	start := time.Now()
	srv.StartDraining()
	select {
	case <-time.After(b.DrainDelay):
	case <-ctx.Done():
	}

	httpCtx, cancelHTTP := context.WithDeadline(ctx, start.Add(b.DrainDelay+b.RequestWait))
	if err := httpSrv.Shutdown(httpCtx); err != nil {
		logger.Error("http shutdown incomplete", "error", err)
	}
	cancelHTTP()

	allocCtx, cancelAlloc := context.WithTimeout(context.Background(), b.AllocatorDrain)
	if err := alloc.Close(allocCtx); err != nil {
		logger.Error("allocator did not drain within allocator_drain; claimed ids may be left without rows", "error", err)
	}
	cancelAlloc()

	for _, f := range after {
		f(ctx)
	}
	logger.Info("shutdown complete")
}
