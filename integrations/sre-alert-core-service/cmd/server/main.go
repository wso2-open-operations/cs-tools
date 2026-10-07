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

// Command server wires Postgres, the poller, dedup engine, notifier and retention; every replica runs independently, no leader election.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	schema "alert-core-service"
	"alert-core-service/internal/auth"
	"alert-core-service/internal/config"
	"alert-core-service/internal/csm"
	"alert-core-service/internal/engine"
	"alert-core-service/internal/hub"
	"alert-core-service/internal/model"
	"alert-core-service/internal/notify"
	"alert-core-service/internal/pglock"
	"alert-core-service/internal/poll"
	"alert-core-service/internal/postgres"
	"alert-core-service/internal/store"
)

// dbProbeTimeout bounds GET /dbz's ping, and dbProbeEvery is how long one result is reused.
const (
	dbProbeTimeout = 2 * time.Second
	dbProbeEvery   = time.Second
)

// lockPoolHeadroom covers the retention job's lock and connection churn beyond notify.delivery_concurrency delivery workers.
const lockPoolHeadroom = 8

func main() {
	base := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("app", "alert-core-service")
	logger := base.With("component", "main")

	depCfg, err := config.Load("")
	if err != nil {
		logger.Error("failed to load deployment config", "error", err)
		os.Exit(1)
	}

	pgCfg, err := postgres.ConfigFromEnv()
	if err != nil {
		logger.Error("failed to read postgres config", "error", err)
		os.Exit(1)
	}
	pgCfg, err = postgres.SizePool(pgCfg, depCfg.Poll.Concurrency)
	if err != nil {
		logger.Error("invalid postgres pool size", "error", err)
		os.Exit(1)
	}
	logger.Info("postgres pools sized", "main_max_conns", pgCfg.PoolMaxConns, "lock_max_conns", depCfg.Notify.DeliveryConcurrency+lockPoolHeadroom)
	// Core writes are replayed after a crash, so they skip the WAL flush wait; ingestion keeps synchronous commits since it acknowledges senders.
	pool, err := connectWithRetry(logger, pgCfg, depCfg.Postgres, true)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.Migrate(context.Background(), pool, schema.SQL); err != nil {
		logger.Error("failed to apply schema", "error", err)
		os.Exit(1)
	}

	// Delivery holds a session advisory lock across CSM/Chat calls, so it gets its own pool and never starves fold queries.
	lockCfg := pgCfg
	lockCfg.PoolMaxConns = int32(depCfg.Notify.DeliveryConcurrency + lockPoolHeadroom)
	lockPool, err := connectWithRetry(logger, lockCfg, depCfg.Postgres, false)
	if err != nil {
		logger.Error("failed to connect lock pool to postgres", "error", err)
		os.Exit(1)
	}
	defer lockPool.Close()
	locker := pglock.New(stdlib.OpenDBFromPool(lockPool))

	alerts := store.NewAlertRepo(pool)
	incidents := store.NewIncidentRepo(pool, locker)
	defaults, err := model.LoadDefaults()
	if err != nil {
		logger.Error("failed to load alert defaults", "error", err)
		os.Exit(1)
	}

	csmClient := csmClientFromEnv(logger, depCfg.Notify.HTTPTimeout.Duration())
	notifyCfg := notify.Config{
		CallerID:         os.Getenv("CSM_CALLER_ID"),
		UnknownServiceID: os.Getenv("CSM_UNKNOWN_SERVICE_ID"),
		// Optional: the group an incident is assigned to when nothing more specific routes it.
		DefaultAssignmentGroupID: os.Getenv("CSM_DEFAULT_ASSIGNMENT_GROUP_ID"),
		AssignmentGroupRoutes:    assignmentGroupRoutes(logger),
		ServiceCacheTTL:          depCfg.Notify.ServiceCacheTTL.Duration(),
		MaxAttempts:              depCfg.Notify.MaxAttempts,
		RetryBaseDelay:           depCfg.Notify.RetryBaseDelay.Duration(),
		HTTPTimeout:              depCfg.Notify.HTTPTimeout.Duration(),
		ChatThreadingEnabled:     depCfg.Notify.ChatThreadingEnabled,
	}
	if err := notify.ValidateGroupIDs(notifyCfg); err != nil {
		logger.Error("invalid assignment group configuration", "error", err)
		os.Exit(1)
	}
	notifier := notify.New(base.With("component", "notify"), csmClient, notifyCfg)
	eng := engine.New(base.With("component", "engine"), incidents, notifier, engine.Config{
		Defaults:             defaults,
		DedupWindow:          depCfg.Engine.DedupWindow.Duration(),
		MaxCSMAttempts:       depCfg.Notify.MaxCSMAttempts,
		StateCheckInterval:   depCfg.Notify.StateCheckInterval.Duration(),
		ChatThreadingEnabled: depCfg.Notify.ChatThreadingEnabled,
		ChatFallbackDelay:    depCfg.Notify.ChatFallbackDelay.Duration(),
		DeliveryConcurrency:  depCfg.Notify.DeliveryConcurrency,
		CSMRetry: engine.CSMRetryConfig{
			BaseDelay:  depCfg.Notify.CSMRetryBaseDelay.Duration(),
			Multiplier: depCfg.Notify.CSMRetryMultiplier,
			MaxDelay:   depCfg.Notify.CSMRetryMaxDelay.Duration(),
		},
	})
	poller := poll.New(base.With("component", "poll"), alerts, eng, poll.Settings{
		Interval:              depCfg.Poll.Interval.Duration(),
		Concurrency:           depCfg.Poll.Concurrency,
		MaxBatch:              depCfg.Poll.MaxBatch,
		ClaimTTL:              depCfg.Poll.ClaimTTL.Duration(),
		DeliverySweepInterval: depCfg.Notify.RetrySweepInterval.Duration(),
	})
	retention := &store.Retention{
		Logger:      base.With("component", "retention"),
		Alerts:      alerts,
		Incidents:   incidents,
		Locker:      locker,
		Interval:    depCfg.Retention.Interval.Duration(),
		AlertTTL:    depCfg.Retention.Alerts.Duration(),
		IncidentTTL: depCfg.Retention.Incidents.Duration(),
	}
	h := hub.New(poller)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pollCtx, cancelPoll := context.WithCancel(context.Background())
	defer cancelPoll()
	pollerDone := make(chan struct{})
	go func() {
		defer close(pollerDone)
		poller.Run(pollCtx)
	}()
	go retention.Run(pollCtx)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			logger.Warn("health check failed: postgres unreachable", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	// dbz reports only whether Postgres is reachable, reusing one ping per dbProbeEvery so frequent probes can't drain the pool.
	dbProbe := postgres.NewProbe(pool, dbProbeTimeout, dbProbeEvery)
	mux.HandleFunc("GET /dbz", func(w http.ResponseWriter, r *http.Request) {
		if err := dbProbe.Check(r.Context()); err != nil {
			logger.Warn("db health check failed: postgres unreachable", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	// livez skips Postgres so a DB blip doesn't trigger pod restarts via the liveness probe.
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// alert-ingestion wakes the poller with a shared token, so /alertz never needs Postgres or an integration user.
	wakeToken := strings.TrimSpace(os.Getenv("ALERT_CORE_WAKE_TOKEN"))
	switch {
	case wakeToken == "":
		logger.Warn("ALERT_CORE_WAKE_TOKEN not set; /alertz rejects every wake and alerts are picked up by poll.interval alone")
	case len(wakeToken) < auth.MinWakeTokenLen:
		logger.Error("ALERT_CORE_WAKE_TOKEN is too short; generate one with openssl rand -hex 32", "min_length", auth.MinWakeTokenLen)
		os.Exit(1)
	}
	mux.Handle("/alertz", auth.RequireWakeToken(wakeToken, base.With("component", "auth"))(http.HandlerFunc(h.ServeAlert)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: mux}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "port", port)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		cancelPoll()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
		// Restores default signal handling so a second Ctrl-C force-kills instead of being ignored.
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), depCfg.Server.ShutdownGrace.Duration())
		defer cancel()

		cancelPoll()
		select {
		case <-pollerDone:
		case <-shutdownCtx.Done():
			logger.Warn("poller did not drain within shutdown_grace")
		}
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}
}

// assignmentGroupRoutes reads CSM_ASSIGNMENT_GROUP_ROUTES, a JSON object of routing key -> CSM group id.
// Optional; one that does not parse stops startup rather than routing every incident to the default.
func assignmentGroupRoutes(logger *slog.Logger) map[string]string {
	raw := strings.TrimSpace(os.Getenv("CSM_ASSIGNMENT_GROUP_ROUTES"))
	if raw == "" {
		return nil
	}
	var routes map[string]string
	if err := json.Unmarshal([]byte(raw), &routes); err != nil {
		logger.Error("CSM_ASSIGNMENT_GROUP_ROUTES is not a JSON object of string to string", "error", err)
		os.Exit(1)
	}
	return routes
}

// csmEnvVars must all be set to enable CSM delivery; otherwise incidents are tracked locally and surfaced via Chat only.
var csmEnvVars = []string{
	"CSM_INTEGRATION_BASE_URL",
	"CSM_INTEGRATION_TOKEN_URL",
	"CSM_INTEGRATION_CLIENT_ID",
	"CSM_INTEGRATION_CLIENT_SECRET",
	"CSM_CALLER_ID",
	"CSM_UNKNOWN_SERVICE_ID",
}

// csmClientFromEnv returns nil, disabling CSM, unless every csmEnvVars entry is set.
func csmClientFromEnv(logger *slog.Logger, httpTimeout time.Duration) *csm.Client {
	var missing []string
	for _, name := range csmEnvVars {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == len(csmEnvVars) {
		logger.Warn("CSM not configured; incidents are tracked locally and sent to Chat only")
		return nil
	}
	if len(missing) > 0 {
		logger.Error("CSM partially configured, disabling CSM delivery", "missing", missing)
		return nil
	}
	return csm.NewClient(csm.Config{
		BaseURL:      os.Getenv("CSM_INTEGRATION_BASE_URL"),
		TokenURL:     os.Getenv("CSM_INTEGRATION_TOKEN_URL"),
		ClientID:     os.Getenv("CSM_INTEGRATION_CLIENT_ID"),
		ClientSecret: os.Getenv("CSM_INTEGRATION_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("CSM_INTEGRATION_SCOPES")),
		HTTPTimeout:  httpTimeout,
	})
}

func splitComma(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// connectWithRetry retries with exponential backoff so a transient startup outage doesn't crash the server.
func connectWithRetry(logger *slog.Logger, cfg postgres.Config, pcfg config.PostgresConfig, asyncCommit bool) (*pgxpool.Pool, error) {
	attempt := 0
	operation := func() (*pgxpool.Pool, error) {
		attempt++
		p, err := postgres.Connect(cfg, pcfg.ConnectTimeout.Duration(), pcfg.QueryTimeout.Duration(), asyncCommit)
		if err != nil {
			logger.Warn("postgres connection failed, retrying", "attempt", attempt, "max_attempts", pcfg.ConnectMaxAttempts, "error", err)
		}
		return p, err
	}

	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = pcfg.ConnectBaseDelay.Duration()
	return backoff.Retry(context.Background(), operation, backoff.WithBackOff(eb), backoff.WithMaxTries(uint(pcfg.ConnectMaxAttempts)))
}
