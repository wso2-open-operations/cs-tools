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

	projectRepo := repository.NewProjectRepository(db)
	pgProjectSvc := service.NewProjectService(projectRepo)
	var activeProjectSvc service.ProjectService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeProjectSvc = service.NewServiceNowProjectService(serviceNowIntegrationServiceClient, pgProjectSvc)
	} else {
		activeProjectSvc = pgProjectSvc
	}
	projectHandler := handler.NewProjectHandler(activeProjectSvc)

	productRepo := repository.NewProductRepository(db)
	productSvc := service.NewProductService(productRepo)
	productHandler := handler.NewProductHandler(productSvc)

	var snProductHandler *handler.SNProductHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snProductHandler = handler.NewSNProductHandler(service.NewServiceNowProductService(serviceNowIntegrationServiceClient))
	}

	productVersionRepo := repository.NewProductVersionRepository(db)
	productVersionSvc := service.NewProductVersionService(productVersionRepo)
	productVersionHandler := handler.NewProductVersionHandler(productVersionSvc)

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

	var snUserHandler *handler.SNUserHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snUserHandler = handler.NewSNUserHandler(service.NewServiceNowUserService(serviceNowIntegrationServiceClient))
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.HealthCheck)
	if snUserHandler != nil {
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
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	if snProductHandler != nil {
		mux.HandleFunc("POST /products/search", snProductHandler.SearchProducts)
	} else {
		mux.HandleFunc("POST /products/search", productHandler.SearchProducts)
	}
	mux.HandleFunc("POST /products/{id}/versions/search", productVersionHandler.SearchProductVersions)
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
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/comments/search", caseHandler.SearchCaseComments)
	mux.HandleFunc("POST /attachments", caseHandler.CreateCaseAttachment)
	mux.HandleFunc("POST /attachments/search", caseHandler.SearchCaseAttachments)
	mux.HandleFunc("GET /attachments/{id}/content", caseHandler.GetCaseAttachmentContent)
	mux.HandleFunc("DELETE /attachments/{id}", caseHandler.DeleteCaseAttachment)

	if callRequestHandler != nil {
		mux.HandleFunc("POST /call-requests", callRequestHandler.CreateCallRequest)
		mux.HandleFunc("POST /call-requests/search", callRequestHandler.SearchCallRequests)
		mux.HandleFunc("PATCH /call-requests/{id}", callRequestHandler.PatchCallRequest)
	}

	if caseGithubIssueHandler != nil {
		mux.HandleFunc("POST /cases/{id}/github-issues", caseGithubIssueHandler.CreateCaseGithubIssue)
	}

	if changeRequestHandler != nil {
		mux.HandleFunc("POST /change-requests/search", changeRequestHandler.SearchChangeRequests)
		mux.HandleFunc("GET /change-requests/{id}", changeRequestHandler.GetChangeRequest)
		mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
	}

	if timeCardHandler != nil {
		mux.HandleFunc("POST /time-cards/search", timeCardHandler.SearchTimeCards)
		mux.HandleFunc("POST /time-cards", timeCardHandler.CreateTimeCard)
		mux.HandleFunc("PATCH /time-cards/{id}", timeCardHandler.UpdateTimeCard)
	}

	if catalogHandler != nil {
		mux.HandleFunc("POST /catalogs/search", catalogHandler.SearchCatalogs)
		mux.HandleFunc("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", catalogHandler.GetCatalogItemVariables)
	}

	if productVulnerabilityHandler != nil {
		mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
		mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)
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
