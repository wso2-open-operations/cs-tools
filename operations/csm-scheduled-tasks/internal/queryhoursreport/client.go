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

// Package queryhoursreport is a narrow, read-only client for entity-service's
// weekly query-hour report endpoint (GET /query-hours/weekly-report), plus
// the wire types it returns. The sub-cron that renders and sends the report
// lives in internal/queryhoursweekly.
//
// It is the Go port of ServiceNow's `[WSO2][Query Hours] Weekly Report`, and
// it is deliberately SEPARATE from the per-project threshold notification
// that integrations/csm-notification-service sends. The two look related and
// are not: that one fires per project when a 75/90/100 PERCENT threshold is
// crossed and renders a five-column table with a bare project name; this one
// runs weekly over the whole estate, decides membership on an ABSOLUTE floor
// of ten remaining hours, and renders seven columns with the project key and
// a Salesforce link. Merging them has already been tried once by accident —
// a project-key suffix leaked from this format into that email — and the
// separation is the fix.
//
// A fourth entity-service client in this component, alongside internal/ledger,
// internal/entitycases and internal/engagementallocations, for the reason
// those packages' own doc comments give: same deployment, unrelated concern.
package queryhoursreport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// reportTimeout bounds the report request. The whole report is one set-based
// query over the in-service product lines rather than a walk over every
// account, so it is fast — but it does scan the estate's approved time cards,
// and a cold cache on a large instance is slower than a normal API call.
const reportTimeout = 2 * time.Minute

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is a narrow HTTP client for entity-service's weekly query-hour
// report endpoint.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant. cfg.TokenURL/cfg.BaseURL must both be https (loopback
// http is allowed for local development — see httpsec.RequireSecureURL).
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("queryhoursreport: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("queryhoursreport: base URL: %w", err)
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
	httpClient.Timeout = reportTimeout
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}, nil
}

// Project is one project cell in the report table.
type Project struct {
	Name            string `json:"name"`
	Key             string `json:"key"`
	SFID            string `json:"sfId"`
	ConsumedMinutes int    `json:"consumedMinutes"`
	// Duplicate marks a project already shown earlier in the same group,
	// funded by more than one of that group's opportunities. Its consumption
	// counts once toward the group total; the row is greyed rather than
	// dropped so the opportunity's own funding stays visible.
	Duplicate bool `json:"duplicate"`
}

// Opportunity is one opportunity and the projects it funds.
type Opportunity struct {
	Name               string    `json:"name"`
	SFID               string    `json:"sfId"`
	EntitlementMinutes int       `json:"entitlementMinutes"`
	Projects           []Project `json:"projects"`
}

// Group is one connected component of the opportunity-to-project funding
// graph — see entity-service's domain.QueryHoursReportGroup for why the
// component, rather than the opportunity, is the unit the thresholds apply to.
type Group struct {
	Opportunities      []Opportunity `json:"opportunities"`
	EntitlementMinutes int           `json:"entitlementMinutes"`
	ConsumedMinutes    int           `json:"consumedMinutes"`
	RemainingMinutes   int           `json:"remainingMinutes"`
	Exceeded           bool          `json:"exceeded"`
	GoingToExceed      bool          `json:"goingToExceed"`
	RowCount           int           `json:"rowCount"`
}

// Account is one account's section of the report.
type Account struct {
	Name          string  `json:"name"`
	SFID          string  `json:"sfId"`
	Groups        []Group `json:"groups"`
	Exceeded      bool    `json:"exceeded"`
	GoingToExceed bool    `json:"goingToExceed"`
	RowCount      int     `json:"rowCount"`
	// AccountManagerEmail and TechnicalOwnerEmail are reported by
	// entity-service and deliberately NOT used to address this email — see
	// SendReport's own doc comment.
	AccountManagerEmail string `json:"accountManagerEmail"`
	TechnicalOwnerEmail string `json:"technicalOwnerEmail"`
}

// Report is entity-service's wire shape for the whole report.
type Report struct {
	GeneratedOn        string    `json:"generatedOn"`
	ExceededCount      int       `json:"exceededCount"`
	GoingToExceedCount int       `json:"goingToExceedCount"`
	Exceeded           []Account `json:"exceeded"`
	GoingToExceed      []Account `json:"goingToExceed"`
	UnmatchedLineCount int       `json:"unmatchedLineCount"`
}

// WeeklyReport fetches the current report.
func (c *Client) WeeklyReport(ctx context.Context) (Report, error) {
	const path = "/query-hours/weekly-report"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return Report{}, fmt.Errorf("queryhoursreport: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Report{}, fmt.Errorf("queryhoursreport: GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Report{}, fmt.Errorf("queryhoursreport: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Report{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var parsed Report
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Report{}, fmt.Errorf("queryhoursreport: decode response: %w", err)
	}
	return parsed, nil
}
