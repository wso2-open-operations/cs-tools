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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// emailTokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var emailTokenFetchTimeout = 10 * time.Second

// EmailConfig holds the configuration for the email notification channel.
type EmailConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string

	// FromAddress is the fixed "From" address used for every outgoing email.
	// It is a config value rather than a SendEmail argument so that all
	// portal-originated emails come from a single, pre-approved sender.
	FromAddress string
}

// EmailClient is an HTTP client for the email notification service
// (https://github.com/wso2-open-operations/infra-operations/tree/main/operations/email-service),
// authenticated via the OAuth2 client credentials grant. Tokens are acquired
// and refreshed automatically; callers need not manage them.
//
// NewEmailClient never fails and never contacts the token endpoint, so it is
// safe to construct with a zero-value EmailConfig (e.g. when the email
// channel is not yet configured for a given deployment) — a missing or
// invalid configuration only surfaces as an error the first time SendEmail
// is called.
type EmailClient struct {
	http        *http.Client
	baseURL     string
	fromAddress string
}

// NewEmailClient constructs an EmailClient that authenticates against the
// email notification service using the OAuth2 client credentials grant type.
func NewEmailClient(cfg EmailConfig) *EmailClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: emailTokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &EmailClient{
		http:        httpClient,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		fromAddress: cfg.FromAddress,
	}
}

// do executes an authenticated HTTP request against the email notification
// service and returns the raw JSON response body. The caller owns the
// returned slice.
func (c *EmailClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("notifications: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("notifications: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("notifications: read response body: %w", err)
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

// EmailAttachment is a single file attached to an outgoing email.
type EmailAttachment struct {
	// ContentName is the attachment's file name (e.g. "invoice.pdf").
	ContentName string `json:"contentName"`
	// ContentType is the attachment's MIME type (e.g. "application/pdf").
	ContentType string `json:"contentType"`
	// Attachment is the raw file content; encoding/json base64-encodes it
	// automatically since the field type is []byte.
	Attachment []byte `json:"attachment"`
}

// sendEmailRequest is the wire shape expected by POST /send-email.
type sendEmailRequest struct {
	To          []string          `json:"to"`
	CC          []string          `json:"cc,omitempty"`
	BCC         []string          `json:"bcc,omitempty"`
	ReplyTo     []string          `json:"replyTo,omitempty"`
	From        string            `json:"from"`
	Subject     string            `json:"subject"`
	Template    []byte            `json:"template"`
	Attachments []EmailAttachment `json:"attachments,omitempty"`
}

// SendEmail sends an HTML email via the notification service. The sender
// address is always the client's configured FromAddress; every other value
// is supplied by the caller.
func (c *EmailClient) SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []EmailAttachment) error {
	if len(to) == 0 {
		return fmt.Errorf("notifications: at least one recipient (to) is required")
	}
	if subject == "" {
		return fmt.Errorf("notifications: subject is required")
	}

	reqBody, err := json.Marshal(sendEmailRequest{
		To:          to,
		CC:          cc,
		BCC:         bcc,
		ReplyTo:     replyTo,
		From:        c.fromAddress,
		Subject:     subject,
		Template:    []byte(htmlBody),
		Attachments: attachments,
	})
	if err != nil {
		return fmt.Errorf("notifications: encode send-email request: %w", err)
	}

	_, err = c.do(ctx, http.MethodPost, "/send-email", reqBody)
	return err
}
