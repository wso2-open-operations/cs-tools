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

// Package salesentity is an HTTP client for the REST app sales/sales-entity-service
// (not GraphQL sales/entity-graphql-service). POST /salesforce/events uses
// POST /customer-search to fetch a Customer by Salesforce Account Id. The write
// side (PATCH /contacts/{id}, PATCH /project-contacts/{id}) backs the
// registration flip -- see internal/service/membership_registration_service.go.
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
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

const (
	customerSearchPath       = "/customer-search"
	contactSearchPath        = "/contacts/search"
	projectContactSearchPath = "/project-contacts/search"
	// contactPath / projectContactPath are the single-record write resources;
	// the record's Salesforce Id is appended: PATCH /contacts/{id} and
	// PATCH /project-contacts/{id}.
	contactPath        = "/contacts/"
	projectContactPath = "/project-contacts/"
	defaultTimeout     = 15 * time.Second
	tokenExpirySlack   = 30 * time.Second
)

// ClientCredentialsConfig holds the OAuth2 client credentials used to obtain
// a bearer token for service-to-service calls to the Choreo API.
type ClientCredentialsConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes is a space-separated list of OAuth2 scopes.
	Scopes string
}

// Customer is the REST sales/sales-entity-service Customer JSON this service maps onto Postgres.
type Customer struct {
	ID                    string  `json:"id"`
	Name                  *string `json:"name"`
	Industry              *string `json:"industry"`
	SubIndustry           *string `json:"subIndustry"`
	SubRegion             *string `json:"subRegion"`
	Region                *string `json:"region"`
	SalesRegions          *string `json:"salesRegions"`
	GlobalPod             *string `json:"globalPod"`
	Phone                 *string `json:"phone"`
	NAICSIndustry         *string `json:"naicsIndustry"`
	Status                *string `json:"status"`
	AccountClassification *string `json:"accountClassification"`
	TechnicalOwner        *string `json:"technicalOwner"`
}

type customerSearchRequest struct {
	IDs        []string `json:"ids"`
	IsRealTime bool     `json:"isRealTime"`
	Limit      int      `json:"limit"`
}

// ProjectContact is one Salesforce Project_Contact__c (a contact's membership
// of a project) as returned by REST sales/sales-entity-service
// POST /project-contacts/search. Field names follow that service's contract
// (a project is a "subscription", an account id a "customerId").
type ProjectContact struct {
	ID               string                      `json:"id"`
	Email            string                      `json:"email"`
	State            *string                     `json:"state"`
	Role             *string                     `json:"role"`
	Roles            []string                    `json:"roles"`
	Type             *string                     `json:"type"`
	Contact          *ProjectContactContact      `json:"contact"`
	Subscription     *ProjectContactSubscription `json:"subscription"`
	CreatedDate      *string                     `json:"createdDate"`
	LastModifiedDate *string                     `json:"lastModifiedDate"`
}

// ProjectContactContact is the linked Contact of a ProjectContact.
type ProjectContactContact struct {
	ID         *string `json:"id"`
	Name       *string `json:"name"`
	Email      *string `json:"email"`
	CustomerID *string `json:"customerId"`
}

// ProjectContactSubscription is the linked Project of a ProjectContact.
type ProjectContactSubscription struct {
	ID         *string `json:"id"`
	Name       *string `json:"name"`
	Key        *string `json:"key"`
	CustomerID *string `json:"customerId"`
}

// Contact is the subset of a REST sales/sales-entity-service Contact
// (POST /contacts/search) the membership ingest needs.
type Contact struct {
	ID                  *string             `json:"id"`
	Email               *string             `json:"email"`
	Name                *string             `json:"name"`
	FirstName           *string             `json:"firstName"`
	LastName            *string             `json:"lastName"`
	IsCsAdmin           *bool               `json:"isCsAdmin"`
	IsCsIntegrationUser *bool               `json:"isCsIntegrationUser"`
	LockoutStatus       *bool               `json:"lockoutStatus"`
	Account             *ContactAccount     `json:"account"`
	Memberships         []ContactMembership `json:"memberships"`
}

// ContactAccount is the parent account of a Contact.
type ContactAccount struct {
	ID             *string `json:"id"`
	Classification *string `json:"classification"`
	IsPartner      *bool   `json:"isPartner"`
}

// ContactMembership is one Project_Contacts__r row embedded in a Contact.
type ContactMembership struct {
	ID             *string `json:"id"`
	SubscriptionID *string `json:"subscriptionId"`
	State          *string `json:"state"`
	Role           *string `json:"role"`
	Type           *string `json:"type"`
}

type idSearchRequest struct {
	ID    string `json:"id"`
	Limit int    `json:"limit"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

var errCustomerUnauthorized = errors.New("salesentity: customer request unauthorized")

// Client fetches customers from REST sales/sales-entity-service using a client-credentials grant.
type Client struct {
	baseURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	scopes       string
	httpClient   *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// New constructs a Client with a default 15-second timeout.
func New(baseURL string, creds ClientCredentialsConfig) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		tokenURL:     creds.TokenURL,
		clientID:     creds.ClientID,
		clientSecret: creds.ClientSecret,
		scopes:       creds.Scopes,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// GetCustomer returns the REST sales/sales-entity-service Customer for the given
// Salesforce Account Id via POST /customer-search with isRealTime true. An empty array is mapped to
// ServiceUnavailableError so the caller can retry (the event can arrive before
// Salesforce commits). On 401 the token is refreshed and the request is retried once.
func (c *Client) GetCustomer(ctx context.Context, id string) (Customer, error) {
	cust, err := c.getCustomer(ctx, id)
	if errors.Is(err, errCustomerUnauthorized) {
		c.invalidateToken()
		cust, err = c.getCustomer(ctx, id)
		if errors.Is(err, errCustomerUnauthorized) {
			return Customer{}, &apierror.UnauthorizedError{Msg: "salesentity: customer-search unauthorized"}
		}
	}
	return cust, err
}

// GetProjectContact fetches one Salesforce Project_Contact__c by Id via
// POST /project-contacts/search {id}. An empty result is a
// ServiceUnavailableError so the caller can retry — the Salesforce event can
// arrive before the record is visible to the query, same as GetCustomer.
func (c *Client) GetProjectContact(ctx context.Context, id string) (ProjectContact, error) {
	var rows []ProjectContact
	if err := c.searchWithRetry(ctx, projectContactSearchPath, idSearchRequest{ID: id, Limit: 1}, "project contact", &rows); err != nil {
		return ProjectContact{}, err
	}
	for _, pc := range rows {
		if salesforceIDEqual(pc.ID, id) {
			return pc, nil
		}
	}
	if len(rows) == 0 {
		return ProjectContact{}, &apierror.ServiceUnavailableError{Msg: "salesentity: project contact not found"}
	}
	return ProjectContact{}, &apierror.ServiceUnavailableError{Msg: "salesentity: project-contacts/search returned an unexpected project contact"}
}

// GetContact fetches one Salesforce Contact by Id via POST /contacts/search
// {id}, with the same empty-result semantics as GetProjectContact.
func (c *Client) GetContact(ctx context.Context, id string) (Contact, error) {
	var rows []Contact
	if err := c.searchWithRetry(ctx, contactSearchPath, idSearchRequest{ID: id, Limit: 1}, "contact", &rows); err != nil {
		return Contact{}, err
	}
	for _, ct := range rows {
		if ct.ID != nil && salesforceIDEqual(*ct.ID, id) {
			return ct, nil
		}
	}
	if len(rows) == 0 {
		return Contact{}, &apierror.ServiceUnavailableError{Msg: "salesentity: contact not found"}
	}
	return Contact{}, &apierror.ServiceUnavailableError{Msg: "salesentity: contacts/search returned an unexpected contact"}
}

// searchWithRetry POSTs a search body to path and decodes the JSON array
// response into out, refreshing the token once on 401 (the same policy
// GetCustomer applies).
func (c *Client) searchWithRetry(ctx context.Context, path string, body any, what string, out any) error {
	err := c.search(ctx, path, body, what, out)
	if errors.Is(err, errCustomerUnauthorized) {
		c.invalidateToken()
		err = c.search(ctx, path, body, what, out)
		if errors.Is(err, errCustomerUnauthorized) {
			return &apierror.UnauthorizedError{Msg: "salesentity: " + path + " unauthorized"}
		}
	}
	return err
}

// search performs one authenticated POST of body to path and decodes a 2xx
// JSON array into out. Status handling mirrors getCustomer: 401 →
// errCustomerUnauthorized (for the caller's single token refresh), 404 and
// 5xx → ServiceUnavailableError, other non-2xx → DownstreamError.
func (c *Client) search(ctx context.Context, path string, body any, what string, out any) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("salesentity: marshal %s request: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("salesentity: build %s request: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		return &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesentity: %s request: %v", path, err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("salesentity: read %s response: %w", path, err)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("salesentity: parse %s response: %w", path, err)
		}
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return errCustomerUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return &apierror.ServiceUnavailableError{Msg: "salesentity: " + what + " not found"}
	case resp.StatusCode >= 500:
		return &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesentity: %s returned %d", path, resp.StatusCode)}
	default:
		return &apierror.DownstreamError{Msg: fmt.Sprintf("salesentity rejected %s (status %d)", path, resp.StatusCode)}
	}
}

func (c *Client) getCustomer(ctx context.Context, id string) (Customer, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return Customer{}, err
	}

	body, err := json.Marshal(customerSearchRequest{
		IDs:        []string{id},
		IsRealTime: true,
		Limit:      1,
	})
	if err != nil {
		return Customer{}, fmt.Errorf("salesentity: marshal customer-search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+customerSearchPath, bytes.NewReader(body))
	if err != nil {
		return Customer{}, fmt.Errorf("salesentity: build customer-search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return Customer{}, err
		}
		return Customer{}, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesentity: customer-search request: %v", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Customer{}, fmt.Errorf("salesentity: read customer-search response: %w", err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var customers []Customer
		if err := json.Unmarshal(raw, &customers); err != nil {
			return Customer{}, fmt.Errorf("salesentity: parse customer-search response: %w", err)
		}
		if len(customers) == 0 {
			return Customer{}, &apierror.ServiceUnavailableError{Msg: "salesentity: customer not found"}
		}
		cust, ok := matchingCustomer(customers, id)
		if !ok {
			return Customer{}, &apierror.ServiceUnavailableError{Msg: "salesentity: customer-search returned an unexpected customer"}
		}
		return cust, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return Customer{}, errCustomerUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return Customer{}, &apierror.ServiceUnavailableError{Msg: "salesentity: customer not found"}
	case resp.StatusCode >= 500:
		return Customer{}, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesentity: customer-search returned %d", resp.StatusCode)}
	default:
		return Customer{}, &apierror.DownstreamError{Msg: fmt.Sprintf("salesentity rejected customer-search (status %d)", resp.StatusCode)}
	}
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedToken != "" && time.Now().Add(tokenExpirySlack).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	if c.scopes != "" {
		data.Set("scope", c.scopes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("salesentity: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesentity: token endpoint unavailable: %v", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("salesentity: read token response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", &apierror.UnauthorizedError{Msg: fmt.Sprintf("salesentity: token endpoint rejected credentials (status %d)", resp.StatusCode)}
	default:
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesentity: token endpoint returned %d", resp.StatusCode)}
	}

	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("salesentity: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("salesentity: token endpoint returned empty access_token")
	}

	c.cachedToken = tr.AccessToken
	if tr.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		c.tokenExpiry = time.Now().Add(3600 * time.Second)
	}
	return c.cachedToken, nil
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedToken = ""
	c.tokenExpiry = time.Time{}
}

func matchingCustomer(customers []Customer, id string) (Customer, bool) {
	for _, cust := range customers {
		if salesforceIDEqual(cust.ID, id) {
			return cust, true
		}
	}
	return Customer{}, false
}

// salesforceIDEqual treats 15-char and 18-char Ids for the same record as equal.
func salesforceIDEqual(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	if strings.EqualFold(got, want) {
		return true
	}
	shorter, longer := got, want
	if len(got) > len(want) {
		shorter, longer = want, got
	}
	return len(shorter) == 15 && len(longer) == 18 && strings.EqualFold(longer[:15], shorter)
}
