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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// UserService defines the operations available on the user entity.
// Handlers depend on this interface rather than the concrete implementation,
// making it straightforward to substitute a test double in unit tests.
type UserService interface {
	// SearchUsers returns a paginated list of users that match the filters in
	// req. A ValidationError is returned for invalid input (e.g. limit > 50);
	// any other error indicates an infrastructure failure.
	SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchUsersResponse, error)
}

// SNUserService defines the user operations backed by the ServiceNow data source.
type SNUserService interface {
	// SearchUsers returns a paginated list of ServiceNow users that match the
	// filters in req. A ValidationError is returned for invalid input; any other
	// error indicates an infrastructure failure.
	SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchSNUsersResponse, error)
	// GetMe returns the profile of the currently authenticated user from ServiceNow.
	// An UnauthorizedError is returned when x-user-id-token is absent.
	GetMe(ctx context.Context) (domain.GetUserMeResponse, error)
	// PatchMe updates mutable fields on the currently authenticated user in ServiceNow.
	// An UnauthorizedError is returned when x-user-id-token is absent.
	PatchMe(ctx context.Context, req domain.PatchUserMeRequest) (domain.PatchUserMeResponse, error)
}

// AccountService defines the operations available on the account entity.
type AccountService interface {
	// SearchAccounts returns a paginated list of accounts that match the filters
	// in req. A ValidationError is returned for invalid input; any other error
	// indicates an infrastructure failure.
	SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) (domain.SearchAccountsResponse, error)
	// GetAccountByID returns the account with the given UUID. A ValidationError is
	// returned for a malformed UUID; a NotFoundError if no account matches.
	GetAccountByID(ctx context.Context, id string) (domain.Account, error)
}

// SNAccountService defines the account operations backed by the ServiceNow data source.
type SNAccountService interface {
	// SearchAccounts returns a paginated list of ServiceNow accounts matching the
	// filters in req. An UnauthorizedError is returned when x-user-id-token is absent.
	SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) (domain.SearchSNAccountsResponse, error)
	// GetAccountByID returns the ServiceNow account for the given UUID.
	// An UnauthorizedError is returned when x-user-id-token is absent.
	GetAccountByID(ctx context.Context, id string) (domain.SNAccount, error)
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
	// search query. An UnauthorizedError is returned when x-user-id-token is absent.
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
	// CreateCaseComment creates a new comment on the case identified by req.CaseID.
	// A ValidationError is returned for invalid input or constraint violations.
	CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error)
	// SearchCaseComments returns a paginated list of comments for the case identified
	// by req.CaseID. A ValidationError is returned for invalid input.
	SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error)
	// UpdateCase updates the state, severity, watch list, or assignee of a case.
	// A ValidationError is returned for invalid values or malformed UUID; a NotFoundError if no case matches.
	// WatchList and AssigneeEmail are only supported for the ServiceNow data source.
	UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error)
	// CreateCaseAttachment uploads a new attachment for the case identified by req.CaseID.
	// A ValidationError is returned for invalid input.
	CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error)
	// SearchCaseAttachments returns a paginated list of attachments for the case identified
	// by req.CaseID. A ValidationError is returned for invalid input.
	SearchCaseAttachments(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error)
	// GetCaseAttachmentContent returns the raw binary content and its Content-Type
	// for the attachment identified by attachmentID.
	// A NotFoundError is returned if absent.
	GetCaseAttachmentContent(ctx context.Context, attachmentID string) (content []byte, contentType string, err error)
	// DeleteCaseAttachment removes the attachment identified by req.AttachmentID from the case.
	// A NotFoundError is returned if the attachment does not exist.
	DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error)
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
	// UpdateCallRequest updates the state or other fields of a call request.
	// A ValidationError is returned for invalid input; a NotFoundError if no call request matches.
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

	// GetChangeRequest returns the full detail of a single change request by its UUID.
	GetChangeRequest(ctx context.Context, id string) (domain.ChangeRequest, error)

	// PatchChangeRequest updates mutable fields on a change request identified by UUID.
	PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error)
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
}

// ConfigurationItemService defines the operations available on the configuration items entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ConfigurationItemService interface {
	// SearchConfigurationItems returns a paginated list of CMDB configuration items filtered by
	// optional search query. An UnauthorizedError is returned when x-user-id-token is absent.
	SearchConfigurationItems(ctx context.Context, req domain.SearchConfigurationItemsRequest) (domain.SearchConfigurationItemsResponse, error)
}

// GroupService defines the operations available on the groups entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type GroupService interface {
	// SearchGroups returns a paginated list of groups filtered by optional search query.
	// An UnauthorizedError is returned when x-user-id-token is absent.
	SearchGroups(ctx context.Context, req domain.SearchGroupsRequest) (domain.SearchGroupsResponse, error)
}

// ServiceOfferingService defines the operations available on the service offerings entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ServiceOfferingService interface {
	// SearchServiceOfferings returns a paginated list of service offerings filtered by
	// optional service IDs. An UnauthorizedError is returned when x-user-id-token is absent.
	SearchServiceOfferings(ctx context.Context, req domain.SearchServiceOfferingsRequest) (domain.SearchServiceOfferingsResponse, error)
}

// ITServiceService defines the operations available on the CMDB IT services entity.
// All methods require the ServiceNow data source; there is no Postgres fallback.
type ITServiceService interface {
	// SearchITServices returns a paginated list of CMDB services from ServiceNow.
	// An UnauthorizedError is returned when x-user-id-token is absent.
	SearchITServices(ctx context.Context, req domain.SearchITServicesRequest) (domain.SearchITServicesResponse, error)
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
}
