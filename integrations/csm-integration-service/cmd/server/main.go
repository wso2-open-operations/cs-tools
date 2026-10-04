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
	"net/mail"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/middleware"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	cfg := entity.Config{
		BaseURL:      mustEnv("ENTITY_BASE_URL"),
		TokenURL:     mustEnv("ENTITY_TOKEN_URL"),
		ClientID:     mustEnv("ENTITY_CLIENT_ID"),
		ClientSecret: mustEnv("ENTITY_CLIENT_SECRET"),
		// Scopes is optional; set ENTITY_SCOPES as a comma-separated list if required.
		Scopes: splitComma(os.Getenv("ENTITY_SCOPES")),
	}

	entityClient := entity.NewClient(cfg)
	// UMT_INTEGRATION_ACTOR_EMAIL is this service's own trusted M2M actor
	// identity, asserted on POST /cases/{id}/comments (CreateCaseComment)
	// and POST /cases/{id}/tags (AddCaseTag), as entity-service's
	// actorEmail field. It must match an entry in entity-service's
	// M2M_TRUSTED_ACTOR_EMAILS allowlist or every call needing it 403s.
	// Required at startup: an unset or malformed value would otherwise only
	// show up later as a 403 on every case comment and tag write.
	umtActorEmail, err := actorEmail(os.Getenv("UMT_INTEGRATION_ACTOR_EMAIL"))
	if err != nil {
		slog.Error("invalid UMT_INTEGRATION_ACTOR_EMAIL", "err", err)
		os.Exit(1)
	}
	// GET /health reports whether the entity service is reachable (cached; see
	// handler.HealthHandler), not merely that this process is up.
	health := handler.NewHealthHandler(entityClient)

	// REQUIRE_OPERATION_SCOPES switches the per-operation scope check (see
	// cmd/server/routes.go for the route-to-scope table and
	// middleware.ScopeGuard for the check itself). Enforcement is the default
	// and the deployed configuration; "false" is for local development against
	// a bare client that forwards no token.
	guard := middleware.NewScopeGuard(envBool("REQUIRE_OPERATION_SCOPES", true))
	if !guard.Enforcing() {
		slog.Warn("operation scope enforcement is disabled; every caller may invoke every operation")
	}

	mux := newMux(newHandlers(entityClient, umtActorEmail, health), guard)

	addr := ":" + envOrDefault("PORT", "8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("Integration Service started", "addr", addr)

	// No authentication layer in this middleware chain: inbound callers are
	// authenticated at the API gateway (subscription + client credentials), not
	// validated again here. Authorization is per route, by the scope guard
	// applied in newMux, which reads the scope claim of the token the gateway
	// forwards.
	srv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CorrelationID(
				middleware.Logger(mux),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("Integration Service stopped")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// actorEmail validates the configured trusted-actor address: it must be set
// and be a single bare address (no display name, no list), since it is sent
// verbatim as the acting identity on case comment and tag writes.
func actorEmail(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errors.New("is not set")
	}
	addr, err := mail.ParseAddress(v)
	if err != nil || addr.Address != v {
		return "", errors.New("is not a single bare e-mail address")
	}
	return v, nil
}

// envBool reads a boolean environment variable, returning def when it is unset.
// A value that is set but not a boolean is a misconfiguration and stops startup,
// so a typo can never silently flip a default.
func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		slog.Error("environment variable is not a boolean", "key", key, "value", v)
		os.Exit(1)
	}
	return b
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

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
