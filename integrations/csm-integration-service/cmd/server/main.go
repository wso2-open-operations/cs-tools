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
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/middleware"
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
	accountHandler := handler.NewAccountHandler(entityClient)
	projectHandler := handler.NewProjectHandler(entityClient)
	vulnerabilityHandler := handler.NewVulnerabilityHandler(entityClient)
	// UMT_INTEGRATION_ACTOR_EMAIL is this service's own trusted M2M actor
	// identity, asserted on POST /cases/{id}/comments (CreateCaseComment)
	// and POST /cases/{id}/tags (AddCaseTag), as entity-service's
	// actorEmail field. It must match an entry in entity-service's
	// M2M_TRUSTED_ACTOR_EMAILS allowlist or every call needing it 403s.
	// Optional here at startup by design: an empty/unset value is a
	// deploy-time misconfiguration, not something this service validates
	// defensively -- the resulting entity-service 403 surfaces normally.
	umtActorEmail := os.Getenv("UMT_INTEGRATION_ACTOR_EMAIL")
	// MARK_FIX_ISSUED_ACTOR_EMAILS maps a third-party consumer's own client id
	// (as asserted in the x-jwt-assertion header Choreo's gateway forwards) to
	// that consumer's display email, so a PATCH /cases/{id} markFixIssued call
	// is attributed to the real integration that made it in entity-service's
	// work_item_activity feed, instead of always showing this service's own
	// single, shared service-to-service client id (see
	// handler.CaseHandler.clientActorEmails' own doc comment). A JSON object,
	// e.g. {"client-a":"team-a@example.com"}. Optional by design: a client id
	// with no entry here is simply not attributed, same as before this
	// existed.
	markFixIssuedActorEmails := parseClientActorEmails(os.Getenv("MARK_FIX_ISSUED_ACTOR_EMAILS"))
	caseHandler := handler.NewCaseHandler(entityClient, umtActorEmail, markFixIssuedActorEmails)
	opportunityHandler := handler.NewOpportunityHandler(entityClient)
	invoiceHandler := handler.NewInvoiceHandler(entityClient)
	projectOpportunityLinkHandler := handler.NewProjectOpportunityLinkHandler(entityClient)
	incidentHandler := handler.NewIncidentHandler(entityClient)
	itServiceHandler := handler.NewITServiceHandler(entityClient)
	alertIncidentMappingHandler := handler.NewAlertIncidentMappingHandler(entityClient)
	cloudStatusHandler := handler.NewCloudStatusHandler(entityClient)
	catalogHandler := handler.NewCatalogHandler(entityClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)
	mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
	mux.HandleFunc("POST /accounts/{id}/contacts/search", accountHandler.SearchAccountContacts)
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	mux.HandleFunc("POST /projects/{id}/contacts/search", projectHandler.SearchProjectContacts)
	mux.HandleFunc("PATCH /projects/{id}", projectHandler.UpdateProject)
	mux.HandleFunc("POST /vulnerabilities/sync", vulnerabilityHandler.SyncProductVulnerabilities)
	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	mux.HandleFunc("PATCH /cases/{id}", caseHandler.PatchCase)
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/tags", caseHandler.AddCaseTag)
	mux.HandleFunc("POST /opportunities/search", opportunityHandler.SearchOpportunities)
	mux.HandleFunc("GET /opportunities/{id}", opportunityHandler.GetOpportunity)
	mux.HandleFunc("POST /invoices/search", invoiceHandler.SearchInvoices)
	mux.HandleFunc("GET /invoices/{id}", invoiceHandler.GetInvoice)
	mux.HandleFunc("POST /project-opportunity-links/search", projectOpportunityLinkHandler.SearchProjectOpportunityLinks)
	mux.HandleFunc("POST /incidents", incidentHandler.CreateIncident)
	mux.HandleFunc("PATCH /incidents/{id}", incidentHandler.PatchIncident)
	mux.HandleFunc("POST /incidents/search", incidentHandler.SearchIncidents)
	mux.HandleFunc("POST /services/search", itServiceHandler.SearchITServices)
	mux.HandleFunc("POST /alert-incident-mappings", alertIncidentMappingHandler.CreateAlertIncidentMapping)
	mux.HandleFunc("POST /alert-incident-mappings/lookup", alertIncidentMappingHandler.LookupAlertIncidentMappings)
	mux.HandleFunc("POST /deployed-products/search", catalogHandler.SearchDeployedProducts)
	mux.HandleFunc("POST /deployments/search", catalogHandler.SearchDeployments)
	mux.HandleFunc("POST /products/search", catalogHandler.SearchProducts)
	mux.HandleFunc("POST /products/{id}/versions/search", catalogHandler.SearchProductVersions)
	mux.HandleFunc("GET /cloud-status/monitors", cloudStatusHandler.GetMonitors)
	mux.HandleFunc("GET /cloud-status/incidents", cloudStatusHandler.GetIncidents)
	mux.HandleFunc("GET /cloud-status/availabilities", cloudStatusHandler.GetAvailabilities)
	mux.HandleFunc("GET /cloud-status/availability-history", cloudStatusHandler.GetAvailabilityHistory)
	mux.HandleFunc("GET /cloud-status/incidents/{id}", cloudStatusHandler.GetIncidentDetail)

	addr := ":" + envOrDefault("PORT", "8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("Integration Service started", "addr", addr)

	// No Auth layer in this middleware chain — inbound requests are trusted at the
	// Choreo API Manager gateway (subscription + M2M app auth), not validated again
	// in this service. See internal/handler's doc comments for the same note.
	srv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CorrelationID(
				middleware.Logger(mux),
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
	slog.Info("Integration Service stopped")
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

// parseClientActorEmails parses a JSON object value, e.g.
// `{"client-a":"team-a@example.com","client-b":"team-b@example.com"}`, into
// a map. An empty input, or one that fails to parse as a JSON object of
// strings, is logged (the latter at WARN) and returns nil -- this config
// only drives a best-effort display-attribution enrichment, never a security
// decision, so a malformed value must not fail startup; it should just
// disable the enrichment, which handler.CaseHandler.attributeMarkFixIssued
// already treats a nil/empty map as. An entry whose client id or email is
// blank once trimmed is dropped individually, with the same reasoning.
func parseClientActorEmails(s string) map[string]string {
	if s == "" {
		return nil
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		slog.Warn("MARK_FIX_ISSUED_ACTOR_EMAILS: not a valid JSON object of client id to email, disabling markFixIssued attribution", "err", err)
		return nil
	}
	result := make(map[string]string, len(raw))
	for clientID, email := range raw {
		clientID, email = strings.TrimSpace(clientID), strings.TrimSpace(email)
		if clientID == "" || email == "" {
			slog.Warn("MARK_FIX_ISSUED_ACTOR_EMAILS: skipping entry with a blank client id or email")
			continue
		}
		result[clientID] = email
	}
	if len(result) == 0 {
		return nil
	}
	return result
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
