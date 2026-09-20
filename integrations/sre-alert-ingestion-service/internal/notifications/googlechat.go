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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
)

// GoogleChatConfig holds the configuration for the Google Chat escalation
// channel: a single space's incoming webhook URL. Unlike
// integrations/csm-notification-service's own GoogleChatClient (which
// routes per-product to many spaces, for case/incident alerts across the
// whole CSM portal), this service escalates to exactly one
// operator-configured space — there is no per-alert routing decision to
// make, only "did delivery to CSM keep failing."
type GoogleChatConfig struct {
	// WebhookURL is that space's incoming webhook URL (Space settings > Apps
	// & integrations > Webhooks). It already carries its own key/token query
	// parameters, so no separate auth flow is needed — same as
	// csm-notification-service's own webhook client.
	WebhookURL string
}

// GoogleChatClient posts a plain-text message to a single Google Chat space
// via an incoming webhook.
//
// NewGoogleChatClient never fails, so it is safe to construct with a
// zero-value GoogleChatConfig (e.g. this channel not yet configured for a
// given deployment) — a missing webhook URL only surfaces as an error the
// first time SendMessage is called, the same posture as TwilioClient.
type GoogleChatClient struct {
	http       *http.Client
	webhookURL string
}

// NewGoogleChatClient constructs a GoogleChatClient posting to cfg.WebhookURL.
func NewGoogleChatClient(cfg GoogleChatConfig) *GoogleChatClient {
	return newGoogleChatClient(cfg, false)
}

// newGoogleChatClient is NewGoogleChatClient's real implementation.
// allowInsecureLoopback exists only for this package's own tests (see
// httpsOnlyTransport's doc comment) — NewGoogleChatClient always passes
// false, so production code has no path to a non-HTTPS endpoint, loopback
// included.
func newGoogleChatClient(cfg GoogleChatConfig, allowInsecureLoopback bool) *GoogleChatClient {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		// httpsOnlyTransport (transport.go, this package) refuses a
		// non-HTTPS webhook URL (test-only loopback exception, never
		// reachable via NewGoogleChatClient) — the URL carries a secret
		// key/token in its query string, which must never go out in
		// cleartext.
		Transport: &httpsOnlyTransport{allowInsecureLoopback: allowInsecureLoopback},
	}
	// A redirect response would resubmit the webhook URL — secret key/token
	// query params included — to whatever host it names; refuse to follow.
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &GoogleChatClient{
		http:       httpClient,
		webhookURL: strings.TrimSpace(cfg.WebhookURL),
	}
}

// chatMessage is the minimal wire shape Google Chat's incoming-webhook API
// accepts for a plain-text message:
// https://developers.google.com/chat/api/guides/message-formats/basic —
// deliberately not the richer cardsV2 schema
// integrations/csm-notification-service/internal/notifications/googlechat.go
// uses for its many case/incident card types; this channel only ever sends
// one kind of message (an escalation notice), so a card adds structure with
// nothing to structure.
type chatMessage struct {
	Text string `json:"text"`
}

// redactURLError strips the request URL — which carries the webhook's secret
// key/token query parameters — out of a *url.Error before it's wrapped and
// potentially logged, matching the same reasoning
// integrations/csm-notification-service's own Google Chat client uses.
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

// SendMessage posts text as a plain-text message to the configured space.
func (c *GoogleChatClient) SendMessage(ctx context.Context, text string) error {
	if c.webhookURL == "" {
		return fmt.Errorf("notifications: google chat is not configured")
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("notifications: text is required")
	}

	body, err := json.Marshal(chatMessage{Text: text})
	if err != nil {
		return fmt.Errorf("notifications: encode google chat message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notifications: build google chat request: %w", redactURLError(err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notifications: post google chat message: %w", redactURLError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt, rerr := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		if rerr != nil {
			return fmt.Errorf("notifications: read google chat response: %w", rerr)
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}
	return nil
}
