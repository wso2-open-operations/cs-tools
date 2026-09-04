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
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/sftpgo"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/updates"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	dashboard.SetActive(loadDashboards())

	// Reference data is resolved once, here, and then only ever read from
	// memory: the team registry (key <-> display name <-> backing group id <->
	// platform UUID) and the assignable-role allow-list are both derivable from
	// configuration alone, so nothing about them needs an upstream call on the
	// request path.
	dir := loadDirectory()

	// All upstream service clients (entity, updates, SCIM, and future notification
	// channels) authenticate as the same OAuth2 client-credentials app; only the
	// base URL and scopes differ per service.
	oauth2ClientID := mustEnv("OAUTH2_CLIENT_ID")
	oauth2ClientSecret := mustEnv("OAUTH2_CLIENT_SECRET")
	oauth2TokenURL := mustEnv("OAUTH2_TOKEN_URL")

	customerEntityCfg := entity.CustomerEntityConfig{
		BaseURL:      mustEnv("CUSTOMER_ENTITY_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		// Scopes is optional; set CUSTOMER_ENTITY_SCOPES as a comma-separated list if required.
		Scopes: splitComma(os.Getenv("CUSTOMER_ENTITY_SCOPES")),
	}

	customerEntityClient := entity.NewCustomerEntityClient(customerEntityCfg)
	roleResolver := middleware.NewRoleResolver(customerEntityClient, 5*time.Minute)

	caseHandler := handler.NewCaseHandler(customerEntityClient)
	dashboardHandler := handler.NewDashboardHandler()
	accountHandler := handler.NewAccountHandler(customerEntityClient)
	projectHandler := handler.NewProjectHandler(customerEntityClient)
	productHandler := handler.NewProductHandler(customerEntityClient)
	deploymentHandler := handler.NewDeploymentHandler(customerEntityClient)
	changeRequestHandler := handler.NewChangeRequestHandler(customerEntityClient)
	itServiceHandler := handler.NewITServiceHandler(customerEntityClient)
	serviceOfferingHandler := handler.NewServiceOfferingHandler(customerEntityClient)
	groupHandler := handler.NewGroupHandler(customerEntityClient)
	referenceHandler := handler.NewReferenceHandler(dir)
	configurationItemHandler := handler.NewConfigurationItemHandler(customerEntityClient)
	catalogHandler := handler.NewCatalogHandler(customerEntityClient)
	timeCardHandler := handler.NewTimeCardHandler(customerEntityClient)
	productVulnerabilityHandler := handler.NewProductVulnerabilityHandler(customerEntityClient)
	conversationHandler := handler.NewConversationHandler(customerEntityClient)
	taskSlaHandler := handler.NewTaskSlaHandler(customerEntityClient)
	taskHandler := handler.NewTaskHandler(customerEntityClient)
	incidentHandler := handler.NewIncidentHandler(customerEntityClient)
	problemHandler := handler.NewProblemHandler(customerEntityClient)
	incidentTaskHandler := handler.NewIncidentTaskHandler(customerEntityClient)
	alertHandler := handler.NewAlertHandler(customerEntityClient)

	// Google Chat is not yet configured for every deployment, so its spaces
	// are read with os.Getenv (never mustEnv) — a missing or malformed value
	// only surfaces as an error the first time an alert is sent for a product
	// with no matching space.
	googleChatClient := notifications.NewGoogleChatClient(notifications.GoogleChatConfig{
		Spaces: parseGoogleChatSpaces(os.Getenv("NOTIFICATIONS_GOOGLE_CHAT_SPACES")),
	})
	notificationHandler := handler.NewNotificationHandler(googleChatClient, os.Getenv("CSM_PORTAL_WEB_BASE_URL"))

	// SFTPGo-backed attachment storage — off by default (see loadSftpgoConfig).
	// When disabled, no SFTPGO_* env var is read at all and neither the client
	// nor its routes are constructed: the existing streaming attachment
	// endpoints on caseHandler above are completely unaffected either way.
	sftpgoAttachmentStorageEnabled, sftpgoCfg := loadSftpgoConfig()
	var attachmentStorageHandler *handler.AttachmentStorageHandler
	if sftpgoAttachmentStorageEnabled {
		sftpgoClientInst := sftpgo.NewClient(sftpgoCfg)
		attachmentStorageHandler = handler.NewAttachmentStorageHandler(customerEntityClient, sftpgoClientInst)
		// Inline-image extraction on CreateCaseComment (base64 data: URIs
		// rewritten into real SFTPGo-backed attachments) shares the same
		// SFTPGo client and is gated by the same flag — see
		// CaseHandler.WithInlineImageProcessor. SN-backed comment creation is
		// unaffected: it never reaches this branch.
		caseHandler.WithInlineImageProcessor(handler.NewInlineImageProcessor(customerEntityClient, sftpgoClientInst))
	}

	updatesCfg := updates.Config{
		BaseURL:      mustEnv("UPDATES_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("UPDATES_SCOPES")),
	}
	updatesClient := updates.NewClient(updatesCfg)
	updatesHandler := handler.NewUpdatesHandler(updatesClient)

	scimCfg := scim.Config{
		BaseURL:      mustEnv("SCIM_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("SCIM_SCOPES")),
	}
	scimClient := scim.NewClient(scimCfg)
	usersHandler := handler.NewUsersHandler(scimClient, customerEntityClient, dir, sftpgoAttachmentStorageEnabled)

	authCfg := middleware.Config{
		JWKSEndpoint:          mustEnv("AUTH_JWKS_ENDPOINT"),
		Issuer:                mustEnv("AUTH_ISSUER"),
		Audiences:             splitComma(mustEnv("AUTH_AUDIENCE")),
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
	mux.HandleFunc("POST /cases/{id}/request-update", caseHandler.RequestCaseUpdate)
	mux.HandleFunc("GET /case-update-request-templates", caseHandler.GetCaseUpdateRequestTemplates)
	mux.HandleFunc("POST /cases/{id}/comments/search", caseHandler.SearchCaseComments)
	mux.HandleFunc("POST /cases/{id}/activities/search", caseHandler.SearchCaseActivities)
	mux.HandleFunc("POST /attachments", caseHandler.CreateCaseAttachment)
	mux.HandleFunc("POST /attachments/search", caseHandler.SearchCaseAttachments)
	mux.HandleFunc("GET /attachments/{id}/content", caseHandler.GetCaseAttachmentContent)
	mux.HandleFunc("DELETE /attachments/{id}", caseHandler.DeleteCaseAttachment)
	// The SFTPGo-backed attachment-storage routes only exist on the mux when
	// the feature flag is on: with it off (default), these paths are not
	// registered at all and 404, rather than existing but erroring, so
	// shipping this dark carries zero risk to the routes above.
	if attachmentStorageHandler != nil {
		mux.HandleFunc("POST /cases/{id}/attachments/upload-token", attachmentStorageHandler.MintUploadToken)
		mux.HandleFunc("POST /attachments/{id}/share", attachmentStorageHandler.CreateAttachmentShare)
		mux.HandleFunc("POST /cases/{caseId}/attachments/{attachmentId}/confirm", attachmentStorageHandler.ConfirmUpload)
	}
	mux.HandleFunc("GET /attachments/{id}", caseHandler.GetAttachment)
	mux.HandleFunc("PATCH /attachments/{id}", caseHandler.UpdateAttachment)
	mux.HandleFunc("POST /cases/{id}/call-requests", caseHandler.CreateCallRequest)
	mux.HandleFunc("POST /cases/{id}/call-requests/search", caseHandler.SearchCallRequests)
	mux.HandleFunc("POST /call-requests/search", caseHandler.SearchAllCallRequests)
	mux.HandleFunc("PATCH /cases/{caseId}/call-requests/{callRequestId}", caseHandler.PatchCallRequest)
	mux.HandleFunc("POST /cases/{id}/github-issues", caseHandler.CreateCaseGithubIssue)
	mux.HandleFunc("POST /cases/{id}/tags", caseHandler.AddCaseTag)
	mux.HandleFunc("DELETE /cases/{id}/tags/{tagId}", caseHandler.RemoveCaseTag)
	mux.HandleFunc("POST /tags/search", caseHandler.SearchTags)
	// Deprecated: the query-parameter form of tag search, kept for one release
	// so this service and its callers can be deployed independently. Remove it
	// (and CaseHandler.SearchTagsQuery) once every caller is on the POST.
	//nolint:staticcheck // SA1019: intentional one-release compatibility route; remove with the handler.
	mux.HandleFunc("GET /tags/search", caseHandler.SearchTagsQuery)
	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	mux.HandleFunc("POST /cases/aggregate", caseHandler.AggregateCases)
	mux.HandleFunc("POST /cases/feedback/search", caseHandler.SearchFeedback)
	mux.HandleFunc("POST /cases/feedback/aggregate", caseHandler.AggregateFeedback)
	mux.HandleFunc("GET /dashboards", dashboardHandler.GetDashboards)
	// Registered before the {dashboardId} wildcard purely for readability —
	// net/http's ServeMux resolves by specificity, not registration order,
	// so these literal paths win over the wildcard regardless.
	mux.HandleFunc("GET /dashboards/filter-presets", dashboardHandler.GetFilterPresets)
	mux.HandleFunc("GET /dashboards/sections", dashboardHandler.GetSharedSections)
	mux.HandleFunc("GET /dashboards/{dashboardId}", dashboardHandler.GetDashboardDetail)
	mux.HandleFunc("GET /updates/product-update-levels", updatesHandler.GetProductUpdateLevels)
	mux.HandleFunc("POST /updates/levels/search", updatesHandler.SearchUpdatesBetweenUpdateLevels)
	mux.HandleFunc("GET /users/me", usersHandler.GetMe)
	mux.HandleFunc("PATCH /users/me", usersHandler.PatchMe)
	mux.HandleFunc("POST /users/search", usersHandler.SearchUsers)
	mux.HandleFunc("GET /users/{id}", usersHandler.GetUser)
	mux.HandleFunc("POST /roles/search", referenceHandler.SearchRoles)
	mux.HandleFunc("POST /teams/search", referenceHandler.SearchTeams)
	mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)
	mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
	mux.HandleFunc("POST /accounts/{id}/contacts/search", accountHandler.SearchAccountContacts)
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	mux.HandleFunc("POST /projects/{id}/contacts/search", projectHandler.SearchProjectContacts)
	mux.HandleFunc("GET /projects/{id}/contacts/{contactId}", projectHandler.GetProjectContact)
	mux.HandleFunc("PATCH /projects/{id}", projectHandler.UpdateProject)
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
	mux.HandleFunc("GET /change-requests/{id}/approvals", changeRequestHandler.GetChangeRequestApprovals)
	mux.HandleFunc("POST /change-requests/{id}/approvals/decision", changeRequestHandler.DecideChangeRequestApproval)
	mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
	mux.HandleFunc("POST /change-requests/search", changeRequestHandler.SearchChangeRequests)
	mux.HandleFunc("POST /change-requests/aggregate", changeRequestHandler.AggregateChangeRequests)
	mux.HandleFunc("POST /services/search", itServiceHandler.SearchITServices)
	mux.HandleFunc("POST /service-offerings/search", serviceOfferingHandler.SearchServiceOfferings)
	mux.HandleFunc("POST /groups/search", groupHandler.SearchGroups)
	mux.HandleFunc("POST /configuration-items/search", configurationItemHandler.SearchConfigurationItems)
	mux.HandleFunc("POST /time-cards/search", timeCardHandler.SearchTimeCards)
	mux.HandleFunc("POST /time-cards", timeCardHandler.CreateTimeCard)
	mux.Handle("PATCH /time-cards/{id}", middleware.RequireRoles(roleResolver, "timecard_approver", "admin")(http.HandlerFunc(timeCardHandler.UpdateTimeCard)))
	mux.HandleFunc("DELETE /time-cards/{id}", timeCardHandler.DeleteTimeCard)
	mux.HandleFunc("POST /catalogs/search", catalogHandler.SearchCatalogs)
	mux.HandleFunc("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", catalogHandler.GetCatalogItemVariables)
	mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
	mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)
	mux.HandleFunc("GET /conversations/{id}/messages", conversationHandler.GetConversationMessages)
	mux.HandleFunc("POST /conversations/search", conversationHandler.SearchConversations)
	mux.HandleFunc("POST /slas/search", taskSlaHandler.SearchTaskSlas)
	mux.HandleFunc("GET /slas/{id}", taskSlaHandler.GetTaskSla)
	mux.HandleFunc("POST /cases/{caseId}/tasks/search", taskHandler.SearchCaseTasks)
	mux.HandleFunc("POST /tasks/search", taskHandler.SearchTasks)
	mux.HandleFunc("GET /tasks/{id}", taskHandler.GetTask)
	mux.HandleFunc("POST /cases/{caseId}/tasks", taskHandler.CreateCaseTask)
	mux.HandleFunc("PATCH /tasks/{id}", taskHandler.UpdateTask)
	mux.HandleFunc("POST /incidents/search", incidentHandler.SearchIncidents)
	mux.HandleFunc("POST /incidents/aggregate", incidentHandler.AggregateIncidents)
	mux.HandleFunc("POST /incidents", incidentHandler.CreateIncident)
	mux.HandleFunc("GET /incidents/{id}", incidentHandler.GetIncident)
	mux.HandleFunc("PATCH /incidents/{id}", incidentHandler.PatchIncident)
	mux.HandleFunc("POST /incidents/{id}/comments", incidentHandler.CreateIncidentComment)
	mux.HandleFunc("POST /incidents/{id}/comments/search", incidentHandler.SearchIncidentComments)
	mux.HandleFunc("POST /incidents/{id}/activities/search", incidentHandler.SearchIncidentActivities)
	mux.HandleFunc("GET /alerts/{id}", alertHandler.GetAlert)
	mux.HandleFunc("GET /smart-alerts/{id}", alertHandler.GetSmartAlert)
	mux.HandleFunc("POST /change-requests/{id}/comments", changeRequestHandler.CreateChangeRequestComment)
	mux.HandleFunc("POST /change-requests/{id}/comments/search", changeRequestHandler.SearchChangeRequestComments)
	mux.HandleFunc("POST /problems", problemHandler.CreateProblem)
	mux.HandleFunc("GET /problems/{id}", problemHandler.GetProblem)
	mux.HandleFunc("PATCH /problems/{id}", problemHandler.PatchProblem)
	mux.HandleFunc("POST /problems/search", problemHandler.SearchProblems)
	mux.HandleFunc("POST /problems/aggregate", problemHandler.AggregateProblems)
	mux.HandleFunc("GET /incident-tasks/{id}", incidentTaskHandler.GetIncidentTask)
	mux.HandleFunc("POST /incident-tasks/search", incidentTaskHandler.SearchIncidentTasks)
	mux.HandleFunc("POST /incident-tasks/aggregate", incidentTaskHandler.AggregateIncidentTasks)
	// Called manually today; not yet wired into real incident/case creation.
	mux.HandleFunc("POST /notifications/google-chat/alerts", notificationHandler.PostGoogleChatAlert)

	// Built once and reused on both listeners below: Auth() does a real JWKS
	// fetch (when TokenValidatorEnabled), so calling it a second time would
	// duplicate that startup network round-trip and double the chance of a
	// transient JWKS hiccup aborting startup, for no benefit — both
	// listeners validate the exact same tokens the exact same way.
	authMiddleware := middleware.Auth(authCfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := ":" + mustPort("PORT", "8080")

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		// SecurityHeaders must stay outermost so its headers are present on
		// every response, including a CORS preflight — CORS runs next,
		// still ahead of Auth (see middleware.CORS's doc comment): a
		// preflight OPTIONS carries no x-jwt-assertion for Auth to accept.
		// In a real deployment Choreo's gateway supplies CORS itself, so
		// this is a no-op there; it matters when the gateway isn't in the
		// path (local development, where the browser calls this listener
		// directly). CORS_ALLOWED_ORIGINS is a comma-separated allow-list;
		// unset allows any origin (see middleware.CORS on why that's safe
		// here).
		Handler: middleware.SecurityHeaders(
			middleware.CORS(splitComma(os.Getenv("CORS_ALLOWED_ORIGINS")))(
				middleware.CorrelationID(
					authMiddleware(
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

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()
	slog.Info("CSM Portal Backend started", "addr", addr)

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var srvErr error
	if err := srv.Shutdown(shutdownCtx); err != nil {
		srvErr = err
	}
	if srvErr != nil {
		slog.Error("graceful shutdown failed", "err", srvErr)
		os.Exit(1)
	}

	slog.Info("CSM Portal Backend stopped")
}

// loadDashboards builds the dashboard registry from configuration, and exits
// the process on any failure. Every failure mode here is a misconfigured
// deploy, and the alternative — starting up with dashboards silently missing
// — is exactly the class of quiet failure this codebase keeps getting bitten
// by. It is deliberately fatal even though no other endpoint depends on the
// registry.
//
// Configuration, in precedence order:
//
//	DASHBOARDS_DIR         a directory of per-dashboard *.json files. Preferred.
//	DASHBOARDS_CONFIG      DEPRECATED single-variable JSON array. Used only
//	                       when DASHBOARDS_DIR is unset.
//	DASHBOARDS_HOT_RELOAD  Any strconv.ParseBool-true value (1, t, T, TRUE,
//	                       true, True) re-reads DASHBOARDS_DIR on every request
//	                       instead of serving the startup snapshot, so editing
//	                       a definition needs no restart. Local development
//	                       only; the default (unset/false) does the startup
//	                       read once and never touches the disk again. A
//	                       non-empty unparseable value warns and is false.
//	DASHBOARD_PRESETS_FILE a JSON file of presetKey -> literal filter fragment
//	                       ({"field":...,"op":...,"values":...}), shared
//	                       across every dashboard in DASHBOARDS_DIR (a
//	                       dashboard's own top-level "filterPresets" shadows a
//	                       same-named entry here). Only consulted on the
//	                       DASHBOARDS_DIR path — the deprecated
//	                       DASHBOARDS_CONFIG path has no directory of its own
//	                       to keep a presets file alongside. Unset is legal
//	                       and means no shared presets, same as unset
//	                       DASHBOARDS_DIR itself.
//	DASHBOARD_SECTIONS_FILE a JSON file of sectionKey -> {"displayName",
//	                       "widgets": [...]}, the shared, reusable widget
//	                       sections a dashboard pulls in by name with
//	                       "includeSections" so a section like "My Work" is
//	                       authored once instead of copy-pasted per
//	                       dashboard. Same scope rules as
//	                       DASHBOARD_PRESETS_FILE: DASHBOARDS_DIR path only,
//	                       unset is legal and means no shared sections.
//
// Neither DASHBOARDS_DIR nor DASHBOARDS_CONFIG set is legal and yields no
// dashboards: a deployment that has not configured any must still start and
// serve every other endpoint.
func loadDashboards() *dashboard.Registry {
	dir := strings.TrimSpace(os.Getenv("DASHBOARDS_DIR"))
	if dir == "" {
		dashboards, err := dashboard.ParseDashboardsConfig(os.Getenv("DASHBOARDS_CONFIG"))
		if err != nil {
			slog.Error("invalid DASHBOARDS_CONFIG", "err", err)
			os.Exit(1)
		}
		return dashboard.NewStaticRegistry(dashboards)
	}

	presetsFile := strings.TrimSpace(os.Getenv("DASHBOARD_PRESETS_FILE"))
	sectionsFile := strings.TrimSpace(os.Getenv("DASHBOARD_SECTIONS_FILE"))

	// ParseBool rather than a "true" string compare: the latter silently reads
	// 1, yes and on as OFF, and never reports a typo at all -- the operator
	// sets the variable, sees no hot reload and no log line, and has nothing to
	// go on. Unparseable is a warning, not fatal: hot reload is a local-dev
	// convenience, and refusing to boot over it would be worse than defaulting
	// to the safe (off) value.
	hotReload := false
	if raw := strings.TrimSpace(os.Getenv("DASHBOARDS_HOT_RELOAD")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			slog.Warn("DASHBOARDS_HOT_RELOAD is not a boolean; treating it as false",
				"value", raw, "expected", "1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False")
		}
		hotReload = parsed
	}
	registry, err := dashboard.NewDirRegistry(dir, hotReload, presetsFile, sectionsFile)
	if err != nil {
		slog.Error("invalid dashboard definitions", "dir", dir, "presetsFile", presetsFile, "sectionsFile", sectionsFile, "err", err)
		os.Exit(1)
	}
	if hotReload {
		slog.Warn("DASHBOARDS_HOT_RELOAD is on: dashboard definitions are re-read from disk on every request. Intended for local development only",
			"dir", dir)
	}
	slog.Info("loaded dashboard definitions", "dir", dir, "presetsFile", presetsFile, "count", len(registry.Dashboards()), "hotReload", hotReload)
	return registry
}

// loadDirectory resolves the reference catalogues from environment
// configuration, once, at startup:
//
//	CSM_TEAM_REGISTRY  the team registry as
//	                   "teamKey|Display Name|FAMILY|creGroupId|sreGroupId" rows
//	                   separated by commas, where FAMILY is one of cre-abt,
//	                   cre, sre-abt or sre (case insensitive) and FAMILY,
//	                   creGroupId, and sreGroupId are all optional. Unset means
//	                   no teams are configured; there is deliberately no
//	                   default, because team names are organisation vocabulary
//	                   that must not be committed here.
//	CSM_USER_ROLES     the assignable-role allow-list, comma separated. Unset
//	                   falls back to the committed default list.
//
// A malformed row is fatal and names the offending row. It has to be: a team
// silently dropped from the registry does not error anywhere -- it just removes
// that team from every picker and resolves its members to no team at all, which
// surfaces days later as "why is my dashboard wrong". An empty registry is
// legal and only warned about, so a deployment that has not configured one yet
// still starts and serves every other endpoint.
func loadDirectory() *directory.Directory {
	teams, err := directory.ParseTeamRegistry(os.Getenv("CSM_TEAM_REGISTRY"))
	if err != nil {
		slog.Error("invalid CSM_TEAM_REGISTRY", "err", err)
		os.Exit(1)
	}
	roles, err := directory.ParseRoles(os.Getenv("CSM_USER_ROLES"))
	if err != nil {
		slog.Error("invalid CSM_USER_ROLES", "err", err)
		os.Exit(1)
	}

	dir, err := directory.New(teams, roles)
	if err != nil {
		slog.Error("invalid reference configuration", "err", err)
		os.Exit(1)
	}

	if dir.TeamCount() == 0 {
		slog.Warn("team registry is empty: the team catalogue and every team filter will return nothing")
	}
	slog.Info("resolved reference catalogues", "teams", dir.TeamCount(), "roles", dir.RoleCount())
	return dir
}

// loadSftpgoConfig resolves the SFTPGo-backed attachment-storage feature
// flag and, only when it is on, the client configuration it needs:
//
//	SFTPGO_ATTACHMENT_STORAGE_ENABLED  Any strconv.ParseBool-true value (1, t,
//	                                   T, TRUE, true, True). Off by default —
//	                                   unset, empty, or any other value keeps
//	                                   this feature dark and every other
//	                                   env var below unread. This mirrors
//	                                   DASHBOARDS_HOT_RELOAD's parsing: an
//	                                   unparseable non-empty value is a
//	                                   warning, not fatal, and defaults to off.
//	SFTPGO_BASE_URL                    SFTPGo's REST API base URL. Required
//	                                   when the flag is on.
//	SFTPGO_PUBLIC_BASE_URL             Public host for constructing share
//	                                   URLs, e.g. when SFTPGo's WebClient
//	                                   share pages are fronted separately
//	                                   from its REST API. Optional; defaults
//	                                   to SFTPGO_BASE_URL when unset.
//
// Returns (false, zero Config) when the flag is off, so the caller never
// touches the returned Config in that case.
func loadSftpgoConfig() (bool, sftpgo.Config) {
	enabled := false
	if raw := strings.TrimSpace(os.Getenv("SFTPGO_ATTACHMENT_STORAGE_ENABLED")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			slog.Warn("SFTPGO_ATTACHMENT_STORAGE_ENABLED is not a boolean; treating it as false",
				"value", raw, "expected", "1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False")
		}
		enabled = parsed
	}
	if !enabled {
		return false, sftpgo.Config{}
	}

	slog.Info("SFTPGO_ATTACHMENT_STORAGE_ENABLED is on: the SFTPGo-backed attachment-storage endpoints are active")
	baseURL := mustHTTPSURL("SFTPGO_BASE_URL", mustEnv("SFTPGO_BASE_URL"))
	publicBaseURL := baseURL
	if raw := os.Getenv("SFTPGO_PUBLIC_BASE_URL"); raw != "" {
		publicBaseURL = mustHTTPSURL("SFTPGO_PUBLIC_BASE_URL", raw)
	}
	return true, sftpgo.Config{
		BaseURL:       baseURL,
		PublicBaseURL: publicBaseURL,
	}
}

// mustHTTPSURL validates value via validateHTTPSURL, exiting the process with
// a logged error if it is invalid. Both SFTPGO_BASE_URL and
// SFTPGO_PUBLIC_BASE_URL are used to build requests/URLs that carry the
// caller's email and raw gateway JWT (see internal/sftpgo.Client.MintToken)
// or are handed to end users as a public download link (see
// internal/sftpgo.Client.PublicShareURL), so a non-HTTPS or spoofed-looking
// value here is a credential-leak/MITM risk, not just a misconfiguration —
// refuse to start rather than proceed with it.
func mustHTTPSURL(key, value string) string {
	if err := validateHTTPSURL(value); err != nil {
		// Deliberately omit the raw value from this log line: it may carry
		// embedded userinfo (e.g. "https://user:pass@host"), which would
		// otherwise write a credential straight into the startup log.
		slog.Error("invalid environment variable", "key", key, "err", err)
		os.Exit(1)
	}
	return value
}

// validateHTTPSURL reports an error unless value parses as a URL with scheme
// "https", a non-empty host, no embedded userinfo (e.g.
// "https://user:pass@host/...", which could indicate a misconfigured or
// spoofed URL), and no path/query/fragment beyond an empty or bare "/" path.
// The path restriction matters beyond cosmetics: internal/sftpgo.Client
// builds request URLs by plain string concatenation (baseURL +
// "/api/v2/user/token", etc.), so a configured value with a path component
// (e.g. "https://host/api") would silently double up into
// "https://host/api/api/v2/user/token" rather than erroring.
func validateHTTPSURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("not a valid URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("must use the https scheme, got %q", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return errors.New("must include a host (e.g. \"https://host/...\")")
	}
	if parsed.User != nil {
		return errors.New("must not contain embedded userinfo (e.g. \"https://user:pass@host/...\")")
	}
	if path := parsed.EscapedPath(); path != "" && path != "/" {
		return fmt.Errorf("must not include a path (got %q); this value is concatenated with API paths, e.g. \"https://host\" not \"https://host/api\"", path)
	}
	if parsed.RawQuery != "" {
		return errors.New("must not include a query string")
	}
	if parsed.Fragment != "" {
		return errors.New("must not include a fragment")
	}
	return nil
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

func parseGoogleChatSpaces(raw string) []notifications.GoogleChatSpace {
	if raw == "" {
		return nil
	}
	var spaces []notifications.GoogleChatSpace
	if err := json.Unmarshal([]byte(raw), &spaces); err != nil {
		slog.Error("failed to parse NOTIFICATIONS_GOOGLE_CHAT_SPACES; Google Chat alerts will be unavailable", "err", err)
		return nil
	}
	return spaces
}
