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

// Command server wires Postgres, the allocator and source transforms, then serves the webhook routes.
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

	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sre-alert-ingestion-service/internal/allocator"
	"sre-alert-ingestion-service/internal/config"
	"sre-alert-ingestion-service/internal/outbound/corewake"
	"sre-alert-ingestion-service/internal/outbound/dbfallback"
	"sre-alert-ingestion-service/internal/outbound/snsconfirm"
	"sre-alert-ingestion-service/internal/payloads"
	"sre-alert-ingestion-service/internal/postgres"
	"sre-alert-ingestion-service/internal/sources"
	"sre-alert-ingestion-service/internal/transport/auth"
	"sre-alert-ingestion-service/internal/transport/server"
)

// authCacheTTL is how long a verified credential skips PBKDF2.
const authCacheTTL = 60 * time.Second

// snsConfirmTimeout bounds the SubscribeURL fetch.
const snsConfirmTimeout = 10 * time.Second

// dbFallbackTimeout bounds one post to the DB fallback Chat space.
const dbFallbackTimeout = 10 * time.Second

// dbFallbackDrain is the fallback Chat's own shutdown window, enough for the post in flight, one gap and the closing summary.
const dbFallbackDrain = 5 * time.Second

// dbProbeTimeout bounds GET /dbz's ping, and dbProbeEvery is how long one result is reused.
const (
	dbProbeTimeout = 2 * time.Second
	dbProbeEvery   = time.Second
)

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
	pgCfg, err = postgres.SizePool(pgCfg, cfg.Allocator.WriteConcurrency, cfg.Postgres.MinConns)
	if err != nil {
		logger.Error("invalid postgres pool size", "error", err)
		os.Exit(1)
	}
	logger.Info("postgres pool sized", "max_conns", pgCfg.PoolMaxConns, "min_conns", pgCfg.PoolMinConns)
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
	if envCfg.AuthEnabled {
		users := auth.NewIntegrationUsers(pool, base.With("component", "auth"), auth.UsersConfig{
			QueryTimeout:    cfg.Postgres.AuthTimeout.Duration(),
			RefreshInterval: cfg.Postgres.AuthRefreshInterval.Duration(),
			MaxStale:        cfg.Postgres.AuthMaxStale.Duration(),
			CacheTTL:        authCacheTTL,
		})
		// A failed first load isn't fatal: requests answer 503 until Run's next refresh succeeds.
		if err := users.Refresh(context.Background()); err != nil {
			logger.Error("integration_users not loaded; webhooks get 503 until a refresh succeeds", "error", err)
		}
		usersCtx, stopUsers := context.WithCancel(context.Background())
		defer stopUsers()
		go users.Run(usersCtx)
		authn = users
	}
	switch {
	case envCfg.AuthEnabled && envCfg.AuthAuditOnly:
		authn = auth.NewAudit(authn, base.With("component", "auth"))
		logger.Warn("AUTH_AUDIT_ONLY is set: credentials are checked but nothing is rejected")
	case envCfg.AuthEnabled:
		logger.Info("auth enabled: source webhooks are checked against an in-memory copy of integration_users")
	default:
		logger.Warn("AUTH_ENABLED is not true: source routes are unauthenticated")
		if envCfg.AuthAuditOnly {
			logger.Warn("AUTH_AUDIT_ONLY is set but ignored, since AUTH_ENABLED is not true")
		}
	}

	store := postgres.NewStore(pool, cfg.Store.QueryTimeout.Duration(), cfg.Store.ClaimTimeout.Duration())

	waker := corewake.New(base.With("component", "corewake"), envCfg.WakeURL, envCfg.WakeToken, cfg.Wake.Timeout.Duration())

	// Alerts that fail their last write go to one Chat space; a nil fallback means they are only logged.
	var fallback allocator.Fallback
	after := []func(context.Context){waker.Wait}
	if envCfg.DBFallbackChatURL == "" {
		logger.Warn("DB_FALLBACK_CHAT_WEBHOOK_URL not set; alerts the database cannot store are only logged")
	} else {
		chat, err := dbfallback.New(base.With("component", "dbfallback"), envCfg.DBFallbackChatURL, dbFallbackTimeout)
		if err != nil {
			logger.Error("invalid db fallback chat config", "error", err)
			os.Exit(1)
		}
		fallback = chat
		// A fresh window like the raw payload flush's, so earlier drain steps can't leave it an expired context.
		after = append(after, func(context.Context) {
			ctx, cancel := context.WithTimeout(context.Background(), dbFallbackDrain)
			defer cancel()
			chat.Close(ctx)
		})
	}

	rawPayloads := payloads.New(base.With("component", "payloads"), store, payloads.Config{
		FlushInterval: cfg.Payloads.FlushInterval.Duration(),
		MaxBytes:      cfg.Payloads.MaxBufferBytes,
		FlushTimeout:  cfg.Payloads.FlushTimeout.Duration(),
	})
	go rawPayloads.Run()

	sns := snsconfirm.New(base.With("component", "snsconfirm"), snsConfirmTimeout)

	alloc := allocator.New(base.With("component", "allocator"), store, waker, fallback, allocator.Config{
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
		Logger:          base.With("component", "server"),
		Auth:            authn,
		Pipeline:        server.NewIngestor(registry, alloc, cfg.Server.RequestWait.Duration()).WithSNSConfirmer(sns),
		Rejects:         nil,
		Sources:         registry.Names(),
		MaxBodyBytes:    cfg.Server.MaxBodyBytes,
		PreviewChars:    cfg.Reject.BodyPreviewChars,
		PayloadLogBytes: cfg.Log.PayloadMaxBytes,
		Payloads:        rawPayloads,
		DB:              postgres.NewProbe(pool, dbProbeTimeout, dbProbeEvery),
		ReadTimeout:     cfg.Server.ReadTimeout.Duration(),
		WriteTimeout:    cfg.Server.WriteTimeout.Duration(),
		IdleTimeout:     cfg.Server.IdleTimeout.Duration(),
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
		shutdown(shutdownCtx, logger, srv, httpSrv, alloc, rawPayloads, budget{
			DrainDelay:     cfg.Server.DrainDelay.Duration(),
			RequestWait:    cfg.Server.RequestWait.Duration(),
			AllocatorDrain: cfg.Server.AllocatorDrain.Duration(),
			PayloadDrain:   cfg.Server.PayloadDrain.Duration(),
		}, after...)
	}
}

// connectWithRetry backs off exponentially so a transient startup outage doesn't crash-loop the pod.
func connectWithRetry(logger *slog.Logger, cfg postgres.Config, pcfg config.PostgresConfig, queryTimeout time.Duration) (*pgxpool.Pool, error) {
	attempt := 0
	operation := func() (*pgxpool.Pool, error) {
		attempt++
		p, err := postgres.Connect(cfg, pcfg.ConnectTimeout.Duration(), queryTimeout)
		if err != nil {
			logger.Warn("postgres connection failed, retrying", "attempt", attempt, "max_attempts", pcfg.ConnectMaxAttempts, "error", err)
		}
		return p, err
	}
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = pcfg.ConnectBaseDelay.Duration()
	return backoff.Retry(context.Background(), operation, backoff.WithBackOff(eb), backoff.WithMaxTries(uint(pcfg.ConnectMaxAttempts)))
}
