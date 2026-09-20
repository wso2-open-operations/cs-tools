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
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// NewRouter builds the dependency graph (repository → service → handler),
// registers all routes, and wraps the mux with the middleware chain:
// CorrelationID → Recovery → Logger → UserIDToken → Timeout. Also returns
// the constructed EventPublisherService (nil if EVENT_HUB_BROKER is unset or
// EVENT_PUBLISHING_ENABLED isn't "true") so the caller (server.New, then
// cmd/api/main.go) can close it gracefully on shutdown.
func NewRouter(db *pgxpool.Pool, cfg *config.Config) (http.Handler, service.EventPublisherService) {
	userRepo := repository.NewUserRepository(db)
	userSvc := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userSvc)

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

	// sla_clocks has no ServiceNow equivalent either, and is gated on the pool
	// for the same reason as event_publish_failures above. Named (not
	// inlined) since NewServiceNowCaseService below also needs it, for its
	// own direct, in-process pause/resume/completion calls (see that
	// service's own applyCaseStateSLAEffects/applyResponseSLAOnComment) —
	// both already treat a nil SLAClockService as "unconfigured, skip",
	// the same posture every other optional-when-no-database dependency in
	// this file has.
	var slaClockService service.SLAClockService
	var slaClockHandler *handler.SLAClockHandler
	if db != nil {
		slaClockRepo := repository.NewSLAClockRepository(db)
		slaClockService = service.NewSLAClockService(slaClockRepo)
		slaClockHandler = handler.NewSLAClockHandler(slaClockService)
	}

	// scheduled_task_run has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/event_publish_failures above. Backs
	// operations/csm-scheduled-tasks; see that component's own CLAUDE.md
	// and this service's CLAUDE.md ("Scheduled task runs").
	var scheduledTaskRunHandler *handler.ScheduledTaskRunHandler
	if db != nil {
		scheduledTaskRunHandler = handler.NewScheduledTaskRunHandler(service.NewScheduledTaskRunService(repository.NewScheduledTaskRunRepository(db)))
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

	accountRepo := repository.NewAccountRepository(db)
	accountHandler := handler.NewAccountHandler(service.NewAccountService(accountRepo))

	var salesforceEventHandler *handler.SalesforceEventHandler
	if db != nil && cfg.DataSource == config.DataSourcePostgres && cfg.SalesEntityConfigured() {
		salesEntityClient := salesentity.New(cfg.SalesEntityBaseURL, salesentity.ClientCredentialsConfig{
			TokenURL:     cfg.SalesEntityTokenURL,
			ClientID:     cfg.SalesEntityClientID,
			ClientSecret: cfg.SalesEntityClientSecret,
			Scopes:       cfg.SalesEntityScopes,
		})
		salesforceEventHandler = handler.NewSalesforceEventHandler(service.NewSalesforceEventService(accountRepo, salesEntityClient))
	}

	var serviceNowIntegrationServiceClient *integrationservice.Client
	if cfg.DataSource == config.DataSourceServiceNow {
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

	projectRepo := repository.NewProjectRepository(db)
	pgProjectSvc := service.NewProjectService(projectRepo)
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

	var projectUpdateHandler *handler.ProjectUpdateHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		projectUpdateHandler = handler.NewProjectUpdateHandler(service.NewServiceNowProjectUpdateService(serviceNowIntegrationServiceClient))
	}

	// referenceDataRepo backs GET /projects/{id}/metadata and GET /metadata's
	// Postgres-mode choice lists (project_type rows, enum labels) -- see
	// ReferenceDataRepository's own doc comment.
	referenceDataRepo := repository.NewReferenceDataRepository(db)

	var projectStatsHandler *handler.ProjectStatsHandler
	var snProjectStatsSvc service.ProjectStatsService
	if cfg.DataSource == config.DataSourceServiceNow {
		snProjectStatsSvc = service.NewServiceNowProjectStatsService(serviceNowIntegrationServiceClient)
		projectStatsHandler = handler.NewProjectStatsHandler(snProjectStatsSvc)
	}

	// GET /projects/{id}/metadata is wired independently of projectStatsHandler
	// above: it's the one ProjectStatsService method with a Postgres-backed
	// implementation, so it's available regardless of cfg.DataSource, while
	// the remaining project-stats routes stay ServiceNow-only. In ServiceNow
	// mode, snProjectStatsSvc already satisfies ProjectMetadataService
	// structurally, so the same client-backed value is reused rather than
	// built twice.
	var projectMetadataSvc service.ProjectMetadataService
	if cfg.DataSource == config.DataSourceServiceNow {
		projectMetadataSvc = snProjectStatsSvc
	} else {
		projectMetadataSvc = service.NewProjectMetadataService(referenceDataRepo)
	}
	projectMetadataHandler := handler.NewProjectMetadataHandler(projectMetadataSvc)

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
	if cfg.DataSource == config.DataSourceServiceNow {
		activeDeploymentSvc = service.NewServiceNowDeploymentService(serviceNowIntegrationServiceClient)
	} else {
		activeDeploymentSvc = service.NewDeploymentService(deploymentRepo)
	}
	deploymentHandler := handler.NewDeploymentHandler(activeDeploymentSvc)

	// KB features are Postgres-only, like event_publish_failures above --
	// db may be nil under DATA_SOURCE=servicenow, and a nil pool panics on
	// first query rather than at construction (CodeRabbit finding). Gate
	// the whole chain on it: nil handlers mean the KB routes below are
	// never registered when there's no database to serve them.
	var kbArticleHandler *handler.KBArticleHandler
	var kbManagerHandler *handler.KBManagerHandler
	var knowledgeBaseHandler *handler.KnowledgeBaseHandler
	if db != nil {
		kbArticleRepo := repository.NewKBArticleRepository(db)
		kbArticleSvc := service.NewKBArticleService(kbArticleRepo)
		kbArticleHandler = handler.NewKBArticleHandler(kbArticleSvc)
		kbManagerRepo := repository.NewKBManagerRepository(db)
		kbManagerSvc := service.NewKBManagerService(kbManagerRepo)
		kbManagerHandler = handler.NewKBManagerHandler(kbManagerSvc)

		knowledgeBaseRepo := repository.NewKnowledgeBaseRepository(db)
		knowledgeBaseSvc := service.NewKnowledgeBaseService(knowledgeBaseRepo)
		knowledgeBaseHandler = handler.NewKnowledgeBaseHandler(knowledgeBaseSvc)
	}

	deployedProductRepo := repository.NewDeployedProductRepository(db)
	var activeDeployedProductSvc service.DeployedProductService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeDeployedProductSvc = service.NewServiceNowDeployedProductService(serviceNowIntegrationServiceClient)
	} else {
		activeDeployedProductSvc = service.NewDeployedProductService(deployedProductRepo)
	}
	deployedProductHandler := handler.NewDeployedProductHandler(activeDeployedProductSvc)

	// Constructed here (rather than down near snUserHandler below) so
	// NewServiceNowCaseService can also take it — see that constructor's
	// own doc comment for what it uses it for (a direct, in-process role
	// lookup backing applyResponseSLAOnComment, not routed through HTTP).
	var snUserService service.SNUserService
	if cfg.DataSource == config.DataSourceServiceNow {
		snUserService = service.NewServiceNowUserService(serviceNowIntegrationServiceClient)
	}

	caseRepo := repository.NewCaseRepository(db)
	pgCaseSvc := service.NewCaseService(caseRepo, userRepo, eventPublisher)
	var activeCaseSvc service.CaseService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeCaseSvc = service.NewServiceNowCaseService(serviceNowIntegrationServiceClient, pgCaseSvc, eventPublisher, slaClockService, snUserService, cfg.SupportEngineerRole, cfg.CustomerRoles)
	} else {
		activeCaseSvc = pgCaseSvc
	}
	caseHandler := handler.NewCaseHandler(activeCaseSvc)

	// customer_call (migration 000072) backs call requests on the Postgres
	// data source, so these routes are registered for both data sources.
	callRequestRepo := repository.NewCallRequestRepository(db)
	var activeCallRequestSvc service.CallRequestService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeCallRequestSvc = service.NewServiceNowCallRequestService(serviceNowIntegrationServiceClient)
	} else {
		activeCallRequestSvc = service.NewCallRequestService(callRequestRepo, userRepo)
	}
	callRequestHandler := handler.NewCallRequestHandler(activeCallRequestSvc)

	var caseGithubIssueHandler *handler.CaseGithubIssueHandler
	if cfg.DataSource == config.DataSourceServiceNow {
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
	if cfg.DataSource == config.DataSourceServiceNow {
		activeChangeRequestSvc = service.NewServiceNowChangeRequestService(serviceNowIntegrationServiceClient)
	} else {
		activeChangeRequestSvc = service.NewChangeRequestService(changeRequestRepo)
	}
	changeRequestHandler := handler.NewChangeRequestHandler(activeChangeRequestSvc)

	timeCardRepo := repository.NewTimeCardRepository(db)
	var activeTimeCardSvc service.TimeCardService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeTimeCardSvc = service.NewServiceNowTimeCardService(serviceNowIntegrationServiceClient)
	} else {
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

	var feedbackHandler *handler.FeedbackHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		feedbackHandler = handler.NewFeedbackHandler(service.NewServiceNowFeedbackService(serviceNowIntegrationServiceClient))
	}

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
	if cfg.DataSource == config.DataSourceServiceNow {
		activeIncidentSvc = service.NewServiceNowIncidentService(serviceNowIntegrationServiceClient, eventPublisher)
	} else {
		activeIncidentSvc = service.NewIncidentService(incidentRepo)
	}
	incidentHandler := handler.NewIncidentHandler(activeIncidentSvc)

	problemRepo := repository.NewProblemRepository(db)
	var activeProblemSvc service.ProblemService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeProblemSvc = service.NewServiceNowProblemService(serviceNowIntegrationServiceClient)
	} else {
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
		activeConversationSvc = service.NewConversationService(conversationRepo)
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
		globalHandler = handler.NewGlobalHandler(service.NewGlobalService(referenceDataRepo))
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
	if cfg.DataSource == config.DataSourceServiceNow {
		activeCommentSvc = service.NewServiceNowCommentService(serviceNowIntegrationServiceClient)
	} else {
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
	if slaClockHandler != nil {
		mux.HandleFunc("POST /cases/{caseId}/sla-clocks", slaClockHandler.RegisterSLAClock)
		mux.HandleFunc("GET /cases/{caseId}/sla-clocks/{clockType}", slaClockHandler.GetSLAClock)
		mux.HandleFunc("PATCH /cases/{caseId}/sla-clocks/{clockType}/tiers/{tier}", slaClockHandler.SetSLAClockTierReached)
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

	if snUserHandler != nil {
		mux.HandleFunc("GET /users/{id}", snUserHandler.GetUser)
		mux.HandleFunc("GET /users/me", snUserHandler.GetMe)
		mux.HandleFunc("PATCH /users/me", snUserHandler.PatchMe)
		mux.HandleFunc("POST /users/search", snUserHandler.SearchUsers)
	} else {
		mux.HandleFunc("GET /users/me", userHandler.GetMe)
		mux.HandleFunc("POST /users/search", userHandler.SearchUsers)
	mux.HandleFunc("POST /users/by-ids", userHandler.GetUsersByIDs)
	}
	if snAccountHandler != nil {
		mux.HandleFunc("GET /accounts/{id}", snAccountHandler.GetAccount)
		mux.HandleFunc("POST /accounts/search", snAccountHandler.SearchAccounts)
	} else {
		mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)
		mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
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
	mux.HandleFunc("POST /projects/{id}/contacts/search", projectContactHandler.SearchProjectContacts)
	mux.HandleFunc("GET /projects/{id}/contacts/{contactId}", projectContactHandler.GetProjectContact)
	if projectUpdateHandler != nil {
		mux.HandleFunc("PATCH /projects/{id}", projectUpdateHandler.UpdateProject)
	}
	mux.HandleFunc("GET /projects/{id}/metadata", projectMetadataHandler.GetProjectMetadata)
	if projectStatsHandler != nil {
		mux.HandleFunc("GET /projects/{id}/stats", projectStatsHandler.GetProjectStats)
		mux.HandleFunc("GET /projects/{id}/cases/stats", projectStatsHandler.GetProjectCaseStats)
		mux.HandleFunc("GET /projects/{id}/conversations/stats", projectStatsHandler.GetProjectConversationStats)
		mux.HandleFunc("GET /projects/{id}/deployments/stats", projectStatsHandler.GetProjectDeploymentStats)
		mux.HandleFunc("GET /projects/{id}/time-cards/stats", projectStatsHandler.GetProjectTimeCardStats)
		mux.HandleFunc("GET /projects/{id}/change-requests/stats", projectStatsHandler.GetProjectChangeRequestStats)
	}
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
	mux.HandleFunc("PATCH /deployed-products/{id}", deployedProductHandler.PatchDeployedProduct)
	mux.HandleFunc("POST /deployed-products/{id}/metrics/search", deployedProductHandler.SearchDeployedProductMetrics)
	mux.HandleFunc("POST /deployed-products/{id}/metrics/usage-counts/search", deployedProductHandler.SearchDeployedProductUsageCounts)
	mux.HandleFunc("GET /cases/{id}", caseHandler.GetCase)
	mux.HandleFunc("PATCH /cases/{id}", caseHandler.PatchCase)
	mux.HandleFunc("POST /cases", caseHandler.CreateCase)
	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	mux.HandleFunc("POST /cases/aggregate", caseHandler.AggregateCases)
	if feedbackHandler != nil {
		mux.HandleFunc("POST /cases/feedback/search", feedbackHandler.SearchFeedback)
		mux.HandleFunc("POST /cases/feedback/aggregate", feedbackHandler.AggregateFeedback)
	}
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/comments/search", caseHandler.SearchCaseComments)
	mux.HandleFunc("POST /cases/{id}/activities/search", caseHandler.SearchCaseActivities)
	mux.HandleFunc("POST /attachments", caseHandler.CreateCaseAttachment)
	mux.HandleFunc("POST /attachments/{id}/confirm", caseHandler.ConfirmCaseAttachment)
	mux.HandleFunc("POST /attachments/search", caseHandler.SearchCaseAttachments)
	mux.HandleFunc("GET /attachments/{id}/content", caseHandler.GetCaseAttachmentContent)
	mux.HandleFunc("GET /attachments/{id}", caseHandler.GetAttachmentByID)
	mux.HandleFunc("PATCH /attachments/{id}", caseHandler.UpdateAttachment)
	mux.HandleFunc("DELETE /attachments/{id}", caseHandler.DeleteCaseAttachment)
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
	// KB routes are only registered when db != nil (see the KB handler
	// construction above) -- matches the eventPublishFailureHandler
	// pattern below: referencing a method on a nil handler here would
	// panic at server startup, before any request ever arrives.
	if kbArticleHandler != nil {
		mux.HandleFunc("POST /kb-articles", kbArticleHandler.CreateKBArticle)
		mux.HandleFunc("GET /kb-articles/{id}", kbArticleHandler.GetKBArticle)
		mux.HandleFunc("POST /kb-articles/search", kbArticleHandler.SearchKBArticles)
		mux.HandleFunc("PATCH /kb-articles/{id}/state", kbArticleHandler.PatchKBArticleState)
		mux.HandleFunc("POST /kb-managers/search", kbManagerHandler.SearchKBManagers)
		mux.HandleFunc("POST /kb-managers", kbManagerHandler.CreateKBManager)
		mux.HandleFunc("DELETE /kb-managers", kbManagerHandler.DeleteKBManager)
		mux.HandleFunc("PATCH /kb-articles/{id}", kbArticleHandler.PatchKBArticleContent)
		mux.HandleFunc("GET /knowledge-bases", knowledgeBaseHandler.ListKnowledgeBases)
		mux.HandleFunc("POST /knowledge-bases", knowledgeBaseHandler.CreateKnowledgeBase)
		mux.HandleFunc("PATCH /knowledge-bases/{id}", knowledgeBaseHandler.UpdateKnowledgeBaseName)
		mux.HandleFunc("PATCH /knowledge-bases/{id}/active", knowledgeBaseHandler.SetKnowledgeBaseActive)
		mux.HandleFunc("DELETE /kb-articles/{id}", kbArticleHandler.DeleteKBArticle)
		mux.HandleFunc("GET /kb-articles/{id}/history", kbArticleHandler.ListKBArticleHistory)
	}

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

	return middleware.CorrelationID(
		middleware.Recovery(
			middleware.Logger(
				middleware.UserIDToken(
					middleware.Timeout(30 * time.Second)(mux),
				),
			),
		),
	), eventPublisher
}
