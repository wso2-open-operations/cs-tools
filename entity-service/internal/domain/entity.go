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

// Package domain defines the core data types shared across all layers of the
// entity service. No business logic lives here — only plain structs and enums.
package domain

import "time"

// UserType classifies a user's role within the system.
type UserType string

const (
	// UserTypeInternal identifies WSO2 or partner staff users.
	UserTypeInternal UserType = "internal"
	// UserTypeCustomer identifies end-customer users.
	UserTypeCustomer UserType = "customer"
	// UserTypeSystem identifies automated system actors.
	UserTypeSystem UserType = "system"
)

// User represents a single user entity as stored in the database.
// Phone and Timezone are optional and omitted from JSON when absent.
type User struct {
	ID        string    `json:"id"`
	UserName  string    `json:"userName"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone"`
	Timezone  *string   `json:"timezone"`
	UserType  UserType  `json:"userType"`
	CreatedOn time.Time `json:"createdOn"`
	UpdatedOn time.Time `json:"updatedOn"`
}

// Pagination controls which page of results is returned.
// Limit defaults to 20 and is capped at 100 by the service layer.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// SearchUsersRequest is the input for a user search operation.
// SearchQuery is matched case-insensitively against username and email.
type SearchUsersRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
}

// SearchUsersResponse is the paginated result of a user search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchUsersResponse struct {
	Users   []User `json:"users"`
	Total   int    `json:"total"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	HasMore bool   `json:"hasMore"`
}

// AccountTier represents the subscription tier of an account.
type AccountTier string

const (
	// AccountTierBasic is the standard subscription tier.
	AccountTierBasic AccountTier = "basic"
	// AccountTierEnterprise is the premium subscription tier.
	AccountTierEnterprise AccountTier = "enterprise"
)

// Account represents a customer account as stored in the database.
// TechnicalOwnerID, Region, and DeactivationDate are optional.
type Account struct {
	ID                  string      `json:"id"`
	SfID                string      `json:"sfId"`
	Name                string      `json:"name"`
	Tier                AccountTier `json:"tier"`
	Region              *string     `json:"region"`
	ActivationDate      time.Time   `json:"activationDate"`
	DeactivationDate    *time.Time  `json:"deactivationDate"`
	OwnerID             string      `json:"ownerId"`
	TechnicalOwnerID    *string     `json:"technicalOwnerId"`
	AgentEnabled        bool        `json:"agentEnabled"`
	KbReferencesEnabled bool        `json:"kbReferencesEnabled"`
	CreatedOn           time.Time   `json:"createdOn"`
	UpdatedOn           time.Time   `json:"updatedOn"`
}

// SearchAccountsRequest is the input for an account search operation.
// SearchQuery is matched case-insensitively against name and sf_id.
type SearchAccountsRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
}

// SearchAccountsResponse is the paginated result of an account search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchAccountsResponse struct {
	Accounts []Account `json:"accounts"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"hasMore"`
}

// SubscriptionType classifies the subscription type of a project.
type SubscriptionType string

const (
	SubscriptionTypeDevelopmentSupport       SubscriptionType = "development_support"
	SubscriptionTypeManagedCloudSubscription SubscriptionType = "managed_cloud_subscription"
	SubscriptionTypeEvaluationSubscription   SubscriptionType = "evaluation_subscription"
	SubscriptionTypeSubscription             SubscriptionType = "subscription"
	SubscriptionTypeCloudEvaluationSupport   SubscriptionType = "cloud_evaluation_support"
	SubscriptionTypeInternal                 SubscriptionType = "internal"
	SubscriptionTypePlatformerSubscription   SubscriptionType = "platformer_subscription"
	SubscriptionTypeCloudSupport             SubscriptionType = "cloud_support"
	SubscriptionTypeProfessionalServices     SubscriptionType = "professional_services"
)

// ClosureStatus represents the closure/access state of a project.
type ClosureStatus string

const (
	ClosureStatusNotify     ClosureStatus = "notify"
	ClosureStatusClosed     ClosureStatus = "closed"
	ClosureStatusOpen       ClosureStatus = "open"
	ClosureStatusReadOnly   ClosureStatus = "read_only"
	ClosureStatusRestricted ClosureStatus = "restricted"
	ClosureStatusSuspended  ClosureStatus = "suspended"
)

// Project represents a customer project linked to an account.
type Project struct {
	ID               string           `json:"id"`
	AccountID        string           `json:"accountId"`
	SfID             string           `json:"sfId"`
	Name             string           `json:"name"`
	Key              string           `json:"key"`
	SubscriptionType SubscriptionType `json:"subscriptionType"`
	ClosureStatus    *ClosureStatus   `json:"closureStatus"`
	StartDate        time.Time        `json:"startDate"`
	EndDate          time.Time        `json:"endDate"`
	CreatedOn        time.Time        `json:"createdOn"`
	UpdatedOn        time.Time        `json:"updatedOn"`
}

// ProjectAccountRef is the embedded account summary returned in project detail responses.
type ProjectAccountRef struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	ActivationDate      *time.Time `json:"activationDate"`
	Tier                string     `json:"tier"`
	Region              *string    `json:"region"`
	AgentEnabled        bool       `json:"agentEnabled"`
	KbReferencesEnabled bool       `json:"kbReferencesEnabled"`
}

// ProjectDetailsView is the enriched response shape for GET /projects/{id}.
// It embeds the linked account and uses createdOn/updatedOn for consistency
// with the ProjectView search result.
type ProjectDetailsView struct {
	ID               string            `json:"id"`
	Account          ProjectAccountRef `json:"account"`
	SfID             string            `json:"sfId"`
	Name             string            `json:"name"`
	Key              string            `json:"key"`
	SubscriptionType SubscriptionType  `json:"subscriptionType"`
	StartDate        time.Time         `json:"startDate"`
	EndDate          time.Time         `json:"endDate"`
	CreatedOn        time.Time         `json:"createdOn"`
	UpdatedOn        time.Time         `json:"updatedOn"`
}

// SearchProjectsRequest is the input for a project search operation.
// SearchQuery is matched case-insensitively against name, key, and subscription_type.
type SearchProjectsRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
}

// ProjectView is the unified search result shape returned for all data sources.
// Both Postgres and ServiceNow responses are mapped to this type so callers
// receive the same fields regardless of which backend is active.
type ProjectView struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Key              string           `json:"key"`
	SubscriptionType SubscriptionType `json:"subscriptionType"`
	CreatedOn        time.Time        `json:"createdOn"`
}

// SearchProjectsResponse is the paginated result of a project search.
type SearchProjectsResponse struct {
	Projects []ProjectView `json:"projects"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	HasMore  bool          `json:"hasMore"`
}

// ProductClass classifies a product as either a standalone software or a managed service.
type ProductClass string

const (
	// ProductClassSoftware identifies installable or deployable software products.
	ProductClassSoftware ProductClass = "software"
	// ProductClassService identifies managed-service or SaaS products.
	ProductClassService ProductClass = "service"
)

// Product represents a WSO2 product entry as stored in the database.
type Product struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Class     ProductClass `json:"class"`
	CreatedOn time.Time    `json:"createdOn"`
	UpdatedOn time.Time    `json:"updatedOn"`
}

// SearchProductsRequest is the input for a product search operation.
// SearchQuery is matched case-insensitively against name.
type SearchProductsRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
}

// SearchProductsResponse is the paginated result of a product search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchProductsResponse struct {
	Products []Product `json:"products"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"hasMore"`
}

// SupportStatus represents the lifecycle state of a product version.
type SupportStatus string

const (
	SupportStatusAvailable    SupportStatus = "available"
	SupportStatusExtended     SupportStatus = "extended"
	SupportStatusDeprecated   SupportStatus = "deprecated"
	SupportStatusDiscontinued SupportStatus = "discontinued"
)

// ProductVersion represents a versioned release of a software product.
// SupportEOLDate and EarliestPossibleSupportEOLDate are optional.
type ProductVersion struct {
	ID                             string        `json:"id"`
	ProductID                      string        `json:"productId"`
	Version                        string        `json:"version"`
	CurrentSupportStatus           SupportStatus `json:"currentSupportStatus"`
	ReleaseDate                    time.Time     `json:"releaseDate"`
	SupportEOLDate                 *time.Time    `json:"supportEolDate"`
	EarliestPossibleSupportEOLDate *time.Time    `json:"earliestPossibleSupportEolDate"`
	CreatedOn                      time.Time     `json:"createdOn"`
	UpdatedOn                      time.Time     `json:"updatedOn"`
}

// SearchProductVersionsRequest is the input for a product version search operation.
// ProductID is populated from the URL path parameter and is not part of the JSON body.
// SearchQuery is optional and matched case-insensitively against the version string.
type SearchProductVersionsRequest struct {
	Pagination  Pagination `json:"pagination"`
	ProductID   string     `json:"-"`
	SearchQuery string     `json:"searchQuery"`
}

// SearchProductVersionsResponse is the paginated result of a product version search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchProductVersionsResponse struct {
	ProductVersions []ProductVersion `json:"productVersions"`
	Total           int              `json:"total"`
	Limit           int              `json:"limit"`
	Offset          int              `json:"offset"`
	HasMore         bool             `json:"hasMore"`
}

// DeploymentType classifies the environment role of a deployment.
type DeploymentType string

const (
	DeploymentTypePrimaryProduction DeploymentType = "primary_production"
	DeploymentTypeStaging           DeploymentType = "staging"
	DeploymentTypeQA                DeploymentType = "qa"
	DeploymentTypeStress            DeploymentType = "stress"
	DeploymentTypeUAT               DeploymentType = "uat"
	DeploymentTypeDevelopment       DeploymentType = "development"
)

// Deployment represents a project deployment environment as stored in the database.
type Deployment struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	Name        string         `json:"name"`
	Type        DeploymentType `json:"type"`
	Description *string        `json:"description"`
	CreatedBy   string         `json:"createdBy"`
	CreatedOn   time.Time      `json:"createdOn"`
	UpdatedOn   time.Time      `json:"updatedOn"`
}

// DeploymentView is the enriched search result for a deployment. It embeds
// project and createdBy as named refs and uses createdOn/updatedOn naming.
type DeploymentView struct {
	ID          string         `json:"id"`
	Number      string         `json:"number"`
	Name        string         `json:"name"`
	Type        DeploymentType `json:"type"`
	Description *string        `json:"description"`
	CreatedBy   *EntityRef     `json:"createdBy"`
	Project     EntityRef      `json:"project"`
	CreatedOn   time.Time      `json:"createdOn"`
	UpdatedOn   time.Time      `json:"updatedOn"`
}

// SearchDeploymentsRequest is the input for a deployment search operation.
// All filter fields are optional. ProjectIDs scopes results to specific projects;
// DeploymentTypes filters by deployment type; SearchQuery is matched
// case-insensitively against name.
type SearchDeploymentsRequest struct {
	Pagination      Pagination       `json:"pagination"`
	SearchQuery     string           `json:"searchQuery"`
	ProjectIDs      []string         `json:"projectIds"`
	DeploymentTypes []DeploymentType `json:"deploymentTypes"`
}

// SearchDeploymentsResponse is the paginated result of a deployment search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchDeploymentsResponse struct {
	Deployments []DeploymentView `json:"deployments"`
	Total       int              `json:"total"`
	Limit       int              `json:"limit"`
	Offset      int              `json:"offset"`
	HasMore     bool             `json:"hasMore"`
}

// DeployedProduct represents a product (and optional version) associated with a deployment.
type DeployedProduct struct {
	ID               string    `json:"id"`
	DeploymentID     string    `json:"deploymentId"`
	ProductID        string    `json:"productId"`
	ProductVersionID *string   `json:"productVersionId"`
	CreatedOn        time.Time `json:"createdOn"`
	UpdatedOn        time.Time `json:"updatedOn"`
}

// DeployedProductVersionRef is the version sub-object in a DeployedProductView.
type DeployedProductVersionRef struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	ReleasedDate   *time.Time `json:"releasedDate"`
	SupportEoLDate *time.Time `json:"supportEoLDate"`
}

// DeployedProductView is the enriched search result for a deployed product.
// It embeds deployment, product, and version as named refs and uses createdOn/updatedOn naming.
// Cores, TPS, and Category are SN-only fields; they are always null for the Postgres path.
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

// SearchDeployedProductsRequest is the input for a deployed-product search operation.
// DeploymentIDs scopes results to the given deployments; it is the only filter besides pagination.
type SearchDeployedProductsRequest struct {
	Pagination    Pagination `json:"pagination"`
	DeploymentIDs []string   `json:"deploymentIds"`
}

// SearchDeployedProductsResponse is the paginated result of a deployed-product search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchDeployedProductsResponse struct {
	DeployedProducts []DeployedProductView `json:"deployedProducts"`
	Total            int                   `json:"total"`
	Limit            int                   `json:"limit"`
	Offset           int                   `json:"offset"`
	HasMore          bool                  `json:"hasMore"`
}

// CaseIssueType classifies the nature of a support case.
type CaseIssueType string

const (
	CaseIssueTypeError                  CaseIssueType = "error"
	CaseIssueTypePartialOutage          CaseIssueType = "partial_outage"
	CaseIssueTypePerformanceDegradation CaseIssueType = "performance_degradation"
	CaseIssueTypeQuestion               CaseIssueType = "question"
	CaseIssueTypeSecurityOrCompliance   CaseIssueType = "security_or_compliance"
	CaseIssueTypeTotalOutage            CaseIssueType = "total_outage"
)

// CaseSeverity represents the urgency level of a support case.
type CaseSeverity string

const (
	CaseSeverityCatastrophic CaseSeverity = "catastrophic"
	CaseSeverityCritical     CaseSeverity = "critical"
	CaseSeverityHigh         CaseSeverity = "high"
	CaseSeverityMedium       CaseSeverity = "medium"
	CaseSeverityLow          CaseSeverity = "low"
)

// CaseState represents the current workflow state of a support case.
type CaseState string

const (
	CaseStateOpen             CaseState = "open"
	CaseStateWorkInProgress   CaseState = "work_in_progress"
	CaseStateWaitingOnWSO2    CaseState = "waiting_on_wso2"
	CaseStateAwaitingInfo     CaseState = "awaiting_info"
	CaseStateReopened         CaseState = "reopened"
	CaseStateSolutionProposed CaseState = "solution_proposed"
	CaseStateClosed           CaseState = "closed"
)

// CaseWorkState represents the work sub-state of a work_in_progress case.
type CaseWorkState string

const (
	CaseWorkStateOngoing CaseWorkState = "ongoing"
	CaseWorkStatePaused  CaseWorkState = "paused"
)

// EngagementType classifies the type of an engagement case.
type EngagementType string

const (
	EngagementTypeMigration             EngagementType = "migration"
	EngagementTypeConsultancy           EngagementType = "consultancy"
	EngagementTypeNewFeatureImprovement EngagementType = "new_feature_improvement"
	EngagementTypeFollowUp              EngagementType = "follow_up"
	EngagementTypeOnboarding            EngagementType = "onboarding"
)

// CaseSortField enumerates the columns available for sorting case search results.
type CaseSortField string

const (
	CaseSortFieldCreatedOn CaseSortField = "createdOn"
	CaseSortFieldUpdatedOn CaseSortField = "updatedOn"
	CaseSortFieldSeverity  CaseSortField = "severity"
	CaseSortFieldState     CaseSortField = "state"
)

// CaseSortOrder controls the sort direction.
type CaseSortOrder string

const (
	CaseSortOrderAsc  CaseSortOrder = "asc"
	CaseSortOrderDesc CaseSortOrder = "desc"
)

// CaseSort specifies the sort field and direction for case search results.
type CaseSort struct {
	Field CaseSortField `json:"field"`
	Order CaseSortOrder `json:"order"`
}

// Case represents a customer support case as stored in the database.
// ClosedOn and WorkState are the only nullable fields; all others are required.
// Used as the response for write operations (create/update).
type Case struct {
	ID                string         `json:"id"`
	Number            string         `json:"number"`
	InternalID        string         `json:"internalId"`
	CreatedBy         string         `json:"createdBy"`
	ProjectID         string         `json:"projectId"`
	DeploymentID      string         `json:"deploymentId"`
	DeployedProductID string         `json:"deployedProductId"`
	Subject           string         `json:"subject"`
	Description       string         `json:"description"`
	Severity          CaseSeverity   `json:"severity"`
	IssueType         CaseIssueType  `json:"issueType"`
	State             CaseState      `json:"state"`
	WorkState         *CaseWorkState `json:"workState"`
	CreatedOn         time.Time      `json:"createdOn"`
	UpdatedOn         time.Time      `json:"updatedOn"`
	ClosedOn          *time.Time     `json:"closedOn"`
}

// UserRef is a reference to a user with key display fields.
type UserRef struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	UserID string `json:"userId,omitempty"`
	Email  string `json:"email"`
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

// AccountRef is a compact reference to an account.
type AccountRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// UserIDEmailRef is a compact user reference carrying only id and email.
type UserIDEmailRef struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// EntityRef is a compact reference to a named entity (project or deployment).
type EntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DeployedProductRef is a compact reference to a deployed product with a
// computed display name combining the product name and version.
type DeployedProductRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// CaseView is the enriched read representation of a case.
type CaseView struct {
	ID                     string               `json:"id"`
	Number                 string               `json:"number"`
	InternalID             string               `json:"internalId"`
	Subject                string               `json:"subject"`
	Description            string               `json:"description"`
	Severity               CaseSeverity         `json:"severity"`
	IssueType              CaseIssueType        `json:"issueType"`
	State                  CaseState            `json:"state"`
	WorkState              *CaseWorkState       `json:"workState"`
	Type                   *string              `json:"type"`
	EngagementType         *string              `json:"engagementType"`
	CreatedOn              time.Time            `json:"createdOn"`
	UpdatedOn              time.Time            `json:"updatedOn"`
	ClosedOn               *time.Time           `json:"closedOn"`
	CreatedByDetails       UserRef              `json:"createdBy"`
	ProjectDetails         EntityRef            `json:"project"`
	DeploymentDetails      *EntityRef           `json:"deployment"`
	DeployedProductDetails *DeployedProductRef  `json:"deployedProduct"`
	ProductDetails         *EntityRef           `json:"product"`
	Catalog                *EntityRef           `json:"catalog"`
	CatalogItem            *EntityRef           `json:"catalogItem"`
	AssignedTeam           *EntityRef           `json:"assignedTeam"`
	Conversation           *EntityRef           `json:"conversation"`
	AssignedEngineer       *AssignedEngineerRef `json:"assignedEngineer"`
	ParentCase             *CaseNumberRef       `json:"parentCase"`
	RelatedCase            *CaseNumberRef       `json:"relatedCase"`
	AccountDetails         *AccountRef          `json:"account"`
}

// SearchCasesFilters holds all optional filter criteria for a case search.
type SearchCasesFilters struct {
	Types           []string         `json:"types"`
	SearchQuery     string           `json:"searchQuery"`
	ProjectIDs      []string         `json:"projectIds"`
	DeploymentIDs   []string         `json:"deploymentIds"`
	States          []CaseState      `json:"states"`
	Severities      []CaseSeverity   `json:"severities"`
	IssueTypes      []CaseIssueType  `json:"issueTypes"`
	EngagementTypes []EngagementType `json:"engagementTypes"`
	ClosedStartDate    *time.Time `json:"closedStartDate"`
	ClosedEndDate      *time.Time `json:"closedEndDate"`
	StartCreatedDate   *time.Time `json:"startCreatedDate"`
	EndCreatedDate     *time.Time `json:"endCreatedDate"`
	StartUpdatedDate   *time.Time `json:"startUpdatedDate"`
	EndUpdatedDate     *time.Time `json:"endUpdatedDate"`
	CreatedBy          []string   `json:"createdBy"`
	CreatedByMe        bool       `json:"createdByMe"`
}

// SearchCasesRequest is the input for a case search operation.
// All filter fields are optional and nested under Filters. SortBy defaults to createdOn desc.
type SearchCasesRequest struct {
	Filters    SearchCasesFilters `json:"filters"`
	SortBy     CaseSort           `json:"sortBy"`
	Pagination Pagination         `json:"pagination"`
}

// SearchCaseView is the unified case representation returned in search results.
// Fields absent for a given data source are nil.
type SearchCaseView struct {
	ID               string               `json:"id"`
	InternalID       string               `json:"internalId"`
	Number           string               `json:"number"`
	CreatedOn        string               `json:"createdOn"`
	CreatedBy        string               `json:"createdBy"`
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

// SearchCasesResponse is the paginated result of a case search.
type SearchCasesResponse struct {
	Cases  []SearchCaseView `json:"cases"`
	Total  int              `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
}



// UpdateCaseRequest is the input for PATCH /cases/{id}.
// Exactly one of State, Severity, WorkState, WatchList, or AssigneeEmail must be provided.
// WatchList and AssigneeEmail are only supported for the ServiceNow data source.
type UpdateCaseRequest struct {
	ID            string         `json:"-"`
	State         *CaseState     `json:"state"`
	Severity      *CaseSeverity  `json:"severity"`
	WorkState     *CaseWorkState `json:"workState"`
	WatchList     []string       `json:"watchList"`
	AssigneeEmail *string        `json:"assigneeEmail"`
}

// UpdateCaseResponse is the response for PATCH /cases/{id}.
type UpdateCaseResponse struct {
	Message string      `json:"message"`
	Case    UpdatedCase `json:"case"`
}

// UpdatedCase carries the fields of a case that may change after an update.
type UpdatedCase struct {
	ID         string               `json:"id"`
	UpdatedOn  time.Time            `json:"updatedOn"`
	UpdatedBy  string               `json:"updatedBy,omitempty"`
	State      CaseState            `json:"state,omitempty"`
	Severity   CaseSeverity         `json:"severity,omitempty"`
	WorkState  *CaseWorkState       `json:"workState"`
	WatchList  []WatchListUser      `json:"watchList,omitempty"`
	AssignedTo *AssignedEngineerRef `json:"assignedTo,omitempty"`
}

// WatchListUser is a compact user reference within the watch list.
type WatchListUser struct {
	ID       string `json:"id"`
	UserName string `json:"userName"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
}

// CreateCaseResponse is the unified response for POST /cases across all data sources.
type CreateCaseResponse struct {
	Message string            `json:"message"`
	Case    CreateCaseDetails `json:"case"`
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

// Variable is a key-value pair used in service request case creation.
type Variable struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// CaseAttachment is a file attachment for security report analysis case creation.
// File must be a base64 data URI (e.g. data:application/pdf;base64,...).
type CaseAttachment struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// CreateCaseRequest is the input for creating a new case.
// id, number, and internal_id are auto-generated; state defaults to open.
// CreatedBy is not accepted from the request body and will be wired from auth context later.
// For type "service_request": catalogId, catalogItemId, and variables are required.
// For type "security_report_analysis": subject, description, and at least one attachment are required.
type CreateCaseRequest struct {
	CreatedBy         string           `json:"-"`
	Type              string           `json:"type"`
	ProjectID         string           `json:"projectId"`
	DeploymentID      string           `json:"deploymentId"`
	DeployedProductID string           `json:"deployedProductId"`
	Subject           string           `json:"subject"`
	Description       string           `json:"description"`
	Severity          CaseSeverity     `json:"severity"`
	IssueType         CaseIssueType    `json:"issueType"`
	// For service_request type
	CatalogID         string           `json:"catalogId"`
	CatalogItemID     string           `json:"catalogItemId"`
	Variables         []Variable       `json:"variables"`
	// Optional fields
	RelatedCaseID     string           `json:"relatedCaseId"`
	ConversationID    string           `json:"conversationId"`
	WatchList         []string         `json:"watchList"`
	// For security_report_analysis type
	Attachments       []CaseAttachment `json:"attachments"`
}

// CommentType classifies the type of a case comment.
type CommentType string

const (
	CommentTypeWorkNote CommentType = "work_note"
	CommentTypeComment  CommentType = "comment"
	CommentTypeActivity CommentType = "activity"
)

// CommentUserRef holds user details embedded in a case comment response.
type CommentUserRef struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	FullName  string `json:"fullName"`
}

// CaseComment represents a comment on a support case.
type CaseComment struct {
	ID        string         `json:"id"`
	CaseID    string         `json:"caseId"`
	Type      CommentType    `json:"type"`
	Content   string         `json:"content"`
	CreatedBy CommentUserRef `json:"createdBy"`
	CreatedOn time.Time      `json:"createdOn"`
}

// CreateCaseCommentRequest is the input for creating a new case comment.
// CaseID is populated from the URL path parameter and is not part of the JSON body.
type CreateCaseCommentRequest struct {
	CaseID    string      `json:"-"`
	CreatedBy string      `json:"-"`
	Type      CommentType `json:"type"`
	Content   string      `json:"content"`
}

// CreateCaseCommentResponse is the response for creating a new case comment.
type CreateCaseCommentResponse struct {
	Message string            `json:"message"`
	Comment CaseCommentDetail `json:"comment"`
}

// CaseCommentDetail holds the core fields returned after creating a comment.
type CaseCommentDetail struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// SearchCaseCommentsRequest is the input for listing comments on a case.
// CaseID is populated from the URL path parameter and is not part of the JSON body.
type SearchCaseCommentsRequest struct {
	CaseID     string      `json:"-"`
	Filters    *CommentFilters `json:"filters"`
	Pagination Pagination  `json:"pagination"`
}

// CommentFilters holds optional filter criteria for searching case comments.
type CommentFilters struct {
	Type *CommentType `json:"type"`
}

// SearchCaseCommentsResponse is the paginated result of a case comment search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchCaseCommentsResponse struct {
	Comments []CaseComment `json:"comments"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	HasMore  bool          `json:"hasMore"`
}

// Attachment represents a file attachment linked to a case.
type Attachment struct {
	ID          string    `json:"id"`
	CaseID      string    `json:"caseId"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	SizeBytes   int       `json:"sizeBytes"`
	Description *string   `json:"description"`
	CreatedBy   string    `json:"createdBy"`
	CreatedOn   time.Time `json:"createdOn"`
	DownloadURL *string   `json:"downloadUrl"`
	PreviewURL  *string   `json:"previewUrl"`
}

// CreateAttachmentRequest is the input for POST /cases/{id}/attachments.
// CaseID is populated from the URL path parameter and is not part of the JSON body.
type CreateAttachmentRequest struct {
	CaseID      string  `json:"-"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	File        string  `json:"file"`
	Description *string `json:"description"`
}

// AttachmentDetail holds the core fields returned after creating an attachment.
type AttachmentDetail struct {
	ID          string    `json:"id"`
	SizeBytes   int       `json:"sizeBytes"`
	CreatedOn   time.Time `json:"createdOn"`
	CreatedBy   string    `json:"createdBy"`
	DownloadURL string    `json:"downloadUrl"`
}

// CreateAttachmentResponse is the response for POST /cases/{id}/attachments.
type CreateAttachmentResponse struct {
	Message    string           `json:"message"`
	Attachment AttachmentDetail `json:"attachment"`
}

// SearchAttachmentsRequest is the input for POST /cases/{id}/attachments/search.
// CaseID is populated from the URL path parameter and is not part of the JSON body.
type SearchAttachmentsRequest struct {
	CaseID     string     `json:"-"`
	Pagination Pagination `json:"pagination"`
}

// SearchAttachmentsResponse is the paginated result of an attachment search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchAttachmentsResponse struct {
	Attachments []Attachment `json:"attachments"`
	Total       int          `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
	HasMore     bool         `json:"hasMore"`
}

// ChangeRequestType classifies the change request type.
type ChangeRequestType string

const (
	ChangeRequestTypeStandard           ChangeRequestType = "standard"
	ChangeRequestTypeNormal             ChangeRequestType = "normal"
	ChangeRequestTypeEmergency          ChangeRequestType = "emergency"
	ChangeRequestTypeModel              ChangeRequestType = "model"
	ChangeRequestTypeSiteReliabilityOps ChangeRequestType = "site_reliability_ops"
	ChangeRequestTypeAzure              ChangeRequestType = "azure"
)

// ChangeRequestState represents the current workflow state of a change request.
type ChangeRequestState string

const (
	ChangeRequestStateCustomerApproval ChangeRequestState = "customer_approval"
	ChangeRequestStateScheduled        ChangeRequestState = "scheduled"
	ChangeRequestStateImplement        ChangeRequestState = "implement"
	ChangeRequestStateReview           ChangeRequestState = "review"
	ChangeRequestStateCustomerReview   ChangeRequestState = "customer_review"
	ChangeRequestStateRollback         ChangeRequestState = "rollback"
	ChangeRequestStateClosed           ChangeRequestState = "closed"
	ChangeRequestStateCanceled         ChangeRequestState = "canceled"
)

// ChangeRequestImpact represents the impact level of a change request.
type ChangeRequestImpact string

const (
	ChangeRequestImpactHigh   ChangeRequestImpact = "high"
	ChangeRequestImpactMedium ChangeRequestImpact = "medium"
	ChangeRequestImpactLow    ChangeRequestImpact = "low"
)

// ChangeRequestSortField enumerates the columns available for sorting change request search results.
type ChangeRequestSortField string

const (
	ChangeRequestSortFieldCreatedOn ChangeRequestSortField = "createdOn"
	ChangeRequestSortFieldUpdatedOn ChangeRequestSortField = "updatedOn"
)

// ChangeRequestSortOrder controls the sort direction.
type ChangeRequestSortOrder string

const (
	ChangeRequestSortOrderAsc  ChangeRequestSortOrder = "asc"
	ChangeRequestSortOrderDesc ChangeRequestSortOrder = "desc"
)

// ChangeRequestSort specifies the sort field and direction for change request search results.
type ChangeRequestSort struct {
	Field ChangeRequestSortField `json:"field"`
	Order ChangeRequestSortOrder `json:"order"`
}

// SearchChangeRequestsFilters holds all optional filter criteria for a change request search.
type SearchChangeRequestsFilters struct {
	ProjectIDs      []string              `json:"projectIds"`
	SearchQuery     string                `json:"searchQuery"`
	States          []ChangeRequestState  `json:"states"`
	Impacts         []ChangeRequestImpact `json:"impacts"`
	ClosedStartDate *time.Time            `json:"closedStartDate"`
	ClosedEndDate   *time.Time            `json:"closedEndDate"`
}

// SearchChangeRequestsRequest is the input for a change request search operation.
type SearchChangeRequestsRequest struct {
	Filters    SearchChangeRequestsFilters `json:"filters"`
	SortBy     ChangeRequestSort           `json:"sortBy"`
	Pagination Pagination                  `json:"pagination"`
}

// SearchChangeRequestView is the unified change request representation returned in search results.
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

// SearchChangeRequestsResponse is the paginated result of a change request search.
type SearchChangeRequestsResponse struct {
	ChangeRequests []SearchChangeRequestView `json:"changeRequests"`
	Total          int                       `json:"total"`
	Offset         int                       `json:"offset"`
	Limit          int                       `json:"limit"`
}

// CatalogItem is a reference to a catalog item within a catalog.
type CatalogItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Catalog represents a service catalog containing one or more catalog items.
type Catalog struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	CatalogItems []CatalogItem `json:"catalogItems"`
}

// SearchCatalogsRequest is the input for POST /catalogs/search.
// DeployedProductID scopes the search to catalogs available for that deployed product.
type SearchCatalogsRequest struct {
	DeployedProductID string     `json:"deployedProductId"`
	Pagination        Pagination `json:"pagination"`
}

// SearchCatalogsResponse is the paginated result of a catalog search.
type SearchCatalogsResponse struct {
	Catalogs []Catalog `json:"catalogs"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
}

// CatalogItemVariable describes a single variable (form field) on a catalog item.
type CatalogItemVariable struct {
	ID           string `json:"id"`
	QuestionText string `json:"questionText"`
	Order        int    `json:"order"`
	Type         string `json:"type"`
}

// GetCatalogItemVariablesResponse is the response for
// GET /catalogs/{catalogId}/items/{catalogItemId}/variables.
type GetCatalogItemVariablesResponse struct {
	Variables []CatalogItemVariable `json:"variables"`
}

// ChangeRequest is the full change request detail returned by GET /change-requests/{id}.
// It extends SearchChangeRequestView with additional fields.
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
}
