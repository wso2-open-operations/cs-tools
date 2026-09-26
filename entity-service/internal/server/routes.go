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

package server

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/choreosubscription"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// NewRouter builds the dependency graph (repository → service → handler),
// registers all routes, and wraps the mux with the middleware chain:
// CorrelationID → Recovery → Logger → UserIDToken → Timeout.
//
// It also returns a shutdown function that closes EVERY Kafka producer it
// constructed — the shared-topic publisher and the onboarding-topic one —
// so the caller (server.New, then cmd/api/main.go) releases both. Returning
// one of the publishers instead, as this used to, left the second producer's
// connections open and a buffered project_contact.invited unflushed at exit.
// The function is never nil; with publishing unconfigured it simply has
// nothing to close.
func NewRouter(db *pgxpool.Pool, cfg *config.Config) (http.Handler, func()) {
	userRepo := repository.NewUserRepository(db)
	userSvc := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userSvc)

	// accessSvc resolves the caller's AccessScope from the validated identity
	// auth.Middleware attaches to every request (see AccessService's own doc
	// comment for the full decision table). Constructed once and shared by
	// every Postgres-backed service that scopes its reads by it. It only takes
	// effect on the Postgres data source: in ServiceNow mode the project/case
	// reads go to ServiceNow itself with the forwarded x-user-id-token, so
	// scoping there is ServiceNow's own (snProjectService/snCaseService hold a
	// pgFallback but do not route GetProjectByID/GetCaseByID through it).
	accessSvc := service.NewAccessService(repository.NewAccessRepository(db), cfg.AuthInternalClientIDs)

	var savedFilterViewHandler *handler.SavedFilterViewHandler
	if db != nil {
		savedFilterViewHandler = handler.NewSavedFilterViewHandler(
			service.NewSavedFilterViewService(repository.NewSavedFilterViewRepository(db), userRepo),
		)
	}

	// event_publish_failures, sla_clocks, scheduled_task_run, and
	// alert_incident_mapping have no ServiceNow equivalent. They are
	// Postgres-backed and registered only when a pool is available
	// (db.NewPoolIfNeeded returns nil for DATA_SOURCE=servicenow so local
	// SN-mode startups are not blocked). Gate the whole chain on db != nil:
	// nil handler means the routes below are never registered, and nil
	// service means EventPublisherService records nothing rather than
	// dereferencing a nil pool.
	var eventPublishFailureSvc service.EventPublishFailureService
	var eventPublishFailureHandler *handler.EventPublishFailureHandler
	if db != nil {
		eventPublishFailureSvc = service.NewEventPublishFailureService(repository.NewEventPublishFailureRepository(db))
		eventPublishFailureHandler = handler.NewEventPublishFailureHandler(eventPublishFailureSvc)
	}

	// Project consumption is gated on having a database, NOT on the data
	// source. It used to be Postgres-only, on the reasoning that ServiceNow
	// deployments keep this state on the customer_project record; it now
	// dual-writes both stores, and staging and production run
	// DATA_SOURCE=servicenow, so gating on the data source would have disabled
	// the feature exactly where it is needed. ServiceNow remains the source of
	// truth for status, reached through the Choreo subscription operation.
	//
	// The two halves are configured independently, because they need different
	// things and failing one must not take out the other.
	//
	// Reading and writing the stored state needs a pool. Issuing a licence does
	// not:
	// the sequence reads status from ServiceNow and runs through the Choreo
	// operation, touching Postgres only to mirror state, which is best-effort
	// and skipped entirely when there is no repository. Gating the licence
	// route on the database would take licence downloads out of any deployment
	// that happens not to have one — and the customer portal now issues every
	// licence through this service.
	//
	// Every path that leaves a route unregistered says so at startup. A
	// disabled route is otherwise indistinguishable from a typo in the URL —
	// both are a bare 404 — and the one thing a person debugging that 404
	// cannot discover from the outside is that the service deliberately chose
	// not to register it.
	// Not logged when db is nil: with no database pool configured, stored state
	// cannot be registered. The licence route below does not depend on it.
	var consumptionRepo repository.ProjectConsumptionRepository
	if db != nil {
		consumptionRepo = repository.NewProjectConsumptionRepository(db)
	}

	// Provisioning reaches an upstream that mints Choreo applications for real
	// customers, so an unconfigured or partially-configured operation leaves
	// the route absent rather than registering something that fails — or worse,
	// succeeds — against the wrong environment.
	var choreoClient choreosubscription.Client
	if cfg.ConsumptionOperationBaseURL == "" {
		slog.Info("deployment licence route not registered: PRODUCT_CONSUMPTION_OPERATION_URL is unset",
			"routes", "POST /projects/{id}/deployments/{deploymentId}/license")
	} else {
		client, err := choreosubscription.NewClient(choreosubscription.Config{
			BaseURL: cfg.ConsumptionOperationBaseURL,
			Creds: choreosubscription.ClientCredentialsConfig{
				TokenURL:     cfg.ConsumptionOperationTokenURL,
				ClientID:     cfg.ConsumptionOperationClientID,
				ClientSecret: cfg.ConsumptionOperationClientSecret,
				Scopes:       cfg.ConsumptionOperationScopes,
			},
		})
		if err != nil {
			// The error names the offending field, never a credential value.
			slog.Error("deployment licence route not registered: the product-consumption operation is not configured correctly",
				"routes", "POST /projects/{id}/deployments/{deploymentId}/license", "error", err)
		} else {
			choreoClient = client
		}
	}

	consumptionStateEnabled := consumptionRepo != nil
	licenseProvisioningEnabled := choreoClient != nil

	var projectConsumptionHandler *handler.ProjectConsumptionHandler
	if consumptionStateEnabled || licenseProvisioningEnabled {
		if !consumptionStateEnabled {
			slog.Info("deployment licence route registered without Postgres state",
				"routes", "POST /projects/{id}/deployments/{deploymentId}/license",
				"reason", "ServiceNow remains the source of truth for status; the Postgres mirror is skipped")
		}
		projectConsumptionHandler = handler.NewProjectConsumptionHandler(
			service.NewProjectConsumptionService(
				consumptionRepo,
				choreoClient,
				accessSvc,
				cfg.ConsumptionDualWriteEnabled,
			),
		)
	}

	// EventPublisherService is optional, like every ServiceNow-only
	// dependency below — gated on EventHubBroker rather than cfg.DataSource,
	// since publishing is a distinct concern from which backend serves reads
	// (see config.Config.EventHubBroker's doc comment). Also gated on
	// EventPublishingEnabled, a separate safe-by-default kill switch: Event
	// Hub can be fully configured and this still stays nil until that's
	// explicitly turned on. nil when unset; every caller (snCaseService,
	// snIncidentService) already handles that. eventPublishFailureSvc is
	// nil without a pool; Publish then skips durable recording.
	var eventPublisher service.EventPublisherService
	if cfg.EventHubBroker != "" && cfg.EventPublishingEnabled {
		eventPublisher = service.NewEventPublisherService(
			eventbus.NewProducer(eventbus.Config{
				Broker:           cfg.EventHubBroker,
				ConnectionString: cfg.EventHubConnectionString,
				Topic:            cfg.EventHubTopic,
			}),
			eventPublishFailureSvc,
		)
	}

	// Onboarding events go to their own topic, not the shared one, so a
	// case-event backlog cannot delay an invitation and the onboarding
	// dead-letter queue can be watched separately. Same broker, same
	// credentials, same failure recording -- only the topic differs. nil
	// under exactly the same conditions as eventPublisher above, and the
	// membership ingest already treats a nil publisher as "write the rows,
	// send nothing".
	var projectEventPublisher service.EventPublisherService
	if cfg.EventHubBroker != "" && cfg.EventPublishingEnabled {
		projectEventPublisher = service.NewEventPublisherService(
			eventbus.NewProducer(eventbus.Config{
				Broker:           cfg.EventHubBroker,
				ConnectionString: cfg.EventHubConnectionString,
				Topic:            cfg.ProjectEventHubTopic,
			}),
			eventPublishFailureSvc,
		)
	}

	// sla-status reads the "sla" table directly (ServiceNow's own SLA data,
	// synced in) — no ServiceNow equivalent of its own, gated on the pool for
	// the same reason as event_publish_failures above. Replaces the old
	// sla_clocks table entirely; see domain.SLAStatus's own doc comment.
	var slaStatusHandler *handler.SLAStatusHandler
	if db != nil {
		slaStatusRepo := repository.NewSLAStatusRepository(db)
		slaStatusHandler = handler.NewSLAStatusHandler(service.NewSLAStatusService(slaStatusRepo, accessSvc))
	}

	// scheduled_task_run has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/event_publish_failures above. Backs
	// operations/csm-scheduled-tasks; see that component's own CLAUDE.md
	// and this service's CLAUDE.md ("Scheduled task runs").
	// The GitHub change-request sync needs a pool (the repository mapping and
	// the delivery log are tables) and its own switch. Gated on both, so the
	// webhook endpoint is not registered merely because a database exists --
	// it authenticates by HMAC rather than by bearer token, and an endpoint
	// that mutates change requests should appear only when asked for.
	var githubWebhookHandler *handler.GithubWebhookHandler
	var githubServiceRequestHandler *handler.GithubServiceRequestHandler

	// Assigned inside the GitHub-integration block below and read further down,
	// where activeCaseSvc finally exists, to build the native issue-filing
	// service. Both halves of the sync then share one client and one mapping
	// table.
	var (
		githubSyncRepo repository.GithubSyncRepository
		githubClient   *github.Client
		githubLabelSet service.GithubLabels
	)
	var scheduledTaskRunHandler *handler.ScheduledTaskRunHandler
	if db != nil {
		scheduledTaskRunHandler = handler.NewScheduledTaskRunHandler(service.NewScheduledTaskRunService(repository.NewScheduledTaskRunRepository(db)))
		if cfg.HasGithubIntegration() {
			githubLabels, labelErr := service.NewGithubLabels(service.GithubLabelOverrides{
				TypeIncident:       cfg.GithubLabelTypeIncident,
				TypeServiceRequest: cfg.GithubLabelTypeServiceRequest,
				Class:              cfg.GithubLabelsClass,
				StatusAssigned:     cfg.GithubLabelStatusAssigned,
			})
			if labelErr != nil {
				// A label override that does not parse would leave the sync
				// silently recognising nothing -- the exact failure that took
				// ServiceNow's integration down. Refuse to start instead.
				log.Fatalf("invalid GitHub label configuration: %v", labelErr)
			}
			// The outbound worker is started by cmd/api, which owns process
			// lifetime; routes.go only builds what the HTTP surface needs.
			githubSyncRepo = repository.NewGithubSyncRepository(db)
			githubClient = github.NewClient(github.Config{
				BaseURL: cfg.GithubBaseURL,
				Token:   cfg.GithubToken,
			})
			githubLabelSet = githubLabels
			githubSyncSvc := service.NewGithubSyncServiceWriting(
				githubSyncRepo,
				repository.NewGithubMutationRepository(db),
				githubClient,
				cfg.GithubIntegrationLogin,
				githubLabels,
			)
			githubWebhookHandler = handler.NewGithubWebhookHandler(githubSyncSvc, cfg.GithubWebhookSecret)
			githubServiceRequestHandler = handler.NewGithubServiceRequestHandler(githubSyncSvc, cfg.AuthInternalClientIDs)
		}
	}

	// alert_incident_mapping has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/scheduled_task_run/event_publish_failures above,
	// gated the same way: nil db means nil handler means the routes below are
	// never registered, rather than panicking on a nil pool.
	var alertIncidentMappingHandler *handler.AlertIncidentMappingHandler
	if db != nil {
		alertIncidentMappingRepo := repository.NewAlertIncidentMappingRepository(db)
		alertIncidentMappingHandler = handler.NewAlertIncidentMappingHandler(service.NewAlertIncidentMappingService(alertIncidentMappingRepo))
	}

	// announcement_requests has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/scheduled_task_run/alert_incident_mapping
	// above, gated the same way: nil db means nil handler means the routes
	// below are never registered, rather than panicking on a nil pool.
	// Constructed further below (once activeCaseSvc exists), not here —
	// AutoPublish needs it for its own in-process case-creation fan-out.
	var announcementRequestHandler *handler.AnnouncementRequestHandler

	accountRepo := repository.NewAccountRepository(db)
	accountHandler := handler.NewAccountHandler(service.NewAccountService(accountRepo))

	var salesforceEventHandler *handler.SalesforceEventHandler
	// membershipRegistrationHandler and projectContactSyncHandler both need
	// the very same membership-ingest-enabled SalesforceEventService this
	// block builds, so all three are wired together rather than side by side.
	var membershipIngestSvc service.SalesforceEventService
	var salesEntityClient *salesentity.Client
	if db != nil && cfg.DataSource == config.DataSourcePostgres && cfg.SalesEntityConfigured() {
		salesEntityClient = salesentity.New(cfg.SalesEntityBaseURL, salesentity.ClientCredentialsConfig{
			TokenURL:     cfg.SalesEntityTokenURL,
			ClientID:     cfg.SalesEntityClientID,
			ClientSecret: cfg.SalesEntityClientSecret,
			Scopes:       cfg.SalesEntityScopes,
		})
		if cfg.CSMMigrationSalesforceMembershipIngestEnabled {
			// The membership branch (Project_Contact__c / Contact envelopes)
			// writes user/account_contact/project_contact rows and the
			// DATABASE onboarding step, and publishes project_contact.invited
			// when eventPublisher is configured (nil is a no-op there).
			membershipIngestSvc = service.NewSalesforceEventServiceWithMembershipIngest(
				accountRepo, salesEntityClient, service.MembershipIngest{
					Memberships: repository.NewProjectMembershipRepository(db),
					Steps:       repository.NewOnboardingStepRepository(db),
					SalesEntity: salesEntityClient,
					Publisher:   projectEventPublisher,
				})
			salesforceEventHandler = handler.NewSalesforceEventHandler(membershipIngestSvc)
		} else {
			salesforceEventHandler = handler.NewSalesforceEventHandler(service.NewSalesforceEventService(accountRepo, salesEntityClient))
		}
	}

	// POST /users/me/memberships/register (H-0 of the customer onboarding flow).
	// Postgres-only, and off unless CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED is
	// exactly "true": with the flag off the route is not registered at all, so
	// it 404s and nothing on this path can write to Salesforce. It also needs
	// what it depends on to exist — the SALES_ENTITY_* client for the two
	// PATCHes, and the membership-ingest service to re-ingest each flipped
	// membership — so CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED being off leaves
	// this 404 too, rather than flipping Salesforce with no matching database
	// write.
	var membershipRegistrationHandler *handler.MembershipRegistrationHandler
	if db != nil && cfg.CSMMigrationMembershipRegistrationEnabled && salesEntityClient != nil && membershipIngestSvc != nil {
		membershipRegistrationHandler = handler.NewMembershipRegistrationHandler(service.NewMembershipRegistrationService(
			repository.NewMembershipRegistrationRepository(db),
			salesEntityClient,
			membershipIngestSvc,
			repository.NewOnboardingStepRepository(db),
		))
	}

	// onboarding_step has no ServiceNow equivalent; Postgres-only, like
	// scheduled_task_run above.
	var onboardingStepHandler *handler.OnboardingStepHandler
	if db != nil {
		onboardingStepHandler = handler.NewOnboardingStepHandler(service.NewOnboardingStepService(repository.NewOnboardingStepRepository(db), accessSvc))
	}

	// The portal-driven membership writes. Gated on a pool AND
	// cfg.HasPortalMembershipWrites() -- the flag, the Postgres data source
	// and a complete sales-entity-service connection -- because every one of
	// these writes is half a Postgres transaction and half a Salesforce
	// call. Off by default: nil handler means the four routes below are
	// never registered, so a portal built against them fails loudly with a
	// 404 rather than writing one system and not the other.
	var projectMembershipHandler *handler.ProjectMembershipHandler
	if db != nil && cfg.HasPortalMembershipWrites() && salesEntityClient != nil {
		projectMembershipHandler = handler.NewProjectMembershipHandler(service.NewProjectMembershipWriteService(service.MembershipWriteDeps{
			Memberships: repository.NewProjectMembershipRepository(db),
			Steps:       repository.NewOnboardingStepRepository(db),
			SalesEntity: salesEntityClient,
			Publisher:   projectEventPublisher,
			Failures:    eventPublishFailureSvc,
			Access:      accessSvc,
		}))
	}

	// Also constructed for DataSourcePostgresServiceNowDualWrite: that mode's
	// active services stay Postgres-backed (see the case wiring below), but
	// its best-effort ServiceNow mirror writes still need this client.
	// config.Validate requires the same four credentials for both modes.
	var serviceNowIntegrationServiceClient *integrationservice.Client
	if cfg.DataSource == config.DataSourceServiceNow || cfg.DataSource == config.DataSourcePostgresServiceNowDualWrite {
		serviceNowIntegrationServiceClient = integrationservice.New(cfg.ServiceNowIntegrationServiceBaseURL, integrationservice.ClientCredentialsConfig{
			TokenURL:     cfg.ServiceNowIntegrationServiceTokenURL,
			ClientID:     cfg.ServiceNowIntegrationServiceClientID,
			ClientSecret: cfg.ServiceNowIntegrationServiceClientSecret,
			Scopes:       cfg.ServiceNowIntegrationServiceScopes,
		})
	}

	var snAccountHandler *handler.SNAccountHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snAccountHandler = handler.NewSNAccountHandler(service.NewServiceNowAccountService(serviceNowIntegrationServiceClient))
	}

	accountContactRepo := repository.NewAccountContactRepository(db)
	var activeAccountContactSvc service.AccountContactService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeAccountContactSvc = service.NewServiceNowAccountContactService(serviceNowIntegrationServiceClient)
	} else {
		activeAccountContactSvc = service.NewAccountContactService(accountContactRepo)
	}
	accountContactHandler := handler.NewAccountContactHandler(activeAccountContactSvc)

	var opportunityHandler *handler.OpportunityHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		opportunityHandler = handler.NewOpportunityHandler(service.NewServiceNowOpportunityService(serviceNowIntegrationServiceClient))
	}

	var invoiceHandler *handler.InvoiceHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		invoiceHandler = handler.NewInvoiceHandler(service.NewServiceNowInvoiceService(serviceNowIntegrationServiceClient))
	}

	var projectOpportunityLinkHandler *handler.ProjectOpportunityLinkHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		projectOpportunityLinkHandler = handler.NewProjectOpportunityLinkHandler(service.NewServiceNowProjectOpportunityLinkService(serviceNowIntegrationServiceClient))
	}

	// snWritebackDispatcher is the single shared SNWritebackDispatcher for
	// every DATA_SOURCE=postgres-servicenow-dual-write best-effort mirror
	// write (see SNWritebackDispatcher's own doc comment) -- one dispatcher,
	// one small worker pool, reused by every entity's mirror rather than each
	// constructing its own: project (immediately below), and case,
	// call_request, time_card, comment, change_request, and case tags/watch
	// list (all further below, riding on the case dispatch). nil in every
	// other mode. Originally constructed only inline for the case pilot;
	// hoisted here once a second entity (project) needed the same instance.
	var snWritebackDispatcher *service.SNWritebackDispatcher
	if cfg.DataSource == config.DataSourcePostgresServiceNowDualWrite {
		snWritebackDispatcher = service.NewSNWritebackDispatcher(repository.NewSNWritebackFailureRepository(db))
	}

	projectRepo := repository.NewProjectRepository(db)
	pgProjectSvc := service.NewProjectService(projectRepo, accessSvc)
	var activeProjectSvc service.ProjectService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeProjectSvc = service.NewServiceNowProjectService(serviceNowIntegrationServiceClient, pgProjectSvc)
	} else {
		activeProjectSvc = pgProjectSvc
	}
	projectHandler := handler.NewProjectHandler(activeProjectSvc)

	projectContactRepo := repository.NewProjectContactRepository(db)
	var activeProjectContactSvc service.ProjectContactService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeProjectContactSvc = service.NewServiceNowProjectContactService(serviceNowIntegrationServiceClient)
	} else {
		activeProjectContactSvc = service.NewProjectContactService(projectContactRepo)
	}
	projectContactHandler := handler.NewProjectContactHandler(activeProjectContactSvc)

	// activeProjectUpdateSvc backs PATCH /projects/{id} on every data source
	// -- see pgProjectUpdateService's own doc comment for exactly which
	// fields the Postgres data sources accept (a subset of the ServiceNow
	// contract; unsupported fields are rejected with a ValidationError, not
	// silently dropped). DATA_SOURCE=postgres-servicenow-dual-write also
	// mirrors a successful write to ServiceNow, asynchronously, via
	// snWritebackDispatcher -- plain DATA_SOURCE=postgres never touches
	// ServiceNow at all (snWritebackDispatcher is nil in that mode, so the
	// nil-check inside NewProjectUpdateServiceWithSNWriteback's caller here
	// never fires for it). projectRepo/pgProjectSvc were already constructed
	// above for GetProject/SearchProjects.
	var activeProjectUpdateSvc service.ProjectUpdateService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeProjectUpdateSvc = service.NewServiceNowProjectUpdateService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		snProjectMirrorSvc := service.NewServiceNowProjectUpdateService(serviceNowIntegrationServiceClient)
		activeProjectUpdateSvc = service.NewProjectUpdateServiceWithSNWriteback(projectRepo, userRepo, snWritebackDispatcher, snProjectMirrorSvc)
	default:
		activeProjectUpdateSvc = service.NewProjectUpdateService(projectRepo, userRepo)
	}
	projectUpdateHandler := handler.NewProjectUpdateHandler(activeProjectUpdateSvc)

	// referenceDataRepo backs GET /projects/{id}/metadata and GET /metadata's
	// Postgres-mode choice lists (project_type rows, enum labels) -- see
	// ReferenceDataRepository's own doc comment.
	referenceDataRepo := repository.NewReferenceDataRepository(db)

	// Every project-stats route is available on both data sources. In
	// ServiceNow mode one client-backed value satisfies all three interfaces
	// structurally, so it is built once and shared; in Postgres mode the
	// narrower metadata and case-stats services are built first and composed
	// into the full ProjectStatsService, which delegates those two methods to
	// them rather than reimplementing either.
	//
	// ProjectMetadataService and ProjectCaseStatsService remain separate
	// interfaces, and keep their own handlers, because each was portable to
	// Postgres before the rest of the bundle was.
	var (
		projectMetadataSvc  service.ProjectMetadataService
		projectCaseStatsSvc service.ProjectCaseStatsService
		projectStatsSvc     service.ProjectStatsService
	)
	if cfg.DataSource == config.DataSourceServiceNow {
		snProjectStatsSvc := service.NewServiceNowProjectStatsService(serviceNowIntegrationServiceClient)
		projectMetadataSvc, projectCaseStatsSvc, projectStatsSvc = snProjectStatsSvc, snProjectStatsSvc, snProjectStatsSvc
	} else {
		projectMetadataSvc = service.NewProjectMetadataService(referenceDataRepo)
		// accessSvc is passed in so a by-id stats read is scoped to what the
		// caller may see. Unlike the scoped list endpoints, which fold the
		// scope into their WHERE clause, the project id here comes from the
		// path and needs an explicit check.
		projectCaseStatsSvc = service.NewProjectCaseStatsService(
			repository.NewProjectCaseStatsRepository(db), referenceDataRepo, accessSvc)
		projectStatsSvc = service.NewProjectStatsService(
			repository.NewProjectStatsRepository(db), referenceDataRepo, accessSvc,
			projectMetadataSvc, projectCaseStatsSvc)
	}
	projectMetadataHandler := handler.NewProjectMetadataHandler(projectMetadataSvc)
	projectCaseStatsHandler := handler.NewProjectCaseStatsHandler(projectCaseStatsSvc)
	projectStatsHandler := handler.NewProjectStatsHandler(projectStatsSvc)

	productRepo := repository.NewProductRepository(db)
	productSvc := service.NewProductService(productRepo)
	productHandler := handler.NewProductHandler(productSvc)

	var snProductHandler *handler.SNProductHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snProductHandler = handler.NewSNProductHandler(service.NewServiceNowProductService(serviceNowIntegrationServiceClient))
	}

	var productVersionHandler *handler.ProductVersionHandler
	var snProductVersionHandler *handler.SNProductVersionHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snProductVersionHandler = handler.NewSNProductVersionHandler(service.NewServiceNowProductVersionService(serviceNowIntegrationServiceClient))
	} else {
		productVersionRepo := repository.NewProductVersionRepository(db)
		productVersionSvc := service.NewProductVersionService(productVersionRepo)
		productVersionHandler = handler.NewProductVersionHandler(productVersionSvc)
	}

	deploymentRepo := repository.NewDeploymentRepository(db)
	var activeDeploymentSvc service.DeploymentService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeDeploymentSvc = service.NewServiceNowDeploymentService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateDeployment is ServiceNow-first and synchronous; UpdateDeployment
		// is Postgres-first with an asynchronous ServiceNow mirror -- see
		// deploymentService.createDeploymentSNFirst/UpdateDeployment's own doc
		// comments for the full reasoning (the same CREATE-vs-UPDATE asymmetry
		// as caseService).
		snDeploymentMirrorSvc := service.NewServiceNowDeploymentService(serviceNowIntegrationServiceClient)
		activeDeploymentSvc = service.NewDeploymentServiceWithSNWriteback(deploymentRepo, snWritebackDispatcher, snDeploymentMirrorSvc)
	default:
		activeDeploymentSvc = service.NewDeploymentService(deploymentRepo)
	}
	deploymentHandler := handler.NewDeploymentHandler(activeDeploymentSvc)

	deployedProductRepo := repository.NewDeployedProductRepository(db)
	var activeDeployedProductSvc service.DeployedProductService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeDeployedProductSvc = service.NewServiceNowDeployedProductService(serviceNowIntegrationServiceClient, activeDeploymentSvc, activeProjectSvc)
	} else {
		activeDeployedProductSvc = service.NewDeployedProductService(deployedProductRepo)
	}
	deployedProductHandler := handler.NewDeployedProductHandler(activeDeployedProductSvc)

	// Constructed here (rather than down near snUserHandler below) so
	// NewServiceNowCaseService can also take it — see that constructor's
	// own doc comment for what it uses it for (a direct, in-process role
	// lookup backing applyResponseSLAOnComment, not routed through HTTP).
	// Also constructed for DataSourcePostgresServiceNowDualWrite, for the same
	// reason serviceNowIntegrationServiceClient above is: the case pilot's
	// SN-mirror snCaseService instance below needs it too.
	var snUserService service.SNUserService
	if cfg.DataSource == config.DataSourceServiceNow || cfg.DataSource == config.DataSourcePostgresServiceNowDualWrite {
		snUserService = service.NewServiceNowUserService(serviceNowIntegrationServiceClient)
	}

	caseRepo := repository.NewCaseRepository(db)
	var activeCaseSvc service.CaseService
	// caseAttachmentOverrideSvc, when non-nil, is the CaseService case
	// attachment routes (registered further below) use INSTEAD of
	// activeCaseSvc -- see its assignment in the DataSourcePostgresServiceNowDualWrite
	// case for why. nil in every other mode: attachments follow activeCaseSvc
	// exactly as before this override existed.
	var caseAttachmentOverrideSvc service.CaseService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		pgCaseFallbackSvc := service.NewCaseService(caseRepo, userRepo, eventPublisher, accessSvc)
		activeCaseSvc = service.NewServiceNowCaseService(serviceNowIntegrationServiceClient, pgCaseFallbackSvc, eventPublisher, snUserService, cfg.CustomerRoles)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Pilot: case CREATE, and UPDATE's WorkState field only.
		//
		// CREATE is ServiceNow-first and synchronous — see
		// caseService.createCaseSNFirst's own doc comment for the full
		// reasoning (a Postgres-first async create could leave a permanent
		// orphan: a Postgres row with no ServiceNow counterpart). This is
		// also what finally makes case creation work on Postgres in this
		// mode at all: CaseRepository.CreateCase's own doc comment explains
		// why Postgres can't generate work_item.number/wso2_id itself (no
		// sequence was ever added); CreateCaseFromServiceNow sidesteps that
		// by using the identity ServiceNow already generated, rather than
		// answering the still-unresolved question of what a Postgres-native
		// case number would even look like. The plain (non-fallback)
		// CreateCase path above (DataSourceServiceNow's pgCaseFallbackSvc,
		// and DataSourcePostgres/default below) is UNCHANGED and still
		// deliberately non-functional — this only unblocks the fallback
		// mode's own path.
		//
		// UPDATE mirrors State/Severity/WorkState, asynchronously, after
		// Postgres — see caseService.UpdateCase's own doc comment for
		// exactly what this mirrors and why. State/Severity joined the
		// mirror later than WorkState did, once patchCaseFields
		// (sn_case_service.go) existed: a bare PATCH with none of
		// snCaseService.UpdateCase's own read-before-write behavior (that
		// method still does a live GetCaseByID before PATCHing State/
		// Severity, which this mode must never do — patchCaseFields is a
		// separate, additional method precisely so UpdateCase itself stays
		// unchanged for live DataSource=servicenow traffic).
		//
		// CreateCaseComment mirrors the comment's content, asynchronously,
		// after Postgres — see that method's own doc comment. It uses
		// CreateBareCaseComment (sn_case_service.go), not the full
		// CreateCaseComment, for the same reason patchCaseFields exists:
		// Postgres already decided the real outcome, so ServiceNow's own
		// state-transition/event side effects must not re-run.
		//
		// snCaseMirrorSvc is a full snCaseService, exactly as constructed
		// for DataSourceServiceNow above, but it is never made the active
		// CaseService — reads always stay on Postgres in this mode. It
		// serves four purposes: CreateCase calls its CreateCase directly and
		// synchronously; UpdateCase dispatches to its patchCaseFields (via
		// the snFieldPatcher interface) through snWritebackDispatcher, asynchronously;
		// CreateCaseComment dispatches to its CreateBareCaseComment (via the
		// snCommentMirror interface) through snWritebackDispatcher, asynchronously;
		// and it is caseAttachmentOverrideSvc below, for case attachments
		// specifically.
		snCaseMirrorSvc := service.NewServiceNowCaseService(serviceNowIntegrationServiceClient, nil, nil, snUserService, cfg.CustomerRoles)
		activeCaseSvc = service.NewCaseServiceWithSNWriteback(caseRepo, userRepo, eventPublisher, accessSvc, snWritebackDispatcher, snCaseMirrorSvc)
		// Case ATTACHMENTS are ServiceNow-only in this mode, permanently —
		// unlike case metadata (CREATE/UPDATE above), not a pilot scope
		// decision but a hard requirement: the sftpgo-backed Postgres
		// attachment implementation (case_attachment table,
		// CaseRepository.CreateCaseAttachment et al. — real, working SQL,
		// unlike the old CreateCase bug) is not production-ready for the
		// Oct 4 go-live, so attachment routes must never reach it while this
		// mode is active, regardless of how case metadata itself is wired.
		// snCaseMirrorSvc (above) is reused as-is: every one of its
		// attachment methods (CreateCaseAttachment/SearchCaseAttachments/
		// GetCaseAttachmentContent/DeleteCaseAttachment/GetAttachmentByID/
		// UpdateAttachment) already converts the platform case UUID to a
		// ServiceNow sys_id via uuidToSysid internally, and that round-trips
		// correctly because CreateCaseFromServiceNow (createCaseSNFirst)
		// stores id = sysidToUUID(the real sys_id) for every case created in
		// this mode — the same identity convention DataSource=servicenow
		// itself relies on. ConfirmCaseAttachment correctly 503s here too,
		// same as it already does in plain DataSource=servicenow — a
		// pre-existing, expected gap (Postgres-only concept: ServiceNow's
		// /attachments API has no pending/in-progress upload state to
		// confirm), not something this override introduces.
		caseAttachmentOverrideSvc = snCaseMirrorSvc
	default:
		activeCaseSvc = service.NewCaseService(caseRepo, userRepo, eventPublisher, accessSvc)
	}
	caseHandler := handler.NewCaseHandler(activeCaseSvc)
	if db != nil {
		announcementRequestHandler = handler.NewAnnouncementRequestHandler(
			service.NewAnnouncementRequestService(repository.NewAnnouncementRequestRepository(db), activeCaseSvc, accessSvc),
		)
	}
	// activeAttachmentSvc backs the case-attachment routes registered below
	// (POST/GET/PATCH/DELETE /attachments...) — see caseAttachmentOverrideSvc's
	// own doc comment above for when and why it differs from activeCaseSvc.
	activeAttachmentSvc := activeCaseSvc
	if caseAttachmentOverrideSvc != nil {
		activeAttachmentSvc = caseAttachmentOverrideSvc
	}
	attachmentHandler := handler.NewCaseHandler(activeAttachmentSvc)

	// customer_call (migration 000072) backs call requests on the Postgres
	// data source, so these routes are registered for both data sources.
	callRequestRepo := repository.NewCallRequestRepository(db)
	var activeCallRequestSvc service.CallRequestService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeCallRequestSvc = service.NewServiceNowCallRequestService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateCallRequest mirrors to ServiceNow, asynchronously, after
		// Postgres -- see callRequestService's own doc comment for why
		// UpdateCallRequest does not (Postgres-first CREATE means
		// customer_call.id has no ServiceNow counterpart to target).
		snCallRequestMirrorSvc := service.NewServiceNowCallRequestService(serviceNowIntegrationServiceClient)
		activeCallRequestSvc = service.NewCallRequestServiceWithSNWriteback(callRequestRepo, userRepo, snWritebackDispatcher, snCallRequestMirrorSvc)
	default:
		activeCallRequestSvc = service.NewCallRequestService(callRequestRepo, userRepo)
	}
	callRequestHandler := handler.NewCallRequestHandler(activeCallRequestSvc)

	// The native implementation when the GitHub integration is enabled,
	// otherwise the ServiceNow proxy. Both are kept: cutover is per account,
	// and an account still on ServiceNow must keep filing issues the old way.
	var caseGithubIssueHandler *handler.CaseGithubIssueHandler
	switch {
	case githubClient != nil:
		// Native: files the issue against GitHub and writes the issue number
		// onto the case, which is what opens the outbound gate for it.
		caseGithubIssueHandler = handler.NewCaseGithubIssueHandler(
			service.NewCaseGithubIssueService(githubClient, githubSyncRepo, activeCaseSvc, githubLabelSet))
	case cfg.DataSource == config.DataSourceServiceNow:
		caseGithubIssueHandler = handler.NewCaseGithubIssueHandler(service.NewServiceNowCaseGithubIssueService(serviceNowIntegrationServiceClient, activeCaseSvc))
	}

	// case_escalation/case_escalation_notification_list (migration 000053)
	// now back SearchEscalations on Postgres for real -- CreateEscalation
	// still isn't supported there (see EscalationRepository's own doc
	// comment for why), so this supersedes an earlier unconditional
	// unavailableCaseEscalationService stand-in that predated the schema.
	escalationRepo := repository.NewEscalationRepository(db)
	var activeEscalationSvc service.EscalationService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeEscalationSvc = service.NewServiceNowEscalationService(serviceNowIntegrationServiceClient)
	} else {
		activeEscalationSvc = service.NewEscalationService(escalationRepo)
	}
	escalationHandler := handler.NewEscalationHandler(activeEscalationSvc)
	caseEscalationHandler := handler.NewCaseEscalationHandler(service.NewCaseEscalationService(activeEscalationSvc, activeCaseSvc))

	changeRequestRepo := repository.NewChangeRequestRepository(db)
	var activeChangeRequestSvc service.ChangeRequestService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeChangeRequestSvc = service.NewServiceNowChangeRequestService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Pilot extension: change request CREATE (ServiceNow-first,
		// synchronous -- see changeRequestService.createChangeRequestSNFirst's
		// own doc comment), PatchChangeRequest's best-effort asynchronous
		// ServiceNow mirror write, and DecideChangeRequestApproval's
		// best-effort asynchronous mirror write (see those methods' own doc
		// comments). Reads (GetChangeRequest, GetChangeRequestApprovals)
		// stay on Postgres in this mode; snChangeRequestMirrorSvc's
		// CreateChangeRequest/PatchChangeRequest/DecideChangeRequestApproval
		// are the only methods of it this mode ever calls.
		snChangeRequestMirrorSvc := service.NewServiceNowChangeRequestService(serviceNowIntegrationServiceClient)
		activeChangeRequestSvc = service.NewChangeRequestServiceWithSNWriteback(changeRequestRepo, userRepo, snChangeRequestMirrorSvc, snWritebackDispatcher)
	default:
		activeChangeRequestSvc = service.NewChangeRequestService(changeRequestRepo, userRepo)
	}
	changeRequestHandler := handler.NewChangeRequestHandler(activeChangeRequestSvc)

	timeCardRepo := repository.NewTimeCardRepository(db)
	var activeTimeCardSvc service.TimeCardService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeTimeCardSvc = service.NewServiceNowTimeCardService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateTimeCard mirrors to ServiceNow, asynchronously, after
		// Postgres -- see timeCardService's own doc comment for why
		// Update/DeleteTimeCard do not (Postgres-first CREATE means
		// time_card.id has no ServiceNow counterpart to target).
		snTimeCardMirrorSvc := service.NewServiceNowTimeCardService(serviceNowIntegrationServiceClient)
		activeTimeCardSvc = service.NewTimeCardServiceWithSNWriteback(timeCardRepo, userRepo, snWritebackDispatcher, snTimeCardMirrorSvc)
	default:
		activeTimeCardSvc = service.NewTimeCardService(timeCardRepo, userRepo)
	}
	timeCardHandler := handler.NewTimeCardHandler(activeTimeCardSvc)

	// sr_category/catalog_item/catalog_item_category/catalog_variable/
	// sr_category_routing_rule (migrations 000067-000071) back the service
	// request catalog on the Postgres data source, so these routes are
	// registered for both data sources.
	catalogRepo := repository.NewCatalogRepository(db)
	var activeCatalogSvc service.CatalogService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeCatalogSvc = service.NewServiceNowCatalogService(serviceNowIntegrationServiceClient)
	} else {
		activeCatalogSvc = service.NewCatalogService(catalogRepo)
	}
	catalogHandler := handler.NewCatalogHandler(activeCatalogSvc)

	// Case feedback (CSAT submissions) is a ServiceNow-only entity -- no
	// feedback table exists anywhere in migrations/ -- but the routes are
	// registered for both data sources, same as tasks above: with no handler
	// the mux answers a silent, undocumented 404, while the OpenAPI spec
	// documents a 503 ErrorResponse for these paths. The Postgres stand-in
	// supplies that 503.
	var activeFeedbackSvc service.FeedbackService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeFeedbackSvc = service.NewServiceNowFeedbackService(serviceNowIntegrationServiceClient)
	} else {
		activeFeedbackSvc = service.NewUnavailableFeedbackService()
	}
	feedbackHandler := handler.NewFeedbackHandler(activeFeedbackSvc)

	productVulnerabilityRepo := repository.NewProductVulnerabilityRepository(db)
	var activeProductVulnerabilitySvc service.ProductVulnerabilityService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeProductVulnerabilitySvc = service.NewServiceNowProductVulnerabilityService(serviceNowIntegrationServiceClient)
	} else {
		activeProductVulnerabilitySvc = service.NewProductVulnerabilityService(productVulnerabilityRepo)
	}
	productVulnerabilityHandler := handler.NewProductVulnerabilityHandler(activeProductVulnerabilitySvc)

	// incident/problem/incident_task/conversation (migrations 000057-000060,
	// 000066) -- see incident_repo.go/problem_repo.go's own doc comments for
	// what's implemented (reads) vs. still ServiceUnavailableError (writes
	// needing work_item.number generation or undiscoverable business rules).
	incidentRepo := repository.NewIncidentRepository(db)
	var activeIncidentSvc service.IncidentService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeIncidentSvc = service.NewServiceNowIncidentService(serviceNowIntegrationServiceClient, eventPublisher)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Pilot extension: incident CREATE only, same ServiceNow-first,
		// synchronous shape as the case pilot above -- see
		// incidentService.createIncidentSNFirst's own doc comment. Reads
		// stay on Postgres in this mode; snIncidentMirrorSvc's CreateIncident
		// is the only method of it this mode ever calls.
		//
		// eventPublisher is passed through here (unlike snCaseMirrorSvc's nil
		// publisher/access args above, which are inert for case because
		// caseService's own CreateCase response building doesn't need them).
		// The mirror is built with publisher=nil deliberately (unlike a
		// plain DataSourceServiceNow instance) -- its own automatic publish
		// fires right after the ServiceNow POST returns, before the
		// Postgres insert this mode's reads depend on has even been
		// attempted, which is exactly the premature-event bug CodeRabbit
		// flagged on PR #1922. incident.created is instead published by
		// createIncidentSNFirst itself, after that Postgres insert
		// succeeds -- see NewIncidentServiceWithSNMirror's own doc comment
		// and publishIncidentCreatedEvent's.
		//
		// snWritebackDispatcher (the single shared instance constructed once
		// above) is reused as-is for incident UPDATE's async ServiceNow
		// mirror -- a *SNWritebackDispatcher is just a fixed background
		// worker pool plus one sn_writeback_failures repository, nothing
		// case-specific about it, so a second instance would only mean a
		// second, redundant worker pool.
		snIncidentMirrorSvc := service.NewServiceNowIncidentService(serviceNowIntegrationServiceClient, nil)
		activeIncidentSvc = service.NewIncidentServiceWithSNMirror(incidentRepo, userRepo, snIncidentMirrorSvc, eventPublisher, snWritebackDispatcher)
	default:
		activeIncidentSvc = service.NewIncidentService(incidentRepo)
	}
	incidentHandler := handler.NewIncidentHandler(activeIncidentSvc)

	problemRepo := repository.NewProblemRepository(db)
	var activeProblemSvc service.ProblemService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeProblemSvc = service.NewServiceNowProblemService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Pilot extension: problem CREATE only, same ServiceNow-first,
		// synchronous shape as the case/incident/change-request pilots
		// above -- see problemService.createProblemSNFirst's own doc
		// comment. Reads stay on Postgres in this mode;
		// snProblemMirrorSvc's CreateProblem is the only method of it this
		// mode ever calls.
		snProblemMirrorSvc := service.NewServiceNowProblemService(serviceNowIntegrationServiceClient)
		activeProblemSvc = service.NewProblemServiceWithSNMirror(problemRepo, snProblemMirrorSvc)
	default:
		activeProblemSvc = service.NewProblemService(problemRepo)
	}
	problemHandler := handler.NewProblemHandler(activeProblemSvc)

	var alertHandler *handler.AlertHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		alertHandler = handler.NewAlertHandler(service.NewServiceNowAlertService(serviceNowIntegrationServiceClient))
	}

	var smartAlertHandler *handler.SmartAlertHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		smartAlertHandler = handler.NewSmartAlertHandler(service.NewServiceNowSmartAlertService(serviceNowIntegrationServiceClient))
	}

	incidentTaskRepo := repository.NewIncidentTaskRepository(db)
	var activeIncidentTaskSvc service.IncidentTaskService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeIncidentTaskSvc = service.NewServiceNowIncidentTaskService(serviceNowIntegrationServiceClient)
	} else {
		activeIncidentTaskSvc = service.NewIncidentTaskService(incidentTaskRepo)
	}
	incidentTaskHandler := handler.NewIncidentTaskHandler(activeIncidentTaskSvc)

	conversationRepo := repository.NewConversationRepository(db)
	var activeConversationSvc service.ConversationService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeConversationSvc = service.NewServiceNowConversationService(serviceNowIntegrationServiceClient)
	} else {
		activeConversationSvc = service.NewConversationService(conversationRepo, accessSvc)
	}
	conversationHandler := handler.NewConversationHandler(activeConversationSvc)

	var outageHandler *handler.OutageHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		outageHandler = handler.NewOutageHandler(service.NewServiceNowOutageService(serviceNowIntegrationServiceClient))
	}
	// globalHandler is wired for both data sources now: GetSystemMetadata has
	// a Postgres-backed implementation (globalService, reusing
	// referenceDataRepo above); GlobalSearch does not yet, and returns a
	// clear error in Postgres mode rather than 404 -- see globalService's own
	// doc comment.
	var globalHandler *handler.GlobalHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		globalHandler = handler.NewGlobalHandler(service.NewServiceNowGlobalService(serviceNowIntegrationServiceClient))
	} else {
		globalHandler = handler.NewGlobalHandler(service.NewGlobalService(
			referenceDataRepo,
			repository.NewGlobalSearchRepository(db),
			accessSvc,
		))
	}

	// instance/usage tracking tables (migration 000054) -- see
	// instance_repo.go's own doc comment for the caveats around resolving an
	// instance's project/deployment/deployed-product references on this data
	// source.
	instanceRepo := repository.NewInstanceRepository(db)
	var activeInstanceSvc service.InstanceService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeInstanceSvc = service.NewServiceNowInstanceService(serviceNowIntegrationServiceClient)
	} else {
		activeInstanceSvc = service.NewInstanceService(instanceRepo)
	}
	instanceHandler := handler.NewInstanceHandler(activeInstanceSvc)

	itServiceRepo := repository.NewITServiceRepository(db)
	var activeITServiceSvc service.ITServiceService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeITServiceSvc = service.NewServiceNowITServiceService(serviceNowIntegrationServiceClient)
	} else {
		activeITServiceSvc = service.NewITServiceService(itServiceRepo)
	}
	itServiceHandler := handler.NewITServiceHandler(activeITServiceSvc)

	serviceOfferingRepo := repository.NewServiceOfferingRepository(db)
	var activeServiceOfferingSvc service.ServiceOfferingService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeServiceOfferingSvc = service.NewServiceNowServiceOfferingService(serviceNowIntegrationServiceClient)
	} else {
		activeServiceOfferingSvc = service.NewServiceOfferingService(serviceOfferingRepo)
	}
	serviceOfferingHandler := handler.NewServiceOfferingHandler(activeServiceOfferingSvc)

	groupRepo := repository.NewGroupRepository(db)
	var activeGroupSvc service.GroupService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeGroupSvc = service.NewServiceNowGroupService(serviceNowIntegrationServiceClient)
	} else {
		activeGroupSvc = service.NewGroupService(groupRepo)
	}
	groupHandler := handler.NewGroupHandler(activeGroupSvc)

	var configurationItemHandler *handler.ConfigurationItemHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		configurationItemHandler = handler.NewConfigurationItemHandler(service.NewServiceNowConfigurationItemService(serviceNowIntegrationServiceClient))
	}

	commentRepo := repository.NewCommentRepository(db)
	var activeCommentSvc service.CommentService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeCommentSvc = service.NewServiceNowCommentService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateComment mirrors to ServiceNow, asynchronously, after
		// Postgres -- see commentService's own doc comment. This is separate
		// from case's own comment mirror (CreateCaseComment/CreateBareCaseComment),
		// which backs the case-scoped comment routes, not these generic ones.
		snCommentMirrorSvc := service.NewServiceNowCommentService(serviceNowIntegrationServiceClient)
		activeCommentSvc = service.NewCommentServiceWithSNWriteback(commentRepo, userRepo, snWritebackDispatcher, snCommentMirrorSvc)
	default:
		activeCommentSvc = service.NewCommentService(commentRepo, userRepo)
	}
	commentHandler := handler.NewCommentHandler(activeCommentSvc)

	taskSlaRepo := repository.NewTaskSlaRepository(db)
	var activeTaskSlaSvc service.TaskSlaService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeTaskSlaSvc = service.NewServiceNowTaskSlaService(serviceNowIntegrationServiceClient)
	} else {
		activeTaskSlaSvc = service.NewTaskSlaService(taskSlaRepo)
	}
	taskSlaHandler := handler.NewTaskSlaHandler(activeTaskSlaSvc)

	var snUserHandler *handler.SNUserHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snUserHandler = handler.NewSNUserHandler(snUserService)
	}

	// Tasks are a ServiceNow-only entity, but the routes are registered for both
	// data sources: with no handler the mux answers 404, while the OpenAPI spec
	// documents a 503 ErrorResponse for these paths. The Postgres stand-in
	// supplies that 503 -- same shape as caseService's ServiceNow-only tag
	// operations.
	var activeTaskSvc service.TaskService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeTaskSvc = service.NewServiceNowTaskService(serviceNowIntegrationServiceClient)
	} else {
		activeTaskSvc = service.NewUnavailableTaskService()
	}
	taskHandler := handler.NewTaskHandler(activeTaskSvc)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.HealthCheck)

	if salesforceEventHandler != nil {
		mux.HandleFunc("POST /salesforce/events", salesforceEventHandler.HandleEvent)
	}
	if onboardingStepHandler != nil {
		mux.HandleFunc("PUT /onboarding-steps/{membershipSfId}/{step}", onboardingStepHandler.UpsertOnboardingStep)
		mux.HandleFunc("GET /onboarding-steps/{membershipSfId}", onboardingStepHandler.GetOnboardingSteps)
		mux.HandleFunc("POST /onboarding-steps/search", onboardingStepHandler.SearchOnboardingSteps)
	}

	// event_publish_failures, sla_clocks, scheduled_task_run and
	// alert_incident_mapping are not data-source specific, but all four are
	// Postgres-backed, and the database is optional when
	// DATA_SOURCE=servicenow — so unlike the role catalogue and team
	// registry below, these are registered only when a pool exists. With no
	// database they 404 rather than panicking on a nil pool.
	if eventPublishFailureHandler != nil {
		mux.HandleFunc("POST /event-publish-failures", eventPublishFailureHandler.CreateEventPublishFailure)
		mux.HandleFunc("POST /event-publish-failures/search", eventPublishFailureHandler.SearchEventPublishFailures)
		mux.HandleFunc("POST /event-publish-failures/{id}/resolve", eventPublishFailureHandler.ResolveEventPublishFailure)
	}
	if slaStatusHandler != nil {
		mux.HandleFunc("GET /sla-status", slaStatusHandler.SearchActiveSLAStatuses)
	}
	if githubWebhookHandler != nil {
		mux.HandleFunc("POST /webhooks/github", githubWebhookHandler.Handle)
		mux.HandleFunc("POST /github/service-requests", githubServiceRequestHandler.Create)
	}

	if scheduledTaskRunHandler != nil {
		mux.HandleFunc("POST /scheduled-tasks/attempts", scheduledTaskRunHandler.AttemptScheduledTaskRun)
		mux.HandleFunc("PATCH /scheduled-tasks/attempts/{id}", scheduledTaskRunHandler.UpdateScheduledTaskRunAttempt)
		mux.HandleFunc("GET /scheduled-tasks/attempts", scheduledTaskRunHandler.ListScheduledTaskRuns)
		mux.HandleFunc("DELETE /scheduled-tasks/attempts", scheduledTaskRunHandler.DeleteScheduledTaskRuns)
	}
	if alertIncidentMappingHandler != nil {
		mux.HandleFunc("POST /alert-incident-mappings", alertIncidentMappingHandler.CreateAlertIncidentMapping)
		mux.HandleFunc("POST /alert-incident-mappings/lookup", alertIncidentMappingHandler.LookupAlertIncidentMappings)
	}
	if announcementRequestHandler != nil {
		mux.HandleFunc("POST /announcement-requests", announcementRequestHandler.CreateAnnouncementRequest)
		mux.HandleFunc("GET /announcement-requests/{id}", announcementRequestHandler.GetAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/search", announcementRequestHandler.SearchAnnouncementRequests)
		mux.HandleFunc("PATCH /announcement-requests/{id}", announcementRequestHandler.UpdateAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/dry-run", announcementRequestHandler.RecordAnnouncementRequestDryRun)
		mux.HandleFunc("POST /announcement-requests/{id}/submit", announcementRequestHandler.SubmitAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/approve", announcementRequestHandler.ApproveAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/schedule", announcementRequestHandler.ScheduleAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/auto-publish", announcementRequestHandler.AutoPublishAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/publish", announcementRequestHandler.PublishAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/updates", announcementRequestHandler.CreateAnnouncementRequestUpdate)
		mux.HandleFunc("GET /announcement-requests/{id}/updates", announcementRequestHandler.ListAnnouncementRequestUpdates)
		mux.HandleFunc("POST /announcement-requests/{id}/deliveries", announcementRequestHandler.RecordAnnouncementRequestDeliveries)
		mux.HandleFunc("GET /announcement-requests/{id}/deliveries", announcementRequestHandler.ListAnnouncementRequestDeliveries)
	}
	if savedFilterViewHandler != nil {
		mux.HandleFunc("GET /users/me/saved-filter-views", savedFilterViewHandler.List)
		mux.HandleFunc("PATCH /users/me/saved-filter-views", savedFilterViewHandler.Save)
		mux.HandleFunc("DELETE /users/me/saved-filter-views", savedFilterViewHandler.Delete)
		mux.HandleFunc("POST /users/me/saved-filter-views/reorder", savedFilterViewHandler.Reorder)
	}

	if membershipRegistrationHandler != nil {
		mux.HandleFunc("POST /users/me/memberships/register", membershipRegistrationHandler.RegisterInvitedMemberships)
	}

	if snUserHandler != nil {
		mux.HandleFunc("GET /users/{id}", snUserHandler.GetUser)
		mux.HandleFunc("GET /users/me", snUserHandler.GetMe)
		mux.HandleFunc("PATCH /users/me", snUserHandler.PatchMe)
		mux.HandleFunc("POST /users/search", snUserHandler.SearchUsers)
	} else {
		mux.HandleFunc("GET /users/{id}", userHandler.GetUser)
		mux.HandleFunc("GET /users/me", userHandler.GetMe)
		mux.HandleFunc("POST /users/search", userHandler.SearchUsers)
		mux.HandleFunc("POST /users", userHandler.CreateUser)
	}
	if snAccountHandler != nil {
		mux.HandleFunc("GET /accounts/{id}", snAccountHandler.GetAccount)
		mux.HandleFunc("POST /accounts/search", snAccountHandler.SearchAccounts)
	} else {
		mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)
		mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
		mux.HandleFunc("PATCH /accounts/{id}", accountHandler.PatchAccountTeams)
	}
	mux.HandleFunc("POST /accounts/{id}/contacts/search", accountContactHandler.SearchAccountContacts)
	if opportunityHandler != nil {
		mux.HandleFunc("POST /opportunities/search", opportunityHandler.SearchOpportunities)
		mux.HandleFunc("GET /opportunities/{id}", opportunityHandler.GetOpportunity)
	}
	if invoiceHandler != nil {
		mux.HandleFunc("POST /invoices/search", invoiceHandler.SearchInvoices)
		mux.HandleFunc("GET /invoices/{id}", invoiceHandler.GetInvoice)
	}
	if projectOpportunityLinkHandler != nil {
		mux.HandleFunc("POST /project-opportunity-links/search", projectOpportunityLinkHandler.SearchProjectOpportunityLinks)
	}
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	// Registered independently: the stored-state routes read and write
	// Postgres, the licence route does not need it at all.
	if consumptionStateEnabled {
		mux.HandleFunc("GET /projects/{id}/consumption", projectConsumptionHandler.GetProjectConsumption)
		mux.HandleFunc("PATCH /projects/{id}/consumption", projectConsumptionHandler.UpdateProjectConsumption)
	}
	if licenseProvisioningEnabled {
		mux.HandleFunc("POST /projects/{id}/deployments/{deploymentId}/license", projectConsumptionHandler.GetDeploymentLicense)
	}
	mux.HandleFunc("POST /projects/{id}/contacts/search", projectContactHandler.SearchProjectContacts)
	mux.HandleFunc("GET /projects/{id}/contacts/{contactId}", projectContactHandler.GetProjectContact)
	if projectMembershipHandler != nil {
		// Beside the search and get above, in the same namespace. {email}
		// keys a membership; {contactId} on the GET above is a user id, and
		// the two never collide because the methods differ.
		mux.HandleFunc("POST /projects/{id}/contacts", projectMembershipHandler.InviteProjectContact)
		mux.HandleFunc("PATCH /projects/{id}/contacts/{email}", projectMembershipHandler.UpdateProjectContactRoles)
		mux.HandleFunc("DELETE /projects/{id}/contacts/{email}", projectMembershipHandler.DeactivateProjectContact)
		mux.HandleFunc("POST /projects/{id}/contacts/{email}/resend-invitation", projectMembershipHandler.ResendProjectContactInvitation)
	}
	mux.HandleFunc("PATCH /projects/{id}", projectUpdateHandler.UpdateProject)
	mux.HandleFunc("GET /projects/{id}/metadata", projectMetadataHandler.GetProjectMetadata)
	mux.HandleFunc("GET /projects/{id}/cases/stats", projectCaseStatsHandler.GetProjectCaseStats)
	mux.HandleFunc("GET /projects/{id}/stats", projectStatsHandler.GetProjectStats)
	mux.HandleFunc("GET /projects/{id}/conversations/stats", projectStatsHandler.GetProjectConversationStats)
	mux.HandleFunc("GET /projects/{id}/deployments/stats", projectStatsHandler.GetProjectDeploymentStats)
	mux.HandleFunc("GET /projects/{id}/time-cards/stats", projectStatsHandler.GetProjectTimeCardStats)
	mux.HandleFunc("GET /projects/{id}/change-requests/stats", projectStatsHandler.GetProjectChangeRequestStats)
	if snProductHandler != nil {
		mux.HandleFunc("POST /products/search", snProductHandler.SearchProducts)
	} else {
		mux.HandleFunc("POST /products/search", productHandler.SearchProducts)
	}
	if snProductVersionHandler != nil {
		mux.HandleFunc("POST /products/{id}/versions/search", snProductVersionHandler.SearchProductVersions)
	} else if productVersionHandler != nil {
		mux.HandleFunc("POST /products/{id}/versions/search", productVersionHandler.SearchProductVersions)
	}
	mux.HandleFunc("POST /deployments", deploymentHandler.CreateDeployment)
	mux.HandleFunc("POST /deployments/search", deploymentHandler.SearchDeployments)
	mux.HandleFunc("PATCH /deployments/{id}", deploymentHandler.PatchDeployment)
	mux.HandleFunc("POST /deployed-products", deployedProductHandler.CreateDeployedProduct)
	mux.HandleFunc("POST /deployed-products/search", deployedProductHandler.SearchDeployedProducts)
	mux.HandleFunc("POST /deployed-products/projects/search", deployedProductHandler.SearchProjectsByProductVersion)
	mux.HandleFunc("PATCH /deployed-products/{id}", deployedProductHandler.PatchDeployedProduct)
	mux.HandleFunc("POST /deployed-products/{id}/metrics/search", deployedProductHandler.SearchDeployedProductMetrics)
	mux.HandleFunc("POST /deployed-products/{id}/metrics/usage-counts/search", deployedProductHandler.SearchDeployedProductUsageCounts)
	mux.HandleFunc("GET /cases/{id}", caseHandler.GetCase)
	mux.HandleFunc("PATCH /cases/{id}", caseHandler.PatchCase)
	mux.HandleFunc("POST /cases", caseHandler.CreateCase)
	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	mux.HandleFunc("POST /cases/aggregate", caseHandler.AggregateCases)
	mux.HandleFunc("POST /cases/feedback/search", feedbackHandler.SearchFeedback)
	mux.HandleFunc("POST /cases/feedback/aggregate", feedbackHandler.AggregateFeedback)
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/comments/search", caseHandler.SearchCaseComments)
	mux.HandleFunc("POST /cases/{id}/activities/search", caseHandler.SearchCaseActivities)
	mux.HandleFunc("POST /attachments", attachmentHandler.CreateCaseAttachment)
	mux.HandleFunc("POST /attachments/{id}/confirm", attachmentHandler.ConfirmCaseAttachment)
	mux.HandleFunc("POST /attachments/search", attachmentHandler.SearchCaseAttachments)
	mux.HandleFunc("GET /attachments/{id}/content", attachmentHandler.GetCaseAttachmentContent)
	mux.HandleFunc("GET /attachments/{id}", attachmentHandler.GetAttachmentByID)
	mux.HandleFunc("PATCH /attachments/{id}", attachmentHandler.UpdateAttachment)
	mux.HandleFunc("DELETE /attachments/{id}", attachmentHandler.DeleteCaseAttachment)
	mux.HandleFunc("GET /cases/{id}/feedback", caseHandler.GetCaseFeedback)
	mux.HandleFunc("POST /cases/{id}/feedback", caseHandler.SubmitCaseFeedback)
	mux.HandleFunc("POST /cases/{id}/tags", caseHandler.AddCaseTag)
	mux.HandleFunc("DELETE /cases/{id}/tags/{tagId}", caseHandler.RemoveCaseTag)
	mux.HandleFunc("POST /tags/search", caseHandler.SearchTags)
	// Deprecated: the query-parameter form of tag search, kept for one release
	// so callers can be rolled out independently of this service. Remove it
	// (and CaseHandler.SearchTagsQuery) once they are all on the POST.
	//nolint:staticcheck // SA1019: intentional one-release compatibility route; remove with the handler.
	mux.HandleFunc("GET /tags/search", caseHandler.SearchTagsQuery)

	mux.HandleFunc("POST /call-requests", callRequestHandler.CreateCallRequest)
	mux.HandleFunc("POST /call-requests/search", callRequestHandler.SearchCallRequests)
	mux.HandleFunc("POST /call-requests/search-all", callRequestHandler.SearchAllCallRequests)
	mux.HandleFunc("PATCH /call-requests/{id}", callRequestHandler.PatchCallRequest)

	if caseGithubIssueHandler != nil {
		mux.HandleFunc("POST /cases/{id}/github-issues", caseGithubIssueHandler.CreateCaseGithubIssue)
	}

	mux.HandleFunc("GET /cases/{id}/escalations", caseEscalationHandler.SearchCaseEscalations)
	mux.HandleFunc("POST /cases/{id}/escalations", caseEscalationHandler.CreateCaseEscalation)

	mux.HandleFunc("POST /change-requests", changeRequestHandler.CreateChangeRequest)
	mux.HandleFunc("POST /change-requests/search", changeRequestHandler.SearchChangeRequests)
	mux.HandleFunc("POST /change-requests/aggregate", changeRequestHandler.AggregateChangeRequests)
	mux.HandleFunc("GET /change-requests/{id}", changeRequestHandler.GetChangeRequest)
	mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
	mux.HandleFunc("GET /change-requests/{id}/approvals", changeRequestHandler.GetChangeRequestApprovals)
	mux.HandleFunc("POST /change-requests/{id}/approvals/decision", changeRequestHandler.DecideChangeRequestApproval)

	mux.HandleFunc("POST /time-cards/search", timeCardHandler.SearchTimeCards)
	mux.HandleFunc("POST /time-cards", timeCardHandler.CreateTimeCard)
	mux.HandleFunc("PATCH /time-cards/{id}", timeCardHandler.UpdateTimeCard)
	mux.HandleFunc("POST /cases/time-cards/search", timeCardHandler.SearchCaseTimeCards)
	mux.HandleFunc("DELETE /time-cards/{id}", timeCardHandler.DeleteTimeCard)

	mux.HandleFunc("POST /catalogs/search", catalogHandler.SearchCatalogs)
	mux.HandleFunc("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", catalogHandler.GetCatalogItemVariables)

	mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
	mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)
	mux.HandleFunc("GET /products/vulnerabilities/meta", productVulnerabilityHandler.GetVulnerabilityMeta)
	mux.HandleFunc("POST /products/vulnerabilities/sync", productVulnerabilityHandler.SyncProductVulnerabilities)

	mux.HandleFunc("POST /services/search", itServiceHandler.SearchITServices)

	mux.HandleFunc("POST /service-offerings/search", serviceOfferingHandler.SearchServiceOfferings)

	mux.HandleFunc("POST /groups/search", groupHandler.SearchGroups)

	if configurationItemHandler != nil {
		mux.HandleFunc("POST /configuration-items/search", configurationItemHandler.SearchConfigurationItems)
	}

	mux.HandleFunc("POST /comments", commentHandler.CreateComment)
	mux.HandleFunc("POST /comments/search", commentHandler.SearchComments)
	mux.HandleFunc("PATCH /comments/{id}", commentHandler.UpdateComment)
	mux.HandleFunc("DELETE /comments/{id}", commentHandler.DeleteComment)
	mux.HandleFunc("GET /comments/{id}/history", commentHandler.GetCommentEditHistory)

	mux.HandleFunc("GET /slas/{id}", taskSlaHandler.GetTaskSla)
	mux.HandleFunc("POST /slas/search", taskSlaHandler.SearchTaskSlas)

	// Registered unconditionally; the non-ServiceNow data source is served by
	// service.NewUnavailableTaskService, which answers 503 (see above).
	mux.HandleFunc("POST /cases/{id}/tasks/search", taskHandler.SearchCaseTasks)
	mux.HandleFunc("POST /tasks/search", taskHandler.SearchTasks)
	mux.HandleFunc("GET /tasks/{id}", taskHandler.GetTask)
	mux.HandleFunc("POST /cases/{id}/tasks", taskHandler.CreateCaseTask)
	mux.HandleFunc("PATCH /tasks/{id}", taskHandler.UpdateTask)

	mux.HandleFunc("GET /incidents/{id}", incidentHandler.GetIncident)
	mux.HandleFunc("PATCH /incidents/{id}", incidentHandler.PatchIncident)
	mux.HandleFunc("POST /incidents", incidentHandler.CreateIncident)
	mux.HandleFunc("POST /incidents/search", incidentHandler.SearchIncidents)
	mux.HandleFunc("POST /incidents/aggregate", incidentHandler.AggregateIncidents)
	mux.HandleFunc("POST /incidents/{id}/activities/search", incidentHandler.SearchIncidentActivities)
	mux.HandleFunc("POST /incidents/{id}/specialist-handoffs", incidentHandler.HandOffIncidentToSpecialist)

	if outageHandler != nil {
		mux.HandleFunc("POST /outages", outageHandler.CreateOutage)
		mux.HandleFunc("POST /outages/search", outageHandler.SearchOutages)
		mux.HandleFunc("GET /outages/metadata", outageHandler.GetOutageMetadata)
		mux.HandleFunc("GET /outages/{id}", outageHandler.GetOutage)
		mux.HandleFunc("PATCH /outages/{id}", outageHandler.PatchOutage)
		mux.HandleFunc("POST /outages/{id}/communications", outageHandler.AddOutageCommunication)
		mux.HandleFunc("POST /outages/{id}/communications/search", outageHandler.SearchOutageCommunications)
	}

	mux.HandleFunc("POST /problems", problemHandler.CreateProblem)
	mux.HandleFunc("POST /problems/search", problemHandler.SearchProblems)
	mux.HandleFunc("POST /problems/aggregate", problemHandler.AggregateProblems)
	mux.HandleFunc("GET /problems/{id}", problemHandler.GetProblem)
	mux.HandleFunc("PATCH /problems/{id}", problemHandler.PatchProblem)

	mux.HandleFunc("POST /incident-tasks/search", incidentTaskHandler.SearchIncidentTasks)
	mux.HandleFunc("POST /incident-tasks/aggregate", incidentTaskHandler.AggregateIncidentTasks)
	mux.HandleFunc("GET /incident-tasks/{id}", incidentTaskHandler.GetIncidentTask)

	if alertHandler != nil {
		mux.HandleFunc("GET /alerts/{id}", alertHandler.GetAlert)
	}

	if smartAlertHandler != nil {
		mux.HandleFunc("GET /smart-alerts/{id}", smartAlertHandler.GetSmartAlert)
	}

	mux.HandleFunc("POST /conversations/search", conversationHandler.SearchConversations)
	mux.HandleFunc("GET /conversations/{id}", conversationHandler.GetConversation)
	mux.HandleFunc("POST /conversations", conversationHandler.CreateConversation)
	mux.HandleFunc("PATCH /conversations/{id}", conversationHandler.UpdateConversation)

	mux.HandleFunc("GET /metadata", globalHandler.GetSystemMetadata)
	mux.HandleFunc("POST /search", globalHandler.GlobalSearch)

	mux.HandleFunc("POST /escalations/search", escalationHandler.SearchEscalations)
	mux.HandleFunc("POST /escalations", escalationHandler.CreateEscalation)

	mux.HandleFunc("POST /instances/search", instanceHandler.SearchInstances)
	mux.HandleFunc("POST /instances/metrics/search", instanceHandler.SearchInstanceMetrics)
	mux.HandleFunc("POST /instances/usages/search", instanceHandler.SearchInstanceUsage)
	mux.HandleFunc("POST /instances/metrics/stats/search", instanceHandler.SearchInstanceMetricsStats)
	mux.HandleFunc("POST /instances/usages/stats/search", instanceHandler.SearchInstanceUsageStats)

	// Token validation always runs -- there is no config flag to disable it.
	// The JWKS must load right here at startup: a wrong URL panics now instead
	// of silently rejecting every token later (same fail-fast posture as
	// apps/csm-portal/backend).
	tokenValidator, err := auth.NewValidator(context.Background(), auth.Config{
		Issuer:             cfg.AuthIssuer,
		JWKSURL:            cfg.AuthJWKSURL,
		UserTokenAudiences: cfg.AuthUserTokenAudiences,
		ClockSkew:          cfg.AuthClockSkew,
	})
	if err != nil {
		panic("auth: could not initialise token validation: " + err.Error())
	}

	// Both producers are closed together: they are constructed under the
	// same conditions and neither caller has any reason to outlive the
	// other.
	closePublishers := func() {
		if eventPublisher != nil {
			eventPublisher.Close()
		}
		if projectEventPublisher != nil {
			projectEventPublisher.Close()
		}
	}

	// PLG Customer Success Portal. Every repository, service, handler and route
	// it needs is in plg_routes.go — this is the only part of entity-service's
	// own wiring the merge touches.
	//
	// Gated on db != nil like every other Postgres-backed route above:
	// NewPoolIfNeeded returns a nil pool for DATA_SOURCE=servicenow, and PLG is
	// Postgres-only by construction — its tables do not exist in that mode.
	// Registering anyway would start cleanly and then nil-pointer on the first
	// query of every PLG request. Not registering means those paths 404, which
	// is the truthful answer where PLG has no data to serve.
	if db != nil {
		registerPLGRoutes(mux, db)
	}

	return middleware.CorrelationID(
		middleware.Recovery(
			middleware.Logger(
				middleware.UserIDToken(
					auth.Middleware(tokenValidator)(
						middleware.Timeout(30 * time.Second)(mux),
					),
				),
			),
		),
	), closePublishers
}
