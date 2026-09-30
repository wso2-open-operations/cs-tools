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

// Command server wires Cassandra, the poller, dedup engine, and notifier, then serves health and alert endpoints.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cenkalti/backoff/v4"
	"github.com/gocql/gocql"

	"alert-core-service/internal/auth"
	"alert-core-service/internal/cassandra"
	"alert-core-service/internal/config"
	"alert-core-service/internal/csm"
	"alert-core-service/internal/engine"
	"alert-core-service/internal/hub"
	"alert-core-service/internal/lease"
	"alert-core-service/internal/model"
	"alert-core-service/internal/notify"
	"alert-core-service/internal/poll"
	"alert-core-service/internal/store"
)

func main() {
	base := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("app", "alert-core-service")
	logger := base.With("component", "main")

	depCfg, err := config.Load("")
	if err != nil {
		logger.Error("failed to load deployment config", "error", err)
		os.Exit(1)
	}

	cfg, err := cassandra.ConfigFromEnv()
	if err != nil {
		logger.Error("failed to read cassandra config", "error", err)
		os.Exit(1)
	}
	session, err := connectWithRetry(logger, cfg, depCfg.Cassandra)
	if err != nil {
		logger.Error("failed to connect to cassandra", "error", err)
		os.Exit(1)
	}
	defer session.Close()

	// Elects one active processor across replicas so multiple containers don't duplicate notifications.
	processorLease, err := lease.New(session, base.With("component", "lease"), lease.Identity(), depCfg.Lease.TTL.Duration())
	if err != nil {
		logger.Error("failed to initialise processor lease", "error", err)
		os.Exit(1)
	}

	userRepo := auth.NewUserRepo(session)
	alerts := store.NewAlertRepo(session)
	incidents, err := store.NewIncidentRepo(session, depCfg.Poll.MaxWindow, depCfg.Engine.DedupWindow.Duration())
	if err != nil {
		logger.Error("failed to initialise incident repository", "error", err)
		os.Exit(1)
	}
	// Backfill pending index for pre-existing rows; startup continues if this fails as it will retry on next restart.
	if err := incidents.BackfillPendingIndex(context.Background()); err != nil {
		logger.Warn("failed to backfill pending incident index, will retry on next restart", "error", err)
	}
	defaults, err := model.LoadDefaults()
	if err != nil {
		logger.Error("failed to load alert defaults", "error", err)
		os.Exit(1)
	}

	csmClient := csm.NewClient(csm.Config{
		BaseURL:      mustEnv(logger, "CSM_INTEGRATION_BASE_URL"),
		TokenURL:     mustEnv(logger, "CSM_INTEGRATION_TOKEN_URL"),
		ClientID:     mustEnv(logger, "CSM_INTEGRATION_CLIENT_ID"),
		ClientSecret: mustEnv(logger, "CSM_INTEGRATION_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("CSM_INTEGRATION_SCOPES")),
	})
	notifier := notify.New(base.With("component", "notify"), csmClient, notify.Config{
		CallerID:         mustEnv(logger, "CSM_CALLER_ID"),
		UnknownServiceID: mustEnv(logger, "CSM_UNKNOWN_SERVICE_ID"),
		ServiceCacheTTL:  depCfg.Notify.ServiceCacheTTL.Duration(),
		MaxAttempts:      depCfg.Notify.MaxAttempts,
		RetryBaseDelay:   depCfg.Notify.RetryBaseDelay.Duration(),
		HTTPTimeout:      depCfg.Notify.HTTPTimeout.Duration(),
	})
	eng := engine.New(base.With("component", "engine"), alerts, incidents, notifier, defaults, depCfg.Notify.MaxCSMAttempts, depCfg.Notify.StateCheckInterval.Duration(), depCfg.Engine.DedupWindow.Duration(), engine.CSMRetryConfig{
		BaseDelay:  depCfg.Notify.CSMRetryBaseDelay.Duration(),
		Multiplier: depCfg.Notify.CSMRetryMultiplier,
		MaxDelay:   depCfg.Notify.CSMRetryMaxDelay.Duration(),
	})
	poller, err := poll.New(base.With("component", "poll"), session, eng, processorLease, poll.Settings{
		Interval:            depCfg.Poll.Interval.Duration(),
		Concurrency:         depCfg.Poll.Concurrency,
		ReadConcurrency:     depCfg.Poll.ReadConcurrency,
		MaxWindow:           depCfg.Poll.MaxWindow,
		NotifySweepInterval: depCfg.Notify.RetrySweepInterval.Duration(),
		GapTimeout:          depCfg.Poll.GapTimeout.Duration(),
	})
	if err != nil {
		logger.Error("failed to initialise poller", "error", err)
		os.Exit(1)
	}
	h := hub.New(poller)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// leaseCtx stays alive through drain so renewal outlives it; lease could expire mid-delivery if renewal stopped at SIGTERM.
	leaseCtx, cancelLease := context.WithCancel(context.Background())
	defer cancelLease()
	leaseDone := make(chan struct{})
	go func() {
		defer close(leaseDone)
		processorLease.Run(leaseCtx, depCfg.Lease.RenewInterval.Duration())
	}()

	// pollCtx is separate from ctx so shutdown can drain the poller before releasing the lease, avoiding duplicate sends.
	pollCtx, cancelPoll := context.WithCancel(context.Background())
	defer cancelPoll()
	pollerDone := make(chan struct{})
	go func() {
		defer close(pollerDone)
		poller.Run(pollCtx)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := session.Query(`SELECT release_version FROM system.local`).WithContext(r.Context()).Exec(); err != nil {
			logger.Warn("health check failed: cassandra unreachable", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	// livez skips Cassandra so a DB blip doesn't trigger pod restarts via the liveness probe.
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/alertz", auth.RequireAuth(userRepo, base.With("component", "auth"))(http.HandlerFunc(h.ServeAlert)))

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

		// Cancel poller and wait for drain before releasing lease; renewal must outlive that wait or a standby could steal the lease mid-delivery.
		cancelPoll()
		select {
		case <-pollerDone:
		case <-shutdownCtx.Done():
			logger.Warn("poller did not drain within shutdown_grace")
		}
		cancelLease()
		// Join Run before releasing: cancelLease alone doesn't wait for in-flight tryAcquireOrRenew to finish, risking concurrent CAS with stale owner.
		<-leaseDone

		// Released only after drain, so a standby resumes immediately and never races a mid-delivery replica.
		if err := processorLease.Release(shutdownCtx); err != nil {
			logger.Warn("failed to release processor lease on shutdown", "error", err)
		}
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}
}

// mustEnv exits the process if name is unset; used for required config with no safe default.
func mustEnv(logger *slog.Logger, name string) string {
	v := os.Getenv(name)
	if v == "" {
		logger.Error(fmt.Sprintf("%s must be set", name))
		os.Exit(1)
	}
	return v
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
func connectWithRetry(logger *slog.Logger, cfg cassandra.Config, ccfg config.CassandraConfig) (*gocql.Session, error) {
	var session *gocql.Session
	attempt := 0
	operation := func() error {
		attempt++
		s, err := cassandra.Connect(cfg, ccfg.ConnectTimeout.Duration(), ccfg.QueryTimeout.Duration())
		if err != nil {
			logger.Warn("cassandra connection failed, retrying", "attempt", attempt, "max_attempts", ccfg.ConnectMaxAttempts, "error", err)
			return err
		}
		session = s
		return nil
	}

	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = ccfg.ConnectBaseDelay.Duration()
	b := backoff.WithMaxRetries(eb, uint64(ccfg.ConnectMaxAttempts-1))
	if err := backoff.Retry(operation, b); err != nil {
		return nil, err
	}
	return session, nil
}
