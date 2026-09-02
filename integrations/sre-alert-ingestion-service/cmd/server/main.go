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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/csmclient"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/store"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/worker"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	// Buffer database. Dedicated to this service — never CSM's own database
	// (see this service's README/CLAUDE.md: the entire point is to not share
	// fate with CSM's own availability). Apply migrations/0001_create_alert_buffer.up.sql
	// against this DSN before first run; this process does not run migrations itself.
	dbStore, err := store.NewPostgresStore(mustEnv("SRE_ALERT_DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to buffer database", "err", err)
		os.Exit(1)
	}
	defer dbStore.Close()

	csmClient := csmclient.NewClient(csmclient.Config{
		BaseURL:      mustEnv("CSM_INTEGRATION_BASE_URL"),
		TokenURL:     mustEnv("CSM_INTEGRATION_TOKEN_URL"),
		ClientID:     mustEnv("CSM_INTEGRATION_CLIENT_ID"),
		ClientSecret: mustEnv("CSM_INTEGRATION_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("CSM_INTEGRATION_SCOPES")),
	})

	twilioClient := notifications.NewTwilioClient(notifications.TwilioConfig{
		AccountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		AuthToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		FromNumber: os.Getenv("TWILIO_FROM_NUMBER"),
		ToNumber:   os.Getenv("SRE_ALERT_ONCALL_NUMBER"),
		Voice:      os.Getenv("TWILIO_VOICE"),
		Language:   os.Getenv("TWILIO_LANGUAGE"),
		APIBaseURL: os.Getenv("TWILIO_API_BASE_URL"),
	})

	w := worker.New(dbStore, csmClient, twilioClient, worker.Config{
		MaxRetries:   envInt("SRE_ALERT_MAX_RETRIES", 3),
		PollInterval: time.Duration(envInt("SRE_ALERT_POLL_INTERVAL_SECONDS", 15)) * time.Second,
	})

	// callerID: a real, operator-provisioned CSM user id. CSM has no
	// "system"/machine-caller concept today, so this is required config,
	// never guessed here — see handler.AlertHandler's doc comment and this
	// service's README/CLAUDE.md.
	alertHandler := handler.NewAlertHandler(dbStore, mustEnv("SRE_ALERT_CALLER_ID"))
	healthHandler := handler.NewHealthHandler(dbStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("POST /alerts", alertHandler.CreateAlert)

	addr := ":" + envOrDefault("PORT", "8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Background retry/escalation worker — runs independently of the HTTP
	// server's own lifecycle, stopped by the same shutdown signal.
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go w.Run(workerCtx)

	slog.Info("SRE Alert Ingestion Service started", "addr", addr)

	// No Auth layer in this middleware chain — inbound requests are trusted at the
	// Choreo API Manager gateway, not validated again in this service, matching
	// csm-integration-service's own convention. See this service's CLAUDE.md.
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

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()
	stopWorker()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("SRE Alert Ingestion Service stopped")
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

// envInt parses key as an int, falling back to def on anything unset or
// unparseable (with a warning logged for the latter, not the former).
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("invalid integer environment variable, using default", "key", key, "value", v, "default", def)
		return def
	}
	return n
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
