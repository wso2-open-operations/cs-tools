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

// Package timecards is the read-only client for the entity-service calls the
// monthly time card report makes (the project search that resolves a project
// key, the project's contacts, and GET /projects/{id}/time-cards/report) and
// the types they return. internal/timecardreport turns the result into
// mail; internal/notify renders it. The types live here, apart from the
// task, so notify can depend on them without the task depending on notify
// in a cycle -- the same split internal/regressionreport uses.
package timecards

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

var tokenFetchTimeout = 10 * time.Second

// maxResponseBytes bounds one response body. A month of one project's cases is
// a few hundred rows at most; eight megabytes is a generous ceiling.
const maxResponseBytes = 8 << 20

// pageSize is the page size asked of the two paged searches; entity-service
// caps it at 50.
const pageSize = 50

// maxPages bounds a paged search so a misbehaving upstream cannot loop it
// forever: 40 pages of 50 is 2000 rows.
const maxPages = 40

// Project is the part of a project search result the report needs.
type Project struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Contact is the part of a project contact the report needs.
type Contact struct {
	Email string `json:"email"`
	// IsCsIntegrationUser marks a system-to-system user that never reads mail.
	IsCsIntegrationUser bool   `json:"isCsIntegrationUser"`
	RegistrationState   string `json:"registrationState"`
}

// Report is one project's approved time per case for a date range, as
// entity-service returns it. Every duration is a whole number of minutes.
type Report struct {
	Project ReportProject `json:"project"`
	// From and To are the inclusive range as requested, "yyyy-MM-dd".
	From   string `json:"from"`
	To     string `json:"to"`
	Totals Totals `json:"totals"`
	Cases  []Case `json:"cases"`
}

// ReportProject identifies the project a report is for. Name is empty when the
// project has none.
type ReportProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Totals are the project-wide sums, in minutes.
type Totals struct {
	TotalMinutes       int `json:"totalMinutes"`
	BillableMinutes    int `json:"billableMinutes"`
	NonBillableMinutes int `json:"nonBillableMinutes"`
	PatchingMinutes    int `json:"patchingMinutes"`
}

// Case is one case's approved time, in minutes. ExternalCaseID and CaseType are
// empty when the case has none; CaseType is a lowercase work item type such as
// "case" or "service_request".
type Case struct {
	CaseID             string `json:"caseId"`
	CaseNumber         string `json:"caseNumber"`
	ExternalCaseID     string `json:"externalCaseId"`
	CaseType           string `json:"caseType"`
	TotalMinutes       int    `json:"totalMinutes"`
	BillableMinutes    int    `json:"billableMinutes"`
	NonBillableMinutes int    `json:"nonBillableMinutes"`
	PatchingMinutes    int    `json:"patchingMinutes"`
}

// Config is the entity-service connection: the same OAuth client
// credentials every other entity-service client in this component uses.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client calls entity-service.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client. Both URLs must be https, as everywhere
// else in this component.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("timecards: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("timecards: base URL: %w", err)
	}

	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenHTTPClient := &http.Client{Timeout: tokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTPClient)
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTPClient)
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 60 * time.Second
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

type pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type searchProjectsRequest struct {
	SearchQuery string     `json:"searchQuery"`
	Pagination  pagination `json:"pagination"`
}

type searchProjectsResponse struct {
	Projects []Project `json:"projects"`
	HasMore  bool      `json:"hasMore"`
}

// SearchProjects returns every project whose name, key or subscription type
// contains query (entity-service matches a substring, case-insensitively), all
// pages. A caller that wants one project by key must pick the exact match out
// of the result.
func (c *Client) SearchProjects(ctx context.Context, query string) ([]Project, error) {
	var all []Project
	for page := 0; page < maxPages; page++ {
		body, err := json.Marshal(searchProjectsRequest{SearchQuery: query, Pagination: pagination{Limit: pageSize, Offset: page * pageSize}})
		if err != nil {
			return nil, fmt.Errorf("timecards: encode project search: %w", err)
		}
		var out searchProjectsResponse
		if err := c.do(ctx, http.MethodPost, "/projects/search", body, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Projects...)
		if !out.HasMore || len(out.Projects) == 0 {
			return all, nil
		}
	}
	return nil, fmt.Errorf("timecards: project search for one key still has more results after %d pages", maxPages)
}

type searchContactsRequest struct {
	Pagination pagination `json:"pagination"`
}

type searchContactsResponse struct {
	Contacts []Contact `json:"contacts"`
	Total    int       `json:"total"`
}

// ProjectContacts returns every contact of the project, all pages.
// entity-service already leaves out deactivated memberships.
func (c *Client) ProjectContacts(ctx context.Context, projectID string) ([]Contact, error) {
	var all []Contact
	for page := 0; page < maxPages; page++ {
		body, err := json.Marshal(searchContactsRequest{Pagination: pagination{Limit: pageSize, Offset: page * pageSize}})
		if err != nil {
			return nil, fmt.Errorf("timecards: encode contact search: %w", err)
		}
		var out searchContactsResponse
		if err := c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/contacts/search", body, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Contacts...)
		if len(out.Contacts) == 0 || len(all) >= out.Total {
			return all, nil
		}
	}
	return nil, fmt.Errorf("timecards: project has more contacts than %d pages hold", maxPages)
}

// Report fetches the project's approved time cards per case for the inclusive
// range from..to, both "yyyy-MM-dd".
func (c *Client) Report(ctx context.Context, projectID, from, to string) (Report, error) {
	q := url.Values{}
	q.Set("from", from)
	q.Set("to", to)
	var out Report
	if err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectID)+"/time-cards/report?"+q.Encode(), nil, &out); err != nil {
		return Report{}, err
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, into any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("timecards: build %s request: %w", method, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("timecards: %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("timecards: read %s response: %w", method, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("timecards: decode %s response: %w", method, err)
	}
	return nil
}
