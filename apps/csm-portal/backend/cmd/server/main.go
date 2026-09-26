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
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/githubissue"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg"
	plgconfig "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/config"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/sftpgo"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/updates"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	dashboard.SetActive(loadDashboards())
	githubissue.SetActive(loadGithubIssueRepoOptions())

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
	metadataHandler := handler.NewMetadataHandler()
	accountHandler := handler.NewAccountHandler(customerEntityClient)
	projectHandler := handler.NewProjectHandler(customerEntityClient)
	announcementExcludedProjectKeys := loadAnnouncementExcludedProjectKeys()
	validateAnnouncementDataSourceCompatibility(loadCustomerEntityDataSource(), announcementExcludedProjectKeys)
	announcementHandler := handler.NewAnnouncementHandler(customerEntityClient, announcementExcludedProjectKeys)
	announcementRequestHandler := handler.NewAnnouncementRequestHandler(customerEntityClient, announcementExcludedProjectKeys)
	announcementRegistryHandler := handler.NewAnnouncementRegistryHandler(customerEntityClient)
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

	// One guard authorises every route below and also backs the permissions
	// GET /users/me reports, so the two cannot drift apart.
	accessGuard := handler.NewAccessGuard(loadAccessConfig())
	usersHandler := handler.NewUsersHandler(scimClient, customerEntityClient, dir, sftpgoAttachmentStorageEnabled).WithAccessGuard(accessGuard)
	dashboardHandler := handler.NewDashboardHandler(accessGuard)
	caseHandler = caseHandler.WithAccessGuard(accessGuard)

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
	route("GET /cases/{id}", handler.PermView, caseHandler.GetCase)
	route("PATCH /cases/{id}", handler.PermWrite, caseHandler.PatchCase)
	route("POST /cases/{id}/comments", handler.PermWrite, caseHandler.CreateCaseComment)
	route("POST /cases/{id}/request-update", handler.PermWrite, caseHandler.RequestCaseUpdate)
	route("GET /case-update-request-templates", handler.PermView, caseHandler.GetCaseUpdateRequestTemplates)
	route("POST /cases/{id}/comments/search", handler.PermView, caseHandler.SearchCaseComments)
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
	route("GET /metadata", handler.PermView, metadataHandler.GetMetadata)
	route("POST /cases/{id}/tags", handler.PermWrite, caseHandler.AddCaseTag)
	route("DELETE /cases/{id}/tags/{tagId}", handler.PermWrite, caseHandler.RemoveCaseTag)
	route("POST /tags/search", handler.PermView, caseHandler.SearchTags)
	// Deprecated: the query-parameter form of tag search, kept for one release
	// so this service and its callers can be deployed independently. Remove it
	// (and CaseHandler.SearchTagsQuery) once every caller is on the POST.
	//nolint:staticcheck // SA1019: intentional one-release compatibility route; remove with the handler.
	route("GET /tags/search", handler.PermView, caseHandler.SearchTagsQuery)
	route("POST /cases/search", handler.PermView, caseHandler.SearchCases)
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
	route("GET /users/{id}", handler.PermView, usersHandler.GetUser)
	route("POST /users", handler.PermAdmin, usersHandler.CreateUser)
	route("POST /roles/search", handler.PermView, referenceHandler.SearchRoles)
	route("POST /teams/search", handler.PermView, referenceHandler.SearchTeams)
	route("GET /accounts/{id}", handler.PermView, accountHandler.GetAccount)
	// Admin-only: CRE/SRE team is a temporary override of ServiceNow's own
	// value (see AccountService.UpdateAccountTeams's doc comment) — no other
	// staff role should be able to set it.
	route("PATCH /accounts/{id}", handler.PermAdmin, accountHandler.UpdateAccountTeams)
	route("POST /accounts/search", handler.PermView, accountHandler.SearchAccounts)
	route("POST /accounts/{id}/contacts/search", handler.PermView, accountHandler.SearchAccountContacts)
	route("GET /projects/{id}", handler.PermView, projectHandler.GetProject)
	route("GET /projects/{id}/metadata", handler.PermView, projectHandler.GetProjectMetadata)
	route("POST /projects/search", handler.PermView, projectHandler.SearchProjects)
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
	route("POST /projects/{id}/contacts/search", handler.PermView, projectHandler.SearchProjectContacts)
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
	route("POST /products/{id}/versions/search", handler.PermView, productHandler.SearchProductVersions)
	route("POST /deployments", handler.PermWrite, deploymentHandler.PostDeployment)
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
	route("POST /services/search", handler.PermView, itServiceHandler.SearchITServices)
	route("POST /service-offerings/search", handler.PermView, serviceOfferingHandler.SearchServiceOfferings)
	route("POST /groups/search", handler.PermView, groupHandler.SearchGroups)

	// Team Schedule. Reads only for now, so everything sits under view: any
	// role that can see the portal can see who is on the rota. Editing the
	// rota is a lead's job and will need a permission of its own when the
	// write routes land -- see the plan's Phase 2b.
	scheduleHandler := handler.NewScheduleHandler(customerEntityClient)
	route("GET /team-schedule/catalogue", handler.PermView, scheduleHandler.GetScheduleCatalogue)
	route("POST /team-schedule/assignments/search", handler.PermView, scheduleHandler.SearchScheduleAssignments)
	route("POST /team-schedule/absences/search", handler.PermView, scheduleHandler.SearchScheduleAbsences)
	route("GET /team-schedule/on-duty", handler.PermView, scheduleHandler.GetScheduleOnDuty)
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
	if err := plg.Mount(mux, os.Getenv("PLG_CONFIG_FILE"), plgconfig.EntityDefaults{
		BaseURL:      customerEntityCfg.BaseURL,
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scope:        os.Getenv("CUSTOMER_ENTITY_SCOPES"),
	}); err != nil {
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

// loadGithubIssueRepoOptions resolves the "Open Git issue" dialog's
// repository catalogue from GITHUB_ISSUE_REPO_OPTIONS (a JSON array — see
// githubissue.ParseRepoOptions for the shape and validation) and exits the
// process on any failure to parse it.
//
// This used to be a hardcoded array in the frontend, which is how a real case
// filed with "Asgardeo" selected landed in the wrong GitHub repository: the
// owner/repo mapping lived in code no config reviewer would think to check.
// Fatal on malformed content, same rationale as loadDashboards: an operator
// error here should stop the deploy, not silently ship an empty or
// half-populated dropdown. Unset is legal and yields no options — a
// deployment that has not configured this yet must still start.
func loadGithubIssueRepoOptions() []githubissue.RepoOption {
	options, err := githubissue.ParseRepoOptions(os.Getenv("GITHUB_ISSUE_REPO_OPTIONS"))
	if err != nil {
		slog.Error("invalid GITHUB_ISSUE_REPO_OPTIONS", "err", err)
		os.Exit(1)
	}
	slog.Info("loaded github issue repo options", "count", len(options))
	return options
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
//	AUTH_DASHBOARD_DESIGNER_ROLES
//	    Each is a comma-separated list of role names; a caller whose token's
//	    "roles" claim holds any one of them has that role.
//
// There is deliberately no default: role names are organisation vocabulary
// that must not be committed here, the same reasoning CSM_TEAM_REGISTRY's own
// lack of a default follows. A role whose variable is unset or empty is held by
// nobody, and startup warns naming each one, since with none configured at all
// nobody can use the portal.
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
