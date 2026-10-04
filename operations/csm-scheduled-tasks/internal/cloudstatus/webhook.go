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

package cloudstatus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

// webhookPath is appended to the per-cloud base URL. The same for every
// dashboard.
const webhookPath = "/api/v1/webhook"

// defaultSecretKey is the key in the secrets map whose value is used for any
// cloud without its own entry.
//
// It exists to express today's arrangement honestly. The legacy workflow
// chose the URL per cloud but signed EVERY post with a single shared secret,
// including Choreo's, Bijira's, Devant's and Moesif's. Whether that is because
// the two dashboards genuinely share a secret, or because the Choreo receiver
// does not verify the header at all, is NOT established.
//
// So this port reproduces the behaviour -- one secret covering every cloud --
// while making it a one-line config change to split them the moment someone
// confirms which it is. Hardcoding the shared-secret assumption into the code
// would bake in a weakness whose cause nobody has checked.
const defaultSecretKey = "default"

// WebhookConfig routes and authenticates posts to the status dashboards.
type WebhookConfig struct {
	// BaseURLs maps a cloud slug ("asgardeo", "choreo-eu") to that
	// dashboard's base URL. A cloud absent from this map is not posted.
	//
	// A MAP, not an if/else chain, and that is the point. The legacy
	// workflow's base-URL lookup named five clouds and fell off the end
	// returning undefined for choreo-eu and agent-manager, so their events
	// posted to a URL of "undefined" and vanished. Configuration turns "we have not set up
	// that dashboard yet" into a visible, skippable state instead of a silent
	// malformed request.
	BaseURLs map[string]string

	// Secrets maps a cloud slug to its FULL X-Webhook-Signature header value,
	// with defaultSecretKey as the fallback. The value carries a
	// scheme prefix -- `Secret <token>` -- so configure the whole string, not
	// the token alone. Sent verbatim; see Post.
	Secrets map[string]string
}

// Webhook posts cloud status events to the public dashboards.
type Webhook struct {
	http    *http.Client
	baseURL map[string]string
	secrets map[string]string
}

// NewWebhook validates the configured dashboard URLs and builds the poster.
//
// Every URL is checked at construction rather than at first use: a typo in one
// cloud's URL should stop the component from starting, not surface hours later
// as one cloud's events quietly failing while the others work.
func NewWebhook(cfg WebhookConfig) (*Webhook, error) {
	for cloud, raw := range cfg.BaseURLs {
		if err := httpsec.RequireSecureURL(raw); err != nil {
			return nil, fmt.Errorf("cloudstatus: webhook URL for %q: %w", cloud, err)
		}
	}
	urls := make(map[string]string, len(cfg.BaseURLs))
	for cloud, raw := range cfg.BaseURLs {
		urls[cloud] = strings.TrimRight(raw, "/")
	}

	// 15s: a status dashboard that has not answered in fifteen seconds is
	// down, and this tick has other clouds to tell.
	httpClient := &http.Client{Timeout: 15 * time.Second}
	httpsec.RejectInsecureRedirects(httpClient)

	return &Webhook{http: httpClient, baseURL: urls, secrets: cfg.Secrets}, nil
}

// Knows reports whether a cloud has a configured dashboard.
func (w *Webhook) Knows(cloud string) bool {
	_, ok := w.baseURL[cloud]
	return ok
}

// webhookBody is the exact payload the dashboards expect. Three fields, no
// more: the receiving dashboards parse this shape and adding to it is a
// coordinated change with them, not a local one.
type webhookBody struct {
	Event     string `json:"event"`
	Timestamp string `json:"timestamp"`
	Cloud     string `json:"cloud"`
}

// Post delivers one event to the dashboard for cloud.
//
// The X-Webhook-Signature header carries the configured value VERBATIM, and
// the whole value -- not just a token.
//
// Despite the header's name it is NOT an HMAC of the body. The dashboards
// expect a fixed string the receiver compares against a known one, and that string has
// a scheme prefix: the literal word `Secret`, a space, then the token. So the
// configured value must be the ENTIRE header, `Secret <token>`, and passing
// only the token produces a request the dashboard rejects.
//
// Computing a real signature here would be strictly better security and would
// be rejected by every dashboard until they are changed to match, so it is a
// coordinated change with them, not an improvement to slip into a port.
func (w *Webhook) Post(ctx context.Context, cloud, event, timestamp string) error {
	base, ok := w.baseURL[cloud]
	if !ok {
		return fmt.Errorf("cloudstatus: no dashboard URL configured for cloud %q", cloud)
	}

	body, err := json.Marshal(webhookBody{Event: event, Timestamp: timestamp, Cloud: cloud})
	if err != nil {
		return fmt.Errorf("cloudstatus: encode webhook body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+webhookPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cloudstatus: build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret := w.secretFor(cloud); secret != "" {
		req.Header.Set("X-Webhook-Signature", secret)
	}

	resp, err := w.http.Do(req)
	if err != nil {
		return fmt.Errorf("cloudstatus: post webhook for %s: %w", cloud, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// The response body is deliberately NOT included. It is recorded into
		// last_error and read by whoever investigates, and an unknown external
		// service's error text is not something to copy into this system's own
		// storage unexamined. The status code is what the retry decision
		// needs.
		return fmt.Errorf("cloudstatus: dashboard for %s returned %d", cloud, resp.StatusCode)
	}
	return nil
}

// secretFor returns the cloud's own secret, or the shared default.
func (w *Webhook) secretFor(cloud string) string {
	if s, ok := w.secrets[cloud]; ok && s != "" {
		return s
	}
	return w.secrets[defaultSecretKey]
}
