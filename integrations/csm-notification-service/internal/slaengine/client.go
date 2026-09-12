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

// Package slaengine is the SLA timer engine: it registers a per-case SLA
// clock on entity-service when it sees an events.TypeSLAClockRegister
// record, tracks 50%/75%/100% elapsed via a Redis wake index (see redis.go),
// and publishes events.TypeSLATierReached when a ticker finds a due entry
// (see engine.go). Ported from a standalone POC
// (internal/services/slatimerengine there), adapted to this repo's actual
// architecture: durable clock state lives in entity-service (a new
// sla_clocks table/API, this package's own HTTP client below) rather than a
// Postgres connection this service would own directly — this service has no
// database of its own, by design (see this package's own CLAUDE.md section).
package slaengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// EntityConfig holds the configuration for the entity-service client below.
// BaseURL/Scopes are this client's own SLA_ENTITY_* env vars; TokenURL/
// ClientID/ClientSecret are filled by cmd/server/main.go from whichever
// OAuth2 app is appropriate for this deployment — unlike
// internal/entity.CustomerEntityConfig, there's no existing shared-app
// precedent to follow here since this is a new, independent capability, so
// main.go is free to point it at the same shared OAUTH2_* app or a
// dedicated one.
type EntityConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// EntityClient is a narrow HTTP client for entity-service's sla_clocks
// endpoints — the entity-service half of this engine's durable state.
// Mirrors internal/entity.CustomerEntityClient's do()/OAuth2 shape exactly.
type EntityClient struct {
	http    *http.Client
	baseURL string
}

// NewEntityClient constructs an EntityClient authenticated via the OAuth2
// client credentials grant. Never fails and never contacts the token
// endpoint — a missing/invalid configuration only surfaces as an error the
// first time a method below is called.
func NewEntityClient(cfg EntityConfig) *EntityClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &EntityClient{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// do executes an authenticated HTTP request against entity-service and
// returns the raw JSON response body, or an *apierror.Error for a non-2xx
// status.
func (c *EntityClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("slaengine: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slaengine: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("slaengine: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

// Clock is the subset of entity-service's SLAClock response this engine
// needs: PausedOn is checked by Tick before firing a tier (see engine.go);
// nothing here reads the reached_*_at fields since SetTierReachedIfUnset's
// own response already reports what's needed after a write. Field names
// match entity-service's own response naming (timestamps use the "On"
// suffix there, not "At"). The eight display fields below are populated
// once at registration time (see RegisterClockRequest) and read back here
// so Engine.sendBreachAlert can build a Google Chat card from this one
// GetClock call, with no second lookup — this service has no other way to
// reach case data.
type Clock struct {
	PausedOn   *time.Time `json:"pausedOn"`
	StartedOn  time.Time  `json:"startedOn"`
	CaseNumber string     `json:"caseNumber"`
	WSO2CaseID string     `json:"wso2CaseId"`
	CaseTitle  string     `json:"caseTitle"`
	CaseType   string     `json:"caseType"`
	Product    string     `json:"product"`
	Team       string     `json:"team"`
	Priority   string     `json:"priority"`
	State      string     `json:"state"`
}

// RegisterClockRequest is RegisterClock's request — a struct rather than a
// growing positional-argument list, since registration now carries display
// data (see Clock's own doc comment) alongside the scheduling fields.
type RegisterClockRequest struct {
	CaseID    string
	ClockType string
	StartedAt time.Time
	DueAt     time.Time
	// The seven fields below are optional display data — see Clock's own
	// doc comment for what they're for.
	CaseNumber string
	WSO2CaseID string
	CaseTitle  string
	CaseType   string
	Product    string
	Team       string
	Priority   string
	State      string
}

// RegisterClock calls POST /cases/{caseId}/sla-clocks.
func (c *EntityClient) RegisterClock(ctx context.Context, req RegisterClockRequest) error {
	body, err := json.Marshal(struct {
		ClockType  string    `json:"clockType"`
		StartedAt  time.Time `json:"startedAt"`
		DueAt      time.Time `json:"dueAt"`
		CaseNumber string    `json:"caseNumber,omitempty"`
		WSO2CaseID string    `json:"wso2CaseId,omitempty"`
		CaseTitle  string    `json:"caseTitle,omitempty"`
		CaseType   string    `json:"caseType,omitempty"`
		Product    string    `json:"product,omitempty"`
		Team       string    `json:"team,omitempty"`
		Priority   string    `json:"priority,omitempty"`
		State      string    `json:"state,omitempty"`
	}{
		ClockType: req.ClockType, StartedAt: req.StartedAt, DueAt: req.DueAt,
		CaseNumber: req.CaseNumber, WSO2CaseID: req.WSO2CaseID, CaseTitle: req.CaseTitle,
		CaseType: req.CaseType, Product: req.Product, Team: req.Team, Priority: req.Priority,
		State: req.State,
	})
	if err != nil {
		return fmt.Errorf("slaengine: encode RegisterClock request: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, "/cases/"+url.PathEscape(req.CaseID)+"/sla-clocks", body)
	return err
}

// GetClock calls GET /cases/{caseId}/sla-clocks/{clockType}.
func (c *EntityClient) GetClock(ctx context.Context, caseID, clockType string) (Clock, error) {
	path := "/cases/" + url.PathEscape(caseID) + "/sla-clocks/" + url.PathEscape(clockType)
	respBody, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return Clock{}, err
	}
	var clock Clock
	if err := json.Unmarshal(respBody, &clock); err != nil {
		return Clock{}, fmt.Errorf("slaengine: decode GetClock response: %w", err)
	}
	return clock, nil
}

// SetTierReachedIfUnset calls
// PATCH /cases/{caseId}/sla-clocks/{clockType}/tiers/{tier} with
// {"status": "reached"} and returns the (possibly pre-existing) reached
// timestamp, plus alreadyReached: whether this call is the one that just
// recorded the tier (false) or it was already recorded by an earlier call
// (true). Callers MUST gate any reaction to the tier being reached (e.g.
// publishing a notification) on alreadyReached being false — see
// entity-service's own doc comment on this field for why: the underlying
// database write already atomically decides which caller "really" set it,
// even when two callers race for the same tier at the same time.
func (c *EntityClient) SetTierReachedIfUnset(ctx context.Context, caseID, clockType, tier string) (reachedAt time.Time, alreadyReached bool, err error) {
	body, err := json.Marshal(struct {
		Status string `json:"status"`
	}{Status: "reached"})
	if err != nil {
		return time.Time{}, false, fmt.Errorf("slaengine: encode SetTierReachedIfUnset request: %w", err)
	}
	path := "/cases/" + url.PathEscape(caseID) + "/sla-clocks/" + url.PathEscape(clockType) + "/tiers/" + url.PathEscape(tier)
	respBody, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return time.Time{}, false, err
	}
	var parsed struct {
		ReachedOn      time.Time `json:"reachedOn"`
		AlreadyReached bool      `json:"alreadyReached"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return time.Time{}, false, fmt.Errorf("slaengine: decode SetTierReachedIfUnset response: %w", err)
	}
	return parsed.ReachedOn, parsed.AlreadyReached, nil
}
