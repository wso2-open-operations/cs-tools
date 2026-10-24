// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
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

// Command server runs the Today I Learned backend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/config"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/novera"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/store"
)

// maxBodyBytes caps a request body. The largest valid one, a full-length
// learning in heavy formatting, is a small fraction of this.
const maxBodyBytes = 256 << 10

func main() {
	middleware.ConfigureLogger(os.Stderr)
	if err := run(); err != nil {
		slog.Error("til-backend stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, warnings, err := config.Load(os.Getenv)
	for _, w := range warnings {
		slog.Warn(w)
	}
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.CheckSchema(ctx); err != nil {
		return err
	}

	keyFunc, err := middleware.NewKeyfunc(ctx, cfg.JWKSEndpoint)
	if err != nil {
		return err
	}

	hc := handler.Config{Store: db, WebappClientID: cfg.WebappClientID, WebappBaseURL: cfg.WebappBaseURL, WhatMaxLength: cfg.WhatMaxLength}
	if cfg.Novera != nil {
		hc.Notifier = novera.New(*cfg.Novera)
	}

	srv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CORS(cfg.CORSOrigins)(
				middleware.CorrelationID(
					middleware.BodyLimit(maxBodyBytes)(
						middleware.Auth(cfg.Auth, keyFunc)(
							routes(handler.New(hc))))))),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", ":"+cfg.Port)
	if err != nil {
		return err
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	slog.Info("til-backend listening", "port", cfg.Port)

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	slog.Info("til-backend shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// routes registers every endpoint. Each one is also described in
// openapi.yaml; TestEveryRouteIsInOpenAPI fails if one isn't.
func routes(h *handler.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /user-info", h.UserInfo)
	mux.HandleFunc("GET /settings", h.Settings)
	mux.HandleFunc("GET /submissions", h.ListEntries)
	mux.HandleFunc("POST /submissions", h.CreateEntry)
	mux.HandleFunc("GET /submissions/{id}", h.GetEntry)
	mux.HandleFunc("PUT /submissions/{id}", h.UpdateEntry)
	mux.HandleFunc("DELETE /submissions/{id}", h.DeleteEntry)
	mux.HandleFunc("POST /submissions/{id}/notify", h.NotifyEntry)
	return mux
}
