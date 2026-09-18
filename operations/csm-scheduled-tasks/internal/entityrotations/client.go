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

// Package entityrotations is a narrow, read-only client for entity-service's
// support-rotation roster API (POST /schedule-spans/search) — who is on duty
// for a given shift on a given date. Kept separate from internal/entitycases
// and internal/ledger for the same reason those two are separate from each
// other: they point at the same entity-service deployment, but the roster,
// case content, and this component's own scheduled_task_run state are three
// unrelated concerns, and sharing one client struct between them would couple
// them for no gain beyond saving a constructor.
//
// The roster rows themselves are not native: csm-sync-service
// (wso2-enterprise/digiops-cs) replicates cmn_schedule / agent_events /
// cmn_schedule_span from ServiceNow into schedule / user_schedule /
// schedule_span, and entity-service reads those. This client neither knows nor
// cares — it sees the same entity-service HTTP surface as every other sub-cron
// here.
package entityrotations

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

// Span type values, mirroring entity-service's schedule_span_type_enum (itself
// a mirror of ServiceNow's cmn_schedule_span.type). Only the weekend pair is
// named here because that is all this component asks for today; add the others
// when a sub-cron needs them.
const (
	TypeWeekend      = "ops_weekend"
	TypeWeekendNight = "ops_weekend_night"
)

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is a narrow HTTP client for entity-service's rotation-roster
// endpoint. Mirrors internal/entitycases.Client's shape exactly — same OAuth2
// client credentials grant, same httpsec guards.
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
		return nil, fmt.Errorf("entityrotations: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("entityrotations: base URL: %w", err)
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

// Member is one person on duty for one shift on one date.
type Member struct {
	// Type is the shift covered, e.g. "ops_weekend".
	Type string
	// Date is the day covered, as YYYY-MM-DD -- the date half of the span's
	// startOn. entity-service returns a wall-clock timestamp in the schedule's
	// own zone; the notice only ever needs the day.
	Date string
	Name string
	// Email is what the notice is actually sent to. Never empty in practice:
	// entity-service joins the roster to users, whose email column is NOT
	// NULL — but SendNotice skips a member without one rather than mailing
	// the empty string.
	Email string
	Notes string
}

// searchRequest/searchResponse mirror entity-service's own
// domain.SearchTeamRotationsRequest/Response wire shape.
type searchRequest struct {
	SpanTypes []string `json:"spanTypes"`
	Date      string   `json:"date"`
}

type searchResponse struct {
	Spans []struct {
		SpanType string `json:"spanType"`
		StartOn  string `json:"startOn"`
		Notes    string `json:"notes"`
		User     *struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"user"`
	} `json:"spans"`
	Total int `json:"total"`
}

// SearchRotations returns everyone rostered for any of spanTypes on date
// (YYYY-MM-DD). An unstaffed shift comes back as an empty slice, not an error.
func (c *Client) SearchRotations(ctx context.Context, spanTypes []string, date string) ([]Member, error) {
	reqBody, err := json.Marshal(searchRequest{SpanTypes: spanTypes, Date: date})
	if err != nil {
		return nil, fmt.Errorf("entityrotations: marshal request: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/schedule-spans/search", reqBody)
	if err != nil {
		return nil, err
	}

	var parsed searchResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("entityrotations: decode response: %w", err)
	}

	members := make([]Member, 0, len(parsed.Spans))
	for _, sp := range parsed.Spans {
		// startOn is "YYYY-MM-DDTHH:MM:SS"; the notice only needs the day, and
		// slicing avoids parsing a deliberately offset-free local timestamp as
		// though it were an instant.
		day := sp.StartOn
		if len(day) >= 10 {
			day = day[:10]
		}
		m := Member{Type: sp.SpanType, Date: day, Notes: sp.Notes}
		if sp.User != nil {
			m.Name, m.Email = sp.User.Name, sp.User.Email
		}
		members = append(members, m)
	}
	return members, nil
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
		return nil, fmt.Errorf("entityrotations: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entityrotations: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("entityrotations: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}
