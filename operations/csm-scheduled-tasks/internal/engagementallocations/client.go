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

// Package engagementallocations is a narrow, read-only client for
// entity-service's engagement status-update reminder endpoint
// (GET /engagement-allocations/status-update-reminders).
//
// A third entity-service client in this component, alongside internal/ledger
// and internal/entitycases, for the reason entitycases' own doc comment
// gives: they point at the same deployment, but "who owes a weekly status
// update" has nothing to do with case content or with this component's own
// scheduled_task_run state, and folding three unrelated concerns into one
// client struct buys nothing.
package engagementallocations

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

// Client is a narrow HTTP client for entity-service's status-update reminder
// endpoint. Mirrors internal/entitycases.Client's shape exactly.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant. cfg.TokenURL/cfg.BaseURL must both be https (loopback
// http is allowed for local development — see httpsec.RequireSecureURL).
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("engagementallocations: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("engagementallocations: base URL: %w", err)
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

// Recipient is one person who owes a status update.
type Recipient struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// reminderResponse is entity-service's wire shape.
type reminderResponse struct {
	CycleStartDate string      `json:"cycleStartDate"`
	Count          int         `json:"count"`
	Recipients     []Recipient `json:"recipients"`
}

// cycleDateLayout is the date form entity-service's cycleStartDate parameter
// accepts.
const cycleDateLayout = "2006-01-02"

// StatusUpdateReminderRecipients returns everyone who owes a weekly status
// update for the cycle beginning cycleStart.
//
// Only the date part of cycleStart is sent: the underlying columns are DATE,
// and the endpoint rejects anything but YYYY-MM-DD.
func (c *Client) StatusUpdateReminderRecipients(ctx context.Context, cycleStart time.Time) ([]Recipient, error) {
	q := url.Values{}
	q.Set("cycleStartDate", cycleStart.Format(cycleDateLayout))
	path := "/engagement-allocations/status-update-reminders?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("engagementallocations: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("engagementallocations: GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("engagementallocations: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var parsed reminderResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("engagementallocations: decode response: %w", err)
	}
	return parsed.Recipients, nil
}
