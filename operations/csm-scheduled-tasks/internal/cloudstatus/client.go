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

// Package cloudstatus delivers cloud status webhooks to WSO2's public uptime
// dashboards, replacing the legacy cloud status event notification workflow.
//
// The division of labour is the one this component uses everywhere:
// entity-service decides which outage transitions the dashboards are owed and
// records them; this package posts them and reports back what happened. It
// makes no decisions of its own -- it cannot tell whether an outage is in
// scope, and must not try.
//
// It is also this effort's FIRST OUTBOUND INTEGRATION. Every other ported
// task reads entity-service and sends mail through the internal email
// service; this one posts to endpoints outside the portal's trust boundary,
// authenticated by a shared secret. That is why the URL and secret handling
// below is more explicit than a comparable internal client's would be.
package cloudstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityhttp"
)

// requestTimeout bounds every entity-service request this client makes.
const requestTimeout = 25 * time.Second

// Config holds the entity-service client's configuration. Transport, when
// set, is a shared entityhttp transport; when nil the client builds its own
// from the credential fields.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	Transport    http.RoundTripper
}

// Client is a narrow entity-service client for the cloud status endpoints.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant.
func NewClient(cfg Config) (*Client, error) {
	httpClient, err := entityhttp.ClientFor(cfg.Transport, entityhttp.Credentials{
		TokenURL: cfg.TokenURL, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Scopes: cfg.Scopes,
	}, cfg.BaseURL, requestTimeout)
	if err != nil {
		return nil, fmt.Errorf("cloudstatus: %w", err)
	}
	return &Client{http: httpClient, baseURL: entityhttp.TrimBase(cfg.BaseURL)}, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return entityhttp.Do(ctx, c.http, c.baseURL, "cloudstatus", method, path, body)
}

// PendingWebhook is one webhook entity-service says is still owed.
type PendingWebhook struct {
	ID       string `json:"id"`
	OutageID string `json:"outageId"`
	Number   string `json:"number"`
	// Event is entity-service's own name for the transition, for logging.
	// It is NOT what goes on the wire -- see WireEvent.
	Event string `json:"event"`
	// WireEvent is the literal the dashboard expects in the body. Posting
	// Event instead sends "OUTAGE_BEGIN" where "outage_begin" is required,
	// which a dashboard accepts with a 200 and then ignores.
	WireEvent    string `json:"wireEvent"`
	Cloud        string `json:"cloud"`
	Timestamp    string `json:"timestamp"`
	AttemptCount int    `json:"attemptCount"`
	LastError    string `json:"lastError"`
}

type pendingResponse struct {
	Count    int              `json:"count"`
	Webhooks []PendingWebhook `json:"webhooks"`
}

// SweepResult reports what one decision sweep recorded.
type SweepResult struct {
	Scanned        int `json:"scanned"`
	Recorded       int `json:"recorded"`
	SkippedNoCloud int `json:"skippedNoCloud"`
}

// Sweep asks entity-service to re-derive which transitions are owed.
func (c *Client) Sweep(ctx context.Context) (SweepResult, error) {
	body, err := c.do(ctx, http.MethodPost, "/internal/cloud-status/sweep", nil)
	if err != nil {
		return SweepResult{}, err
	}
	var out SweepResult
	if err := json.Unmarshal(body, &out); err != nil {
		return SweepResult{}, fmt.Errorf("cloudstatus: decode sweep response: %w", err)
	}
	return out, nil
}

// Pending returns the webhooks still owed to the dashboards.
func (c *Client) Pending(ctx context.Context) ([]PendingWebhook, error) {
	body, err := c.do(ctx, http.MethodGet, "/internal/cloud-status/pending", nil)
	if err != nil {
		return nil, err
	}
	var out pendingResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("cloudstatus: decode pending response: %w", err)
	}
	return out.Webhooks, nil
}

type deliveryReport struct {
	Delivered bool   `json:"delivered"`
	Error     string `json:"error,omitempty"`
}

// RecordDelivery reports one attempt's outcome back to entity-service.
func (c *Client) RecordDelivery(ctx context.Context, id string, delivered bool, errMsg string) error {
	body, err := json.Marshal(deliveryReport{Delivered: delivered, Error: errMsg})
	if err != nil {
		return fmt.Errorf("cloudstatus: encode delivery report: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, "/internal/cloud-status/"+url.PathEscape(id)+"/delivery", body)
	return err
}
