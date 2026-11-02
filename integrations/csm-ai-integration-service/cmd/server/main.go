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
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/catalogue"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/claude"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/dashboards"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/middleware"
)

// main reads the configuration, loads the catalogue, serves the routes and,
// on SIGINT or SIGTERM, shuts down after in-flight requests finish (see
// shutdownTimeout). A bad setting or an unreadable catalogue directory stops the start.
func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	// A catalogue directory that cannot be read fails the start, so a mistyped
	// mount path is found here and not on the first request.
	dashboardCatalogue, err := catalogue.NewDashboard(cfg.CatalogueDir)
	if err != nil {
		slog.Error("dashboard catalogue", "err", err)
		os.Exit(1)
	}
	if cfg.OldCatalogueFile {
		slog.Warn("dashboard catalogue: CATALOGUE_FILE is ignored, the parts in CATALOGUE_DIR are used; remove it")
	}
	// Never the key. The parts are listed in the order they were joined, so
	// the order can be checked in the log.
	slog.Info("model calls", "enabled", cfg.Enabled(), "model", cfg.Model, "timeout", cfg.Timeout,
		"gateway", cfg.GatewayHost(), "catalogue", dashboardCatalogue.Origin(),
		"catalogueParts", dashboardCatalogue.Parts())

	// Without a key the AI routes answer 503 not_configured.
	var dashboardGenerator *dashboards.Generator
	if cfg.Enabled() {
		client := claude.NewClient(claude.Config{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Timeout: cfg.Timeout})
		dashboardGenerator = dashboards.NewGenerator(client, cfg.Model, cfg.Timeout, dashboardCatalogue)
	}
	dashboardHandler := handler.NewDashboardHandler(dashboardGenerator)

	mux := newMux(dashboardHandler)

	addr := ":" + envOrDefault("PORT", "8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("AI Integration Service started", "addr", addr)

	// No Auth layer in this middleware chain: the endpoint is Organization-visible
	// and inbound requests are trusted at the Choreo gateway (the CSM portal
	// backend's client-credentials app), not validated again here. The portal
	// backend authenticates and authorises the end user before it calls us.
	srv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CorrelationID(
				middleware.UserID(
					middleware.Logger(mux),
				),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// A generate call waits for the model (up to CLAUDE_TIMEOUT), so the
		// write timeout leaves room for that and the reply.
		WriteTimeout: writeTimeout(cfg.Timeout),
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	// Wait for in-flight requests, which can include a model call of up to
	// CLAUDE_TIMEOUT, before exiting.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout(cfg.Timeout))
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("AI Integration Service stopped")
}

// writeTimeout is the server's write timeout: the model call's timeout plus a
// minute for reading the request and writing the reply, and never under two
// minutes.
func writeTimeout(modelTimeout time.Duration) time.Duration {
	return max(2*time.Minute, modelTimeout+time.Minute)
}

// shutdownTimeout is how long a shutdown waits for in-flight requests: a
// model call can run for the whole model timeout, plus 15 seconds to write
// its reply. The platform's termination grace period must be at least this
// long for the wait to help.
func shutdownTimeout(modelTimeout time.Duration) time.Duration {
	return modelTimeout + 15*time.Second
}

// envOrDefault is the environment variable key, or def when it is unset or empty.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
