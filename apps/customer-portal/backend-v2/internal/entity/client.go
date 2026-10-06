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

// Package entity is the HTTP client for this repo's entity-service
// (cs-tools/entity-service), which fronts projects, cases, accounts, deployed
// products, and related resources.
package entity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// maxResponseBodyBytes bounds how much of an entity-service response this
// client will read into memory, protecting against a huge or malicious
// upstream response exhausting process memory.
const maxResponseBodyBytes = 10 << 20 // 10 MiB

// noRedirect stops an *http.Client from following any redirect, returning the
// redirect response itself instead. Used on both the token-fetch client and
// the entity-service request client — see the comment in NewClient.
func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

type ctxKey string

const userIDTokenKey ctxKey = "x-user-id-token"        // #nosec G101 -- context map key, not a credential
const correlationIDKey ctxKey = "x-csm-correlation-id" // #nosec G101 -- context map key, not a credential

// WithUserIDToken returns a copy of ctx carrying the x-user-id-token value to
// be forwarded on every outgoing entity-service request.
func WithUserIDToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, userIDTokenKey, token)
}

func userIDTokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDTokenKey).(string)
	return v
}

// WithCorrelationID returns a copy of ctx carrying the correlation ID to be
// forwarded as X-CSM-Correlation-ID on every outgoing entity-service request.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

func correlationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey).(string)
	return v
}

// Config holds the configuration for the entity-service client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	// Timeout bounds one entity-service request end to end. Zero means
	// DefaultTimeout.
	Timeout time.Duration
}

// DefaultTimeout is the entity-service request timeout used when
// Config.Timeout is unset. create-case relays inline base64 attachments (up
// to ~15 MiB), so it is longer than a typical API timeout. Operators may
// override it (ENTITY_SERVICE_TIMEOUT); it is advisable to keep it below the
// backend server's WriteTimeout so a clean error can be returned, but that is
// not enforced.
const DefaultTimeout = 60 * time.Second

// Client is an HTTP client for cs-tools/entity-service, authenticated via the
// OAuth2 client credentials grant. Tokens are acquired and refreshed
// automatically; callers need not manage them.
//
// Note: entity-service itself does not validate inbound credentials (see its
// own internal/middleware/usertoken.go) — it forwards x-user-id-token
// downstream to ServiceNow as-is. The OAuth2 layer here exists for the
// gateway/deployment fronting entity-service, mirroring the pattern used by
// apps/csm-portal/backend's entity client.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client that authenticates against entity-service
// using the OAuth2 client credentials grant type.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	// x-user-id-token carries the caller's JWT to entity-service, and the
	// client-credentials exchange carries cfg.ClientSecret to TokenURL. Go's
	// client only strips sensitive headers on cross-origin redirects for a
	// fixed allowlist (Authorization, Cookie, etc.) that covers neither, so a
	// redirecting TokenURL or BaseURL could otherwise receive one of them at a
	// different origin. Disable redirect-following entirely on both instead.
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout, CheckRedirect: noRedirect})

	// clientcredentials.Config.Client's returned *http.Client and its
	// Transport must not be mutated (see the package doc comment) — its
	// Transport is the only part we need (it injects the bearer token), so
	// wrap it in a fresh http.Client we own instead of setting fields
	// directly on the returned one.
	oauthClient := cc.Client(tokenCtx)
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	httpClient := &http.Client{
		Transport:     oauthClient.Transport,
		Timeout:       timeout,
		CheckRedirect: noRedirect,
	}

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// do executes an authenticated HTTP request against entity-service and
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
		req.Header.Set("X-CSM-Correlation-ID", id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("entity: read response body: %w", err)
	}
	if len(respBody) > maxResponseBodyBytes {
		return nil, fmt.Errorf("entity: %s %s: response body exceeds %d bytes", method, path, maxResponseBodyBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apierror.NewUpstreamError(resp.StatusCode, respBody)
	}

	return respBody, nil
}

// maxBinaryResponseBytes bounds attachment downloads specifically — larger
// than maxResponseBodyBytes since attachments (documents, images) are
// legitimately bigger than a typical JSON API response.
const maxBinaryResponseBytes = 25 << 20 // 25 MiB

// doBinary executes an authenticated GET request against entity-service and
// returns the raw response body together with the upstream Content-Type
// header. Use this instead of do for endpoints that return non-JSON binary
// content (e.g. attachment downloads).
func (c *Client) doBinary(ctx context.Context, path string) (body []byte, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, "", fmt.Errorf("entity: build request GET %s: %w", path, err)
	}
	if token := userIDTokenFromContext(ctx); token != "" {
		req.Header.Set("x-user-id-token", token)
	}
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set("X-CSM-Correlation-ID", id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("entity: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBinaryResponseBytes+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("entity: read response body: %w", err)
	}
	if len(respBody) > maxBinaryResponseBytes {
		return nil, "", fmt.Errorf("entity: GET %s: response body exceeds %d bytes", path, maxBinaryResponseBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := apierror.NewUpstreamError(resp.StatusCode, respBody)
		return nil, "", err
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	return respBody, ct, nil
}

// getJSON issues a GET request and decodes the JSON response into out.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("entity: decode response for GET %s: %w", path, err)
	}
	return nil
}

// postJSON marshals reqBody, issues a POST request, and decodes the JSON
// response into out.
func (c *Client) postJSON(ctx context.Context, path string, reqBody, out any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("entity: encode request for POST %s: %w", path, err)
	}
	body, err := c.do(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("entity: decode response for POST %s: %w", path, err)
	}
	return nil
}

// patchJSON marshals reqBody, issues a PATCH request, and decodes the JSON
// response into out.
func (c *Client) patchJSON(ctx context.Context, path string, reqBody, out any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("entity: encode request for PATCH %s: %w", path, err)
	}
	body, err := c.do(ctx, http.MethodPatch, path, payload)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("entity: decode response for PATCH %s: %w", path, err)
	}
	return nil
}

// deleteJSON issues a DELETE request and decodes the JSON response into out.
func (c *Client) deleteJSON(ctx context.Context, path string, out any) error {
	body, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("entity: decode response for DELETE %s: %w", path, err)
	}
	return nil
}
