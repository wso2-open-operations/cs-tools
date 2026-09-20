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

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/handler"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/jobs"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/middleware"
)

// main wires up config, the DB pool, config→DB sync, the job lock/recompute
// scheduler, and the HTTP handler chain, then serves until a signal arrives.
func main() {
	loadDotEnv(".env")
	configureLogger()

	appCfg, err := appconfig.Load()
	if err != nil {
		slog.Error("invalid app-config.yaml", "err", err)
		os.Exit(1)
	}
	if _, statErr := os.Stat(appconfig.ResolvedPath()); statErr != nil {
		slog.Info("app config: file absent, using built-in defaults", "path", appconfig.ResolvedPath())
	} else {
		slog.Info("app config loaded", "path", appconfig.ResolvedPath())
	}

	// Boot-only: must run before any github.Client or jobs.Scheduler/Lock is
	// constructed below, since both packages' tuning vars are not safe to
	// change concurrently with in-flight requests.
	github.Apply(appCfg.GitHub)
	jobs.Apply(appCfg.Jobs)

	slaCfg, err := config.Load()
	if err != nil {
		slog.Error("invalid sla-config.yaml", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseURL := mustEnv("DATABASE_URL")
	pool, err := db.NewPoolWithConfig(ctx, databaseURL, appCfg.Database)
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Boot-time config→DB sync: idempotent, keeps projects/repositories in
	// referential-integrity lockstep with sla-config.yaml before anything
	// else touches the database.
	syncSummary, err := db.SyncConfigToDB(ctx, pool, slaCfg)
	if err != nil {
		slog.Error("config sync failed", "err", err)
		os.Exit(1)
	}
	slog.Info("config sync complete", "activeRepos", syncSummary.ActiveRepos, "disabledRepos", syncSummary.DisabledRepos)

	// Shared job lock: one instance for both the recompute scheduler and
	// POST /sync/runs, so a manual sync and a scheduled tick never interleave
	// on this replica or any other.
	runtime := ingest.BuildRuntimeConfig(slaCfg)
	lock := jobs.NewLock(databaseURL)

	// Recompute scheduler: one tick immediately, then every
	// recomputeIntervalMinutes. RECOMPUTE_ENABLED=0 disables it (tests/CI).
	if os.Getenv("RECOMPUTE_ENABLED") != "0" {
		interval := time.Duration(slaCfg.Settings.RecomputeIntervalMinutes) * time.Minute
		jobs.NewScheduler(pool, lock, runtime, interval).Start(ctx)
		slog.Info("recompute scheduler started", "intervalMinutes", slaCfg.Settings.RecomputeIntervalMinutes)
	} else {
		slog.Info("recompute scheduler disabled via RECOMPUTE_ENABLED=0")
	}

	githubToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))

	healthHandler := handler.NewHealthHandler(pool, appCfg.Readiness)
	taxonomyHandler := handler.NewTaxonomyHandler(slaCfg)
	issuesHandler := handler.NewIssuesHandler(pool, slaCfg, appCfg.API)
	metricsHandler := handler.NewMetricsHandler(pool, slaCfg, appCfg.Cache, appCfg.API)
	titlesHandler := handler.NewTitlesHandler(pool, githubToken, appCfg.Cache.Titles, appCfg.GitHub.TitlesBatchSize, appCfg.API)
	syncHandler := handler.NewSyncHandler(pool, slaCfg, lock, runtime, githubToken,
		time.Duration(appCfg.Jobs.SyncRunDeadlineMinutes)*time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler.GetHealthz)
	mux.HandleFunc("GET /readyz", healthHandler.GetReadyz)
	mux.HandleFunc("GET /taxonomy", taxonomyHandler.GetTaxonomy)
	mux.HandleFunc("GET /issues", issuesHandler.ListIssues)
	mux.HandleFunc("GET /issues/{id}", issuesHandler.GetIssue)
	mux.HandleFunc("POST /issues/titles", titlesHandler.PostTitles)
	mux.HandleFunc("GET /metrics/overview", metricsHandler.GetOverview)
	mux.HandleFunc("GET /metrics/timeseries", metricsHandler.GetTimeseries)
	mux.HandleFunc("POST /sync/runs", syncHandler.PostSyncRuns)
	mux.HandleFunc("GET /sync/status", syncHandler.GetSyncStatus)

	// Handler chain, outermost first: Recovery must be outermost so a panic
	// anywhere downstream (including in SecurityHeaders, CORS, or Logger)
	// still gets the standard error envelope instead of an empty connection
	// reset. SecurityHeaders wraps CORS (rather than the other way around)
	// so a CORS preflight — which short-circuits before reaching Logger's
	// wrapped handler — still carries the same security headers as every
	// other response; see middleware.SecurityHeaders's doc comment. CORS
	// wraps Logger so a preflight still gets its own headers; see
	// middleware.CORS's doc comment for why this backend needs CORS at all
	// despite doing no authentication of its own.
	rootHandler := middleware.Recovery(
		middleware.SecurityHeaders(appCfg.SecurityHeaders)(
			middleware.CORS(splitComma(os.Getenv("CORS_ALLOWED_ORIGINS")))(
				middleware.Logger(mux),
			),
		),
	)

	addr := ":" + mustPort("PORT", strconv.Itoa(appCfg.Server.Port))
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Handler:           rootHandler,
		ReadHeaderTimeout: time.Duration(appCfg.Server.ReadHeaderTimeoutSeconds) * time.Second,
		ReadTimeout:       time.Duration(appCfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(appCfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(appCfg.Server.IdleTimeoutSeconds) * time.Second,
		MaxHeaderBytes:    appCfg.Server.MaxHeaderBytes,
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()
	slog.Info("git-internals-dashboard backend started", "addr", addr)

	<-ctx.Done()
	stop()

	// Give the load balancer time to notice this replica is going away and
	// stop routing here before in-flight requests get cut: /readyz starts
	// reporting 503 "draining" now, /healthz keeps reporting 200 since the
	// process is still alive.
	healthHandler.BeginDraining()
	if drain := time.Duration(appCfg.Readiness.DrainGracePeriodSeconds) * time.Second; drain > 0 {
		slog.Info("draining before shutdown", "seconds", appCfg.Readiness.DrainGracePeriodSeconds)
		time.Sleep(drain)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(appCfg.Server.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}

	slog.Info("git-internals-dashboard backend stopped")
}

// configureLogger installs a level-controlled slog handler as the process
// default. LOG_LEVEL (default "info") accepts slog's usual names
// (debug/info/warn/error); an unrecognized value falls back to info with a
// warning rather than failing boot over a logging preference.
func configureLogger() {
	level := slog.LevelInfo
	if raw := strings.TrimSpace(os.Getenv("LOG_LEVEL")); raw != "" {
		if err := level.UnmarshalText([]byte(raw)); err != nil {
			level = slog.LevelInfo
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
			slog.Warn("LOG_LEVEL is not a recognized slog level; defaulting to info", "value", raw)
			return
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

// loadDotEnv reads a .env file and sets any unset environment variables from
// it. Silently ignored if the file does not exist; logs a warning for any
// other error.
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

// mustEnv returns the value of the given required environment variable,
// exiting the process with a logged error if it is unset.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

// splitComma splits s on commas into trimmed, non-empty entries, or nil for
// an empty/whitespace-only string.
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

// mustPort returns the value of the given environment variable (or def if
// unset) as a bare port number, e.g. "8080". Exits the process if the value
// isn't a valid TCP port.
func mustPort(key, def string) string {
	v := def
	if raw := os.Getenv(key); raw != "" {
		v = raw
	}
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Error("environment variable must be a plain port number (e.g. \"8080\"), not an address", "key", key, "value", v)
		os.Exit(1)
	}
	return v
}
