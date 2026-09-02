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

// SLAClockService defines the operations available on the sla_clocks
// entity — see domain.SLAClock's doc comment for what it's for.
type SLAClockService interface {
	// RegisterSLAClock (re)creates the clock for req.CaseID/req.ClockType. A
	// ValidationError is returned if caseId, clockType is missing, or dueAt
	// is not after startedAt.
	RegisterSLAClock(ctx context.Context, req domain.RegisterSLAClockRequest) (domain.SLAClock, error)
	// GetSLAClock returns the clock for caseID/clockType. A NotFoundError is
	// returned if no such clock has been registered.
	GetSLAClock(ctx context.Context, caseID, clockType string) (domain.SLAClock, error)
	// SetSLAClockTierReached marks tier ("50"/"75"/"100") reached for
	// caseID/clockType if it isn't already (req.Status must be
	// domain.SLATierStatusReached), and returns the (possibly pre-existing)
	// reached timestamp. A ValidationError is returned for an unrecognized
	// tier or status; a NotFoundError if no such clock has been registered.
	SetSLAClockTierReached(ctx context.Context, caseID, clockType, tier string, req domain.SetSLAClockTierRequest) (domain.SetSLAClockTierReachedResponse, error)
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
	// A ValidationError is returned for a malformed UUID; a NotFoundError if no project matches.
	GetProjectByID(ctx context.Context, id string) (domain.ProjectDetailsView, error)
}

// ProjectUpdateService defines the write operations available on a project.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ProjectUpdateService interface {
	// UpdateProject applies the given field changes to the project identified by
	// id. A ValidationError is returned for a malformed UUID or an empty request;
	// a NotFoundError if no project matches; an UnauthorizedError if the caller
	// lacks the required SN role.
	UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResponse, error)
}

// ProjectStatsService defines the project-scoped metadata and statistics
// operations. All methods require the ServiceNow data source; there is no
// Postgres fallback.
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

// ProjectContactService defines the operations available on project contacts.
// All methods require the ServiceNow data source; there is no Postgres fallback.
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
// All methods require the ServiceNow data source; there is no Postgres fallback.
type AccountContactService interface {
	// SearchAccountContacts returns a paginated list of contacts associated with
	// the account identified by accountID.
	SearchAccountContacts(ctx context.Context, accountID string, req domain.SearchAccountContactsRequest) (domain.SearchAccountContactsResponse, error)
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
	// CreateDeployment creates a new deployment in ServiceNow.
	// Supported by the ServiceNow data source only.
	CreateDeployment(ctx context.Context, req domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error)
	// UpdateDeployment updates a deployment's name, type, description, or deactivates it.
	// Either detail fields or Active=false must be provided, but not both.
	// Supported by the ServiceNow data source only.
	UpdateDeployment(ctx context.Context, req domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error)
}

// DeployedProductService defines the operations available on the deployed_products entity.
type DeployedProductService interface {
	// SearchDeployedProducts returns a paginated list of deployed products filtered by
	// optional deployment IDs. A ValidationError is returned for invalid input; any other
	// error indicates an infrastructure failure.
	SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) (domain.SearchDeployedProductsResponse, error)
	// CreateDeployedProduct creates a new deployed product in ServiceNow.
	// Supported by the ServiceNow data source only.
	CreateDeployedProduct(ctx context.Context, req domain.CreateDeployedProductRequest) (domain.CreateDeployedProductResponse, error)
	// UpdateDeployedProduct updates a deployed product's cores, tps, description, or deactivates it.
	// Either detail fields or Active=false must be provided, but not both.
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
	// ValidationError is returned for a malformed UUID; a NotFoundError if no case matches.
	GetCaseByID(ctx context.Context, id string) (domain.CaseView, error)
	// SearchCases returns a paginated list of cases filtered by optional project IDs,
	// deployment IDs, deployed product IDs, state keys, severity keys, and search query.
	// A ValidationError is returned for invalid input; any other error indicates an
	// infrastructure failure.
	SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error)
	// GroupCasesBy returns server-side aggregated counts of cases per value of
	// req.GroupBy (e.g. account), capped to the top req.MaxGroups buckets with
	// the remainder folded into GroupByResponse.OthersCount. A ValidationError
	// is returned for invalid input.
	GroupCasesBy(ctx context.Context, req domain.GroupCasesByRequest) (domain.GroupByResponse, error)
	// CreateCaseComment creates a new comment on the case identified by req.CaseID.
	// A ValidationError is returned for invalid input or constraint violations.
	CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error)
	// SearchCaseComments returns a paginated list of comments for the case identified
	// by req.CaseID. A ValidationError is returned for invalid input.
	SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error)
	// UpdateCase updates the state, severity, watch list, assignee, or internal-only
	// fix-ETA estimate (best-case/most-likely/worst-case) of a case.
	// A ValidationError is returned for invalid values or malformed UUID; a NotFoundError if no case matches.
	// WatchList, AssigneeEmail, BestCaseFixEta, MostLikelyFixEta, and WorstCaseFixEta are
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
	// for invalid input. Supported by the ServiceNow data source only.
	SearchCaseActivities(ctx context.Context, req domain.SearchCaseActivitiesRequest) (domain.SearchCaseActivitiesResponse, error)
	// GetCaseAttachmentContent returns the raw binary content and its Content-Type
	// for the attachment identified by attachmentID.
	// A NotFoundError is returned if absent.
	GetCaseAttachmentContent(ctx context.Context, attachmentID string) (content []byte, contentType string, err error)
	// DeleteCaseAttachment removes the attachment identified by req.AttachmentID from the case.
	// A NotFoundError is returned if the attachment does not exist.
	DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error)
	// AddCaseTag attaches a free-text label to the case identified by caseID.
	// A ValidationError is returned for invalid input (e.g. malformed UUID, empty label).
	AddCaseTag(ctx context.Context, caseID, label string) (domain.Tag, error)
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
	GetCaseFeedback(ctx context.Context, id string) (domain.CaseFeedback, error)
	// SubmitCaseFeedback records feedback for the case identified by id. A ValidationError
	// is returned for invalid input. Supported by the ServiceNow data source only.
	SubmitCaseFeedback(ctx context.Context, id string, req domain.SubmitCaseFeedbackRequest) (domain.SubmitCaseFeedbackResponse, error)
	// GetAttachmentByID returns the metadata and base64-encoded content of the attachment
	// identified by id. A NotFoundError is returned if it does not exist. For the CSM-native
	// (Postgres) data source, Content is always "" and StorageKey is populated instead: that
	// data source holds no bytes, only a reference into external (SFTPGo) storage.
	GetAttachmentByID(ctx context.Context, id string) (domain.AttachmentDetails, error)
	// UpdateAttachment updates the name and/or description of the attachment identified by
	// req.ID. A ValidationError is returned for invalid input. For ServiceNow, referenceType
	// must be "case" or "deployment" ("case" requires name and forbids description;
	// "deployment" requires at least one of name or description). The CSM-native (Postgres)
	// data source only models case attachments, so it accepts referenceType "case" only, with
	// the same name-required/description-forbidden rule.
	UpdateAttachment(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error)
}

// CaseGithubIssueService defines the operation for filing a GitHub issue from a case.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type CaseGithubIssueService interface {
	// CreateCaseGithubIssue files a new GitHub issue on the internal repo mapped to the
	// case's product, and appends a work note on the case with the resulting issue URL.
	// A ValidationError is returned for invalid input; a NotFoundError if no case matches.
	CreateCaseGithubIssue(ctx context.Context, req domain.CreateCaseGithubIssueRequest) (domain.CreateCaseGithubIssueResponse, error)
}

// CatalogService defines the operations available on service catalogs.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type CatalogService interface {
	// SearchCatalogs returns catalogs available for the given deployed product.
	// DeployedProductID is required. A ValidationError is returned for missing input.
	SearchCatalogs(ctx context.Context, req domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error)
	// GetCatalogItemVariables returns the variables (form fields) for a specific catalog item.
	// A NotFoundError is returned if the catalog or item does not exist.
	GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) (domain.GetCatalogItemVariablesResponse, error)
}

// CallRequestService defines the operations available on the call_requests entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
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

// ChangeRequestService defines the operations available on the change_requests entity.
type ChangeRequestService interface {
	// CreateChangeRequest creates a new change request in ServiceNow. Subject is required.
	// Supported by the ServiceNow data source only.
	CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error)

	// SearchChangeRequests returns a paginated list of change requests filtered by optional
	// project IDs, state keys, impact keys, date ranges, and search query.
	SearchChangeRequests(ctx context.Context, req domain.SearchChangeRequestsRequest) (domain.SearchChangeRequestsResponse, error)

	// GroupChangeRequestsBy returns server-side aggregated counts of change requests
	// per value of req.GroupBy, capped to the top req.MaxGroups buckets with the
	// remainder folded into GroupByResponse.OthersCount. A ValidationError is
	// returned for invalid input.
	GroupChangeRequestsBy(ctx context.Context, req domain.GroupChangeRequestsByRequest) (domain.GroupByResponse, error)

	// GetChangeRequest returns the full detail of a single change request by its UUID.
	GetChangeRequest(ctx context.Context, id string) (domain.ChangeRequest, error)

	// PatchChangeRequest updates mutable fields on a change request identified by UUID.
	PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error)

	// GetChangeRequestApprovals returns the approval stages and per-approver status
	// for a single change request identified by UUID.
	GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error)

	// DecideChangeRequestApproval submits the caller's decision ("approved" or
	// "rejected") on their own pending approval for a change request identified
	// by UUID. Supported by the ServiceNow data source only.
	DecideChangeRequestApproval(ctx context.Context, id, decision string) (domain.ChangeRequestApprovalDecisionResponse, error)
}

// TimeCardService defines the operations available on the time-cards entity.
type TimeCardService interface {
	// SearchTimeCards returns a paginated list of time cards filtered by optional
	// project IDs, case, user, approver, date range, and states.
	SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchTimeCardsResponse, error)
	// CreateTimeCard logs a new time card against a case in the submitted state.
	CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error)
	// UpdateTimeCard edits an editable (submitted) time card, or transitions its
	// state (approve/reject) when req.State is set. SN enforces authorization.
	UpdateTimeCard(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error)
	// SearchCaseTimeCards returns a paginated list of time cards grouped and rolled up by
	// case, using the same filters as SearchTimeCards. Supported by the ServiceNow data
	// source only.
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

// GroupService defines the operations available on the groups entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type GroupService interface {
	// SearchGroups returns a paginated list of groups filtered by optional search query.
	SearchGroups(ctx context.Context, req domain.SearchGroupsRequest) (domain.SearchGroupsResponse, error)
}

// ServiceOfferingService defines the operations available on the service offerings entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ServiceOfferingService interface {
	// SearchServiceOfferings returns a paginated list of service offerings filtered by
	// optional service IDs.
	SearchServiceOfferings(ctx context.Context, req domain.SearchServiceOfferingsRequest) (domain.SearchServiceOfferingsResponse, error)
}

// ITServiceService defines the operations available on the CMDB IT services entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ITServiceService interface {
	// SearchITServices returns a paginated list of CMDB services from ServiceNow.
	SearchITServices(ctx context.Context, req domain.SearchITServicesRequest) (domain.SearchITServicesResponse, error)
}

// CommentService defines generic comment search operations across all reference types
// (case, conversation, change_request, etc.).
// All methods require the ServiceNow data source; there is no Postgres fallback.
type CommentService interface {
	// SearchComments returns a paginated list of comments for the given reference entity.
	SearchComments(ctx context.Context, req domain.SearchCommentsRequest) (domain.SearchCommentsResponse, error)
	// CreateComment creates a new comment on the given reference entity.
	CreateComment(ctx context.Context, req domain.CreateCommentRequest) (domain.CreateCommentResponse, error)
}

// TaskSlaService defines the operations available on the task-slas entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
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
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ProductVulnerabilityService interface {
	// SearchProductVulnerabilities returns a paginated list of vulnerabilities filtered by
	// optional priority, product name, product version, and search query.
	// A ValidationError is returned for invalid input.
	SearchProductVulnerabilities(ctx context.Context, req domain.SearchProductVulnerabilitiesRequest) (domain.SearchProductVulnerabilitiesResponse, error)

	// GetProductVulnerability returns the detail of a single vulnerability by its UUID.
	// A NotFoundError is returned if the vulnerability does not exist.
	GetProductVulnerability(ctx context.Context, id string) (domain.ProductVulnerabilityView, error)

	// GetVulnerabilityMeta returns the valid severity choices for product vulnerabilities.
	// Supported by the ServiceNow data source only.
	GetVulnerabilityMeta(ctx context.Context) (domain.VulnerabilityMetaResponse, error)
}

// IncidentService defines the operations available on the incidents entity.
type IncidentService interface {
	// SearchIncidents returns a paginated list of incidents filtered by optional search query,
	// priority keys, and parent IDs. A ValidationError is returned for invalid input.
	SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error)

	// GroupIncidentsBy returns server-side aggregated counts of incidents per
	// value of req.GroupBy, capped to the top req.MaxGroups buckets with the
	// remainder folded into GroupByResponse.OthersCount. A ValidationError is
	// returned for invalid input.
	GroupIncidentsBy(ctx context.Context, req domain.GroupIncidentsByRequest) (domain.GroupByResponse, error)

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
}

// ProblemService defines the operations available on the problems entity.
type ProblemService interface {
	// SearchProblems returns a paginated list of problems filtered by optional search query.
	// A ValidationError is returned for invalid input.
	SearchProblems(ctx context.Context, req domain.SearchProblemsRequest) (domain.SearchProblemsResponse, error)

	// GroupProblemsBy returns server-side aggregated counts of problems per
	// value of req.GroupBy, capped to the top req.MaxGroups buckets with the
	// remainder folded into GroupByResponse.OthersCount. A ValidationError is
	// returned for invalid input.
	GroupProblemsBy(ctx context.Context, req domain.GroupProblemsByRequest) (domain.GroupByResponse, error)

	// GetProblem returns the full detail of a single problem by its UUID.
	// A NotFoundError is returned if the problem does not exist.
	GetProblem(ctx context.Context, id string) (domain.ProblemDetail, error)

	// CreateProblem creates a new problem. Subject is required; OriginCaseID is optional.
	// Supported by the ServiceNow data source only.
	CreateProblem(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error)
}

// IncidentTaskService defines the operations available on the incident_task entity.
// Search and get only -- there is no create/update path.
type IncidentTaskService interface {
	// SearchIncidentTasks returns a paginated list of incident tasks filtered by
	// optional search query and field filters. A ValidationError is returned for
	// invalid input.
	SearchIncidentTasks(ctx context.Context, req domain.SearchIncidentTasksRequest) (domain.SearchIncidentTasksResponse, error)

	// GroupIncidentTasksBy returns server-side aggregated counts of incident
	// tasks per value of req.GroupBy, capped to the top req.MaxGroups buckets
	// with the remainder folded into GroupByResponse.OthersCount. A
	// ValidationError is returned for invalid input.
	GroupIncidentTasksBy(ctx context.Context, req domain.GroupIncidentTasksByRequest) (domain.GroupByResponse, error)

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
// isn't scoped to any single project or case.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type GlobalService interface {
	// GetSystemMetadata returns system-wide reference data (time zones, project types,
	// and feedback emoji choices) used across the frontend.
	GetSystemMetadata(ctx context.Context) (domain.SystemMetadataResponse, error)
	// GlobalSearch searches projects and/or cases matching req's filters. Every field of
	// req is optional; an empty request searches both tables with default pagination.
	GlobalSearch(ctx context.Context, req domain.GlobalSearchRequest) (domain.GlobalSearchResponse, error)
}

// EscalationService defines the operations available on the escalations entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
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
