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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmintegration"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmnotification"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/googledrive"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg"
	plgconfig "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/config"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/risk"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
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

	// Real role name -> identity-provider role id mapping (ASGARDEO_ROLE_IDS),
	// a JSON object string, e.g.
	// {"example-timecard-approver-role":"11111111-1111-1111-1111-111111111111"}.
	// Keyed by the real role name (the same strings AUTH_<ROLE>_ROLES already
	// lists), not an invented portal-role key -- see directory.ParseRoleIDs's
	// own doc comment for why. Used by handlers that need a role's real
	// membership via the SCIM operations service (GET
	// /users/time-card-approvers, POST /users' own grant, GET
	// /roles/grantable below). Optional and empty by default: a real role
	// name with no entry here just means that role has no SCIM-backed
	// feature wired up in this deployment.
	roleIDsByName, err := directory.ParseRoleIDs(os.Getenv("ASGARDEO_ROLE_IDS"))
	if err != nil {
		slog.Error("invalid ASGARDEO_ROLE_IDS", "err", err)
		os.Exit(1)
	}

	reqTimeouts, err := loadTimeouts(os.Getenv)
	if err != nil {
		slog.Error("invalid timeout configuration", "err", err)
		os.Exit(1)
	}

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
		// Timeout is ENTITY_SERVICE_TIMEOUT (default 60s).
		Timeout: reqTimeouts.EntityService,
	}

	customerEntityClient := entity.NewCustomerEntityClient(customerEntityCfg)

	caseHandler := handler.NewCaseHandler(customerEntityClient)
	// Optional: with the engineering entity service configured, "Open Git issue"
	// (POST /cases/{id}/github-issues) files the issue through it rather than
	// forwarding to the entity service. It authenticates as the same shared
	// OAuth2 app as every other upstream; only its base URL and scopes are its
	// own. Unset keeps the entity-service path exactly as it was.
	// engineeringEntityClient is also read by GET /health/dependencies below,
	// nil the same way it's unset here when ENGINEERING_ENTITY_BASE_URL is unset.
	var engineeringEntityClient *entity.EngineeringEntityClient
	if engineeringBaseURL := strings.TrimSpace(os.Getenv("ENGINEERING_ENTITY_BASE_URL")); engineeringBaseURL != "" {
		engineeringBaseURL = mustHTTPSBaseURL("ENGINEERING_ENTITY_BASE_URL", engineeringBaseURL)
		engineeringEntityClient = entity.NewEngineeringEntityClient(entity.EngineeringEntityConfig{
			BaseURL:      engineeringBaseURL,
			TokenURL:     oauth2TokenURL,
			ClientID:     oauth2ClientID,
			ClientSecret: oauth2ClientSecret,
			Scopes:       splitComma(os.Getenv("ENGINEERING_ENTITY_SCOPES")),
		})
		caseHandler.WithEngineeringClient(engineeringEntityClient)
		slog.Info("GitHub issues are created through the engineering entity service")
	}
	accountHandler := handler.NewAccountHandler(customerEntityClient)
	projectHandler := handler.NewProjectHandler(customerEntityClient)
	teamHandler := handler.NewTeamHandler(customerEntityClient)
	announcementExcludedProjectKeys := loadAnnouncementExcludedProjectKeys()
	validateAnnouncementDataSourceCompatibility(loadCustomerEntityDataSource(), announcementExcludedProjectKeys)
	announcementHandler := handler.NewAnnouncementHandler(customerEntityClient, announcementExcludedProjectKeys)
	announcementRequestHandler := handler.NewAnnouncementRequestHandler(customerEntityClient, announcementExcludedProjectKeys)
	announcementRegistryHandler := handler.NewAnnouncementRegistryHandler(customerEntityClient)
	productHandler := handler.NewProductHandler(customerEntityClient)
	deploymentHandler := handler.NewDeploymentHandler(customerEntityClient)
	kbArticleHandler := handler.NewKBArticleHandler(customerEntityClient)
	kbAdminHandler := handler.NewKBAdminHandler(customerEntityClient)
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
	outageHandler := handler.NewOutageHandler(customerEntityClient)
	commentHandler := handler.NewCommentHandler(customerEntityClient)

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

	// One guard authorises every route below (including /spl/*) and also
	// backs the permissions GET /users/me reports, so the two cannot drift
	// apart. Built before the SPL block below since its SPL handlers need
	// it too. accessCfg is also kept (not just the guard built from it) since
	// grantableRoles below needs the same AUTH_<ROLE>_ROLES lists directly.
	accessCfg := loadAccessConfig()
	accessGuard := handler.NewAccessGuard(accessCfg)

	// Which portal roles CreateUser may grant via SCIM when an admin adds a
	// new user through the webapp, each already resolved to its real role ID
	// -- see handler.ResolveGrantableRoles's own doc comment. Empty when
	// ASGARDEO_ROLE_IDS configures none of AUTH_<ROLE>_ROLES' real role
	// names, in which case GET /roles/grantable reports none and CreateUser
	// rejects any grantRoles value as unknown.
	grantableRoles := handler.ResolveGrantableRoles(accessCfg, roleIDsByName)

	// SupportPortalLite — off by default; see loadViewerConfig. Ported
	// from digiops-cs/apps/support-portal-lite's Ballerina backend, which is
	// being retired.
	viewerEnabled, viewerCfg := loadViewerConfig()
	var viewerHandlers *viewerHandlerSet
	if viewerEnabled {
		salesEntityClient := entity.NewSalesEntityClient(entity.SalesEntityConfig{
			BaseURL:      viewerCfg.salesEntityBaseURL,
			TokenURL:     oauth2TokenURL,
			ClientID:     oauth2ClientID,
			ClientSecret: oauth2ClientSecret,
		})
		snClient := servicenow.NewClient(servicenow.Config{
			BaseURL:              viewerCfg.snHost,
			Username:             viewerCfg.snUsername,
			Password:             viewerCfg.snPassword,
			EscalationTemplateID: viewerCfg.snEscalationTemplateID,
			TeamScheduleURL:      viewerCfg.teamScheduleURL,
		})
		driveClient := googledrive.NewClient(googledrive.Config{
			ClientID:     viewerCfg.driveClientID,
			ClientSecret: viewerCfg.driveClientSecret,
			RefreshToken: viewerCfg.driveRefreshToken,
		})
		// A live DB ping happens here, unlike every other client above —
		// this backend's standing convention (see loadDashboards,
		// loadDirectory) is that a broken required integration fails startup
		// loudly rather than serving traffic it cannot actually handle.
		// risk.NewClient returns a typed-nil client on a failed ping if this
		// were ignored, and NewCustomerHealthHandler would then store that
		// nil client in its risk interface -- Customer Health routes would
		// dispatch to a nil receiver instead of failing at startup where the
		// cause is obvious. A 30s deadline bounds the ping so a hung network
		// doesn't hang startup forever.
		riskCtx, riskCancel := context.WithTimeout(context.Background(), 30*time.Second)
		riskClient, err := risk.NewClient(riskCtx, risk.Config{DSN: viewerCfg.riskMySQLDSN})
		riskCancel()
		if err != nil {
			slog.Error("failed to connect to SPL_RISK_MYSQL_DSN", "err", err)
			os.Exit(1)
		}

		// Accounts/projects/cases/team-members read/search/comment paths used
		// to have their own Postgres translation layer here, wrapping
		// customerEntityClient into a ServiceNow-shaped response for SPL's
		// frontend. All four merged onto CS Portal's own /accounts,
		// /projects, /cases, and /teams/{id}/members routes below instead,
		// now that SPL's data source for them is the exact same
		// entity-service data those routes already serve raw, with no
		// ServiceNow-shape translation left to justify a second, parallel
		// /spl/* contract. Only attachments (no entity-service storage path)
		// and account escalations (CreateEscalation is an explicit stub on
		// this data source) remain ServiceNow-backed and SPL-specific.
		postgresLookups := handler.NewPostgresLookupsClient(customerEntityClient, snClient)
		postgresReports := handler.NewPostgresReportsClient(customerEntityClient, snClient)
		postgresUsageMetrics := handler.NewPostgresUsageMetricsClient(customerEntityClient)

		viewerHandlers = &viewerHandlerSet{
			cases:          handler.NewViewerCaseHandler(snClient, accessGuard),
			reports:        handler.NewReportsHandler(postgresReports, accessGuard),
			schedule:       handler.NewViewerScheduleHandler(snClient, accessGuard, viewerCfg.teamScheduleURL),
			attachments:    handler.NewAttachmentsHandler(snClient, accessGuard),
			lookups:        handler.NewLookupsHandler(postgresLookups, accessGuard),
			usageMetrics:   handler.NewUsageMetricsHandler(postgresUsageMetrics, accessGuard),
			files:          handler.NewFilesHandler(driveClient, accessGuard),
			customerHealth: handler.NewCustomerHealthHandler(riskClient, snClient, accessGuard),
			userInfo:       handler.NewUserInfoHandler(customerEntityClient, accessGuard),
			userScan:       handler.NewSplUserScanHandler(salesEntityClient, customerEntityClient, accessGuard),
			accountEsc:     handler.NewViewerAccountHandler(snClient, accessGuard),
		}
		slog.Info("SPL_ENABLED is on: SupportPortalLite's /spl/* endpoints are active")
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

	// csm-notification-service and csm-integration-service are only used
	// today to back GET /health/dependencies below — this backend has no
	// other reason to call either directly (notifications and Event Hub
	// publishing both live in entity-service/csm-notification-service now,
	// see this file's own "Upstream service modules" note in CLAUDE.md).
	// Both base URLs are optional, same as ENGINEERING_ENTITY_BASE_URL above:
	// an environment that hasn't wired one yet just reports that dependency
	// as "not_configured" rather than failing startup.
	var notificationPinger handler.HealthPinger
	if v := strings.TrimSpace(os.Getenv("CSM_NOTIFICATION_SERVICE_BASE_URL")); v != "" {
		v = mustHTTPSBaseURL("CSM_NOTIFICATION_SERVICE_BASE_URL", v)
		notificationPinger = csmnotification.NewClient(csmnotification.Config{
			BaseURL:      v,
			TokenURL:     oauth2TokenURL,
			ClientID:     oauth2ClientID,
			ClientSecret: oauth2ClientSecret,
			Scopes:       splitComma(os.Getenv("CSM_NOTIFICATION_SERVICE_SCOPES")),
		})
	}
	var integrationPinger handler.HealthPinger
	if v := strings.TrimSpace(os.Getenv("CSM_INTEGRATION_SERVICE_BASE_URL")); v != "" {
		v = mustHTTPSBaseURL("CSM_INTEGRATION_SERVICE_BASE_URL", v)
		integrationPinger = csmintegration.NewClient(csmintegration.Config{
			BaseURL:      v,
			TokenURL:     oauth2TokenURL,
			ClientID:     oauth2ClientID,
			ClientSecret: oauth2ClientSecret,
			Scopes:       splitComma(os.Getenv("CSM_INTEGRATION_SERVICE_SCOPES")),
		})
	}
	// engineeringPinger is declared as the interface type directly (never a
	// *entity.EngineeringEntityClient variable passed straight through) so a
	// nil engineeringEntityClient yields a true nil interface here, not a
	// non-nil interface boxing a nil pointer — the same typed-nil pitfall
	// noted on notificationPinger/integrationPinger above.
	var engineeringPinger handler.HealthPinger
	if engineeringEntityClient != nil {
		engineeringPinger = engineeringEntityClient
	}
	healthHandler := handler.NewHealthHandler(scimClient, updatesClient, notificationPinger, integrationPinger, engineeringPinger)

	// timecardApproverRoleIDs is optional: empty means none of
	// AUTH_TIMECARD_APPROVER_ROLES' real role names has a configured ID, in
	// which case GetTimeCardApprovers itself returns 404 rather than the
	// route going unregistered -- see its own route registration below for
	// why. More than one ID is possible: AUTH_TIMECARD_APPROVER_ROLES can name
	// several real role names, each with its own configured ID, and an
	// approver holding any one of them must be listed. Derived from
	// grantableRoles (the same resolution CreateUser's own grant uses) rather
	// than a second, parallel lookup.
	timecardApproverRoleIDs := handler.RoleIDsForKey(grantableRoles, "timecard_approver")
	usersHandler := handler.NewUsersHandler(scimClient, customerEntityClient, dir, sftpgoAttachmentStorageEnabled, timecardApproverRoleIDs).
		WithAccessGuard(accessGuard).
		WithGrantableRoles(grantableRoles)
	grantableRolesHandler := handler.NewGrantableRolesHandler(grantableRoles)
	dashboardHandler := handler.NewDashboardHandler(accessGuard)
	caseHandler = caseHandler.WithAccessGuard(accessGuard)
	timeCardHandler = timeCardHandler.WithAccessGuard(accessGuard)
	incidentHandler = incidentHandler.WithAccessGuard(accessGuard)
	changeRequestHandler = changeRequestHandler.WithAccessGuard(accessGuard)

	authCfg := middleware.Config{
		JWKSEndpoint:          mustEnv("AUTH_JWKS_ENDPOINT"),
		Issuer:                mustEnv("AUTH_ISSUER"),
		Audiences:             splitComma(mustEnv("AUTH_AUDIENCE")),
		ClockSkew:             5 * time.Second,
		TokenValidatorEnabled: os.Getenv("AUTH_TOKEN_VALIDATOR_ENABLED") != "false",
	}

	// Every route goes through route(), which takes the permission it needs as
	// a required argument: there is no default, so a new route cannot be
	// registered without someone deciding who may call it. /health and
	// /health/dependencies are the two exceptions and are exempt in the Auth
	// middleware too.
	mux := http.NewServeMux()
	route := func(pattern string, perm handler.Permission, h http.HandlerFunc) {
		mux.HandleFunc(pattern, accessGuard.Require(perm, h))
	}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// GET /health/dependencies is a second, exempt-from-auth probe alongside
	// GET /health above, not a route requiring a permission — see
	// HealthHandler's own doc comment for why the two are kept separate.
	mux.HandleFunc("GET /health/dependencies", healthHandler.GetHealthDependencies)
	route("POST /cases", handler.PermWrite, caseHandler.CreateCase)
	route("GET /cases/{id}", handler.PermViewSharedEntity, caseHandler.GetCase)
	route("PATCH /cases/{id}", handler.PermWrite, caseHandler.PatchCase)
	// PermCreateWorkNote, not PermWrite -- the route-level floor is
	// deliberately broader (includes worknote_creator) since a work_note is
	// a narrower action than every other write this handler's siblings
	// guard; CreateCaseComment itself requires full PermWrite for any
	// comment that isn't a work_note -- see PermCreateWorkNote's own doc
	// comment.
	route("POST /cases/{id}/comments", handler.PermCreateWorkNote, caseHandler.CreateCaseComment)
	route("POST /cases/{id}/request-update", handler.PermWrite, caseHandler.RequestCaseUpdate)
	route("GET /case-update-request-templates", handler.PermView, caseHandler.GetCaseUpdateRequestTemplates)
	route("POST /cases/{id}/comments/search", handler.PermViewSharedEntity, caseHandler.SearchCaseComments)
	// Generic comment edit/delete — applies to a comment by id regardless of
	// which aggregate (case, change request, incident, ...) it was created
	// under. Case, incident and change-request comments are PermWrite (see
	// backend CLAUDE.md's Access control section); this is the same
	// underlying resource.
	route("PATCH /comments/{id}", handler.PermWrite, commentHandler.UpdateComment)
	route("DELETE /comments/{id}", handler.PermWrite, commentHandler.DeleteComment)
	route("POST /cases/{id}/activities/search", handler.PermView, caseHandler.SearchCaseActivities)
	route("GET /cases/{id}/escalations", handler.PermView, caseHandler.GetCaseEscalations)
	route("POST /cases/{id}/escalations", handler.PermEscalate, caseHandler.CreateCaseEscalation)
	route("POST /attachments", handler.PermWrite, caseHandler.CreateCaseAttachment)
	route("POST /attachments/search", handler.PermView, caseHandler.SearchCaseAttachments)
	route("GET /attachments/{id}/content", handler.PermDownloadAttachment, caseHandler.GetCaseAttachmentContent)
	route("DELETE /attachments/{id}", handler.PermWrite, caseHandler.DeleteCaseAttachment)
	// The SFTPGo-backed attachment-storage routes only exist on the mux when
	// the feature flag is on: with it off (default), these paths are not
	// registered at all and 404, rather than existing but erroring, so
	// shipping this dark carries zero risk to the routes above.
	if attachmentStorageHandler != nil {
		route("POST /cases/{id}/attachments/upload-token", handler.PermWrite, attachmentStorageHandler.MintUploadToken)
		route("POST /attachments/{id}/share", handler.PermDownloadAttachment, attachmentStorageHandler.CreateAttachmentShare)
		route("POST /cases/{caseId}/attachments/{attachmentId}/confirm", handler.PermWrite, attachmentStorageHandler.ConfirmUpload)
	}
	route("GET /attachments/{id}", handler.PermView, caseHandler.GetAttachment)
	route("PATCH /attachments/{id}", handler.PermWrite, caseHandler.UpdateAttachment)
	route("POST /cases/{id}/call-requests", handler.PermWrite, caseHandler.CreateCallRequest)
	route("POST /cases/{id}/call-requests/search", handler.PermView, caseHandler.SearchCallRequests)
	route("POST /call-requests/search", handler.PermView, caseHandler.SearchAllCallRequests)
	route("PATCH /cases/{caseId}/call-requests/{callRequestId}", handler.PermWrite, caseHandler.PatchCallRequest)
	route("POST /cases/{id}/github-issues", handler.PermWrite, caseHandler.CreateCaseGithubIssue)
	route("POST /cases/{id}/tags", handler.PermWrite, caseHandler.AddCaseTag)
	route("DELETE /cases/{id}/tags/{tagId}", handler.PermWrite, caseHandler.RemoveCaseTag)
	route("POST /tags/search", handler.PermView, caseHandler.SearchTags)
	// Deprecated: the query-parameter form of tag search, kept for one release
	// so this service and its callers can be deployed independently. Remove it
	// (and CaseHandler.SearchTagsQuery) once every caller is on the POST.
	//nolint:staticcheck // SA1019: intentional one-release compatibility route; remove with the handler.
	route("GET /tags/search", handler.PermView, caseHandler.SearchTagsQuery)
	route("POST /cases/search", handler.PermViewSharedEntity, caseHandler.SearchCases)
	route("POST /cases/aggregate", handler.PermView, caseHandler.AggregateCases)
	route("POST /cases/feedback/search", handler.PermView, caseHandler.SearchFeedback)
	route("POST /cases/feedback/aggregate", handler.PermView, caseHandler.AggregateFeedback)
	route("GET /dashboards", handler.PermView, dashboardHandler.GetDashboards)
	// Registered before the {dashboardId} wildcard purely for readability —
	// net/http's ServeMux resolves by specificity, not registration order,
	// so these literal paths win over the wildcard regardless.
	route("GET /dashboards/filter-presets", handler.PermView, dashboardHandler.GetFilterPresets)
	route("GET /dashboards/sections", handler.PermView, dashboardHandler.GetSharedSections)
	route("GET /dashboards/{dashboardId}", handler.PermView, dashboardHandler.GetDashboardDetail)
	route("GET /updates/product-update-levels", handler.PermTimeCardsAndUpdates, updatesHandler.GetProductUpdateLevels)
	route("POST /updates/levels/search", handler.PermTimeCardsAndUpdates, updatesHandler.SearchUpdatesBetweenUpdateLevels)
	route("GET /users/me", handler.PermAuthenticated, usersHandler.GetMe)
	route("PATCH /users/me", handler.PermAuthenticated, usersHandler.PatchMe)
	route("GET /users/me/saved-filter-views", handler.PermAuthenticated, usersHandler.ListSavedFilterViews)
	route("PATCH /users/me/saved-filter-views", handler.PermAuthenticated, usersHandler.SaveSavedFilterView)
	route("DELETE /users/me/saved-filter-views", handler.PermAuthenticated, usersHandler.DeleteSavedFilterView)
	route("POST /users/me/saved-filter-views/reorder", handler.PermAuthenticated, usersHandler.ReorderSavedFilterView)
	route("POST /users/search", handler.PermView, usersHandler.SearchUsers)
	// Batch id-to-name lookup for the KB article lists (author and
	// reviewer columns). Not /users/search with an id filter: that filter
	// is ServiceNow-only and 400s against Postgres.
	route("POST /users/by-ids", handler.PermView, usersHandler.GetUsersByIDs)
	route("GET /users/{id}", handler.PermView, usersHandler.GetUser)
	route("POST /users", handler.PermAdmin, usersHandler.CreateUser)
	// Registered unconditionally, even when timecardApproverRoleIDs is empty:
	// GetTimeCardApprovers itself returns 404 when disabled. Registering it
	// only when configured would instead let the request fall through to the
	// wildcard GET /users/{id} above, which rejects the literal path segment
	// "time-card-approvers" as an invalid UUID with 400, not a clean 404.
	route("GET /users/time-card-approvers", handler.PermView, usersHandler.GetTimeCardApprovers)
	route("POST /roles/search", handler.PermView, referenceHandler.SearchRoles)
	// Admin-only: the portal roles an admin may grant a new user via POST
	// /users' own grantRoles field -- the Add User dialog's only caller.
	route("GET /roles/grantable", handler.PermAdmin, grantableRolesHandler.GetGrantableRoles)
	route("POST /teams/search", handler.PermView, referenceHandler.SearchTeams)
	route("GET /teams/{id}/members", handler.PermViewSharedEntity, teamHandler.GetTeamMembers)
	route("GET /accounts/{id}", handler.PermViewSharedEntity, accountHandler.GetAccount)
	// Admin-only: CRE/SRE team is a temporary override of ServiceNow's own
	// value (see AccountService.UpdateAccountTeams's doc comment) — no other
	// staff role should be able to set it.
	route("PATCH /accounts/{id}", handler.PermAdmin, accountHandler.UpdateAccountTeams)
	route("POST /accounts/search", handler.PermViewSharedEntity, accountHandler.SearchAccounts)
	route("POST /accounts/{id}/contacts/search", handler.PermView, accountHandler.SearchAccountContacts)
	route("GET /projects/{id}", handler.PermViewSharedEntity, projectHandler.GetProject)
	route("GET /projects/{id}/metadata", handler.PermView, projectHandler.GetProjectMetadata)
	route("POST /projects/search", handler.PermViewSharedEntity, projectHandler.SearchProjects)
	route("POST /announcements/audience/search", handler.PermView, announcementHandler.SearchCustomerAnnouncementAudience)
	route("GET /announcements/audience/excluded-project-keys", handler.PermView, announcementHandler.GetExcludedProjectKeys)
	route("POST /announcement-requests", handler.PermWrite, announcementRequestHandler.CreateAnnouncementRequest)
	route("GET /announcement-requests/{id}", handler.PermView, announcementRequestHandler.GetAnnouncementRequest)
	route("POST /announcement-requests/search", handler.PermView, announcementRequestHandler.SearchAnnouncementRequests)
	route("POST /announcements/registry/search", handler.PermView, announcementRegistryHandler.SearchAnnouncementRegistry)
	route("PATCH /announcement-requests/{id}", handler.PermWrite, announcementRequestHandler.UpdateAnnouncementRequest)
	route("POST /announcement-requests/{id}/dry-run", handler.PermWrite, announcementRequestHandler.RecordAnnouncementRequestDryRun)
	route("POST /announcement-requests/{id}/submit", handler.PermWrite, announcementRequestHandler.SubmitAnnouncementRequest)
	route("POST /announcement-requests/{id}/approve", handler.PermWrite, announcementRequestHandler.ApproveAnnouncementRequest)
	route("POST /announcement-requests/{id}/schedule", handler.PermWrite, announcementRequestHandler.ScheduleAnnouncementRequest)
	route("POST /announcement-requests/{id}/publish", handler.PermWrite, announcementRequestHandler.PublishAnnouncementRequest)
	route("POST /announcement-requests/{id}/updates", handler.PermWrite, announcementRequestHandler.CreateAnnouncementRequestUpdate)
	route("GET /announcement-requests/{id}/updates", handler.PermView, announcementRequestHandler.ListAnnouncementRequestUpdates)
	route("POST /announcement-requests/{id}/deliveries", handler.PermWrite, announcementRequestHandler.RecordAnnouncementRequestDeliveries)
	route("GET /announcement-requests/{id}/deliveries", handler.PermView, announcementRequestHandler.ListAnnouncementRequestDeliveries)
	route("POST /projects/{id}/contacts/search", handler.PermViewSharedEntity, projectHandler.SearchProjectContacts)
	route("GET /projects/{id}/contacts/{contactId}", handler.PermView, projectHandler.GetProjectContact)
	// Customer-onboarding status per project contact — off by default (see
	// loadOnboardingStatusEnabled). When off the handler is not constructed
	// and the route is not registered, so the path 404s like any unknown one
	// and nothing else in this backend changes.
	if loadOnboardingStatusEnabled() {
		onboardingStepHandler := handler.NewOnboardingStepHandler(customerEntityClient)
		route("GET /projects/{id}/onboarding-steps", handler.PermView, onboardingStepHandler.GetProjectOnboardingSteps)
	}
	route("PATCH /projects/{id}", handler.PermWrite, projectHandler.UpdateProject)
	route("POST /products/search", handler.PermView, productHandler.SearchProducts)
	route("GET /products/github-repo", handler.PermView, productHandler.GetProductRepoMapping)
	route("POST /products/{id}/versions/search", handler.PermView, productHandler.SearchProductVersions)
	route("POST /deployments", handler.PermWrite, deploymentHandler.PostDeployment)
	route("POST /kb-articles", handler.PermWrite, kbArticleHandler.CreateKBArticle)
	route("GET /kb-articles/{id}", handler.PermView, kbArticleHandler.GetKBArticle)
	route("POST /kb-articles/search", handler.PermView, kbArticleHandler.SearchKBArticles)
	route("PATCH /kb-articles/{id}/state", handler.PermWrite, kbArticleHandler.PatchKBArticleState)
	route("PATCH /kb-articles/{id}", handler.PermWrite, kbArticleHandler.PatchKBArticleContent)
	route("GET /knowledge-bases", handler.PermView, kbArticleHandler.ListKnowledgeBases)
	route("POST /knowledge-bases", handler.PermAdmin, kbAdminHandler.CreateKnowledgeBase)
	route("PATCH /knowledge-bases/{id}", handler.PermAdmin, kbAdminHandler.UpdateKnowledgeBaseName)
	route("PATCH /knowledge-bases/{id}/active", handler.PermAdmin, kbAdminHandler.SetKnowledgeBaseActive)
	route("POST /kb-manager-users/search", handler.PermAdmin, kbAdminHandler.SearchKBManagerUsers)
	route("POST /kb-manager-users", handler.PermAdmin, kbAdminHandler.CreateKBManagerUser)
	route("DELETE /kb-manager-users", handler.PermAdmin, kbAdminHandler.DeleteKBManagerUser)
	route("POST /kb-manager-groups/search", handler.PermAdmin, kbAdminHandler.SearchKBManagerGroups)
	route("POST /kb-manager-groups", handler.PermAdmin, kbAdminHandler.CreateKBManagerGroup)
	route("DELETE /kb-manager-groups", handler.PermAdmin, kbAdminHandler.DeleteKBManagerGroup)
	route("GET /kb-managers/my-knowledge-bases", handler.PermView, kbArticleHandler.ListMyManagedKnowledgeBases)
	route("DELETE /kb-articles/{id}", handler.PermWrite, kbArticleHandler.DeleteKBArticle)
	route("GET /kb-articles/{id}/history", handler.PermView, kbArticleHandler.ListKBArticleHistory)
	route("POST /deployments/search", handler.PermView, deploymentHandler.SearchDeployments)
	route("PATCH /deployments/{id}", handler.PermWrite, deploymentHandler.PatchDeployment)
	route("POST /deployments/{id}/products", handler.PermWrite, deploymentHandler.PostDeployedProduct)
	route("POST /deployments/{id}/products/search", handler.PermView, deploymentHandler.SearchDeployedProducts)
	route("PATCH /deployments/{deploymentId}/products/{productId}", handler.PermWrite, deploymentHandler.PatchDeployedProduct)
	route("POST /deployed-products/projects/search", handler.PermView, deploymentHandler.SearchProjectsByProductVersion)
	route("POST /change-requests", handler.PermWrite, changeRequestHandler.CreateChangeRequest)
	route("GET /change-requests/{id}", handler.PermViewOperations, changeRequestHandler.GetChangeRequest)
	route("GET /change-requests/{id}/approvals", handler.PermViewOperations, changeRequestHandler.GetChangeRequestApprovals)
	route("POST /change-requests/{id}/approvals/decision", handler.PermWrite, changeRequestHandler.DecideChangeRequestApproval)
	route("PATCH /change-requests/{id}", handler.PermWrite, changeRequestHandler.PatchChangeRequest)
	route("POST /change-requests/search", handler.PermViewOperations, changeRequestHandler.SearchChangeRequests)
	route("POST /change-requests/aggregate", handler.PermViewOperations, changeRequestHandler.AggregateChangeRequests)
	route("POST /change-requests/link-options", handler.PermViewOperations, changeRequestHandler.GetChangeRequestLinkOptions)
	route("POST /services/search", handler.PermView, itServiceHandler.SearchITServices)
	route("POST /service-offerings/search", handler.PermView, serviceOfferingHandler.SearchServiceOfferings)
	route("POST /groups/search", handler.PermView, groupHandler.SearchGroups)
	// One group and its members -- what opens from a change request approval
	// stage's assignment group. A "group" id (from the approvals response), not a
	// team id; internal staff only (entity-service refuses anyone else).
	route("GET /groups/{id}", handler.PermView, groupHandler.GetGroup)

	// Team Schedule. Reads only for now, so everything sits under view: any
	// role that can see the portal can see who is on the rota. Editing the
	// rota is a lead's job and will need a permission of its own when the
	// write routes land -- see the plan's Phase 2b.
	scheduleHandler := handler.NewScheduleHandler(customerEntityClient)
	route("GET /team-schedule/catalogue", handler.PermView, scheduleHandler.GetScheduleCatalogue)
	route("POST /team-schedule/assignments/search", handler.PermView, scheduleHandler.SearchScheduleAssignments)
	route("POST /team-schedule/absences/search", handler.PermView, scheduleHandler.SearchScheduleAbsences)
	route("GET /team-schedule/on-duty", handler.PermView, scheduleHandler.GetScheduleOnDuty)

	// Lead edit. PermWrite keeps the control away from a caller who could not
	// use it at all; whether this particular person leads this particular team
	// is decided by entity-service, which has the membership to decide it.
	route("POST /team-schedule/assignments", handler.PermWrite, scheduleHandler.CreateScheduleAssignment)
	route("PATCH /team-schedule/assignments/{id}", handler.PermWrite, scheduleHandler.UpdateScheduleAssignment)
	route("DELETE /team-schedule/assignments/{id}", handler.PermWrite, scheduleHandler.DeleteScheduleAssignment)
	route("GET /team-schedule/activity", handler.PermView, scheduleHandler.GetScheduleActivity)
	route("GET /team-schedule/edit-markers", handler.PermView, scheduleHandler.GetScheduleEditMarkers)
	route("GET /team-schedule/my-lead-teams", handler.PermView, scheduleHandler.GetMyLeadTeams)
	route("POST /team-schedule/assignments/apply", handler.PermWrite, scheduleHandler.ApplyScheduleRange)
	route("POST /team-schedule/absences/apply", handler.PermWrite, scheduleHandler.ApplyScheduleAbsence)
	route("DELETE /team-schedule/absences/{id}", handler.PermWrite, scheduleHandler.DeleteScheduleAbsence)
	route("POST /team-schedule/absence-kinds", handler.PermWrite, scheduleHandler.CreateScheduleAbsenceKind)
	route("DELETE /team-schedule/absence-kinds/{code}", handler.PermWrite, scheduleHandler.DeleteScheduleAbsenceKind)
	route("POST /configuration-items/search", handler.PermView, configurationItemHandler.SearchConfigurationItems)
	route("POST /time-cards/search", handler.PermTimeCardsAndUpdates, timeCardHandler.SearchTimeCards)
	route("POST /time-cards", handler.PermTimeCardsAndUpdates, timeCardHandler.CreateTimeCard)
	route("PATCH /time-cards/{id}", handler.PermTimeCardsAndUpdates, timeCardHandler.UpdateTimeCard)
	route("DELETE /time-cards/{id}", handler.PermTimeCardsAndUpdates, timeCardHandler.DeleteTimeCard)
	route("POST /catalogs/search", handler.PermView, catalogHandler.SearchCatalogs)
	route("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", handler.PermView, catalogHandler.GetCatalogItemVariables)
	route("POST /products/vulnerabilities/search", handler.PermViewSecurityCenter, productVulnerabilityHandler.SearchProductVulnerabilities)
	route("GET /products/vulnerabilities/{id}", handler.PermViewSecurityCenter, productVulnerabilityHandler.GetProductVulnerability)
	route("GET /conversations/{id}/messages", handler.PermView, conversationHandler.GetConversationMessages)
	route("POST /conversations/search", handler.PermView, conversationHandler.SearchConversations)
	route("POST /slas/search", handler.PermView, taskSlaHandler.SearchTaskSlas)
	route("GET /slas/{id}", handler.PermView, taskSlaHandler.GetTaskSla)
	route("POST /cases/{caseId}/tasks/search", handler.PermView, taskHandler.SearchCaseTasks)
	route("POST /tasks/search", handler.PermView, taskHandler.SearchTasks)
	route("GET /tasks/{id}", handler.PermView, taskHandler.GetTask)
	route("POST /cases/{caseId}/tasks", handler.PermWrite, taskHandler.CreateCaseTask)
	route("PATCH /tasks/{id}", handler.PermWrite, taskHandler.UpdateTask)
	route("POST /incidents/search", handler.PermViewOperations, incidentHandler.SearchIncidents)
	route("POST /incidents/aggregate", handler.PermViewOperations, incidentHandler.AggregateIncidents)
	route("POST /incidents", handler.PermWrite, incidentHandler.CreateIncident)
	route("GET /incidents/{id}", handler.PermViewOperations, incidentHandler.GetIncident)
	route("PATCH /incidents/{id}", handler.PermWrite, incidentHandler.PatchIncident)
	route("POST /incidents/{id}/comments", handler.PermWrite, incidentHandler.CreateIncidentComment)
	route("POST /incidents/{id}/comments/search", handler.PermViewOperations, incidentHandler.SearchIncidentComments)
	route("POST /incidents/{id}/activities/search", handler.PermViewOperations, incidentHandler.SearchIncidentActivities)
	route("POST /incidents/{id}/specialist-handoffs", handler.PermWrite, incidentHandler.HandOffIncidentToSpecialist)
	route("GET /alerts/{id}", handler.PermViewOperations, alertHandler.GetAlert)
	route("GET /smart-alerts/{id}", handler.PermViewOperations, alertHandler.GetSmartAlert)
	route("POST /change-requests/{id}/comments", handler.PermWrite, changeRequestHandler.CreateChangeRequestComment)
	route("POST /change-requests/{id}/comments/search", handler.PermViewOperations, changeRequestHandler.SearchChangeRequestComments)
	route("POST /problems", handler.PermWrite, problemHandler.CreateProblem)
	route("GET /problems/{id}", handler.PermViewOperations, problemHandler.GetProblem)
	route("PATCH /problems/{id}", handler.PermWrite, problemHandler.PatchProblem)
	route("POST /problems/search", handler.PermViewOperations, problemHandler.SearchProblems)
	route("POST /problems/aggregate", handler.PermViewOperations, problemHandler.AggregateProblems)
	route("GET /incident-tasks/{id}", handler.PermViewOperations, incidentTaskHandler.GetIncidentTask)
	route("PATCH /incident-tasks/{id}", handler.PermWrite, incidentTaskHandler.PatchIncidentTask)
	route("POST /incident-tasks/search", handler.PermViewOperations, incidentTaskHandler.SearchIncidentTasks)
	route("POST /incident-tasks/aggregate", handler.PermViewOperations, incidentTaskHandler.AggregateIncidentTasks)
	route("POST /outages", handler.PermWrite, outageHandler.CreateOutage)
	route("POST /outages/search", handler.PermViewOperations, outageHandler.SearchOutages)
	// Registered before the {id} wildcard purely for readability — net/http's
	// ServeMux resolves by specificity, not registration order, so this
	// literal path wins over the wildcard regardless.
	route("GET /outages/metadata", handler.PermViewOperations, outageHandler.GetOutageMetadata)
	route("GET /outages/{id}", handler.PermViewOperations, outageHandler.GetOutage)
	route("PATCH /outages/{id}", handler.PermWrite, outageHandler.PatchOutage)
	route("POST /outages/{id}/communications", handler.PermWrite, outageHandler.AddOutageCommunication)
	route("POST /outages/{id}/communications/search", handler.PermViewOperations, outageHandler.SearchOutageCommunications)
	// Called manually today; not yet wired into real incident/case creation.
	route("POST /notifications/google-chat/alerts", handler.PermWrite, notificationHandler.PostGoogleChatAlert)

	// SupportPortalLite — see viewerHandlers above. Registered only when
	// SPL_ENABLED is on, so an unconfigured deployment sees no new routes at
	// all. None of the routes below carry a /spl/ prefix: SPL and
	// csm-portal are the same backend, so the prefix only ever existed to
	// avoid colliding with csm-portal's own, differently-shaped
	// case-management domain, and none of these routes do. Accounts,
	// projects, and cases (read/search/comment) used to live under that
	// prefix for exactly that reason — all three now go through the shared
	// routes above (/accounts, /projects, /cases) instead, gated by
	// PermViewSharedEntity like every other caller of those specific routes
	// now that sales_solutions holds it (see PermViewSharedEntity's own doc
	// comment) -- unlike GET /teams/{id}/members above, which is a genuinely
	// unconditional CS Portal route (see teamHandler's own construction) and
	// so is registered outside this block, not in here. Account escalations
	// stay unmerged: CreateEscalation is an explicit stub on this data
	// source (no entity-service equivalent at all, so nothing to merge
	// onto), and the account-scoped read has no shared route either (CS
	// Portal's own /cases/{id}/escalations is per-case, not per-account).
	// Case attachments are unmerged for the same no-entity-service-equivalent
	// reason.
	//
	// Every route below is registered with PermViewerAccess, the blanket SPL
	// audience gate (formerly SPL_ALLOWED_GROUPS's raw-Asgardeo-groups
	// check -- see PermViewerAccess's own doc comment). EscalateCase,
	// DownloadAttachment, and every usage-metrics route additionally check
	// a narrower permission (PermEscalate/PermDownloadAttachment/
	// PermUsageMetricsViewer) inside the handler itself, the same layered
	// shape SPL_ADD_ESCALATION_GROUPS/SPL_DOWNLOAD_ATTACHMENT_GROUPS/
	// SPL_USAGE_METRICS_GROUPS enforced on top of SPL_ALLOWED_GROUPS
	// before -- see requireViewerPermission's own doc comment for why that
	// second check couldn't just move to route-level registration like
	// every other route in this file.
	if viewerHandlers != nil {
		route("GET /accounts/{accountId}/escalations", handler.PermViewerAccess, viewerHandlers.accountEsc.GetAccountEscalations)
		route("POST /accounts/{accountId}/cases/{caseId}/escalate", handler.PermViewerAccess, viewerHandlers.accountEsc.EscalateCase)
		route("GET /cases/{caseId}/attachments-info", handler.PermViewerAccess, viewerHandlers.cases.GetAttachmentsInfo)
		route("GET /attachments/{attachmentId}/download", handler.PermViewerAccess, viewerHandlers.attachments.DownloadAttachment)
		route("GET /products", handler.PermViewerAccess, viewerHandlers.lookups.GetProducts)
		route("GET /abt-teams", handler.PermViewerAccess, viewerHandlers.lookups.GetABTTeams)
		route("GET /generate-sla-report", handler.PermViewerAccess, viewerHandlers.reports.GenerateSLAReport)
		route("GET /report-details", handler.PermViewerAccess, viewerHandlers.reports.GetReportDetails)
		route("GET /generate-timelogs-breakdown-report", handler.PermViewerAccess, viewerHandlers.reports.GenerateTimelogsBreakdownReport)
		route("GET /abt-team-schedule", handler.PermViewerAccess, viewerHandlers.schedule.GetABTTeamSchedule)
		route("GET /user-info", handler.PermViewerAccess, viewerHandlers.userInfo.GetUserInfo)
		route("POST /scan-user", handler.PermViewerAccess, viewerHandlers.userScan.ScanUser)
		route("GET /files", handler.PermViewerAccess, viewerHandlers.files.ListFiles)
		route("GET /files/search", handler.PermViewerAccess, viewerHandlers.files.SearchFolder)
		route("GET /usage-metrics/projects", handler.PermViewerAccess, viewerHandlers.usageMetrics.GetProjects)
		route("POST /usage-metrics/instances/metrics/search", handler.PermViewerAccess, viewerHandlers.usageMetrics.SearchInstanceMetrics)
		route("POST /usage-metrics/instances/metrics/stats", handler.PermViewerAccess, viewerHandlers.usageMetrics.GetInstanceMetricsStats)
		route("POST /usage-metrics/instances/usages/search", handler.PermViewerAccess, viewerHandlers.usageMetrics.SearchInstanceUsages)
		route("POST /usage-metrics/instances/usages/stats", handler.PermViewerAccess, viewerHandlers.usageMetrics.GetInstanceUsagesStats)
		route("POST /usage-metrics/deployments/search", handler.PermViewerAccess, viewerHandlers.usageMetrics.SearchDeployments)
		route("POST /usage-metrics/projects/search", handler.PermViewerAccess, viewerHandlers.usageMetrics.SearchProjects)
		route("POST /usage-metrics/deployed-products/search", handler.PermViewerAccess, viewerHandlers.usageMetrics.SearchDeployedProducts)
		route("POST /usage-metrics/instances/search", handler.PermViewerAccess, viewerHandlers.usageMetrics.SearchInstances)
		route("POST /usage-metrics/deployed-products/{id}/metrics/search", handler.PermViewerAccess, viewerHandlers.usageMetrics.GetDeployedProductMetrics)
		route("POST /usage-metrics/deployed-products/{id}/metrics/usage-counts/search", handler.PermViewerAccess, viewerHandlers.usageMetrics.GetDeployedProductUsageCounts)
		route("POST /customer-health/summary", handler.PermViewerAccess, viewerHandlers.customerHealth.GetSummary)
		route("POST /customer-health/accounts/{accountSysId}/init-health-tracking", handler.PermViewerAccess, viewerHandlers.customerHealth.InitHealthTracking)
		route("GET /customer-health/accounts/{accountId}", handler.PermViewerAccess, viewerHandlers.customerHealth.GetAccountDetail)
		route("POST /customer-health/projects/{projectSysId}/risk", handler.PermViewerAccess, viewerHandlers.customerHealth.OpenRisk)
		route("PUT /customer-health/risks/{riskId}/close", handler.PermViewerAccess, viewerHandlers.customerHealth.CloseRisk)
		route("POST /customer-health/projects/{projectSysId}/mark-healthy", handler.PermViewerAccess, viewerHandlers.customerHealth.MarkHealthy)
		route("POST /customer-health/projects/{projectSysId}/revert-review", handler.PermViewerAccess, viewerHandlers.customerHealth.RevertReview)
		route("GET /customer-health/accounts/{accountSysId}/health-status", handler.PermViewerAccess, viewerHandlers.customerHealth.GetAccountHealthStatus)
		route("GET /customer-health/accounts/{accountSysId}/health-summary", handler.PermViewerAccess, viewerHandlers.customerHealth.GetAccountHealthSummary)
		route("GET /customer-health/projects/{projectSysId}/risk-history", handler.PermViewerAccess, viewerHandlers.customerHealth.GetProjectRiskHistory)
		route("POST /customer-health/risks/{riskId}/action-items", handler.PermViewerAccess, viewerHandlers.customerHealth.CreateActionItem)
		route("PUT /customer-health/action-items/{actionItemId}/status", handler.PermViewerAccess, viewerHandlers.customerHealth.UpdateActionItemStatus)
		route("PUT /customer-health/action-items/{actionItemId}", handler.PermViewerAccess, viewerHandlers.customerHealth.UpdateActionItem)
		route("GET /customer-health/risks/{riskId}/action-items", handler.PermViewerAccess, viewerHandlers.customerHealth.GetActionItemsByRisk)
		route("GET /customer-health/accounts/{accountSysId}/action-items", handler.PermViewerAccess, viewerHandlers.customerHealth.GetActionItemsByAccount)
		route("POST /customer-health/action-items/{actionItemId}/comments", handler.PermViewerAccess, viewerHandlers.customerHealth.CreateActionItemComment)
		route("GET /customer-health/action-items/{actionItemId}/comments", handler.PermViewerAccess, viewerHandlers.customerHealth.GetActionItemComments)
	}

	// Built once and reused on both listeners below: Auth() does a real JWKS
	// fetch (when TokenValidatorEnabled), so calling it a second time would
	// duplicate that startup network round-trip and double the chance of a
	// transient JWKS hiccup aborting startup, for no benefit — both
	// listeners validate the exact same tokens the exact same way.
	authMiddleware := middleware.Auth(authCfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// PLG Customer Success Portal. Its config, entity-service client, services,
	// handlers, identity middleware and 26 plg/* routes are all assembled in
	// internal/plg — this is the only line of csm-portal's own wiring the merge
	// touches.
	//
	// Mounted on the same mux, so PLG runs inside the middleware chain below:
	// SecurityHeaders, CORS, CorrelationID and — the reason the merge is worth
	// doing — Auth. PLG's routes are JWT-validated by csm-portal, and the
	// X-PLG-User header the standalone build trusted no longer exists.
	//
	// PLG inherits entity-service's address and credentials rather than keeping
	// its own copy: it reaches the same service as the same OAuth2 application
	// as every other upstream client above. PLG_* overrides exist but are not
	// normally set.
	if err := plg.Mount(os.Getenv("PLG_CONFIG_FILE"), plgconfig.EntityDefaults{
		BaseURL:      customerEntityCfg.BaseURL,
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scope:        os.Getenv("CUSTOMER_ENTITY_SCOPES"),
	}, route); err != nil {
		slog.Error("failed to mount PLG", "err", err)
		os.Exit(1)
	}

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
		// REST_READ_TIMEOUT / REST_WRITE_TIMEOUT, default 60s each (raised from
		// 30s so large inline-attachment uploads are not cut off).
		ReadTimeout:  reqTimeouts.RESTRead,
		WriteTimeout: reqTimeouts.RESTWrite,
		IdleTimeout:  60 * time.Second,
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

// loadAccessConfig resolves, per portal role, the token role names that grant it:
//
//	AUTH_VIEWER_ROLES, AUTH_ESCALATOR_ROLES,
//	AUTH_ATTACHMENT_DOWNLOADER_ROLES, AUTH_USAGE_METRICS_VIEWER_ROLES,
//	AUTH_SUPPORT_ENGINEER_ROLES, AUTH_ADMIN_ROLES, AUTH_TIMECARD_APPROVER_ROLES,
//	AUTH_DASHBOARD_DESIGNER_ROLES, AUTH_SALES_SOLUTIONS_ROLES,
//	AUTH_WORKNOTE_CREATOR_ROLES
//	    Each is a comma-separated list of role names; a caller whose token's
//	    "roles" claim holds any one of them has that role.
//
// There is deliberately no default: role names are organisation vocabulary
// that must not be committed here, the same reasoning CSM_TEAM_REGISTRY's own
// lack of a default follows. A role whose variable is unset or empty is held by
// nobody, and startup warns naming each one, since with none configured at all
// nobody can use the portal.
//
// AUTH_SALES_SOLUTIONS_ROLES and AUTH_WORKNOTE_CREATOR_ROLES are unlike the
// rest: leaving either unset does not warn. sales_solutions is a normal,
// expected unconfigured state (CS Portal alone still works fine) rather than
// a misconfiguration nobody can use the portal at all without — see
// AccessConfig.SalesSolutions's own doc comment. worknote_creator is
// unconfigured-safe for a different reason: CsEngineer/Admin already hold
// PermCreateWorkNote regardless (see AccessConfig.WorknoteCreator's own doc
// comment), so leaving it empty is purely "this extra role isn't provisioned
// yet," never a state that locks anyone out of work notes.
func loadAccessConfig() handler.AccessConfig {
	var unset []string
	roles := func(name string) []string {
		configured := splitComma(os.Getenv(name))
		if len(configured) == 0 {
			unset = append(unset, name)
		}
		return configured
	}
	cfg := handler.AccessConfig{
		Viewer:               roles("AUTH_VIEWER_ROLES"),
		Escalator:            roles("AUTH_ESCALATOR_ROLES"),
		AttachmentDownloader: roles("AUTH_ATTACHMENT_DOWNLOADER_ROLES"),
		UsageMetricsViewer:   roles("AUTH_USAGE_METRICS_VIEWER_ROLES"),
		// The env var name stays AUTH_SUPPORT_ENGINEER_ROLES even though the
		// portal role itself was renamed to cs_engineer -- see
		// handler.AccessConfig.CsEngineer's own doc comment for why.
		CsEngineer:        roles("AUTH_SUPPORT_ENGINEER_ROLES"),
		Admin:             roles("AUTH_ADMIN_ROLES"),
		TimecardApprover:  roles("AUTH_TIMECARD_APPROVER_ROLES"),
		DashboardDesigner: roles("AUTH_DASHBOARD_DESIGNER_ROLES"),
		// Unlike the roles above, an unset AUTH_SALES_SOLUTIONS_ROLES or
		// AUTH_WORKNOTE_CREATOR_ROLES is a normal, supported state (see this
		// function's own doc comment for why each is), so both deliberately
		// bypass the roles() helper to avoid adding themselves to the
		// unset-variable warning below.
		SalesSolutions:  splitComma(os.Getenv("AUTH_SALES_SOLUTIONS_ROLES")),
		WorknoteCreator: splitComma(os.Getenv("AUTH_WORKNOTE_CREATOR_ROLES")),
	}
	if len(unset) > 0 {
		slog.Warn("access-control role variables are unset, so no token role grants them", "variables", unset)
	}
	return cfg
}

// loadAnnouncementExcludedProjectKeys resolves the "All customer projects"
// announcement audience's mandatory excluded-project-key denylist from its
// configuration form:
//
//	CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS  A comma-separated list of project
//	                                         keys, whitespace around each
//	                                         entry trimmed. AnnouncementHandler
//	                                         injects this list into every
//	                                         POST /announcements/audience/search
//	                                         call unconditionally — the
//	                                         caller cannot opt out — mirroring
//	                                         the real ServiceNow flow this
//	                                         replaces, whose own "Create
//	                                         announcement for customers" flow
//	                                         hardcodes an equivalent Project
//	                                         Key exclusion with no way for
//	                                         whoever triggers it to opt out.
//
// Unlike directory.DefaultRoles, this deliberately has no committed default:
// project keys are organisation-specific data, not generic platform
// vocabulary, so there is nothing safe to commit — the same reasoning
// CSM_TEAM_REGISTRY's own lack of a default follows. An unset or empty value
// yields no exclusions, so a deployment that has not configured this yet
// still starts and simply excludes nothing extra.
func loadAnnouncementExcludedProjectKeys() []string {
	keys := splitComma(os.Getenv("CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS"))
	slog.Info("resolved announcement excluded-project-key list", "count", len(keys))
	return keys
}

// customerEntityDataSourcePostgres and customerEntityDataSourceServiceNow
// mirror entity-service's own DATA_SOURCE values exactly (see
// entity-service/internal/config/config.go's DataSource type) — this is not
// an independent enum, it describes a property of the entity service this
// backend is paired with.
const (
	customerEntityDataSourcePostgres   = "postgres"
	customerEntityDataSourceServiceNow = "servicenow"
)

// validateCustomerEntityDataSource is the pure check behind
// loadCustomerEntityDataSource: v (already lowercased/trimmed) must be
// "postgres" or "servicenow".
func validateCustomerEntityDataSource(v string) error {
	if v != customerEntityDataSourcePostgres && v != customerEntityDataSourceServiceNow {
		return fmt.Errorf("CUSTOMER_ENTITY_DATA_SOURCE must be %q or %q, got %q",
			customerEntityDataSourcePostgres, customerEntityDataSourceServiceNow, v)
	}
	return nil
}

// loadCustomerEntityDataSource resolves which data source the paired
// entity-service instance is configured with, from CUSTOMER_ENTITY_DATA_SOURCE
// ("postgres" or "servicenow"). Defaults to "servicenow" when unset — the
// data source every existing deployment has always effectively used, since
// nothing here read this before now.
//
// This exists purely so checkAnnouncementDataSourceCompatibility (see below)
// can catch a specific, otherwise-silent misconfiguration at startup:
// entity-service's Postgres-backed project search rejects
// excludeClosureStates/excludeSubscriptionTypes/excludeProjectKeys outright
// (see entity-service/internal/service/project_service.go), so a deployment
// with both DATA_SOURCE=postgres on entity-service and a non-empty
// CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS here would have every "All customer
// projects" audience search fail with a 400 — every time, with no caller
// action able to avoid it, since the mandatory denylist is injected
// unconditionally. Exits the process on an unrecognized value, same as any
// other malformed required config in this file.
func loadCustomerEntityDataSource() string {
	v := strings.ToLower(strings.TrimSpace(envOrDefault("CUSTOMER_ENTITY_DATA_SOURCE", customerEntityDataSourceServiceNow)))
	if err := validateCustomerEntityDataSource(v); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	slog.Info("resolved paired entity-service data source", "dataSource", v)
	return v
}

// checkAnnouncementDataSourceCompatibility is the pure check behind
// validateAnnouncementDataSourceCompatibility: non-nil exactly when the
// announcement audience-search feature is configured in a way it can never
// actually serve — a mandatory excluded-project-key denylist with no way to
// enforce it. See loadCustomerEntityDataSource's doc comment for why this
// specific combination is unserviceable rather than merely degraded.
func checkAnnouncementDataSourceCompatibility(dataSource string, excludedProjectKeys []string) error {
	if dataSource == customerEntityDataSourcePostgres && len(excludedProjectKeys) > 0 {
		return fmt.Errorf(
			"CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS is set (%d keys) but the paired entity-service runs "+
				"DATA_SOURCE=postgres, which does not support excludeProjectKeys — every announcement audience "+
				"search would fail. Either unset CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS, or point "+
				"CUSTOMER_ENTITY_DATA_SOURCE at a servicenow-backed entity-service instance",
			len(excludedProjectKeys),
		)
	}
	return nil
}

// validateAnnouncementDataSourceCompatibility exits the process if
// checkAnnouncementDataSourceCompatibility finds a problem.
//
// This is deliberately a hard startup failure, not a runtime fallback that
// silently stops enforcing the denylist when it can't be sent — the whole
// point of CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS is that an "All customer
// projects" send must never reach those projects; quietly omitting the
// filter so the request merely succeeds would defeat that guarantee instead
// of failing loudly the one time it's actually needed.
func validateAnnouncementDataSourceCompatibility(dataSource string, excludedProjectKeys []string) {
	if err := checkAnnouncementDataSourceCompatibility(dataSource, excludedProjectKeys); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

// onboardingStatusFlag is the env var gating GET /projects/{id}/onboarding-steps.
const onboardingStatusFlag = "CSM_MIGRATION_ONBOARDING_STATUS_ENABLED"

// loadOnboardingStatusEnabled resolves the customer-onboarding status feature
// flag:
//
//	CSM_MIGRATION_ONBOARDING_STATUS_ENABLED  Exactly "true" (after trimming
//	                                         whitespace) turns the feature on.
//	                                         Off by default — unset, empty, or
//	                                         any other value (including "1",
//	                                         "TRUE", "yes") keeps it dark and
//	                                         changes nothing else in this
//	                                         backend. Deliberately stricter
//	                                         than the strconv.ParseBool
//	                                         parsing SFTPGO_* uses: every
//	                                         CSM_MIGRATION_* flag is a
//	                                         cutover switch that must not
//	                                         flip on by accident.
func loadOnboardingStatusEnabled() bool {
	enabled := onboardingStatusEnabled(os.Getenv(onboardingStatusFlag))
	if enabled {
		slog.Info(onboardingStatusFlag + " is on: GET /projects/{id}/onboarding-steps is registered")
	}
	return enabled
}

// onboardingStatusEnabled is the pure parse behind loadOnboardingStatusEnabled.
func onboardingStatusEnabled(raw string) bool {
	return strings.TrimSpace(raw) == "true"
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
// mustHTTPSBaseURL is mustHTTPSURL for a base URL that may carry a path; see
// validateHTTPSBaseURL. Used for upstream services the backend authenticates to
// with an OAuth2 client, whose token and requests must not travel in cleartext.
func mustHTTPSBaseURL(key, value string) string {
	if err := validateHTTPSBaseURL(value); err != nil {
		slog.Error("invalid environment variable", "key", key, "err", err)
		os.Exit(1)
	}
	return value
}

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
	return validateSecureURL(value, false)
}

// validateHTTPSBaseURL is validateHTTPSURL for a base URL that API paths are
// appended to and that may itself sit under a path (a gateway-hosted service
// such as "https://host/org/service/v1.0"): the same https, host, userinfo,
// query and fragment rules, but a path is allowed.
func validateHTTPSBaseURL(value string) error {
	return validateSecureURL(value, true)
}

func validateSecureURL(value string, allowPath bool) error {
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
	if path := parsed.EscapedPath(); !allowPath && path != "" && path != "/" {
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

// viewerHandlerSet holds every SupportPortalLite handler, constructed
// only when SPL_ENABLED is on. See loadViewerConfig for the environment
// variables backing each field.
type viewerHandlerSet struct {
	cases          *handler.ViewerCaseHandler
	reports        *handler.ReportsHandler
	schedule       *handler.ViewerScheduleHandler
	attachments    *handler.AttachmentsHandler
	lookups        *handler.LookupsHandler
	usageMetrics   *handler.UsageMetricsHandler
	files          *handler.FilesHandler
	customerHealth *handler.CustomerHealthHandler
	userInfo       *handler.UserInfoHandler
	userScan       *handler.SplUserScanHandler
	accountEsc     *handler.ViewerAccountHandler
}

// viewerConfig holds every environment value SupportPortalLite's /spl/*
// endpoints need, resolved by loadViewerConfig.
type viewerConfig struct {
	snHost                 string
	snUsername             string
	snPassword             string
	snEscalationTemplateID string
	teamScheduleURL        string
	driveClientID          string
	driveClientSecret      string
	driveRefreshToken      string
	riskMySQLDSN           string
	salesEntityBaseURL     string
}

// loadViewerConfig resolves SupportPortalLite's (/spl/*) configuration.
//
//	SPL_ENABLED  Any strconv.ParseBool-true value (1, t, T, TRUE, true,
//	             True). Off by default — unset, empty, or any other value
//	             keeps every /spl/* route unregistered and every other env
//	             var below unread, mirroring SFTPGO_ATTACHMENT_STORAGE_ENABLED's
//	             parsing convention. An unparseable non-empty value is a
//	             warning, not fatal, and defaults to off.
//
// When on, every value below is required (mustEnv) except
// SERVICENOW_ESCALATION_TEMPLATE_ID and TEAM_SCHEDULE_URL, which are
// only exercised by the escalation and ABT-team-schedule endpoints
// respectively and default to empty. SERVICENOW_*, GOOGLE_DRIVE_*, and the
// entity vars below have no SPL_ prefix even though they're only read when
// SPL is on: they aren't SPL-specific concepts (ServiceNow, Google Drive,
// and the sales-side entity service are just this feature's own upstreams)
// so they follow this file's existing convention of naming a service's own
// credentials after the service, not the caller -- SPL_RISK_MYSQL_DSN
// below is the one exception, since "risk" isn't a distinct upstream
// service name to key on. See .env.example for what each variable
// configures.
//
// Returns (false, zero viewerConfig) when the flag is off, so the caller never
// touches the returned viewerConfig in that case.
func loadViewerConfig() (bool, viewerConfig) {
	enabled := false
	if raw := strings.TrimSpace(os.Getenv("SPL_ENABLED")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			slog.Warn("SPL_ENABLED is not a boolean; treating it as false",
				"value", raw, "expected", "1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False")
		}
		enabled = parsed
	}
	if !enabled {
		return false, viewerConfig{}
	}

	return true, viewerConfig{
		snHost:                 mustHTTPSBaseURL("SERVICENOW_HOST", mustEnv("SERVICENOW_HOST")),
		snUsername:             mustEnv("SERVICENOW_USERNAME"),
		snPassword:             mustEnv("SERVICENOW_PASSWORD"),
		snEscalationTemplateID: os.Getenv("SERVICENOW_ESCALATION_TEMPLATE_ID"),
		teamScheduleURL:        os.Getenv("TEAM_SCHEDULE_URL"),
		driveClientID:          mustEnv("GOOGLE_DRIVE_CLIENT_ID"),
		driveClientSecret:      mustEnv("GOOGLE_DRIVE_CLIENT_SECRET"),
		driveRefreshToken:      mustEnv("GOOGLE_DRIVE_REFRESH_TOKEN"),
		riskMySQLDSN:           mustEnv("SPL_RISK_MYSQL_DSN"),
		salesEntityBaseURL:     mustHTTPSBaseURL("SALES_ENTITY_BASE_URL", mustEnv("SALES_ENTITY_BASE_URL")),
	}
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
