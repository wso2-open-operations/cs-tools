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

package queryhours

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

var tokenFetchTimeout = 10 * time.Second

// maxResponseBytes caps the report body. It lists only breaching
// accounts, so it is small; anything past this is a bug upstream.
const maxResponseBytes = 8 << 20

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is a narrow client for entity-service's query-hour endpoints.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via client credentials.
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
	// A recompute walks every opportunity in service and every project
	// linked to each, so it is allowed noticeably longer than a read.
	//
	// THIS BUDGET IS A CEILING, NOT A GUARANTEE. entity-service cancels
	// each request at its own REQUEST_TIMEOUT and stops writing at
	// SERVER_WRITE_TIMEOUT, both of which default to 60s -- so a
	// recompute that runs longer than a minute is cut off server-side
	// and this client never gets near five minutes. Raise both on the
	// entity-service deployment if recomputes start timing out; do not
	// lower this value in response, which would only hide where the
	// limit really is.
	httpClient.Timeout = 5 * time.Minute
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// WeeklyReport fetches the exceeded and close-to-exceeding accounts.
func (c *Client) WeeklyReport(ctx context.Context) (Report, error) {
	var out Report
	if err := c.do(ctx, http.MethodGet, "/reports/query-hours", nil, &out); err != nil {
		return Report{}, err
	}
	return out, nil
}

// Recompute rebuilds utilisation and returns the threshold crossings.
func (c *Client) Recompute(ctx context.Context) ([]Transition, error) {
	var out RecomputeResponse
	if err := c.do(ctx, http.MethodPost, "/query-hours/recompute", []byte("{}"), &out); err != nil {
		return nil, err
	}
	return out.Transitions, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, into any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("queryhours: build %s %s: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("queryhours: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("queryhours: read %s %s: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	if into == nil {
		return nil
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("queryhours: decode %s %s: %w", method, path, err)
	}
	return nil
}
