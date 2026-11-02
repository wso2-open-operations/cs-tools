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

// Package csmai is the client for integrations/csm-ai-integration-service,
// the service that holds every model call of the CSM platform. This backend
// holds no model client of its own: the dashboard generator sends the
// prompt, the draft and the team list there and judges what comes back
// (handler.serviceDashboardGenerator). It also backs GET
// /health/dependencies.
package csmai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests,
// kept within handler.dependencyHealthCheckTimeout (5s) so a health check
// with a cold token still fits its budget.
var tokenFetchTimeout = 5 * time.Second

// maxResponseBytes caps a response read from the service (a generated
// dashboard is far below it).
const maxResponseBytes = 4 << 20

// Error codes the service answers with (its "code"). See the service's
// CLAUDE.md for when each is given.
const (
	CodeInvalidRequest  = "invalid_request"
	CodeNotConfigured   = "not_configured"
	CodeRefused         = "refused"
	CodeTruncated       = "truncated"
	CodeUnusable        = "unusable"
	CodeTimeout         = "timeout"
	CodeModelCallFailed = "model_call_failed"
)

// Config holds the client's configuration. Timeout bounds one generate call
// as a whole and must leave room for the service's own model timeout.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	Timeout      time.Duration
}

// Client calls the service, authenticated with the OAuth2 client credentials
// grant (the shared app every upstream client of this backend uses).
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient returns a Client for cfg.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = cfg.Timeout
	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}
}

// Error is the service answering a status other than 200. Code is its
// error code ("" when the body carried none, e.g. a gateway's own error).
// The service's message is not kept: nothing of it is shown to a user.
type Error struct {
	StatusCode int
	Code       string
}

// Error names the status and, when the service sent one, its error code.
func (e *Error) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("csmai: status %d", e.StatusCode)
	}
	return fmt.Sprintf("csmai: status %d (%s)", e.StatusCode, e.Code)
}

// ErrCall is the request not getting an answer at all (connection failed,
// timed out, unreadable response). Wraps the transport error.
var ErrCall = errors.New("csmai: call failed")

// Team is one team as the model is told about it.
type Team struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Family string `json:"family"`
	Field  string `json:"field"`
}

// DashboardRequest is one chat message of the JSON generator. Draft is the
// dashboard being edited in the definition file format, or nil.
type DashboardRequest struct {
	Prompt string          `json:"prompt"`
	Draft  json.RawMessage `json:"draft,omitempty"`
	Teams  []Team          `json:"teams"`
}

// Usage is the model call's token counts.
type Usage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
}

// DashboardResult is the model's answer: its decline sentence, or every JSON
// object in its reply, in order.
type DashboardResult struct {
	Declined   string            `json:"declined"`
	Candidates []json.RawMessage `json:"candidates"`
	Usage      Usage             `json:"usage"`
}

// GenerateDashboard makes one POST /ai/dashboards/generate.
func (c *Client) GenerateDashboard(ctx context.Context, req DashboardRequest) (DashboardResult, error) {
	if req.Teams == nil {
		req.Teams = []Team{}
	}
	var res DashboardResult
	err := c.post(ctx, "/ai/dashboards/generate", req, &res)
	return res, err
}

// Health calls the service's GET /health and returns an error unless it
// answers 200.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("csmai: build health request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: health check: %w", ErrCall, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return &Error{StatusCode: resp.StatusCode}
	}
	return nil
}

// post sends body as JSON to path and decodes a 200 into out. The request
// carries the caller's correlation id, so one chat message can be followed
// across both services' logs, and the caller's user id, so the service can log
// each model call's token usage against the user. The email is never sent.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("csmai: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("csmai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if id := middleware.CorrelationIDFromContext(ctx); id != "" {
		req.Header.Set("X-CSM-Correlation-ID", id)
	}
	if user := middleware.UserInfoFromContext(ctx); user != nil && user.UserID != "" {
		req.Header.Set("X-CSM-User-ID", user.UserID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCall, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("%w: read response: %w", ErrCall, err)
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(data, &e)
		return &Error{StatusCode: resp.StatusCode, Code: e.Code}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%w: decode response: %w", ErrCall, err)
	}
	return nil
}
