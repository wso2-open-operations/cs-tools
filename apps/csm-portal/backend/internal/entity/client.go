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

package entity

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

type ctxKey string

const userIDTokenKey ctxKey = "x-user-id-token"
const correlationIDKey ctxKey = "x-correlation-id"

// WithUserIDToken returns a copy of ctx carrying the x-user-id-token value to
// be forwarded on every outgoing entity request.
func WithUserIDToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, userIDTokenKey, token)
}

func userIDTokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDTokenKey).(string)
	return v
}

// WithCorrelationID returns a copy of ctx carrying the correlation ID to be
// forwarded as X-Correlation-ID on every outgoing entity request.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

func correlationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey).(string)
	return v
}

// Config holds the configuration for the entity service client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is an HTTP client authenticated via the OAuth2 client credentials grant.
// Tokens are acquired and refreshed automatically; callers need not manage them.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client that authenticates against the entity service
// using the OAuth2 client credentials grant type, mirroring the Ballerina
// ClientCredentialsOauth2Config pattern in the customer-portal entity module.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	// TODO: wrap the base transport with a retry transport that retries on
	// HTTP 502, 503, and 504 (up to 3 attempts with a 2 s interval) to match
	// the retryConfig defined in the Ballerina entity client.
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// do executes an authenticated HTTP request against the entity service and
// returns the raw JSON response body. The caller owns the returned slice.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("entity: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := userIDTokenFromContext(ctx); token != "" {
		req.Header.Set("x-user-id-token", token)
	}
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set("X-Correlation-ID", id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("entity: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return respBody, nil
}

// doBinary executes an authenticated GET request against the entity service and
// returns the raw response body together with the upstream Content-Type header.
// Use this instead of do for endpoints that return non-JSON binary content.
func (c *Client) doBinary(ctx context.Context, path string) (body []byte, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, "", fmt.Errorf("entity: build request GET %s: %w", path, err)
	}
	if token := userIDTokenFromContext(ctx); token != "" {
		req.Header.Set("x-user-id-token", token)
	}
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set("X-Correlation-ID", id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("entity: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("entity: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, "", &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	return respBody, ct, nil
}
