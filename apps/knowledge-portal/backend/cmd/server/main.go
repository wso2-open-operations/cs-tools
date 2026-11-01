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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Command server runs the public Knowledge Base API. Everything it serves is
// public and read-only: published KB articles, active knowledge bases, search
// and related-article lookups. There is no authentication by design, so there is
// no identity or authorization middleware here; the controls that replace it are
// strict input validation, a forced published-only filter and whitelist response
// shapes (see internal/handler).
package main

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/pinecone"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	entityClient := entity.NewClient(entity.Config{
		BaseURL:      cfg.EntityBaseURL,
		TokenURL:     cfg.TokenURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.EntityScopes,
		Timeout:      cfg.Timeouts.EntityService,
	})

	var similarity handler.Similarity
	if cfg.PineconeHost != "" {
		pc, err := pinecone.New(cfg.PineconeHost, cfg.PineconeKey)
		if err != nil {
			slog.Error("invalid related-articles index configuration", "err", err)
			os.Exit(1)
		}
		similarity = pc
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := ":" + cfg.Port
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Handler:           newHandler(handler.NewKBHandler(entityClient, similarity)),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       cfg.Timeouts.RESTRead,
		WriteTimeout:      cfg.Timeouts.RESTWrite,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()
	slog.Info("Knowledge Portal Backend started", "addr", addr)

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("Knowledge Portal Backend stopped")
}

// newHandler builds the full HTTP handler: routes plus middleware. main and the
// tests both use it, so the tested chain is the served chain.
//
// SecurityHeaders stays outermost so its headers are on every response, error
// and panic responses included. Logger wraps Recovery so a recovered panic is
// recorded as a 500 in the access log. CORS is not handled here: the API
// gateway sets it.
func newHandler(kb *handler.KBHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /public/knowledge-bases", kb.ListKnowledgeBases)
	mux.HandleFunc("POST /public/kb-articles/search", kb.SearchArticles)
	mux.HandleFunc("GET /public/kb-articles/{id}", kb.GetArticle)
	mux.HandleFunc("GET /public/kb-articles/{id}/related", kb.RelatedArticles)
	mux.HandleFunc("/", handler.NotFound)

	return middleware.SecurityHeaders(
		middleware.CorrelationID(
			middleware.Logger(
				middleware.Recovery(mux),
			),
		),
	)
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Silently ignored if the file does not exist; logs a warning for any other error.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// Strip surrounding quotes from value.
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}
