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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package salesentity

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// The reads and the one write the ACP reopen check needs
// (internal/service/acp_reopen.go): the project's linked opportunities, an
// opportunity's links and invoices, and the project's State__c.

const (
	// projectPath is the single-record project resource: PATCH /projects/{id}.
	projectPath = "/projects/"
	// childSearchPageSize and childSearchMaxPages bound a paged child search:
	// 10 pages of 200 rows is far more than any project or opportunity has,
	// and a search that is still returning full pages after that is stopped
	// rather than followed forever.
	childSearchPageSize = 200
	childSearchMaxPages = 10
)

// ProjectStatusOpen is the Salesforce project status (State__c) of a project
// that is not suspended. sales-entity-service accepts exactly "Open" and
// "Suspended".
const ProjectStatusOpen = "Open"

// childSearchRequest is a paged search by one parent id. Exactly one of the
// two ids is set; sales-entity-service ANDs the filters it is given.
type childSearchRequest struct {
	ProjectID     string `json:"projectId,omitempty"`
	OpportunityID string `json:"opportunityId,omitempty"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}

// LinkedOpportunitiesByProject returns every Linked_Opportunity__c of a
// project (POST /linked-opportunities/search {projectId}).
func (c *Client) LinkedOpportunitiesByProject(ctx context.Context, projectID string) ([]LinkedOpportunity, error) {
	return pagedChildSearch[LinkedOpportunity](ctx, c, linkedOpportunitySearchPath, "linked opportunities",
		childSearchRequest{ProjectID: projectID})
}

// LinkedOpportunitiesByOpportunity returns every Linked_Opportunity__c of an
// opportunity (POST /linked-opportunities/search {opportunityId}): the
// projects an opportunity's invoices belong to.
func (c *Client) LinkedOpportunitiesByOpportunity(ctx context.Context, opportunityID string) ([]LinkedOpportunity, error) {
	return pagedChildSearch[LinkedOpportunity](ctx, c, linkedOpportunitySearchPath, "linked opportunities",
		childSearchRequest{OpportunityID: opportunityID})
}

// InvoicesByOpportunity returns every Invoice__c of an opportunity
// (POST /invoices/search {opportunityId}), auto-created ones included.
func (c *Client) InvoicesByOpportunity(ctx context.Context, opportunityID string) ([]Invoice, error) {
	return pagedChildSearch[Invoice](ctx, c, invoiceSearchPath, "invoices",
		childSearchRequest{OpportunityID: opportunityID})
}

// UpdateProjectStatus sets a Salesforce project's State__c through
// PATCH /projects/{id} {status}. sales-entity-service accepts "Open" and
// "Suspended" only; Salesforce adds or removes the "[S]" in the project name
// itself.
func (c *Client) UpdateProjectStatus(ctx context.Context, projectID, status string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fmt.Errorf("salesentity: project id is required")
	}
	return c.mutateWithRetry(ctx, http.MethodPatch, projectPath+url.PathEscape(projectID),
		map[string]string{"status": status}, "project", nil)
}

// pagedChildSearch reads every page of a search by one parent id. An empty
// result is an empty slice, not an error: a project with no linked
// opportunity, or an opportunity with no invoice, is an ordinary state.
func pagedChildSearch[T any](ctx context.Context, c *Client, path, what string, req childSearchRequest) ([]T, error) {
	var all []T
	for page := 0; page < childSearchMaxPages; page++ {
		req.Limit = childSearchPageSize
		req.Offset = page * childSearchPageSize
		var rows []T
		if err := c.searchWithRetry(ctx, path, req, what, &rows); err != nil {
			return nil, err
		}
		all = append(all, rows...)
		if len(rows) < childSearchPageSize {
			return all, nil
		}
	}
	return nil, fmt.Errorf("salesentity: %s still returns full pages after %d pages; stopping", path, childSearchMaxPages)
}
