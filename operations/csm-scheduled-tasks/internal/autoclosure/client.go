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

// Package autoclosure drives entity-service's case auto-closure workflow:
// reminders and automatic closure for tickets that sit in Awaiting Info or
// Solution Proposed with no activity. Everything that decides anything -- which
// tickets are eligible, whether one is still due, what the comment says, how a
// ticket is closed -- lives in entity-service (see its CLAUDE.md, "Case
// auto-closure"); this package only asks for what is due, stage by stage, and
// advances each ticket, so that logic has one home and one set of tests.
package autoclosure

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

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// callTimeout bounds one entity-service call. An advance posts a comment and,
// for a closure, closes a case, which fans out to events and SLA clocks, so it
// gets more than a plain read.
var callTimeout = 60 * time.Second

// ClientConfig holds this client's configuration.
type ClientConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is a narrow HTTP client for entity-service's two auto-closure routes.
// Same OAuth2 client-credentials grant and httpsec guards as the other clients
// here; its Bearer token must be one entity-service treats as internal
// (M2M_CLIENT_IDS) -- the routes refuse anyone else.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client. Both URLs must be https (loopback http is
// allowed for local development -- see httpsec.RequireSecureURL).
func NewClient(cfg ClientConfig) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("autoclosure: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("autoclosure: base URL: %w", err)
	}
	cc := clientcredentials.Config{
		ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, TokenURL: cfg.TokenURL, Scopes: cfg.Scopes,
	}
	tokenHTTPClient := &http.Client{Timeout: tokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTPClient)
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTPClient)
	httpClient := cc.Client(tokenCtx)
	httpsec.RejectInsecureRedirects(httpClient)
	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// Candidate is one ticket due for a stage.
type Candidate struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	State  string `json:"state"`
}

// CandidatesRequest asks which tickets are due for a stage.
type CandidatesRequest struct {
	Stage        string   `json:"stage"`
	IntervalDays int      `json:"intervalDays,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	DailyLimit   int      `json:"dailyLimit,omitempty"`
	States       []string `json:"states,omitempty"`
}

// AdvanceRequest performs one stage on one ticket.
type AdvanceRequest struct {
	Stage        string            `json:"stage"`
	IntervalDays int               `json:"intervalDays,omitempty"`
	States       []string          `json:"states,omitempty"`
	Templates    map[string]string `json:"templates,omitempty"`
	// DailyLimit is the per-UTC-day budget entity-service checks again right
	// before it claims the ticket (0 = none).
	DailyLimit int `json:"dailyLimit,omitempty"`
	// DryRun asks entity-service to decide and record, but change nothing.
	DryRun bool `json:"dryRun,omitempty"`
}

// Outcome of an advance: "advanced", "skipped" or, for a dry run, "dry_run"
// (the ticket would have been advanced).
type Outcome string

const (
	OutcomeAdvanced Outcome = "advanced"
	OutcomeSkipped  Outcome = "skipped"
	OutcomeDryRun   Outcome = "dry_run"
)

func (c *Client) post(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("autoclosure: encode request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("autoclosure: build request POST %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("autoclosure: POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("autoclosure: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("autoclosure: decode response: %w", err)
	}
	return nil
}

// Candidates returns the tickets due for a stage, oldest first, and how many
// more entity-service left out because they failed the stage repeatedly.
func (c *Client) Candidates(ctx context.Context, req CandidatesRequest) ([]Candidate, int, error) {
	var resp struct {
		Candidates  []Candidate `json:"candidates"`
		Quarantined int         `json:"quarantined"`
	}
	if err := c.post(ctx, "/autoclosure/candidates", req, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Candidates, resp.Quarantined, nil
}

// Advance performs a stage on one ticket and reports what happened.
func (c *Client) Advance(ctx context.Context, id string, req AdvanceRequest) (Outcome, error) {
	var resp struct {
		Outcome Outcome `json:"outcome"`
	}
	if err := c.post(ctx, "/autoclosure/cases/"+url.PathEscape(id)+"/advance", req, &resp); err != nil {
		return "", err
	}
	return resp.Outcome, nil
}
