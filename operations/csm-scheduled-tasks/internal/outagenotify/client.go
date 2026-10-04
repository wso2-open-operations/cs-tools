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

// Package outagenotify is a narrow client for entity-service's outage
// internal-notification sweep, plus the wire types it returns.
//
// The replacement for the legacy internal-stakeholder outage e-mail workflow.
// A sweep rather than a record trigger because nothing in this stack writes
// `outage` — csm-sync-service mirrors it in from the upstream data source, so
// there is no local write to react to.
package outagenotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityhttp"
)

// sweepTimeout bounds one sweep. The scan is small — only outages opted into
// notification and not yet resolved — so a slow one means trouble, not volume.
const sweepTimeout = 60 * time.Second

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	// Transport, when set, is a shared entityhttp transport; when nil the
	// client builds its own from the credential fields.
	Transport http.RoundTripper
}

// Client calls entity-service's outage notification sweep.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the client credentials grant.
func NewClient(cfg Config) (*Client, error) {
	httpClient, err := entityhttp.ClientFor(cfg.Transport, entityhttp.Credentials{
		TokenURL: cfg.TokenURL, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Scopes: cfg.Scopes,
	}, cfg.BaseURL, sweepTimeout)
	if err != nil {
		return nil, fmt.Errorf("outagenotify: %w", err)
	}
	return &Client{http: httpClient, baseURL: entityhttp.TrimBase(cfg.BaseURL)}, nil
}

// Decision is one outage's evaluation: which email is due, and its rendered
// subject and body.
type Decision struct {
	OutageID string `json:"outageId"`
	Number   string `json:"number"`
	Kind     string `json:"kind"`
	Reason   string `json:"reason"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
}

// SweepResult is entity-service's wire shape.
type SweepResult struct {
	Evaluated int               `json:"evaluated"`
	Decisions []Decision        `json:"decisions"`
	Errors    map[string]string `json:"errors,omitempty"`
}

// Sweep asks entity-service which outage emails are due.
//
// *** THE SWEEP RECORDS BEFORE IT RETURNS. *** Each decision is written down
// server-side as sent before this client ever sees it, so calling Sweep twice
// does not yield the same email twice — but it also means a decision this
// caller fails to deliver is LOST rather than retried. That is deliberate: a
// duplicate outage notice to the whole internal audience is worse than a
// missed one. See entity-service's Sweep doc comment.
func (c *Client) Sweep(ctx context.Context, limit int) (SweepResult, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/outage-notifications/sweep"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	body, err := entityhttp.Do(ctx, c.http, c.baseURL, "outagenotify", http.MethodPost, path, nil)
	if err != nil {
		return SweepResult{}, err
	}
	var parsed SweepResult
	if err := json.Unmarshal(body, &parsed); err != nil {
		return SweepResult{}, fmt.Errorf("outagenotify: decode response: %w", err)
	}
	return parsed, nil
}
