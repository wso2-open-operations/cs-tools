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
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CreateCase calls POST /cases on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) CreateCase(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases", body)
}

// SearchCases calls POST /cases/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases/search", body)
}

// GetCase calls GET /cases/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) GetCase(ctx context.Context, caseID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/cases/%s", url.PathEscape(caseID)), nil)
}

// PatchCase calls PATCH /cases/{id} on the entity service to update case state.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) PatchCase(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/cases/%s", url.PathEscape(caseID)), body)
}

// CreateCaseComment calls POST /cases/{id}/comments on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/comments", url.PathEscape(caseID)), body)
}

// SearchCaseComments calls POST /cases/{id}/comments/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchCaseComments(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/comments/search", url.PathEscape(caseID)), body)
}

// GetUserMe calls GET /users/me on the entity service.
// Response is returned as raw JSON.
func (c *Client) GetUserMe(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/users/me", nil)
}

// PatchUserMe calls PATCH /users/me on the entity service.
// Response is returned as raw JSON.
func (c *Client) PatchUserMe(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/users/me", body)
}

// SearchUsers calls POST /users/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *Client) SearchUsers(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/users/search", body)
}

// GetAccount calls GET /accounts/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) GetAccount(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/accounts/%s", url.PathEscape(id)), nil)
}

// SearchAccounts calls POST /accounts/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *Client) SearchAccounts(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/accounts/search", body)
}

// GetProject calls GET /projects/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) GetProject(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%s", url.PathEscape(id)), nil)
}

// SearchProjects calls POST /projects/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *Client) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/projects/search", body)
}

// SearchProducts calls POST /products/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *Client) SearchProducts(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/products/search", body)
}

// SearchProductVersions calls POST /products/{id}/versions/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *Client) SearchProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/products/%s/versions/search", url.PathEscape(productID)), body)
}

// PostDeployment calls POST /deployments on the entity service to create a new deployment.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) PostDeployment(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployments", body)
}

// PatchDeployment calls PATCH /deployments/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) PatchDeployment(ctx context.Context, deploymentID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/deployments/%s", url.PathEscape(deploymentID)), body)
}

// SearchDeployments calls POST /deployments/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *Client) SearchDeployments(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployments/search", body)
}

// SearchDeployedProducts calls POST /deployed-products/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *Client) SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployed-products/search", body)
}

// PostDeployedProduct calls POST /deployed-products on the entity service to create a new deployed product.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) PostDeployedProduct(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployed-products", body)
}

// PatchDeployedProduct calls PATCH /deployed-products/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) PatchDeployedProduct(ctx context.Context, deployedProductID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/deployed-products/%s", url.PathEscape(deployedProductID)), body)
}

// SearchChangeRequests calls POST /change-requests/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchChangeRequests(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/change-requests/search", body)
}

// GetChangeRequest calls GET /change-requests/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) GetChangeRequest(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/change-requests/%s", url.PathEscape(id)), nil)
}

// PatchChangeRequest calls PATCH /change-requests/{id} on the entity service.
// Response is returned as raw JSON.
func (c *Client) PatchChangeRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/change-requests/%s", url.PathEscape(id)), body)
}

// SearchTimeCards calls POST /time-cards/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchTimeCards(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/time-cards/search", body)
}

// CreateTimeCard calls POST /time-cards on the entity service.
// Response is returned as raw JSON.
func (c *Client) CreateTimeCard(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/time-cards", body)
}

// UpdateTimeCard calls PATCH /time-cards/{id} on the entity service. The body may
// carry editable fields, or a state transition ({"state":"approved"} or
// {"state":"rejected","leadComment":"..."}). Response is returned as raw JSON.
func (c *Client) UpdateTimeCard(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/time-cards/%s", url.PathEscape(id)), body)
}

// CreateCaseAttachment calls POST /attachments on the entity service.
// Response is returned as raw JSON.
func (c *Client) CreateCaseAttachment(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments", body)
}

// SearchCaseAttachments calls POST /attachments/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchCaseAttachments(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments/search", body)
}

// GetCaseAttachmentContent calls GET /attachments/{attachmentId}/content
// and returns the raw binary body with its Content-Type.
func (c *Client) GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error) {
	return c.doBinary(ctx, fmt.Sprintf("/attachments/%s/content", url.PathEscape(attachmentID)))
}

// DeleteCaseAttachment calls DELETE /attachments/{attachmentId} on the entity service.
// Response is returned as raw JSON.
func (c *Client) DeleteCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/attachments/%s", url.PathEscape(attachmentID)), nil)
}

// SearchCatalogs calls POST /catalogs/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchCatalogs(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/catalogs/search", body)
}

// GetCatalogItemVariables calls GET /catalogs/{catalogId}/items/{catalogItemId}/variables
// on the entity service. Response is returned as raw JSON.
func (c *Client) GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/catalogs/%s/items/%s/variables", url.PathEscape(catalogID), url.PathEscape(catalogItemID)), nil)
}

// SearchProductVulnerabilities calls POST /products/vulnerabilities/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchProductVulnerabilities(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/products/vulnerabilities/search", body)
}

// GetProductVulnerability calls GET /products/vulnerabilities/{id} on the entity service.
// Response is returned as raw JSON.
func (c *Client) GetProductVulnerability(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/products/vulnerabilities/%s", url.PathEscape(id)), nil)
}

// CreateCallRequest calls POST /call-requests on the entity service.
// Response is returned as raw JSON.
func (c *Client) CreateCallRequest(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/call-requests", body)
}

// SearchCallRequests calls POST /call-requests/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchCallRequests(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/call-requests/search", body)
}

// PatchCallRequest calls PATCH /call-requests/{id} on the entity service.
// Response is returned as raw JSON.
func (c *Client) PatchCallRequest(ctx context.Context, callRequestID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/call-requests/%s", url.PathEscape(callRequestID)), body)
}

// CreateChangeRequest calls POST /change-requests on the entity service.
// Response is returned as raw JSON.
func (c *Client) CreateChangeRequest(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/change-requests", body)
}

// SearchITServices calls POST /services/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchITServices(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/services/search", body)
}

// SearchServiceOfferings calls POST /service-offerings/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchServiceOfferings(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/service-offerings/search", body)
}

// SearchGroups calls POST /groups/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchGroups(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/groups/search", body)
}

// SearchConfigurationItems calls POST /configuration-items/search on the entity service.
// Response is returned as raw JSON.
func (c *Client) SearchConfigurationItems(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/configuration-items/search", body)
}
