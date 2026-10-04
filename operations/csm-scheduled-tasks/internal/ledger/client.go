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

// Package ledger is the entity-service half of this component's durable
// state: this process has no database of its own (same as
// integrations/csm-notification-service — see that repo's own CLAUDE.md for
// the precedent), so every claim/retry/succeed/fail decision is persisted
// against entity-service's scheduled_task_run table via the client below.
// See entity-service's own CLAUDE.md ("Scheduled task runs") for the API
// contract this client talks to.
package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityhttp"
)

// requestTimeout bounds every request this client makes.
const requestTimeout = 25 * time.Second

// Config holds this client's configuration. TokenURL/ClientID/ClientSecret
// are whichever OAuth2 app is appropriate for this deployment. Transport,
// when set, is a shared entityhttp transport (cmd/server/main.go builds one
// for every entity-service client so a tick performs one token grant);
// when nil the client builds its own from the credential fields.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	Transport    http.RoundTripper
}

// Client is a narrow HTTP client for entity-service's scheduled-task-runs
// endpoints.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant. cfg.TokenURL/cfg.BaseURL must both be https (loopback
// http is allowed for local development — see httpsec.RequireSecureURL)
// since both carry credentials or a bearer token. It never contacts the
// token endpoint itself — a valid-looking but wrong URL only surfaces as an
// error the first time a method below is called.
func NewClient(cfg Config) (*Client, error) {
	httpClient, err := entityhttp.ClientFor(cfg.Transport, entityhttp.Credentials{
		TokenURL: cfg.TokenURL, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Scopes: cfg.Scopes,
	}, cfg.BaseURL, requestTimeout)
	if err != nil {
		return nil, fmt.Errorf("ledger: %w", err)
	}
	return &Client{http: httpClient, baseURL: entityhttp.TrimBase(cfg.BaseURL)}, nil
}

// do executes an authenticated request — see entityhttp.Do.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return entityhttp.Do(ctx, c.http, c.baseURL, "ledger", method, path, body)
}

// Run is the subset of entity-service's ScheduledTaskRun response this
// component reads. Field names match entity-service's own response naming
// (timestamps use the "On" suffix there, not "At").
type Run struct {
	ID               string     `json:"id"`
	TaskName         string     `json:"taskName"`
	PeriodKey        time.Time  `json:"periodKey"`
	AttemptCount     int        `json:"attemptCount"`
	LastError        *string    `json:"lastError"`
	NextRetryOn      *time.Time `json:"nextRetryOn"`
	FirstAttemptedOn time.Time  `json:"firstAttemptedOn"`
	LastAttemptedOn  time.Time  `json:"lastAttemptedOn"`
	SucceededOn      *time.Time `json:"succeededOn"`
	SupersededOn     *time.Time `json:"supersededOn"`
}

// Claim is the response from Attempt.
type Claim struct {
	// Allowed is the actual decision the caller must act on — false does
	// not mean error, it means this exact claim was denied (already
	// succeeded, already superseded, not yet due, or claimed by another
	// still-live attempt). Inspect Run to tell those apart.
	Allowed bool `json:"allowed"`
	Run     Run  `json:"run"`
}

// Attempt calls POST /scheduled-tasks/attempts — see entity-service's
// own CLAUDE.md ("Scheduled task runs") for the full decision table.
// staleClaimAfter bounds how long a row that looks currently-claimed is
// trusted before being treated as an orphaned claim (this process crashed
// after claiming but before calling Complete/Fail) and made eligible for
// another attempt; pass 0 to use entity-service's own default (1 hour).
func (c *Client) Attempt(ctx context.Context, taskName string, periodKey time.Time, staleClaimAfter time.Duration) (Claim, error) {
	body, err := json.Marshal(struct {
		TaskName               string    `json:"taskName"`
		PeriodKey              time.Time `json:"periodKey"`
		StaleClaimAfterSeconds int       `json:"staleClaimAfterSeconds,omitempty"`
	}{TaskName: taskName, PeriodKey: periodKey, StaleClaimAfterSeconds: int(staleClaimAfter.Seconds())})
	if err != nil {
		return Claim{}, fmt.Errorf("ledger: encode Attempt request: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/scheduled-tasks/attempts", body)
	if err != nil {
		return Claim{}, err
	}
	var claim Claim
	if err := json.Unmarshal(respBody, &claim); err != nil {
		return Claim{}, fmt.Errorf("ledger: decode Attempt response: %w", err)
	}
	return claim, nil
}

// Complete calls PATCH /scheduled-tasks/attempts/{id} with status
// "succeeded". attemptCount must be the Claim.Run.AttemptCount the Attempt
// call handed back for this same id — entity-service rejects a mismatched
// attemptCount (a stale claim, or one that's already been resolved by
// another caller) rather than overwriting a newer attempt's outcome.
func (c *Client) Complete(ctx context.Context, id string, attemptCount int) error {
	body, err := json.Marshal(struct {
		AttemptCount int    `json:"attemptCount"`
		Status       string `json:"status"`
	}{AttemptCount: attemptCount, Status: "succeeded"})
	if err != nil {
		return fmt.Errorf("ledger: encode Complete request: %w", err)
	}
	_, err = c.do(ctx, http.MethodPatch, "/scheduled-tasks/attempts/"+url.PathEscape(id), body)
	return err
}

// Fail calls PATCH /scheduled-tasks/attempts/{id} with status "failed".
// nextRetryOn is this client's own choice (see registry.Task.RetryBackoff)
// — entity-service has no backoff policy opinion of its own. attemptCount
// — see Complete's own doc comment; the same binding applies here.
func (c *Client) Fail(ctx context.Context, id string, attemptCount int, errMsg string, nextRetryOn time.Time) error {
	body, err := json.Marshal(struct {
		AttemptCount int       `json:"attemptCount"`
		Status       string    `json:"status"`
		Error        string    `json:"error"`
		NextRetryOn  time.Time `json:"nextRetryOn"`
	}{AttemptCount: attemptCount, Status: "failed", Error: errMsg, NextRetryOn: nextRetryOn})
	if err != nil {
		return fmt.Errorf("ledger: encode Fail request: %w", err)
	}
	_, err = c.do(ctx, http.MethodPatch, "/scheduled-tasks/attempts/"+url.PathEscape(id), body)
	return err
}

// DeleteResolvedBefore calls
// DELETE /scheduled-tasks/attempts?resolvedBefore=<cutoff> — deletes every
// row that succeeded or was superseded before cutoff (by its own
// resolution time, not when it was created; see entity-service's own
// CLAUDE.md, "Scheduled task runs"). A row still failed/retrying is never
// deleted regardless of age. Returns how many rows were removed.
func (c *Client) DeleteResolvedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	path := "/scheduled-tasks/attempts?resolvedBefore=" + url.QueryEscape(cutoff.Format(time.RFC3339))
	respBody, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		DeletedCount int `json:"deletedCount"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return 0, fmt.Errorf("ledger: decode DeleteResolvedBefore response: %w", err)
	}
	return resp.DeletedCount, nil
}
