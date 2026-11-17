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

// Package alertcore is a client for sre-alert-core-service's read API: which
// incidents each alert mute rule muted, and the values real alerts carry in a
// field. This service exposes those reads to M2M callers (the CSM portal) behind
// the gateway, so alert-core itself needs no public endpoint.
package alertcore

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/entity"
)

// maxBody caps a response read from alert-core (one page is at most 100 incidents).
const maxBody = 1 << 20

// Config holds alert-core's base URL and the shared token its read API expects
// (ALERT_CORE_API_TOKEN on both services).
type Config struct {
	BaseURL string
	Token   string
}

// Client calls alert-core's read API with Authorization: Bearer <token>.
type Client struct {
	http    *http.Client
	baseURL string
	token   string
}

// NewClient returns a Client, or nil when either value is empty or the base URL
// is not https (http is allowed only for a loopback host, for local runs): the
// shared bearer token is never sent in plaintext. The routes that need the client
// then answer 503, so a missing or unsafe setting never fails startup.
func NewClient(cfg Config) *Client {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	token := strings.TrimSpace(cfg.Token)
	if base == "" || token == "" || !SafeBaseURL(base) {
		return nil
	}
	return &Client{
		http: &http.Client{
			Timeout: 10 * time.Second,
			// The token is never sent anywhere alert-core redirects to.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		baseURL: base,
		token:   token,
	}
}

// SafeBaseURL reports whether base may carry the bearer token: an absolute https
// URL, or http to a loopback host (localhost, 127.0.0.0/8, ::1).
func SafeBaseURL(base string) bool {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		if strings.EqualFold(host, "localhost") {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	default:
		return false
	}
}

// MutedIncidents calls GET /alert-mute-rules/{id}/muted-incidents.
func (c *Client) MutedIncidents(ctx context.Context, ruleID string, query url.Values) ([]byte, error) {
	return c.get(ctx, "/alert-mute-rules/"+url.PathEscape(ruleID)+"/muted-incidents", query)
}

// MutedCounts calls GET /alert-mute-rules/muted-counts.
func (c *Client) MutedCounts(ctx context.Context, query url.Values) ([]byte, error) {
	return c.get(ctx, "/alert-mute-rules/muted-counts", query)
}

// FieldValues calls GET /alert-field-values.
func (c *Client) FieldValues(ctx context.Context, query url.Values) ([]byte, error) {
	return c.get(ctx, "/alert-field-values", query)
}

// get returns alert-core's JSON body, or *apierror.Error for a non-2xx answer.
func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("alertcore: build request GET %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if id := entity.CorrelationIDFromContext(ctx); id != "" {
		req.Header.Set("X-CSM-Correlation-ID", id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alertcore: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("alertcore: read GET %s response: %w", path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 4096
		if len(body) > maxErrBody {
			body = body[:maxErrBody]
		}
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("alertcore: GET %s response is larger than %d bytes", path, maxBody)
	}
	return body, nil
}
