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

package slabreachreport

import (
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

// maxResponseBytes bounds the report's body. A week's breaches across the
// covered definitions is a few hundred rows at most; eight megabytes is a
// generous ceiling.
const maxResponseBytes = 8 << 20

// Report is one week's SLA breaches under one report's filters, as
// entity-service returns it.
type Report struct {
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
	// Rows are the matching breaches, ordered by the account's
	// integration CS team, then SLA definition, then breach time.
	Rows []Row `json:"rows"`
}

// Row is one breached SLA clock, carrying the columns the source report
// lists plus the case number its link needs.
type Row struct {
	Team                   string    `json:"team"`
	WSO2CaseID             string    `json:"wso2CaseId"`
	CaseNumber             string    `json:"caseNumber"`
	Product                string    `json:"product"`
	CaseType               string    `json:"caseType"`
	Priority               string    `json:"priority"`
	Target                 string    `json:"target"`
	BusinessElapsedPercent float64   `json:"businessElapsedPercent"`
	Definition             string    `json:"definition"`
	BreachedOn             time.Time `json:"breachedOn"`
	OnboardingStatus       string    `json:"onboardingStatus"`
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

// Client fetches the report.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client. Both URLs must be https, as everywhere
// else in this component.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("slabreachreport: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("slabreachreport: base URL: %w", err)
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

// Report fetches one week's breaches under one report's filters.
//
// A zero week asks for last week, which entity-service resolves in the
// report's own timezone from the instant of the call -- the scheduled
// path. A non-zero week asks for the week CONTAINING that date, which is
// the manual re-send and testing path.
//
// onboardingStatuses is ServiceNow's "Onboarding Status is one of" and
// excludeOnboardingStatuses its "is not one of"; the two reports use one
// each. An empty filter is omitted entirely rather than sent blank, so
// the server reads it as "no restriction".
func (c *Client) Report(ctx context.Context, definitions, stages, onboardingStatuses, excludeOnboardingStatuses []string, week time.Time) (Report, error) {
	q := url.Values{}
	q.Set("definitions", strings.Join(definitions, ","))
	if len(stages) > 0 {
		q.Set("stages", strings.Join(stages, ","))
	}
	if len(onboardingStatuses) > 0 {
		q.Set("onboardingStatuses", strings.Join(onboardingStatuses, ","))
	}
	if len(excludeOnboardingStatuses) > 0 {
		q.Set("excludeOnboardingStatuses", strings.Join(excludeOnboardingStatuses, ","))
	}
	if !week.IsZero() {
		q.Set("week", week.Format("2006-01-02"))
	}

	endpoint := c.baseURL + "/reports/sla-breaches?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Report{}, fmt.Errorf("slabreachreport: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Report{}, fmt.Errorf("slabreachreport: fetch report: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Report{}, fmt.Errorf("slabreachreport: read report: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Report{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var report Report
	if err := json.Unmarshal(body, &report); err != nil {
		return Report{}, fmt.Errorf("slabreachreport: decode report: %w", err)
	}
	return report, nil
}
