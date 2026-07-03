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
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/updates"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()
	cfg := entity.Config{
		BaseURL:      mustEnv("ENTITY_BASE_URL"),
		TokenURL:     mustEnv("ENTITY_TOKEN_URL"),
		ClientID:     mustEnv("ENTITY_CLIENT_ID"),
		ClientSecret: mustEnv("ENTITY_CLIENT_SECRET"),
		// Scopes is optional; set ENTITY_SCOPES as a comma-separated list if required.
		Scopes: splitComma(os.Getenv("ENTITY_SCOPES")),
	}

	entityClient := entity.NewClient(cfg)
	caseHandler := handler.NewCaseHandler(entityClient)
	accountHandler := handler.NewAccountHandler(entityClient)
	projectHandler := handler.NewProjectHandler(entityClient)
	productHandler := handler.NewProductHandler(entityClient)
	deploymentHandler := handler.NewDeploymentHandler(entityClient)
	changeRequestHandler := handler.NewChangeRequestHandler(entityClient)
	itServiceHandler := handler.NewITServiceHandler(entityClient)
	serviceOfferingHandler := handler.NewServiceOfferingHandler(entityClient)
	groupHandler := handler.NewGroupHandler(entityClient)
	configurationItemHandler := handler.NewConfigurationItemHandler(entityClient)
	catalogHandler := handler.NewCatalogHandler(entityClient)
	timeCardHandler := handler.NewTimeCardHandler(entityClient)
	productVulnerabilityHandler := handler.NewProductVulnerabilityHandler(entityClient)

	updatesCfg := updates.Config{
		BaseURL:      mustEnv("UPDATES_BASE_URL"),
		TokenURL:     mustEnv("UPDATES_TOKEN_URL"),
		ClientID:     mustEnv("UPDATES_CLIENT_ID"),
		ClientSecret: mustEnv("UPDATES_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("UPDATES_SCOPES")),
	}
	updatesClient := updates.NewClient(updatesCfg)
	updatesHandler := handler.NewUpdatesHandler(updatesClient)

	scimCfg := scim.Config{
		BaseURL:      mustEnv("SCIM_BASE_URL"),
		TokenURL:     mustEnv("SCIM_TOKEN_URL"),
		ClientID:     mustEnv("SCIM_CLIENT_ID"),
		ClientSecret: mustEnv("SCIM_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("SCIM_SCOPES")),
	}
	scimClient := scim.NewClient(scimCfg)
	usersHandler := handler.NewUsersHandler(scimClient, entityClient)

	authCfg := middleware.Config{
		JWKSEndpoint:          mustEnv("AUTH_JWKS_ENDPOINT"),
		Issuer:                mustEnv("AUTH_ISSUER"),
		Audience:              mustEnv("AUTH_AUDIENCE"),
		ClockSkew:             5 * time.Second,
		TokenValidatorEnabled: os.Getenv("AUTH_TOKEN_VALIDATOR_ENABLED") != "false",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /cases", caseHandler.CreateCase)
	mux.HandleFunc("GET /cases/{id}", caseHandler.GetCase)
	mux.HandleFunc("PATCH /cases/{id}", caseHandler.PatchCase)
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/comments/search", caseHandler.SearchCaseComments)
	mux.HandleFunc("POST /attachments", caseHandler.CreateCaseAttachment)
	mux.HandleFunc("POST /attachments/search", caseHandler.SearchCaseAttachments)
	mux.HandleFunc("GET /attachments/{id}/content", caseHandler.GetCaseAttachmentContent)
	mux.HandleFunc("DELETE /attachments/{id}", caseHandler.DeleteCaseAttachment)
	mux.HandleFunc("POST /cases/{id}/call-requests", caseHandler.CreateCallRequest)
	mux.HandleFunc("POST /cases/{id}/call-requests/search", caseHandler.SearchCallRequests)
	mux.HandleFunc("PATCH /cases/{caseId}/call-requests/{callRequestId}", caseHandler.PatchCallRequest)
	mux.HandleFunc("POST /cases/{id}/github-issues", caseHandler.CreateCaseGithubIssue)
	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	mux.HandleFunc("GET /updates/product-update-levels", updatesHandler.GetProductUpdateLevels)
	mux.HandleFunc("POST /updates/levels/search", updatesHandler.SearchUpdatesBetweenUpdateLevels)
	mux.HandleFunc("GET /users/me", usersHandler.GetMe)
	mux.HandleFunc("PATCH /users/me", usersHandler.PatchMe)
	mux.HandleFunc("POST /users/search", usersHandler.SearchUsers)
	mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)
	mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	mux.HandleFunc("POST /products/search", productHandler.SearchProducts)
	mux.HandleFunc("POST /products/{id}/versions/search", productHandler.SearchProductVersions)
	mux.HandleFunc("POST /deployments", deploymentHandler.PostDeployment)
	mux.HandleFunc("POST /deployments/search", deploymentHandler.SearchDeployments)
	mux.HandleFunc("PATCH /deployments/{id}", deploymentHandler.PatchDeployment)
	mux.HandleFunc("POST /deployments/{id}/products", deploymentHandler.PostDeployedProduct)
	mux.HandleFunc("POST /deployments/{id}/products/search", deploymentHandler.SearchDeployedProducts)
	mux.HandleFunc("PATCH /deployments/{deploymentId}/products/{productId}", deploymentHandler.PatchDeployedProduct)
	mux.HandleFunc("POST /change-requests", changeRequestHandler.CreateChangeRequest)
	mux.HandleFunc("GET /change-requests/{id}", changeRequestHandler.GetChangeRequest)
	mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
	mux.HandleFunc("POST /change-requests/search", changeRequestHandler.SearchChangeRequests)
	mux.HandleFunc("POST /services/search", itServiceHandler.SearchITServices)
	mux.HandleFunc("POST /service-offerings/search", serviceOfferingHandler.SearchServiceOfferings)
	mux.HandleFunc("POST /groups/search", groupHandler.SearchGroups)
	mux.HandleFunc("POST /configuration-items/search", configurationItemHandler.SearchConfigurationItems)
	mux.HandleFunc("POST /time-cards/search", timeCardHandler.SearchTimeCards)
	mux.HandleFunc("POST /time-cards", timeCardHandler.CreateTimeCard)
	mux.HandleFunc("PATCH /time-cards/{id}", timeCardHandler.UpdateTimeCard)
	mux.HandleFunc("POST /catalogs/search", catalogHandler.SearchCatalogs)
	mux.HandleFunc("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", catalogHandler.GetCatalogItemVariables)
	mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
	mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)

	addr := envOrDefault("PORT", ":8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("CSM Portal Backend started", "addr", addr)

	srv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CorrelationID(
				middleware.Auth(authCfg)(
					middleware.Logger(mux),
				),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("CSM Portal Backend stopped")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Silently ignored if the file does not exist; logs a warning for any other error.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// Strip surrounding quotes from value.
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
