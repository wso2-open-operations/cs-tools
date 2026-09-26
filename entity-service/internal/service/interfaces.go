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

// Package service contains business logic that sits between the HTTP handlers
// and the repository layer.
package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// UserService defines the operations available on the user entity.
// Handlers depend on this interface rather than the concrete implementation,
// making it straightforward to substitute a test double in unit tests.
type UserService interface {
	// SearchUsers returns a paginated list of users that match the filters in
	// req. A ValidationError is returned for invalid input (e.g. limit > 50);
	// any other error indicates an infrastructure failure.
	SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchUsersResponse, error)
	// GetMe returns the profile of the currently authenticated user, resolved
	// from the Postgres users table by the email claim in the caller's
	// x-user-id-token JWT. An UnauthorizedError is returned when that header
	// is missing; a ValidationError when the token cannot be decoded; a
	// NotFoundError when no user row matches the email.
	GetMe(ctx context.Context) (domain.GetUserMeResponse, error)
	// GetUsersByIDs returns every user matching the given ids. Not gated to
	// any particular data source -- ids are this platform's own primary
	// keys, so an id-based lookup works the same way regardless of source.
	GetUsersByIDs(ctx context.Context, ids []string) (domain.GetUsersByIDsResponse, error)
	// GetUser returns one user's profile: the user row, roles, groups, and for a
	// customer the project-contact rows with whether each grants access. A
	// ValidationError is returned for a malformed id and a NotFoundError when no
	// user has it.
	GetUser(ctx context.Context, id string) (domain.UserDetail, error)
	// CreateUser inserts a new "user" row, optionally granting the roles named
	// in req.Roles. The acting caller is resolved from x-user-id-token, the
	// same way GetMe resolves its own caller, and stamped as created_by on
	// every row this writes -- an UnauthorizedError is returned when that
	// header is missing. A ValidationError is returned for a missing/malformed
	// email; a ConflictError when a user with that email already exists; a
	// ServiceUnavailableError naming any requested role not seeded in the role
	// table.
	CreateUser(ctx context.Context, req domain.CreateUserRequest) (domain.User, error)
}

// SavedFilterViewService is the caller's own named list-filter bookmarks
// (CSM portal saved views). Postgres-only; the caller is always the
// authenticated user resolved from x-user-id-token — never a client-supplied
// user id.
type SavedFilterViewService interface {
	List(ctx context.Context, listKey domain.SavedFilterListKey) (domain.SavedFilterViewList, error)
	Save(ctx context.Context, req domain.SaveSavedFilterViewRequest) (domain.SavedFilterViewList, error)
	Delete(ctx context.Context, listKey domain.SavedFilterListKey, name string) (domain.SavedFilterViewList, error)
	Reorder(ctx context.Context, req domain.ReorderSavedFilterViewRequest) (domain.SavedFilterViewList, error)
}

// SNUserService defines the user operations backed by the ServiceNow data source.
type SNUserService interface {
	// SearchUsers returns a paginated list of ServiceNow users that match the
	// filters in req. A ValidationError is returned for invalid input; any other
	// error indicates an infrastructure failure.
	SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchSNUsersResponse, error)
	// GetMe returns the profile of the currently authenticated user from ServiceNow.
	GetMe(ctx context.Context) (domain.GetUserMeResponse, error)
	// PatchMe updates mutable fields on the currently authenticated user in ServiceNow.
	PatchMe(ctx context.Context, req domain.PatchUserMeRequest) (domain.PatchUserMeResponse, error)
	// GetUser returns one user's full profile: the user row plus group and team
	// membership, and for external contacts their per-project access. A NotFoundError
	// is returned when no user has that id.
	GetUser(ctx context.Context, id string) (domain.SNUserDetail, error)
}

// AccountService defines the operations available on the account entity.
type AccountService interface {
	// SearchAccounts returns a paginated list of accounts that match the filters
	// in req. A ValidationError is returned for invalid input; any other error
	// indicates an infrastructure failure.
	SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) (domain.SearchAccountsResponse, error)
	// GetAccountByID returns the account with the given UUID. A ValidationError is
	// returned for a malformed UUID; a NotFoundError if no account matches.
	GetAccountByID(ctx context.Context, id string) (domain.AccountDetail, error)
	// UpdateAccountTeams sets the account's CRE and/or SRE team (Postgres data
	// source only). A ValidationError is returned for a malformed UUID, if
	// neither field is provided, or if a provided team ID does not reference
	// an existing team; a NotFoundError if no account matches.
	//
	// This is a temporary override, not a durable value: the ServiceNow-to-
	// Postgres sync maps u_integration_cs_team/u_sre_team into these same
	// columns, so a value set here can be silently reverted the next time the
	// account's ServiceNow record changes for any reason. Intentional, by
	// product decision — ServiceNow remains authoritative.
	UpdateAccountTeams(ctx context.Context, req domain.UpdateAccountTeamsRequest) (domain.AccountDetail, error)
}

// SalesforceEventService handles POST /salesforce/events. Account fetch goes
// through REST sales/sales-entity-service POST /customer-search, not GraphQL.
type SalesforceEventService interface {
	HandleEvent(ctx context.Context, req domain.SalesforceEventRequest) error
}

// MembershipRegistrationService backs POST /users/me/memberships/register: the Customer Portal
// calls it on every profile load, and it marks the signed-in user's still
// un-accepted memberships as REGISTERED in Salesforce. The portal itself
// therefore never needs Salesforce write access. Postgres-only, and gated on
// CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED — see membership_registration_service.go.
type MembershipRegistrationService interface {
	// RegisterInvitedMemberships flips every INVITED / RE-INVITED membership of the
	// caller (resolved from x-user-id-token, exactly like GetMe) to
	// REGISTERED in Salesforce, re-ingests each one so Postgres matches, and
	// records the REGISTRATION onboarding step per membership. A caller with
	// no such membership — almost every call — does nothing at all.
	//
	// An UnauthorizedError is returned when the header is missing and a
	// ValidationError when the token cannot be decoded. One membership
	// failing never stops the others: an error is only returned when every
	// membership failed, so a partial success is still a success.
	RegisterInvitedMemberships(ctx context.Context) error
}

// EventPublishFailureService defines the operations available on the
// event_publish_failures entity — see domain.EventPublishFailure's doc
// comment for what it's for.
type EventPublishFailureService interface {
	// CreateEventPublishFailure inserts a new unresolved failure row. A
	// ValidationError is returned if eventType, entityId, payload, or error
	// is missing.
	CreateEventPublishFailure(ctx context.Context, req domain.CreateEventPublishFailureRequest) (domain.EventPublishFailure, error)
	// ResolveEventPublishFailure marks id resolved and returns the updated
	// row. Idempotent — resolving an already-resolved row is a no-op
	// success. A ValidationError is returned for a malformed UUID; a
	// NotFoundError if id does not exist.
	ResolveEventPublishFailure(ctx context.Context, id string) (domain.EventPublishFailure, error)
	// SearchEventPublishFailures returns a paginated list of rows matching
	// the filters in req, newest first.
	SearchEventPublishFailures(ctx context.Context, req domain.SearchEventPublishFailuresRequest) (domain.SearchEventPublishFailuresResponse, error)
}

// EventPublisherService publishes domain events to the case-events Event Hub
// topic for csm-notification-service (and any other future consumer) to
// react to — see eventPublisherService's doc comment for the wire format and
// failure handling. Constructed in internal/server/routes.go, gated on
// config.Config.EventHubBroker being set (nil otherwise — every caller must
// handle a nil EventPublisherService, matching every other optional
// dependency in this service). Currently called only from
// snCaseService.CreateCase (case.created) and
// snIncidentService.CreateIncident (incident.created), both synchronously
// and best-effort: a publish failure there is logged, not returned, since
// the case/incident already exists in ServiceNow by that point and a
// notification-side hiccup must not be reported as a failed create.
type EventPublisherService interface {
	// Publish builds the {type, entityId, payload} envelope for eventType/
	// entityID/payload and publishes it to Event Hub, keyed by entityID so
	// every event about the same entity stays ordered on the same
	// partition. If the publish itself fails (Event Hub never acknowledges
	// it), Publish makes a best-effort call to CreateEventPublishFailure to
	// durably record the failure before returning the original publish
	// error.
	Publish(ctx context.Context, eventType events.Type, entityID string, payload json.RawMessage) error
	// Close releases the underlying Kafka connection. Safe to call once
	// during shutdown.
	Close()
}

// SLAStatusService defines the operations available on SLA status — see
// domain.SLAStatus's doc comment for what it's for and what it replaced.
type SLAStatusService interface {
	// SearchActiveSLAStatuses returns every currently-active SLA clock across
	// every case-like work item, paginated. A ValidationError is returned for
	// an invalid pagination limit.
	SearchActiveSLAStatuses(ctx context.Context, req domain.Pagination) (domain.SearchSLAStatusResponse, error)
}

// OnboardingStepService records and reads the per-membership status ledger
// of the customer onboarding flow (onboarding_step). The DATABASE step is
// written in-process by the Salesforce membership ingest; IDENTITY, EMAIL
// and REGISTRATION are written over HTTP by csm-notification-service and
// the customer portal backend.
type OnboardingStepService interface {
	// Upsert writes the latest outcome of one step. MembershipSfID and Step
	// come from the path; a repeat for the same pair updates the row and
	// increments attemptCount.
	Upsert(ctx context.Context, req domain.UpsertOnboardingStepRequest) (domain.OnboardingStep, error)
	// GetByMembership returns every recorded step for a membership (an empty
	// list for an unknown membership, never a 404).
	GetByMembership(ctx context.Context, membershipSfID string) (domain.GetOnboardingStepsResponse, error)
	// Search returns a filtered, paginated list of steps, newest first.
	Search(ctx context.Context, req domain.SearchOnboardingStepsRequest) (domain.SearchOnboardingStepsResponse, error)
}

// ProjectMembershipWriteService owns every change to who is a contact on a
// project: the Customer Portal (a customer admin managing their own users)
// and the CSM Portal (an account manager doing it for them) both call these
// same four operations, and each one writes the CSM database and Salesforce
// together or writes neither.
//
// The database is the source of truth; Salesforce is kept in step rather
// than read from as an authority. See NewProjectMembershipWriteService for
// the ordering that makes "both or neither" hold without a distributed
// transaction, and why the database commits last.
//
// Every method is restricted to internal callers (AUTH_INTERNAL_CLIENT_IDS).
// The portal backends decide who may invite whom; this service does not.
type ProjectMembershipWriteService interface {
	// Invite adds a contact to a project: state INVITED (RE-INVITED when a
	// previously deactivated membership is being brought back) in both
	// systems, and a project_contact.invited event. A ConflictError when the
	// address is already an active contact on the project.
	Invite(ctx context.Context, projectID string, req domain.CreateProjectMembershipRequest) (domain.ProjectMembership, error)
	// UpdateRoles replaces the membership's Salesforce roles, and with them
	// its project groups. The state is untouched.
	UpdateRoles(ctx context.Context, projectID, email string, req domain.UpdateProjectMembershipRolesRequest) (domain.ProjectMembership, error)
	// Deactivate moves the membership to DEACTIVATED in both systems. It
	// never deletes: DEACTIVATED is a real state in both the Salesforce
	// picklist and project_contact_state_enum.
	Deactivate(ctx context.Context, projectID, email string) error
	// ResendInvitation re-publishes project_contact.invited with a resend
	// marker so csm-notification-service bypasses its already-sent guard.
	// Valid only while the membership is INVITED (ConflictError otherwise),
	// and rate-limited per membership (TooManyRequestsError).
	ResendInvitation(ctx context.Context, projectID, email string) error
}

// ScheduledTaskRunService defines the operations available on the
// scheduled_task_run entity — see domain.ScheduledTaskRun's doc comment for
// what it's for.
type ScheduledTaskRunService interface {
	// Attempt decides whether req.TaskName/req.PeriodKey may run right now,
	// claiming it if so — see domain.ClaimScheduledTaskRunResponse's doc
	// comment for how to read the result. A ValidationError is returned if
	// taskName or periodKey is missing.
	Attempt(ctx context.Context, req domain.ClaimScheduledTaskRunRequest) (domain.ClaimScheduledTaskRunResponse, error)
	// UpdateAttempt reports the outcome of the attempt id — succeeded or
	// failed, per req.Status — but only if req.AttemptCount still matches
	// the active claim (see domain.UpdateScheduledTaskRunAttemptRequest's
	// own doc comment). A ValidationError is returned if attemptCount is
	// missing/non-positive, status isn't "succeeded"/"failed", or status is
	// "failed" and error/nextRetryOn is missing; a NotFoundError if id
	// doesn't exist or the claim is no longer active.
	UpdateAttempt(ctx context.Context, id string, req domain.UpdateScheduledTaskRunAttemptRequest) (domain.ScheduledTaskRun, error)
	// List returns every run matching statusFilter ("failed", "succeeded",
	// "superseded"), or every run if statusFilter is empty. A
	// ValidationError is returned for any other value.
	List(ctx context.Context, statusFilter string) (domain.ListScheduledTaskRunsResponse, error)
	// DeleteResolvedBefore deletes every run that succeeded or was
	// superseded before cutoff (by its own resolution time, not when it
	// was created — see the repository's own doc comment for why that
	// distinction matters). A ValidationError is returned if cutoff is the
	// zero time.
	DeleteResolvedBefore(ctx context.Context, cutoff time.Time) (domain.DeleteScheduledTaskRunsResponse, error)
}

// AlertIncidentMappingService defines the operations available on the
// alert_incident_mapping entity — see domain.AlertIncidentMappingView's doc
// comment for what it's for.
type AlertIncidentMappingService interface {
	// CreateAlertIncidentMapping inserts a new mapping row. A ValidationError
	// is returned if alertNumber, source, alertStatus, or incidentId is
	// missing; a ConflictError if alertNumber is already mapped.
	CreateAlertIncidentMapping(ctx context.Context, req domain.CreateAlertIncidentMappingRequest) (domain.AlertIncidentMappingView, error)
	// LookupAlertIncidentMappings returns every mapping for
	// (req.Source, req.UniqueIdentifier), most-recent-first. A
	// ValidationError is returned if source or uniqueIdentifier is missing.
	// An empty (never nil) Mappings slice, not an error, is returned when
	// nothing matches.
	LookupAlertIncidentMappings(ctx context.Context, req domain.LookupAlertIncidentMappingsRequest) (domain.LookupAlertIncidentMappingsResponse, error)
}

// AnnouncementRequestService defines the operations available on the
// announcement_requests entity — see domain.AnnouncementRequest's own doc
// comment for the full state machine (draft -> pending_approval -> approved
// -> published) and what each transition does and doesn't allow.
type AnnouncementRequestService interface {
	// CreateDraft creates a new request in state draft. A ValidationError is
	// returned if kind isn't "customer"/"eol" or createdBy is missing.
	CreateDraft(ctx context.Context, req domain.CreateAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	// Get returns the request by id. A NotFoundError is returned if it
	// doesn't exist.
	Get(ctx context.Context, id string) (domain.AnnouncementRequest, error)
	// Search returns requests matching req's optional state/createdBy
	// filters, paginated.
	Search(ctx context.Context, req domain.SearchAnnouncementRequestsRequest) (domain.SearchAnnouncementRequestsResponse, error)
	// Update edits the request's own content — what actually happens
	// (plain edit, edit-and-revert-to-draft, or edit-in-place) depends
	// entirely on the request's current state; see the service's own
	// implementation doc comment for the full breakdown. A ConflictError is
	// returned if the request is published; a ValidationError if an
	// audience change is attempted while approved (the approved snapshot
	// is frozen — this is a rejected request shape, not a state conflict).
	Update(ctx context.Context, id string, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	// RecordDryRun records that a dry run (created by the caller's own
	// mechanism, not this service) has completed for this request. A
	// ConflictError is returned unless the current state is draft.
	RecordDryRun(ctx context.Context, id string, req domain.RecordAnnouncementDryRunRequest) (domain.AnnouncementRequest, error)
	// Submit moves draft -> pending_approval, freezing req.ResolvedProjectIDs
	// as the audience snapshot. A ConflictError is returned unless the
	// current state is draft and a dry run has already been recorded; a
	// ValidationError if resolvedProjectIds is empty.
	Submit(ctx context.Context, id string, req domain.SubmitAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	// Approve moves pending_approval -> approved. A ConflictError is
	// returned unless the current state is pending_approval. There is no
	// approver-role check — see the interface's own doc comment.
	Approve(ctx context.Context, id, actorID, actorEmail string) (domain.AnnouncementRequest, error)
	// MarkPublished moves approved -> published, storing caseIDs (the real
	// case created for each resolved project, from the caller's own
	// fan-out) as PublishedCaseIDs. Does not itself create any cases. A
	// ConflictError is returned unless the current state is approved; a
	// ValidationError if caseIDs is empty. Unlike Approve, this IS
	// restricted: a ForbiddenError is returned unless actorID matches the
	// request's own CreatedBy -- an approver's job is only to approve, not
	// to also trigger the real send to customers.
	MarkPublished(ctx context.Context, id, actorID, actorEmail string, caseIDs []string) (domain.AnnouncementRequest, error)
	// Schedule sets or clears (nil) scheduledFor for an approved request —
	// once set, operations/csm-scheduled-tasks' publish_scheduled_announcements
	// sub-cron publishes it automatically once that time arrives. A
	// ConflictError is returned unless the current state is approved; a
	// ForbiddenError unless actorID matches the request's own CreatedBy
	// (same creator-only restriction as MarkPublished — scheduling is
	// choosing when Publish happens); a ValidationError if scheduledFor is
	// non-nil and not strictly in the future.
	Schedule(ctx context.Context, id, actorID, actorEmail string, scheduledFor *time.Time) (domain.AnnouncementRequest, error)
	// AutoPublish runs the entire Publish fan-out in-process (create a case
	// per unresolved project, attach the security tag, record deliveries,
	// mark published) for one already-due scheduled request — the automatic
	// counterpart to the webapp's own manual Publish flow, callable only by
	// an internal service (a ForbiddenError otherwise). A ConflictError is
	// returned if the request isn't approved, its scheduled time hasn't
	// arrived yet, it has no resolved audience, or a pass still has
	// outstanding case-creation/tag failures (safe to call again — it
	// resumes from the delivery ledger exactly like a manual retry would).
	AutoPublish(ctx context.Context, id string) (domain.AnnouncementRequest, error)
	// AddUpdate posts a new AnnouncementRequestUpdate for a published
	// request. Does not itself apply Content as a comment anywhere -- the
	// caller's own fan-out does that, separately, after this call succeeds
	// (same separation as MarkPublished). A ConflictError is returned
	// unless the current state is published; a ForbiddenError unless
	// actorID matches the request's own CreatedBy (same creator-only
	// restriction as MarkPublished, for the same reason).
	AddUpdate(ctx context.Context, id, actorID, actorEmail, content string) (domain.AnnouncementRequestUpdate, error)
	// ListUpdates returns every update posted for id, newest first. A
	// NotFoundError is returned if the request itself doesn't exist.
	ListUpdates(ctx context.Context, id string) (domain.SearchAnnouncementRequestUpdatesResponse, error)
	// RecordDeliveries upserts one Publish fan-out pass's worth of
	// per-project outcomes for id. Same creator-only restriction as
	// MarkPublished, and for the same reason: this is bookkeeping for the
	// real send, which only the request's own creator can drive. A
	// ConflictError is returned unless the current state is approved (a
	// delivery only means anything mid-fan-out, before the request reaches
	// published); a ValidationError if deliveries is empty.
	RecordDeliveries(ctx context.Context, id, actorID string, deliveries []domain.RecordAnnouncementRequestDeliveryInput) (domain.SearchAnnouncementRequestDeliveriesResponse, error)
	// ListDeliveries returns every delivery recorded for id. A NotFoundError
	// is returned if the request itself doesn't exist.
	ListDeliveries(ctx context.Context, id string) (domain.SearchAnnouncementRequestDeliveriesResponse, error)
}

// SNAccountService defines the account operations backed by the ServiceNow data source.
type SNAccountService interface {
	// SearchAccounts returns a paginated list of ServiceNow accounts matching the
	// filters in req.
	SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) (domain.SearchAccountsResponse, error)
	// GetAccountByID returns the full account detail for the given UUID.
	GetAccountByID(ctx context.Context, id string) (domain.AccountDetail, error)
}

// ProjectService defines the operations available on the project entity.
type ProjectService interface {
	// SearchProjects returns a paginated list of projects that match the filters
	// in req. A ValidationError is returned for invalid input; any other error
	// indicates an infrastructure failure.
	SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) (domain.SearchProjectsResponse, error)
	// GetProjectByID returns the enriched project detail with the linked account.
	// A ValidationError is returned for a malformed UUID; a NotFoundError if no
	// project matches OR (Postgres data source) it exists but is outside the
	// caller's AccessScope -- see CLAUDE.md's "Token validation and
	// caller-scoped access" for the full rule and why existence is hidden
	// rather than returning a 403.
	GetProjectByID(ctx context.Context, id string) (domain.ProjectDetailsView, error)
}

// ProjectUpdateService defines the write operations available on a project.
// Implemented by snProjectUpdateService (DataSourceServiceNow, the full
// domain.ProjectUpdateRequest contract) and pgProjectUpdateService
// (DataSourcePostgres/DataSourcePostgresServiceNowDualWrite -- only the
// fields with a real Postgres column; see that type's own doc comment for
// exactly which, and why the rest are rejected rather than silently
// dropped).
type ProjectUpdateService interface {
	// UpdateProject applies the given field changes to the project identified by
	// id. A ValidationError is returned for a malformed UUID, an empty request,
	// or (Postgres data sources only) a field with no Postgres column; a
	// NotFoundError if no project matches; an UnauthorizedError if the caller
	// lacks the required SN role (ServiceNow data source) or has no resolvable
	// identity (Postgres data sources).
	UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResponse, error)
}

// ProjectMetadataService is the GetProjectMetadata slice of ProjectStatsService,
// split out because it's the one method of that interface with a Postgres-backed
// implementation (projectMetadataService) as well as the ServiceNow one --
// snProjectStatsService satisfies this interface structurally, so the same
// concrete value backs both ProjectStatsService and ProjectMetadataService in
// ServiceNow mode. See ProjectMetadataHandler.
type ProjectMetadataService interface {
	// GetProjectMetadata returns the reference data (choice lists, feature
	// flags) needed to build the project's UI.
	GetProjectMetadata(ctx context.Context, projectID string) (domain.ProjectMetadataResponse, error)
}

// ProjectCaseStatsService is the GetProjectCaseStats slice of
// ProjectStatsService, split out for the same reason ProjectMetadataService
// is: it has a Postgres-backed implementation (projectCaseStatsService) as
// well as the ServiceNow one, so it is wired and registered independently of
// the remaining stats methods, which stay ServiceNow-only. In ServiceNow mode
// snProjectStatsService satisfies this interface structurally, so the same
// concrete value backs both. See ProjectCaseStatsHandler.
type ProjectCaseStatsService interface {
	// GetProjectCaseStats returns the case statistics for a project,
	// optionally narrowed by case type and creator. A ValidationError is
	// returned for a malformed UUID or an unrecognised case type; a
	// NotFoundError if no project matches.
	GetProjectCaseStats(ctx context.Context, projectID string, req domain.ProjectCaseStatsRequest) (domain.ProjectCaseStatsResponse, error)
}

// ProjectStatsService defines the project-scoped metadata and statistics
// operations. Every method has both a ServiceNow implementation
// (snProjectStatsService) and a Postgres one (projectStatsService), so all of
// these routes are registered regardless of the data source.
//
// GetProjectMetadata and GetProjectCaseStats additionally have their own
// narrower interfaces (ProjectMetadataService, ProjectCaseStatsService):
// each was portable to Postgres before the rest of the bundle was, and the
// Postgres projectStatsService composes them rather than reimplementing
// either.
type ProjectStatsService interface {
	// GetProjectMetadata returns the reference data (choice lists, feature
	// flags) needed to build the project's UI.
	GetProjectMetadata(ctx context.Context, projectID string) (domain.ProjectMetadataResponse, error)
	// GetProjectStats returns the project's overall statistics.
	GetProjectStats(ctx context.Context, projectID string) (domain.ProjectStatsResponse, error)
	// GetProjectCaseStats returns the project's case statistics, optionally
	// filtered by case type and/or creator.
	GetProjectCaseStats(ctx context.Context, projectID string, req domain.ProjectCaseStatsRequest) (domain.ProjectCaseStatsResponse, error)
	// GetProjectConversationStats returns the project's conversation
	// statistics, optionally filtered by creator.
	GetProjectConversationStats(ctx context.Context, projectID, createdBy string) (domain.ProjectConversationStatsResponse, error)
	// GetProjectDeploymentStats returns the project's deployment statistics.
	GetProjectDeploymentStats(ctx context.Context, projectID string) (domain.ProjectDeploymentStatsResponse, error)
	// GetProjectTimeCardStats returns the project's time-card statistics,
	// optionally filtered by a startDate/endDate range (each yyyy-MM-dd).
	GetProjectTimeCardStats(ctx context.Context, projectID, startDate, endDate string) (domain.ProjectTimeCardStatsResponse, error)
	// GetProjectChangeRequestStats returns the project's change-request statistics.
	GetProjectChangeRequestStats(ctx context.Context, projectID string) (domain.ProjectChangeRequestStatsResponse, error)
}

// ProjectConsumptionService defines the operations on a project's
// product-consumption provisioning state — the resumable sequence that creates
// a Choreo application for the project, subscribes it to the tracking API and
// mints the credentials a deployment's license is built from.
//
// The two halves need different things, and neither is gated on DATA_SOURCE —
// staging and production both run DATA_SOURCE=servicenow and need both.
//
//   - GetProjectConsumption and UpdateProjectConsumption read and write the
//     Postgres mirror, so they need a pool. A pool enables them on either
//     data source.
//   - ProcessLicenseDownload needs neither. It reads status from ServiceNow
//     through the configured Choreo subscription operation and touches
//     Postgres only to mirror what it did, which is best-effort and skipped
//     entirely when there is no repository.
//
// ServiceNow remains the source of truth for the status itself. There it lives
// on the customer_project record, reached through the product-consumption
// scripted REST API that the Choreo subscription operation calls directly —
// neither this service nor the ServiceNow integration service sits in that
// path at all.
//
// Every method is scoped to the caller (see AccessService): the project id
// comes from the request path, so a caller who cannot see a project can
// neither read its provisioning state nor drive provisioning for it.
type ProjectConsumptionService interface {
	// GetProjectConsumption returns the project's current provisioning state.
	// A project that has never entered the flow reports status 1 (pending)
	// rather than a not-found error; an unknown project ID is not found.
	GetProjectConsumption(ctx context.Context, projectID string) (domain.ProjectConsumptionView, error)
	// UpdateProjectConsumption records the completion of one provisioning step.
	// The status may only move forward; a status that is not ahead of what is
	// stored returns the stored state unchanged instead of failing.
	UpdateProjectConsumption(ctx context.Context, projectID string, req domain.UpdateProjectConsumptionRequest) (domain.UpdateProjectConsumptionResponse, error)
	// ProcessLicenseDownload executes the 5-step resumable provisioning sequence
	// and issues the signed deployment license.
	ProcessLicenseDownload(ctx context.Context, projectID, deploymentID, email string) (domain.License, error)
}

// ProjectContactService defines the operations available on project contacts.
// The Postgres-backed implementation (projectContactService) reads from
// project_contact (migration 000022), joined through account_contact to
// "user" and through project_contact_group/project_group_role/project_role
// (migrations 000023-000025) for roles.
type ProjectContactService interface {
	// SearchProjectContacts returns a paginated list of contacts associated with
	// the project identified by projectID.
	SearchProjectContacts(ctx context.Context, projectID string, req domain.SearchProjectContactsRequest) (domain.SearchProjectContactsResponse, error)
	// GetProjectContact returns one contact's attributes for a single project: their
	// roles on it, their registration state and their notification preference. A
	// NotFoundError is returned when that contact is not a contact on that project.
	GetProjectContact(ctx context.Context, projectID, contactID string) (domain.ProjectContact, error)
}

// AccountContactService defines the operations available on account contacts.
// The Postgres-backed implementation (accountContactService) reads from the
// account_contact table (migration 000020), joined against "user" to
// resolve a display name/email where possible.
type AccountContactService interface {
	// SearchAccountContacts returns a paginated list of contacts associated with
	// the account identified by accountID.
	SearchAccountContacts(ctx context.Context, accountID string, req domain.SearchAccountContactsRequest) (domain.SearchAccountContactsResponse, error)
}

// OpportunityService defines the operations available on the opportunity entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type OpportunityService interface {
	// SearchOpportunities returns a paginated list of opportunities matching the
	// filters in req.
	SearchOpportunities(ctx context.Context, req domain.SearchOpportunitiesRequest) (domain.SearchOpportunitiesResponse, error)
	// GetOpportunityByID returns a single opportunity's detail. A NotFoundError is
	// returned when no opportunity matches.
	GetOpportunityByID(ctx context.Context, id string) (domain.Opportunity, error)
}

// InvoiceService defines the operations available on the invoice entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type InvoiceService interface {
	// SearchInvoices returns a paginated list of invoices matching the filters in req.
	SearchInvoices(ctx context.Context, req domain.SearchInvoicesRequest) (domain.SearchInvoicesResponse, error)
	// GetInvoiceByID returns a single invoice's detail. A NotFoundError is returned
	// when no invoice matches.
	GetInvoiceByID(ctx context.Context, id string) (domain.Invoice, error)
}

// ProjectOpportunityLinkService defines the operations available on
// project-opportunity links. ServiceNow data source only; there is no Postgres
// fallback, and no by-id fetch -- the underlying ServiceNow data has no
// single-record endpoint for this resource (search only).
type ProjectOpportunityLinkService interface {
	// SearchProjectOpportunityLinks returns a paginated list of project-opportunity
	// links matching the filters in req.
	SearchProjectOpportunityLinks(ctx context.Context, req domain.SearchProjectOpportunityLinksRequest) (domain.SearchProjectOpportunityLinksResponse, error)
}

// ProductService defines the operations available on the product entity.
type ProductService interface {
	// SearchProducts returns a paginated list of products that match the filters
	// in req. A ValidationError is returned for invalid input; any other error
	// indicates an infrastructure failure.
	SearchProducts(ctx context.Context, req domain.SearchProductsRequest) (domain.SearchProductsResponse, error)
}

// SNProductService defines the product operations backed by the ServiceNow data source.
type SNProductService interface {
	// SearchProducts returns a paginated list of ServiceNow products matching the
	// search query.
	SearchProducts(ctx context.Context, req domain.SearchProductsRequest) (domain.SearchSNProductsResponse, error)
}

// ProductVersionService defines the operations available on the product version entity.
type ProductVersionService interface {
	// SearchProductVersions returns a paginated list of product versions filtered
	// by product_id and optionally by version string. A ValidationError is returned
	// for invalid input; any other error indicates an infrastructure failure.
	SearchProductVersions(ctx context.Context, req domain.SearchProductVersionsRequest) (domain.SearchProductVersionsResponse, error)
}

// SNProductVersionService is the ServiceNow-backed variant of ProductVersionService.
// It returns SNProductVersion items with string date fields to avoid time.Parse errors
// on empty SN date strings.
type SNProductVersionService interface {
	SearchProductVersions(ctx context.Context, req domain.SearchProductVersionsRequest) (domain.SearchSNProductVersionsResponse, error)
}

// DeploymentService defines the operations available on the deployment entity.
type DeploymentService interface {
	// SearchDeployments returns a paginated list of deployments filtered by optional
	// project IDs, deployment type keys, and name search query. A ValidationError is
	// returned for invalid input; any other error indicates an infrastructure failure.
	SearchDeployments(ctx context.Context, req domain.SearchDeploymentsRequest) (domain.SearchDeploymentsResponse, error)
	// CreateDeployment creates a new deployment. Supported by the ServiceNow
	// data source, and by DATA_SOURCE=postgres-servicenow-dual-write (SN-first,
	// synchronous — see deploymentService.createDeploymentSNFirst). Not
	// supported by plain DATA_SOURCE=postgres.
	CreateDeployment(ctx context.Context, req domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error)
	// UpdateDeployment updates a deployment's name, type, description, or
	// deactivates it. Either detail fields or Active=false must be provided,
	// but not both. Supported by the ServiceNow data source, and by
	// DATA_SOURCE=postgres-servicenow-dual-write (Postgres-first, ServiceNow
	// mirrored asynchronously afterward). Not supported by plain
	// DATA_SOURCE=postgres.
	UpdateDeployment(ctx context.Context, req domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error)
}

// DeployedProductService defines the operations available on the deployed_products entity.
type DeployedProductService interface {
	// SearchDeployedProducts returns a paginated list of deployed products filtered by
	// optional deployment IDs. A ValidationError is returned for invalid input; any other
	// error indicates an infrastructure failure.
	SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) (domain.SearchDeployedProductsResponse, error)
	// SearchProjectsByProductVersion returns the paginated, deduplicated set
	// of projects running the given product version — the reverse of
	// SearchDeployedProducts' own DeploymentIDs-scoped lookup. A
	// ValidationError is returned for invalid input. Supported by the
	// ServiceNow data source only.
	SearchProjectsByProductVersion(ctx context.Context, req domain.SearchProjectsByProductVersionRequest) (domain.SearchProjectsByProductVersionResponse, error)
	// CreateDeployedProduct creates a new deployed product in ServiceNow.
	// Supported by the ServiceNow data source only.
	CreateDeployedProduct(ctx context.Context, req domain.CreateDeployedProductRequest) (domain.CreateDeployedProductResponse, error)
	// UpdateDeployedProduct updates a deployed product's cores, tps, description, update-level
	// history, or deactivates it. Either detail fields (which now include Updates, a whole-array
	// replace of the update-level history) or Active=false must be provided, but not both.
	// Supported by the ServiceNow data source only.
	UpdateDeployedProduct(ctx context.Context, req domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error)
	// SearchDeployedProductMetrics returns core-count metrics for the deployed product
	// identified by id, charted over req's date range. A ValidationError is returned for
	// invalid input (malformed UUID, invalid/unordered dates, or a range exceeding one year).
	// Supported by the ServiceNow data source only.
	SearchDeployedProductMetrics(ctx context.Context, id string, req domain.DeployedProductMetricsRequest) (domain.DeployedProductMetricsResponse, error)
	// SearchDeployedProductUsageCounts returns usage-count metrics for the deployed product
	// identified by id, charted over req's date range. Same validation as
	// SearchDeployedProductMetrics. Supported by the ServiceNow data source only.
	SearchDeployedProductUsageCounts(ctx context.Context, id string, req domain.DeployedProductUsageCountsRequest) (domain.DeployedProductUsageCountsResponse, error)
}

// CaseService defines the operations available on the cases entity.
type CaseService interface {
	// CreateCase creates a new case with auto-generated id, number, and internal_id.
	// State defaults to open. A ValidationError is returned for invalid input.
	CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error)
	// GetCaseByID returns the enriched case view for the given UUID. A
	// ValidationError is returned for a malformed UUID; a NotFoundError if no
	// case matches OR (Postgres data source) it exists but is outside the
	// caller's AccessScope -- see ProjectService.GetProjectByID's identical
	// note and CLAUDE.md for the full rule.
	GetCaseByID(ctx context.Context, id string) (domain.CaseView, error)
	// SearchCases returns a paginated list of cases filtered by optional project IDs,
	// deployment IDs, deployed product IDs, state keys, severity keys, and search query.
	// A ValidationError is returned for invalid input; any other error indicates an
	// infrastructure failure.
	SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error)
	// AggregateCases returns server-side aggregated counts of cases per value of
	// req.GroupBy (e.g. account), capped to the top req.MaxGroups buckets with
	// the remainder folded into AggregateResponse.OthersCount. A ValidationError
	// is returned for invalid input.
	AggregateCases(ctx context.Context, req domain.AggregateCasesRequest) (domain.AggregateResponse, error)
	// CreateCaseComment creates a new comment on the case identified by req.CaseID.
	// A ValidationError is returned for invalid input or constraint violations.
	CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error)
	// SearchCaseComments returns a paginated list of comments for the case identified
	// by req.CaseID. A ValidationError is returned for invalid input.
	SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error)
	// UpdateCase updates the state, severity, watch list, assignee, or internal-only
	// fix-ETA estimate (best-case/most-likely/worst-case) of a case.
	// A ValidationError is returned for invalid values or malformed UUID; a NotFoundError if no case matches.
	// WatchList is supported by both data sources (Postgres via work_item_watcher,
	// migration 000040) and is mutually exclusive with State/Severity/WorkState.
	// AssigneeEmail, BestCaseFixEta, MostLikelyFixEta, and WorstCaseFixEta are
	// only supported for the ServiceNow data source.
	// Transitioning State to closed is rejected with a ValidationError if the case has any
	// open task that is visible to the customer (the authoritative case-close gate).
	UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error)
	// CreateCaseAttachment uploads a new attachment for the case identified by req.CaseID.
	// A ValidationError is returned for invalid input. For the CSM-native (Postgres) data
	// source, req.Status controls the initial lifecycle state (see domain.AttachmentStatus):
	// empty/omitted and "complete" behave exactly as before this field existed; "pending"
	// registers the row before the caller has uploaded the file to SFTPGo, to be finished off
	// later via ConfirmCaseAttachment. ServiceNow ignores this field.
	CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error)
	// ConfirmCaseAttachment transitions the CSM-native (Postgres) data source attachment
	// identified by id from status "pending" to "complete", once its file has finished
	// uploading to SFTPGo. A NotFoundError is returned if it does not exist; a
	// ForbiddenError if the caller did not create it; a ConflictError if it is not
	// currently "pending" (including if it was already confirmed). Supported by the
	// CSM-native (Postgres) data source only -- ServiceNow attachments have no such
	// lifecycle, since SN's /attachments API only ever returns fully-uploaded files.
	ConfirmCaseAttachment(ctx context.Context, id string) (domain.ConfirmAttachmentResponse, error)
	// SearchCaseAttachments returns a paginated list of attachments for the case identified
	// by req.CaseID. A ValidationError is returned for invalid input.
	SearchCaseAttachments(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error)
	// SearchCaseActivities returns a paginated activity feed (comments, attachments, and
	// optionally field changes) for the case identified by req.CaseID. Field-change entries
	// are included only when req.IncludeFieldChanges is set. A ValidationError is returned
	// for invalid input. The Postgres-backed implementation merges comments and complete
	// attachments only -- there is no field-change audit table in this schema, so
	// req.IncludeFieldChanges has no effect there.
	SearchCaseActivities(ctx context.Context, req domain.SearchCaseActivitiesRequest) (domain.SearchCaseActivitiesResponse, error)
	// GetCaseAttachmentContent returns the raw binary content and its Content-Type
	// for the attachment identified by attachmentID.
	// A NotFoundError is returned if absent.
	GetCaseAttachmentContent(ctx context.Context, attachmentID string) (content []byte, contentType string, err error)
	// DeleteCaseAttachment removes the attachment identified by req.AttachmentID from the case.
	// A NotFoundError is returned if the attachment does not exist.
	DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error)
	// UpdateAttachment updates the name and/or description of the attachment identified by
	// req.AttachmentID, regardless of which reference type it is linked to. At least one of
	// Name or Description must be provided. A NotFoundError is returned if the attachment
	// does not exist.
	UpdateAttachment(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error)
	// AddCaseTag attaches a free-text label to the case identified by caseID.
	// A ValidationError is returned for invalid input (e.g. malformed UUID, empty label).
	AddCaseTag(ctx context.Context, caseID, label string) (domain.Tag, error)
	// AddCaseTagAs is AddCaseTag for a caller that already knows who is
	// acting (actorEmail) and has no live x-user-id-token to resolve it
	// from -- see AnnouncementRequestService.AutoPublish's own doc comment
	// for why that caller can never have one. Skips the token-based actor
	// resolution AddCaseTag does; everything else is identical.
	AddCaseTagAs(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error)
	// RemoveCaseTag removes the tag identified by tagID from the case identified by caseID.
	// A NotFoundError is returned if the tag does not exist on the case.
	RemoveCaseTag(ctx context.Context, caseID, tagID string) error
	// SearchTags returns the tags (not scoped to any single case) whose label matches
	// req.Filters.SearchQuery, for FE autocomplete when attaching a tag to a case. An empty
	// query returns all known tags. req.Limit caps the number of results (<=0 means use the
	// downstream default).
	SearchTags(ctx context.Context, req domain.SearchTagsRequest) ([]domain.Tag, error)
	// GetCaseFeedback returns the feedback previously submitted for the case identified
	// by id. A NotFoundError is returned if none has been submitted.
	// Supported by the ServiceNow data source only.
	GetCaseFeedback(ctx context.Context, id string) (domain.CaseEmojiFeedback, error)
	// SubmitCaseFeedback records feedback for the case identified by id. A ValidationError
	// is returned for invalid input. Supported by the ServiceNow data source only.
	SubmitCaseFeedback(ctx context.Context, id string, req domain.SubmitCaseFeedbackRequest) (domain.SubmitCaseFeedbackResponse, error)
	// GetAttachmentByID returns the metadata and base64-encoded content of the attachment
	// identified by id. A NotFoundError is returned if it does not exist. For the CSM-native
	// (Postgres) data source, Content is always "" and StorageKey is populated instead: that
	// data source holds no bytes, only a reference into external (SFTPGo) storage.
	GetAttachmentByID(ctx context.Context, id string) (domain.AttachmentDetails, error)
}

// CaseGithubIssueService defines the operation for filing a GitHub issue from a case.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type CaseGithubIssueService interface {
	// CreateCaseGithubIssue files a new GitHub issue on the internal repo mapped to the
	// case's product, and appends a work note on the case with the resulting issue URL.
	// A ValidationError is returned for invalid input; a NotFoundError if no case matches.
	CreateCaseGithubIssue(ctx context.Context, req domain.CreateCaseGithubIssueRequest) (domain.CreateCaseGithubIssueResponse, error)
}

// CaseEscalationService is a case-scoped convenience layer over
// EscalationService (below): it's what backs GET/POST /cases/{id}/escalations,
// delegating to EscalationService.SearchEscalations/CreateEscalation with the
// case's own filter/CaseID rather than duplicating the SN adapter. All methods
// require the ServiceNow data source; there is no Postgres fallback.
type CaseEscalationService interface {
	// SearchCaseEscalations returns the full escalation history for the given
	// case, newest first, plus CurrentNotifiedUsers (the most recent record's
	// notified-users list — who is authorized to de-escalate the case's
	// current level). A ValidationError is returned for a malformed case UUID.
	SearchCaseEscalations(ctx context.Context, caseID string) (domain.CaseEscalationHistory, error)
	// CreateCaseEscalation escalates or de-escalates the given case, then
	// records a work note on the case (verified live against SN dev data that
	// the backing API does not do this itself). Action defaults to ESCALATE
	// when nil; reason is required when escalating. A ValidationError is
	// returned for invalid input; a NotFoundError if no case matches.
	CreateCaseEscalation(ctx context.Context, caseID string, reason *string, action *domain.EscalationAction) (domain.CreatedEscalation, error)
}

// CatalogService defines the operations available on service catalogs. The
// Postgres-backed implementation (catalogService) reads sr_category,
// catalog_item, catalog_item_category, catalog_variable and
// sr_category_routing_rule (migrations 000067-000071) -- see catalog_repo.go
// for how a "catalog" and item availability are defined there.
type CatalogService interface {
	// SearchCatalogs returns catalogs available for the given deployed product.
	// DeployedProductID is required. A ValidationError is returned for missing input.
	SearchCatalogs(ctx context.Context, req domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error)
	// GetCatalogItemVariables returns the variables (form fields) for a specific catalog item.
	// A NotFoundError is returned if the catalog or item does not exist.
	GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) (domain.GetCatalogItemVariablesResponse, error)
}

// FeedbackService defines the operations available on the case-feedback (satisfaction
// rating) entity, for the case-feedback dashboard's list and rating-trend views.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type FeedbackService interface {
	// SearchFeedback returns a paginated list of case-feedback records, optionally
	// filtered by case, accounts, and submission date range. A ValidationError is
	// returned for invalid input.
	SearchFeedback(ctx context.Context, req domain.SearchFeedbackRequest) (domain.SearchFeedbackResponse, error)
	// AggregateFeedback returns date-bucketed average-rating aggregates across cases,
	// optionally filtered by accounts and submission date range. Bucket is required.
	// A ValidationError is returned for invalid input.
	AggregateFeedback(ctx context.Context, req domain.AggregateFeedbackRequest) (domain.AggregateFeedbackResponse, error)
}

// CallRequestService defines the operations available on call requests. The
// Postgres-backed implementation (callRequestService) reads and writes
// customer_call (migration 000072) -- see call_request_repo.go for the fields
// with no backing column.
type CallRequestService interface {
	// CreateCallRequest creates a new call request for the given case.
	// A ValidationError is returned for invalid input.
	CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error)
	// SearchCallRequests returns a paginated list of call requests for the given case.
	// A ValidationError is returned for invalid input.
	SearchCallRequests(ctx context.Context, req domain.SearchCallRequestsRequest) (domain.SearchCallRequestsResponse, error)
	// SearchAllCallRequests returns a paginated list of call requests across all
	// cases, filtered by assignee/state -- distinct from SearchCallRequests, which
	// is scoped to one case and has no filter set of its own.
	// A ValidationError is returned for invalid input.
	SearchAllCallRequests(ctx context.Context, req domain.SearchAllCallRequestsRequest) (domain.SearchCallRequestsResponse, error)
	// UpdateCallRequest updates the state or other fields of a call request.
	// The target state selects the behaviour (customer/agent transitions, scheduling,
	// rejection, conclusion with notes). A ValidationError is returned for invalid
	// input; a NotFoundError if no call request matches.
	UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error)
}

// ChangeRequestService defines the operations available on the change_requests
// entity. The Postgres-backed implementation (changeRequestService) reads
// from change_request (migration 000047), a shared-PK extension of
// work_item -- see that repository's own doc comment for the fields with no
// real column at all (ServiceID/ServiceOfferingID/ConfigurationItemID/
// GroupID/AssignedTeamID/Type/ApprovedBy/ApprovedOn/LegalNextStates).
// CreateChangeRequest and both approval methods have no Postgres
// implementation: the first needs a number-generation scheme this schema
// doesn't have (same blocker as CaseService.CreateCase); the other two need
// per-stage, per-approver approval records this schema doesn't have either.
type ChangeRequestService interface {
	// CreateChangeRequest creates a new change request in ServiceNow. Subject is required.
	// Supported by the ServiceNow data source only.
	CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error)

	// SearchChangeRequests returns a paginated list of change requests filtered by optional
	// project IDs, state keys, impact keys, date ranges, and search query.
	SearchChangeRequests(ctx context.Context, req domain.SearchChangeRequestsRequest) (domain.SearchChangeRequestsResponse, error)

	// AggregateChangeRequests returns server-side aggregated counts of change requests
	// per value of req.GroupBy, capped to the top req.MaxGroups buckets with the
	// remainder folded into AggregateResponse.OthersCount. A ValidationError is
	// returned for invalid input.
	AggregateChangeRequests(ctx context.Context, req domain.AggregateChangeRequestsRequest) (domain.AggregateResponse, error)

	// GetChangeRequest returns the full detail of a single change request by its UUID.
	GetChangeRequest(ctx context.Context, id string) (domain.ChangeRequest, error)

	// PatchChangeRequest updates mutable fields on a change request identified by UUID.
	PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error)

	// GetChangeRequestApprovals returns the approval stages and per-approver status
	// for a single change request identified by UUID. Supported by the ServiceNow data
	// source only.
	GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error)

	// DecideChangeRequestApproval submits the caller's decision ("approved" or
	// "rejected") on their own pending approval for a change request identified
	// by UUID. Supported by the ServiceNow data source only.
	DecideChangeRequestApproval(ctx context.Context, id, decision string) (domain.ChangeRequestApprovalDecisionResponse, error)
}

// TimeCardService defines the operations available on the time-cards entity.
// The Postgres-backed implementation (timeCardService) resolves the caller's
// identity from their x-user-id-token the same way caseService does for case
// comments (see repository.TimeCardRepository), rather than forwarding a
// token to a downstream service the way the ServiceNow-backed implementation
// does -- so "SN enforces authorization" below applies to that implementation
// only; the Postgres-backed one enforces submitter/state checks itself
// (e.g. UpdateTimeCardFields' "AND state = 'submitted'" guard).
type TimeCardService interface {
	// SearchTimeCards returns a paginated list of time cards filtered by optional
	// project IDs, case, user, approver, date range, and states.
	SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchTimeCardsResponse, error)
	// CreateTimeCard logs a new time card against a case in the submitted state.
	CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error)
	// UpdateTimeCard edits an editable (submitted) time card, or transitions its
	// state (approve/reject) when req.State is set. SN enforces authorization.
	UpdateTimeCard(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error)
	// SearchCaseTimeCards returns a paginated list of time cards grouped and
	// rolled up by case, using the same filters as SearchTimeCards.
	SearchCaseTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchCaseTimeCardsResponse, error)
	// DeleteTimeCard permanently deletes a time card. Matches UpdateTimeCard's
	// trust model exactly: this only validates the ID's shape and forwards the
	// caller's token to SN, which enforces that only the submitter may delete
	// their own card, and only while it's still in the submitted state — see
	// UpdateTimeCard's own doc comment for why that authorization isn't (and,
	// consistent with every other write here, shouldn't be) duplicated in Go.
	DeleteTimeCard(ctx context.Context, req domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error)
}

// ConfigurationItemService defines the operations available on the configuration items entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ConfigurationItemService interface {
	// SearchConfigurationItems returns a paginated list of CMDB configuration items filtered by
	// optional search query.
	SearchConfigurationItems(ctx context.Context, req domain.SearchConfigurationItemsRequest) (domain.SearchConfigurationItemsResponse, error)
}

// GroupService defines the operations available on the groups entity. On
// Postgres this is backed by the team table (migration 000028); Group.Active
// is always true and Group.Parent always nil there -- see
// GroupRepository's own doc comment.
type GroupService interface {
	// SearchGroups returns a paginated list of groups filtered by optional search query.
	SearchGroups(ctx context.Context, req domain.SearchGroupsRequest) (domain.SearchGroupsResponse, error)
}

// ServiceOfferingService defines the operations available on the service
// offerings entity. On Postgres this is backed by the service_offering
// table (migration 000049).
type ServiceOfferingService interface {
	// SearchServiceOfferings returns a paginated list of service offerings filtered by
	// optional service IDs.
	SearchServiceOfferings(ctx context.Context, req domain.SearchServiceOfferingsRequest) (domain.SearchServiceOfferingsResponse, error)
}

// ITServiceService defines the operations available on the CMDB IT services entity.
// On Postgres this is backed by the standalone service table (migration
// 000048); ServiceClassification always comes back nil there -- see
// ITServiceRepository's own doc comment for why.
type ITServiceService interface {
	// SearchITServices returns a paginated list of services.
	SearchITServices(ctx context.Context, req domain.SearchITServicesRequest) (domain.SearchITServicesResponse, error)
}

// CommentService defines generic comment search operations across all reference types
// (case, conversation, change_request, etc.).
// The Postgres-backed implementation (commentService) supports referenceType
// "case", "conversation", "change_request", and "incident" -- every work_item
// subtype the comment table's work_item_id foreign key can point at (see
// repository.ReferenceTypeToWorkItemType). "deployment" is ServiceNow-only:
// deployment is its own standalone table (migration 000013), not a work_item
// subtype, so a Postgres-backed comment can never reference one.
type CommentService interface {
	// SearchComments returns a paginated list of comments for the given reference entity.
	SearchComments(ctx context.Context, req domain.SearchCommentsRequest) (domain.SearchCommentsResponse, error)
	// CreateComment creates a new comment on the given reference entity.
	CreateComment(ctx context.Context, req domain.CreateCommentRequest) (domain.CreateCommentResponse, error)
	// UpdateComment edits an existing comment's content. Only the comment's
	// original author or a caller holding the "admin" role may call this; see
	// commentService.UpdateComment's own doc comment for the authorization
	// rule. Returns a ForbiddenError if the caller may not edit this comment,
	// a NotFoundError if it doesn't exist, and a ValidationError if it is
	// already soft-deleted.
	UpdateComment(ctx context.Context, req domain.UpdateCommentRequest) (domain.UpdateCommentResponse, error)
	// DeleteComment soft-deletes a comment (content is retained but no longer
	// generally visible -- see commentService's own visibility rule doc
	// comment). Same author-or-admin authorization rule as UpdateComment.
	// Returns a ConflictError if the comment is already deleted.
	DeleteComment(ctx context.Context, id string) error
	// GetCommentEditHistory returns a comment's prior versions, newest first.
	GetCommentEditHistory(ctx context.Context, id string) (domain.GetCommentEditHistoryResponse, error)
}

// TaskSlaService defines the operations available on the task-slas entity.
// On Postgres this is backed by sla/sla_policy (migrations 000051/000052) --
// see TaskSlaRepository's own doc comment for the fields with no confirmed
// rendering format that are left nil there.
type TaskSlaService interface {
	// SearchTaskSlas returns a paginated list of task SLA records filtered by optional task IDs.
	SearchTaskSlas(ctx context.Context, req domain.SearchTaskSlasRequest) (domain.SearchTaskSlasResponse, error)
	// GetTaskSla returns the full detail of a single task SLA record by its UUID.
	// A NotFoundError is returned if the record does not exist.
	GetTaskSla(ctx context.Context, id string) (domain.TaskSlaDetail, error)
}

// TaskService defines the operations available on the tasks entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type TaskService interface {
	// SearchCaseTasks returns a paginated list of tasks for the case identified by
	// caseID. A ValidationError is returned for invalid input (e.g. malformed UUID).
	SearchCaseTasks(ctx context.Context, caseID string, req domain.SearchCaseTasksRequest) (domain.SearchCaseTasksResponse, error)
	// SearchTasks returns a paginated list of all tasks filtered by optional state, type,
	// assigned user ID, and due date range. A ValidationError is returned for invalid input.
	SearchTasks(ctx context.Context, req domain.SearchTasksRequest) (domain.SearchTasksResponse, error)
	// GetTask returns the full detail of a single task by its UUID.
	// A NotFoundError is returned if the task does not exist.
	GetTask(ctx context.Context, id string) (domain.TaskDetail, error)
	// CreateCaseTask creates a new task on the case identified by caseID.
	// A ValidationError is returned for invalid input (e.g. malformed UUID, empty subject).
	// Returns a ServiceUnavailableError until the downstream endpoint ships (not yet
	// available in the backing service); see CreateCaseTaskRequest doc comment.
	CreateCaseTask(ctx context.Context, caseID string, req domain.CreateCaseTaskRequest) (domain.TaskDetail, error)
	// UpdateTask updates exactly one of state, assignedToEmail, or dueDate on the task
	// identified by taskID. A ValidationError is returned for invalid values, a malformed
	// UUID, or if zero or more than one field is provided.
	// Returns a ServiceUnavailableError until the downstream endpoint ships, same as CreateCaseTask above.
	UpdateTask(ctx context.Context, taskID string, req domain.UpdateTaskRequest) (domain.TaskDetail, error)
}

// ProductVulnerabilityService defines the operations available on product vulnerabilities.
// The Postgres-backed implementation (productVulnerabilityService) reads
// from the product_vulnerability table (migration 000034), which mirrors
// ServiceNow's own record 1:1 -- see that migration's own doc comment.
// SyncProductVulnerabilities is the one method with no Postgres equivalent
// (see its own doc comment for why).
type ProductVulnerabilityService interface {
	// SearchProductVulnerabilities returns a paginated list of vulnerabilities filtered by
	// optional priority, product name, product version, and search query.
	// A ValidationError is returned for invalid input.
	SearchProductVulnerabilities(ctx context.Context, req domain.SearchProductVulnerabilitiesRequest) (domain.SearchProductVulnerabilitiesResponse, error)

	// GetProductVulnerability returns the detail of a single vulnerability by its UUID.
	// A NotFoundError is returned if the vulnerability does not exist.
	GetProductVulnerability(ctx context.Context, id string) (domain.ProductVulnerabilityView, error)

	// GetVulnerabilityMeta returns the valid severity choices for product vulnerabilities.
	GetVulnerabilityMeta(ctx context.Context) (domain.VulnerabilityMetaResponse, error)
	// SyncProductVulnerabilities replaces the full set of product vulnerabilities with the
	// given items. This is a full-replace sync: ServiceNow deletes any existing record whose
	// WSO2ID is not present in items and upserts everything that is. Callers MUST submit the
	// complete current set, never a partial delta, or downstream records will be deleted.
	// Supported by the ServiceNow data source only: the full-replace semantics need a stable
	// external join key with a database-enforced uniqueness guarantee to upsert against, and
	// product_vulnerability has no UNIQUE constraint on any column other than its own
	// generated id -- adding one is a schema change, out of scope here (see
	// productVulnerabilityService.SyncProductVulnerabilities).
	SyncProductVulnerabilities(ctx context.Context, items []domain.ProductVulnerabilitySyncItem) (domain.ProductVulnerabilitySyncResult, error)
}

// AlertService defines the operations available on alerts.
type AlertService interface {
	// GetAlertByID returns the detail of a single alert by its UUID.
	// A NotFoundError is returned if the alert does not exist.
	GetAlertByID(ctx context.Context, id string) (domain.AlertView, error)
}

// SmartAlertService defines the operations available on smart alerts.
type SmartAlertService interface {
	// GetSmartAlertByID returns the detail of a single smart alert by its UUID.
	// A NotFoundError is returned if the smart alert does not exist.
	GetSmartAlertByID(ctx context.Context, id string) (domain.SmartAlertView, error)
}

// IncidentService defines the operations available on the incidents entity.
type IncidentService interface {
	// SearchIncidents returns a paginated list of incidents filtered by optional search query,
	// priority keys, and parent IDs. A ValidationError is returned for invalid input.
	SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error)

	// AggregateIncidents returns server-side aggregated counts of incidents per
	// value of req.GroupBy, capped to the top req.MaxGroups buckets with the
	// remainder folded into AggregateResponse.OthersCount. A ValidationError is
	// returned for invalid input.
	AggregateIncidents(ctx context.Context, req domain.AggregateIncidentsRequest) (domain.AggregateResponse, error)

	// CreateIncident creates a new incident in ServiceNow.
	// callerId, category, serviceId, impact, urgency, and subject are required.
	CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error)

	// GetIncidentByID returns the full detail of a single incident by its UUID.
	// A NotFoundError is returned if the incident does not exist.
	GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error)

	// UpdateIncident partially updates an existing incident. At least one field must be
	// provided. A NotFoundError is returned if the incident does not exist.
	UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error)

	// SearchIncidentActivities returns a paginated activity feed for an incident.
	// Confirmed as a real, distinct endpoint from SearchCaseActivities.
	SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) (domain.SearchIncidentActivitiesResponse, error)

	// HandOffIncidentToSpecialist hands an incident off to its specialist group in one
	// call: moves the incident to the specialist group for its business service, clears
	// the assignee, opens a runbook-gap task, and (by default) files an internal issue for
	// the receiving team. The internal issue filing is best-effort, not atomic with the
	// rest of the handoff: a 200 response means the handoff itself succeeded even if the
	// issue could not be created, in which case the response's GithubIssueError is set and
	// callers must check it rather than assume all-or-nothing. A ValidationError is
	// returned for invalid input, a NotFoundError if the incident does not exist, and a
	// ConflictError if the incident is not eligible (wrong business service, not in
	// progress, or already with the specialist group for this service).
	HandOffIncidentToSpecialist(ctx context.Context, req domain.HandOffIncidentToSpecialistRequest) (domain.HandOffIncidentToSpecialistResponse, error)
}

// ProblemService defines the operations available on the problems entity.
type ProblemService interface {
	// SearchProblems returns a paginated list of problems filtered by optional search query.
	// A ValidationError is returned for invalid input.
	SearchProblems(ctx context.Context, req domain.SearchProblemsRequest) (domain.SearchProblemsResponse, error)

	// AggregateProblems returns server-side aggregated counts of problems per
	// value of req.GroupBy, capped to the top req.MaxGroups buckets with the
	// remainder folded into AggregateResponse.OthersCount. A ValidationError is
	// returned for invalid input.
	AggregateProblems(ctx context.Context, req domain.AggregateProblemsRequest) (domain.AggregateResponse, error)

	// GetProblem returns the full detail of a single problem by its UUID.
	// A NotFoundError is returned if the problem does not exist.
	GetProblem(ctx context.Context, id string) (domain.ProblemDetail, error)

	// CreateProblem creates a new problem. Subject is required; OriginCaseID is optional.
	// Supported by the ServiceNow data source only.
	CreateProblem(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error)

	// UpdateProblem partially updates an existing problem -- a forward state transition
	// (assess/confirm/fix/resolve/close), plain-field updates, or both in the same request. At
	// least one field must be provided. Transition is validated by the data source, not here.
	// A NotFoundError is returned if the problem does not exist; a ConflictError if the data
	// source rejects or reverts the requested transition.
	UpdateProblem(ctx context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error)
}

// IncidentTaskService defines the operations available on the incident_task entity.
// Search and get only -- there is no create/update path.
type IncidentTaskService interface {
	// SearchIncidentTasks returns a paginated list of incident tasks filtered by
	// optional search query and field filters. A ValidationError is returned for
	// invalid input.
	SearchIncidentTasks(ctx context.Context, req domain.SearchIncidentTasksRequest) (domain.SearchIncidentTasksResponse, error)

	// AggregateIncidentTasks returns server-side aggregated counts of incident
	// tasks per value of req.GroupBy, capped to the top req.MaxGroups buckets
	// with the remainder folded into AggregateResponse.OthersCount. A
	// ValidationError is returned for invalid input.
	AggregateIncidentTasks(ctx context.Context, req domain.AggregateIncidentTasksRequest) (domain.AggregateResponse, error)

	// GetIncidentTask returns the full detail of a single incident task by its UUID.
	// A NotFoundError is returned if the incident task does not exist.
	GetIncidentTask(ctx context.Context, id string) (domain.IncidentTaskDetail, error)
}

// ConversationService defines the operations available on the conversations entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ConversationService interface {
	// SearchConversations returns a paginated list of conversations filtered by optional
	// project IDs, states, search query, and createdByMe. A ValidationError is returned
	// for invalid input.
	SearchConversations(ctx context.Context, req domain.SearchConversationsRequest) (domain.SearchConversationsResponse, error)
	// GetConversation returns the detail of a single conversation by its UUID.
	// A NotFoundError is returned if the conversation does not exist.
	GetConversation(ctx context.Context, id string) (domain.ConversationDetails, error)
	// CreateConversation starts a new conversation on the project identified by
	// req.ProjectID. A ValidationError is returned for invalid input.
	CreateConversation(ctx context.Context, req domain.CreateConversationRequest) (domain.CreateConversationResponse, error)
	// UpdateConversation transitions the conversation identified by id to req.State. A
	// ValidationError is returned if State is not one of ACTIVE, RESOLVED, CONVERTED,
	// ABANDONED, or CLOSED.
	UpdateConversation(ctx context.Context, id string, req domain.UpdateConversationRequest) (domain.UpdateConversationResponse, error)
}

// GlobalService serves system-wide metadata and cross-entity search that
// isn't scoped to any single project or case. Both methods have Postgres-backed
// implementations (globalService); GlobalSearch on that data source returns
// only what the caller may see (AccessService), so it also needs token
// validation to be configured.
type GlobalService interface {
	// GetSystemMetadata returns system-wide reference data (time zones, project types,
	// and feedback emoji choices) used across the frontend.
	GetSystemMetadata(ctx context.Context) (domain.SystemMetadataResponse, error)
	// GlobalSearch searches projects and/or cases matching req's filters. Every field of
	// req is optional; an empty request searches both tables with default pagination.
	GlobalSearch(ctx context.Context, req domain.GlobalSearchRequest) (domain.GlobalSearchResponse, error)
}

// EscalationService defines the operations available on the escalations
// entity. On Postgres, SearchEscalations is backed by case_escalation/
// case_escalation_notification_list (migration 000053); CreateEscalation
// requires the ServiceNow data source -- see EscalationRepository's own doc
// comment for why.
type EscalationService interface {
	// SearchEscalations returns a paginated list of escalations filtered by optional case
	// IDs and current escalation levels. A ValidationError is returned for invalid input.
	SearchEscalations(ctx context.Context, req domain.SearchEscalationsRequest) (domain.SearchEscalationsResponse, error)
	// CreateEscalation escalates or de-escalates the case identified by req.CaseID.
	// Action defaults to ESCALATE when omitted; Reason is required when the (defaulted)
	// action is ESCALATE. A ValidationError is returned for invalid input.
	CreateEscalation(ctx context.Context, req domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error)
}

// InstanceService defines the operations available on the instances entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type InstanceService interface {
	// SearchInstances returns a paginated list of instances filtered by optional
	// project/deployment/deployed-product IDs (mutually exclusive) and date range.
	// A ValidationError is returned for invalid input.
	SearchInstances(ctx context.Context, req domain.SearchInstancesRequest) (domain.SearchInstancesResponse, error)
	// SearchInstanceMetrics returns per-instance metric time series over req's required
	// date range, filtered by optional project/deployment/deployed-product IDs (mutually
	// exclusive). A ValidationError is returned for invalid input.
	SearchInstanceMetrics(ctx context.Context, req domain.InstanceMetricsRequest) (domain.InstanceMetricsResponse, error)
	// SearchInstanceUsage returns per-instance usage time series over req's required date
	// range. Same filter rules as SearchInstanceMetrics.
	SearchInstanceUsage(ctx context.Context, req domain.InstanceUsageRequest) (domain.InstanceUsageResponse, error)
	// SearchInstanceMetricsStats returns aggregated metric statistics over req's required
	// date range. Same filter rules as SearchInstanceMetrics, plus an optional data-source
	// filter.
	SearchInstanceMetricsStats(ctx context.Context, req domain.InstanceMetricsStatsRequest) (domain.InstanceMetricsStatsResponse, error)
	// SearchInstanceUsageStats returns aggregated usage statistics over req's required
	// date range. Same filter rules as SearchInstanceMetricsStats.
	SearchInstanceUsageStats(ctx context.Context, req domain.InstanceUsageStatsRequest) (domain.InstanceUsageStatsResponse, error)
}

// KBArticleService defines the operations available on the kb_article entity.
type KBArticleService interface {
	// CreateKBArticle creates a new article in the draft state.
	CreateKBArticle(ctx context.Context, req domain.CreateKBArticleRequest) (domain.CreateKBArticleResponse, error)
	// GetKBArticle returns a single article by id.
	GetKBArticle(ctx context.Context, id string) (domain.KBArticle, error)
	// SearchKBArticles returns a paginated list of articles filtered by
	// knowledge base, state, author, and title search query.
	SearchKBArticles(ctx context.Context, req domain.SearchKBArticlesRequest) (domain.SearchKBArticlesResponse, error)
	// UpdateKBArticleState transitions an article's state. Illegal transitions
	// return a ValidationError.
	UpdateKBArticleState(ctx context.Context, id string, req domain.UpdateKBArticleStateRequest) (domain.UpdateKBArticleStateResponse, error)
	// UpdateKBArticleContent edits an existing draft's title/body.
	UpdateKBArticleContent(ctx context.Context, id string, req domain.UpdateKBArticleContentRequest) (domain.KBArticle, error)
	DeleteKBArticle(ctx context.Context, id string) error
	ListKBArticleHistory(ctx context.Context, kbArticleID string) (domain.ListKBArticleHistoryResponse, error)
}

// KBManagerService defines the operations available on the kb_manager entity.
type KBManagerService interface {
	// SearchKBManagers returns kb_managers rows matching the given filters.
	// Called with both knowledgeBaseId and userId set, an empty result means
	// "this user does not manage this knowledge base."
	SearchKBManagers(ctx context.Context, req domain.SearchKBManagersRequest) (domain.SearchKBManagersResponse, error)
	// CreateKBManager grants a user approver access to a knowledge base.
	CreateKBManager(ctx context.Context, req domain.CreateKBManagerRequest) (domain.KBManager, error)
	// DeleteKBManager revokes a user's approver access to a knowledge base.
	DeleteKBManager(ctx context.Context, knowledgeBaseID, userID string) error
}

// KnowledgeBaseService defines the operations available on the knowledge_base entity.
type KnowledgeBaseService interface {
	ListKnowledgeBases(ctx context.Context) (domain.ListKnowledgeBasesResponse, error)
	CreateKnowledgeBase(ctx context.Context, req domain.CreateKnowledgeBaseRequest) (domain.KnowledgeBase, error)
	UpdateKnowledgeBaseName(ctx context.Context, id string, req domain.UpdateKnowledgeBaseRequest) (domain.KnowledgeBase, error)
	SetKnowledgeBaseActive(ctx context.Context, id string, req domain.UpdateKnowledgeBaseActiveRequest) (domain.KnowledgeBase, error)
}

// OutageService defines the operations available on the outages entity. All
// methods require the ServiceNow data source; there is no Postgres fallback.
type OutageService interface {
	// CreateOutage creates a new outage. Type, Begin, and ShortDescription are
	// required. AcknowledgePublicPublication is required when the resolved
	// configuration item publishes to a status page; omitting it in that case
	// returns a ConflictError.
	CreateOutage(ctx context.Context, req domain.CreateOutageRequest) (domain.CreateOutageResponse, error)

	// SearchOutages returns a paginated list of outages filtered by optional
	// type, status, configuration item, incident, and date-range criteria.
	// A ValidationError is returned for invalid input.
	SearchOutages(ctx context.Context, req domain.SearchOutagesRequest) (domain.SearchOutagesResponse, error)

	// GetOutageByID returns the full detail of a single outage by its UUID,
	// including per-channel communication counts. A NotFoundError is returned
	// if the outage does not exist.
	GetOutageByID(ctx context.Context, id string) (domain.OutageDetail, error)

	// UpdateOutage applies a partial update to an outage. Closing an outage is
	// done by setting End; there is no separate state field or close verb.
	// A ValidationError is returned if no field is provided.
	UpdateOutage(ctx context.Context, req domain.PatchOutageRequest) (domain.PatchOutageResponse, error)

	// AddOutageCommunication appends a communication journal entry to an
	// outage. An external entry is a publishing action and is subject to the
	// same publication-acknowledgement gate as CreateOutage.
	AddOutageCommunication(ctx context.Context, req domain.AddOutageCommunicationRequest) (domain.AddOutageCommunicationResponse, error)

	// SearchOutageCommunications returns a paginated list of an outage's
	// communication journal entries, optionally filtered by channel.
	SearchOutageCommunications(ctx context.Context, req domain.SearchOutageCommunicationsRequest) (domain.SearchOutageCommunicationsResponse, error)

	// GetOutageMetadata returns the live choice lists (types, statuses,
	// channels, monitored clouds) needed to render an outage create/edit form.
	GetOutageMetadata(ctx context.Context) (domain.OutageMetadataResponse, error)
}
