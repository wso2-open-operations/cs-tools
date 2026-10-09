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

// Package statuspage posts cloud status webhooks to the public status
// dashboards (outage.status_page_due).
//
// THE SAME REQUEST csm-scheduled-tasks SENDS, field for field: that component
// (operations/csm-scheduled-tasks/internal/cloudstatus/webhook.go) stays the
// retry path, and a dashboard must not be able to tell the two senders apart.
// Separate Go modules, so the request is duplicated rather than shared --
// keep the two in step:
//
//   - POST <base URL>/api/v1/webhook, the base URL chosen by cloud slug;
//   - body {"event","timestamp","cloud"}, nothing else;
//   - X-Webhook-Signature carries the configured value VERBATIM -- the whole
//     header, "Secret <token>", not an HMAC -- the cloud's own or "default".
package statuspage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const (
	webhookPath   = "/api/v1/webhook"
	defaultSecret = "default"
)

// Webhook posts to the configured dashboards.
type Webhook struct {
	http    *http.Client
	baseURL map[string]string
	secrets map[string]string
}

// New validates every dashboard URL up front -- a typo in one should stop the
// service starting, not surface later as one cloud's posts quietly failing. A
// URL must be https, or plain http to a loopback host for local development.
func New(baseURLs, secrets map[string]string) (*Webhook, error) {
	if len(baseURLs) == 0 {
		return nil, errors.New("statuspage: no dashboard URLs")
	}
	if len(secrets) == 0 {
		// The dashboards answer an unsigned post with 401.
		return nil, errors.New("statuspage: no secrets")
	}
	urls := make(map[string]string, len(baseURLs))
	for cloud, raw := range baseURLs {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			// Not echoing raw or url.Parse's text: either can carry a token.
			return nil, fmt.Errorf("statuspage: the URL for %q is not a valid URL", cloud)
		}
		if u.Scheme != "https" && !isLoopback(u.Hostname()) {
			return nil, fmt.Errorf("statuspage: the URL for %q must use https", cloud)
		}
		urls[cloud] = strings.TrimRight(raw, "/")
	}
	client := &http.Client{
		// 15s, as the scheduled task: a dashboard that has not answered by
		// then is down, and the scheduled task will retry.
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if req.URL.Scheme != "https" {
				return errors.New("statuspage: refusing a redirect off https")
			}
			return nil
		},
	}
	return &Webhook{http: client, baseURL: urls, secrets: secrets}, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// UnknownOutcomeError is a post whose request reached the dashboard but got
// no answer (a timeout, a reset). The dashboard may have taken it, so it must
// not be sent again. Every other error from Post is definite: the request was
// never sent, or the dashboard answered that it did not take it.
type UnknownOutcomeError struct{ Err error }

func (e *UnknownOutcomeError) Error() string { return "outcome unknown: " + e.Err.Error() }
func (e *UnknownOutcomeError) Unwrap() error { return e.Err }

// IsUnknownOutcome reports whether err is an UnknownOutcomeError.
func IsUnknownOutcome(err error) bool {
	var u *UnknownOutcomeError
	return errors.As(err, &u)
}

// Post delivers one event. A non-2xx answer is an error carrying the status
// code only: the dashboard's body is not copied into entity-service's
// last_error. A request that was written but got no answer is an
// *UnknownOutcomeError.
func (w *Webhook) Post(ctx context.Context, cloud, event, timestamp string) error {
	base, ok := w.baseURL[cloud]
	if !ok {
		return fmt.Errorf("no dashboard URL configured for cloud %q", cloud)
	}
	body, err := json.Marshal(struct {
		Event     string `json:"event"`
		Timestamp string `json:"timestamp"`
		Cloud     string `json:"cloud"`
	}{event, timestamp, cloud})
	if err != nil {
		return fmt.Errorf("encode webhook body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+webhookPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret := w.secretFor(cloud); secret != "" {
		req.Header.Set("X-Webhook-Signature", secret)
	}
	// Whether the request reached the wire decides whether a failure is safe
	// to retry: one that never left cannot have been processed.
	var wrote atomic.Bool
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				wrote.Store(true)
			}
		},
	}))
	resp, err := w.http.Do(req)
	if err != nil {
		err = fmt.Errorf("post webhook for %s: %w", cloud, err)
		if wrote.Load() {
			return &UnknownOutcomeError{Err: err}
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("dashboard for %s returned %d", cloud, resp.StatusCode)
	}
	return nil
}

func (w *Webhook) secretFor(cloud string) string {
	if s, ok := w.secrets[cloud]; ok && s != "" {
		return s
	}
	return w.secrets[defaultSecret]
}
