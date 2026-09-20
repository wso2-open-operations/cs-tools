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
	"net/url"
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
	//
	// Configured as discrete DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/
	// DB_SSLMODE vars, not a single connection-string env var, matching
	// entity-service's internal/config.Config — a hand-built
	// postgres://user:password@host/db string requires the operator to
	// manually percent-encode any reserved character in the password (a
	// bare "?" gets read as the start of the query string), which
	// url.UserPassword below does automatically.
	dbStore, err := store.NewPostgresStore(buildDatabaseDSN())
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

	googleChatClient := notifications.NewGoogleChatClient(notifications.GoogleChatConfig{
		WebhookURL: os.Getenv("GOOGLE_CHAT_ESCALATION_WEBHOOK_URL"),
	})

	emailClient := notifications.NewEmailClient(notifications.EmailConfig{
		BaseURL:      os.Getenv("EMAIL_SERVICE_BASE_URL"),
		TokenURL:     os.Getenv("EMAIL_SERVICE_TOKEN_URL"),
		ClientID:     os.Getenv("EMAIL_SERVICE_CLIENT_ID"),
		ClientSecret: os.Getenv("EMAIL_SERVICE_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("EMAIL_SERVICE_SCOPES")),
		FromAddress:  os.Getenv("SRE_ALERT_ESCALATION_EMAIL_FROM"),
		ToAddresses:  splitComma(os.Getenv("SRE_ALERT_ESCALATION_EMAIL_TO")),
	})

	// All three channels fire independently on escalation — deliberate
	// redundancy, not a first-success-wins race. See
	// notifications.MultiChannelEscalator's own doc comment.
	escalator := notifications.NewMultiChannelEscalator(
		twilioClient, googleChatClient, emailClient,
		"SRE Alert Ingestion Service: incident delivery to CSM has been failing",
	)

	w := worker.New(dbStore, csmClient, escalator, worker.Config{
		MaxRetries:   envInt("SRE_ALERT_MAX_RETRIES", 3),
		PollInterval: time.Duration(envInt("SRE_ALERT_POLL_INTERVAL_SECONDS", 15)) * time.Second,
		GroupWindow:  time.Duration(envInt("SRE_ALERT_GROUP_WINDOW_MINUTES", 15)) * time.Minute,
	})

	// callerID: a real, operator-provisioned CSM user id. CSM has no
	// "system"/machine-caller concept today, so this is required config,
	// never guessed here — see handler.AlertHandler's doc comment and this
	// service's README/CLAUDE.md.
	alertHandler := handler.NewAlertHandler(dbStore, mustEnv("SRE_ALERT_CALLER_ID"))
	healthHandler := handler.NewHealthHandler(dbStore)

	// SRE_ALERT_AUTH_USERS is required: this service's only inbound
	// authentication is HTTP Basic Auth on POST /alerts (see the wiring
	// comment below), so a missing/malformed value must fail startup, not
	// silently leave the route unauthenticated. See internal/middleware.BasicAuth
	// and cmd/gen-basic-auth-hash for the credential format and how to
	// generate a hash.
	authUsers, err := middleware.ParseBasicAuthUsers(mustEnv("SRE_ALERT_AUTH_USERS"))
	if err != nil {
		slog.Error("invalid SRE_ALERT_AUTH_USERS", "err", err)
		os.Exit(1)
	}
	basicAuth := middleware.BasicAuth(authUsers)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.Handle("POST /alerts", basicAuth(http.HandlerFunc(alertHandler.CreateAlert)))

	// Vendor-adapter routes: each translates one vendor's own native
	// alert-webhook payload into AlertRequest, then reuses the exact same
	// validation/buffering/worker/grouping/dedup/escalation path as POST
	// /alerts above (see internal/handler.AlertHandler.enqueueAlert). One
	// dedicated route per vendor shape, not a single shared endpoint that
	// branches on payload shape internally — deliberately simpler to reason
	// about, route, and test than that alternative. Same basicAuth
	// middleware as POST /alerts: every inbound route on this service
	// authenticates itself, with no exceptions (see the comment on srv
	// below).
	mux.Handle("POST /alerts/adapters/azure", basicAuth(http.HandlerFunc(alertHandler.CreateAlertFromAzure)))
	mux.Handle("POST /alerts/adapters/site24x7", basicAuth(http.HandlerFunc(alertHandler.CreateAlertFromSite24x7)))
	mux.Handle("POST /alerts/adapters/opensearch", basicAuth(http.HandlerFunc(alertHandler.CreateAlertFromOpenSearch)))
	mux.Handle("POST /alerts/adapters/grafana", basicAuth(http.HandlerFunc(alertHandler.CreateAlertFromGrafana)))

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
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		w.Run(workerCtx)
	}()

	slog.Info("SRE Alert Ingestion Service started", "addr", addr)

	// This service is deployed on AKS with no gateway/ingress auth layer in
	// front of it — unlike this repo's other integrations/* services, which
	// sit behind Choreo's API Manager and trust inbound requests at the
	// gateway. So POST /alerts authenticates every request itself, end to
	// end, via the per-route HTTP Basic Auth middleware wired above
	// (middleware.BasicAuth) — that is this service's sole inbound
	// authentication enforcement point, not a layer added on top of
	// something else. GET /health deliberately stays unauthenticated so
	// liveness/readiness probes don't need credentials. The outer chain
	// below (SecurityHeaders/CorrelationID/Logger) applies to every route
	// but performs no authentication of its own.
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

	// Wait for the worker to actually stop before closing dbStore (deferred
	// above) and exiting. Without this, an in-flight w.attempt can still be
	// mid-CreateIncident when the process exits: CSM creates the incident,
	// MarkDelivered never runs, the row stays pending with RetryCount == 0,
	// and the next start's retry skips the pre-retry dedup search (which
	// only runs when RetryCount > 0) and creates a duplicate incident.
	select {
	case <-workerDone:
	case <-time.After(15 * time.Second):
		slog.Warn("worker did not stop within the shutdown timeout")
	}
	slog.Info("SRE Alert Ingestion Service stopped")
}

// buildDatabaseDSN reads the discrete DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/
// DB_NAME/DB_SSLMODE environment variables and builds a postgres:// DSN from
// them via databaseDSN. DB_HOST/DB_PORT default to "localhost"/"5432";
// DB_USER/DB_PASSWORD/DB_NAME are required (mustEnv exits the process if any
// is unset); DB_SSLMODE has no default (an empty sslmode is a valid,
// meaningful value — pgx applies its own default behavior for it).
func buildDatabaseDSN() string {
	return databaseDSN(
		envOrDefault("DB_HOST", "localhost"),
		envOrDefault("DB_PORT", "5432"),
		mustEnv("DB_USER"),
		mustEnv("DB_PASSWORD"),
		mustEnv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)
}

// databaseDSN constructs a postgres:// connection string from discrete
// host/port/user/password/name/sslmode parts, matching entity-service's
// internal/config.Config.DSN(). Building it via net/url + url.UserPassword
// (rather than string concatenation) means any reserved character in user or
// password — "?", "@", "/", a space, etc. — is automatically percent-encoded,
// so the resulting DSN always parses back to the exact input.
func databaseDSN(host, port, user, password, name, sslmode string) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		// net.JoinHostPort, not "+":" + port" — a bare IPv6 literal
		// (DB_HOST=2001:db8::1) needs brackets ("[2001:db8::1]:5432") to
		// keep its own colons from being read as the host:port separator;
		// JoinHostPort adds them only when host contains a colon, so
		// hostnames and IPv4 addresses are unaffected.
		Host: net.JoinHostPort(host, port),
		Path: name,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
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
