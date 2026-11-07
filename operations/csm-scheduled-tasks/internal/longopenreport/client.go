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

// Package longopenreport is the read-only client for entity-service's long-open
// cases report (GET /reports/long-open-cases) and the types it returns.
// internal/stalecases turns the result into the mail; internal/notify renders
// it. The types live here, apart from the task, so notify can depend on them
// without the task depending on notify in a cycle.
package longopenreport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

var tokenFetchTimeout = 10 * time.Second

// maxResponseBytes bounds the report's body. Entity-service caps each table, so
// the body is far below this; eight megabytes is a generous ceiling.
const maxResponseBytes = 8 << 20

// Report is the long-open cases report as entity-service returns it: two
// tables, oldest first, each capped by entity-service.
type Report struct {
	// Variant is set by entity-service on a variant report (for example
	// "open-before-yesterday") and empty on the default age-based report.
	Variant       string    `json:"variant"`
	GeneratedAt   time.Time `json:"generatedAt"`
	OlderThanDays int       `json:"olderThanDays"`
	CreatedBefore time.Time `json:"createdBefore"`
	// Cases are the long-open cases whose project is not being onboarded.
	Cases []Row `json:"cases"`
	// OnboardingCases are the long-open cases whose project onboarding is in
	// progress or not yet started.
	OnboardingCases []Row `json:"onboardingCases"`
	// CasesTotal and OnboardingCasesTotal are the full sizes of the two tables;
	// a table holds fewer rows than its total only when it was cut at the cap.
	CasesTotal               int  `json:"casesTotal"`
	OnboardingCasesTotal     int  `json:"onboardingCasesTotal"`
	CasesTruncated           bool `json:"casesTruncated"`
	OnboardingCasesTruncated bool `json:"onboardingCasesTruncated"`
}

// Row is one long-open case.
type Row struct {
	Number string `json:"number"`
	// InternalID is the case's internal identifier; empty when it has none.
	InternalID string    `json:"internalId"`
	CreatedOn  time.Time `json:"createdOn"`
	UpdatedOn  time.Time `json:"updatedOn"`
	State      string    `json:"state"`
	// Severity is the case's severity in the API's vocabulary; empty when none
	// is set.
	Severity string `json:"severity"`
	// AssignedEngineer is the display name of the assignee, or empty.
	AssignedEngineer string `json:"assignedEngineer"`
	// CsTeam is the name of the CS team that owns the case's account.
	CsTeam string `json:"csTeam"`
	// AccountName is the name of the case's account; empty when absent.
	AccountName string `json:"accountName"`
}

// Config is the entity-service connection: the same OAuth client credentials
// every other entity-service client in this component uses.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client fetches the report.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client. Both URLs must be https, as everywhere else in
// this component.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("longopenreport: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("longopenreport: base URL: %w", err)
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

// Report fetches the report for cases open longer than olderThanDays.
func (c *Client) Report(ctx context.Context, olderThanDays int) (Report, error) {
	q := url.Values{}
	q.Set("olderThanDays", strconv.Itoa(olderThanDays))
	return c.fetch(ctx, q)
}

// OpenBeforeYesterday fetches the "open-before-yesterday" variant: every case
// still in the open state that was created before the start of yesterday, as
// entity-service reckons the day (in the team's time zone). The cases are in
// Report.Cases, oldest first; the variant has no onboarding table.
func (c *Client) OpenBeforeYesterday(ctx context.Context) (Report, error) {
	q := url.Values{}
	q.Set("variant", "open-before-yesterday")
	return c.fetch(ctx, q)
}

func (c *Client) fetch(ctx context.Context, q url.Values) (Report, error) {
	endpoint := c.baseURL + "/reports/long-open-cases?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Report{}, fmt.Errorf("longopenreport: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Report{}, fmt.Errorf("longopenreport: fetch report: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Report{}, fmt.Errorf("longopenreport: read report: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Report{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var report Report
	if err := json.Unmarshal(body, &report); err != nil {
		return Report{}, fmt.Errorf("longopenreport: decode report: %w", err)
	}
	return report, nil
}
