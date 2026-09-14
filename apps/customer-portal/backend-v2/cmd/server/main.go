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

// Command server runs the customer-portal backend-v2 HTTP server.
package main

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/productconsumption"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/registry"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/updates"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/usermanagement"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	// entity-service, the updates service, and SCIM all authenticate as the
	// same OAuth2 client-credentials app; only each service's base URL and
	// scopes differ.
	oauth2ClientID := os.Getenv("OAUTH2_CLIENT_ID")
	oauth2ClientSecret := os.Getenv("OAUTH2_CLIENT_SECRET")
	oauth2TokenURL := optionalURL("OAUTH2_TOKEN_URL", "http", "https")

	entityCfg := entity.Config{
		BaseURL:      mustURL("ENTITY_SERVICE_BASE_URL", "http", "https"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("ENTITY_SERVICE_SCOPES")),
	}
	entityClient := entity.NewClient(entityCfg)

	updatesCfg := updates.Config{
		BaseURL:      mustURL("UPDATES_BASE_URL", "http", "https"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("UPDATES_SCOPES")),
	}
	updatesClient := updates.NewClient(updatesCfg)

	scimCfg := scim.Config{
		BaseURL:      mustURL("SCIM_BASE_URL", "http", "https"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("SCIM_SCOPES")),
	}
	scimClient := scim.NewClient(scimCfg)

	// The AI chat agent is a separate Python service (not entity-service),
	// but authenticates as the same shared OAuth2 client-credentials app as
	// entity/updates/scim above; only its base URLs differ.
	aiChatAgentCfg := aichatagent.Config{
		BaseURL:      mustURL("AI_CHAT_AGENT_BASE_URL", "http", "https"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("AI_CHAT_AGENT_SCOPES")),
	}
	aiChatAgentClient := aichatagent.NewClient(aiChatAgentCfg)

	aiChatAgentWsCfg := aichatagent.WSConfig{
		BaseURL:      mustURL("AI_CHAT_AGENT_WS_BASE_URL", "ws", "wss"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("AI_CHAT_AGENT_WS_SCOPES")),
	}
	aiChatAgentWsClient := aichatagent.NewWSClient(aiChatAgentWsCfg)

	// The product-consumption service(s) are separate services (not
	// entity-service) that provision deployment licenses and import usage
	// data; both authenticate as the same shared OAuth2 app. These are two
	// independently-configurable base URLs — PRODUCT_CONSUMPTION_TRACKING_BASE_URL
	// defaults to PRODUCT_CONSUMPTION_SUBSCRIPTION_URL when unset, for
	// deployments where both happen to point at the same host.
	productConsumptionCfg := productconsumption.Config{
		SubscriptionBaseURL: mustURL("PRODUCT_CONSUMPTION_SUBSCRIPTION_URL", "http", "https"),
		TrackingBaseURL:     optionalURL("PRODUCT_CONSUMPTION_TRACKING_BASE_URL", "http", "https"),
		TokenURL:            oauth2TokenURL,
		ClientID:            oauth2ClientID,
		ClientSecret:        oauth2ClientSecret,
		Scopes:              splitComma(os.Getenv("PRODUCT_CONSUMPTION_SCOPES")),
	}
	productConsumptionClient := productconsumption.NewClient(productConsumptionCfg)

	// The registry service (robot-account/registry-token issuance) is a
	// separate microservice (not entity-service), authenticating as the same
	// shared OAuth2 app.
	registryCfg := registry.Config{
		BaseURL:      mustURL("REGISTRY_BASE_URL", "http", "https"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("REGISTRY_SCOPES")),
	}
	registryClient, err := registry.NewClient(registryCfg)
	if err != nil {
		slog.Error("failed to construct registry client", "err", err)
		os.Exit(1)
	}

	// The project-contact onboarding service (contact/membership management)
	// is a separate microservice (not entity-service, not SCIM),
	// authenticating as the same shared OAuth2 app.
	userManagementCfg := usermanagement.Config{
		BaseURL:      mustURL("USER_MANAGEMENT_BASE_URL", "http", "https"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("USER_MANAGEMENT_SCOPES")),
	}
	userManagementClient, err := usermanagement.NewClient(userManagementCfg)
	if err != nil {
		slog.Error("failed to construct user-management client", "err", err)
		os.Exit(1)
	}

	// adminRole is the role string (from entity.GetUserMeResponse.Roles) that
	// grants admin privileges for registry-token and contact management.
	adminRole := mustEnv("AUTH_ADMIN_ROLE")

	authCfg := middleware.Config{
		JWKSEndpoint:          mustURL("AUTH_JWKS_ENDPOINT", "http", "https"),
		Issuer:                mustEnv("AUTH_ISSUER"),
		Audiences:             splitComma(mustEnv("AUTH_AUDIENCE")),
		ClockSkew:             5 * time.Second,
		TokenValidatorEnabled: os.Getenv("AUTH_TOKEN_VALIDATOR_ENABLED") != "false",
	}
	// One validator, shared by the REST middleware chain and the WebSocket
	// listener, so the JWKS is fetched and refreshed once per process.
	tokenValidator := middleware.NewTokenValidator(authCfg)

	userHandler := handler.NewUserHandler(entityClient, scimClient)
	projectHandler := handler.NewProjectHandler(entityClient)
	projectStatsHandler := handler.NewProjectStatsHandler(entityClient)
	caseHandler := handler.NewCaseHandler(entityClient)
	deploymentHandler := handler.NewDeploymentHandler(entityClient)
	deployedProductHandler := handler.NewDeployedProductHandler(entityClient)
	attachmentHandler := handler.NewAttachmentHandler(entityClient)
	productHandler := handler.NewProductHandler(entityClient)
	changeRequestHandler := handler.NewChangeRequestHandler(entityClient)
	callRequestHandler := handler.NewCallRequestHandler(entityClient)
	accountHandler := handler.NewAccountHandler(entityClient)
	updatesHandler := handler.NewUpdatesHandler(updatesClient)
	commentHandler := handler.NewCommentHandler(entityClient)
	productVulnerabilityHandler := handler.NewProductVulnerabilityHandler(entityClient)
	catalogHandler := handler.NewCatalogHandler(entityClient)
	timeCardHandler := handler.NewTimeCardHandler(entityClient)
	aiChatHandler := handler.NewAIChatHandler(aiChatAgentClient, entityClient)
	webSocketHandler := handler.NewWebSocketHandler(aiChatAgentWsClient, entityClient, tokenValidator, nil)
	productConsumptionHandler := handler.NewProductConsumptionHandler(productConsumptionClient, entityClient)
	globalHandler := handler.NewGlobalHandler(entityClient)
	instanceHandler := handler.NewInstanceHandler(entityClient)
	registryHandler := handler.NewRegistryHandler(entityClient, registryClient, adminRole)
	contactHandler := handler.NewContactHandler(entityClient, userManagementClient)

	// Caller-scoped project/case search — always enforced, no kill switch.
	// See handler.CallerScopeResolver: there is no bulk "projects for this
	// email" lookup anywhere upstream, so membership is checked one project
	// at a time via entity-service's own native project-contacts search
	// (the same endpoint CSM's backend already calls in production) — no
	// separate dependency beyond entityClient.
	callerScopeResolver := handler.NewCallerScopeResolver(entityClient)
	projectHandler.SetCallerScope(callerScopeResolver)
	caseHandler.SetCallerScope(callerScopeResolver)
	attachmentHandler.SetCallerScope(callerScopeResolver)
	registryHandler.SetCallerScope(callerScopeResolver)
	changeRequestHandler.SetCallerScope(callerScopeResolver)
	timeCardHandler.SetCallerScope(callerScopeResolver)
	projectStatsHandler.SetCallerScope(callerScopeResolver)
	aiChatHandler.SetCallerScope(callerScopeResolver)
	deploymentHandler.SetCallerScope(callerScopeResolver)
	instanceHandler.SetCallerScope(callerScopeResolver)
	callRequestHandler.SetCallerScope(callerScopeResolver)
	contactHandler.SetCallerScope(callerScopeResolver)
	productConsumptionHandler.SetCallerScope(callerScopeResolver)
	commentHandler.SetCallerScope(callerScopeResolver)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// GET/PATCH /users/me are only served by entity-service when it runs
	// with DATA_SOURCE=servicenow — see internal/entity/users.go.
	mux.HandleFunc("GET /users/me", userHandler.GetMe)
	mux.HandleFunc("PATCH /users/me", userHandler.PatchMe)

	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("GET /projects/{id}/filters", projectStatsHandler.GetProjectFilters)
	mux.HandleFunc("GET /projects/{id}/features", projectStatsHandler.GetProjectFeatures)
	mux.HandleFunc("GET /projects/{id}/stats", projectStatsHandler.GetProjectDashboardStats)
	mux.HandleFunc("GET /projects/{id}/stats/cases", projectStatsHandler.GetProjectCaseStats)
	mux.HandleFunc("GET /projects/{id}/stats/conversations", projectStatsHandler.GetProjectConversationStats)
	mux.HandleFunc("GET /projects/{id}/stats/support", projectStatsHandler.GetProjectSupportStats)
	mux.HandleFunc("GET /projects/{id}/stats/time-cards", projectStatsHandler.GetProjectTimeCardStats)
	mux.HandleFunc("GET /projects/{id}/stats/change-requests", projectStatsHandler.GetProjectChangeRequestStats)
	mux.HandleFunc("GET /projects/{id}/stats/usage", projectStatsHandler.GetProjectUsageStats)
	mux.HandleFunc("POST /projects/{id}/cases/time-cards/search", projectStatsHandler.SearchProjectCaseTimeCards)
	mux.HandleFunc("POST /projects/{id}/instances/search", instanceHandler.SearchProjectInstances)
	mux.HandleFunc("POST /projects/{id}/instances/metrics/search", instanceHandler.SearchProjectInstanceMetrics)
	mux.HandleFunc("POST /projects/{id}/instances/usages/search", instanceHandler.SearchProjectInstanceUsage)
	mux.HandleFunc("POST /projects/{id}/instances/stats/metrics/search", instanceHandler.SearchProjectInstanceMetricsStats)
	mux.HandleFunc("POST /projects/{id}/instances/stats/usages/search", instanceHandler.SearchProjectInstanceUsageStats)
	mux.HandleFunc("POST /projects/{id}/registry-tokens", registryHandler.CreateRegistryToken)
	mux.HandleFunc("POST /projects/{id}/registry-tokens/search", registryHandler.SearchRegistryTokens)
	mux.HandleFunc("GET /projects/{id}/integration-users", registryHandler.GetProjectIntegrationUsers)
	mux.HandleFunc("GET /projects/{id}/contacts", contactHandler.GetProjectContacts)
	mux.HandleFunc("POST /projects/{id}/contacts", contactHandler.CreateProjectContact)
	mux.HandleFunc("DELETE /projects/{id}/contacts/{email}", contactHandler.RemoveProjectContact)
	mux.HandleFunc("PATCH /projects/{id}/contacts/{email}", contactHandler.UpdateProjectContactRole)
	mux.HandleFunc("POST /projects/{id}/contacts/validate", contactHandler.ValidateProjectContact)
	mux.HandleFunc("DELETE /registry-tokens/{id}", registryHandler.DeleteRegistryToken)
	mux.HandleFunc("POST /registry-tokens/{id}/regenerate", registryHandler.RegenerateRegistryToken)

	mux.HandleFunc("POST /projects/{id}/cases/search", caseHandler.SearchCases)
	mux.HandleFunc("GET /cases/{id}", caseHandler.GetCase)
	mux.HandleFunc("POST /cases", caseHandler.CreateCase)
	mux.HandleFunc("PATCH /cases/{id}", caseHandler.PatchCase)
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/activities/search", caseHandler.SearchCaseActivities)
	mux.HandleFunc("GET /cases/{id}/attachments", caseHandler.SearchCaseAttachments)
	mux.HandleFunc("POST /cases/{id}/attachments", caseHandler.CreateCaseAttachment)
	mux.HandleFunc("GET /cases/{id}/feedback", caseHandler.GetCaseFeedback)
	mux.HandleFunc("POST /cases/{id}/feedback", caseHandler.SubmitCaseFeedback)
	mux.HandleFunc("PATCH /cases/{caseId}/attachments/{attachmentId}", caseHandler.PatchCaseAttachment)
	mux.HandleFunc("POST /cases/{caseId}/escalations", caseHandler.CreateCaseEscalation)
	mux.HandleFunc("POST /cases/{caseId}/escalations/search", caseHandler.SearchCaseEscalations)

	mux.HandleFunc("POST /projects/{id}/deployments/search", deploymentHandler.SearchDeployments)
	// POST /projects/{id}/deployments only succeeds against entity-service's
	// ServiceNow data source — see internal/entity/deployments.go.
	mux.HandleFunc("POST /projects/{id}/deployments", deploymentHandler.CreateDeployment)
	mux.HandleFunc("PATCH /projects/{projectId}/deployments/{id}", deploymentHandler.PatchDeployment)
	mux.HandleFunc("GET /deployments/{deploymentId}/attachments", deploymentHandler.SearchDeploymentAttachments)
	mux.HandleFunc("POST /deployments/{deploymentId}/attachments", deploymentHandler.CreateDeploymentAttachment)
	mux.HandleFunc("PATCH /deployments/{deploymentId}/attachments/{attachmentId}", deploymentHandler.PatchDeploymentAttachment)
	mux.HandleFunc("POST /deployments/{id}/instances/search", instanceHandler.SearchDeploymentInstances)
	mux.HandleFunc("POST /deployments/{id}/instances/metrics/search", instanceHandler.SearchDeploymentInstanceMetrics)
	mux.HandleFunc("POST /deployments/{id}/instances/usages/search", instanceHandler.SearchDeploymentInstanceUsage)
	mux.HandleFunc("POST /deployments/{id}/instances/stats/metrics/search", instanceHandler.SearchDeploymentInstanceMetricsStats)
	mux.HandleFunc("POST /deployments/{id}/instances/stats/usages/search", instanceHandler.SearchDeploymentInstanceUsageStats)

	mux.HandleFunc("POST /deployments/{deploymentId}/products/search", deployedProductHandler.SearchDeployedProducts)
	// POST/PATCH .../products only succeed against entity-service's
	// ServiceNow data source — see internal/entity/deployed_products.go.
	mux.HandleFunc("POST /deployments/{deploymentId}/products", deployedProductHandler.CreateDeployedProduct)
	mux.HandleFunc("PATCH /deployments/{deploymentId}/products/{id}", deployedProductHandler.PatchDeployedProduct)
	// POST /deployments/{deploymentId}/products/{productId}/metrics/search and
	// POST /deployments/products/{id}/instances/metrics/search cannot both be
	// registered as literal patterns: net/http.ServeMux (Go 1.22+) rejects
	// them as ambiguous (neither is more specific than the other — e.g. both
	// match "/deployments/products/products/instances/metrics/search") and
	// panics at startup. Both path shapes are genuine, distinct routes this
	// backend must expose exactly as-is, so they're merged under one
	// wildcard pattern and dispatched by
	// dispatchDeploymentsProductsMetricsSearch below instead of renaming
	// either one.
	mux.HandleFunc("POST /deployments/{seg1}/{seg2}/{seg3}/metrics/search",
		dispatchDeploymentsProductsMetricsSearch(instanceHandler, deployedProductHandler))
	mux.HandleFunc("POST /deployments/{deploymentId}/products/{productId}/metrics/usage-counts/search", deployedProductHandler.SearchDeployedProductUsageCounts)
	mux.HandleFunc("POST /deployments/products/{id}/instances/search", instanceHandler.SearchDeployedProductInstances)
	mux.HandleFunc("POST /deployments/products/{id}/instances/usages/search", instanceHandler.SearchDeployedProductInstanceUsage)
	mux.HandleFunc("POST /deployments/products/{id}/instances/stats/metrics/search", instanceHandler.SearchDeployedProductInstanceMetricsStats)
	mux.HandleFunc("POST /deployments/products/{id}/instances/stats/usages/search", instanceHandler.SearchDeployedProductInstanceUsageStats)

	mux.HandleFunc("POST /attachments", attachmentHandler.CreateAttachment)
	mux.HandleFunc("GET /attachments/{id}/content", attachmentHandler.GetAttachmentContent)
	mux.HandleFunc("GET /attachments/{id}", attachmentHandler.GetAttachment)
	mux.HandleFunc("DELETE /attachments/{id}", attachmentHandler.DeleteAttachment)

	mux.HandleFunc("GET /products", productHandler.GetProducts)
	mux.HandleFunc("POST /products/search", productHandler.SearchProducts)
	mux.HandleFunc("POST /products/{id}/versions/search", productHandler.SearchProductVersions)

	// entity-service only supports change requests and call requests on its
	// ServiceNow data source — see internal/entity/change_requests.go and
	// internal/entity/call_requests.go.
	mux.HandleFunc("POST /change-requests", changeRequestHandler.CreateChangeRequest)
	mux.HandleFunc("POST /projects/{id}/change-requests/search", changeRequestHandler.SearchChangeRequests)
	mux.HandleFunc("GET /change-requests/{id}", changeRequestHandler.GetChangeRequest)
	mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
	mux.HandleFunc("GET /change-requests/{id}/approvals", changeRequestHandler.GetChangeRequestApprovals)
	mux.HandleFunc("POST /change-requests/{id}/approvals/decision", changeRequestHandler.DecideChangeRequestApproval)

	mux.HandleFunc("POST /cases/{caseId}/call-requests", callRequestHandler.CreateCallRequest)
	mux.HandleFunc("POST /cases/{caseId}/call-requests/search", callRequestHandler.SearchCallRequests)
	mux.HandleFunc("PATCH /cases/{caseId}/call-requests/{id}", callRequestHandler.PatchCallRequest)

	mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)

	mux.HandleFunc("POST /comments", commentHandler.CreateComment)

	mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
	mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)
	mux.HandleFunc("GET /products/vulnerabilities/meta", globalHandler.GetVulnerabilityMeta)

	mux.HandleFunc("GET /metadata", globalHandler.GetMetadata)
	mux.HandleFunc("POST /search", globalHandler.GlobalSearch)

	mux.HandleFunc("POST /deployments/products/{deployedProductId}/catalogs/search", catalogHandler.SearchCatalogs)
	mux.HandleFunc("GET /catalogs/{catalogId}/items/{itemId}", catalogHandler.GetCatalogItemVariables)

	// entity-service only supports time cards on its ServiceNow data source —
	// see internal/entity/time_cards.go.
	mux.HandleFunc("POST /projects/{id}/time-cards/search", timeCardHandler.SearchTimeCards)

	// AI chat feature: case classification and KB recommendations call the
	// upstream AI chat agent directly; conversation search/messages/summary
	// mix the AI agent with entity-service's conversation/comment routes.
	// See handler.AIChatHandler and handler.WebSocketHandler's doc comments
	// for the comment createdBy override gap this works around.
	mux.HandleFunc("POST /cases/classify", aiChatHandler.ClassifyCase)
	mux.HandleFunc("POST /conversations/recommendations/search", aiChatHandler.SearchRecommendations)
	mux.HandleFunc("POST /projects/{id}/conversations/search", aiChatHandler.SearchConversations)
	mux.HandleFunc("POST /projects/{id}/conversations", aiChatHandler.CreateConversation)
	mux.HandleFunc("GET /conversations/{id}", aiChatHandler.GetConversation)
	mux.HandleFunc("PATCH /conversations/{id}", aiChatHandler.UpdateConversation)
	mux.HandleFunc("GET /conversations/{id}/messages", aiChatHandler.GetConversationMessages)
	mux.HandleFunc("POST /projects/{projectId}/conversations/{conversationId}/messages", aiChatHandler.SendConversationMessage)
	mux.HandleFunc("GET /projects/{id}/conversations/{conversationId}/summary", aiChatHandler.GetConversationSummary)

	// The product-consumption service is a separate service (not
	// entity-service) — see internal/productconsumption's package doc comment.
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", productConsumptionHandler.GetDeploymentLicense)
	mux.HandleFunc("POST /deployment-usages", productConsumptionHandler.ImportDeploymentUsage)

	mux.HandleFunc("GET /updates/product-update-levels", updatesHandler.GetProductUpdateLevels)
	mux.HandleFunc("POST /updates/levels/search", updatesHandler.SearchUpdatesBetweenUpdateLevels)

	addr := ":" + mustPort("PORT", "8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("Customer Portal Backend (v2) started", "addr", addr)

	srv := &http.Server{
		// CORS must be outermost: a preflight OPTIONS request carries no JWT,
		// so if Auth ran first it would reject every preflight with 401
		// before the browser ever saw a CORS header.
		Handler: middleware.CORS(nil)(
			middleware.SecurityHeaders(
				middleware.CorrelationID(
					middleware.AuthWithValidator(tokenValidator)(
						middleware.Logger(mux),
					),
				),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Established before the WebSocket listener binds, so that listener's setup
	// is cancellable (see lc.Listen below) and a signal arriving mid-bind is
	// caught rather than killing the process outright.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// GET /ws lives on its own listener and port, mirroring the Ballerina
	// backend's split (REST on 9090, WebSocket on 9091 — see that service's
	// .choreo/component.yaml). Two reasons it cannot share the REST server:
	//
	//  1. middleware.Auth demands an x-jwt-assertion header, which a browser
	//     cannot set on a WebSocket handshake. This chain therefore omits Auth;
	//     WebSocketHandler authenticates the token itself (see
	//     handler.userIDTokenFromRequest).
	//  2. The REST server's Read/WriteTimeout would cap the lifetime of an
	//     upgraded connection. The handler clears those deadlines after the
	//     upgrade, but only a listener without them is safe by construction.
	//
	// Each Choreo endpoint maps to one port, so this also gives the
	// customer-portal-websocket endpoint a port of its own — see
	// .choreo/component.yaml.
	wsMux := http.NewServeMux()
	wsMux.HandleFunc("GET /ws", webSocketHandler.HandleWebSocket)

	wsAddr := ":" + mustPort("WS_PORT", "8081")
	// ctx here covers only the listen operation itself (address resolution and
	// socket setup) — it does NOT tie the listener's lifetime to the context,
	// so cancelling ctx will not close wsLn. Shutdown still comes from
	// wsSrv.Shutdown below.
	var lc net.ListenConfig
	wsLn, err := lc.Listen(ctx, "tcp", wsAddr)
	if err != nil {
		slog.Error("failed to bind websocket listener", "addr", wsAddr, "err", err)
		os.Exit(1)
	}
	slog.Info("Customer Portal Backend (v2) websocket listener started", "addr", wsAddr)

	wsSrv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CorrelationID(
				middleware.Logger(wsMux),
			),
		),
		// ReadHeaderTimeout bounds the handshake itself. Read/Write/Idle
		// timeouts are deliberately unset: they would terminate a healthy but
		// idle chat session. handler.wsIdleTimeout bounds the connection
		// instead, per-frame.
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	go func() {
		if err := wsSrv.Serve(wsLn); err != nil && err != http.ErrServerClosed {
			slog.Error("websocket server exited", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Shut the WebSocket listener down first so it stops accepting new
	// upgrades while the REST server drains. Shutdown does not close
	// already-hijacked connections; those end when their peer disconnects or
	// handler.wsIdleTimeout expires.
	if err := wsSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("websocket graceful shutdown failed", "err", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("Customer Portal Backend (v2) stopped")
}

// dispatchDeploymentsProductsMetricsSearch resolves the two distinct routes
// merged under the "POST
// /deployments/{seg1}/{seg2}/{seg3}/metrics/search" pattern registered in
// main() (see the comment at that registration for why they can't be
// registered as separate literal patterns). Exactly one of the two shapes
// can match a given request: seg1=="products" identifies the
// deployed-product-scoped instances-metrics route (seg2 is the deployed
// product ID, seg3 is the literal "instances"); otherwise seg2=="products"
// identifies the deployed-product's own metrics route (seg1 is the
// deployment ID, seg3 is the product ID). Path values are injected via
// SetPathValue so the existing handler methods can read them exactly as
// they would from their own literal pattern.
func dispatchDeploymentsProductsMetricsSearch(instances *handler.InstanceHandler, deployedProducts *handler.DeployedProductHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seg1 := r.PathValue("seg1")
		seg2 := r.PathValue("seg2")
		seg3 := r.PathValue("seg3")

		if seg1 == "products" && seg3 == "instances" {
			r.SetPathValue("id", seg2)
			instances.SearchDeployedProductInstanceMetrics(w, r)
			return
		}
		if seg2 == "products" {
			r.SetPathValue("deploymentId", seg1)
			r.SetPathValue("productId", seg3)
			deployedProducts.SearchDeployedProductMetrics(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

// sanitizeURLValue trims surrounding whitespace and strips a matched pair of
// enclosing quotes from a URL taken from configuration.
//
// Both are real deployment failures, not hypotheticals: a value stored as
// `"https://host/path"` (with literal quotes) or with a leading space never
// parses as a URL at all — url.Parse stops recognising the scheme and reports
// `first path segment in URL cannot contain colon`. That took down
// POST /conversations/recommendations/search in staging with a 500 on every
// call, and the value looked perfectly correct in the config UI.
func sanitizeURLValue(v string) string {
	v = strings.TrimSpace(v)
	for len(v) >= 2 {
		first, last := v[0], v[len(v)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			v = strings.TrimSpace(v[1 : len(v)-1])
			continue
		}
		break
	}
	return v
}

// urlProblem classifies why a configured URL is unusable.
//
// Deliberately a code rather than a message built from the value: the reason is
// logged, and interpolating anything derived from configuration into a log line
// lets a newline in that value forge log entries (gosec G706). Each code maps to
// a fixed string below, so nothing input-derived is ever logged.
type urlProblem int

const (
	urlOK urlProblem = iota
	urlUnparseable
	urlNoScheme
	urlBadScheme
	urlNoHost
)

func (p urlProblem) String() string {
	switch p {
	case urlUnparseable:
		return "not a valid URL (check for stray quotes or whitespace)"
	case urlNoScheme:
		return "missing a scheme, expected http:// or https:// (or ws://, wss:// for a WebSocket URL)"
	case urlBadScheme:
		return "scheme is not accepted for this setting"
	case urlNoHost:
		return "missing a host (check for a single slash after the scheme)"
	default:
		return "ok"
	}
}

// checkURLValue reports why v is unusable as a base URL with one of schemes, or
// urlOK if it is fine.
func checkURLValue(v string, schemes ...string) urlProblem {
	u, err := url.Parse(v)
	if err != nil {
		return urlUnparseable
	}
	if u.Scheme == "" {
		return urlNoScheme
	}
	if !slices.Contains(schemes, u.Scheme) {
		return urlBadScheme
	}
	if u.Host == "" {
		return urlNoHost
	}
	return urlOK
}

// mustURL is mustEnv for a URL: it sanitizes the value, then validates it is
// absolute with an accepted scheme and a host, exiting at startup if not.
//
// Validating here rather than at first use is deliberate. A URL that cannot be
// parsed fails inside the HTTP client on every request, which surfaces as a 500
// on one endpoint while the rest of the service looks healthy — the failure is
// per-request and easily mistaken for an upstream problem. A misconfiguration
// should stop the deployment instead, the same reasoning as the JWKS panic in
// middleware.NewTokenValidator.
func mustURL(key string, schemes ...string) string {
	raw := os.Getenv(key)
	v := sanitizeURLValue(raw)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	if v != raw {
		// Worth a warning even though it now works: the stored value is still
		// malformed and the next person to copy it hits the same thing.
		slog.Warn("stripped surrounding quotes/whitespace from URL configuration", "key", key)
	}
	if problem := checkURLValue(v, schemes...); problem != urlOK {
		slog.Error("environment variable is not a usable URL", "key", key, "reason", problem.String())
		os.Exit(1)
	}
	return v
}

// optionalURL is mustURL for a value that may legitimately be unset.
func optionalURL(key string, schemes ...string) string {
	raw := os.Getenv(key)
	v := sanitizeURLValue(raw)
	if v == "" {
		return ""
	}
	if v != raw {
		slog.Warn("stripped surrounding quotes/whitespace from URL configuration", "key", key)
	}
	if problem := checkURLValue(v, schemes...); problem != urlOK {
		slog.Error("environment variable is not a usable URL", "key", key, "reason", problem.String())
		os.Exit(1)
	}
	return v
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

// mustPort returns the value of the given environment variable (or def if
// unset) as a bare port number, e.g. "8080" — not an address like ":8080" or
// "localhost:8080". Exits the process if the value isn't a valid TCP port.
func mustPort(key, def string) string {
	v := envOrDefault(key, def)
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Error("environment variable must be a plain port number (e.g. \"8080\"), not an address", "key", key, "value", v)
		os.Exit(1)
	}
	return v
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
		if _, exists := os.LookupEnv(k); !exists {
			if err := os.Setenv(k, v); err != nil {
				slog.Warn("loadDotEnv: failed to set environment variable", "key", k, "err", err)
			}
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
