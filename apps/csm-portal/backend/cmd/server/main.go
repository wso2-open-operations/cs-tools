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
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/updates"
)

func main() {
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
	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	mux.HandleFunc("GET /cases/{id}", caseHandler.GetCase)
	mux.HandleFunc("GET /updates/recommended-update-levels", updatesHandler.GetRecommendedUpdateLevels)
	mux.HandleFunc("GET /updates/product-update-levels", updatesHandler.GetProductUpdateLevels)
	mux.HandleFunc("POST /updates/levels/search", updatesHandler.SearchUpdatesBetweenUpdateLevels)
	mux.HandleFunc("GET /users/me", usersHandler.GetMe)
	mux.HandleFunc("PATCH /users/me", usersHandler.PatchMe)
	mux.HandleFunc("POST /users/search", usersHandler.SearchUsers)
	mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	mux.HandleFunc("POST /products/search", productHandler.SearchProducts)
	mux.HandleFunc("POST /product/{id}/versions/search", productHandler.SearchProductVersions)

	addr := envOrDefault("PORT", ":8080")
	slog.Info("starting csm-portal backend", "addr", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           middleware.Auth(authCfg)(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
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
