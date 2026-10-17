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

// Package teamschedule is a narrow, read-only client for entity-service's Team
// Schedule API: the catalogue (shift windows and team names) and the rota
// itself. It exists for the rota reminder emails (internal/rotareminder) and is
// kept apart from internal/entitycases for the same reason that one is kept
// apart from internal/ledger: a different endpoint, a different concern, the
// same deployment and credentials.
//
// Every rota read asks entity-service to leave out people who are away
// (excludeAbsent), so a reminder can never tell somebody on leave that they are
// working. entity-service decides who is away, by the same rule the on-duty
// lookup that pages people uses; this client never second-guesses it.
package teamschedule

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

// Client reads the Team Schedule from entity-service. Same OAuth2 client
// credentials grant and httpsec guards as internal/entitycases.Client.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant. Both URLs must be https (loopback http is allowed for
// local development, see httpsec.RequireSecureURL).
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("teamschedule: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("teamschedule: base URL: %w", err)
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

	return newClient(httpClient, cfg.BaseURL), nil
}

// newClient builds a Client over an already-authenticated HTTP client. Tests
// use it to talk to an httptest server without a token endpoint.
func newClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{http: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

// do executes a request and returns the body, or an *apierror.Error for a
// non-2xx status.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("teamschedule: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teamschedule: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("teamschedule: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

// Shift is the part of a catalogue shift window a reminder needs.
type Shift struct {
	Code              string `json:"code"`
	Label             string `json:"label"`
	Family            string `json:"family"`
	AuthoringTimeZone string `json:"authoringTimeZone"`
	IsOnCall          bool   `json:"isOnCall"`
	SortOrder         int    `json:"sortOrder"`
}

// Team is one team the rota is run for.
type Team struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Catalogue is the part of GET /team-schedule/catalogue a reminder needs.
type Catalogue struct {
	Shifts []Shift `json:"shifts"`
	Teams  []Team  `json:"teams"`
}

// Engineer is the person an assignment belongs to.
type Engineer struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

// Assignment is one engineer, one rota day, one window.
type Assignment struct {
	ID        string    `json:"id"`
	Engineer  Engineer  `json:"engineer"`
	TeamKey   string    `json:"teamKey"`
	ShiftCode string    `json:"shiftCode"`
	RotaDate  string    `json:"rotaDate"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	IsOnCall  bool      `json:"isOnCall"`
	Note      string    `json:"note"`
}

// Catalogue reads the shift windows and teams.
func (c *Client) Catalogue(ctx context.Context) (Catalogue, error) {
	body, err := c.do(ctx, http.MethodGet, "/team-schedule/catalogue", nil)
	if err != nil {
		return Catalogue{}, err
	}
	var cat Catalogue
	if err := json.Unmarshal(body, &cat); err != nil {
		return Catalogue{}, fmt.Errorf("teamschedule: decode catalogue: %w", err)
	}
	return cat, nil
}

// searchRequest mirrors entity-service's SearchScheduleAssignmentsRequest,
// scoped to the fields this client sends.
type searchRequest struct {
	From          string `json:"from"`
	To            string `json:"to"`
	Family        string `json:"family,omitempty"`
	ExcludeAbsent bool   `json:"excludeAbsent"`
}

type searchResponse struct {
	Assignments []Assignment `json:"assignments"`
}

// WorkingAssignments returns the assignments of family whose rota day falls in
// [from, to] (YYYY-MM-DD, inclusive), leaving out everybody who is away on
// their day. An entity-service that predates excludeAbsent rejects the request
// (it refuses unknown fields), which fails the task loudly rather than
// reminding somebody on leave.
func (c *Client) WorkingAssignments(ctx context.Context, from, to, family string) ([]Assignment, error) {
	reqBody, err := json.Marshal(searchRequest{From: from, To: to, Family: family, ExcludeAbsent: true})
	if err != nil {
		return nil, fmt.Errorf("teamschedule: encode search: %w", err)
	}
	body, err := c.do(ctx, http.MethodPost, "/team-schedule/assignments/search", reqBody)
	if err != nil {
		return nil, err
	}
	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("teamschedule: decode assignments: %w", err)
	}
	return resp.Assignments, nil
}
