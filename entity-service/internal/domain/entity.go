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

import (
	"encoding/json"
	"time"
)

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

// UserRole represents a ServiceNow role that can be assigned to a user.
type UserRole string

const (
	UserRoleInternal      UserRole = "internal"
	UserRoleAgent         UserRole = "agent"
	UserRoleAdmin         UserRole = "admin"
	UserRoleCommenter     UserRole = "commenter"
	UserRoleExternal      UserRole = "external"
	UserRoleCustomer      UserRole = "customer"
	UserRoleCustomerAdmin UserRole = "customer_admin"
	UserRolePartner       UserRole = "partner"
	UserRolePartnerAdmin  UserRole = "partner_admin"
)

// UserSortField enumerates the columns by which user search results may be ordered.
type UserSortField string

const (
	UserSortFieldName      UserSortField = "name"
	UserSortFieldCreatedOn UserSortField = "createdOn"
	UserSortFieldUpdatedOn UserSortField = "updatedOn"
)

// UserSortOrder is the direction of a user search sort.
type UserSortOrder string

const (
	UserSortOrderAsc  UserSortOrder = "asc"
	UserSortOrderDesc UserSortOrder = "desc"
)

// Pagination controls which page of results is returned.
// Limit defaults to 10 and is capped at 50 by the service layer.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// SearchUsersFilters holds the optional filter criteria for a user search.
type SearchUsersFilters struct {
	SearchQuery string     `json:"searchQuery"`
	Roles       []UserRole `json:"roles"`
	UserNames   []string   `json:"userNames"`
	Emails      []string   `json:"emails"`
	Active      *bool      `json:"active"`
}

// UserSortBy specifies the sort field and direction for a user search.
type UserSortBy struct {
	Field UserSortField `json:"field"`
	Order UserSortOrder `json:"order"`
}

// SearchUsersRequest is the input for a user search operation.
type SearchUsersRequest struct {
	Pagination Pagination         `json:"pagination"`
	Filters    SearchUsersFilters `json:"filters"`
	SortBy     UserSortBy         `json:"sortBy"`
}

// SearchUsersResponse is the paginated result of a postgres user search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchUsersResponse struct {
	Users   []User `json:"users"`
	Total   int    `json:"total"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	HasMore bool   `json:"hasMore"`
}

// SNUser is the user view returned by the ServiceNow data source.
type SNUser struct {
	ID        string   `json:"id"`
	UserName  string   `json:"userName"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	TimeZone  *string  `json:"timeZone"`
	Active    bool     `json:"active"`
	CreatedOn string   `json:"createdOn"`
	UpdatedOn string   `json:"updatedOn"`
	Roles     []string `json:"roles"`
}

// SearchSNUsersResponse is the paginated result of a ServiceNow user search.
type SearchSNUsersResponse struct {
	Users  []SNUser `json:"users"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

// GetUserMeResponse is the response for GET /users/me from the ServiceNow data source.
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

// PatchUserMeResponse is the response for PATCH /users/me.
type PatchUserMeResponse struct {
	Message string             `json:"message"`
	User    PatchUserMeUpdated `json:"user"`
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

// SNAccount is the account view returned by the ServiceNow data source.
// Timestamp fields are kept as strings to accommodate empty values from ServiceNow.
type SNAccount struct {
	ID                  string  `json:"id"`
	SfID                string  `json:"sfId"`
	Name                string  `json:"name"`
	Tier                string  `json:"tier"`
	Region              *string `json:"region"`
	ActivationDate      string  `json:"activationDate"`
	DeactivationDate    *string `json:"deactivationDate"`
	OwnerID             string  `json:"ownerId"`
	TechnicalOwnerID    *string `json:"technicalOwnerId"`
	AgentEnabled        bool    `json:"agentEnabled"`
	KbReferencesEnabled bool    `json:"kbReferencesEnabled"`
	CreatedOn           string  `json:"createdOn"`
	UpdatedOn           string  `json:"updatedOn"`
}

// SearchSNAccountsResponse is the paginated result of a ServiceNow account search.
type SearchSNAccountsResponse struct {
	Accounts []SNAccount `json:"accounts"`
	Total    int         `json:"total"`
	Limit    int         `json:"limit"`
	Offset   int         `json:"offset"`
	HasMore  bool        `json:"hasMore"`
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

// SNProduct is the product view returned by the ServiceNow data source.
// SN products do not carry timestamp fields; class is a free-form label from ServiceNow.
type SNProduct struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

// SearchSNProductsResponse is the paginated result of a ServiceNow product search.
type SearchSNProductsResponse struct {
	Products []SNProduct `json:"products"`
	Total    int         `json:"total"`
	Limit    int         `json:"limit"`
	Offset   int         `json:"offset"`
	HasMore  bool        `json:"hasMore"`
}

// SupportStatus represents the lifecycle state of a product version.
type SupportStatus string

const (
	SupportStatusAvailable    SupportStatus = "available"
	SupportStatusExtended     SupportStatus = "extended"
	SupportStatusDeprecated   SupportStatus = "deprecated"
	SupportStatusDiscontinued SupportStatus = "discontinued"
)

// SNProductVersion is the ServiceNow-backed product version type.
// Date fields are strings (e.g. "2024-01-15") rather than time.Time to avoid
// parse errors when SN returns empty strings for optional date fields.
type SNProductVersion struct {
	ID                             string  `json:"id"`
	ProductID                      string  `json:"productId"`
	Version                        string  `json:"version"`
	CurrentSupportStatus           string  `json:"currentSupportStatus"`
	ReleaseDate                    string  `json:"releaseDate"`
	SupportEOLDate                 *string `json:"supportEolDate"`
	EarliestPossibleSupportEOLDate *string `json:"earliestPossibleSupportEolDate"`
}

// SearchSNProductVersionsResponse is the paginated SN product version search result.
type SearchSNProductVersionsResponse struct {
	ProductVersions []SNProductVersion `json:"productVersions"`
	Total           int                `json:"total"`
	Limit           int                `json:"limit"`
	Offset          int                `json:"offset"`
	HasMore         bool               `json:"hasMore"`
}

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

// CreateDeploymentRequest is the input for POST /deployments.
// All four fields are required. Type uses a pointer to distinguish an omitted field from a zero value.
type CreateDeploymentRequest struct {
	ProjectID   string          `json:"projectId"`
	Name        string          `json:"name"`
	Type        *DeploymentType `json:"type"`
	Description string          `json:"description"`
}

// CreateDeploymentResponse is the response for POST /deployments.
type CreateDeploymentResponse struct {
	Message    string            `json:"message"`
	Deployment CreatedDeployment `json:"deployment"`
}

// CreatedDeployment carries the key fields of a newly created deployment.
type CreatedDeployment struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// UpdateDeploymentRequest is the input for PATCH /deployments/{id}.
// Either detail fields (Name, Type, Description) or Active (to deactivate) must be provided,
// but not both groups in the same request. Active can only be set to false.
// Description uses a pointer-to-pointer to distinguish three states:
//   - nil outer pointer: field omitted — leave description unchanged
//   - non-nil outer, nil inner (*Description == nil): explicit null — clear the description
//   - non-nil outer, non-nil inner: set description to the given value
type UpdateDeploymentRequest struct {
	ID          string          `json:"-"`
	Name        *string         `json:"name"`
	Type        *DeploymentType `json:"type"`
	Description **string        `json:"description"`
	Active      *bool           `json:"active"`
}

// UpdateDeploymentResponse is the response for PATCH /deployments/{id}.
type UpdateDeploymentResponse struct {
	Message    string            `json:"message"`
	Deployment UpdatedDeployment `json:"deployment"`
}

// UpdatedDeployment carries the fields of a deployment that may change after an update.
type UpdatedDeployment struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
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

// CreateDeployedProductRequest is the input for POST /deployed-products.
// ProjectID, DeploymentID, ProductID, and VersionID are required.
type CreateDeployedProductRequest struct {
	ProjectID    string   `json:"projectId"`
	DeploymentID string   `json:"deploymentId"`
	ProductID    string   `json:"productId"`
	VersionID    string   `json:"versionId"`
	Cores        *int     `json:"cores"`
	TPS          *float64 `json:"tps"`
	Description  *string  `json:"description"`
}

// CreateDeployedProductResponse is the response for POST /deployed-products.
type CreateDeployedProductResponse struct {
	Message         string                 `json:"message"`
	DeployedProduct CreatedDeployedProduct `json:"deployedProduct"`
}

// CreatedDeployedProduct carries the key fields of a newly created deployed product.
type CreatedDeployedProduct struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// UpdateDeployedProductRequest is the input for PATCH /deployed-products/{id}.
// Either detail fields (Cores, TPS, Description) or Active=false must be provided, but not both.
// Description uses json.RawMessage to preserve three states: nil/empty = omit, "null" = clear, `"value"` = set.
// DeploymentID, when provided, scopes the update: the deployed product must belong to that
// deployment or the operation returns a NotFoundError.
type UpdateDeployedProductRequest struct {
	ID           string          `json:"-"`
	DeploymentID *string         `json:"deploymentId,omitempty"`
	Cores        *int            `json:"cores"`
	TPS          *float64        `json:"tps"`
	Description  json.RawMessage `json:"description,omitempty"`
	Active       *bool           `json:"active"`
}

// UpdateDeployedProductResponse is the response for PATCH /deployed-products/{id}.
type UpdateDeployedProductResponse struct {
	Message         string                 `json:"message"`
	DeployedProduct UpdatedDeployedProduct `json:"deployedProduct"`
}

// UpdatedDeployedProduct carries the fields that may change after an update.
type UpdatedDeployedProduct struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
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
	Types            []string         `json:"types"`
	SearchQuery      string           `json:"searchQuery"`
	ProjectIDs       []string         `json:"projectIds"`
	DeploymentIDs    []string         `json:"deploymentIds"`
	States           []CaseState      `json:"states"`
	Severities       []CaseSeverity   `json:"severities"`
	IssueTypes       []CaseIssueType  `json:"issueTypes"`
	EngagementTypes  []EngagementType `json:"engagementTypes"`
	ClosedStartDate  *time.Time       `json:"closedStartDate"`
	ClosedEndDate    *time.Time       `json:"closedEndDate"`
	StartCreatedDate *time.Time       `json:"startCreatedDate"`
	EndCreatedDate   *time.Time       `json:"endCreatedDate"`
	StartUpdatedDate *time.Time       `json:"startUpdatedDate"`
	EndUpdatedDate   *time.Time       `json:"endUpdatedDate"`
	CreatedBy        []string         `json:"createdBy"`
	CreatedByMe      bool             `json:"createdByMe"`
	WorkStates       []CaseWorkState  `json:"workStates"`
	AssignedUserIDs  []string         `json:"assignedUserIds"`
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
	CreatedBy         string        `json:"-"`
	Type              string        `json:"type"`
	ProjectID         string        `json:"projectId"`
	DeploymentID      string        `json:"deploymentId"`
	DeployedProductID string        `json:"deployedProductId"`
	Subject           string        `json:"subject"`
	Description       string        `json:"description"`
	Severity          CaseSeverity  `json:"severity"`
	IssueType         CaseIssueType `json:"issueType"`
	// For service_request type
	CatalogID     string     `json:"catalogId"`
	CatalogItemID string     `json:"catalogItemId"`
	Variables     []Variable `json:"variables"`
	// Optional fields
	RelatedCaseID  string   `json:"relatedCaseId"`
	ConversationID string   `json:"conversationId"`
	WatchList      []string `json:"watchList"`
	// For security_report_analysis type
	Attachments []CaseAttachment `json:"attachments"`
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
	CaseID     string          `json:"-"`
	Filters    *CommentFilters `json:"filters"`
	Pagination Pagination      `json:"pagination"`
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

// CaseGithubIssueReason classifies why a GitHub issue is being filed from a case.
type CaseGithubIssueReason string

const (
	CaseGithubIssueReasonDefault   CaseGithubIssueReason = "default"
	CaseGithubIssueReasonMigration CaseGithubIssueReason = "migration"
	CaseGithubIssueReasonRDTicket  CaseGithubIssueReason = "rd_ticket"
)

// CaseGithubIssueRepoOverride explicitly overrides the product-based repo
// routing lookup for a GitHub issue.
type CaseGithubIssueRepoOverride struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

// CreateCaseGithubIssueRequest is the input for POST /cases/{id}/github-issues.
// CaseID is populated from the URL path parameter and is not part of the JSON body.
// Supported by the ServiceNow data source only.
type CreateCaseGithubIssueRequest struct {
	CaseID         string                       `json:"-"`
	Reason         CaseGithubIssueReason        `json:"reason"`
	Title          string                       `json:"title"`
	Description    string                       `json:"description"`
	RepoOverride   *CaseGithubIssueRepoOverride `json:"repoOverride,omitempty"`
	UpdateLevel    *string                      `json:"updateLevel,omitempty"`
	PublicIssueURL *string                      `json:"publicIssueUrl,omitempty"`
	Regression     bool                         `json:"regression,omitempty"`
	HotFixRequired bool                         `json:"hotFixRequired,omitempty"`
	IssueTypeLabel *string                      `json:"issueTypeLabel,omitempty"`
	PriorityLevel  *string                      `json:"priorityLevel,omitempty"`
}

// CaseGithubIssueDetail holds the result of filing a GitHub issue from a case.
type CaseGithubIssueDetail struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
	Repo   string `json:"repo"`
}

// CreateCaseGithubIssueResponse is the response for POST /cases/{id}/github-issues.
type CreateCaseGithubIssueResponse struct {
	Message string                `json:"message"`
	Issue   CaseGithubIssueDetail `json:"issue"`
}

// Attachment represents a file attachment linked to a reference entity.
type Attachment struct {
	ID            string        `json:"id"`
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	SizeBytes     int           `json:"sizeBytes"`
	Description   *string       `json:"description"`
	CreatedBy     string        `json:"createdBy"`
	CreatedOn     time.Time     `json:"createdOn"`
	DownloadURL   *string       `json:"downloadUrl"`
	PreviewURL    *string       `json:"previewUrl"`
}

// CreateAttachmentRequest is the input for POST /attachments.
type CreateAttachmentRequest struct {
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	File          string        `json:"file"`
	Description   *string       `json:"description"`
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

// ReferenceType identifies the entity type an attachment is associated with.
type ReferenceType string

const (
	ReferenceTypeCase          ReferenceType = "case"
	ReferenceTypeConversation  ReferenceType = "conversation"
	ReferenceTypeChangeRequest ReferenceType = "change_request"
	ReferenceTypeDeployment    ReferenceType = "deployment"
)

// SearchAttachmentsRequest is the input for POST /attachments/search.
type SearchAttachmentsRequest struct {
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Pagination    Pagination    `json:"pagination"`
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

// DeleteAttachmentRequest is the input for DELETE /attachments/{attachmentId}.
type DeleteAttachmentRequest struct {
	AttachmentID string `json:"-"`
}

// DeleteAttachmentResponse is the output for DELETE /cases/{id}/attachments/{attachmentId}.
type DeleteAttachmentResponse struct {
	Message string `json:"message"`
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
	ChangeRequestStateNew              ChangeRequestState = "new"
	ChangeRequestStateAssess           ChangeRequestState = "assess"
	ChangeRequestStateAuthorize        ChangeRequestState = "authorize"
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

// ChangeRequestRisk represents the risk level of a change request.
type ChangeRequestRisk string

const (
	ChangeRequestRiskHigh     ChangeRequestRisk = "high"
	ChangeRequestRiskModerate ChangeRequestRisk = "moderate"
	ChangeRequestRiskLow      ChangeRequestRisk = "low"
)

// ChangeRequestPriority represents the priority of a change request.
type ChangeRequestPriority string

const (
	ChangeRequestPriorityCritical ChangeRequestPriority = "critical"
	ChangeRequestPriorityHigh     ChangeRequestPriority = "high"
	ChangeRequestPriorityModerate ChangeRequestPriority = "moderate"
	ChangeRequestPriorityLow      ChangeRequestPriority = "low"
)

// ChangeRequestCategory represents the category of a change request.
type ChangeRequestCategory string

const (
	ChangeRequestCategoryHardware             ChangeRequestCategory = "hardware"
	ChangeRequestCategorySoftware             ChangeRequestCategory = "software"
	ChangeRequestCategoryService              ChangeRequestCategory = "service"
	ChangeRequestCategorySystemSoftware       ChangeRequestCategory = "system_software"
	ChangeRequestCategoryApplicationsSoftware ChangeRequestCategory = "applications_software"
	ChangeRequestCategoryNetwork              ChangeRequestCategory = "network"
	ChangeRequestCategoryTelecom              ChangeRequestCategory = "telecom"
	ChangeRequestCategoryDocumentation        ChangeRequestCategory = "documentation"
	ChangeRequestCategoryOther                ChangeRequestCategory = "other"
	ChangeRequestCategoryRegularReleaseCloud  ChangeRequestCategory = "regular_release_cloud"
	ChangeRequestCategoryHotfixReleaseCloud   ChangeRequestCategory = "hotfix_release_cloud"
	ChangeRequestCategoryDevOps               ChangeRequestCategory = "devops"
	ChangeRequestCategoryCloudComputing       ChangeRequestCategory = "cloud_computing"
)

// CreateChangeRequestRequest is the input for POST /change-requests.
// Subject is the only required field; all others are optional.
type CreateChangeRequestRequest struct {
	Subject             string                 `json:"subject"`
	Category            *ChangeRequestCategory `json:"category,omitempty"`
	ServiceID           *string                `json:"serviceId,omitempty"`
	ServiceOfferingID   *string                `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string                `json:"configurationItemId,omitempty"`
	Priority            *ChangeRequestPriority `json:"priority,omitempty"`
	Impact              *ChangeRequestImpact   `json:"impact,omitempty"`
	Type                *ChangeRequestType     `json:"type,omitempty"`
	State               *ChangeRequestState    `json:"state,omitempty"`
	GroupID             *string                `json:"groupId,omitempty"`
	AssignedEngineerID  *string                `json:"assignedEngineerId,omitempty"`
	Risk                *ChangeRequestRisk     `json:"risk,omitempty"`
	RequestedByID       *string                `json:"requestedById,omitempty"`
	Description         *string                `json:"description,omitempty"`
	Justification       *string                `json:"justification,omitempty"`
	ImplementationPlan  *string                `json:"implementationPlan,omitempty"`
	RiskImpactAnalysis  *string                `json:"riskImpactAnalysis,omitempty"`
	BackoutPlan         *string                `json:"backoutPlan,omitempty"`
	TestPlan            *string                `json:"testPlan,omitempty"`
	PlannedStartDate    *string                `json:"plannedStartDate,omitempty"`
	PlannedEndDate      *string                `json:"plannedEndDate,omitempty"`
	Comment             *string                `json:"comment,omitempty"`
	WorkNote            *string                `json:"workNote,omitempty"`
}

// CreateChangeRequestResponse is the output for POST /change-requests.
type CreateChangeRequestResponse struct {
	Message       string `json:"message"`
	ChangeRequest struct {
		ID        string `json:"id"`
		Number    string `json:"number"`
		CreatedOn string `json:"createdOn"`
		CreatedBy string `json:"createdBy"`
	} `json:"changeRequest"`
}

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

// PatchChangeRequestRequest is the request body for PATCH /change-requests/{id}.
type PatchChangeRequestRequest struct {
	PlannedStartOn     *string `json:"plannedStartOn,omitempty"`
	IsCustomerApproved *bool   `json:"isCustomerApproved,omitempty"`
	IsCustomerReviewed *bool   `json:"isCustomerReviewed,omitempty"`
}

// PatchChangeRequestResponse is the response for PATCH /change-requests/{id}.
type PatchChangeRequestResponse struct {
	ID        string `json:"id"`
	UpdatedOn string `json:"updatedOn"`
	UpdatedBy string `json:"updatedBy"`
}

// TimeCardState represents the workflow state of a time card.
type TimeCardState string

const (
	TimeCardStatePending   TimeCardState = "pending"
	TimeCardStateSubmitted TimeCardState = "submitted"
	TimeCardStateApproved  TimeCardState = "approved"
	TimeCardStateRejected  TimeCardState = "rejected"
	TimeCardStateProcessed TimeCardState = "processed"
	TimeCardStateRecalled  TimeCardState = "recalled"
)

// SearchTimeCardsFilters holds optional filter criteria for POST /time-cards/search.
type SearchTimeCardsFilters struct {
	ProjectIDs   []string        `json:"projectIds,omitempty"`
	CaseID       *string         `json:"caseId,omitempty"`
	UserID       *string         `json:"userId,omitempty"`
	ApproverID   *string         `json:"approverId,omitempty"`   // eligible approver (SN approver_list)
	ApprovedByID *string         `json:"approvedById,omitempty"` // who actually approved (SN approved_by)
	StartDate    *string         `json:"startDate,omitempty"`
	EndDate      *string         `json:"endDate,omitempty"`
	States       []TimeCardState `json:"states,omitempty"`
}

// SearchTimeCardsRequest is the request body for POST /time-cards/search.
type SearchTimeCardsRequest struct {
	Filters    *SearchTimeCardsFilters `json:"filters,omitempty"`
	Pagination Pagination              `json:"pagination"`
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

// TimeCardView is a single time card in search results.
type TimeCardView struct {
	ID          string           `json:"id"`
	TotalTime   float64          `json:"totalTime"`
	CreatedOn   string           `json:"createdOn"`
	HasBillable bool             `json:"hasBillable"`
	State       *string          `json:"state"`
	User        *TimeCardRef     `json:"user"`
	ApprovedBy  *TimeCardRef     `json:"approvedBy"`
	Project     *TimeCardRef     `json:"project"`
	Case        *TimeCardCaseRef `json:"case"`
}

// SearchTimeCardsResponse is the response for POST /time-cards/search.
type SearchTimeCardsResponse struct {
	TimeCards []TimeCardView `json:"timeCards"`
	Total     int            `json:"total"`
	Limit     int            `json:"limit"`
	Offset    int            `json:"offset"`
}

// CreateTimeCardRequest is the request body for POST /time-cards. The submitter
// is taken from the authenticated session, never from the payload; the card is
// created in the "submitted" state. approverIds populate the eligible-approver list.
type CreateTimeCardRequest struct {
	CaseID                   string   `json:"caseId"`
	ProjectID                string   `json:"projectId"`
	Date                     string   `json:"date"` // YYYY-MM-DD
	ApproverIDs              []string `json:"approverIds"`
	IsBillable               bool     `json:"isBillable"`
	IssueComplexity          *string  `json:"issueComplexity,omitempty"`
	WorkLogComment           *string  `json:"workLogComment,omitempty"`
	TimeAnalyzing            int      `json:"timeAnalyzing"`
	TimeSettingUp            int      `json:"timeSettingUp"`
	TimeReproducingDebugging int      `json:"timeReproducingDebugging"`
	TimeProvidingSolution    int      `json:"timeProvidingSolution"`
	TimePatching             int      `json:"timePatching"`
}

// UpdateTimeCardRequest is the request body for PATCH /time-cards/{id}. ID is
// injected from the path. It carries EITHER editable fields (submitter, while the
// card is submitted) OR a state transition: State="approved", or State="rejected"
// with LeadComment. SN enforces authorization (submitter for edits, an eligible
// approver in approver_list for transitions).
type UpdateTimeCardRequest struct {
	ID                       string         `json:"-"`
	State                    *TimeCardState `json:"state,omitempty"`
	LeadComment              *string        `json:"leadComment,omitempty"`
	Date                     *string        `json:"date,omitempty"`
	ApproverIDs              []string       `json:"approverIds,omitempty"`
	IsBillable               *bool          `json:"isBillable,omitempty"`
	IssueComplexity          *string        `json:"issueComplexity,omitempty"`
	WorkLogComment           *string        `json:"workLogComment,omitempty"`
	TimeAnalyzing            *int           `json:"timeAnalyzing,omitempty"`
	TimeSettingUp            *int           `json:"timeSettingUp,omitempty"`
	TimeReproducingDebugging *int           `json:"timeReproducingDebugging,omitempty"`
	TimeProvidingSolution    *int           `json:"timeProvidingSolution,omitempty"`
	TimePatching             *int           `json:"timePatching,omitempty"`
}

// TimeCardMutationResponse is returned by create and update.
type TimeCardMutationResponse struct {
	Message  string        `json:"message,omitempty"`
	TimeCard *TimeCardView `json:"timeCard,omitempty"`
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

// VulnerabilityPriority is a string enum for the priority (severity) of a product vulnerability.
type VulnerabilityPriority string

const (
	VulnerabilityPriorityInfo     VulnerabilityPriority = "info"
	VulnerabilityPriorityLow      VulnerabilityPriority = "low"
	VulnerabilityPriorityMedium   VulnerabilityPriority = "medium"
	VulnerabilityPriorityHigh     VulnerabilityPriority = "high"
	VulnerabilityPriorityCritical VulnerabilityPriority = "critical"
	VulnerabilityPriorityUnknown  VulnerabilityPriority = "unknown"
)

// SearchProductVulnerabilitiesFilters holds optional filter fields for a vulnerability search.
type SearchProductVulnerabilitiesFilters struct {
	SearchQuery    string                 `json:"searchQuery,omitempty"`
	Priority       *VulnerabilityPriority `json:"priority,omitempty"`
	ProductName    string                 `json:"productName,omitempty"`
	ProductVersion string                 `json:"productVersion,omitempty"`
}

// SearchProductVulnerabilitiesRequest is the input for POST /products/vulnerabilities/search.
type SearchProductVulnerabilitiesRequest struct {
	Filters    *SearchProductVulnerabilitiesFilters `json:"filters,omitempty"`
	Pagination Pagination                           `json:"pagination"`
}

// ProductVulnerabilityView is the representation of a vulnerability returned in search results
// and as the single-item GET response.
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

// SearchProductVulnerabilitiesResponse is the paginated result of a vulnerability search.
type SearchProductVulnerabilitiesResponse struct {
	ProductVulnerabilities []ProductVulnerabilityView `json:"productVulnerabilities"`
	Total                  int                        `json:"total"`
	Limit                  int                        `json:"limit"`
	Offset                 int                        `json:"offset"`
}

// CallRequestStateType is the state of a call request as a domain string enum.
type CallRequestStateType string

const (
	CallRequestStatePendingOnCustomer CallRequestStateType = "pending_on_customer"
	CallRequestStatePendingOnWSO2     CallRequestStateType = "pending_on_wso2"
	CallRequestStateScheduled         CallRequestStateType = "scheduled"
	CallRequestStateCustomerRejected  CallRequestStateType = "customer_rejected"
	CallRequestStateWSO2Rejected      CallRequestStateType = "wso2_rejected"
	CallRequestStateCanceled          CallRequestStateType = "canceled"
	CallRequestStateNotesPending      CallRequestStateType = "notes_pending"
	CallRequestStateConcluded         CallRequestStateType = "concluded"
)

// CallRequestState holds the state of a call request.
// ID uses json.RawMessage because the ServiceNow API returns either an int or a string.
type CallRequestState struct {
	ID    json.RawMessage `json:"id"`
	Label string          `json:"label"`
}

// CallRequestCaseRef is a reference to a case embedded in a call request.
type CallRequestCaseRef struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Number *string `json:"number,omitempty"`
}

// BusinessCriticality represents the criticality level of an IT service.
type BusinessCriticality string

const (
	BusinessCriticalityMostCritical     BusinessCriticality = "most_critical"
	BusinessCriticalitySomewhatCritical BusinessCriticality = "somewhat_critical"
	BusinessCriticalityLessCritical     BusinessCriticality = "less_critical"
	BusinessCriticalityNotCritical      BusinessCriticality = "not_critical"
)

// ServiceClassification represents the classification of an IT service.
type ServiceClassification string

const (
	ServiceClassificationBusinessService             ServiceClassification = "business_service"
	ServiceClassificationTechnologyManagementService ServiceClassification = "technology_management_service"
	ServiceClassificationApplicationService          ServiceClassification = "application_service"
)

// ITService is a single CMDB service entry returned in a search response.
type ITService struct {
	ID                    string                 `json:"id"`
	Name                  *string                `json:"name"`
	Class                 *string                `json:"class"`
	BusinessCriticality   *BusinessCriticality   `json:"businessCriticality"`
	ServiceClassification *ServiceClassification `json:"serviceClassification"`
}

// ConfigurationItem is a single CMDB configuration item returned in a search response.
type ConfigurationItem struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Class       *string `json:"class"`
}

// SearchConfigurationItemsFilters holds optional filter criteria for configuration item searches.
type SearchConfigurationItemsFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

// SearchConfigurationItemsRequest is the input for POST /configuration-items/search.
type SearchConfigurationItemsRequest struct {
	Filters    *SearchConfigurationItemsFilters `json:"filters,omitempty"`
	Pagination Pagination                       `json:"pagination"`
}

// SearchConfigurationItemsResponse is the paginated result of a configuration items search.
type SearchConfigurationItemsResponse struct {
	ConfigurationItems []ConfigurationItem `json:"configurationItems"`
	Total              int                 `json:"total"`
	Offset             int                 `json:"offset"`
	Limit              int                 `json:"limit"`
}

// GroupParentRef is the parent group reference within a group.
type GroupParentRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Group is a single group entry returned in a search response.
type Group struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Active bool            `json:"active"`
	Parent *GroupParentRef `json:"parent"`
}

// SearchGroupsFilters holds optional filter criteria for group searches.
type SearchGroupsFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

// SearchGroupsRequest is the input for POST /groups/search.
type SearchGroupsRequest struct {
	Filters    *SearchGroupsFilters `json:"filters,omitempty"`
	Pagination Pagination           `json:"pagination"`
}

// SearchGroupsResponse is the paginated result of a groups search.
type SearchGroupsResponse struct {
	Groups []Group `json:"groups"`
	Total  int     `json:"total"`
	Offset int     `json:"offset"`
	Limit  int     `json:"limit"`
}

// ServiceOfferingServiceRef is the parent service reference within a service offering.
type ServiceOfferingServiceRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ServiceOffering is a single service offering entry returned in a search response.
type ServiceOffering struct {
	ID      string                     `json:"id"`
	Name    string                     `json:"name"`
	Service *ServiceOfferingServiceRef `json:"service"`
}

// SearchServiceOfferingsFilters holds optional filter criteria for service offering searches.
type SearchServiceOfferingsFilters struct {
	ServiceIDs  []string `json:"serviceIds,omitempty"`
	SearchQuery string   `json:"searchQuery,omitempty"`
}

// SearchServiceOfferingsRequest is the input for POST /service-offerings/search.
type SearchServiceOfferingsRequest struct {
	Filters    *SearchServiceOfferingsFilters `json:"filters,omitempty"`
	Pagination Pagination                     `json:"pagination"`
}

// SearchServiceOfferingsResponse is the paginated result of a service offerings search.
type SearchServiceOfferingsResponse struct {
	ServiceOfferings []ServiceOffering `json:"serviceOfferings"`
	Total            int               `json:"total"`
	Offset           int               `json:"offset"`
	Limit            int               `json:"limit"`
}

// SearchITServicesFilters holds optional filter criteria for IT service searches.
type SearchITServicesFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

// SearchITServicesRequest is the input for POST /services/search.
type SearchITServicesRequest struct {
	Filters    *SearchITServicesFilters `json:"filters,omitempty"`
	Pagination Pagination               `json:"pagination"`
}

// SearchITServicesResponse is the paginated result of a services search.
type SearchITServicesResponse struct {
	Services []ITService `json:"services"`
	Total    int         `json:"total"`
	Offset   int         `json:"offset"`
	Limit    int         `json:"limit"`
}

// CreateCallRequestRequest is the input for POST /call-requests.
type CreateCallRequestRequest struct {
	CaseID          string   `json:"caseId"`
	Reason          string   `json:"reason"`
	UTCTimes        []string `json:"utcTimes"`
	DurationMinutes int      `json:"durationInMinutes"`
}

// CreateCallRequestResponse is the output for POST /call-requests.
type CreateCallRequestResponse struct {
	Message     string `json:"message"`
	CallRequest struct {
		ID        string           `json:"id"`
		CreatedOn string           `json:"createdOn"`
		CreatedBy string           `json:"createdBy"`
		State     CallRequestState `json:"state"`
	} `json:"callRequest"`
}

// SearchCallRequestsFilters holds optional filter criteria for call request searches.
type SearchCallRequestsFilters struct {
	States []CallRequestStateType `json:"states,omitempty"`
}

// SearchCallRequestsRequest is the input for POST /call-requests/search.
type SearchCallRequestsRequest struct {
	CaseID     string                     `json:"caseId"`
	Filters    *SearchCallRequestsFilters `json:"filters,omitempty"`
	Pagination Pagination                 `json:"pagination"`
}

// CallRequestView is a single call request returned in a search response.
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
}

// SearchCallRequestsResponse is the paginated result of a call request search.
type SearchCallRequestsResponse struct {
	CallRequests []CallRequestView `json:"callRequests"`
	Total        int               `json:"total"`
	Offset       int               `json:"offset"`
	Limit        int               `json:"limit"`
}

// UpdateCallRequestRequest is the input for PATCH /call-requests/{id}.
// ID is injected from the URL path parameter and excluded from JSON decoding.
// CaseID is optional; when provided the SN service verifies the call request
// belongs to that case before applying the update (IDOR guard).
type UpdateCallRequestRequest struct {
	ID                 string               `json:"-"`
	CaseID             string               `json:"caseId,omitempty"`
	State              CallRequestStateType `json:"state"`
	CancellationReason *string              `json:"cancellationReason,omitempty"`
	UTCTimes           []string             `json:"utcTimes,omitempty"`
	DurationMinutes    *int                 `json:"durationInMinutes,omitempty"`
}

// UpdateCallRequestResponse is the output for PATCH /call-requests/{id}.
type UpdateCallRequestResponse struct {
	Message     string `json:"message"`
	CallRequest struct {
		ID        string `json:"id"`
		UpdatedOn string `json:"updatedOn"`
		UpdatedBy string `json:"updatedBy"`
	} `json:"callRequest"`
}
