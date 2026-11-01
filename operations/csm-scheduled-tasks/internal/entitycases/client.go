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

// Package entitycases is a narrow, read-only client for entity-service's
// case-search API (POST /cases/search) — used by report-style sub-crons that
// need to look at live case data. Kept separate from internal/ledger, which
// is this component's own durable scheduled_task_run state and has nothing
// to do with case content; the two happen to point at the same entity-service
// deployment, but that's an operational detail, not a reason to share one
// client struct between two otherwise-unrelated concerns.
package entitycases

import (
	"bytes"
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

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is a narrow HTTP client for entity-service's case-search endpoint.
// Mirrors internal/ledger.Client's shape exactly (same OAuth2 client
// credentials grant, same httpsec guards) — see that package's own doc
// comment for why this isn't just a second method set on Client there.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant. cfg.TokenURL/cfg.BaseURL must both be https (loopback
// http is allowed for local development — see httpsec.RequireSecureURL)
// since both carry credentials or a bearer token.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("entitycases: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("entitycases: base URL: %w", err)
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
	httpClient.Timeout = 25 * time.Second
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}, nil
}

// do executes an authenticated HTTP request against entity-service and
// returns the raw JSON response body, or an *apierror.Error for a non-2xx
// status.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("entitycases: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entitycases: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("entitycases: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

// Case is the report-relevant subset of entity-service's case-search result
// (domain.SearchCaseView there). Account and AssignedTo are "" when
// entity-service returns no value for that case — an unassigned case, or a
// data source that doesn't populate account details (see entity-service's
// own CLAUDE.md on DATA_SOURCE=servicenow-gated fields) — rather than a
// pointer a caller has to nil-check.
type Case struct {
	ID         string
	Number     string
	InternalID string
	Subject    string
	State      string
	Severity   string
	Account    string
	AssignedTo string
	CreatedOn  time.Time
	UpdatedOn  time.Time
}

// searchPageSize is the page size searchCases requests — entity-service's
// own case-search caps limit at 50 (see that service's normalizePagination),
// so this just always asks for the largest page allowed, to fetch the fewest
// pages possible.
const searchPageSize = 50

// maxSearchPages caps how many pages searchCases will fetch, as a safety net
// against a huge or unexpectedly-shaped result turning one report run into
// an unbounded loop. 20 pages at searchPageSize is 1,000 cases — a report
// like this anywhere near that size has bigger problems than this cap, but
// the cap keeps a malformed response (e.g. Total always greater than what's
// actually returned) from looping forever rather than just producing an
// incomplete report.
const maxSearchPages = 20

// searchCasesRequest/searchCasesFilters/caseFieldFilter/caseSort/pagination
// mirror entity-service's own domain.SearchCasesRequest wire shape exactly
// (see that service's openapi.yaml, "/cases/search") — kept private and
// scoped to exactly the fields this client uses, rather than importing or
// duplicating that service's full domain package.
type caseFieldFilter struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values,omitempty"`
}

type searchCasesFilters struct {
	Filters []caseFieldFilter `json:"filters,omitempty"`
}

type caseSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type searchCasesRequest struct {
	Filters    searchCasesFilters `json:"filters"`
	SortBy     caseSort           `json:"sortBy"`
	Pagination pagination         `json:"pagination"`
}

type userReference struct {
	Name string `json:"name"`
}

type accountRef struct {
	Name string `json:"name"`
}

type searchCaseView struct {
	ID               string         `json:"id"`
	Number           string         `json:"number"`
	InternalID       string         `json:"internalId"`
	Subject          *string        `json:"subject"`
	State            string         `json:"state"`
	Severity         *string        `json:"severity"`
	CreatedOn        string         `json:"createdOn"`
	UpdatedOn        string         `json:"updatedOn"`
	AssignedEngineer *userReference `json:"assignedEngineer"`
	AccountDetails   *accountRef    `json:"account"`
}

// caseDateTimeLayouts are tried in order to parse a case-search
// createdOn/updatedOn value. entity-service's own domain.SearchCaseView
// declares these as plain strings, not a guaranteed RFC3339 shape: the
// ServiceNow-backed data source passes ServiceNow's raw datetime fields
// straight through unreformatted (see entity-service's own
// internal/service.parseSNDateTime and snCreatedOnLayout/snAltCreatedOnLayout
// — root-caused there to a GlideRecord.getDisplayValue() vs getValue() bug on
// the ServiceNow side that occasionally renders a locale-formatted date
// instead of canonical ISO). This client tries the same two layouts for the
// same reason, plus RFC3339 first for a Postgres-backed data source.
var caseDateTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"01-02-2006 15:04:05",
}

// parseCaseDateTime parses value against caseDateTimeLayouts in order,
// returning the first successful result. A value in any of these formats
// with no explicit zone offset is treated as UTC, matching
// entity-service's own parseSNDateTime (time.Parse with no zone abbreviation
// in the layout defaults to UTC).
func parseCaseDateTime(value string) (time.Time, error) {
	var lastErr error
	for _, layout := range caseDateTimeLayouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	return time.Time{}, fmt.Errorf("unrecognized date-time format %q: %w", value, lastErr)
}

type searchCasesResponse struct {
	Cases  []searchCaseView `json:"cases"`
	Total  int              `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
}

// SearchOpenCasesOlderThan returns every case whose state is not "closed"
// and whose createdOn is at least olderThan in the past — the entity-service
// query behind an "open too long" report. Sorted oldest-first (createdOn
// ascending), so a caller building a report table can just take the
// returned order as given.
//
// Unlike SearchCasesInStateCreatedBeforeYesterday, a result set larger than
// the pagination cap is not an error: the backlog this query covers (every
// non-closed case, including long-abandoned ones) can legitimately be far
// larger than any email is useful at. It returns the oldest
// maxSearchPages*searchPageSize cases plus total, the true match count, so
// the caller can say "showing N of total" rather than fail every run.
// total == len(cases) when nothing was cut off.
func (c *Client) SearchOpenCasesOlderThan(ctx context.Context, olderThan time.Duration) (cases []Case, total int, err error) {
	cutoff := time.Now().Add(-olderThan).UTC().Format(time.RFC3339)
	return c.searchCases(ctx, []caseFieldFilter{
		{Field: "state", Op: "notIn", Values: []string{"closed"}},
		{Field: "createdOn", Op: "lte", Values: []string{cutoff}},
	})
}

// SearchCasesInStateCreatedBeforeYesterday returns every case whose state is
// exactly state (not "not closed" — an exact match, e.g. "open") and whose
// createdOn falls before the start of yesterday. Unlike
// SearchOpenCasesOlderThan's rolling-duration cutoff, this is a calendar-day
// boundary computed in UTC: a case created at 23:59 yesterday is excluded, one
// created at 00:01 the day before is included, regardless of what time of day
// this method itself is called — "before yesterday" is a calendar concept,
// not a fixed number of hours ago. Sorted oldest-first, same as
// SearchOpenCasesOlderThan.
//
// Errors, rather than returning a partial list, if the result set exceeds the
// pagination cap — see searchCases.
func (c *Client) SearchCasesInStateCreatedBeforeYesterday(ctx context.Context, state string) ([]Case, error) {
	startOfToday := time.Now().UTC().Truncate(24 * time.Hour)
	startOfYesterday := startOfToday.Add(-24 * time.Hour)
	cutoff := startOfYesterday.Format(time.RFC3339)
	cases, total, err := c.searchCases(ctx, []caseFieldFilter{
		{Field: "state", Op: "in", Values: []string{state}},
		{Field: "createdOn", Op: "lte", Values: []string{cutoff}},
	})
	if err != nil {
		return nil, err
	}
	if total > len(cases) {
		return nil, fmt.Errorf("entitycases: search results exceed the %d-row pagination cap (total=%d) — refusing to return a silently-incomplete report",
			maxSearchPages*searchPageSize, total)
	}
	return cases, nil
}

// searchCases runs one case-search query to completion, paginating through
// entity-service's own 50-row page cap internally (see
// searchPageSize/maxSearchPages) and returning one flat, oldest-first slice.
//
// total is entity-service's reported match count. If it exceeds
// maxSearchPages*searchPageSize, the returned slice holds only the first
// maxSearchPages*searchPageSize cases and total > len(cases): callers must
// compare the two and either surface the truncation or error, never present
// the slice as complete.
func (c *Client) searchCases(ctx context.Context, filters []caseFieldFilter) ([]Case, int, error) {
	var all []Case
	offset := 0
	for page := 0; page < maxSearchPages; page++ {
		reqBody, err := json.Marshal(searchCasesRequest{
			Filters:    searchCasesFilters{Filters: filters},
			SortBy:     caseSort{Field: "createdOn", Order: "asc"},
			Pagination: pagination{Limit: searchPageSize, Offset: offset},
		})
		if err != nil {
			return nil, 0, fmt.Errorf("entitycases: encode search request: %w", err)
		}

		respBody, err := c.do(ctx, http.MethodPost, "/cases/search", reqBody)
		if err != nil {
			return nil, 0, err
		}
		var resp searchCasesResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, 0, fmt.Errorf("entitycases: decode search response: %w", err)
		}

		for _, cv := range resp.Cases {
			c, err := toCase(cv)
			if err != nil {
				return nil, 0, fmt.Errorf("entitycases: case %s: %w", cv.ID, err)
			}
			all = append(all, c)
		}

		offset += searchPageSize
		if len(resp.Cases) == 0 || offset >= resp.Total {
			return all, len(all), nil
		}
		if page == maxSearchPages-1 {
			return all, resp.Total, nil
		}
	}
	// Unreachable: every iteration above returns before the loop can exit on
	// its own condition. Required only because the compiler can't prove that.
	return all, len(all), nil
}

func toCase(cv searchCaseView) (Case, error) {
	createdOn, err := parseCaseDateTime(cv.CreatedOn)
	if err != nil {
		return Case{}, fmt.Errorf("createdOn: %w", err)
	}
	updatedOn, err := parseCaseDateTime(cv.UpdatedOn)
	if err != nil {
		return Case{}, fmt.Errorf("updatedOn: %w", err)
	}

	c := Case{
		ID:         cv.ID,
		Number:     cv.Number,
		InternalID: cv.InternalID,
		State:      cv.State,
		CreatedOn:  createdOn,
		UpdatedOn:  updatedOn,
	}
	if cv.Subject != nil {
		c.Subject = *cv.Subject
	}
	if cv.Severity != nil {
		c.Severity = *cv.Severity
	}
	if cv.AssignedEngineer != nil {
		c.AssignedTo = cv.AssignedEngineer.Name
	}
	if cv.AccountDetails != nil {
		c.Account = cv.AccountDetails.Name
	}
	return c, nil
}
