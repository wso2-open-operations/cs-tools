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
)

// User represents a single user entity as stored in the database.
// Phone and Timezone are optional and omitted from JSON when absent.
type User struct {
	ID        string    `json:"id"`
	UserName  string    `json:"userName"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone,omitempty"`
	Timezone  *string   `json:"timezone,omitempty"`
	UserType  UserType  `json:"userType"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
	Region              *string     `json:"region,omitempty"`
	ActivationDate      time.Time   `json:"activationDate"`
	DeactivationDate    *time.Time  `json:"deactivationDate,omitempty"`
	OwnerID             string      `json:"ownerId"`
	TechnicalOwnerID    *string     `json:"technicalOwnerId,omitempty"`
	AgentEnabled        bool        `json:"agentEnabled"`
	KbReferencesEnabled bool        `json:"kbReferencesEnabled"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`
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

// Project represents a customer project linked to an account.
type Project struct {
	ID               string           `json:"id"`
	AccountID        string           `json:"accountId"`
	SfID             string           `json:"sfId"`
	Name             string           `json:"name"`
	ProjectKey       string           `json:"projectKey"`
	SubscriptionType SubscriptionType `json:"subscriptionType"`
	StartDate        time.Time        `json:"startDate"`
	EndDate          time.Time        `json:"endDate"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

// SearchProjectsRequest is the input for a project search operation.
// SearchQuery is matched case-insensitively against name, project_key, and subscription_type.
type SearchProjectsRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
}

// SearchProjectsResponse is the paginated result of a project search.
type SearchProjectsResponse struct {
	Projects []Project `json:"projects"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"hasMore"`
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
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
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
	SupportEOLDate                 *time.Time    `json:"supportEolDate,omitempty"`
	EarliestPossibleSupportEOLDate *time.Time    `json:"earliestPossibleSupportEolDate,omitempty"`
	CreatedAt                      time.Time     `json:"createdAt"`
	UpdatedAt                      time.Time     `json:"updatedAt"`
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
// Description is optional and omitted from JSON when absent.
type Deployment struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	Name        string         `json:"name"`
	Type        DeploymentType `json:"type"`
	Description *string        `json:"description,omitempty"`
	CreatedBy   string         `json:"createdBy"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// SearchDeploymentsRequest is the input for a deployment search operation.
// All filter fields are optional. ProjectIDs scopes results to specific projects;
// DeploymentTypeKeys filters by deployment type; SearchQuery is matched
// case-insensitively against name.
type SearchDeploymentsRequest struct {
	Pagination         Pagination       `json:"pagination"`
	SearchQuery        string           `json:"searchQuery"`
	ProjectIDs         []string         `json:"projectIds"`
	DeploymentTypeKeys []DeploymentType `json:"deploymentTypeKeys"`
}

// SearchDeploymentsResponse is the paginated result of a deployment search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchDeploymentsResponse struct {
	Deployments []Deployment `json:"deployments"`
	Total       int          `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
	HasMore     bool         `json:"hasMore"`
}

// DeployedProduct represents a product (and optional version) associated with a deployment.
// ProductVersionID is optional and omitted from JSON when absent.
type DeployedProduct struct {
	ID               string     `json:"id"`
	DeploymentID     string     `json:"deploymentId"`
	ProductID        string     `json:"productId"`
	ProductVersionID *string    `json:"productVersionId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
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
	DeployedProducts []DeployedProduct `json:"deployedProducts"`
	Total            int               `json:"total"`
	Limit            int               `json:"limit"`
	Offset           int               `json:"offset"`
	HasMore          bool              `json:"hasMore"`
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

// CasePriority represents the urgency level of a support case.
type CasePriority string

const (
	CasePriorityCatastrophic CasePriority = "catastrophic"
	CasePriorityCritical     CasePriority = "critical"
	CasePriorityHigh         CasePriority = "high"
	CasePriorityMedium       CasePriority = "medium"
	CasePriorityLow          CasePriority = "low"
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

// CaseSortField enumerates the columns available for sorting case search results.
type CaseSortField string

const (
	CaseSortFieldCreatedAt CaseSortField = "created_at"
	CaseSortFieldUpdatedAt CaseSortField = "updated_at"
	CaseSortFieldClosedAt  CaseSortField = "closed_at"
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
// Priority and ClosedAt are optional and omitted from JSON when absent.
type Case struct {
	ID                string       `json:"id"`
	Number            string       `json:"number"`
	Wso2ID            string       `json:"wso2Id"`
	CreatedBy         string       `json:"createdBy"`
	ProjectID         string       `json:"projectId"`
	DeploymentID      string       `json:"deploymentId"`
	DeployedProductID string       `json:"deployedProductId"`
	Subject           string       `json:"subject"`
	Description       string       `json:"description"`
	Priority          CasePriority  `json:"priority"`
	IssueType         CaseIssueType `json:"issueType"`
	State             CaseState     `json:"state"`
	CreatedAt         time.Time    `json:"createdAt"`
	UpdatedAt         time.Time    `json:"updatedAt"`
	ClosedAt          *time.Time   `json:"closedAt,omitempty"`
}

// SearchCasesRequest is the input for a case search operation.
// SearchQuery is matched case-insensitively against subject, number, and wso2_id.
// All filter slices are optional. SortBy defaults to created_at.
type SearchCasesRequest struct {
	Pagination          Pagination     `json:"pagination"`
	SearchQuery         string         `json:"searchQuery"`
	ProjectIDs          []string       `json:"projectIds"`
	DeploymentIDs       []string       `json:"deploymentIds"`
	DeployedProductIDs  []string       `json:"deployedProductIds"`
	StateKeys           []CaseState     `json:"stateKeys"`
	PriorityKeys        []CasePriority  `json:"priorityKeys"`
	IssueTypeKeys       []CaseIssueType `json:"issueTypeKeys"`
	SortBy              CaseSort        `json:"sortBy"`
}

// SearchCasesResponse is the paginated result of a case search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchCasesResponse struct {
	Cases   []Case `json:"cases"`
	Total   int    `json:"total"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	HasMore bool   `json:"hasMore"`
}

// CreateCaseRequest is the input for creating a new case.
// id, number, and wso2_id are auto-generated; state defaults to open.
type CreateCaseRequest struct {
	CreatedBy          string        `json:"createdBy"`
	ProjectID          string        `json:"projectId"`
	DeploymentID       string        `json:"deploymentId"`
	DeployedProductID  string        `json:"deployedProductId"`
	Subject            string        `json:"subject"`
	Description        string        `json:"description"`
	Priority           CasePriority  `json:"priority"`
	IssueType          CaseIssueType `json:"issueType"`
}
