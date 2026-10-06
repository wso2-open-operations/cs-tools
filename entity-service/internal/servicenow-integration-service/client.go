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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Package integrationservice provides an HTTP client for the ServiceNow integration service API.
package integrationservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// sanitizeLog strips CR/LF characters from a string to prevent log injection.
// Apply to every downstream-derived operand before passing it to a log call.
var sanitizeLog = strings.NewReplacer("\n", `\n`, "\r", `\r`).Replace

// internalErrorTagPattern matches a leading bracketed all-caps tag (e.g.
// "[SERVICENOW_ERROR] ") that a downstream service prepends to its error
// messages to identify which internal layer raised the error. The tag is
// useful for correlating log lines with the originating layer, but it is an
// implementation detail that must never reach the client-facing message.
var internalErrorTagPattern = regexp.MustCompile(`^\[[A-Z][A-Z0-9_]*\]\s*`)

// stripInternalErrorTag splits msg into the client-safe message with any
// leading internal tag removed, and the tag itself (empty if msg carried no
// tag). Only the client-safe message should ever be placed in a field the
// caller can see; the tag, if present, belongs in logs only.
func stripInternalErrorTag(msg string) (clientMsg string, tag string) {
	loc := internalErrorTagPattern.FindStringIndex(msg)
	if loc == nil {
		return msg, ""
	}
	return msg[loc[1]:], strings.TrimSpace(msg[loc[0]:loc[1]])
}

// ClientCredentialsConfig holds the OAuth2 client credentials used to obtain
// a bearer token for service-to-service calls to the Choreo API.
type ClientCredentialsConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes is a space-separated list of OAuth2 scopes.
	Scopes string
}

// tokenResponse is the subset of the OAuth2 token endpoint response we use.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// Client is a thin HTTP wrapper around the Choreo ServiceNow API base URL.
// It automatically fetches and caches an OAuth2 bearer token using client
// credentials, refreshing it 30 seconds before expiry.
type Client struct {
	baseURL    string
	creds      ClientCredentialsConfig
	httpClient *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// New constructs a Client whose HTTP calls time out after timeout. Create-case
// carries inline base64 attachments (up to 15 MiB), which the upstream call
// must be allowed to finish; keep it below the server write timeout so the
// handler can still return a clean error.
func New(baseURL string, creds ClientCredentialsConfig, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		creds:   creds,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// accessToken returns a valid bearer token, fetching a new one if the cached
// token is absent or within 30 seconds of expiry.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedToken != "" && time.Now().Add(30*time.Second).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.creds.ClientID)
	data.Set("client_secret", c.creds.ClientSecret)
	if c.creds.Scopes != "" {
		data.Set("scope", c.creds.Scopes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.creds.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("snclient: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("snclient: token endpoint unavailable: %v", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("snclient: read token response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", &apierror.UnauthorizedError{Msg: fmt.Sprintf("integrationservice: token endpoint rejected credentials (status %d) — check client ID and secret", resp.StatusCode)}
	default:
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("integrationservice: token endpoint returned %d", resp.StatusCode)}
	}

	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("snclient: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("snclient: token endpoint returned empty access_token")
	}

	c.cachedToken = tr.AccessToken
	if tr.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		c.tokenExpiry = time.Now().Add(3600 * time.Second)
	}

	return c.cachedToken, nil
}

// Get sends a GET request to the given path. The OAuth2 bearer token is added
// as Authorization header; userIDToken is forwarded as x-user-id-token for
// user context. Returns the raw response body on 2xx.
func (c *Client) Get(ctx context.Context, path string, userIDToken string) (json.RawMessage, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("snclient: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if userIDToken != "" {
		req.Header.Set("x-user-id-token", userIDToken)
	}

	return c.do(req, path)
}

// BinaryResponse holds the raw body and the upstream Content-Type for binary downloads.
type BinaryResponse struct {
	Body        []byte
	ContentType string
}

// GetBinary sends a GET request and returns the raw response body together with
// the upstream Content-Type header. Use this instead of Get for endpoints that
// return non-JSON binary content (e.g. file downloads).
func (c *Client) GetBinary(ctx context.Context, path string, userIDToken string) (BinaryResponse, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return BinaryResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return BinaryResponse{}, fmt.Errorf("snclient: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if userIDToken != "" {
		req.Header.Set("x-user-id-token", userIDToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return BinaryResponse{}, err
		}
		return BinaryResponse{}, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("snclient: %s: %v", path, err)}
	}
	defer resp.Body.Close()

	const maxBinaryBytes = 10*1024*1024 + 1 // 10 MB + 1 to detect over-limit responses
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBinaryBytes))
	if err != nil {
		return BinaryResponse{}, fmt.Errorf("snclient: read response body: %w", err)
	}
	if len(body) >= maxBinaryBytes {
		return BinaryResponse{}, &apierror.ValidationError{Msg: "attachment content exceeds maximum allowed size of 10 MB"}
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			ct = http.DetectContentType(body)
		}
		return BinaryResponse{Body: body, ContentType: ct}, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return BinaryResponse{}, &apierror.UnauthorizedError{Msg: "invalid or missing x-user-id-token"}
	case resp.StatusCode == http.StatusForbidden:
		return BinaryResponse{}, &apierror.ForbiddenError{Msg: "not authorized to access this resource"}
	case resp.StatusCode == http.StatusNotFound:
		return BinaryResponse{}, &apierror.NotFoundError{Msg: "resource not found in downstream service"}
	case resp.StatusCode == http.StatusServiceUnavailable:
		return BinaryResponse{}, &apierror.ServiceUnavailableError{Msg: "downstream service unavailable"}
	default:
		return BinaryResponse{}, fmt.Errorf("snclient: %s: unexpected status %d: %s", path, resp.StatusCode, body)
	}
}

// Patch sends a PATCH request to the given path. The OAuth2 bearer token is
// added as Authorization header; userIDToken is forwarded as x-user-id-token.
// Returns the raw response body on 2xx.
func (c *Client) Patch(ctx context.Context, path string, userIDToken string, payload any) (json.RawMessage, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("snclient: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("snclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if userIDToken != "" {
		req.Header.Set("x-user-id-token", userIDToken)
	}

	return c.do(req, path)
}

// Delete sends a DELETE request to the given path. The OAuth2 bearer token is
// added as Authorization header; userIDToken is forwarded as x-user-id-token.
// Returns the raw response body on 2xx.
func (c *Client) Delete(ctx context.Context, path string, userIDToken string) (json.RawMessage, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("snclient: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if userIDToken != "" {
		req.Header.Set("x-user-id-token", userIDToken)
	}

	return c.do(req, path)
}

// Post sends a POST request to the given path. The OAuth2 bearer token is
// added as Authorization header; userIDToken is forwarded as x-user-id-token.
// Returns the raw response body on 2xx.
func (c *Client) Post(ctx context.Context, path string, userIDToken string, payload any) (json.RawMessage, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("snclient: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("snclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if userIDToken != "" {
		req.Header.Set("x-user-id-token", userIDToken)
	}

	return c.do(req, path)
}

// extractDownstreamMessage attempts to parse a "message" field from the JSON
// error body returned by the downstream service. Falls back to defaultMsg if
// the body is empty, not JSON, or has no "message" field.
//
// The downstream service sometimes prefixes its message with an internal
// bracketed tag identifying which layer raised it (e.g.
// "[SERVICENOW_ERROR] State transition rejected"). That tag is logged here
// for correlation but stripped from the returned string, since the returned
// string is used verbatim as the client-facing apierror Msg — it must never
// carry backend/vendor implementation detail.
func extractDownstreamMessage(body []byte, defaultMsg string) string {
	if len(body) == 0 {
		return defaultMsg
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Message != "" {
		clientMsg, tag := stripInternalErrorTag(payload.Message)
		if tag != "" {
			// Log only the tag, never the downstream-controlled message text —
			// the message is attacker/upstream-influenced and must not be
			// persisted verbatim per this repo's logging guidelines.
			log.Printf("snclient: downstream error tagged %s", sanitizeLog(tag)) // #nosec G706 -- tag sanitized
		}
		if clientMsg == "" {
			return defaultMsg
		}
		return clientMsg
	}
	return defaultMsg
}

func (c *Client) do(req *http.Request, path string) (json.RawMessage, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("snclient: %s: %v", path, err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("snclient: read response body: %w", err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return json.RawMessage(raw), nil
	case resp.StatusCode == http.StatusBadRequest:
		return nil, &apierror.ValidationError{Msg: extractDownstreamMessage(raw, "downstream service rejected the request")}
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, &apierror.UnauthorizedError{Msg: extractDownstreamMessage(raw, "invalid or missing x-user-id-token")}
	case resp.StatusCode == http.StatusForbidden:
		return nil, &apierror.ForbiddenError{Msg: extractDownstreamMessage(raw, "not authorized to access this resource")}
	case resp.StatusCode == http.StatusNotFound:
		return nil, &apierror.NotFoundError{Msg: extractDownstreamMessage(raw, "resource not found in downstream service")}
	case resp.StatusCode == http.StatusConflict:
		return nil, &apierror.ConflictError{Msg: extractDownstreamMessage(raw, "request conflicts with current state of the resource")}
	case resp.StatusCode == http.StatusServiceUnavailable:
		return nil, &apierror.ServiceUnavailableError{Msg: "downstream service unavailable"}
	default:
		// Most commonly a downstream 500. The downstream layer's error envelope
		// carries a real reason there (a rejected state transition, a payload
		// validation failure), and flattening it into an untyped error loses it:
		// writeServiceError's default branch replaces it with the fixed
		// "internal server error" literal, so the caller — and the user — is told
		// nothing. Extract the reason through the same sanitizing helper the
		// mapped branches use and keep it. The raw body is deliberately not
		// included: it is downstream-controlled text and must not be logged
		// verbatim.
		log.Printf("snclient: %s: unexpected status %d", sanitizeLog(path), resp.StatusCode) // #nosec G706 -- path sanitized
		return nil, &apierror.DownstreamError{
			Msg: extractDownstreamMessage(raw, fmt.Sprintf("downstream service returned an unexpected status (%d)", resp.StatusCode)),
		}
	}
}
