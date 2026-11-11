// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package alertcore is a minimal client for integrations/sre-alert-core-service. The CSM portal uses it to list which
// incidents an alert mute rule muted, and the values alerts carry in each field (to suggest when writing a rule); both
// live in alert-core's own database, not in entity-service.
package alertcore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// maxBody caps how much of a response is read; a page is at most 100 incidents.
const maxBody = 4 << 20

// Config holds the alert-core base URL and the shared token its read API expects (ALERT_CORE_API_TOKEN there).
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

// NewClient returns a Client for cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		http:    &http.Client{Timeout: 10 * time.Second},
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
	}
}

// MutedIncidents returns alert-core's JSON page of the incidents alert mute rule ruleID muted.
func (c *Client) MutedIncidents(ctx context.Context, ruleID string, limit, offset int) ([]byte, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}
	return c.get(ctx, "/alert-mute-rules/"+url.PathEscape(ruleID)+"/muted-incidents?"+q.Encode(), "muted incidents")
}

// FieldValues returns alert-core's JSON list of the values alerts carry in field that contain query.
func (c *Client) FieldValues(ctx context.Context, field, query string, limit int) ([]byte, error) {
	q := url.Values{"field": {field}, "q": {query}, "limit": {strconv.Itoa(limit)}}
	return c.get(ctx, "/alert-field-values?"+q.Encode(), "field values")
}

// get calls alert-core at pathAndQuery and returns the body of a 2xx answer; any other status is an *apierror.Error.
func (c *Client) get(ctx context.Context, pathAndQuery, what string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+pathAndQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("alertcore: build %s request: %w", what, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alertcore: %s: %w", what, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("alertcore: read %s: %w", what, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode}
	}
	return body, nil
}
