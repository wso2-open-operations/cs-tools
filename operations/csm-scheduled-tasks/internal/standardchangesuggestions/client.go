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

// Package standardchangesuggestions refreshes entity-service's standard change
// suggestion snapshot: the clusters of each customer's completed change
// requests that look like the same change done again, which the Standard
// changes page reads. Twice a day by default, so the page never computes them.
//
// *** A TRIGGER, NOT A DECIDER. *** The clustering, the criteria and the
// snapshot all live in entity-service (POST /standard-change-suggestions/refresh,
// internal/service/standard_change_suggestion_service.go). This package starts a
// run and keeps asking for the next page while entity-service says there is
// more; the last page publishes the run. The endpoint is for a service
// credential only: this component's client id must be in entity-service's
// M2M_CLIENT_IDS.
package standardchangesuggestions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
var tokenFetchTimeout = 10 * time.Second

// callTimeout bounds one page. entity-service stops starting customers after
// 40 seconds of a page and its REQUEST_TIMEOUT (60s by default) cancels the
// request, so waiting longer gains nothing. A page cut short fails the run but
// publishes nothing: the snapshot readers see is the previous one until a run
// finishes.
var callTimeout = 60 * time.Second

// ClientConfig holds this client's configuration.
type ClientConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client calls entity-service's POST /standard-change-suggestions/refresh with
// the same OAuth2 client-credentials grant and httpsec guards as the other
// clients here.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client. Both URLs must be https (loopback http is
// allowed for local development -- see httpsec.RequireSecureURL).
func NewClient(cfg ClientConfig) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("standardchangesuggestions: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("standardchangesuggestions: base URL: %w", err)
	}
	cc := clientcredentials.Config{
		ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, TokenURL: cfg.TokenURL, Scopes: cfg.Scopes,
	}
	tokenHTTP := &http.Client{Timeout: tokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTP)
	httpClient := cc.Client(context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTP))
	httpsec.RejectInsecureRedirects(httpClient)
	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// Request is the body of POST /standard-change-suggestions/refresh: empty to
// start a run, then the run and the previous page's cursor.
type Request struct {
	RunID  string `json:"runId,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// Totals are the published run's counts.
type Totals struct {
	Customers      int `json:"customers"`
	ChangeRequests int `json:"changeRequests"`
	Clusters       int `json:"clusters"`
	Eligible       int `json:"eligible"`
}

// Response is one page's answer. More: call again with RunID and NextCursor.
// When false the run was published (ComputedOn, Totals).
type Response struct {
	RunID          string  `json:"runId"`
	Customers      int     `json:"customers"`
	ChangeRequests int     `json:"changeRequests"`
	Suggestions    int     `json:"suggestions"`
	More           bool    `json:"more"`
	NextCursor     string  `json:"nextCursor"`
	ComputedOn     string  `json:"computedOn"`
	Totals         *Totals `json:"totals"`
}

// Refresh asks entity-service to compute one page.
func (c *Client) Refresh(ctx context.Context, req Request) (Response, error) {
	const path = "/standard-change-suggestions/refresh"
	var out Response
	body, err := json.Marshal(req)
	if err != nil {
		return out, fmt.Errorf("standardchangesuggestions: encode request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return out, fmt.Errorf("standardchangesuggestions: build request POST %s: %w", path, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return out, fmt.Errorf("standardchangesuggestions: POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, fmt.Errorf("standardchangesuggestions: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return out, &apierror.Error{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("standardchangesuggestions: decode response: %w", err)
	}
	return out, nil
}
