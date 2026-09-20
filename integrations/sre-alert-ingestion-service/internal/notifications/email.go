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

package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// emailTokenFetchTimeout is the HTTP client timeout for token-endpoint
// requests. Overridden in tests to keep them fast.
var emailTokenFetchTimeout = 10 * time.Second

// EmailConfig holds the configuration for the email escalation channel: the
// same internal email-notification-service
// integrations/csm-notification-service and apps/csm-portal/backend already
// call, reached via the OAuth2 client credentials grant. Trimmed to this
// service's one use case (a fixed recipient list, a plain HTML body, no
// CC/BCC/attachments/reply-to) — see EmailClient.SendEscalation.
type EmailConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string

	// FromAddress is the fixed "From" address used for every escalation
	// email.
	FromAddress string
	// ToAddresses is the fixed recipient list every escalation email goes
	// to — an operator-provisioned distribution list/mailbox, not a
	// per-alert decision.
	ToAddresses []string
}

// EmailClient is an HTTP client for the internal email-notification
// service, authenticated via the OAuth2 client credentials grant. Tokens
// are acquired and refreshed automatically; callers need not manage them.
//
// NewEmailClient never fails, so it is safe to construct with a zero-value
// EmailConfig (e.g. this channel not yet configured for a given deployment)
// — a missing configuration only surfaces as an error the first time
// SendEscalation is called, the same posture as TwilioClient/GoogleChatClient.
type EmailClient struct {
	http        *http.Client
	baseURL     string
	fromAddress string
	toAddresses []string
}

// NewEmailClient constructs an EmailClient that authenticates against the
// email notification service using the OAuth2 client credentials grant type.
func NewEmailClient(cfg EmailConfig) *EmailClient {
	return newEmailClient(cfg, false)
}

// newEmailClient is NewEmailClient's real implementation.
// allowInsecureLoopback exists only for this package's own tests (see
// httpsOnlyTransport's doc comment) — NewEmailClient always passes false, so
// production code has no path to a non-HTTPS endpoint, loopback included.
func newEmailClient(cfg EmailConfig, allowInsecureLoopback bool) *EmailClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	// Same httpsOnlyTransport treatment as csmclient.NewClient: both the
	// token fetch (carries ClientID/ClientSecret) and every subsequent
	// email-send call (carries the bearer token it returns) must never go
	// out over a non-HTTPS endpoint. See transport.go's doc comment.
	tokenHTTPClient := &http.Client{
		Timeout:   emailTokenFetchTimeout,
		Transport: &httpsOnlyTransport{allowInsecureLoopback: allowInsecureLoopback},
	}
	// A 307/308 redirect on the token endpoint would resubmit ClientID/
	// ClientSecret to whatever host it names; refuse to follow.
	tokenHTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTPClient)
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second
	httpClient.Transport = &httpsOnlyTransport{base: httpClient.Transport, allowInsecureLoopback: allowInsecureLoopback}
	// oauth2.Transport reattaches the Authorization bearer token to every
	// request it processes, including a followed redirect to a different
	// host. Refuse to follow so the token can never leak off-host; the 3xx
	// response is surfaced through the normal non-2xx error path instead.
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &EmailClient{
		http:        httpClient,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		fromAddress: cfg.FromAddress,
		toAddresses: cfg.ToAddresses,
	}
}

// sendEmailRequest is the wire shape expected by POST /send-email — the
// subset of the full internal email-service contract
// (integrations/csm-notification-service/internal/notifications/email.go)
// this service actually needs.
type sendEmailRequest struct {
	To       []string `json:"to"`
	From     string   `json:"from"`
	Subject  string   `json:"subject"`
	Template []byte   `json:"template"`
}

// SendEscalation sends subject/htmlBody as an HTML email to the configured
// ToAddresses, from the configured FromAddress.
func (c *EmailClient) SendEscalation(ctx context.Context, subject, htmlBody string) error {
	if len(c.toAddresses) == 0 {
		return fmt.Errorf("notifications: no escalation email recipients configured")
	}
	if strings.TrimSpace(subject) == "" {
		return fmt.Errorf("notifications: subject is required")
	}

	reqBody, err := json.Marshal(sendEmailRequest{
		To:       c.toAddresses,
		From:     c.fromAddress,
		Subject:  subject,
		Template: []byte(htmlBody),
	})
	if err != nil {
		return fmt.Errorf("notifications: encode send-email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/send-email", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("notifications: build send-email request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notifications: post send-email request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt, rerr := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		if rerr != nil {
			return fmt.Errorf("notifications: read send-email response: %w", rerr)
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}
	return nil
}
