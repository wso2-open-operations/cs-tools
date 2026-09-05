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

// Package emailservice is an HTTP client for WSO2's internal email
// notification service (owned by Rashmika's team). The real wire contract
// was confirmed directly against that service's own client code
// (integrations/csm-notification-service/internal/notifications/email.go,
// dev-app-csm-portal branch) — POST /send-email, OAuth2 client-credentials
// auth, no scopes required (confirmed explicitly with Rashmika — unlike
// csm-integration-service, this token endpoint doesn't need one). Only the
// fields this component actually uses (to, cc, subject, an HTML body) are
// exposed; bcc/replyTo/attachments exist on the real API but have no ACP
// use case, so they're deliberately left out here rather than plumbed
// through unused.
package emailservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// Config holds the configuration for the email notification service client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	// FromAddress is the fixed sender address used for every outgoing
	// email — confirmed (via a real received email) to be
	// "no-reply@wso2.com". It's a client-level config value, not a
	// per-SendEmail argument, matching the real service's own
	// EmailConfig.FromAddress: all ACP emails come from one pre-approved
	// sender, not a caller-chosen one.
	FromAddress string
}

// Client is an HTTP client authenticated to the email notification service
// via the OAuth2 client credentials grant. Tokens are acquired and
// refreshed automatically; callers need not manage them.
type Client struct {
	http        *http.Client
	baseURL     string
	fromAddress string
}

// NewClient constructs a Client that authenticates against the email
// notification service using the OAuth2 client credentials grant type,
// mirroring internal/entity.NewClient's identical setup for
// csm-integration-service. Returns an error if TokenURL or BaseURL isn't
// https:// — the token request carries the real ClientSecret, and BaseURL
// carries real notice content on every call, neither of which should ever
// travel in cleartext (CodeRabbit, PR #1657). internal/entity.Client has
// the same gap, not yet fixed there — see CLAUDE.md.
func NewClient(cfg Config) (*Client, error) {
	if err := requireHTTPS("TokenURL", cfg.TokenURL); err != nil {
		return nil, err
	}
	if err := requireHTTPS("BaseURL", cfg.BaseURL); err != nil {
		return nil, err
	}

	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		// No Scopes: confirmed with Rashmika that this token endpoint,
		// unlike csm-integration-service's, requires none.
	}

	// The redirect protection below must go on *this* client, not the one
	// cc.Client returns — this is the one that actually performs the
	// token-fetch request (POSTing the real client secret to TokenURL).
	// A version of this fix once applied CheckRedirect only to the
	// client returned below, leaving this one — the one that matters for
	// the secret itself — unprotected.
	tokenHTTPClient := &http.Client{
		Timeout: tokenFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTPClient)
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second
	// Same protection for regular API calls: oauth2.Transport reattaches
	// the Authorization bearer token to every request it processes,
	// including a followed redirect to a different host. Refuse to
	// follow so the token can never leak to a redirect target.
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Client{
		http:        httpClient,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		fromAddress: cfg.FromAddress,
	}, nil
}

// requireHTTPS rejects a URL that doesn't use the https scheme — name is
// the config field being checked, used only to make the error message
// point at the right place. A loopback host (127.0.0.1, ::1, localhost)
// is exempt even over plain http: that traffic never leaves the machine,
// so the cleartext-interception risk this check exists for doesn't apply
// — and it's exactly what every test in this package uses via
// httptest.NewServer, which only ever binds to loopback.
func requireHTTPS(name, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("emailservice: parse %s %q: %w", name, rawURL, err)
	}
	if u.Scheme == "https" || isLoopback(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("emailservice: %s must use https, got %q", name, rawURL)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// do executes an authenticated HTTP request against the email notification
// service and returns the raw JSON response body. The caller owns the
// returned slice.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("emailservice: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("emailservice: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt, err := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		if err != nil {
			return nil, fmt.Errorf("emailservice: read error response body: %w", err)
		}
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("emailservice: read response body: %w", err)
	}

	return respBody, nil
}

// sendEmailRequest is the wire shape expected by POST /send-email —
// confirmed against the real service's own client code. Only the fields
// this component uses are included; bcc/replyTo/attachments are valid on
// the real API but omitted here entirely (see package doc comment).
type sendEmailRequest struct {
	To      []string `json:"to"`
	CC      []string `json:"cc,omitempty"`
	From    string   `json:"from"`
	Subject string   `json:"subject"`
	// Template is the HTML email body, deliberately typed []byte (not
	// string) to match the real service's own request struct exactly —
	// encoding/json base64-encodes a []byte field automatically, and the
	// real server expects that encoding on this field. Sending it as a
	// plain JSON string instead would not match what the server decodes.
	// The Go method parameter is named htmlBody (see SendEmail) even
	// though the wire field is "template" — matches the real client's own
	// naming split between its Go API and its wire shape.
	Template []byte `json:"template"`
}

// SendEmail sends an HTML email via the notification service. htmlBody must
// already be valid HTML — this client does no conversion or escaping of its
// own; that's the caller's responsibility (see notify.EmailNotifier).
func (c *Client) SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error {
	if len(to) == 0 {
		return fmt.Errorf("emailservice: at least one recipient (to) is required")
	}
	if subject == "" {
		return fmt.Errorf("emailservice: subject is required")
	}

	reqBody, err := json.Marshal(sendEmailRequest{
		To:       to,
		CC:       cc,
		From:     c.fromAddress,
		Subject:  subject,
		Template: []byte(htmlBody),
	})
	if err != nil {
		return fmt.Errorf("emailservice: encode send-email request: %w", err)
	}

	_, err = c.do(ctx, http.MethodPost, "/send-email", reqBody)
	return err
}
