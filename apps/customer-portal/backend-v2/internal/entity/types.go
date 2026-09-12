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

package entity

import (
	"encoding/json"
	"time"
)

// These types mirror entity-service's wire format 1:1 (see
// cs-tools/entity-service/internal/domain/entity.go) so json.Unmarshal can
// decode its responses directly. They are internal to this package — the
// dto package maps them into the portal's own response contracts before
// anything reaches the frontend.

// Pagination controls which page of results is requested/returned.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// --- users/me ---

// GetUserMeResponse is entity-service's response for GET /users/me.
type GetUserMeResponse struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	FirstName *string  `json:"firstName,omitempty"`
	LastName  string   `json:"lastName"`
	TimeZone  *string  `json:"timeZone,omitempty"`
	Roles     []string `json:"roles"`
}

// PatchUserMeRequest is the request body for PATCH /users/me.
type PatchUserMeRequest struct {
	TimeZone string `json:"timeZone"`
}

// PatchUserMeUpdated contains the key fields returned after a successful user update.
type PatchUserMeUpdated struct {
	ID        string `json:"id"`
	UpdatedBy string `json:"updatedBy"`
	UpdatedOn string `json:"updatedOn"`
}

// PatchUserMeResponse is entity-service's response for PATCH /users/me.
type PatchUserMeResponse struct {
	Message string             `json:"message"`
	User    PatchUserMeUpdated `json:"user"`
}

// --- projects ---

// SearchProjectsRequest is the input for POST /projects/search.
type SearchProjectsRequest struct {
	Pagination    Pagination `json:"pagination"`
	SearchQuery   string     `json:"searchQuery,omitempty"`
	ClosureStatus string     `json:"closureStatus,omitempty"`
	EndDateFrom   string     `json:"endDateFrom,omitempty"`
	EndDateTo     string     `json:"endDateTo,omitempty"`
	SortBy        string     `json:"sortBy,omitempty"`
	SortOrder     string     `json:"sortOrder,omitempty"`
}

// ProjectClosureFields groups the ServiceNow-only closure-tracking fields
// shared by ProjectDetailsView and ProjectView.
type ProjectClosureFields struct {
	ClosureState                    *string         `json:"closureState"`
	EndDateClosureState             *string         `json:"endDateClosureState"`
	InvoiceDueDateClosureState      *string         `json:"invoiceDueDateClosureState"`
	ComplianceViolationClosureState *string         `json:"complianceViolationClosureState"`
	ComplianceViolationDate         *string         `json:"complianceViolationDate"`
	SuspensionProcessState          json.RawMessage `json:"suspensionProcessState"`
}

// ProjectView is a single search result item from POST /projects/search.
type ProjectView struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Key              string     `json:"key"`
	SubscriptionType string     `json:"subscriptionType"`
	StartDate        *time.Time `json:"startDate"`
	EndDate          *time.Time `json:"endDate"`
	CreatedOn        time.Time  `json:"createdOn"`
	ActiveCasesCount int        `json:"activeCasesCount"`
	ProjectClosureFields
}

// SearchProjectsResponse is entity-service's response for POST /projects/search.
type SearchProjectsResponse struct {
	Projects []ProjectView `json:"projects"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	HasMore  bool          `json:"hasMore"`
}

// ProjectAccountRef is the account summary embedded in ProjectDetailsView.
type ProjectAccountRef struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	ActivationDate      *time.Time `json:"activationDate"`
	Tier                string     `json:"tier"`
	Region              *string    `json:"region"`
	AgentEnabled        bool       `json:"agentEnabled"`
	KbReferencesEnabled bool       `json:"kbReferencesEnabled"`
	DeactivationDate    *time.Time `json:"deactivationDate"`
	OwnerEmail          *string    `json:"ownerEmail"`
	TechnicalOwnerEmail *string    `json:"technicalOwnerEmail"`
}

// ProjectDetailsView is entity-service's response for GET /projects/{id}.
type ProjectDetailsView struct {
	ID               string            `json:"id"`
	Account          ProjectAccountRef `json:"account"`
	SfID             string            `json:"sfId"`
	Name             string            `json:"name"`
	Key              string            `json:"key"`
	SubscriptionType string            `json:"subscriptionType"`
	StartDate        time.Time         `json:"startDate"`
	EndDate          time.Time         `json:"endDate"`
	CreatedOn        time.Time         `json:"createdOn"`
	UpdatedOn        time.Time         `json:"updatedOn"`

	// Query/onboarding entitlement balances and onboarding milestones.
	// entity-service groups these in an embedded ProjectEngagementFields, which
	// flattens in JSON, so they are declared flat here. Pointers throughout: a
	// nil balance means "not tracked for this project", which is a different
	// fact from a zero one, "tracked, none remaining".
	TotalQueryHours          *float64   `json:"totalQueryHours"`
	ConsumedQueryHours       *float64   `json:"consumedQueryHours"`
	RemainingQueryHours      *float64   `json:"remainingQueryHours"`
	TotalOnboardingHours     *float64   `json:"totalOnboardingHours"`
	ConsumedOnboardingHours  *float64   `json:"consumedOnboardingHours"`
	RemainingOnboardingHours *float64   `json:"remainingOnboardingHours"`
	GoLiveDate               *time.Time `json:"goLiveDate"`
	GoLivePlanDate           *time.Time `json:"goLivePlanDate"`
	OnboardingExpiryDate     *time.Time `json:"onboardingExpiryDate"`
	OnboardingStatus         *string    `json:"onboardingStatus"`
	ProjectClosureFields
}

// --- project metadata/stats ---
//
// entity-service only supports these routes on its ServiceNow data source —
// see cs-tools/entity-service/internal/server/routes.go's projectStatsHandler
// gating.

// ChoiceListItem is one available option for a ServiceNow choice-list field —
// used in metadata responses that enumerate the valid values for a field
// (e.g. case states, severities) rather than classify a single record, and in
// stats responses that report a count per option.
type ChoiceListItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count *int   `json:"count,omitempty"`
}

// ReferenceTableItem is a reference to a row in another ServiceNow table,
// used in metadata/stats responses (e.g. case types) — richer than EntityRef:
// adds an optional record number, WSO2-internal ID, and per-item count.
type ReferenceTableItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Number     *string `json:"number,omitempty"`
	InternalID *string `json:"internalId,omitempty"`
	Count      *int    `json:"count,omitempty"`
	// Abbreviation is the short reference name; entity-service decodes it from
	// ServiceNow's ReferenceTableItem.
	Abbreviation *string `json:"abbreviation,omitempty"`
}

// ProjectFeatures is the feature-access configuration for a project.
type ProjectFeatures struct {
	ProjectType                    ReferenceTableItem `json:"projectType"`
	AcceptedSeverityValues         []ChoiceListItem   `json:"acceptedSeverityValues"`
	HasServiceRequestWriteAccess   bool               `json:"hasServiceRequestWriteAccess"`
	HasServiceRequestReadAccess    bool               `json:"hasServiceRequestReadAccess"`
	HasSraWriteAccess              bool               `json:"hasSraWriteAccess"`
	HasSraReadAccess               bool               `json:"hasSraReadAccess"`
	HasChangeRequestReadAccess     bool               `json:"hasChangeRequestReadAccess"`
	HasEngagementsReadAccess       bool               `json:"hasEngagementsReadAccess"`
	HasUpdatesReadAccess           bool               `json:"hasUpdatesReadAccess"`
	HasTimeLogsReadAccess          bool               `json:"hasTimeLogsReadAccess"`
	HasDeploymentWriteAccess       bool               `json:"hasDeploymentWriteAccess"`
	HasDeploymentReadAccess        bool               `json:"hasDeploymentReadAccess"`
	HasComponentAnalysisReadAccess bool               `json:"hasComponentAnalysisReadAccess"`
	HasUsageMetricsReadAccess      bool               `json:"hasUsageMetricsReadAccess"`
	DefaultCaseProductCategories   []string           `json:"defaultCaseProductCategories,omitempty"`
	SrProductCategories            []string           `json:"srProductCategories,omitempty"`
}

// ProjectMetadataResponse is entity-service's response for GET /projects/{id}/metadata.
type ProjectMetadataResponse struct {
	CaseStates                  []ChoiceListItem     `json:"caseStates"`
	CallRequestStates           []ChoiceListItem     `json:"callRequestStates"`
	ChangeRequestStates         []ChoiceListItem     `json:"changeRequestStates"`
	ConversationStates          []ChoiceListItem     `json:"conversationStates"`
	TimeCardStates              []ChoiceListItem     `json:"timeCardStates"`
	ChangeRequestImpacts        []ChoiceListItem     `json:"changeRequestImpacts"`
	Severities                  []ChoiceListItem     `json:"severities"`
	SeverityBasedAllocationTime map[string]int       `json:"severityBasedAllocationTime"`
	IssueTypes                  []ChoiceListItem     `json:"issueTypes"`
	DeploymentTypes             []ChoiceListItem     `json:"deploymentTypes"`
	CaseTypes                   []ReferenceTableItem `json:"caseTypes"`
	EngagementTypes             []ChoiceListItem     `json:"engagementTypes"`
	EngagementPaymentTypes      []ChoiceListItem     `json:"engagementPaymentTypes"`
	Features                    ProjectFeatures      `json:"features"`
}

// ProjectStatsOutstandingCount groups the outstanding-work-item counts
// embedded in ProjectStatsResponse.
type ProjectStatsOutstandingCount struct {
	CaseCount           int `json:"caseCount"`
	ServiceRequestCount int `json:"serviceRequestCount"`
	EngagementCount     int `json:"engagementCount"`
	SraCount            int `json:"sraCount"`
	ChangeRequestCount  int `json:"changeRequestCount"`
	AnnouncementCount   int `json:"announcementCount"`
}

// ProjectStatsResponse is entity-service's response for GET /projects/{id}/stats.
type ProjectStatsResponse struct {
	TotalHours           float64                      `json:"totalHours"`
	BillableHours        float64                      `json:"billableHours"`
	SLAStatus            string                       `json:"slaStatus"`
	DeploymentCount      int                          `json:"deploymentCount"`
	DeployedProductCount int                          `json:"deployedProductCount"`
	InstanceCount        int                          `json:"instanceCount"`
	OutstandingCount     ProjectStatsOutstandingCount `json:"outstandingCount"`
}

// ResolvedCountBreakdown groups the resolved-count breakdown shared by case
// and change-request stats responses.
type ResolvedCountBreakdown struct {
	Total          int `json:"total"`
	CurrentMonth   int `json:"currentMonth"`
	PastThirtyDays int `json:"pastThirtyDays"`
}

// ProjectCaseStatsChangeRate groups the period-over-period change-rate
// figures embedded in ProjectCaseStatsResponse.
type ProjectCaseStatsChangeRate struct {
	ResolvedEngagements float64 `json:"resolvedEngagements"`
	AverageResponseTime float64 `json:"averageResponseTime"`
}

// CasesTrend is the severity breakdown for a single time-unit bucket in a
// case-count trend series.
type CasesTrend struct {
	Period     string           `json:"period"`
	Severities []ChoiceListItem `json:"severities"`
}

// ProjectCaseStatsResponse is entity-service's response for GET /projects/{id}/cases/stats.
type ProjectCaseStatsResponse struct {
	TotalCount                     int                        `json:"totalCount"`
	ActiveCount                    int                        `json:"activeCount"`
	OutstandingCount               int                        `json:"outstandingCount"`
	ActionRequiredCount            int                        `json:"actionRequiredCount"`
	AverageResponseTime            float64                    `json:"averageResponseTime"`
	ResolvedCount                  ResolvedCountBreakdown     `json:"resolvedCount"`
	ChangeRate                     ProjectCaseStatsChangeRate `json:"changeRate"`
	StateCount                     []ChoiceListItem           `json:"stateCount"`
	SeverityCount                  []ChoiceListItem           `json:"severityCount"`
	OutstandingSeverityCount       []ChoiceListItem           `json:"outstandingSeverityCount"`
	EngagementTypeCount            []ChoiceListItem           `json:"engagementTypeCount"`
	OutstandingEngagementTypeCount []ChoiceListItem           `json:"outstandingEngagementTypeCount"`
	CaseTypeCount                  []ReferenceTableItem       `json:"caseTypeCount"`
	CasesTrend                     []CasesTrend               `json:"casesTrend"`
}

// ProjectConversationStatsResponse is entity-service's response for
// GET /projects/{id}/conversations/stats.
type ProjectConversationStatsResponse struct {
	TotalCount  int              `json:"totalCount"`
	ActiveCount int              `json:"activeCount"`
	StateCount  []ChoiceListItem `json:"stateCount"`
}

// ProjectDeploymentStatsResponse is entity-service's response for
// GET /projects/{id}/deployments/stats.
type ProjectDeploymentStatsResponse struct {
	TotalCount       int     `json:"totalCount"`
	LastDeploymentOn *string `json:"lastDeploymentOn"`
}

// ProjectTimeCardStatsResponse is entity-service's response for
// GET /projects/{id}/time-cards/stats.
type ProjectTimeCardStatsResponse struct {
	TotalHours       float64 `json:"totalHours"`
	BillableHours    float64 `json:"billableHours"`
	NonBillableHours float64 `json:"nonBillableHours"`
}

// ProjectChangeRequestStatsResponse is entity-service's response for
// GET /projects/{id}/change-requests/stats.
type ProjectChangeRequestStatsResponse struct {
	TotalCount          int                    `json:"totalCount"`
	ActiveCount         int                    `json:"activeCount"`
	OutstandingCount    int                    `json:"outstandingCount"`
	ActionRequiredCount int                    `json:"actionRequiredCount"`
	StateCount          []ChoiceListItem       `json:"stateCount"`
	ResolvedCount       ResolvedCountBreakdown `json:"resolvedCount"`
}

// --- accounts ---
//
// entity-service returns a different wire shape for these two endpoints
// depending on its DATA_SOURCE (postgres vs servicenow) — unlike projects and
// cases, account responses have not been unified upstream. The structs below
// are a superset of both shapes: JSON key names never collide between the two
// data sources (they use different field names for the same concept, e.g.
// "tier" vs "classification", "agentEnabled" vs "hasAgent"), and every date/
// time field is typed as *string here since a Go string field decodes a JSON
// string value regardless of whether the source type was time.Time or a
// plain string — so only the fields the active data source actually
// populates come out non-nil.

// SupportTierRef is a compact reference to a support tier carrying its label
// (ServiceNow data source, GET /accounts/{id} only).
type SupportTierRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// SearchAccountsFilters holds the optional filter criteria for an account search.
type SearchAccountsFilters struct {
	SearchQuery    string `json:"searchQuery,omitempty"`
	Active         *bool  `json:"active,omitempty"`
	Pod            string `json:"pod,omitempty"`
	Classification string `json:"classification,omitempty"`
}

// SearchAccountsRequest is the input for POST /accounts/search.
type SearchAccountsRequest struct {
	Pagination Pagination            `json:"pagination"`
	Filters    SearchAccountsFilters `json:"filters,omitempty"`
}

// AccountSummary is a single search result item from POST /accounts/search —
// a superset of entity-service's Postgres Account and ServiceNow SNAccountView.
type AccountSummary struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Region *string `json:"region,omitempty"`
	// Postgres-only.
	SfID                *string `json:"sfId,omitempty"`
	Tier                *string `json:"tier,omitempty"`
	OwnerID             *string `json:"ownerId,omitempty"`
	TechnicalOwnerID    *string `json:"technicalOwnerId,omitempty"`
	AgentEnabled        *bool   `json:"agentEnabled,omitempty"`
	KbReferencesEnabled *bool   `json:"kbReferencesEnabled,omitempty"`
	// ServiceNow-only.
	Classification *string    `json:"classification,omitempty"`
	Pod            *string    `json:"pod,omitempty"`
	SupportTier    *string    `json:"supportTier,omitempty"`
	ArrToday       *string    `json:"arrToday,omitempty"`
	Owner          *EntityRef `json:"owner,omitempty"`
	// AccountManager is entity-service's current name for what this backend
	// exposes as `owner`. The unified account view renamed `owner` ->
	// `accountManager`; both are decoded so the mapping survives either
	// service deploying first. PersonRef carries an extra `email` field that
	// EntityRef simply ignores.
	AccountManager  *EntityRef `json:"accountManager,omitempty"`
	TechnicalOwner  *EntityRef `json:"technicalOwner,omitempty"`
	HasAgent        *bool      `json:"hasAgent,omitempty"`
	HasKbReferences *bool      `json:"hasKbReferences,omitempty"`
	CreatedBy       *string    `json:"createdBy,omitempty"`
	// Shared (identical key/type on both data sources).
	ActivationDate   *string `json:"activationDate,omitempty"`
	DeactivationDate *string `json:"deactivationDate,omitempty"`
	CreatedOn        *string `json:"createdOn,omitempty"`
	UpdatedOn        *string `json:"updatedOn,omitempty"`
}

// SearchAccountsResponse is entity-service's response for POST /accounts/search.
type SearchAccountsResponse struct {
	Accounts []AccountSummary `json:"accounts"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	HasMore  bool             `json:"hasMore"`
}

// AccountDetail is entity-service's response for GET /accounts/{id} — a
// superset of entity-service's Postgres Account and ServiceNow SNAccountDetail.
type AccountDetail struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Region *string `json:"region,omitempty"`
	// Postgres-only.
	SfID                *string `json:"sfId,omitempty"`
	Tier                *string `json:"tier,omitempty"`
	OwnerID             *string `json:"ownerId,omitempty"`
	TechnicalOwnerID    *string `json:"technicalOwnerId,omitempty"`
	AgentEnabled        *bool   `json:"agentEnabled,omitempty"`
	KbReferencesEnabled *bool   `json:"kbReferencesEnabled,omitempty"`
	// ServiceNow-only.
	Classification *string         `json:"classification,omitempty"`
	Pod            *string         `json:"pod,omitempty"`
	SupportTier    *SupportTierRef `json:"supportTier,omitempty"`
	ArrToday       *string         `json:"arrToday,omitempty"`
	Owner          *EntityRef      `json:"owner,omitempty"`
	// See AccountDetail.AccountManager -- same rename, same reason.
	AccountManager  *EntityRef `json:"accountManager,omitempty"`
	TechnicalOwner  *EntityRef `json:"technicalOwner,omitempty"`
	HasAgent        *bool      `json:"hasAgent,omitempty"`
	HasKbReferences *bool      `json:"hasKbReferences,omitempty"`
	CreatedBy       *string    `json:"createdBy,omitempty"`
	// Shared (identical key/type on both data sources).
	ActivationDate   *string `json:"activationDate,omitempty"`
	DeactivationDate *string `json:"deactivationDate,omitempty"`
	CreatedOn        *string `json:"createdOn,omitempty"`
	UpdatedOn        *string `json:"updatedOn,omitempty"`
}

// --- cases ---

// CaseSort specifies the sort field and direction for case search results.
type CaseSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// CaseFieldFilter is one predicate in a case search's generic filter
// expression array: "field op values". entity-service redesigned its case
// search contract to this shape (from the old one-named-field-per-filter
// style dto.CaseSearchRequest still exposes to the portal frontend) — see
// dto.BuildEntitySearchCasesRequest, which is the only place that builds one
// of these, and entity-service's own openapi.yaml for the exact supported
// field/op combinations.
type CaseFieldFilter struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values,omitempty"`
}

// SearchCasesFilters is entity-service's current case-search filter
// contract. SearchQuery stays special-cased (a free-text match, not a field
// predicate); every other criterion is an entry in Filters. OrGroups is not
// populated by this backend — the portal doesn't currently expose
// cross-field OR filtering.
type SearchCasesFilters struct {
	SearchQuery string              `json:"searchQuery,omitempty"`
	Filters     []CaseFieldFilter   `json:"filters,omitempty"`
	OrGroups    [][]CaseFieldFilter `json:"orGroups,omitempty"`
}

// SearchCasesRequest is entity-service's current wire format for
// POST /cases/search. Never decode a portal request directly into this —
// see dto.CaseSearchRequest (the portal's own stable, unchanged contract)
// and dto.BuildEntitySearchCasesRequest.
type SearchCasesRequest struct {
	Filters    SearchCasesFilters `json:"filters"`
	SortBy     CaseSort           `json:"sortBy"`
	Pagination Pagination         `json:"pagination"`
}

// EntityRef is a compact reference to a named entity (project, deployment, product, etc.).
type EntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AssignedEngineerRef is a compact reference to an assigned support engineer.
type AssignedEngineerRef struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

// CaseNumberRef is a compact reference to a case carrying its human-readable number.
type CaseNumberRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// LinkedServiceRequestRef is a compact reference to a service-request case
// linked to another case as its parent.
type LinkedServiceRequestRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Name   string `json:"name"`
}

// LinkedChangeRequestRef is a compact reference to a change request raised
// from a service-request case.
type LinkedChangeRequestRef struct {
	ID     string  `json:"id"`
	Number string  `json:"number"`
	Name   *string `json:"name"`
}

// AccountRef is a compact reference to an account.
type AccountRef struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Type    string     `json:"type"`
	CreTeam *EntityRef `json:"creTeam,omitempty"`
	SreTeam *EntityRef `json:"sreTeam,omitempty"`
}

// DeployedProductRef is a compact reference to a deployed product.
type DeployedProductRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// UserRef is a reference to a user with key display fields.
// UserReference is entity-service's canonical person reference. It replaced the
// former string createdBy plus sibling createdByUser/assignedEngineerUser fields
// (see its "one canonical person reference per response field" change), so every
// mirrored response field named createdBy is now an object. Declaring it as a
// string made json.Unmarshal fail with "cannot unmarshal object into Go value of
// type string", aborting the whole decode.
type UserReference struct {
	ID    *string `json:"id"`
	Email string  `json:"email"`
	Name  string  `json:"name"`
}

type UserRef struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	UserID string `json:"userId,omitempty"`
	Email  string `json:"email"`
}

// Tag is a free-text label attached to a case.
type Tag struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Color *string `json:"color"`
}

// SearchCaseView is a single search result item from POST /cases/search.
type SearchCaseView struct {
	ID               string               `json:"id"`
	InternalID       string               `json:"internalId"`
	Number           string               `json:"number"`
	CreatedOn        string               `json:"createdOn"`
	UpdatedOn        string               `json:"updatedOn"`
	CreatedBy        *UserReference       `json:"createdBy"`
	Subject          *string              `json:"subject"`
	Description      *string              `json:"description"`
	IssueType        *string              `json:"issueType"`
	State            string               `json:"state"`
	Severity         *string              `json:"severity"`
	Catalog          *EntityRef           `json:"catalog"`
	CatalogItem      *EntityRef           `json:"catalogItem"`
	AssignedTeam     *EntityRef           `json:"assignedTeam"`
	Product          *EntityRef           `json:"product"`
	EngagementType   *string              `json:"engagementType"`
	WorkState        *string              `json:"workState"`
	Type             string               `json:"type"`
	Project          EntityRef            `json:"project"`
	Deployment       *EntityRef           `json:"deployment"`
	DeployedProduct  *EntityRef           `json:"deployedProduct"`
	AssignedEngineer *AssignedEngineerRef `json:"assignedEngineer"`
	ParentCase       *EntityRef           `json:"parentCase"`
	RelatedCase      *EntityRef           `json:"relatedCase"`
	Conversation     *EntityRef           `json:"conversation"`
}

// SearchCasesResponse is entity-service's response for POST /cases/search.
type SearchCasesResponse struct {
	Cases  []SearchCaseView `json:"cases"`
	Total  int              `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
}

// Variable is a key-value pair used in service-request case creation.
type Variable struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// CaseAttachment is a file attachment for security-report-analysis case
// creation. File must be a base64 data URI (e.g. "data:application/pdf;base64,...").
type CaseAttachment struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// CreateCaseRequest is the input for POST /cases. CreatedBy is never
// serialized (json:"-") — entity-service derives the creator from its own
// auth context, not the request body.
type CreateCaseRequest struct {
	CreatedBy         string           `json:"-"`
	Type              string           `json:"type"`
	ProjectID         string           `json:"projectId"`
	DeploymentID      string           `json:"deploymentId"`
	DeployedProductID string           `json:"deployedProductId,omitempty"`
	Subject           string           `json:"subject"`
	Description       string           `json:"description"`
	Severity          string           `json:"severity"`
	IssueType         string           `json:"issueType"`
	CatalogID         string           `json:"catalogId,omitempty"`
	CatalogItemID     string           `json:"catalogItemId,omitempty"`
	Variables         []Variable       `json:"variables,omitempty"`
	RelatedCaseID     string           `json:"relatedCaseId,omitempty"`
	ConversationID    string           `json:"conversationId,omitempty"`
	WatchList         []string         `json:"watchList,omitempty"`
	Attachments       []CaseAttachment `json:"attachments,omitempty"`
}

// CreateCaseDetails carries the key fields of a newly created case.
type CreateCaseDetails struct {
	ID         string    `json:"id"`
	InternalID string    `json:"internalId"`
	Number     string    `json:"number"`
	CreatedBy  string    `json:"createdBy"`
	CreatedOn  time.Time `json:"createdOn"`
	State      string    `json:"state"`
}

// CreateCaseResponse is entity-service's response for POST /cases.
type CreateCaseResponse struct {
	Message string            `json:"message"`
	Case    CreateCaseDetails `json:"case"`
}

// UpdateCaseRequest is the full field set entity-service accepts for
// PATCH /cases/{id}. This is entity-service's raw contract — the portal only
// exposes a customer-safe subset of these fields; see dto.UpdateCaseRequest
// and dto.BuildEntityUpdateCaseRequest for which ones and why.
type UpdateCaseRequest struct {
	ID                 string     `json:"-"`
	State              *string    `json:"state,omitempty"`
	Severity           *string    `json:"severity,omitempty"`
	WorkState          *string    `json:"workState,omitempty"`
	WatchList          []string   `json:"watchList,omitempty"`
	AssigneeEmail      *string    `json:"assigneeEmail,omitempty"`
	ResolutionCode     *string    `json:"resolutionCode,omitempty"`
	Cause              *string    `json:"cause,omitempty"`
	CloseNotes         *string    `json:"closeNotes,omitempty"`
	ParentID           *string    `json:"parentId,omitempty"`
	RelatedCaseID      *string    `json:"relatedCaseId,omitempty"`
	AutocloseHoldUntil *time.Time `json:"autocloseHoldUntil,omitempty"`
	Subject            *string    `json:"subject,omitempty"`
	Description        *string    `json:"description,omitempty"`
	DeploymentID       *string    `json:"deploymentId,omitempty"`
	DeployedProductID  *string    `json:"deployedProductId,omitempty"`
	FixEta             *time.Time `json:"fixEta,omitempty"`
	BestCaseFixEta     *time.Time `json:"bestCaseFixEta,omitempty"`
	MostLikelyFixEta   *time.Time `json:"mostLikelyFixEta,omitempty"`
	WorstCaseFixEta    *time.Time `json:"worstCaseFixEta,omitempty"`
}

// WatchListUser is a user watching a case (ServiceNow data source only).
type WatchListUser struct {
	ID       string `json:"id"`
	UserName string `json:"userName"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
}

// UpdatedCase carries the case fields entity-service returns after a
// successful PATCH /cases/{id}.
type UpdatedCase struct {
	ID             string               `json:"id"`
	UpdatedOn      time.Time            `json:"updatedOn"`
	UpdatedBy      string               `json:"updatedBy,omitempty"`
	State          string               `json:"state,omitempty"`
	Severity       string               `json:"severity,omitempty"`
	WorkState      *string              `json:"workState"`
	WatchList      []WatchListUser      `json:"watchList,omitempty"`
	AssignedTo     *AssignedEngineerRef `json:"assignedTo,omitempty"`
	ResolutionCode *string              `json:"resolutionCode,omitempty"`
	Cause          *string              `json:"cause,omitempty"`
	CloseNotes     *string              `json:"closeNotes,omitempty"`
	ResolvedOn     *time.Time           `json:"resolvedOn,omitempty"`
	ParentCase     *CaseNumberRef       `json:"parentCase,omitempty"`
	// FixEta is the customer-facing fix-commitment date; the internal-only
	// Best/MostLikely/WorstCaseFixEta fields are intentionally not decoded
	// here — see CaseView's doc comment above for why.
	FixEta *time.Time `json:"fixEta,omitempty"`
}

// UpdateCaseResponse is entity-service's response for PATCH /cases/{id}.
type UpdateCaseResponse struct {
	Message string      `json:"message"`
	Case    UpdatedCase `json:"case"`
}

// CommentType classifies a case comment. entity-service supports
// "work_note" and "activity" too, but the customer portal only ever creates
// (and should only ever create) plain "comment" entries — see
// dto.BuildEntityCreateCaseCommentRequest.
type CommentType string

const (
	CommentTypeWorkNote CommentType = "work_note"
	CommentTypeComment  CommentType = "comment"
	CommentTypeActivity CommentType = "activity"
)

// CreateCaseCommentRequest is the input for POST /cases/{id}/comments.
// CaseID and CreatedBy are never serialized (json:"-") — CaseID comes from
// the URL path, CreatedBy from entity-service's own auth context.
type CreateCaseCommentRequest struct {
	CaseID    string      `json:"-"`
	CreatedBy string      `json:"-"`
	Type      CommentType `json:"type"`
	Content   string      `json:"content"`
}

// CaseCommentDetail carries the key fields of a newly created comment.
type CaseCommentDetail struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// CreateCaseCommentResponse is entity-service's response for POST /cases/{id}/comments.
type CreateCaseCommentResponse struct {
	Message string            `json:"message"`
	Comment CaseCommentDetail `json:"comment"`
}

// CaseView is entity-service's response for GET /cases/{id}.
type CaseView struct {
	ID                     string                    `json:"id"`
	Number                 string                    `json:"number"`
	InternalID             string                    `json:"internalId"`
	Subject                string                    `json:"subject"`
	Description            string                    `json:"description"`
	Severity               string                    `json:"severity"`
	IssueType              string                    `json:"issueType"`
	State                  string                    `json:"state"`
	WorkState              *string                   `json:"workState"`
	Type                   *string                   `json:"type"`
	EngagementType         *string                   `json:"engagementType"`
	CreatedOn              time.Time                 `json:"createdOn"`
	UpdatedOn              time.Time                 `json:"updatedOn"`
	ClosedOn               *time.Time                `json:"closedOn"`
	CreatedByDetails       UserRef                   `json:"createdBy"`
	ProjectDetails         EntityRef                 `json:"project"`
	DeploymentDetails      *EntityRef                `json:"deployment"`
	DeployedProductDetails *DeployedProductRef       `json:"deployedProduct"`
	ProductDetails         *EntityRef                `json:"product"`
	Catalog                *EntityRef                `json:"catalog"`
	CatalogItem            *EntityRef                `json:"catalogItem"`
	AssignedTeam           *EntityRef                `json:"assignedTeam"`
	Conversation           *EntityRef                `json:"conversation"`
	AssignedEngineer       *AssignedEngineerRef      `json:"assignedEngineer"`
	ParentCase             *CaseNumberRef            `json:"parentCase"`
	RelatedCase            *CaseNumberRef            `json:"relatedCase"`
	AccountDetails         *AccountRef               `json:"account"`
	LinkedServiceRequests  []LinkedServiceRequestRef `json:"linkedServiceRequests"`
	// LinkedChangeRequests lists the change requests raised from this case
	// (populated for service-request cases only; empty otherwise).
	LinkedChangeRequests []LinkedChangeRequestRef `json:"linkedChangeRequests"`
	ResolvedOn           *time.Time               `json:"resolvedOn"`
	ResolutionCode       *string                  `json:"resolutionCode"`
	Cause                *string                  `json:"cause"`
	ResolutionNotes      *string                  `json:"resolutionNotes"`
	// WatchList is the set of users watching the case (ServiceNow data
	// source only) — a customer self-service feature (the customer's own
	// PATCH /cases/{id} request can set it), not CSM-engineer-only.
	WatchList []WatchListUser `json:"watchList,omitempty"`
	// AutoclosureStep/AutoclosureStateTime and the Best/MostLikely/
	// WorstCaseFixEta fields are intentionally NOT decoded here — entity-service
	// documents them as CSM-engineer-facing only (see entity.go comments on
	// CaseView), and the customer portal must not surface internal WSO2 support
	// workflow state to end customers.
	FixEta *time.Time `json:"fixEta"`
	// Supplied by entity-service; previously not decoded here at all.
	// Nullable throughout — nil means the upstream gave no value.
	SLAResponseTime       *string    `json:"slaResponseTime"`
	ClosedBy              *EntityRef `json:"closedBy"`
	HasAutoClosed         *bool      `json:"hasAutoClosed"`
	EngagementStartDate   *string    `json:"engagementStartDate"`
	EngagementEndDate     *string    `json:"engagementEndDate"`
	AcknowledgedBy        *EntityRef `json:"acknowledgedBy"`
	EngagementPaymentType *string    `json:"engagementPaymentType"`
	Tags                  []Tag      `json:"tags"`
	// Duration is upstream display text; EscalationLevel is the level id
	// ("0".."5") that entity-service exposes rather than the "EL0" label.
	Duration        *string `json:"duration"`
	EscalationLevel *string `json:"escalationLevel"`
	IsEscalated     *bool   `json:"isEscalated"`
}

// --- deployments ---

// SearchDeploymentsRequest is the input for POST /deployments/search.
type SearchDeploymentsRequest struct {
	Pagination      Pagination `json:"pagination"`
	SearchQuery     string     `json:"searchQuery,omitempty"`
	ProjectIDs      []string   `json:"projectIds,omitempty"`
	DeploymentTypes []string   `json:"deploymentTypes,omitempty"`
}

// DeploymentView is a single search result item from POST /deployments/search.
type DeploymentView struct {
	ID          string     `json:"id"`
	Number      string     `json:"number"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description *string    `json:"description"`
	URL         *string    `json:"url"`
	CreatedBy   *EntityRef `json:"createdBy"`
	Project     EntityRef  `json:"project"`
	CreatedOn   time.Time  `json:"createdOn"`
	UpdatedOn   time.Time  `json:"updatedOn"`
	// DeployedProductCount comes straight from entity-service, which decodes it
	// from the upstream deployment payload. Exposed to the frontend as
	// productCount — see dto.DeploymentSummary.
	DeployedProductCount int `json:"deployedProductCount"`
}

// SearchDeploymentsResponse is entity-service's response for POST /deployments/search.
type SearchDeploymentsResponse struct {
	Deployments []DeploymentView `json:"deployments"`
	Total       int              `json:"total"`
	Limit       int              `json:"limit"`
	Offset      int              `json:"offset"`
	HasMore     bool             `json:"hasMore"`
}

// CreateDeploymentRequest is the input for POST /deployments.
//
// NOTE: entity-service only supports deployment creation on its ServiceNow
// data source — a Postgres-mode deployment always returns 400 for this
// route (see cs-tools/entity-service/internal/service/deployment_service.go).
type CreateDeploymentRequest struct {
	ProjectID   string  `json:"projectId"`
	Name        string  `json:"name"`
	Type        *string `json:"type,omitempty"`
	Description string  `json:"description,omitempty"`
}

// CreatedDeployment carries the key fields of a newly created deployment.
type CreatedDeployment struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// CreateDeploymentResponse is entity-service's response for POST /deployments.
type CreateDeploymentResponse struct {
	Message    string            `json:"message"`
	Deployment CreatedDeployment `json:"deployment"`
}

// UpdateDeploymentRequest is the input for PATCH /deployments/{id}. Either
// detail fields (Name, Type, Description) or Active (to deactivate) must be
// provided, but not both groups in the same request. Active can only be set
// to false. Description uses json.RawMessage to preserve the three-state
// absent/null/value semantics entity-service expects (see
// UpdateDeployedProductRequest's doc comment for the same convention).
type UpdateDeploymentRequest struct {
	ID          string          `json:"-"`
	Name        *string         `json:"name,omitempty"`
	Type        *string         `json:"type,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Active      *bool           `json:"active,omitempty"`
}

// UpdatedDeployment carries the fields of a deployment that may change after an update.
type UpdatedDeployment struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
}

// UpdateDeploymentResponse is entity-service's response for PATCH /deployments/{id}.
type UpdateDeploymentResponse struct {
	Message    string            `json:"message"`
	Deployment UpdatedDeployment `json:"deployment"`
}

// --- deployed products ---

// CreateDeployedProductRequest is the input for POST /deployed-products.
//
// NOTE: entity-service only supports deployed-product creation on its
// ServiceNow data source — a Postgres-mode deployment always returns 400
// for this route (see cs-tools/entity-service/internal/service/deployed_product_service.go).
type CreateDeployedProductRequest struct {
	ProjectID    string   `json:"projectId"`
	DeploymentID string   `json:"deploymentId"`
	ProductID    string   `json:"productId"`
	VersionID    string   `json:"versionId"`
	Cores        *int     `json:"cores,omitempty"`
	TPS          *float64 `json:"tps,omitempty"`
	Description  *string  `json:"description,omitempty"`
}

// CreatedDeployedProduct carries the key fields of a newly created deployed product.
type CreatedDeployedProduct struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// CreateDeployedProductResponse is entity-service's response for POST /deployed-products.
type CreateDeployedProductResponse struct {
	Message         string                 `json:"message"`
	DeployedProduct CreatedDeployedProduct `json:"deployedProduct"`
}

// SearchDeployedProductsRequest is the input for POST /deployed-products/search.
type SearchDeployedProductsRequest struct {
	Pagination    Pagination `json:"pagination"`
	DeploymentIDs []string   `json:"deploymentIds,omitempty"`
}

// DeployedProductVersionRef is the version sub-object in a DeployedProductView.
type DeployedProductVersionRef struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	ReleasedDate   *time.Time `json:"releasedDate"`
	SupportEoLDate *time.Time `json:"supportEoLDate"`
}

// DeployedProductView is a single search result item from POST /deployed-products/search.
// Cores, TPS, and Category are ServiceNow-only fields, always nil on the
// Postgres data source.
type DeployedProductView struct {
	ID         string                     `json:"id"`
	Deployment EntityRef                  `json:"deployment"`
	Product    EntityRef                  `json:"product"`
	Version    *DeployedProductVersionRef `json:"version"`
	Cores      *string                    `json:"cores"`
	TPS        *string                    `json:"tps"`
	Category   *string                    `json:"category"`
	CreatedOn  time.Time                  `json:"createdOn"`
	UpdatedOn  time.Time                  `json:"updatedOn"`
}

// SearchDeployedProductsResponse is entity-service's response for POST /deployed-products/search.
type SearchDeployedProductsResponse struct {
	DeployedProducts []DeployedProductView `json:"deployedProducts"`
	Total            int                   `json:"total"`
	Limit            int                   `json:"limit"`
	Offset           int                   `json:"offset"`
	HasMore          bool                  `json:"hasMore"`
}

// UpdateDeployedProductRequest is the input for PATCH /deployed-products/{id}.
// Either detail fields (Cores, TPS, Description) or Active=false must be
// provided, but not both. Description uses json.RawMessage to preserve three
// states: absent = omit, "null" = clear, `"value"` = set — decoding a client's
// request body directly into this field naturally preserves that semantic.
//
// NOTE: entity-service only supports this route on its ServiceNow data
// source — see CreateDeployedProductRequest's doc comment.
type UpdateDeployedProductRequest struct {
	ID           string          `json:"-"`
	DeploymentID *string         `json:"deploymentId,omitempty"`
	Cores        *int            `json:"cores,omitempty"`
	TPS          *float64        `json:"tps,omitempty"`
	Description  json.RawMessage `json:"description,omitempty"`
	Active       *bool           `json:"active,omitempty"`
}

// UpdatedDeployedProduct carries the fields that may change after an update.
type UpdatedDeployedProduct struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
}

// UpdateDeployedProductResponse is entity-service's response for PATCH /deployed-products/{id}.
type UpdateDeployedProductResponse struct {
	Message         string                 `json:"message"`
	DeployedProduct UpdatedDeployedProduct `json:"deployedProduct"`
}

// --- attachments ---

// ReferenceType identifies which kind of entity an attachment or comment is
// attached to.
type ReferenceType string

const (
	ReferenceTypeCase          ReferenceType = "case"
	ReferenceTypeConversation  ReferenceType = "conversation"
	ReferenceTypeChangeRequest ReferenceType = "change_request"
	ReferenceTypeDeployment    ReferenceType = "deployment"
	ReferenceTypeIncident      ReferenceType = "incident"
)

// CreateAttachmentRequest is the input for POST /attachments.
type CreateAttachmentRequest struct {
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	File          string        `json:"file"`
	Description   *string       `json:"description,omitempty"`
}

// AttachmentDetail holds the core fields returned after creating an attachment.
type AttachmentDetail struct {
	ID        string    `json:"id"`
	SizeBytes int       `json:"sizeBytes"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
	// DownloadURL is nil for a CSM-native (Postgres) data source attachment:
	// entity-service holds no download location for it, only its storage key.
	// Always non-nil for ServiceNow-sourced attachments.
	DownloadURL *string `json:"downloadUrl"`
}

// CreateAttachmentResponse is entity-service's response for POST /attachments.
type CreateAttachmentResponse struct {
	Message    string           `json:"message"`
	Attachment AttachmentDetail `json:"attachment"`
}

// SearchAttachmentsRequest is the input for POST /attachments/search.
type SearchAttachmentsRequest struct {
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Pagination    Pagination    `json:"pagination"`
}

// Attachment is a single search result item from POST /attachments/search.
type Attachment struct {
	ID            string        `json:"id"`
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	SizeBytes     int           `json:"sizeBytes"`
	Description   *string       `json:"description"`
	// CreatedBy is an object, not a string: entity-service emits
	// domain.UserRef ({id, name, userId, email}) here, unlike
	// AttachmentDetail.CreatedBy which genuinely is a plain string. Declaring
	// it as a string made json.Unmarshal fail with "cannot unmarshal object
	// into Go value of type string", which aborted the whole decode and turned
	// every GET /cases/{id}/attachments into a 500 even though entity-service
	// had returned 200 with a valid list.
	//
	// entity-service also sends a sibling createdByUser (*domain.UserReference);
	// it is deliberately not mirrored here because no portal consumer reads it
	// and responses decode leniently, so the extra field is simply ignored.
	CreatedBy   UserRef   `json:"createdBy"`
	CreatedOn   time.Time `json:"createdOn"`
	DownloadURL *string   `json:"downloadUrl"`
	PreviewURL  *string   `json:"previewUrl"`
}

// SearchAttachmentsResponse is entity-service's response for POST /attachments/search.
type SearchAttachmentsResponse struct {
	Attachments []Attachment `json:"attachments"`
	Total       int          `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
	HasMore     bool         `json:"hasMore"`
}

// DeleteAttachmentResponse is entity-service's response for DELETE /attachments/{id}.
type DeleteAttachmentResponse struct {
	Message string `json:"message"`
}

// --- case activities ---

// ActivityType discriminates the kind of entry in a case's activity feed.
type ActivityType string

const (
	ActivityTypeComment     ActivityType = "comment"
	ActivityTypeAttachment  ActivityType = "attachment"
	ActivityTypeFieldChange ActivityType = "field_change"
)

// FieldChange describes a single field's value change, present only on
// CaseActivity entries with Type == ActivityTypeFieldChange.
type FieldChange struct {
	Field         string `json:"field"`
	FieldLabel    string `json:"fieldLabel"`
	PreviousValue string `json:"previousValue"`
	NewValue      string `json:"newValue"`
}

// CaseActivity is a single entry in a case's activity feed — a discriminated
// union on Type. The shared fields are always present; type-specific fields
// are populated only for the matching Type (CommentType for comments;
// FileName/ContentType/SizeBytes/DownloadURL for attachments; Changes for
// field changes). Field types/omitempty here mirror entity-service's struct
// exactly (see its doc comment) — do not change to pointers.
type CaseActivity struct {
	ID                 string         `json:"id"`
	Type               ActivityType   `json:"type"`
	Content            string         `json:"content"`
	CreatedOn          time.Time      `json:"createdOn"`
	CreatedBy          *UserReference `json:"createdBy"`
	CreatedByFirstName string         `json:"createdByFirstName"`
	CreatedByLastName  string         `json:"createdByLastName"`
	CommentType        *CommentType   `json:"commentType,omitempty"`
	FileName           string         `json:"fileName,omitempty"`
	ContentType        string         `json:"contentType,omitempty"`
	SizeBytes          int            `json:"sizeBytes,omitempty"`
	DownloadURL        string         `json:"downloadUrl,omitempty"`
	Changes            []FieldChange  `json:"changes,omitempty"`
}

// SearchCaseActivitiesRequest is the input for POST /cases/{id}/activities/search.
// CaseID is populated from the URL path parameter and is not part of the JSON body.
type SearchCaseActivitiesRequest struct {
	CaseID              string     `json:"-"`
	Pagination          Pagination `json:"pagination"`
	IncludeFieldChanges *bool      `json:"includeFieldChanges,omitempty"`
}

// SearchCaseActivitiesResponse is entity-service's response for POST /cases/{id}/activities/search.
type SearchCaseActivitiesResponse struct {
	Activity []CaseActivity `json:"activity"`
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
	HasMore  bool           `json:"hasMore"`
}

// --- comments ---
//
// Generic comments attached to any reference entity (case, conversation,
// change_request, deployment, incident — see ReferenceType above) — distinct
// from case-specific comments (see CreateCaseCommentRequest above).

// CreateCommentRequest is the input for POST /comments. Type is always
// CommentTypeComment for portal-originated comments — see
// dto.BuildEntityCreateCommentRequest.
type CreateCommentRequest struct {
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Type          CommentType   `json:"type"`
	Content       string        `json:"content"`
	// CreatedBy overrides the comment's author, which entity-service otherwise
	// derives from the caller's own token. Only ever set by this backend itself
	// (to CreatedByAgent, for an AI reply) — never populated from a client
	// request body, or a customer could post as the assistant.
	CreatedBy string `json:"createdBy,omitempty"`
}

// CreatedByAgent is the CreateCommentRequest.CreatedBy value that attributes a
// comment to the Novera AI assistant instead of the customer whose token
// relayed it. Mirrors the Ballerina backend's entity:CHAT_SENT_AGENT.
//
// Note the asymmetry between what is written and what is read back: the write
// value is "agent", but ServiceNow resolves it to the Novera user, so reads
// return createdBy "novera" — which is what the webapp matches on to render
// the assistant's bubble (see ConversationDetailsPage's isBot check). Don't
// "fix" this constant to "novera"; that is the read form, not the write form.
const CreatedByAgent = "agent"

// CommentCreated carries the fields returned after creating a comment.
type CommentCreated struct {
	ID        string         `json:"id"`
	CreatedOn time.Time      `json:"createdOn"`
	CreatedBy *UserReference `json:"createdBy"`
}

// CreateCommentResponse is entity-service's response for POST /comments.
type CreateCommentResponse struct {
	Message string         `json:"message"`
	Comment CommentCreated `json:"comment"`
}

// CommentFilters holds optional filter criteria for POST /comments/search.
type CommentFilters struct {
	Type *CommentType `json:"type,omitempty"`
}

// SearchCommentsRequest is the input for POST /comments/search.
type SearchCommentsRequest struct {
	ReferenceID   string          `json:"referenceId"`
	ReferenceType ReferenceType   `json:"referenceType"`
	Pagination    Pagination      `json:"pagination"`
	Filters       *CommentFilters `json:"filters,omitempty"`
}

// CommentView is a single search result item from POST /comments/search.
// InlineAttachment is an image embedded in a comment body.
type InlineAttachment struct {
	ID          string     `json:"id"`
	FileName    string     `json:"fileName"`
	ContentType string     `json:"contentType"`
	DownloadURL string     `json:"downloadUrl"`
	CreatedOn   *time.Time `json:"createdOn"`
	CreatedBy   string     `json:"createdBy"`
}

type CommentView struct {
	ID          string         `json:"id"`
	ReferenceID string         `json:"referenceId"`
	Content     string         `json:"content"`
	Type        CommentType    `json:"type"`
	CreatedOn   time.Time      `json:"createdOn"`
	CreatedBy   *UserReference `json:"createdBy"`
	// Images embedded in the comment body, supplied by entity-service.
	HasInlineAttachments bool               `json:"hasInlineAttachments"`
	InlineAttachments    []InlineAttachment `json:"inlineAttachments"`
}

// SearchCommentsResponse is entity-service's response for POST /comments/search.
type SearchCommentsResponse struct {
	Comments []CommentView `json:"comments"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	HasMore  bool          `json:"hasMore"`
}

// --- products ---
//
// Like accounts, entity-service returns a different wire shape for these
// routes depending on its DATA_SOURCE. The structs below are a superset of
// both shapes for the same reasons documented on AccountSummary/AccountDetail
// above: no colliding JSON keys, and every ambiguous-typed field (class,
// dates) is typed as *string so either shape decodes cleanly.

// ProductView is a single search result item from POST /products/search.
type ProductView struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Class     *string `json:"class,omitempty"`
	CreatedOn *string `json:"createdOn,omitempty"`
	UpdatedOn *string `json:"updatedOn,omitempty"`
}

// SearchProductsRequest is the input for POST /products/search.
type SearchProductsRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery,omitempty"`
}

// SearchProductsResponse is entity-service's response for POST /products/search.
type SearchProductsResponse struct {
	Products []ProductView `json:"products"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	HasMore  bool          `json:"hasMore"`
}

// SearchProductVersionsRequest is the input for POST /products/{id}/versions/search.
// ProductID is populated from the URL path parameter and is not part of the JSON body.
type SearchProductVersionsRequest struct {
	Pagination  Pagination `json:"pagination"`
	ProductID   string     `json:"-"`
	SearchQuery string     `json:"searchQuery,omitempty"`
}

// ProductVersionView is a single search result item from
// POST /products/{id}/versions/search.
type ProductVersionView struct {
	ID                             string  `json:"id"`
	ProductID                      string  `json:"productId"`
	Version                        string  `json:"version"`
	CurrentSupportStatus           *string `json:"currentSupportStatus,omitempty"`
	ReleaseDate                    *string `json:"releaseDate,omitempty"`
	SupportEOLDate                 *string `json:"supportEolDate,omitempty"`
	EarliestPossibleSupportEOLDate *string `json:"earliestPossibleSupportEolDate,omitempty"`
	CreatedOn                      *string `json:"createdOn,omitempty"`
	UpdatedOn                      *string `json:"updatedOn,omitempty"`
}

// SearchProductVersionsResponse is entity-service's response for POST /products/{id}/versions/search.
type SearchProductVersionsResponse struct {
	ProductVersions []ProductVersionView `json:"productVersions"`
	Total           int                  `json:"total"`
	Limit           int                  `json:"limit"`
	Offset          int                  `json:"offset"`
	HasMore         bool                 `json:"hasMore"`
}

// --- product vulnerabilities ---

// SearchProductVulnerabilitiesFilters holds the optional filter criteria for
// a product vulnerability search.
type SearchProductVulnerabilitiesFilters struct {
	SearchQuery    string  `json:"searchQuery,omitempty"`
	Priority       *string `json:"priority,omitempty"`
	ProductName    string  `json:"productName,omitempty"`
	ProductVersion string  `json:"productVersion,omitempty"`
}

// SearchProductVulnerabilitiesRequest is the input for POST /products/vulnerabilities/search.
type SearchProductVulnerabilitiesRequest struct {
	Filters    *SearchProductVulnerabilitiesFilters `json:"filters,omitempty"`
	Pagination Pagination                           `json:"pagination"`
}

// ProductVulnerabilityView is entity-service's representation of a
// vulnerability, returned both in search results and as the single-item
// GET response.
type ProductVulnerabilityView struct {
	ID              string  `json:"id"`
	CveID           string  `json:"cveId"`
	VulnerabilityID string  `json:"vulnerabilityId"`
	Priority        string  `json:"priority"`
	ProductName     *string `json:"productName"`
	ProductVersion  *string `json:"productVersion"`
	ComponentName   string  `json:"componentName"`
	Version         string  `json:"version"`
	Type            string  `json:"type"`
	ComponentType   *string `json:"componentType"`
	UpdateLevel     *string `json:"updateLevel"`
	UseCase         *string `json:"useCase"`
	Justification   *string `json:"justification"`
	Resolution      *string `json:"resolution"`
}

// SearchProductVulnerabilitiesResponse is entity-service's response for
// POST /products/vulnerabilities/search.
type SearchProductVulnerabilitiesResponse struct {
	ProductVulnerabilities []ProductVulnerabilityView `json:"productVulnerabilities"`
	Total                  int                        `json:"total"`
	Limit                  int                        `json:"limit"`
	Offset                 int                        `json:"offset"`
}

// --- catalogs ---

// CatalogItem is a single selectable item within a service catalog.
type CatalogItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CatalogView represents a service catalog containing one or more catalog items.
type CatalogView struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	CatalogItems []CatalogItem `json:"catalogItems"`
}

// SearchCatalogsRequest is the input for POST /catalogs/search. DeployedProductID
// scopes the search to catalogs available for that deployed product.
type SearchCatalogsRequest struct {
	DeployedProductID string     `json:"deployedProductId"`
	Pagination        Pagination `json:"pagination"`
}

// SearchCatalogsResponse is entity-service's response for POST /catalogs/search.
type SearchCatalogsResponse struct {
	Catalogs []CatalogView `json:"catalogs"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
}

// CatalogItemVariable describes a single variable (form field) on a catalog item.
type CatalogItemVariable struct {
	ID           string `json:"id"`
	QuestionText string `json:"questionText"`
	Order        int    `json:"order"`
	Type         string `json:"type"`
}

// GetCatalogItemVariablesResponse is entity-service's response for
// GET /catalogs/{catalogId}/items/{catalogItemId}/variables.
type GetCatalogItemVariablesResponse struct {
	Variables []CatalogItemVariable `json:"variables"`
}

// --- time cards ---

// TimeCardFilters holds the optional filter criteria for a time card search.
type TimeCardFilters struct {
	ProjectIDs []string `json:"projectIds,omitempty"`
	CaseID     *string  `json:"caseId,omitempty"`
	StartDate  *string  `json:"startDate,omitempty"`
	EndDate    *string  `json:"endDate,omitempty"`
	States     []string `json:"states,omitempty"`
}

// TimeCardSort specifies the sort field and direction for time-card search results.
type TimeCardSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// SearchTimeCardsRequest is the input for POST /time-cards/search. entity-service
// also accepts userId/approverId/approvedById/userIds filters that scope results
// to a specific WSO2 engineer — deliberately not exposed here since the portal
// has no notion of a customer selecting a WSO2 engineer to filter by.
type SearchTimeCardsRequest struct {
	Filters    *TimeCardFilters `json:"filters,omitempty"`
	SortBy     TimeCardSort     `json:"sortBy"`
	Pagination Pagination       `json:"pagination"`
}

// TimeCardRef is a lightweight reference used for user, approvedBy, and project fields.
type TimeCardRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TimeCardCaseRef is a reference to the case associated with a time card.
type TimeCardCaseRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
}

// TimeCardView is a single time card in search results. entity-service also
// returns per-category time breakdowns (timeAnalyzing, timeSettingUp, etc.),
// issueComplexity, workLogComment, rejectionReason, and the eligible-approvers
// list — internal WSO2 support bookkeeping this backend does not expose to
// the customer portal; mirrored here only insofar as this backend actually
// decodes them, but see dto.TimeCardSummary for what the portal exposes.
type TimeCardView struct {
	ID              string           `json:"id"`
	TotalTime       float64          `json:"totalTime"`
	WorkDate        string           `json:"workDate"`
	HasBillable     bool             `json:"hasBillable"`
	IssueComplexity *string          `json:"issueComplexity"`
	WorkLogComment  *string          `json:"workLogComment"`
	RejectionReason *string          `json:"rejectionReason"`
	State           *string          `json:"state"`
	User            *TimeCardRef     `json:"user"`
	ApprovedBy      *TimeCardRef     `json:"approvedBy"`
	Project         *TimeCardRef     `json:"project"`
	Case            *TimeCardCaseRef `json:"case"`
}

// SearchTimeCardsResponse is entity-service's response for POST /time-cards/search.
type SearchTimeCardsResponse struct {
	TimeCards []TimeCardView `json:"timeCards"`
	Total     int            `json:"total"`
	Limit     int            `json:"limit"`
	Offset    int            `json:"offset"`
}

// --- conversations ---
//
// Backing store for the AI chat feature's conversation threads. entity-service
// currently only supports searching conversations — there is no create/update/
// get-single route yet (see internal/handler/ai_chat.go's doc comment for the
// features this blocks).

// SearchConversationsFilters holds the optional filter criteria for a conversation search.
type SearchConversationsFilters struct {
	ProjectIDs  []string `json:"projectIds,omitempty"`
	States      []string `json:"states,omitempty"`
	SearchQuery string   `json:"searchQuery,omitempty"`
	CreatedByMe bool     `json:"createdByMe,omitempty"`
}

// ConversationSort specifies the sort field and direction for conversation search results.
type ConversationSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// SearchConversationsRequest is the input for POST /conversations/search.
type SearchConversationsRequest struct {
	Filters    SearchConversationsFilters `json:"filters"`
	SortBy     ConversationSort           `json:"sortBy"`
	Pagination Pagination                 `json:"pagination"`
}

// SearchConversationView is the conversation representation returned in search results.
type SearchConversationView struct {
	ID             *string        `json:"id"`
	Number         *string        `json:"number"`
	InitialMessage *string        `json:"initialMessage"`
	MessageCount   int            `json:"messageCount"`
	Project        *EntityRef     `json:"project"`
	Case           *EntityRef     `json:"case"`
	State          *string        `json:"state"`
	CreatedOn      string         `json:"createdOn"`
	CreatedBy      *UserReference `json:"createdBy"`
}

// SearchConversationsResponse is entity-service's response for POST /conversations/search.
type SearchConversationsResponse struct {
	Conversations []SearchConversationView `json:"conversations"`
	Total         int                      `json:"total"`
	Offset        int                      `json:"offset"`
	Limit         int                      `json:"limit"`
}

// --- change requests ---
//
// entity-service only supports change requests on its ServiceNow data
// source — DATA_SOURCE=postgres deployments don't register these routes at
// all (404, not a data-shape difference like accounts/products).

// CreateChangeRequestRequest is the input for POST /change-requests. Subject
// is the only required field. entity-service's category/priority/impact/
// type/state/risk fields are plain strings here (not named Go enum types),
// matching this file's convention elsewhere (e.g. CaseView.Severity).
type CreateChangeRequestRequest struct {
	Subject             string  `json:"subject"`
	Category            *string `json:"category,omitempty"`
	ServiceID           *string `json:"serviceId,omitempty"`
	ServiceOfferingID   *string `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string `json:"configurationItemId,omitempty"`
	Priority            *string `json:"priority,omitempty"`
	Impact              *string `json:"impact,omitempty"`
	Type                *string `json:"type,omitempty"`
	State               *string `json:"state,omitempty"`
	GroupID             *string `json:"groupId,omitempty"`
	AssignedEngineerID  *string `json:"assignedEngineerId,omitempty"`
	Risk                *string `json:"risk,omitempty"`
	RequestedByID       *string `json:"requestedById,omitempty"`
	Description         *string `json:"description,omitempty"`
	Justification       *string `json:"justification,omitempty"`
	ImplementationPlan  *string `json:"implementationPlan,omitempty"`
	RiskImpactAnalysis  *string `json:"riskImpactAnalysis,omitempty"`
	BackoutPlan         *string `json:"backoutPlan,omitempty"`
	TestPlan            *string `json:"testPlan,omitempty"`
	PlannedStartDate    *string `json:"plannedStartDate,omitempty"`
	PlannedEndDate      *string `json:"plannedEndDate,omitempty"`
	Comment             *string `json:"comment,omitempty"`
	WorkNote            *string `json:"workNote,omitempty"`
}

// ChangeRequestCreated carries the key fields of a newly created change request.
type ChangeRequestCreated struct {
	ID        string `json:"id"`
	Number    string `json:"number"`
	CreatedOn string `json:"createdOn"`
	CreatedBy string `json:"createdBy"`
}

// CreateChangeRequestResponse is entity-service's response for POST /change-requests.
type CreateChangeRequestResponse struct {
	Message       string               `json:"message"`
	ChangeRequest ChangeRequestCreated `json:"changeRequest"`
}

// ChangeRequestSort specifies the sort field and direction for change request search results.
type ChangeRequestSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// SearchChangeRequestsFilters holds the optional filter criteria for a change request search.
type SearchChangeRequestsFilters struct {
	ProjectIDs      []string `json:"projectIds,omitempty"`
	SearchQuery     string   `json:"searchQuery,omitempty"`
	States          []string `json:"states,omitempty"`
	Impacts         []string `json:"impacts,omitempty"`
	ClosedStartDate *string  `json:"closedStartDate,omitempty"`
	ClosedEndDate   *string  `json:"closedEndDate,omitempty"`
}

// SearchChangeRequestsRequest is the input for POST /change-requests/search.
type SearchChangeRequestsRequest struct {
	Filters    SearchChangeRequestsFilters `json:"filters"`
	SortBy     ChangeRequestSort           `json:"sortBy"`
	Pagination Pagination                  `json:"pagination"`
}

// SearchChangeRequestView is a single search result item from POST /change-requests/search.
type SearchChangeRequestView struct {
	ID               string     `json:"id"`
	Number           string     `json:"number"`
	Subject          *string    `json:"subject"`
	Description      *string    `json:"description"`
	Project          EntityRef  `json:"project"`
	Case             *EntityRef `json:"case"`
	Deployment       *EntityRef `json:"deployment"`
	DeployedProduct  *EntityRef `json:"deployedProduct"`
	Product          *EntityRef `json:"product"`
	AssignedEngineer *EntityRef `json:"assignedEngineer"`
	AssignedTeam     *EntityRef `json:"assignedTeam"`
	PlannedStartOn   *string    `json:"plannedStartOn"`
	PlannedEndOn     *string    `json:"plannedEndOn"`
	Duration         *string    `json:"duration"`
	Impact           *string    `json:"impact"`
	State            *string    `json:"state"`
	Type             *string    `json:"type"`
	CreatedOn        string     `json:"createdOn"`
	UpdatedOn        string     `json:"updatedOn"`
}

// SearchChangeRequestsResponse is entity-service's response for POST /change-requests/search.
type SearchChangeRequestsResponse struct {
	ChangeRequests []SearchChangeRequestView `json:"changeRequests"`
	Total          int                       `json:"total"`
	Offset         int                       `json:"offset"`
	Limit          int                       `json:"limit"`
}

// ChangeRequest is entity-service's response for GET /change-requests/{id}.
// Embeds SearchChangeRequestView (matching entity-service's own struct
// embedding, which flattens its fields into the same JSON object).
type ChangeRequest struct {
	SearchChangeRequestView
	CreatedBy           string     `json:"createdBy"`
	Justification       *string    `json:"justification"`
	ImpactDescription   *string    `json:"impactDescription"`
	ServiceOutage       *string    `json:"serviceOutage"`
	CommunicationPlan   *string    `json:"communicationPlan"`
	RollbackPlan        *string    `json:"rollbackPlan"`
	TestPlan            *string    `json:"testPlan"`
	HasCustomerApproved bool       `json:"hasCustomerApproved"`
	HasCustomerReviewed bool       `json:"hasCustomerReviewed"`
	ApprovedBy          *EntityRef `json:"approvedBy"`
	ApprovedOn          *string    `json:"approvedOn"`
	LegalNextStates     []string   `json:"legalNextStates"`
}

// PatchChangeRequestRequest is the full field set entity-service accepts for
// PATCH /change-requests/{id}. This is entity-service's raw contract — the
// portal only exposes a customer-safe subset; see
// dto.ChangeRequestUpdateRequest for which ones and why.
type PatchChangeRequestRequest struct {
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	ProjectID          *string `json:"projectId,omitempty"`
	CaseID             *string `json:"caseId,omitempty"`
	DeploymentID       *string `json:"deploymentId,omitempty"`
	DeployedProductID  *string `json:"deployedProductId,omitempty"`
	AssignedEngineerID *string `json:"assignedEngineerId,omitempty"`
	AssignedTeamID     *string `json:"assignedTeamId,omitempty"`
	PlannedStartOn     *string `json:"plannedStartOn,omitempty"`
	PlannedEndOn       *string `json:"plannedEndOn,omitempty"`
	Impact             *string `json:"impact,omitempty"`
	State              *string `json:"state,omitempty"`
	Type               *string `json:"type,omitempty"`
	Justification      *string `json:"justification,omitempty"`
	ImpactDescription  *string `json:"impactDescription,omitempty"`
	ServiceOutage      *string `json:"serviceOutage,omitempty"`
	CommunicationPlan  *string `json:"communicationPlan,omitempty"`
	RollbackPlan       *string `json:"rollbackPlan,omitempty"`
	TestPlan           *string `json:"testPlan,omitempty"`
	IsCustomerApproved *bool   `json:"isCustomerApproved,omitempty"`
	IsCustomerReviewed *bool   `json:"isCustomerReviewed,omitempty"`
	RequestApproval    *bool   `json:"requestApproval,omitempty"`
}

// PatchChangeRequestResponse is entity-service's response for PATCH /change-requests/{id}.
type PatchChangeRequestResponse struct {
	Message       string        `json:"message"`
	ChangeRequest ChangeRequest `json:"changeRequest"`
}

// ChangeRequestApprover is a single approver's response within an approval
// stage. Status is deliberately a plain string, not a closed enum —
// entity-service documents it as an open set (APPROVED, NOT_REQUIRED,
// REQUESTED, REJECTED, CANCELLED, NO_CONSENSUS, or an unrecognized
// uppercased value).
type ChangeRequestApprover struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	RespondedOn *string `json:"respondedOn"`
}

// ChangeRequestApproval represents a single approval stage (e.g. Assess,
// Authorize, Customer Approval) on a change request.
type ChangeRequestApproval struct {
	Stage        string                  `json:"stage"`
	ApproverType string                  `json:"approverType"`
	ApproverName string                  `json:"approverName"`
	Status       string                  `json:"status"`
	Approvers    []ChangeRequestApprover `json:"approvers"`
}

// ChangeRequestApprovals is entity-service's response for GET /change-requests/{id}/approvals.
type ChangeRequestApprovals struct {
	Approvals []ChangeRequestApproval `json:"approvals"`
}

// ChangeRequestApprovalDecisionRequest is the input for
// POST /change-requests/{id}/approvals/decision. Decision is a plain string
// ("approved" or "rejected" per entity-service's doc comment), not an enum type.
type ChangeRequestApprovalDecisionRequest struct {
	Decision string `json:"decision"`
}

// ChangeRequestApprovalDecisionResponse is entity-service's response for
// POST /change-requests/{id}/approvals/decision.
type ChangeRequestApprovalDecisionResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// --- call requests ---
//
// entity-service only supports call requests on its ServiceNow data source.

// CreateCallRequestRequest is the input for POST /call-requests.
type CreateCallRequestRequest struct {
	CaseID          string   `json:"caseId"`
	Reason          string   `json:"reason"`
	UTCTimes        []string `json:"utcTimes"`
	DurationMinutes int      `json:"durationInMinutes"`
}

// CallRequestCreated carries the key fields of a newly created call request.
type CallRequestCreated struct {
	ID        string `json:"id"`
	CreatedOn string `json:"createdOn"`
	CreatedBy string `json:"createdBy"`
	State     string `json:"state"`
}

// CreateCallRequestResponse is entity-service's response for POST /call-requests.
type CreateCallRequestResponse struct {
	Message     string             `json:"message"`
	CallRequest CallRequestCreated `json:"callRequest"`
}

// CallRequestState holds the state of a call request: ID is the string state
// enum key, Label is the human-readable display label.
type CallRequestState struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CallRequestCaseRef is a reference to a case embedded in a call request.
type CallRequestCaseRef struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Number *string `json:"number,omitempty"`
}

// SearchCallRequestsFilters holds the optional filter criteria for a call request search.
type SearchCallRequestsFilters struct {
	States []string `json:"states,omitempty"`
}

// SearchCallRequestsRequest is the input for POST /call-requests/search.
type SearchCallRequestsRequest struct {
	CaseID     string                     `json:"caseId"`
	Filters    *SearchCallRequestsFilters `json:"filters,omitempty"`
	Pagination Pagination                 `json:"pagination"`
}

// CallRequestView is a single search result item from POST /call-requests/search.
// Assignee/Notes/Plan/Attendees/ActionItems/ActualDurationMin are agent-side
// fields, populated once a support engineer schedules or concludes the call.
type CallRequestView struct {
	ID                 string             `json:"id"`
	Number             string             `json:"number"`
	Case               CallRequestCaseRef `json:"case"`
	Reason             *string            `json:"reason"`
	PreferredTimes     []string           `json:"preferredTimes"`
	DurationMin        int                `json:"durationMin"`
	ScheduleTime       *string            `json:"scheduleTime"`
	MeetingLink        *string            `json:"meetingLink"`
	CreatedOn          string             `json:"createdOn"`
	UpdatedOn          string             `json:"updatedOn"`
	State              CallRequestState   `json:"state"`
	CancellationReason *string            `json:"cancellationReason,omitempty"`
	Assignee           *string            `json:"assignee,omitempty"`
	Notes              *string            `json:"notes,omitempty"`
	Plan               *string            `json:"plan,omitempty"`
	Attendees          *string            `json:"attendees,omitempty"`
	ActionItems        *string            `json:"actionItems,omitempty"`
	ActualDurationMin  *int               `json:"actualDurationMin,omitempty"`
}

// SearchCallRequestsResponse is entity-service's response for POST /call-requests/search.
type SearchCallRequestsResponse struct {
	CallRequests []CallRequestView `json:"callRequests"`
	Total        int               `json:"total"`
	Offset       int               `json:"offset"`
	Limit        int               `json:"limit"`
}

// UpdateCallRequestRequest is the full field set entity-service accepts for
// PATCH /call-requests/{id}. This is entity-service's raw contract — the
// portal only exposes a customer-safe subset; see
// dto.CallRequestUpdateRequest for which ones and why (the agent-side
// fields below must never be customer-settable).
type UpdateCallRequestRequest struct {
	ID                 string   `json:"-"`
	CaseID             string   `json:"caseId,omitempty"`
	State              string   `json:"state"`
	CancellationReason *string  `json:"cancellationReason,omitempty"`
	UTCTimes           []string `json:"utcTimes,omitempty"`
	DurationMinutes    *int     `json:"durationInMinutes,omitempty"`
	// Agent-side fields, set when an engineer schedules or concludes the call.
	MeetingDate       *string `json:"meetingDate,omitempty"`
	Assignee          *string `json:"assignee,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	Plan              *string `json:"plan,omitempty"`
	Attendees         *string `json:"attendees,omitempty"`
	ActionItems       *string `json:"actionItems,omitempty"`
	ActualDurationMin *int    `json:"actualDurationMin,omitempty"`
}

// CallRequestUpdated carries the fields that may change after an update.
type CallRequestUpdated struct {
	ID        string `json:"id"`
	UpdatedOn string `json:"updatedOn"`
	UpdatedBy string `json:"updatedBy"`
}

// UpdateCallRequestResponse is entity-service's response for PATCH /call-requests/{id}.
type UpdateCallRequestResponse struct {
	Message     string             `json:"message"`
	CallRequest CallRequestUpdated `json:"callRequest"`
}

// --- global metadata and search ---

// FeedbackEmojiChip mirrors entity-service's domain.FeedbackEmojiChip.
type FeedbackEmojiChip struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// FeedbackEmoji mirrors entity-service's domain.FeedbackEmoji.
type FeedbackEmoji struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Value           string              `json:"value"`
	UnselectedImage string              `json:"unselectedImage"`
	SelectedImage   string              `json:"selectedImage"`
	Chips           []FeedbackEmojiChip `json:"chips"`
}

// SystemMetadataResponse is entity-service's response for GET /metadata.
type SystemMetadataResponse struct {
	TimeZones      []ChoiceListItem     `json:"timeZones"`
	ProjectTypes   []ReferenceTableItem `json:"projectTypes"`
	FeedbackEmojis []FeedbackEmoji      `json:"feedbackEmojies"`
}

// GlobalSearchFilters holds the optional filter criteria for POST /search.
type GlobalSearchFilters struct {
	SearchQuery string   `json:"searchQuery,omitempty"`
	Tables      []string `json:"tables,omitempty"`
}

// GlobalSearchSort specifies the sort field/order for POST /search.
type GlobalSearchSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// GlobalSearchRequest is entity-service's request body for POST /search.
type GlobalSearchRequest struct {
	Filters            *GlobalSearchFilters `json:"filters,omitempty"`
	SortBy             *GlobalSearchSort    `json:"sortBy,omitempty"`
	ProjectsPagination *Pagination          `json:"projectsPagination,omitempty"`
	CasesPagination    *Pagination          `json:"casesPagination,omitempty"`
}

// GlobalSearchProject is a project row in a global search result.
type GlobalSearchProject struct {
	ID                  string             `json:"id"`
	Name                string             `json:"name"`
	Description         *string            `json:"description"`
	Key                 string             `json:"key"`
	Type                ReferenceTableItem `json:"type"`
	CreatedOn           string             `json:"createdOn"`
	StartDate           *string            `json:"startDate"`
	EndDate             *string            `json:"endDate"`
	HasPdpSubscription  bool               `json:"hasPdpSubscription"`
	ClosureState        *string            `json:"closureState"`
	Account             ReferenceTableItem `json:"account"`
	ActiveChatsCount    int                `json:"activeChatsCount"`
	ActionRequiredCount int                `json:"actionRequiredCount"`
	OutstandingCount    int                `json:"outstandingCount"`
}

// GlobalSearchCase is a case row in a global search result.
type GlobalSearchCase struct {
	ID               string              `json:"id"`
	InternalID       string              `json:"internalId"`
	Number           string              `json:"number"`
	Title            *string             `json:"title"`
	Description      *string             `json:"description"`
	CreatedOn        string              `json:"createdOn"`
	CreatedBy        string              `json:"createdBy"`
	UpdatedOn        string              `json:"updatedOn"`
	Project          *ReferenceTableItem `json:"project"`
	CaseType         *ReferenceTableItem `json:"caseType"`
	State            *ChoiceListItem     `json:"state"`
	Severity         *ChoiceListItem     `json:"severity"`
	AssignedEngineer *ReferenceTableItem `json:"assignedEngineer"`
	Account          ReferenceTableItem  `json:"account"`
}

// GlobalSearchResponse is entity-service's response for POST /search.
type GlobalSearchResponse struct {
	Query         string                `json:"query"`
	ProjectsTotal int                   `json:"projectsTotal"`
	CasesTotal    int                   `json:"casesTotal"`
	Projects      []GlobalSearchProject `json:"projects"`
	Cases         []GlobalSearchCase    `json:"cases"`
}

// VulnerabilityMetaResponse is entity-service's response for GET /products/vulnerabilities/meta.
type VulnerabilityMetaResponse struct {
	Severities []ChoiceListItem `json:"severities"`
}

// --- conversations CRUD ---

// ConversationDetails is entity-service's response for GET /conversations/{id}.
type ConversationDetails struct {
	ID             string     `json:"id"`
	Number         *string    `json:"number"`
	InitialMessage *string    `json:"initialMessage"`
	MessageCount   int        `json:"messageCount"`
	Project        *EntityRef `json:"project"`
	Case           *EntityRef `json:"case"`
	State          *string    `json:"state"`
	CreatedOn      string     `json:"createdOn"`
	CreatedBy      string     `json:"createdBy"`
	UpdatedOn      string     `json:"updatedOn"`
	UpdatedBy      string     `json:"updatedBy"`
}

// CreateConversationRequest is entity-service's request body for POST /conversations.
type CreateConversationRequest struct {
	ProjectID      string `json:"projectId"`
	InitialMessage string `json:"initialMessage"`
}

// CreatedConversation is the conversation summary returned after creation.
type CreatedConversation struct {
	ID        string  `json:"id"`
	Number    string  `json:"number"`
	CreatedBy string  `json:"createdBy"`
	CreatedOn string  `json:"createdOn"`
	State     *string `json:"state"`
}

// CreateConversationResponse is entity-service's response for POST /conversations.
type CreateConversationResponse struct {
	Message      string              `json:"message"`
	Conversation CreatedConversation `json:"conversation"`
}

// UpdateConversationRequest is entity-service's request body for PATCH
// /conversations/{id}. State must be one of ACTIVE, RESOLVED, CONVERTED,
// ABANDONED, or CLOSED.
type UpdateConversationRequest struct {
	State string `json:"state"`
}

// UpdatedConversation is the conversation summary returned after an update.
type UpdatedConversation struct {
	ID        string  `json:"id"`
	Number    *string `json:"number"`
	UpdatedOn string  `json:"updatedOn"`
	UpdatedBy string  `json:"updatedBy"`
	State     *string `json:"state"`
}

// UpdateConversationResponse is entity-service's response for PATCH /conversations/{id}.
type UpdateConversationResponse struct {
	Message      string              `json:"message"`
	Conversation UpdatedConversation `json:"conversation"`
}

// --- case feedback ---

// SubmitCaseFeedbackRequest is entity-service's request body for POST /cases/{id}/feedback.
type SubmitCaseFeedbackRequest struct {
	EmojiID           string   `json:"emojiId"`
	ChipIDs           []string `json:"chipIds,omitempty"`
	AdditionalComment *string  `json:"additionalComment,omitempty"`
}

// CaseFeedbackResult is the feedback record created by a submission.
type CaseFeedbackResult struct {
	ID           string `json:"id"`
	AssessmentID string `json:"assessmentId"`
	CaseID       string `json:"caseId"`
	CreatedBy    string `json:"createdBy"`
	CreatedOn    string `json:"createdOn"`
}

// SubmitCaseFeedbackResponse is entity-service's response for POST /cases/{id}/feedback.
type SubmitCaseFeedbackResponse struct {
	Message  string             `json:"message"`
	Feedback CaseFeedbackResult `json:"feedback"`
}

// CaseFeedbackEmojiRef is the emoji reference embedded in GET /cases/{id}/feedback.
type CaseFeedbackEmojiRef struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	SelectedImage string `json:"selectedImage"`
}

// CaseFeedback is entity-service's response for GET /cases/{id}/feedback.
type CaseFeedback struct {
	ID                string               `json:"id"`
	Emoji             CaseFeedbackEmojiRef `json:"emoji"`
	ChipIDs           []string             `json:"chips"`
	AssessmentID      string               `json:"assessmentId"`
	CreatedBy         string               `json:"createdBy"`
	CreatedOn         string               `json:"createdOn"`
	AdditionalComment *string              `json:"additionalComment"`
}

// --- attachment gaps ---

// AttachmentDetails is entity-service's response for GET /attachments/{id}.
type AttachmentDetails struct {
	ID          string    `json:"id"`
	ReferenceID string    `json:"referenceId"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	SizeBytes   int       `json:"sizeBytes"`
	Description *string   `json:"description"`
	CreatedBy   string    `json:"createdBy"`
	CreatedOn   time.Time `json:"createdOn"`
	DownloadURL *string   `json:"downloadUrl"`
	PreviewURL  *string   `json:"previewUrl"`
	// Content is nil for a CSM-native (Postgres) data source attachment:
	// entity-service holds no bytes for it, only its storage key. Always
	// non-nil for ServiceNow-sourced attachments.
	Content *string `json:"content"`
}

// UpdateAttachmentRequest is entity-service's request body for PATCH /attachments/{id}.
type UpdateAttachmentRequest struct {
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Name          *string       `json:"name,omitempty"`
	Description   *string       `json:"description,omitempty"`
}

// UpdatedAttachment holds the fields returned after updating an attachment.
type UpdatedAttachment struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
}

// UpdateAttachmentResponse is entity-service's response for PATCH /attachments/{id}.
type UpdateAttachmentResponse struct {
	Message    string            `json:"message"`
	Attachment UpdatedAttachment `json:"attachment"`
}

// --- deployed product metrics ---

// DeployedProductMetricsRequest is entity-service's request body for
// POST /deployed-products/{id}/metrics/search.
type DeployedProductMetricsRequest struct {
	DeploymentID string `json:"deploymentId"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
}

// DeployedProductMetricsInstance is a single instance's contribution to a
// metrics chart entry.
type DeployedProductMetricsInstance struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Cores int    `json:"cores"`
}

// DeployedProductMetricsChartEntry is one date's worth of core-count data.
type DeployedProductMetricsChartEntry struct {
	Date          string                           `json:"date"`
	InstanceCount int                              `json:"instanceCount"`
	TotalCores    int                              `json:"totalCores"`
	MinCores      int                              `json:"minCores"`
	MaxCores      int                              `json:"maxCores"`
	AvgCores      float64                          `json:"avgCores"`
	Instances     []DeployedProductMetricsInstance `json:"instances"`
}

// DeployedProductMetricsDateRange is the queried date range echoed back in a summary.
type DeployedProductMetricsDateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// DeployedProductMetricsSummary is the summary statistics for a metrics query.
type DeployedProductMetricsSummary struct {
	DateRange      DeployedProductMetricsDateRange `json:"dateRange"`
	TotalInstances int                             `json:"totalInstances"`
	MinCores       *int                            `json:"minCores"`
	MaxCores       *int                            `json:"maxCores"`
	AvgCores       *float64                        `json:"avgCores"`
}

// DeployedProductMetricsResponse is entity-service's response for
// POST /deployed-products/{id}/metrics/search.
type DeployedProductMetricsResponse struct {
	DeployedProduct ReferenceTableItem                 `json:"deployedProduct"`
	Summary         DeployedProductMetricsSummary      `json:"summary"`
	ChartData       []DeployedProductMetricsChartEntry `json:"chartData"`
}

// DeployedProductUsageCountsRequest is entity-service's request body for
// POST /deployed-products/{id}/metrics/usage-counts/search.
type DeployedProductUsageCountsRequest struct {
	DeploymentID string `json:"deploymentId"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
}

// UsageCountInstance is a single instance's contribution to a usage-count entry.
type UsageCountInstance struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// UsageCountEntry is one count type's aggregated value on a single date.
type UsageCountEntry struct {
	Value       float64              `json:"value"`
	Aggregation string               `json:"aggregation"`
	Instances   []UsageCountInstance `json:"instances"`
}

// DeployedProductUsageCountsChartEntry is one date's worth of usage-count data.
type DeployedProductUsageCountsChartEntry struct {
	Date   string                     `json:"date"`
	Counts map[string]UsageCountEntry `json:"counts"`
}

// CountTypeAggregation is the summary statistics for a single count type over
// the queried date range.
type CountTypeAggregation struct {
	Aggregation string  `json:"aggregation"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Avg         float64 `json:"avg"`
}

// DeployedProductUsageCountsSummary is the summary statistics for a usage-counts query.
type DeployedProductUsageCountsSummary struct {
	DateRange  DeployedProductMetricsDateRange `json:"dateRange"`
	CountTypes map[string]CountTypeAggregation `json:"countTypes"`
}

// DeployedProductUsageCountsResponse is entity-service's response for
// POST /deployed-products/{id}/metrics/usage-counts/search.
type DeployedProductUsageCountsResponse struct {
	DeployedProduct ReferenceTableItem                     `json:"deployedProduct"`
	Summary         DeployedProductUsageCountsSummary      `json:"summary"`
	ChartData       []DeployedProductUsageCountsChartEntry `json:"chartData"`
}

// --- escalations ---

// CreateEscalationRequest is entity-service's request body for POST /escalations.
type CreateEscalationRequest struct {
	CaseID string  `json:"caseId"`
	Reason *string `json:"reason,omitempty"`
	Action *string `json:"action,omitempty"`
}

// EscalationNotifiedUser is a user notified about an escalation.
type EscalationNotifiedUser struct {
	ID       string  `json:"id"`
	UserName string  `json:"userName"`
	Name     *string `json:"name"`
	Email    *string `json:"email"`
}

// CreatedEscalation holds the escalation details returned after creation.
type CreatedEscalation struct {
	ID                 string                   `json:"id"`
	Case               ReferenceTableItem       `json:"case"`
	CurrentLevel       ChoiceListItem           `json:"currentLevel"`
	PreviousLevel      ChoiceListItem           `json:"previousLevel"`
	CreatedBy          string                   `json:"createdBy"`
	CreatedOn          string                   `json:"createdOn"`
	Reason             *string                  `json:"reason"`
	NotificationSentTo []EscalationNotifiedUser `json:"notificationSentTo"`
}

// CreateEscalationResponse is entity-service's response for POST /escalations.
type CreateEscalationResponse struct {
	Message    string            `json:"message"`
	Escalation CreatedEscalation `json:"escalation"`
}

// Escalation is a single escalation row in search results.
type Escalation struct {
	ID                 string                   `json:"id"`
	Case               ReferenceTableItem       `json:"case"`
	CurrentLevel       ChoiceListItem           `json:"currentLevel"`
	PreviousLevel      ChoiceListItem           `json:"previousLevel"`
	CreatedBy          string                   `json:"createdBy"`
	CreatedOn          string                   `json:"createdOn"`
	UpdatedOn          string                   `json:"updatedOn"`
	Reason             *string                  `json:"reason"`
	NotificationSentTo []EscalationNotifiedUser `json:"notificationSentTo"`
}

// SearchEscalationsFilters holds the optional filter criteria for POST /escalations/search.
type SearchEscalationsFilters struct {
	CaseIDs       []string `json:"caseIds,omitempty"`
	CurrentLevels []int    `json:"currentLevels,omitempty"`
}

// EscalationSort specifies the sort field/order for POST /escalations/search.
type EscalationSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// SearchEscalationsRequest is entity-service's request body for POST /escalations/search.
type SearchEscalationsRequest struct {
	Filters    *SearchEscalationsFilters `json:"filters,omitempty"`
	SortBy     *EscalationSort           `json:"sortBy,omitempty"`
	Pagination Pagination                `json:"pagination"`
}

// SearchEscalationsResponse is the paginated result of an escalation search.
type SearchEscalationsResponse struct {
	Escalations []Escalation `json:"escalations"`
	Total       int          `json:"total"`
	Offset      int          `json:"offset"`
	Limit       int          `json:"limit"`
}

// --- case-grouped time cards ---

// CaseTimeCardBillingInfo is a billable/non-billable time breakdown.
type CaseTimeCardBillingInfo struct {
	TotalTime float64 `json:"totalTime"`
	Count     int     `json:"count"`
}

// CaseTimeCardCaseRef is the case reference embedded in a case time-card summary.
type CaseTimeCardCaseRef struct {
	ID        string     `json:"id"`
	Number    string     `json:"number"`
	Name      string     `json:"name"`
	UpdatedOn string     `json:"updatedOn"`
	Project   *EntityRef `json:"project"`
	CreatedOn *string    `json:"createdOn"`
	CreatedBy *string    `json:"createdBy"`
	UpdatedBy *string    `json:"updatedBy"`
}

// CaseTimeCardSummary is the time-card rollup for a single case.
type CaseTimeCardSummary struct {
	Case        CaseTimeCardCaseRef     `json:"case"`
	TotalTime   float64                 `json:"totalTime"`
	TotalCount  int                     `json:"totalCount"`
	Billable    CaseTimeCardBillingInfo `json:"billable"`
	NonBillable CaseTimeCardBillingInfo `json:"nonBillable"`
}

// SearchCaseTimeCardsResponse is entity-service's response for
// POST /cases/time-cards/search — reuses SearchTimeCardsRequest for the request.
type SearchCaseTimeCardsResponse struct {
	Cases  []CaseTimeCardSummary `json:"cases"`
	Total  int                   `json:"total"`
	Offset int                   `json:"offset"`
	Limit  int                   `json:"limit"`
}

// --- instances / metrics ---

// InstanceSearchFilters holds the optional filter criteria for POST /instances/search.
type InstanceSearchFilters struct {
	StartDate          *string  `json:"startDate,omitempty"`
	EndDate            *string  `json:"endDate,omitempty"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
}

// SearchInstancesRequest is entity-service's request body for POST /instances/search.
type SearchInstancesRequest struct {
	Filters    *InstanceSearchFilters `json:"filters,omitempty"`
	Pagination Pagination             `json:"pagination"`
}

// InstanceMetadata is the metadata block embedded in an Instance.
type InstanceMetadata struct {
	ID                 string         `json:"id"`
	CoreCount          *int           `json:"coreCount"`
	Updates            *int           `json:"updates"`
	JDKVersion         *string        `json:"jdkVersion"`
	DeploymentMetadata map[string]any `json:"deploymentMetadata"`
	CreatedOn          string         `json:"createdOn"`
	UpdatedOn          string         `json:"updatedOn"`
	CustomCreatedOn    *string        `json:"customCreatedOn"`
	CustomUpdatedOn    *string        `json:"customUpdatedOn"`
}

// Instance is a single instance row.
type Instance struct {
	ID              string              `json:"id"`
	Key             string              `json:"key"`
	Project         *ReferenceTableItem `json:"project"`
	Deployment      *ReferenceTableItem `json:"deployment"`
	Product         *ReferenceTableItem `json:"product"`
	DeployedProduct *ReferenceTableItem `json:"deployedProduct"`
	CreatedOn       string              `json:"createdOn"`
	UpdatedOn       string              `json:"updatedOn"`
	Metadata        *InstanceMetadata   `json:"metadata"`
}

// SearchInstancesResponse is entity-service's response for POST /instances/search.
type SearchInstancesResponse struct {
	Instances []Instance `json:"instances"`
	Total     int        `json:"total"`
	Offset    int        `json:"offset"`
	Limit     int        `json:"limit"`
}

// InstanceDateRangeFilters is the shared filter shape for
// POST /instances/metrics/search and POST /instances/usages/search.
type InstanceDateRangeFilters struct {
	StartDate          string   `json:"startDate"`
	EndDate            string   `json:"endDate"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
}

// InstanceMetricsRequest is entity-service's request body for POST /instances/metrics/search.
type InstanceMetricsRequest struct {
	Filters InstanceDateRangeFilters `json:"filters"`
}

// InstanceDataPoint is a single metric snapshot for an instance.
type InstanceDataPoint struct {
	Date               string         `json:"date"`
	CreatedOn          string         `json:"createdOn"`
	CoreCount          *int           `json:"coreCount"`
	JDKVersion         *string        `json:"jdkVersion"`
	Updates            *int           `json:"updates"`
	DeploymentMetadata map[string]any `json:"deploymentMetadata"`
}

// InstanceMetric is one instance's metric time series, ordered newest to oldest.
type InstanceMetric struct {
	InstanceID      string              `json:"instanceId"`
	InstanceKey     string              `json:"instanceKey"`
	Project         *ReferenceTableItem `json:"project"`
	Deployment      *ReferenceTableItem `json:"deployment"`
	Product         *ReferenceTableItem `json:"product"`
	DeployedProduct *ReferenceTableItem `json:"deployedProduct"`
	DataPoints      []InstanceDataPoint `json:"dataPoints"`
}

// InstanceMetricsResponse is entity-service's response for POST /instances/metrics/search.
type InstanceMetricsResponse struct {
	Metrics        []InstanceMetric `json:"metrics"`
	TotalInstances int              `json:"totalInstances"`
	StartDate      string           `json:"startDate"`
	EndDate        string           `json:"endDate"`
}

// InstanceUsageRequest is entity-service's request body for POST /instances/usages/search.
type InstanceUsageRequest struct {
	Filters InstanceDateRangeFilters `json:"filters"`
}

// InstanceSummary is one period's usage counts for an instance.
type InstanceSummary struct {
	Period string         `json:"period"`
	Counts map[string]int `json:"counts"`
}

// InstanceUsageEntry is one instance's usage time series.
type InstanceUsageEntry struct {
	InstanceID      string              `json:"instanceId"`
	InstanceKey     string              `json:"instanceKey"`
	Project         *ReferenceTableItem `json:"project"`
	Deployment      *ReferenceTableItem `json:"deployment"`
	Product         *ReferenceTableItem `json:"product"`
	DeployedProduct *ReferenceTableItem `json:"deployedProduct"`
	PeriodSummaries []InstanceSummary   `json:"periodSummaries"`
}

// InstanceUsageResponse is entity-service's response for POST /instances/usages/search.
type InstanceUsageResponse struct {
	Usages         []InstanceUsageEntry `json:"usages"`
	TotalInstances int                  `json:"totalInstances"`
	StartDate      string               `json:"startDate"`
	EndDate        string               `json:"endDate"`
}

// InstanceStatsFilters extends InstanceDateRangeFilters with an optional
// dataSource discriminator, used by the two .../stats/search endpoints only.
type InstanceStatsFilters struct {
	StartDate          string   `json:"startDate"`
	EndDate            string   `json:"endDate"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
	DataSource         *int     `json:"dataSource,omitempty"`
}

// InstanceMetricsStatsRequest is entity-service's request body for
// POST /instances/metrics/stats/search.
type InstanceMetricsStatsRequest struct {
	Filters InstanceStatsFilters `json:"filters"`
}

// InstanceMetricSummary is the current/min/max/avg summary for a metrics-stats query.
type InstanceMetricSummary struct {
	Current float64 `json:"current"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
}

// InstanceMetricsStatsResponse is entity-service's response for
// POST /instances/metrics/stats/search.
type InstanceMetricsStatsResponse struct {
	Stats     map[string]map[string]int `json:"stats"`
	Summary   InstanceMetricSummary     `json:"summary"`
	Total     int                       `json:"total"`
	StartDate string                    `json:"startDate"`
	EndDate   string                    `json:"endDate"`
}

// InstanceUsageStatsRequest is entity-service's request body for
// POST /instances/usages/stats/search.
type InstanceUsageStatsRequest struct {
	Filters InstanceStatsFilters `json:"filters"`
}

// InstanceUsageStatsResponse is entity-service's response for
// POST /instances/usages/stats/search.
type InstanceUsageStatsResponse struct {
	Stats     map[string]map[string]int `json:"stats"`
	Total     int                       `json:"total"`
	StartDate string                    `json:"startDate"`
	EndDate   string                    `json:"endDate"`
}
