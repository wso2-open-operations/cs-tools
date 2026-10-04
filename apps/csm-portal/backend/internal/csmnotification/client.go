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

// Package csmnotification is a minimal client for
// integrations/csm-notification-service, today used only to back the
// portal's aggregating GET /health/dependencies check. That service's own
// GET /health has no dependency of its own (a plain 200), so there is
// nothing else for this client to call yet.
package csmnotification

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/upstreamhttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Kept within handler.dependencyHealthCheckTimeout (5s) — Health is this
// client's only method, called under that per-dependency budget, and a cold
// or expired token would otherwise be able to acquire a fresh one past that
// deadline. Overridden in tests to keep them fast.
var tokenFetchTimeout = 5 * time.Second

// Config holds the configuration for the csm-notification-service client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is an HTTP client authenticated via the OAuth2 client credentials
// grant. Tokens are acquired and refreshed automatically; callers need not
// manage them.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client that authenticates against
// csm-notification-service using the OAuth2 client credentials grant type —
// the same shared app every other upstream client in this backend uses.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		upstreamhttp.TokenClient(tokenFetchTimeout))
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 10 * time.Second

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// Health calls csm-notification-service's own health endpoint and returns an
// error unless it answers 200.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("csmnotification: build health request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("csmnotification: health check: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &apierror.Error{StatusCode: resp.StatusCode}
	}
	return nil
}
