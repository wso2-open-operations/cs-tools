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

// Package notify sends report/alert emails directly against the same
// internal email notification service integrations/csm-notification-service
// uses (POST /send-email, OAuth2 client credentials) — this component has
// no event-emission hook yet (see this component's own CLAUDE.md, "Future:
// events"), so it sends its own email rather than publishing an event for
// csm-notification-service to relay.
package notify

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
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityhttp"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

// sendTimeout bounds every send-email request.
const sendTimeout = 25 * time.Second

// Config holds the configuration for the email client below.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	// FromAddress is the fixed "From" address used for every outgoing
	// email — a config value, not a SendEmail argument, so every email
	// this component sends comes from one pre-approved sender.
	FromAddress string
	// Transport, when set, is a shared entityhttp transport built for the
	// e-mail scopes; when nil the client builds its own from the credential
	// fields.
	Transport http.RoundTripper
}

// Client is an HTTP client for the internal email notification service,
// authenticated via the OAuth2 client credentials grant. Tokens are
// acquired and refreshed automatically.
//
// NewClient never contacts the token endpoint itself, so a valid-looking
// but wrong URL only surfaces as an error the first time SendEmail is
// called — but it does now validate cfg.TokenURL/cfg.BaseURL are https
// (loopback http allowed for local development — see
// httpsec.RequireSecureURL), since both carry credentials or a bearer
// token. That's the one point this diverges from
// integrations/csm-notification-service's own
// internal/notifications.EmailClient, whose constructor never fails.
type Client struct {
	http        *http.Client
	baseURL     string
	fromAddress string
}

// NewClient constructs a Client that authenticates against the email
// notification service using the OAuth2 client credentials grant type.
func NewClient(cfg Config) (*Client, error) {
	// BaseURL is allowed to be empty — email is optional per deployment
	// (see this type's own doc comment); an empty BaseURL just means
	// SendEmail will fail the first time something actually tries to use
	// it, same as before this validation existed. TokenURL is never
	// optional (both clients share OAUTH2_TOKEN_URL, which cmd/server/main.go
	// requires via mustEnv regardless of whether email itself is
	// configured), so it's always checked.
	rt := cfg.Transport
	if rt == nil {
		var err error
		rt, err = entityhttp.NewTransport(entityhttp.Credentials{
			TokenURL: cfg.TokenURL, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Scopes: cfg.Scopes,
		})
		if err != nil {
			return nil, fmt.Errorf("notify: %w", err)
		}
	}
	if cfg.BaseURL != "" {
		if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
			return nil, fmt.Errorf("notify: base URL: %w", err)
		}
	}

	return &Client{
		http:        entityhttp.NewClient(rt, sendTimeout),
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		fromAddress: cfg.FromAddress,
	}, nil
}

// sendEmailRequest is the wire shape expected by POST /send-email.
type sendEmailRequest struct {
	To       []string `json:"to"`
	CC       []string `json:"cc,omitempty"`
	From     string   `json:"from"`
	Subject  string   `json:"subject"`
	Template []byte   `json:"template"`
}

// SendEmail sends a plain HTML email via the notification service. The
// sender address is always the client's configured FromAddress. A request
// with no recipients is a silent no-op — see registry.Task.To's own doc
// comment for why that's a deliberate, valid configuration rather than an
// error. cc may be nil; a nil/empty cc with a non-empty to still sends
// normally, just with no one cc'd.
func (c *Client) SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error {
	if len(to) == 0 {
		return nil
	}
	if subject == "" {
		return fmt.Errorf("notify: subject is required")
	}

	reqBody, err := json.Marshal(sendEmailRequest{
		To:       to,
		CC:       cc,
		From:     c.fromAddress,
		Subject:  subject,
		Template: []byte(htmlBody),
	})
	if err != nil {
		return fmt.Errorf("notify: encode send-email request: %w", err)
	}

	var reqReader io.Reader = bytes.NewReader(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/send-email", reqReader)
	if err != nil {
		return fmt.Errorf("notify: build send-email request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notify: send-email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("notify: read send-email response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}
	return nil
}
