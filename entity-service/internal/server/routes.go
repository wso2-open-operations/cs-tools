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
	accountSvc := service.NewAccountService(accountRepo)
	accountHandler := handler.NewAccountHandler(accountSvc)

	var serviceNowIntegrationServiceClient *integrationservice.Client
	if cfg.DataSource == config.DataSourceServiceNow {
		serviceNowIntegrationServiceClient = integrationservice.New(cfg.ServiceNowIntegrationServiceBaseURL, integrationservice.ClientCredentialsConfig{
			TokenURL:     cfg.ServiceNowIntegrationServiceTokenURL,
			ClientID:     cfg.ServiceNowIntegrationServiceClientID,
			ClientSecret: cfg.ServiceNowIntegrationServiceClientSecret,
			Scopes:       cfg.ServiceNowIntegrationServiceScopes,
		})
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
	deployedProductSvc := service.NewDeployedProductService(deployedProductRepo)
	deployedProductHandler := handler.NewDeployedProductHandler(deployedProductSvc)

	caseRepo := repository.NewCaseRepository(db)
	pgCaseSvc := service.NewCaseService(caseRepo)
	var activeCaseSvc service.CaseService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeCaseSvc = service.NewServiceNowCaseService(serviceNowIntegrationServiceClient, pgCaseSvc)
	} else {
		activeCaseSvc = pgCaseSvc
	}
	caseHandler := handler.NewCaseHandler(activeCaseSvc)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.HealthCheck)
	mux.HandleFunc("POST /users/search", userHandler.SearchUsers)
	mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)
	mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	mux.HandleFunc("POST /products/search", productHandler.SearchProducts)
	mux.HandleFunc("POST /products/{id}/versions/search", productVersionHandler.SearchProductVersions)
	mux.HandleFunc("POST /deployments/search", deploymentHandler.SearchDeployments)
	mux.HandleFunc("POST /deployed-products/search", deployedProductHandler.SearchDeployedProducts)
	mux.HandleFunc("GET /cases/{id}", caseHandler.GetCase)
	mux.HandleFunc("PATCH /cases/{id}", caseHandler.PatchCase)
	mux.HandleFunc("POST /cases", caseHandler.CreateCase)
	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/comments/search", caseHandler.SearchCaseComments)

	return middleware.CorrelationID(
		middleware.Recovery(
			middleware.Logger(
				middleware.UserIDToken(
					middleware.Timeout(10 * time.Second)(mux),
				),
			),
		),
	)
}
