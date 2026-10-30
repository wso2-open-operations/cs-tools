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

// Package salesentity reads the Salesforce data the closure sweep needs
// (project dates, the account's people and classification, contacts, linked
// opportunities, opportunities and invoices) live from REST
// sales/sales-entity-service, and writes a suspended project's State__c back.
// CSM keeps the closure sweep's own records (the open-project list, closure
// states and the notices sent); everything Salesforce owns is read here, so
// it is never a stale copy.
package salesentity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/httpsec"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ErrNotFound is returned when a record looked up by id is not in Salesforce.
var ErrNotFound = errors.New("salesentity: record not found")

// Paging: sales-entity-service caps a page at 1000 rows and an offset at
// 2000, so pageSize x maxPages stays inside both. Reaching maxPages is an
// error, so a misbehaving upstream cannot keep the run going and skipped
// rows are visible.
const (
	pageSize        = 200
	maxPages        = 10
	maxResponseBody = 10 << 20
	// opportunityBatch is how many opportunity ids go in one search.
	opportunityBatch = 100
)

var tokenFetchTimeout = 10 * time.Second

// Config holds the sales-entity-service connection settings.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes is optional: sales-entity-service relies on the gateway.
	Scopes []string
}

// Client calls sales-entity-service with a client-credentials token, built
// through internal/httpsec like the CSM and email clients: https only, no
// redirects on either the token or the API client, bounded success bodies.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient validates cfg and builds a Client.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireHTTPS("TokenURL", cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("salesentity: %w", err)
	}
	if err := httpsec.RequireHTTPS("BaseURL", cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("salesentity: %w", err)
	}
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenHTTPClient := &http.Client{Timeout: tokenFetchTimeout, CheckRedirect: httpsec.RefuseRedirects}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTPClient)
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second
	httpClient.CheckRedirect = httpsec.RefuseRedirects
	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// Project is the part of a Salesforce Project__c the sweep reads.
type Project struct {
	ID         string  `json:"id"`
	Key        *string `json:"key"`
	StartDate  *string `json:"startDate"`
	EndDate    *string `json:"endDate"`
	CustomerID *string `json:"customerId"`
}

// User is a Salesforce user reference (account owner, renewal manager).
type User struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

// Customer is the part of a Salesforce Account the sweep reads. The owner is
// the account manager; technicalOwner is an email. isPartner in the
// sales-entity-service response is Salesforce's Partner Portal flag, not the
// classification, so it is deliberately not read: a partner account is one
// whose accountClassification is "Partner".
type Customer struct {
	ID                      string  `json:"id"`
	Owner                   *User   `json:"owner"`
	TechnicalOwner          *string `json:"technicalOwner"`
	RenewalManager          *User   `json:"renewalManager"`
	AccountClassification   *string `json:"accountClassification"`
	PrimaryPartnerAccountID *string `json:"primaryPartnerAccountId"`
}

// ProjectContact is a Salesforce Project_Contact__c membership.
type ProjectContact struct {
	Email   string                `json:"email"`
	State   *string               `json:"state"`
	Roles   []string              `json:"roles"`
	Contact *ProjectContactPerson `json:"contact"`
}

// ProjectContactPerson is the contact a membership belongs to.
type ProjectContactPerson struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

// Contact is a Salesforce Contact of an account.
type Contact struct {
	Name             *string `json:"name"`
	Email            *string `json:"email"`
	IsPrimaryContact bool    `json:"isPrimaryContact"`
}

// LinkedOpportunity is a Salesforce Linked_Opportunity__c (project → opportunity).
type LinkedOpportunity struct {
	ID            string  `json:"id"`
	OpportunityID *string `json:"opportunityId"`
}

// Opportunity is the part of a Salesforce Opportunity the sweep reads.
type Opportunity struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	StageName   *string `json:"stageName"`
	EulaVersion *string `json:"eulaVersion"`
}

// Invoice is a Salesforce Invoice__c.
type Invoice struct {
	ID             *string `json:"id"`
	Name           *string `json:"name"`
	OpportunityID  *string `json:"opportunityId"`
	DueDate        *string `json:"dueDate"`
	PaidDate       *string `json:"paidDate"`
	InvoiceDate    *string `json:"invoiceDate"`
	Classification *string `json:"classification"`
}

// GetProject reads one project by its Salesforce id.
func (c *Client) GetProject(ctx context.Context, id string) (Project, error) {
	var rows []Project
	if err := c.post(ctx, "/projects/search", map[string]any{"id": id, "limit": 1}, &rows); err != nil {
		return Project{}, err
	}
	if len(rows) == 0 {
		return Project{}, fmt.Errorf("project %s: %w", id, ErrNotFound)
	}
	return rows[0], nil
}

// GetCustomer reads one account live from Salesforce (isRealTime). The
// non-real-time path reads a database copy that can be stale and has no
// renewal manager.
func (c *Client) GetCustomer(ctx context.Context, id string) (Customer, error) {
	var rows []Customer
	if err := c.post(ctx, "/customer-search", map[string]any{"ids": []string{id}, "isRealTime": true, "limit": 1}, &rows); err != nil {
		return Customer{}, err
	}
	if len(rows) == 0 {
		return Customer{}, fmt.Errorf("customer %s: %w", id, ErrNotFound)
	}
	return rows[0], nil
}

// ProjectContacts reads every membership of a project.
func (c *Client) ProjectContacts(ctx context.Context, projectID string) ([]ProjectContact, error) {
	var rows []ProjectContact
	if err := c.post(ctx, "/project-contacts/search", map[string]any{"projectId": projectID, "fetchAll": true}, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// AccountContacts reads every contact of an account.
func (c *Client) AccountContacts(ctx context.Context, accountID string) ([]Contact, error) {
	return pagedSearch[Contact](ctx, c, "/contacts/search", "accountId", accountID)
}

// LinkedOpportunities reads every opportunity link of a project.
func (c *Client) LinkedOpportunities(ctx context.Context, projectID string) ([]LinkedOpportunity, error) {
	return pagedSearch[LinkedOpportunity](ctx, c, "/linked-opportunities/search", "projectId", projectID)
}

// Invoices reads every invoice of an opportunity.
func (c *Client) Invoices(ctx context.Context, opportunityID string) ([]Invoice, error) {
	return pagedSearch[Invoice](ctx, c, "/invoices/search", "opportunityId", opportunityID)
}

// Opportunities reads opportunities by id, in batches. No ids, no call.
func (c *Client) Opportunities(ctx context.Context, ids []string) ([]Opportunity, error) {
	var all []Opportunity
	for start := 0; start < len(ids); start += opportunityBatch {
		end := min(start+opportunityBatch, len(ids))
		var rows []Opportunity
		if err := c.post(ctx, "/opportunities/search", map[string]any{"ids": ids[start:end], "limit": end - start}, &rows); err != nil {
			return nil, err
		}
		all = append(all, rows...)
	}
	return all, nil
}

// UpdateProjectStatus sets a project's State__c ("Open" or "Suspended") by its
// Salesforce id, through PATCH /projects/{id}. The response (the project
// re-read after the write) is not used.
func (c *Client) UpdateProjectStatus(ctx context.Context, id, status string) error {
	return c.send(ctx, http.MethodPatch, "/projects/"+url.PathEscape(id), map[string]string{"status": status}, nil)
}

// pagedSearch reads filter=value page by page until a short page, up to
// maxPages.
func pagedSearch[T any](ctx context.Context, c *Client, path, filter, value string) ([]T, error) {
	var all []T
	for page := 0; page < maxPages; page++ {
		var rows []T
		body := map[string]any{filter: value, "limit": pageSize, "offset": page * pageSize}
		if err := c.post(ctx, path, body, &rows); err != nil {
			return nil, err
		}
		all = append(all, rows...)
		if len(rows) < pageSize {
			return all, nil
		}
	}
	return nil, fmt.Errorf("salesentity: %s still returns full pages after %d pages; stopping", path, maxPages)
}

// post sends body as JSON and decodes a 2xx JSON response into out.
func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	return c.send(ctx, http.MethodPost, path, body, out)
}

// send sends body as JSON with method and, when out is not nil, decodes a 2xx
// JSON response into it. A non-2xx is an *apierror.Error carrying the status
// and at most 256 bytes of body.
func (c *Client) send(ctx context.Context, method, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("salesentity: marshal %s request: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("salesentity: build %s request: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("salesentity: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt, err := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		if err != nil {
			return fmt.Errorf("salesentity: read error response body: %w", err)
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}
	raw, err := httpsec.ReadBounded(resp.Body, maxResponseBody)
	if err != nil {
		return fmt.Errorf("salesentity: read %s response: %w", path, err)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("salesentity: parse %s response: %w", path, err)
	}
	return nil
}
