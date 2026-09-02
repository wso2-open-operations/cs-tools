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

	// event_publish_failures has no ServiceNow equivalent — always backed by
	// Postgres regardless of cfg.DataSource, same as the pool itself (see
	// db.NewPool's call site in cmd/api/main.go).
	eventPublishFailureRepo := repository.NewEventPublishFailureRepository(db)
	eventPublishFailureSvc := service.NewEventPublishFailureService(eventPublishFailureRepo)
	eventPublishFailureHandler := handler.NewEventPublishFailureHandler(eventPublishFailureSvc)

	// EventPublisherService is optional, like every ServiceNow-only
	// dependency below — gated on EventHubBroker rather than cfg.DataSource,
	// since publishing is a distinct concern from which backend serves reads
	// (see config.Config.EventHubBroker's doc comment). Also gated on
	// EventPublishingEnabled, a separate safe-by-default kill switch: Event
	// Hub can be fully configured and this still stays nil until that's
	// explicitly turned on. nil when unset; every caller (snCaseService,
	// snIncidentService) already handles that.
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

	// sla_clocks has no ServiceNow equivalent either — same reasoning as
	// event_publish_failures above.
	slaClockRepo := repository.NewSLAClockRepository(db)
	slaClockHandler := handler.NewSLAClockHandler(service.NewSLAClockService(slaClockRepo))

	// scheduled_task_run has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/event_publish_failures above. Backs
	// operations/csm-scheduled-tasks; see that component's own CLAUDE.md
	// and this service's CLAUDE.md ("Scheduled task runs").
	scheduledTaskRunRepo := repository.NewScheduledTaskRunRepository(db)
	scheduledTaskRunHandler := handler.NewScheduledTaskRunHandler(service.NewScheduledTaskRunService(scheduledTaskRunRepo))

	// alert_incident_mapping has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/scheduled_task_run/event_publish_failures above.
	alertIncidentMappingRepo := repository.NewAlertIncidentMappingRepository(db)
	alertIncidentMappingHandler := handler.NewAlertIncidentMappingHandler(service.NewAlertIncidentMappingService(alertIncidentMappingRepo))

	accountRepo := repository.NewAccountRepository(db)
	accountHandler := handler.NewAccountHandler(service.NewAccountService(accountRepo))

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

	var accountContactHandler *handler.AccountContactHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		accountContactHandler = handler.NewAccountContactHandler(service.NewServiceNowAccountContactService(serviceNowIntegrationServiceClient))
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

	var projectContactHandler *handler.ProjectContactHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		projectContactHandler = handler.NewProjectContactHandler(service.NewServiceNowProjectContactService(serviceNowIntegrationServiceClient))
	}

	var projectUpdateHandler *handler.ProjectUpdateHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		projectUpdateHandler = handler.NewProjectUpdateHandler(service.NewServiceNowProjectUpdateService(serviceNowIntegrationServiceClient))
	}

	var projectStatsHandler *handler.ProjectStatsHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		projectStatsHandler = handler.NewProjectStatsHandler(service.NewServiceNowProjectStatsService(serviceNowIntegrationServiceClient))
	}

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

	deployedProductRepo := repository.NewDeployedProductRepository(db)
	var activeDeployedProductSvc service.DeployedProductService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeDeployedProductSvc = service.NewServiceNowDeployedProductService(serviceNowIntegrationServiceClient)
	} else {
		activeDeployedProductSvc = service.NewDeployedProductService(deployedProductRepo)
	}
	deployedProductHandler := handler.NewDeployedProductHandler(activeDeployedProductSvc)

	caseRepo := repository.NewCaseRepository(db)
	pgCaseSvc := service.NewCaseService(caseRepo, userRepo)
	var activeCaseSvc service.CaseService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeCaseSvc = service.NewServiceNowCaseService(serviceNowIntegrationServiceClient, pgCaseSvc, eventPublisher)
	} else {
		activeCaseSvc = pgCaseSvc
	}
	caseHandler := handler.NewCaseHandler(activeCaseSvc)

	var callRequestHandler *handler.CallRequestHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		callRequestHandler = handler.NewCallRequestHandler(service.NewServiceNowCallRequestService(serviceNowIntegrationServiceClient))
	}

	var caseGithubIssueHandler *handler.CaseGithubIssueHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		caseGithubIssueHandler = handler.NewCaseGithubIssueHandler(service.NewServiceNowCaseGithubIssueService(serviceNowIntegrationServiceClient))
	}

	var changeRequestHandler *handler.ChangeRequestHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		changeRequestHandler = handler.NewChangeRequestHandler(service.NewServiceNowChangeRequestService(serviceNowIntegrationServiceClient))
	}

	var timeCardHandler *handler.TimeCardHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		timeCardHandler = handler.NewTimeCardHandler(service.NewServiceNowTimeCardService(serviceNowIntegrationServiceClient))
	}

	var catalogHandler *handler.CatalogHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		catalogHandler = handler.NewCatalogHandler(service.NewServiceNowCatalogService(serviceNowIntegrationServiceClient))
	}

	var productVulnerabilityHandler *handler.ProductVulnerabilityHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		productVulnerabilityHandler = handler.NewProductVulnerabilityHandler(service.NewServiceNowProductVulnerabilityService(serviceNowIntegrationServiceClient))
	}

	var incidentHandler *handler.IncidentHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		incidentHandler = handler.NewIncidentHandler(service.NewServiceNowIncidentService(serviceNowIntegrationServiceClient, eventPublisher))
	}

	var problemHandler *handler.ProblemHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		problemHandler = handler.NewProblemHandler(service.NewServiceNowProblemService(serviceNowIntegrationServiceClient))
	}

	var incidentTaskHandler *handler.IncidentTaskHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		incidentTaskHandler = handler.NewIncidentTaskHandler(service.NewServiceNowIncidentTaskService(serviceNowIntegrationServiceClient))
	}

	var conversationHandler *handler.ConversationHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		conversationHandler = handler.NewConversationHandler(service.NewServiceNowConversationService(serviceNowIntegrationServiceClient))
	}

	var globalHandler *handler.GlobalHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		globalHandler = handler.NewGlobalHandler(service.NewServiceNowGlobalService(serviceNowIntegrationServiceClient))
	}

	var escalationHandler *handler.EscalationHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		escalationHandler = handler.NewEscalationHandler(service.NewServiceNowEscalationService(serviceNowIntegrationServiceClient))
	}

	var instanceHandler *handler.InstanceHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		instanceHandler = handler.NewInstanceHandler(service.NewServiceNowInstanceService(serviceNowIntegrationServiceClient))
	}

	var itServiceHandler *handler.ITServiceHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		itServiceHandler = handler.NewITServiceHandler(service.NewServiceNowITServiceService(serviceNowIntegrationServiceClient))
	}

	var serviceOfferingHandler *handler.ServiceOfferingHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		serviceOfferingHandler = handler.NewServiceOfferingHandler(service.NewServiceNowServiceOfferingService(serviceNowIntegrationServiceClient))
	}

	var groupHandler *handler.GroupHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		groupHandler = handler.NewGroupHandler(service.NewServiceNowGroupService(serviceNowIntegrationServiceClient))
	}

	var configurationItemHandler *handler.ConfigurationItemHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		configurationItemHandler = handler.NewConfigurationItemHandler(service.NewServiceNowConfigurationItemService(serviceNowIntegrationServiceClient))
	}

	var commentHandler *handler.CommentHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		commentHandler = handler.NewCommentHandler(service.NewServiceNowCommentService(serviceNowIntegrationServiceClient))
	}

	var taskSlaHandler *handler.TaskSlaHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		taskSlaHandler = handler.NewTaskSlaHandler(service.NewServiceNowTaskSlaService(serviceNowIntegrationServiceClient))
	}

	var snUserHandler *handler.SNUserHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snUserHandler = handler.NewSNUserHandler(service.NewServiceNowUserService(serviceNowIntegrationServiceClient))
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

	// event_publish_failures is not data-source specific, same rationale as
	// the role catalogue and team registry below — registered unconditionally.
	mux.HandleFunc("POST /event-publish-failures", eventPublishFailureHandler.CreateEventPublishFailure)
	mux.HandleFunc("POST /event-publish-failures/search", eventPublishFailureHandler.SearchEventPublishFailures)
	mux.HandleFunc("POST /event-publish-failures/{id}/resolve", eventPublishFailureHandler.ResolveEventPublishFailure)
	mux.HandleFunc("POST /cases/{caseId}/sla-clocks", slaClockHandler.RegisterSLAClock)
	mux.HandleFunc("GET /cases/{caseId}/sla-clocks/{clockType}", slaClockHandler.GetSLAClock)
	mux.HandleFunc("PATCH /cases/{caseId}/sla-clocks/{clockType}/tiers/{tier}", slaClockHandler.SetSLAClockTierReached)
	mux.HandleFunc("POST /scheduled-tasks/attempts", scheduledTaskRunHandler.AttemptScheduledTaskRun)
	mux.HandleFunc("PATCH /scheduled-tasks/attempts/{id}", scheduledTaskRunHandler.UpdateScheduledTaskRunAttempt)
	mux.HandleFunc("GET /scheduled-tasks/attempts", scheduledTaskRunHandler.ListScheduledTaskRuns)
	mux.HandleFunc("DELETE /scheduled-tasks/attempts", scheduledTaskRunHandler.DeleteScheduledTaskRuns)
	mux.HandleFunc("POST /alert-incident-mappings", alertIncidentMappingHandler.CreateAlertIncidentMapping)
	mux.HandleFunc("POST /alert-incident-mappings/lookup", alertIncidentMappingHandler.LookupAlertIncidentMappings)

	if snUserHandler != nil {
		mux.HandleFunc("GET /users/{id}", snUserHandler.GetUser)
		mux.HandleFunc("GET /users/me", snUserHandler.GetMe)
		mux.HandleFunc("PATCH /users/me", snUserHandler.PatchMe)
		mux.HandleFunc("POST /users/search", snUserHandler.SearchUsers)
	} else {
		mux.HandleFunc("GET /users/me", userHandler.GetMe)
		mux.HandleFunc("POST /users/search", userHandler.SearchUsers)
	}
	if snAccountHandler != nil {
		mux.HandleFunc("GET /accounts/{id}", snAccountHandler.GetAccount)
		mux.HandleFunc("POST /accounts/search", snAccountHandler.SearchAccounts)
	} else {
		mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)
		mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
	}
	if accountContactHandler != nil {
		mux.HandleFunc("POST /accounts/{id}/contacts/search", accountContactHandler.SearchAccountContacts)
	}
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	if projectContactHandler != nil {
		mux.HandleFunc("POST /projects/{id}/contacts/search", projectContactHandler.SearchProjectContacts)
		mux.HandleFunc("GET /projects/{id}/contacts/{contactId}", projectContactHandler.GetProjectContact)
	}
	if projectUpdateHandler != nil {
		mux.HandleFunc("PATCH /projects/{id}", projectUpdateHandler.UpdateProject)
	}
	if projectStatsHandler != nil {
		mux.HandleFunc("GET /projects/{id}/metadata", projectStatsHandler.GetProjectMetadata)
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
	mux.HandleFunc("POST /cases/group-by", caseHandler.GroupCasesBy)
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

	if callRequestHandler != nil {
		mux.HandleFunc("POST /call-requests", callRequestHandler.CreateCallRequest)
		mux.HandleFunc("POST /call-requests/search", callRequestHandler.SearchCallRequests)
		mux.HandleFunc("POST /call-requests/search-all", callRequestHandler.SearchAllCallRequests)
		mux.HandleFunc("PATCH /call-requests/{id}", callRequestHandler.PatchCallRequest)
	}

	if caseGithubIssueHandler != nil {
		mux.HandleFunc("POST /cases/{id}/github-issues", caseGithubIssueHandler.CreateCaseGithubIssue)
	}

	if changeRequestHandler != nil {
		mux.HandleFunc("POST /change-requests", changeRequestHandler.CreateChangeRequest)
		mux.HandleFunc("POST /change-requests/search", changeRequestHandler.SearchChangeRequests)
		mux.HandleFunc("POST /change-requests/group-by", changeRequestHandler.GroupChangeRequestsBy)
		mux.HandleFunc("GET /change-requests/{id}", changeRequestHandler.GetChangeRequest)
		mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
		mux.HandleFunc("GET /change-requests/{id}/approvals", changeRequestHandler.GetChangeRequestApprovals)
		mux.HandleFunc("POST /change-requests/{id}/approvals/decision", changeRequestHandler.DecideChangeRequestApproval)
	}

	if timeCardHandler != nil {
		mux.HandleFunc("POST /time-cards/search", timeCardHandler.SearchTimeCards)
		mux.HandleFunc("POST /time-cards", timeCardHandler.CreateTimeCard)
		mux.HandleFunc("PATCH /time-cards/{id}", timeCardHandler.UpdateTimeCard)
		mux.HandleFunc("POST /cases/time-cards/search", timeCardHandler.SearchCaseTimeCards)
		mux.HandleFunc("DELETE /time-cards/{id}", timeCardHandler.DeleteTimeCard)
	}

	if catalogHandler != nil {
		mux.HandleFunc("POST /catalogs/search", catalogHandler.SearchCatalogs)
		mux.HandleFunc("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", catalogHandler.GetCatalogItemVariables)
	}

	if productVulnerabilityHandler != nil {
		mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
		mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)
		mux.HandleFunc("GET /products/vulnerabilities/meta", productVulnerabilityHandler.GetVulnerabilityMeta)
	}

	if itServiceHandler != nil {
		mux.HandleFunc("POST /services/search", itServiceHandler.SearchITServices)
	}

	if serviceOfferingHandler != nil {
		mux.HandleFunc("POST /service-offerings/search", serviceOfferingHandler.SearchServiceOfferings)
	}

	if groupHandler != nil {
		mux.HandleFunc("POST /groups/search", groupHandler.SearchGroups)
	}

	if configurationItemHandler != nil {
		mux.HandleFunc("POST /configuration-items/search", configurationItemHandler.SearchConfigurationItems)
	}

	if commentHandler != nil {
		mux.HandleFunc("POST /comments", commentHandler.CreateComment)
		mux.HandleFunc("POST /comments/search", commentHandler.SearchComments)
	}

	if taskSlaHandler != nil {
		mux.HandleFunc("GET /slas/{id}", taskSlaHandler.GetTaskSla)
		mux.HandleFunc("POST /slas/search", taskSlaHandler.SearchTaskSlas)
	}

	// Registered unconditionally; the non-ServiceNow data source is served by
	// service.NewUnavailableTaskService, which answers 503 (see above).
	mux.HandleFunc("POST /cases/{id}/tasks/search", taskHandler.SearchCaseTasks)
	mux.HandleFunc("POST /tasks/search", taskHandler.SearchTasks)
	mux.HandleFunc("GET /tasks/{id}", taskHandler.GetTask)
	mux.HandleFunc("POST /cases/{id}/tasks", taskHandler.CreateCaseTask)
	mux.HandleFunc("PATCH /tasks/{id}", taskHandler.UpdateTask)

	if incidentHandler != nil {
		mux.HandleFunc("GET /incidents/{id}", incidentHandler.GetIncident)
		mux.HandleFunc("PATCH /incidents/{id}", incidentHandler.PatchIncident)
		mux.HandleFunc("POST /incidents", incidentHandler.CreateIncident)
		mux.HandleFunc("POST /incidents/search", incidentHandler.SearchIncidents)
		mux.HandleFunc("POST /incidents/group-by", incidentHandler.GroupIncidentsBy)
		mux.HandleFunc("POST /incidents/{id}/activities/search", incidentHandler.SearchIncidentActivities)
	}

	if problemHandler != nil {
		mux.HandleFunc("POST /problems", problemHandler.CreateProblem)
		mux.HandleFunc("POST /problems/search", problemHandler.SearchProblems)
		mux.HandleFunc("POST /problems/group-by", problemHandler.GroupProblemsBy)
		mux.HandleFunc("GET /problems/{id}", problemHandler.GetProblem)
	}

	if incidentTaskHandler != nil {
		mux.HandleFunc("POST /incident-tasks/search", incidentTaskHandler.SearchIncidentTasks)
		mux.HandleFunc("POST /incident-tasks/group-by", incidentTaskHandler.GroupIncidentTasksBy)
		mux.HandleFunc("GET /incident-tasks/{id}", incidentTaskHandler.GetIncidentTask)
	}

	if conversationHandler != nil {
		mux.HandleFunc("POST /conversations/search", conversationHandler.SearchConversations)
		mux.HandleFunc("GET /conversations/{id}", conversationHandler.GetConversation)
		mux.HandleFunc("POST /conversations", conversationHandler.CreateConversation)
		mux.HandleFunc("PATCH /conversations/{id}", conversationHandler.UpdateConversation)
	}

	if globalHandler != nil {
		mux.HandleFunc("GET /metadata", globalHandler.GetSystemMetadata)
		mux.HandleFunc("POST /search", globalHandler.GlobalSearch)
	}

	if escalationHandler != nil {
		mux.HandleFunc("POST /escalations/search", escalationHandler.SearchEscalations)
		mux.HandleFunc("POST /escalations", escalationHandler.CreateEscalation)
	}

	if instanceHandler != nil {
		mux.HandleFunc("POST /instances/search", instanceHandler.SearchInstances)
		mux.HandleFunc("POST /instances/metrics/search", instanceHandler.SearchInstanceMetrics)
		mux.HandleFunc("POST /instances/usages/search", instanceHandler.SearchInstanceUsage)
		mux.HandleFunc("POST /instances/metrics/stats/search", instanceHandler.SearchInstanceMetricsStats)
		mux.HandleFunc("POST /instances/usages/stats/search", instanceHandler.SearchInstanceUsageStats)
	}

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
