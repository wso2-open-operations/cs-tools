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
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/server"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load .env: %v", err)
	}

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	// A database is mandatory for DATA_SOURCE=postgres and skipped entirely
	// for servicenow, where entity traffic goes to the SN integration
	// service instead. With no pool the Postgres-only feature sets
	// (event_publish_failures, sla_clocks, scheduled_task_run) are left
	// unregistered rather than failing startup — see db.NewPoolIfNeeded and
	// server.NewRouter.
	pool, err := db.NewPoolIfNeeded(cfg)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if pool != nil {
		defer pool.Close()
	} else {
		log.Printf("no database pool (DATA_SOURCE=%s): event-publish-failures, sla-clocks, and scheduled-task-run endpoints are disabled", cfg.DataSource)
	}

	addr := ":" + cfg.ServerPort
	srv, closePublishers := server.New(addr, pool, cfg)

	// The outbound GitHub worker: drains github_outbound_queue and pushes
	// change-request activity to the linked issue. Same gate as the webhook --
	// one switch turns the whole integration on or off, so it can never run
	// half-connected.
	githubCtx, stopGithub := context.WithCancel(context.Background())
	defer stopGithub()
	if cfg.HasGithubIntegration() {
		if pool == nil {
			log.Printf("GITHUB_INTEGRATION_ENABLED is set but there is no database pool (DATA_SOURCE=%s): the outbound worker is disabled", cfg.DataSource)
		} else {
			worker := service.NewGithubOutboundWorker(
				repository.NewGithubOutboundRepository(pool),
				service.NewGithubOutboundService(
					github.NewClient(github.Config{BaseURL: cfg.GithubBaseURL, Token: cfg.GithubToken}),
				),
				cfg.GithubOutboundInterval,
			)
			go worker.Run(githubCtx)
			log.Printf("github outbound worker enabled (every %s)", cfg.GithubOutboundInterval)
		}
	}

	// Change-request notices: a background poller over event_outbox, gated on
	// CR_NOTICES_ENABLED. Off by default because ServiceNow still sends these
	// mails — turning it on is a paired change with disabling them there, or
	// every approver is notified twice.
	//
	// Its own producer on its own topic, NOT the case-events one above. Every
	// consumer group reads its whole topic, so sharing would make the case
	// consumer read and discard every change-request record and vice versa —
	// a separate topic is what isolates the two volumes, where a separate
	// consumer group would only isolate the processing.
	crNoticeCtx, stopCRNotices := context.WithCancel(context.Background())
	defer stopCRNotices()
	var crPublisher service.EventPublisherService
	if cfg.CRNoticesEnabled {
		switch {
		case pool == nil:
			log.Printf("CR_NOTICES_ENABLED is set but there is no database pool (DATA_SOURCE=%s): change-request notices are disabled", cfg.DataSource)
		case cfg.EventHubBroker == "" || !cfg.EventPublishingEnabled:
			log.Printf("CR_NOTICES_ENABLED is set but event publishing is not configured (EVENT_HUB_BROKER/EVENT_PUBLISHING_ENABLED): change-request notices are disabled")
		case cfg.CREventHubTopic == "":
			log.Printf("CR_NOTICES_ENABLED is set but CR_EVENT_HUB_TOPIC is empty: change-request notices are disabled")
		default:
			crPublisher = service.NewEventPublisherService(
				eventbus.NewProducer(eventbus.Config{
					Broker:           cfg.EventHubBroker,
					ConnectionString: cfg.EventHubConnectionString,
					Topic:            cfg.CREventHubTopic,
				}),
				service.NewEventPublishFailureService(repository.NewEventPublishFailureRepository(pool)),
			)
			crRepo := repository.NewCRNoticeRepository(pool)
			drainer := service.NewCRNoticeDrainer(
				crRepo,
				service.NewCRNoticeService(crRepo, crPublisher),
				cfg.CRNoticePollInterval,
			)
			go drainer.Run(crNoticeCtx)
			log.Printf("change-request notices enabled: publishing to topic %q every %s", cfg.CREventHubTopic, cfg.CRNoticePollInterval)
		}
	}

	// The health probe listens separately, on its own port, so that only its
	// own route is reachable at the public visibility it is published with —
	// see server.NewHealthServer and .choreo/component.yaml.
	healthSrv := server.NewHealthServer(":"+cfg.HealthPort, pool)

	go func() {
		log.Printf("Customer Entity REST Service started in PORT : %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Health probe listening on PORT : %s", cfg.HealthPort)
		if err := healthSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Fatal, like the main listener: a health endpoint that never
			// came up is worse than one that is down, because the alerting
			// that would have caught it is the thing that is missing.
			log.Fatalf("health server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Health first, so the probe starts failing before in-flight API
	// requests are drained — an orchestrator or alerting system watching it
	// sees this instance leave rotation rather than reporting healthy right
	// up to the moment it stops answering.
	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("health server shutdown failed: %v", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	// Closes both of the router's producers -- the shared event topic and
	// the onboarding one.
	closePublishers()
	// Stop the drainer before closing its producer, so a notice in flight is
	// not handed a writer that has already gone away.
	stopCRNotices()
	if crPublisher != nil {
		crPublisher.Close()
	}
	log.Println("server stopped")
}
