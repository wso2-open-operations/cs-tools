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
	"unicode"
	"unicode/utf8"

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

// Response size bounds. A JSON response larger than maxJSONResponseBytes, or a
// token response larger than maxTokenResponseBytes, is refused rather than
// buffered: a misbehaving upstream or an intermediate error page must not be
// able to make this service allocate without limit.
const (
	maxJSONResponseBytes   = 16 << 20
	maxTokenResponseBytes  = 64 << 10
	maxBinaryResponseBytes = 10 << 20
)

// errResponseTooLarge is returned (as a DownstreamError) when a response
// exceeds its size bound.
var errResponseTooLarge = &apierror.DownstreamError{Msg: "downstream response exceeds the allowed size"}

// readBounded reads at most limit bytes from r and reports whether the body
// was longer than that.
func readBounded(r io.Reader, limit int64) ([]byte, bool, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > limit {
		return nil, true, nil
	}
	return body, false, nil
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
// credentials, refreshing it 30 seconds before expiry, and again whenever the
// downstream rejects the cached one.
type Client struct {
	baseURL    string
	creds      ClientCredentialsConfig
	httpClient *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// New constructs a Client with a default 15-second timeout.
func New(baseURL string, creds ClientCredentialsConfig) *Client {
	return &Client{
		baseURL: baseURL,
		creds:   creds,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// invalidateToken drops the cached service token so the next request fetches
// a new one.
func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedToken = ""
	c.tokenExpiry = time.Time{}
}

// accessToken returns a valid bearer token, fetching a new one if the cached
// token is absent or within 30 seconds of expiry.
//
// A token endpoint that rejects this service's own credentials is a
// configuration problem on this side, not something the API caller did or
// can fix: it is logged with the detail an operator needs and reported to the
// caller as a plain service-unavailable error.
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

	raw, tooLarge, err := readBounded(resp.Body, maxTokenResponseBytes)
	if err != nil {
		return "", fmt.Errorf("snclient: read token response: %w", err)
	}
	if tooLarge {
		return "", errResponseTooLarge
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized, http.StatusForbidden:
		log.Printf("snclient: token endpoint rejected this service's client credentials (status %d); check the configured client id and secret", resp.StatusCode)
		return "", &apierror.ServiceUnavailableError{Msg: "downstream service unavailable"}
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

// upstreamResponse is one completed round trip: status, headers and the
// (size-bounded) body.
type upstreamResponse struct {
	status int
	header http.Header
	body   []byte
}

// roundTrip performs one authenticated request. body may be nil; when it is
// not, it is sent as application/json.
func (c *Client) roundTrip(ctx context.Context, method, path, userIDToken string, body []byte, limit int64) (upstreamResponse, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return upstreamResponse{}, err
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return upstreamResponse{}, fmt.Errorf("snclient: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if userIDToken != "" {
		req.Header.Set("x-user-id-token", userIDToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return upstreamResponse{}, err
		}
		return upstreamResponse{}, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("snclient: %s: %v", path, err)}
	}
	defer resp.Body.Close()

	raw, tooLarge, err := readBounded(resp.Body, limit)
	if err != nil {
		return upstreamResponse{}, fmt.Errorf("snclient: read response body: %w", err)
	}
	if tooLarge {
		return upstreamResponse{status: resp.StatusCode, header: resp.Header}, errResponseTooLarge
	}
	return upstreamResponse{status: resp.StatusCode, header: resp.Header, body: raw}, nil
}

// send performs a request, and when the downstream answers 401 it treats the
// cached service token as the likely cause first: the token is dropped, a
// fresh one fetched, and the request retried once. Only a 401 that survives
// a fresh service token is reported as a problem with the caller's user
// token. Without this, a rotated or revoked service token made every request
// fail as "invalid user token" until the cached token expired.
func (c *Client) send(ctx context.Context, method, path, userIDToken string, body []byte, limit int64) (upstreamResponse, error) {
	resp, err := c.roundTrip(ctx, method, path, userIDToken, body, limit)
	if err != nil || resp.status != http.StatusUnauthorized {
		return resp, err
	}
	log.Printf("snclient: %s: downstream answered 401; refreshing the service token and retrying once", sanitizeLog(path)) // #nosec G706 -- path sanitized
	c.invalidateToken()
	return c.roundTrip(ctx, method, path, userIDToken, body, limit)
}

// Get sends a GET request to the given path. The OAuth2 bearer token is added
// as Authorization header; userIDToken is forwarded as x-user-id-token for
// user context. Returns the raw response body on 2xx.
func (c *Client) Get(ctx context.Context, path string, userIDToken string) (json.RawMessage, error) {
	resp, err := c.send(ctx, http.MethodGet, path, userIDToken, nil, maxJSONResponseBytes)
	if err != nil {
		return nil, err
	}
	return mapResponse(resp, path)
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
	resp, err := c.send(ctx, http.MethodGet, path, userIDToken, nil, maxBinaryResponseBytes)
	if errors.Is(err, errResponseTooLarge) {
		return BinaryResponse{}, &apierror.ValidationError{Msg: "attachment content exceeds maximum allowed size of 10 MB"}
	}
	if err != nil {
		return BinaryResponse{}, err
	}

	switch {
	case resp.status >= 200 && resp.status < 300:
		ct := resp.header.Get("Content-Type")
		if ct == "" {
			ct = http.DetectContentType(resp.body)
		}
		return BinaryResponse{Body: resp.body, ContentType: ct}, nil
	case resp.status == http.StatusUnauthorized:
		return BinaryResponse{}, &apierror.UnauthorizedError{Msg: "invalid or missing x-user-id-token"}
	case resp.status == http.StatusForbidden:
		return BinaryResponse{}, &apierror.ForbiddenError{Msg: "not authorized to access this resource"}
	case resp.status == http.StatusNotFound:
		return BinaryResponse{}, &apierror.NotFoundError{Msg: "resource not found in downstream service"}
	case resp.status == http.StatusServiceUnavailable:
		return BinaryResponse{}, &apierror.ServiceUnavailableError{Msg: "downstream service unavailable"}
	default:
		// The body is downstream-controlled and may be large binary content:
		// it is not embedded in the error.
		return BinaryResponse{}, fmt.Errorf("snclient: %s: unexpected status %d (%d bytes)", path, resp.status, len(resp.body))
	}
}

// Patch sends a PATCH request to the given path. The OAuth2 bearer token is
// added as Authorization header; userIDToken is forwarded as x-user-id-token.
// Returns the raw response body on 2xx.
func (c *Client) Patch(ctx context.Context, path string, userIDToken string, payload any) (json.RawMessage, error) {
	return c.sendJSON(ctx, http.MethodPatch, path, userIDToken, payload)
}

// Delete sends a DELETE request to the given path. The OAuth2 bearer token is
// added as Authorization header; userIDToken is forwarded as x-user-id-token.
// Returns the raw response body on 2xx.
func (c *Client) Delete(ctx context.Context, path string, userIDToken string) (json.RawMessage, error) {
	resp, err := c.send(ctx, http.MethodDelete, path, userIDToken, nil, maxJSONResponseBytes)
	if err != nil {
		return nil, err
	}
	return mapResponse(resp, path)
}

// Post sends a POST request to the given path. The OAuth2 bearer token is
// added as Authorization header; userIDToken is forwarded as x-user-id-token.
// Returns the raw response body on 2xx.
func (c *Client) Post(ctx context.Context, path string, userIDToken string, payload any) (json.RawMessage, error) {
	return c.sendJSON(ctx, http.MethodPost, path, userIDToken, payload)
}

func (c *Client) sendJSON(ctx context.Context, method, path, userIDToken string, payload any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("snclient: marshal request: %w", err)
	}
	resp, err := c.send(ctx, method, path, userIDToken, body, maxJSONResponseBytes)
	if err != nil {
		return nil, err
	}
	return mapResponse(resp, path)
}

// maxDownstreamMessageRunes bounds the downstream message text passed on to
// the API caller.
const maxDownstreamMessageRunes = 300

// downstreamVocabulary rewrites backing-system terms that can appear in
// downstream error text into the platform's own neutral wording, so the
// message reads the same whichever data source produced it.
var downstreamVocabulary = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\bservice\s?now\b`), "the backing system"},
	{regexp.MustCompile(`(?i)\bsys_?ids?\b`), "id"},
	{regexp.MustCompile(`(?i)\bglide\s?record\b`), "record"},
}

// launderDownstreamMessage keeps the substance of a downstream error message
// -- it is usually the actionable reason (a rejected state transition, a
// missing required field) -- but makes it fit to show: control characters
// removed, whitespace collapsed, backing-system terms rewritten, and the
// length bounded.
func launderDownstreamMessage(msg string) string {
	msg = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, msg)
	msg = strings.Join(strings.Fields(msg), " ")
	for _, v := range downstreamVocabulary {
		msg = v.pattern.ReplaceAllString(msg, v.replacement)
	}
	if utf8.RuneCountInString(msg) > maxDownstreamMessageRunes {
		msg = string([]rune(msg)[:maxDownstreamMessageRunes]) + "..."
	}
	return msg
}

// extractDownstreamMessage attempts to parse a "message" field from the JSON
// error body returned by the downstream service. Falls back to defaultMsg if
// the body is empty, not JSON, or has no "message" field.
//
// The downstream service sometimes prefixes its message with an internal
// bracketed tag identifying which layer raised it (e.g.
// "[SERVICENOW_ERROR] State transition rejected"). That tag is logged here
// for correlation but stripped from the returned string, and the rest is
// laundered (launderDownstreamMessage), since the returned string is used as
// the client-facing apierror Msg -- it must never carry backend/vendor
// implementation detail.
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
		clientMsg = launderDownstreamMessage(clientMsg)
		if clientMsg == "" {
			return defaultMsg
		}
		return clientMsg
	}
	return defaultMsg
}

// mapResponse turns a completed JSON round trip into the body (2xx) or the
// matching typed error.
func mapResponse(resp upstreamResponse, path string) (json.RawMessage, error) {
	raw := resp.body
	switch {
	case resp.status >= 200 && resp.status < 300:
		return json.RawMessage(raw), nil
	case resp.status == http.StatusBadRequest:
		return nil, &apierror.ValidationError{Msg: extractDownstreamMessage(raw, "downstream service rejected the request")}
	case resp.status == http.StatusUnauthorized:
		// Already retried with a fresh service token (see send): what is
		// left is the caller's own user token.
		return nil, &apierror.UnauthorizedError{Msg: extractDownstreamMessage(raw, "invalid or missing x-user-id-token")}
	case resp.status == http.StatusForbidden:
		return nil, &apierror.ForbiddenError{Msg: extractDownstreamMessage(raw, "not authorized to access this resource")}
	case resp.status == http.StatusNotFound:
		return nil, &apierror.NotFoundError{Msg: extractDownstreamMessage(raw, "resource not found in downstream service")}
	case resp.status == http.StatusConflict:
		return nil, &apierror.ConflictError{Msg: extractDownstreamMessage(raw, "request conflicts with current state of the resource")}
	case resp.status == http.StatusServiceUnavailable:
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
		log.Printf("snclient: %s: unexpected status %d", sanitizeLog(path), resp.status) // #nosec G706 -- path sanitized
		return nil, &apierror.DownstreamError{
			Msg: extractDownstreamMessage(raw, fmt.Sprintf("downstream service returned an unexpected status (%d)", resp.status)),
		}
	}
}
