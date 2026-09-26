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

// Package queryhours is a narrow client for entity-service's query-hour
// sweep endpoint (POST /query-hours/sweep).
//
// A fourth entity-service client in this component, for the reason
// internal/entitycases' doc comment gives: same deployment, unrelated
// concern. Query-hour consumption has nothing to do with case content,
// allocations, or this component's own scheduled_task_run state.
//
// WHY THIS IS A CRON AND NOT AN EVENT. ServiceNow's `[Query Hour] UpdateTime
// Card` is record-triggered: it fires the moment a time card is approved.
// There is no equivalent trigger in this stack, because nothing here writes
// `time_card` — csm-sync-service mirrors it in from ServiceNow, and
// entity-service's own time-card endpoints proxy straight to the ServiceNow
// API. Hooking entity-service's PATCH /time-cards/{id} would also race the
// sync: the recompute would read Postgres rows that the write it just made
// has not been mirrored into yet. A sweep over stale projects has neither
// problem and is naturally idempotent. When csm-sync-service grows a change
// feed, this becomes the fallback rather than the trigger.
package queryhours

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// sweepTimeout is generous because one sweep recomputes up to `limit`
// projects server-side, each an aggregate over that project's time cards.
const sweepTimeout = 45 * time.Second

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is a narrow HTTP client for entity-service's query-hour sweep.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant. Mirrors engagementallocations.NewClient exactly.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("queryhours: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("queryhours: base URL: %w", err)
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
	httpClient.Timeout = sweepTimeout
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// SweepResult is entity-service's wire shape for POST /query-hours/sweep.
type SweepResult struct {
	Requested int               `json:"requested"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Errors    map[string]string `json:"errors,omitempty"`
}

// Sweep recomputes up to limit projects whose stored position is older than
// staleFor, stalest first.
//
// A 2xx with per-project errors in the body is NOT an error here: the sweep
// deliberately continues past a failing project so it cannot stall on one.
// The caller decides what to do with SweepResult.Failed.
func (c *Client) Sweep(ctx context.Context, staleFor time.Duration, limit int) (SweepResult, error) {
	q := url.Values{}
	q.Set("staleForMinutes", strconv.Itoa(int(staleFor.Minutes())))
	q.Set("limit", strconv.Itoa(limit))
	path := "/query-hours/sweep?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return SweepResult{}, fmt.Errorf("queryhours: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return SweepResult{}, fmt.Errorf("queryhours: POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SweepResult{}, fmt.Errorf("queryhours: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return SweepResult{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var parsed SweepResult
	if err := json.Unmarshal(body, &parsed); err != nil {
		return SweepResult{}, fmt.Errorf("queryhours: decode response: %w", err)
	}
	return parsed, nil
}
