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

// Package choreo holds outbound clients for Choreo-hosted services that
// entity-service must call directly rather than through an event.
package choreo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// defaultTimeout bounds one outbound call. The recompute that triggers it is
// on a scheduled sweep, so a slow Choreo must not stall the whole batch.
const defaultTimeout = 15 * time.Second

// SubscriptionClosureClient calls Choreo Sales Operations' subscription
// closure endpoint — the port of ServiceNow's REST message
// "Choreo API Sales Operations" / "Update Subscription Closure State", which
// the `Consumed Query Hour Update` business rule invokes with
// setStringParameterNoEscape('subscriptionId', current.u_project_id).
//
// The payload field names (consumedQueryTime / totalQueryTime) are kept
// exactly as ServiceNow sends them so the receiving service needs no change
// at cutover.
type SubscriptionClosureClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// Config carries what the client needs. BaseURL is required; APIKey is sent
// as a Choreo API key header when set.
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// requireSecureURL rejects a base URL that is not https, so an api-key never
// travels in plaintext. Loopback http is allowed for local development, the
// same exception operations/csm-scheduled-tasks/internal/httpsec makes.
func requireSecureURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" {
		host := u.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
	}
	return fmt.Errorf("must be https (loopback http is allowed for local development), got %q", u.Scheme)
}

// rejectInsecureRedirects stops Go from replaying this client's api-key to
// wherever a 3xx names.
//
// The default CheckRedirect copies the original request's headers on a
// redirect, stripping only Authorization, Cookie and WWW-Authenticate when the
// host changes. A custom header like api-key is NOT in that list, so it would
// be forwarded to any host a redirect points at — including an http one.
// Refusing to follow redirects at all is the right call here: the Choreo
// endpoint is a fixed API, and a redirect from it is a misconfiguration worth
// surfacing rather than silently chasing.
func rejectInsecureRedirects(c *http.Client) {
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return fmt.Errorf("refusing to follow a redirect to %q: this client sends an api-key header, which Go would replay to the new host", req.URL.Redacted())
	}
}

// NewSubscriptionClosureClient constructs a client. It returns nil when
// BaseURL is empty, so an unconfigured deployment simply has pushing
// disabled rather than failing at startup — the recompute still runs and
// still records its result. Matches the nil-publisher convention used for
// Event Hub elsewhere in this service.
//
// It also returns nil when BaseURL is not https, after logging: a
// misconfigured URL must not silently downgrade an api-key onto the wire, and
// failing closed here costs only the outbound push, which is already optional.
func NewSubscriptionClosureClient(cfg Config) *SubscriptionClosureClient {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil
	}
	if err := requireSecureURL(cfg.BaseURL); err != nil {
		slog.Error("query-hour Choreo push disabled: insecure QUERY_HOUR_CHOREO_BASE_URL", "error", err)
		return nil
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	httpClient := &http.Client{Timeout: timeout}
	rejectInsecureRedirects(httpClient)
	return &SubscriptionClosureClient{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:  strings.TrimSpace(cfg.APIKey),
		http:    httpClient,
	}
}

// NotifyClosureState implements service.SubscriptionClosureNotifier.
func (c *SubscriptionClosureClient) NotifyClosureState(ctx context.Context, subscriptionID string, payload domain.SubscriptionClosureUpdate) error {
	if strings.TrimSpace(subscriptionID) == "" {
		return fmt.Errorf("subscriptionId is required")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding subscription closure payload: %w", err)
	}

	// The id goes in the path, so it is escaped — ServiceNow used
	// setStringParameterNoEscape, which would have broken on any id needing
	// encoding. Salesforce ids never do, but relying on that is not a reason
	// to reproduce the bug.
	endpoint := fmt.Sprintf("%s/subscriptions/%s/closure-state", c.baseURL, url.PathEscape(subscriptionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building subscription closure request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling subscription closure endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Read a bounded amount of the body: enough to diagnose, not enough
		// for a misbehaving upstream to fill the logs.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("subscription closure endpoint returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	// Drain so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return nil
}
