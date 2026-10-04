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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityhttp"
)

// requestTimeout bounds every request this client makes.
const requestTimeout = 25 * time.Second

// Config holds this client's configuration. Transport, when set, is a
// shared entityhttp transport; when nil the client builds its own from the
// credential fields (see internal/ledger.Config).
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	Transport    http.RoundTripper
}

// Client is a narrow HTTP client for entity-service's case-search endpoint
// — see this package's doc comment for why this isn't just a second method
// set on internal/ledger.Client.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant. cfg.TokenURL/cfg.BaseURL must both be https (loopback
// http is allowed for local development — see httpsec.RequireSecureURL)
// since both carry credentials or a bearer token.
func NewClient(cfg Config) (*Client, error) {
	httpClient, err := entityhttp.ClientFor(cfg.Transport, entityhttp.Credentials{
		TokenURL: cfg.TokenURL, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Scopes: cfg.Scopes,
	}, cfg.BaseURL, requestTimeout)
	if err != nil {
		return nil, fmt.Errorf("entitycases: %w", err)
	}
	return &Client{http: httpClient, baseURL: entityhttp.TrimBase(cfg.BaseURL)}, nil
}

// do executes an authenticated request — see entityhttp.Do.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return entityhttp.Do(ctx, c.http, c.baseURL, "entitycases", method, path, body)
}

// Case is the report-relevant subset of entity-service's case-search result
// (domain.SearchCaseView there). Account and AssignedTo are "" when
// entity-service returns no value for that case — an unassigned case, or a
// data source that doesn't populate account details (see entity-service's
// own CLAUDE.md on data-source-gated fields) — rather than a
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
// legacy backing data source passes its raw datetime fields straight through
// unreformatted, and occasionally renders a locale-formatted date instead of
// canonical ISO (entity-service's own adapter for that data source accepts
// the same two layouts). This client tries those two layouts for the same
// reason, plus RFC3339 first for a Postgres-backed data source.
var caseDateTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"01-02-2006 15:04:05",
}

// parseCaseDateTime parses value against caseDateTimeLayouts in order,
// returning the first successful result. A value in any of these formats
// with no explicit zone offset is treated as UTC, matching
// entity-service's own data-source adapter (time.Parse with no zone
// abbreviation in the layout defaults to UTC).
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
func (c *Client) SearchOpenCasesOlderThan(ctx context.Context, olderThan time.Duration) ([]Case, error) {
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
func (c *Client) SearchCasesInStateCreatedBeforeYesterday(ctx context.Context, state string) ([]Case, error) {
	startOfToday := time.Now().UTC().Truncate(24 * time.Hour)
	startOfYesterday := startOfToday.Add(-24 * time.Hour)
	cutoff := startOfYesterday.Format(time.RFC3339)
	return c.searchCases(ctx, []caseFieldFilter{
		{Field: "state", Op: "in", Values: []string{state}},
		{Field: "createdOn", Op: "lte", Values: []string{cutoff}},
	})
}

// searchCases runs one case-search query to completion, paginating through
// entity-service's own 50-row page cap internally (see
// searchPageSize/maxSearchPages) and returning one flat, oldest-first slice.
//
// Returns an error, rather than a silently-truncated slice, if the result
// set is still larger than maxSearchPages*searchPageSize after the last
// allowed page — a caller building a report from a partial result would
// otherwise under-report without any indication it happened.
func (c *Client) searchCases(ctx context.Context, filters []caseFieldFilter) ([]Case, error) {
	var all []Case
	offset := 0
	for page := 0; page < maxSearchPages; page++ {
		reqBody, err := json.Marshal(searchCasesRequest{
			Filters:    searchCasesFilters{Filters: filters},
			SortBy:     caseSort{Field: "createdOn", Order: "asc"},
			Pagination: pagination{Limit: searchPageSize, Offset: offset},
		})
		if err != nil {
			return nil, fmt.Errorf("entitycases: encode search request: %w", err)
		}

		respBody, err := c.do(ctx, http.MethodPost, "/cases/search", reqBody)
		if err != nil {
			return nil, err
		}
		var resp searchCasesResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("entitycases: decode search response: %w", err)
		}

		for _, cv := range resp.Cases {
			c, err := toCase(cv)
			if err != nil {
				return nil, fmt.Errorf("entitycases: case %s: %w", cv.ID, err)
			}
			all = append(all, c)
		}

		// Advance by what actually came back, not by what was asked for: a
		// server that returns a short non-final page (a lowered page cap)
		// would otherwise have the rows in between skipped silently.
		offset += len(resp.Cases)
		if entityhttp.PageDone(len(resp.Cases), offset, resp.Total, searchPageSize) {
			return all, nil
		}
		if page == maxSearchPages-1 {
			return nil, fmt.Errorf("entitycases: search results exceed the %d-row pagination cap (total=%d) — refusing to return a silently-incomplete report",
				maxSearchPages*searchPageSize, resp.Total)
		}
	}
	// Unreachable: every iteration above returns before the loop can exit on
	// its own condition. Required only because the compiler can't prove that.
	return all, nil
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
