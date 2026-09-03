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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// NewRouter builds the dependency graph (repository → service → handler),
// registers all routes, and wraps the mux with the middleware chain:
// CorrelationID → Recovery → Logger → UserIDToken → Timeout.
func NewRouter(db *pgxpool.Pool, cfg *config.Config) http.Handler {
	userRepo := repository.NewUserRepository(db)
	userSvc := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userSvc)

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
	} else if db != nil {
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
		activeCaseSvc = service.NewServiceNowCaseService(serviceNowIntegrationServiceClient, pgCaseSvc)
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
		caseGithubIssueHandler = handler.NewCaseGithubIssueHandler(service.NewServiceNowCaseGithubIssueService(serviceNowIntegrationServiceClient, activeCaseSvc))
	}

	var escalationHandler *handler.EscalationHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		escalationHandler = handler.NewEscalationHandler(service.NewServiceNowEscalationService(serviceNowIntegrationServiceClient, activeCaseSvc))
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

	var feedbackHandler *handler.FeedbackHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		feedbackHandler = handler.NewFeedbackHandler(service.NewServiceNowFeedbackService(serviceNowIntegrationServiceClient))
	}

	var productVulnerabilityHandler *handler.ProductVulnerabilityHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		productVulnerabilityHandler = handler.NewProductVulnerabilityHandler(service.NewServiceNowProductVulnerabilityService(serviceNowIntegrationServiceClient))
	}

	var incidentHandler *handler.IncidentHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		incidentHandler = handler.NewIncidentHandler(service.NewServiceNowIncidentService(serviceNowIntegrationServiceClient))
	}

	var problemHandler *handler.ProblemHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		problemHandler = handler.NewProblemHandler(service.NewServiceNowProblemService(serviceNowIntegrationServiceClient))
	}

	var alertHandler *handler.AlertHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		alertHandler = handler.NewAlertHandler(service.NewServiceNowAlertService(serviceNowIntegrationServiceClient))
	}

	var smartAlertHandler *handler.SmartAlertHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		smartAlertHandler = handler.NewSmartAlertHandler(service.NewServiceNowSmartAlertService(serviceNowIntegrationServiceClient))
	}

	var incidentTaskHandler *handler.IncidentTaskHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		incidentTaskHandler = handler.NewIncidentTaskHandler(service.NewServiceNowIncidentTaskService(serviceNowIntegrationServiceClient))
	}

	var conversationHandler *handler.ConversationHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		conversationHandler = handler.NewConversationHandler(service.NewServiceNowConversationService(serviceNowIntegrationServiceClient))
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
	if snUserHandler != nil {
		mux.HandleFunc("GET /users/{id}", snUserHandler.GetUser)
		mux.HandleFunc("GET /users/me", snUserHandler.GetMe)
		mux.HandleFunc("PATCH /users/me", snUserHandler.PatchMe)
		mux.HandleFunc("POST /users/search", snUserHandler.SearchUsers)
	} else {
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
	mux.HandleFunc("POST /cases/{id}/activities/search", caseHandler.SearchCaseActivities)
	mux.HandleFunc("POST /attachments", caseHandler.CreateCaseAttachment)
	mux.HandleFunc("POST /attachments/search", caseHandler.SearchCaseAttachments)
	mux.HandleFunc("GET /attachments/{id}/content", caseHandler.GetCaseAttachmentContent)
	mux.HandleFunc("GET /attachments/{id}", caseHandler.GetAttachment)
	mux.HandleFunc("PATCH /attachments/{id}", caseHandler.UpdateAttachment)
	mux.HandleFunc("DELETE /attachments/{id}", caseHandler.DeleteCaseAttachment)
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

	if escalationHandler != nil {
		mux.HandleFunc("GET /cases/{id}/escalations", escalationHandler.SearchCaseEscalations)
		mux.HandleFunc("POST /cases/{id}/escalations", escalationHandler.CreateCaseEscalation)
	}

	if changeRequestHandler != nil {
		mux.HandleFunc("POST /change-requests", changeRequestHandler.CreateChangeRequest)
		mux.HandleFunc("POST /change-requests/search", changeRequestHandler.SearchChangeRequests)
		mux.HandleFunc("POST /change-requests/aggregate", changeRequestHandler.AggregateChangeRequests)
		mux.HandleFunc("GET /change-requests/{id}", changeRequestHandler.GetChangeRequest)
		mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
		mux.HandleFunc("GET /change-requests/{id}/approvals", changeRequestHandler.GetChangeRequestApprovals)
		mux.HandleFunc("POST /change-requests/{id}/approvals/decision", changeRequestHandler.DecideChangeRequestApproval)
	}

	if timeCardHandler != nil {
		mux.HandleFunc("POST /time-cards/search", timeCardHandler.SearchTimeCards)
		mux.HandleFunc("POST /time-cards", timeCardHandler.CreateTimeCard)
		mux.HandleFunc("PATCH /time-cards/{id}", timeCardHandler.UpdateTimeCard)
		mux.HandleFunc("DELETE /time-cards/{id}", timeCardHandler.DeleteTimeCard)
	}

	if catalogHandler != nil {
		mux.HandleFunc("POST /catalogs/search", catalogHandler.SearchCatalogs)
		mux.HandleFunc("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", catalogHandler.GetCatalogItemVariables)
	}

	if productVulnerabilityHandler != nil {
		mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
		mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)
		mux.HandleFunc("POST /products/vulnerabilities/sync", productVulnerabilityHandler.SyncProductVulnerabilities)
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
		mux.HandleFunc("POST /incidents/aggregate", incidentHandler.AggregateIncidents)
		mux.HandleFunc("POST /incidents/{id}/activities/search", incidentHandler.SearchIncidentActivities)
	}

	if problemHandler != nil {
		mux.HandleFunc("POST /problems", problemHandler.CreateProblem)
		mux.HandleFunc("POST /problems/search", problemHandler.SearchProblems)
		mux.HandleFunc("POST /problems/aggregate", problemHandler.AggregateProblems)
		mux.HandleFunc("GET /problems/{id}", problemHandler.GetProblem)
		mux.HandleFunc("PATCH /problems/{id}", problemHandler.PatchProblem)
	}

	if incidentTaskHandler != nil {
		mux.HandleFunc("POST /incident-tasks/search", incidentTaskHandler.SearchIncidentTasks)
		mux.HandleFunc("POST /incident-tasks/aggregate", incidentTaskHandler.AggregateIncidentTasks)
		mux.HandleFunc("GET /incident-tasks/{id}", incidentTaskHandler.GetIncidentTask)
	}

	if alertHandler != nil {
		mux.HandleFunc("GET /alerts/{id}", alertHandler.GetAlert)
	}

	if smartAlertHandler != nil {
		mux.HandleFunc("GET /smart-alerts/{id}", smartAlertHandler.GetSmartAlert)
	}

	if conversationHandler != nil {
		mux.HandleFunc("POST /conversations/search", conversationHandler.SearchConversations)
	}

	return middleware.CorrelationID(
		middleware.Recovery(
			middleware.Logger(
				middleware.UserIDToken(
					middleware.Timeout(30 * time.Second)(mux),
				),
			),
		),
	)
}
