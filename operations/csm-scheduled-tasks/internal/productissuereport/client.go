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

// Package productissuereport is the client for entity-service's product issue
// report routes (GET /reports/product-issues and its recipients route), the
// source of the weekly and monthly product issue report emails.
package productissuereport

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

// maxResponseBytes bounds a response body; the dashboard lists are capped
// server-side (at most 50 rows each), so eight megabytes is a generous ceiling.
const maxResponseBytes = 8 << 20

// Period values of the report route.
const (
	PeriodWeek  = "week"
	PeriodMonth = "month"
)

// SLA is the SLA standing of one issue; only what the email shows is decoded.
type SLA struct {
	State      string  `json:"state"`
	Percentage float64 `json:"percentage"`
}

// CaseRef is the case an issue was raised from.
type CaseRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// Item is one issue in a list (ProductIssueDashboardItem).
type Item struct {
	ID           string     `json:"id"`
	Number       string     `json:"number"`
	Title        string     `json:"title"`
	ProductName  *string    `json:"productName"`
	Owner        string     `json:"owner"`
	Repository   string     `json:"repository"`
	IssueNumber  int64      `json:"issueNumber"`
	HTMLURL      string     `json:"htmlUrl"`
	BoardStatus  string     `json:"boardStatus"`
	Priority     string     `json:"priority"`
	OpenedOn     time.Time  `json:"openedOn"`
	WocSince     *time.Time `json:"wocSince"`
	Case         *CaseRef   `json:"case"`
	OpenedByMail *string    `json:"openedByEmail"`
	SLA          SLA        `json:"sla"`
}

// OpenBySLA counts open issues per SLA state.
type OpenBySLA struct {
	OK       int `json:"ok"`
	AtRisk   int `json:"atRisk"`
	Breached int `json:"breached"`
	Paused   int `json:"paused"`
	None     int `json:"none"`
}

// Totals are the headline figures. CompliancePercentage is null when no SLA
// outcome fell in the period.
type Totals struct {
	Open                  int       `json:"open"`
	OpenedInPeriod        int       `json:"openedInPeriod"`
	ResolvedInPeriod      int       `json:"resolvedInPeriod"`
	WaitingOnCs           int       `json:"waitingOnCs"`
	WaitingOnProductTeam  int       `json:"waitingOnProductTeam"`
	UnmatchedOpen         int       `json:"unmatchedOpen"`
	NonStandardColumnOpen int       `json:"nonStandardColumnOpen"`
	OpenBySLA             OpenBySLA `json:"openBySla"`
	SLAMet                int       `json:"slaMet"`
	SLABreached           int       `json:"slaBreached"`
	CompliancePercentage  *float64  `json:"compliancePercentage"`
}

// ProductRow is one product's (or Unmapped repository's) figures.
type ProductRow struct {
	ProductName          *string   `json:"productName"`
	Owner                string    `json:"owner"`
	Repository           string    `json:"repository"`
	Open                 int       `json:"open"`
	OpenedInPeriod       int       `json:"openedInPeriod"`
	ResolvedInPeriod     int       `json:"resolvedInPeriod"`
	WaitingOnCs          int       `json:"waitingOnCs"`
	WaitingOnProductTeam int       `json:"waitingOnProductTeam"`
	OpenBySLA            OpenBySLA `json:"openBySla"`
	SLAMet               int       `json:"slaMet"`
	SLABreached          int       `json:"slaBreached"`
	CompliancePercentage *float64  `json:"compliancePercentage"`
}

// AgeingBucket is one WOC ageing bucket.
type AgeingBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// WocAgeing is the waiting-on-CS ageing.
type WocAgeing struct {
	Buckets []AgeingBucket `json:"buckets"`
	Oldest  []Item         `json:"oldest"`
}

// BlockedIssue is one product issue blocking a case.
type BlockedIssue struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Title       string    `json:"title"`
	ProductName *string   `json:"productName"`
	BoardStatus string    `json:"boardStatus"`
	OpenedOn    time.Time `json:"openedOn"`
}

// BlockedCase is a case blocked by open issues waiting on the product team.
type BlockedCase struct {
	CaseID         string         `json:"caseId"`
	CaseNumber     string         `json:"caseNumber"`
	Subject        string         `json:"subject"`
	Issues         []BlockedIssue `json:"issues"`
	OldestOpenedOn time.Time      `json:"oldestOpenedOn"`
}

// BlockedByProduct counts blocked cases per product.
type BlockedByProduct struct {
	ProductName *string `json:"productName"`
	Owner       string  `json:"owner"`
	Repository  string  `json:"repository"`
	Cases       int     `json:"cases"`
}

// BlockedCases is the blocked-case section.
type BlockedCases struct {
	Total     int                `json:"total"`
	ByProduct []BlockedByProduct `json:"byProduct"`
	Cases     []BlockedCase      `json:"cases"`
}

// TrendBucket is one bucket of the opened/resolved trend.
type TrendBucket struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Opened   int       `json:"opened"`
	Resolved int       `json:"resolved"`
}

// Dashboard is entity-service's ProductIssueDashboard.
type Dashboard struct {
	PeriodStart  time.Time     `json:"periodStart"`
	PeriodEnd    time.Time     `json:"periodEnd"`
	GeneratedOn  time.Time     `json:"generatedOn"`
	Bucket       string        `json:"bucket"`
	Products     []string      `json:"products"`
	Totals       Totals        `json:"totals"`
	Trend        []TrendBucket `json:"trend"`
	ByProduct    []ProductRow  `json:"byProduct"`
	WocAgeing    WocAgeing     `json:"wocAgeing"`
	BlockedCases BlockedCases  `json:"blockedCases"`
	BreachedOpen []Item        `json:"breachedOpen"`
}

// Lead is one product lead and the products they lead.
type Lead struct {
	Email        string   `json:"email"`
	ProductNames []string `json:"productNames"`
}

// Config is the entity-service connection: the OAuth client credentials every
// other entity-service client in this component uses.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client fetches the report data.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client. Both URLs must be https.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("productissuereport: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("productissuereport: base URL: %w", err)
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

// Dashboard fetches one period's report. period is PeriodWeek or PeriodMonth;
// date (YYYY-MM-DD) names the week or month to cover; products filters by
// product name (empty means all).
//
// products travel as one comma-separated value, as the route defines, so a
// product name containing a comma cannot be expressed; ContainsComma lets the
// caller refuse rather than send a different filter.
func (c *Client) Dashboard(ctx context.Context, period, date string, products []string) (Dashboard, error) {
	q := url.Values{}
	q.Set("period", period)
	if date != "" {
		q.Set("date", date)
	}
	if len(products) > 0 {
		q.Set("products", strings.Join(products, ","))
	}
	var d Dashboard
	if err := c.get(ctx, "/reports/product-issues?"+q.Encode(), &d); err != nil {
		return Dashboard{}, err
	}
	return d, nil
}

// Recipients fetches the product leads and their products.
func (c *Client) Recipients(ctx context.Context) ([]Lead, error) {
	var out struct {
		Leads []Lead `json:"leads"`
	}
	if err := c.get(ctx, "/reports/product-issues/recipients", &out); err != nil {
		return nil, err
	}
	return out.Leads, nil
}

// ContainsComma reports whether any name has a comma, which the products
// parameter cannot carry.
func ContainsComma(names []string) bool {
	for _, n := range names {
		if strings.Contains(n, ",") {
			return true
		}
	}
	return false
}

func (c *Client) get(ctx context.Context, pathAndQuery string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+pathAndQuery, nil)
	if err != nil {
		return fmt.Errorf("productissuereport: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("productissuereport: fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("productissuereport: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("productissuereport: decode response: %w", err)
	}
	return nil
}
