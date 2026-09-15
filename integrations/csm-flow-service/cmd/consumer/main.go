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

// Command consumer is the flow engine's event consumer: it joins the shared
// event bus, routes each record to the flows that match it (internal/flows),
// and runs them. It is the pragmatic-hand-port equivalent of the design's
// cmd/engine (docs/architecture.md §18) — the durable timer sweeper and admin
// API are separate lifecycles added later.
//
// A small HTTP server runs alongside for /health, so Choreo (and any probe)
// can see the process is up; it carries no business endpoints.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/config"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/flows"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/outbox"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/store"
)

// shutdownGracePeriod bounds the HTTP server's graceful shutdown. The
// consumer's own graceful close (kafka-go's Close) can itself take ~30s during
// a rebalance, so Choreo's termination grace period must exceed that — see
// docs/architecture.md §17.1.
const shutdownGracePeriod = 10 * time.Second

func main() {
	middleware.ConfigureLogger()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("startup: configuration error", "err", err)
		os.Exit(1)
	}

	// Root context canceled on SIGINT/SIGTERM — cancels the consumer loop.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	busCfg := eventbus.Config{
		Broker:           cfg.EventHubBroker,
		ConnectionString: cfg.EventHubConnectionString,
		Topic:            cfg.EventHubTopic,
	}

	// The CSM database: where the records that trigger flows live, and where
	// a flow resolves its recipients from. Required — a flow with no store can
	// resolve nobody, so failing at startup beats discovering it per record.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("startup: database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	csmStore := store.New(pool)

	// Compile-time proof the store still satisfies what flows ask for. Asserted
	// here rather than in internal/flows, which has no other reason to import
	// the store package.
	var _ flows.Recipients = csmStore
	var _ flows.ChangeRequests = csmStore

	// Producer flows use to publish back onto the main topic — a notification
	// request csm-notification-service sends, or (later) timer.fired.
	flowProducer := eventbus.NewProducer(busCfg)
	defer flowProducer.Close()

	// Optional dead-letter producer. When no DLQ topic is configured, a record
	// that exhausts its retries is logged and dropped (onExhausted stays nil).
	var dlqProducer *eventbus.Producer
	var onExhausted eventbus.OnExhausted
	if cfg.DLQTopic != "" {
		dlqCfg := busCfg
		dlqCfg.Topic = cfg.DLQTopic
		dlqProducer = eventbus.NewProducer(dlqCfg)
		defer dlqProducer.Close()
		onExhausted = func(ctx context.Context, rec eventbus.Record, handleErr error) error {
			slog.ErrorContext(ctx, "consumer: dead-lettering record after exhausted retries",
				"partition", rec.Partition, "offset", rec.Offset, "topic", rec.Topic, "handleErr", handleErr)
			return dlqProducer.Publish(ctx, rec.Key, rec.Value)
		}
	}

	registry := flows.NewRegistry(
		flows.Deps{
			Recipients:           csmStore,
			ChangeRequests:       csmStore,
			Producer:             flowProducer,
			EmailDebugRecipients: cfg.EmailDebugRecipients,
		},
		flows.All()...,
	)

	// Wrap each record's handling in a fresh correlation ID so consumed-record
	// logs are traceable end to end, the same way HTTP requests are.
	handle := func(ctx context.Context, rec eventbus.Record) error {
		ctx = middleware.WithCorrelationID(ctx, middleware.NewCorrelationID())
		return registry.Handle(ctx, rec)
	}

	// Drain committed row changes into the same registry the bus feeds. This is
	// what makes a record-triggered port fire: the ServiceNow original reacts to
	// "Change Request Updated", and the native equivalent is a row changing
	// here. Runs alongside the bus consumer, not instead of it -- a flow may
	// react to either source.
	drainer := &outbox.Drainer{
		Claimer:     csmStore,
		Dispatcher:  registry,
		EntityTypes: flows.TriggerEntityTypes(registry.Flows()),
		Interval:    cfg.OutboxInterval,
	}
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		slog.Info("outbox: draining", "entityTypes", drainer.EntityTypes, "interval", drainer.Interval)
		drainer.Run(ctx)
	}()

	consumer := eventbus.NewConsumer(busCfg, cfg.ConsumerGroup)

	done := make(chan struct{})
	go func() {
		defer close(done)
		slog.Info("consumer: starting",
			"topic", cfg.EventHubTopic, "group", cfg.ConsumerGroup,
			"dlq", cfg.DLQTopic, "registeredFlows", len(registry.Flows()))
		consumer.Run(ctx, handle, onExhausted)
	}()

	srv := healthServer(cfg.Port, registry)
	go func() {
		slog.Info("health server: listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("health server: failed", "err", err)
			stop() // treat a health-server failure as a shutdown trigger
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown: signal received, draining")

	// Stop the consumer first (leaves the group cleanly), then the HTTP server.
	consumer.Close()
	<-done
	<-drainDone

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown: health server did not stop cleanly", "err", err)
	}
	slog.Info("shutdown: complete")
}

// healthServer builds the minimal HTTP server: the standard middleware chain
// plus a /health endpoint reporting the registered flow count.
func healthServer(port string, registry *flows.Registry) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"registeredFlows": len(registry.Flows()),
		})
	})

	handler := middleware.CorrelationID(middleware.Logger(middleware.SecurityHeaders(mux)))
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
}
