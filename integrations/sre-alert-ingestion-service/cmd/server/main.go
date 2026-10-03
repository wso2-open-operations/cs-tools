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

// Command server wires Cassandra, the allocator and source transforms, then serves the webhook routes.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v5/pgxpool"

	"sre-alert-ingestion-service/internal/allocator"
	"sre-alert-ingestion-service/internal/config"
	"sre-alert-ingestion-service/internal/outbound/corewake"
	"sre-alert-ingestion-service/internal/outbound/snsconfirm"
	"sre-alert-ingestion-service/internal/postgres"
	"sre-alert-ingestion-service/internal/transport/auth"
	"sre-alert-ingestion-service/internal/transport/server"
	"sre-alert-ingestion-service/internal/sources"
)

// authCacheTTL is how long a verified credential is reused, capped at the row's expires_at.
const authCacheTTL = 60 * time.Second

// snsConfirmTimeout bounds the SubscribeURL fetch.
const snsConfirmTimeout = 10 * time.Second

func main() {
	base := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("app", "sre-alert-ingestion-service")
	logger := base.With("component", "main")

	cfg, err := config.Load("")
	if err != nil {
		logger.Error("failed to load deployment config", "error", err)
		os.Exit(1)
	}
	envCfg, err := config.LoadEnv()
	if err != nil {
		logger.Error("failed to read environment", "error", err)
		os.Exit(1)
	}
	registry, err := sources.New()
	if err != nil {
		logger.Error("failed to load source config", "error", err)
		os.Exit(1)
	}

	pgCfg, err := postgres.ConfigFromEnv()
	if err != nil {
		logger.Error("failed to read postgres config", "error", err)
		os.Exit(1)
	}
	// The driver's own timeout must not cut the longer claim_timeout short.
	pool, err := connectWithRetry(logger, pgCfg, cfg.Postgres,
		max(cfg.Store.QueryTimeout.Duration(), cfg.Store.ClaimTimeout.Duration()))
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.LegacyAuthSection {
		logger.Warn("config.toml has an [auth] section, which is no longer read; set AUTH_ENABLED (and AUTH_AUDIT_ONLY) in the environment instead")
	}
	// After the pool: AUTH_ENABLED checks webhooks against alerts-core's integration_users.
	var authn auth.Authenticator = auth.None{}
	switch {
	case envCfg.AuthEnabled && envCfg.AuthAuditOnly:
		authn = auth.NewAudit(auth.NewIntegrationUsers(pool, cfg.Store.QueryTimeout.Duration(), authCacheTTL),
			base.With("component", "auth"))
		logger.Warn("AUTH_AUDIT_ONLY is set: credentials are checked but nothing is rejected")
	case envCfg.AuthEnabled:
		authn = auth.NewIntegrationUsers(pool, cfg.Store.QueryTimeout.Duration(), authCacheTTL)
		logger.Info("auth enabled: source webhooks are checked against integration_users")
	default:
		logger.Warn("AUTH_ENABLED is not true: source routes are unauthenticated")
		if envCfg.AuthAuditOnly {
			logger.Warn("AUTH_AUDIT_ONLY is set but ignored, since AUTH_ENABLED is not true")
		}
	}

	// alert_seq is a native sequence created by schema.sql, so there's nothing to seed here.
	store, err := postgres.NewStore(pool, cfg.Store.QueryTimeout.Duration(), cfg.Store.ClaimTimeout.Duration())
	if err != nil {
		logger.Error("failed to open postgres store", "error", err)
		os.Exit(1)
	}

	waker := corewake.New(base.With("component", "corewake"), envCfg.WakeURL, envCfg.WakeUsername, envCfg.WakeSecret, cfg.Wake.Timeout.Duration())

	sns := snsconfirm.New(base.With("component", "snsconfirm"), snsConfirmTimeout)

	alloc := allocator.New(base.With("component", "allocator"), store, nil, waker, allocator.Config{
		QueueSize:        cfg.Allocator.QueueSize,
		QueueMaxBytes:    cfg.Allocator.QueueMaxBytes,
		MaxBatch:         cfg.Allocator.MaxBatch,
		WriteConcurrency: cfg.Allocator.WriteConcurrency,
		InsertAttempts:   cfg.Store.InsertAttempts,
		InsertBaseDelay:  cfg.Store.InsertBaseDelay.Duration(),
		QueryTimeout:     cfg.Store.QueryTimeout.Duration(),
		WriteDeadline:    cfg.Store.WriteDeadline.Duration(),
	})

	srv := server.New(server.Options{
		Logger:       base.With("component", "server"),
		Auth:         authn,
		Pipeline:     server.NewIngestor(registry, alloc, cfg.Server.RequestWait.Duration()).WithSNSConfirmer(sns),
		Rejects:      nil,
		Sources:      registry.Names(),
		MaxBodyBytes: cfg.Server.MaxBodyBytes,
		PreviewChars: cfg.Reject.BodyPreviewChars,
		ReadTimeout:  cfg.Server.ReadTimeout.Duration(),
		WriteTimeout: cfg.Server.WriteTimeout.Duration(),
		IdleTimeout:  cfg.Server.IdleTimeout.Duration(),
	})
	httpSrv := srv.HTTPServer(":" + envCfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "port", envCfg.Port)
		serveErr <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining")
		// Restores default signal handling so a second Ctrl-C force-kills.
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownGrace.Duration())
		defer cancel()
		shutdown(shutdownCtx, logger, srv, httpSrv, alloc, budget{
			DrainDelay:     cfg.Server.DrainDelay.Duration(),
			RequestWait:    cfg.Server.RequestWait.Duration(),
			AllocatorDrain: cfg.Server.AllocatorDrain.Duration(),
		}, waker.Wait)
	}
}

// connectWithRetry backs off exponentially so a transient startup outage doesn't crash-loop the pod.
func connectWithRetry(logger *slog.Logger, cfg postgres.Config, pcfg config.PostgresConfig, queryTimeout time.Duration) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	attempt := 0
	operation := func() error {
		attempt++
		p, err := postgres.Connect(cfg, pcfg.ConnectTimeout.Duration(), queryTimeout)
		if err != nil {
			logger.Warn("postgres connection failed, retrying", "attempt", attempt, "max_attempts", pcfg.ConnectMaxAttempts, "error", err)
			return err
		}
		pool = p
		return nil
	}
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = pcfg.ConnectBaseDelay.Duration()
	if err := backoff.Retry(operation, backoff.WithMaxRetries(eb, uint64(pcfg.ConnectMaxAttempts-1))); err != nil {
		return nil, err
	}
	return pool, nil
}
