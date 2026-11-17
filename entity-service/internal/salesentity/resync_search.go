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

import "context"

// SearchFilter is the body of the list searches the admin resync makes:
// POST /projects/search, /opportunities/search, /linked-opportunities/search,
// /invoices/search, /contacts/search and /project-contacts/search. Every
// field is optional and the ones a search does not know are never sent
// (omitempty), so one struct serves all six; each search names the filters
// it reads below.
//
// MinLastModifiedDate with MinID is keyset pagination, ordered by
// (LastModifiedDate, Id): the first page sends only the date (an inclusive
// lower bound), the next sends the previous page's last lastModifiedDate and
// Id. It has no offset ceiling, unlike limit/offset (capped at 2000).
type SearchFilter struct {
	// CustomerID is the owning Account Id (projects, opportunities).
	CustomerID string `json:"customerId,omitempty"`
	// AccountID is the parent Account Id of a contact (contacts only).
	AccountID string `json:"accountId,omitempty"`
	// ProjectID is the Project__c Id (linked opportunities, project contacts).
	ProjectID string `json:"projectId,omitempty"`
	// OpportunityID is the parent Opportunity Id (invoices, linked opportunities).
	OpportunityID string `json:"opportunityId,omitempty"`
	// IDs is the batch lookup by record Id.
	IDs []string `json:"ids,omitempty"`
	// Limit is the page size; Sales Entity caps it at 1000.
	Limit               int    `json:"limit,omitempty"`
	MinLastModifiedDate string `json:"minLastModifiedDate,omitempty"`
	MinID               string `json:"minId,omitempty"`
}

// SearchProjects lists projects (filters: customerId, ids). The states filter
// does not exist here; a project search returns every project.
func (c *Client) SearchProjects(ctx context.Context, f SearchFilter) ([]Project, error) {
	var rows []Project
	err := c.searchWithRetry(ctx, projectSearchPath, f, "project", &rows)
	return rows, err
}

// SearchOpportunities lists opportunities, each with its embedded line items
// (filters: customerId, ids). Every opportunity carries its line items, so
// callers keep the page small.
func (c *Client) SearchOpportunities(ctx context.Context, f SearchFilter) ([]Opportunity, error) {
	var rows []Opportunity
	err := c.searchWithRetry(ctx, opportunitySearchPath, f, "opportunity", &rows)
	return rows, err
}

// SearchLinkedOpportunities lists project-opportunity links (filters:
// projectId, opportunityId).
func (c *Client) SearchLinkedOpportunities(ctx context.Context, f SearchFilter) ([]LinkedOpportunity, error) {
	var rows []LinkedOpportunity
	err := c.searchWithRetry(ctx, linkedOpportunitySearchPath, f, "linked opportunity", &rows)
	return rows, err
}

// SearchInvoices lists invoices (filter: opportunityId, which takes ONE
// opportunity Id).
func (c *Client) SearchInvoices(ctx context.Context, f SearchFilter) ([]Invoice, error) {
	var rows []Invoice
	err := c.searchWithRetry(ctx, invoiceSearchPath, f, "invoice", &rows)
	return rows, err
}

// SearchContacts lists contacts (filters: accountId, ids).
func (c *Client) SearchContacts(ctx context.Context, f SearchFilter) ([]Contact, error) {
	var rows []Contact
	err := c.searchWithRetry(ctx, contactSearchPath, f, "contact", &rows)
	return rows, err
}

// SearchProjectContacts lists memberships (filter: projectId). The request
// carries no states filter, which Sales Entity reads as "every state", so
// DEACTIVATED memberships are listed too.
func (c *Client) SearchProjectContacts(ctx context.Context, f SearchFilter) ([]ProjectContact, error) {
	var rows []ProjectContact
	err := c.searchWithRetry(ctx, projectContactSearchPath, f, "project contact", &rows)
	return rows, err
}
