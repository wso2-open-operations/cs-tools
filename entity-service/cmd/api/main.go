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
	// WithSystemIdentity: same reasoning as slaEngineCtx/crNoticeCtx below -- the
	// outbound repo is a plain pool today, but stamping it now means migrating it
	// to Scoped later cannot silently fail every tick with ErrNoCallerIdentity.
	githubCtx, stopGithub := context.WithCancel(repository.WithSystemIdentity(context.Background()))
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

	// CSM-native SLA engine recompute worker: removed. It used to rewrite
	// every active source='CSM' "sla" row's elapsed percentage/breach status
	// on a timer (migration 000088) purely so GET /sla-status/
	// POST /task-slas/search would show a fresh number -- a continuous
	// Postgres write with no bearing on alerting. The sla_live view
	// (migration 0204) computes the same number live, at read time, instead.

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
	// WithSystemIdentity: same reasoning as slaEngineCtx above -- this
	// drainer runs on its own process-startup context, never an HTTP
	// request, and CRNoticeRepository's writes are Scoped-wrapped now too.
	crNoticeCtx, stopCRNotices := context.WithCancel(repository.WithSystemIdentity(context.Background()))
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
			crRepo := repository.NewCRNoticeRepository(repository.NewScoped(pool), server.CRVisibilityFromConfig(cfg))
			drainer := service.NewCRNoticeDrainer(
				crRepo,
				service.NewCRNoticeService(crRepo, crPublisher),
				cfg.CRNoticePollInterval,
			)
			go drainer.Run(crNoticeCtx)
			log.Printf("change-request notices enabled: publishing to topic %q every %s", cfg.CREventHubTopic, cfg.CRNoticePollInterval)
		}
	}

	// Outage emails: the internal-stakeholder notification and the SRE
	// outage communication, decided every OUTAGE_NOTICE_POLL_INTERVAL and
	// published on their own topic for csm-notification-service to send --
	// seconds after the change, as ServiceNow's record-triggered flows are,
	// instead of on csm-scheduled-tasks' tick.
	//
	// No switch of its own: an email whose recipient list is empty is never
	// swept (see OutageNoticeDrainer), so OUTAGE_NOTIFICATION_RECIPIENTS and
	// OUTAGE_COMMUNICATION_RECIPIENTS are what turn each one on.
	if pool != nil && cfg.DataSource != config.DataSourceServiceNow {
		switch {
		case len(cfg.OutageNotificationRecipients) == 0 && len(cfg.OutageCommunicationRecipients) == 0:
			log.Printf("outage emails off: OUTAGE_NOTIFICATION_RECIPIENTS and OUTAGE_COMMUNICATION_RECIPIENTS are both empty")
		case cfg.EventHubBroker == "" || !cfg.EventPublishingEnabled:
			log.Printf("outage emails off: event publishing is not configured (EVENT_HUB_BROKER/EVENT_PUBLISHING_ENABLED)")
		default:
			outageNoticeCtx, stopOutageNotices := context.WithCancel(repository.WithSystemIdentity(context.Background()))
			defer stopOutageNotices()
			// The context below carries the system identity, which ResolveScope
			// returns before ever consulting client ids, so none are configured.
			outageAccess := service.NewAccessService(repository.NewAccessRepository(pool), service.AccessClientConfig{})
			drainer := &service.OutageNoticeDrainer{
				Notifications: service.NewOutageNotificationService(
					repository.NewOutageNotificationRepository(pool), outageAccess),
				Communications: service.NewOutageCommunicationService(
					repository.NewOutageCommunicationRepository(pool), outageAccess),
				Publisher: service.NewEventPublisherService(
					eventbus.NewProducer(eventbus.Config{
						Broker:           cfg.EventHubBroker,
						ConnectionString: cfg.EventHubConnectionString,
						Topic:            cfg.OutageEventHubTopic,
					}),
					service.NewEventPublishFailureService(repository.NewEventPublishFailureRepository(pool)),
				),
				NotificationRecipients:  cfg.OutageNotificationRecipients,
				CommunicationRecipients: cfg.OutageCommunicationRecipients,
				// Woken by migration 0186's NOTIFY on every outage change;
				// the interval is only the fallback poll.
				Listener: repository.NewOutageChangeListener(pool),
				Interval: cfg.OutageNoticePollInterval,
			}
			go drainer.Run(outageNoticeCtx)
			log.Printf("outage emails enabled: publishing to topic %q on each outage change (fallback poll %s; internal notification: %d recipients, outage communication: %d recipients)",
				cfg.OutageEventHubTopic, cfg.OutageNoticePollInterval,
				len(cfg.OutageNotificationRecipients), len(cfg.OutageCommunicationRecipients))
		}
	}

	// Cloud status: the record-triggered path. Started whenever the scope is
	// configured, because it is only useful when there is a scope to decide
	// against -- and harmless without one, since HandleOutages returns early.
	//
	// Not gated on the delivery side's CLOUD_STATUS_ENABLED: nothing leaves
	// the estate until csm-scheduled-tasks posts it, and that is where the
	// double-fire guard belongs.
	//
	// It IS gated on its own CLOUD_STATUS_DRAINER_ENABLED, off by default,
	// because this drainer rewrites cloud_monitor.status -- a column
	// csm-sync-service also writes while its one-time bulk migration is
	// still running. Clearing CLOUD_STATUS_SERVICE_IDS would stop the
	// drainer but take the sweep endpoint and the dashboard reads with it,
	// so the write needs a switch that does not.
	if cfg.CloudStatusDrainerEnabled && cfg.DataSource != config.DataSourceServiceNow &&
		len(cfg.CloudStatusServiceIDs) > 0 {
		cloudStatusCtx, stopCloudStatus := context.WithCancel(context.Background())
		defer stopCloudStatus()
		cloudStatusRepo := repository.NewCloudStatusRepository(pool)
		cloudStatusDrainer := service.NewCloudStatusDrainer(
			cloudStatusRepo,
			service.NewCloudStatusService(cloudStatusRepo, cfg.CloudStatusServiceIDs),
			cfg.CloudStatusPollInterval,
		)
		go cloudStatusDrainer.Run(cloudStatusCtx)
		log.Printf("cloud status notices enabled: draining every %s across %d services",
			cfg.CloudStatusPollInterval, len(cfg.CloudStatusServiceIDs))
	}

	// Incident report flows: the record-triggered port of ServiceNow's
	// "Create Incident Report Task" and "Incident Report Generator". Always
	// on wherever there is a database -- like the ServiceNow flows, there is
	// no switch. Running even with DATA_SOURCE=servicenow is deliberate: the
	// 0181 trigger records incident changes whenever the table is written,
	// and a drainer that is off lets them pile up, to be replayed as stale
	// tasks the day it comes on. Writes Postgres only, never ServiceNow, so it
	// needs no caller token and no publisher.
	// WithSystemIdentity: a background process with no viewer, and
	// work_item's RLS insert policy (0147) admits it only as internal.
	incidentReportCtx, stopIncidentReport := context.WithCancel(repository.WithSystemIdentity(context.Background()))
	defer stopIncidentReport()
	var specialOpsPublisher service.EventPublisherService
	if pool != nil {
		// Dual-write creates the workaround problem in the resolve request,
		// in ServiceNow and Postgres (workaround_problem.go), not here.
		incidentReportFlows := service.NewIncidentReportService()
		if cfg.DataSource == config.DataSourcePostgresServiceNowDualWrite {
			incidentReportFlows = service.NewDualWriteIncidentReportService()
		}
		// incident.special_ops_alert: an incident's assignment group changing
		// to a Special Ops team's group (migration 0207 records it) is
		// published on the operations topic for csm-notification-service.
		// Needs that topic; the teams are SPECIALIST_HANDOFF_CONFIG's.
		switch handoffTeams, err := service.ParseSpecialistHandoffConfig(cfg.SpecialistHandoffConfig); {
		case err != nil:
			log.Printf("special ops alerts off: SPECIALIST_HANDOFF_CONFIG is invalid: %v", err)
		case cfg.SREEventHubTopic == "" || cfg.EventHubBroker == "" || !cfg.EventPublishingEnabled:
			log.Printf("special ops alerts off: SRE_EVENT_HUB_TOPIC, EVENT_HUB_BROKER or EVENT_PUBLISHING_ENABLED is not set")
		default:
			specialOpsPublisher = service.NewEventPublisherService(
				eventbus.NewProducer(eventbus.Config{
					Broker:           cfg.EventHubBroker,
					ConnectionString: cfg.EventHubConnectionString,
					Topic:            cfg.SREEventHubTopic,
				}),
				service.NewEventPublishFailureService(repository.NewEventPublishFailureRepository(pool)),
			)
			incidentReportFlows = service.WithSpecialOpsAlerts(incidentReportFlows, specialOpsPublisher, handoffTeams)
			log.Printf("special ops alerts on: incident.special_ops_alert to topic %q", cfg.SREEventHubTopic)
		}
		incidentReportDrainer := service.NewIncidentReportDrainer(
			repository.NewIncidentReportRepository(repository.NewScoped(pool)),
			incidentReportFlows,
			cfg.IncidentReportPollInterval,
			service.IncidentReportMaxAttempts,
		)
		go incidentReportDrainer.Run(incidentReportCtx)
		log.Printf("incident report flows running: draining every %s", cfg.IncidentReportPollInterval)
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
	// Same for the incident report drainer and its special ops alert producer.
	stopIncidentReport()
	if specialOpsPublisher != nil {
		specialOpsPublisher.Close()
	}
	log.Println("server stopped")
}
