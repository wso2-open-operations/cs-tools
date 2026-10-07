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

// Package slaengine is the SLA breach-alerting engine. entity-service's job
// is to trigger — it publishes the case.*/sla-duration-policy facts it
// already owns; this engine owns the actual policy interpretation and
// tracking. RegisterClocks (case.created) computes each clock's due dates
// itself, from a duration policy fetched once at startup (GetDurationPolicy
// below) and the case's own severity/creation time — ApplyStateEffects
// (case.status_changed) and CompleteResponseClock (case.comment_added, when
// IsSupportEngineerResponse is true) adjust them from there.
// RunTicker/Tick scan a Redis wake-index (see redis.go) for a newly-due
// 50/75/100% checkpoint and react (engine.go).
//
// This replaces the design that polled entity-service's GET /sla-status
// (backed by the ServiceNow-synced "sla"/"sla_policy" tables) in bulk, every
// tick — that endpoint's own OFFSET-paginated query recomputed a
// full-table DISTINCT ON/sort/join from scratch on every single page,
// genuinely too slow at the data volumes actually seen in production (each
// page measured 6-34+ seconds against real data), reliably tripping the
// gateway timeout between this service and entity-service. Before that, an
// even earlier design hand-registered a clock on a now-removed
// entity-service "sla_clocks" table and scheduled wake-ups off a
// locally-computed due date — this design revives that mechanism (the
// Redis wake-index is genuinely solid), without reviving that design's own
// dependency on a dedicated entity-service table/event: all clock state
// lives in Redis, which this service already depends on and entity-service
// doesn't touch. See this package's own CLAUDE.md section for the full
// history.
package slaengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/oauthhttp"
)

// EntityConfig holds the configuration for the entity-service client below.
// BaseURL/Scopes are this client's own SLA_ENTITY_* env vars; TokenURL/
// ClientID/ClientSecret are filled by cmd/server/main.go from whichever
// OAuth2 app is appropriate for this deployment.
type EntityConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// EntityClient is a narrow HTTP client for entity-service's
// GET /sla-duration-policy — the only entity-service call this engine makes
// now, and only once, at startup. Mirrors internal/entity.CustomerEntityClient's
// do()/OAuth2 shape exactly.
type EntityClient struct {
	http    *http.Client
	baseURL string
}

// NewEntityClient constructs an EntityClient authenticated via the OAuth2
// client credentials grant. Never fails and never contacts the token
// endpoint — a missing/invalid configuration only surfaces as an error the
// first time GetDurationPolicy is called.
func NewEntityClient(cfg EntityConfig) *EntityClient {
	httpClient := oauthhttp.NewClient(oauthhttp.Config{
		TokenURL:     cfg.TokenURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
	})

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

// slaDurationPolicyItem mirrors entity-service's domain.SLADurationPolicyItem.
type slaDurationPolicyItem struct {
	Severity        string `json:"severity"`
	ClockType       string `json:"clockType"`
	DurationSeconds int64  `json:"durationSeconds"`
}

// slaDurationPolicyResponse mirrors entity-service's domain.SLADurationPolicyResponse.
type slaDurationPolicyResponse struct {
	Policies []slaDurationPolicyItem `json:"policies"`
}

// GetDurationPolicy calls GET /sla-duration-policy and returns it as
// map[severity]map[clockType]time.Duration — severity is entity-service's
// own uppercase English word ("CATASTROPHIC"), matching
// events.CaseCreatedPayload.Priority exactly, so RegisterClocks needs no
// translation of its own to look a case's clocks up by its Priority field.
func (c *EntityClient) GetDurationPolicy(ctx context.Context) (map[string]map[string]time.Duration, error) {
	respBody, err := c.do(ctx, http.MethodGet, "/sla-duration-policy", nil)
	if err != nil {
		return nil, fmt.Errorf("slaengine: fetch sla duration policy: %w", err)
	}
	var resp slaDurationPolicyResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("slaengine: decode sla duration policy response: %w", err)
	}

	out := make(map[string]map[string]time.Duration, 5)
	for _, p := range resp.Policies {
		if out[p.Severity] == nil {
			out[p.Severity] = make(map[string]time.Duration, 3)
		}
		out[p.Severity][p.ClockType] = time.Duration(p.DurationSeconds) * time.Second
	}
	return out, nil
}

// activeSLAClock mirrors the one shape of entity-service's
// domain.SLAStatus this engine's reconciliation pass actually needs — a
// deliberately narrow subset (no BusinessElapsedPercent/HasBreached/the
// onboarding-routing or email-reaction fields: none of them feed
// RegisterClocks-equivalent state), decoded from the same wire response as
// every other field on that type.
type activeSLAClock struct {
	CaseID     string     `json:"caseId"`
	ClockType  string     `json:"clockType"`
	IsPaused   bool       `json:"isPaused"`
	StartedOn  *time.Time `json:"startedOn"`
	CaseNumber string     `json:"caseNumber"`
	WSO2CaseID string     `json:"wso2CaseId"`
	CaseTitle  string     `json:"caseTitle"`
	CaseType   string     `json:"caseType"`
	Product    string     `json:"product"`
	Team       string     `json:"team"`
	Priority   string     `json:"priority"`
	State      string     `json:"state"`
}

type searchActiveSLAStatusResponse struct {
	Statuses []activeSLAClock `json:"statuses"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
}

// activeSLAStatusPageSize is this client's own page size for
// GetActiveCSMSLAClocks — well under entity-service's own 2000-row cap
// (internal/service/sla_status_service.go's maxSLAStatusLimit), chosen
// purely so one slow page can't single-handedly reproduce the gateway
// timeout the abandoned poll design hit (see this package's own doc
// comment above) — this call is scoped to source=csm, a much smaller row
// set than that design ever had to page through, but there's no reason to
// risk a single, large round trip when several small ones cost nothing
// extra at startup.
const activeSLAStatusPageSize = 200

// GetActiveCSMSLAClocks calls GET /sla-status?source=csm, paging through
// every currently-open clock this engine's own entity-service counterpart
// (source='CSM' "sla" rows) is tracking, and returns the full, flattened
// list. Called once, from Engine.Reconcile, itself called once at process
// startup (see that method's own doc comment for why) — never on a
// recurring basis, unlike the poll design this package's own doc comment
// describes abandoning GET /sla-status for.
func (c *EntityClient) GetActiveCSMSLAClocks(ctx context.Context) ([]activeSLAClock, error) {
	var all []activeSLAClock
	offset := 0
	for {
		path := fmt.Sprintf("/sla-status?source=csm&limit=%d&offset=%d", activeSLAStatusPageSize, offset)
		respBody, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("slaengine: fetch active csm sla clocks (offset %d): %w", offset, err)
		}
		var resp searchActiveSLAStatusResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("slaengine: decode active csm sla clocks response (offset %d): %w", offset, err)
		}
		all = append(all, resp.Statuses...)
		offset += len(resp.Statuses)
		if len(resp.Statuses) == 0 || offset >= resp.Total {
			break
		}
	}
	return all, nil
}
