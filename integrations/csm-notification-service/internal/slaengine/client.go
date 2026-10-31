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
	"net/url"
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
// events.CaseCreatedPayload.Priority exactly.
//
// NOTE: entity-service dropped the static sla_duration_policy table GET
// /sla-duration-policy used to serve, but the endpoint itself is still
// live — it now computes its response from the real "sla_policy" table
// instead (see entity-service's own CLAUDE.md), and this Engine no longer
// calls this method at all (RegisterClocks/reconcileClock both get their
// duration from activeSLAClock.DurationSeconds/clockState.DurationSeconds
// instead — see each one's own doc comment). This method is kept,
// unchanged, purely because internal/paging's own rung-email feature
// (cmd/server/main.go's slaDurations/RungEmailSending.SLADurations) is a
// real, live consumer of it for its "response SLA remaining time" display —
// a deliberately separate, pre-existing feature this change does not
// touch. This was briefly, mistakenly assumed dead and the endpoint
// deleted entirely at one point, then restored once that was caught — see
// entity-service's own CLAUDE.md for that history.
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
	CaseID    string     `json:"caseId"`
	ClockType string     `json:"clockType"`
	IsPaused  bool       `json:"isPaused"`
	StartedOn *time.Time `json:"startedOn"`
	// PausedOn/PauseDurationSeconds back reconcileClock's own pause-aware
	// due-date math — see that function's own doc comment. PausedOn is only
	// meaningful alongside IsPaused=true (the currently-open pause segment's
	// own start); PauseDurationSeconds is the clock's total ALREADY-completed
	// pause time, same field entity-service's domain.SLAStatus carries.
	PausedOn             *time.Time `json:"pausedOn"`
	PauseDurationSeconds int64      `json:"pauseDurationSeconds"`
	// DurationSeconds is the clock's own resolved sla_policy duration
	// (entity-service's domain.SLAStatus.DurationSeconds) — reconcileClock
	// reads this directly instead of the static, severity-keyed duration
	// table GET /sla-duration-policy used to serve (that endpoint is still
	// live, just repointed to sla_policy now — see GetDurationPolicy's own
	// doc comment), since this bulk fetch already carries it with no extra
	// round trip needed.
	DurationSeconds int64 `json:"durationSeconds"`
	// Schedule mirrors entity-service's own domain.SLAStatus.Schedule (e.g.
	// "WSO2 Business days", "24 x 7", or "") — reconcileClock uses this to
	// decide whether to anchor the clock's due date/wake-ups on
	// businessdays.AddBusinessDuration instead of flat wall-clock
	// start.Add(duration) math. See RegisterClocks' own doc comment for the
	// same decision on the registration path.
	Schedule   string `json:"schedule"`
	CaseNumber string `json:"caseNumber"`
	WSO2CaseID string `json:"wso2CaseId"`
	CaseTitle  string `json:"caseTitle"`
	CaseType   string `json:"caseType"`
	Product    string `json:"product"`
	Team       string `json:"team"`
	Priority   string `json:"priority"`
	State      string `json:"state"`
}

type searchActiveSLAStatusResponse struct {
	Statuses []activeSLAClock `json:"statuses"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
}

// slaEngineSourceFilter is the sla.source value entity-service's own
// in-process SLA engine tags every clock it registers — 'csm', migration
// 0134's own original, unchanged intent: 'CSM' rows are owned by that
// in-process engine, 'SERVICENOW' rows are owned by a real, independent,
// continuously-running external sync job that populates sla_policy/sla.
// Every call in this file that scopes a GET /sla-status(/clock-state) read
// to this engine's own rows must use this constant, not a literal, so the
// two can never drift apart — setting it to the sync job's own tag would
// make GetActiveCSMSLAClocks/GetClockState scope to rows that job owns
// instead, letting a pre-alert check pick up an unrelated synced row for the
// same case, or the bulk reconciliation pass ingest sync-owned rows as if
// they were this engine's own.
const slaEngineSourceFilter = "csm"

// activeSLAStatusPageSize is this client's own page size for
// GetActiveCSMSLAClocks — well under entity-service's own 2000-row cap
// (internal/service/sla_status_service.go's maxSLAStatusLimit), chosen
// purely so one slow page can't single-handedly reproduce the gateway
// timeout the abandoned poll design hit (see this package's own doc
// comment above) — this call is scoped to source=csm (see
// slaEngineSourceFilter), a much smaller row set than that design ever had
// to page through, but there's no reason to risk a single, large round trip
// when several small ones cost nothing extra at startup.
const activeSLAStatusPageSize = 200

// GetActiveCSMSLAClocks calls GET /sla-status?source=csm, paging
// through every currently-open clock this engine's own entity-service
// counterpart is tracking, and returns the full, flattened list. Called
// once, from Engine.Reconcile, itself called once at process startup (see
// that method's own doc comment for why) — never on a recurring basis,
// unlike the poll design this package's own doc comment describes
// abandoning GET /sla-status for.
func (c *EntityClient) GetActiveCSMSLAClocks(ctx context.Context) ([]activeSLAClock, error) {
	var all []activeSLAClock
	offset := 0
	for {
		path := fmt.Sprintf("/sla-status?source=%s&limit=%d&offset=%d", slaEngineSourceFilter, activeSLAStatusPageSize, offset)
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

// clockState mirrors entity-service's domain.SLAClockState — the full,
// current state of one (work item, clock target) "sla" row, regardless of
// whether it is currently active. Found is false, every other field at its
// zero value, when no row exists for this (workItemID, target, source) at
// all — see GetClockState's own doc comment for why that distinction
// matters here specifically.
type clockState struct {
	Found       bool       `json:"found"`
	IsActive    bool       `json:"isActive"`
	Stage       string     `json:"stage"`
	HasBreached bool       `json:"hasBreached"`
	StartedOn   *time.Time `json:"startedOn"`
	// PausedOn/PauseDurationSeconds mirror entity-service's own
	// domain.SLAClockState fields of the same name — used by
	// Engine.resumeShift to compute how far a just-resumed clock's
	// not-yet-alerted wake entries need pushing forward.
	PausedOn             *time.Time `json:"pausedOn"`
	PauseDurationSeconds int64      `json:"pauseDurationSeconds"`
	// IsPaused mirrors entity-service's own domain.SLAClockState.IsPaused
	// (Stage == "PAUSED", spelled out as its own bool) — used by
	// Engine.entityPauseStatus to derive a clock's real pause status
	// directly from entity-service's own record, replacing a hardcoded
	// match against entity-service's human case-state labels (see that
	// function's own doc comment for why those two can disagree).
	IsPaused bool `json:"isPaused"`
	// BusinessElapsedPercent mirrors entity-service's own
	// domain.SLAClockState.BusinessElapsedPercent — the live, business-day-
	// aware elapsed percentage for a still-running clock. Used by the
	// Redis-outage fallback path (Engine.processDueMemberFallback) to decide
	// which tier, if any, is newly due when this engine's own Redis-held
	// alertedTier cursor can't be read at all.
	BusinessElapsedPercent float64 `json:"businessElapsedPercent"`
	// DurationSeconds mirrors entity-service's own
	// domain.SLAClockState.DurationSeconds — the clock's real, resolved
	// policy duration. RegisterClocks reads this to schedule tier
	// checkpoints from the real per-policy duration instead of the static,
	// severity-keyed table GET /sla-duration-policy used to serve (still
	// live, repointed to sla_policy now — see GetDurationPolicy's own doc
	// comment).
	DurationSeconds int64 `json:"durationSeconds"`
	// Schedule mirrors entity-service's own
	// domain.SLAClockState.Schedule — see activeSLAClock.Schedule's own doc
	// comment for the full reasoning (business-day-aware due-date math).
	Schedule string `json:"schedule"`
	// BusinessPauseDurationSeconds mirrors entity-service's own
	// domain.SLAClockState.BusinessPauseDurationSeconds — the clock's total
	// business-hours-aware pause time, distinct from PauseDurationSeconds'
	// own flat wall-clock total. Engine.resumeShift uses this (not
	// PauseDurationSeconds) to recompute a business-day-scheduled clock's
	// wake-up schedule after a resume — shifting by the flat total instead
	// produced wake times off by however much of the pause fell on a
	// weekend.
	BusinessPauseDurationSeconds int64 `json:"businessPauseDurationSeconds"`
}

// GetClockState calls GET /sla-status/clock-state?workItemId=<caseID>&target=<clockType>&source=csm
// and returns entity-service's own durable record for this exact clock,
// right before a breach alert for it is actually sent (Engine.
// sendBreachAlert) — a last, authoritative check against the real source of
// truth, independent of whatever this engine's own Redis copy of the
// clock's alertedTier cursor says. This exists because that Redis cursor is
// not always reliably advanced: ApplyStateEffects'/CompleteResponseClock's
// own AdvanceAlertedTier write on a case closing or a qualifying comment
// landing is a separate, non-atomic Redis write from whatever triggered it
// (a Kafka-consumed case.* event), and a confirmed, reproduced incident
// showed it can be silently lost with no retry — see this package's own
// CLAUDE.md, "ApplyStateEffects" — leaving stale wake entries to fire a
// false breach alert later for a clock that had, in truth, already
// completed cleanly in entity-service's own record.
//
// Deliberately NOT a simple bool: a CodeRabbit review on the first version
// of this check (a plain "is this clock type still in the active list"
// query) caught two real correctness gaps that a bare true/false answer
// can't fix:
//  1. entity-service's CSM clock registration is itself best-effort (see
//     SLAEngineService's own doc comment) and runs on a different trigger
//     than this engine's own Redis registration -- a work item with NO row
//     at all for this clock (Found=false) could mean either "genuinely
//     never registered there" (unconfirmed, not evidence of resolution) or
//     a coincidental, momentary gap; neither should be read as "resolved".
//  2. A severity revision cancels the old clock and registers a fresh one
//     for the same (work item, target) pair -- an "any active row of this
//     type" check would accept the NEW incarnation as confirming the OLD,
//     Redis-tracked one (the one the firing wake entry is actually about)
//     is still fine, which it says nothing about. StartedOn lets the call
//     site compare incarnations instead of just clock types.
//
// See the call site's own doc comment (Engine.sendBreachAlert) for exactly
// how Found/IsActive/Stage/HasBreached/StartedOn are combined into a
// suppress/send decision — this method itself makes no such decision, it
// only reports what entity-service's record says. A decode/transport
// failure returns a zero clockState and the error; the call site's own doc
// comment explains why that fails open (sends anyway) rather than
// suppressing a possibly-real alert over a transient entity-service hiccup.
func (c *EntityClient) GetClockState(ctx context.Context, caseID, clockType string) (clockState, error) {
	path := fmt.Sprintf("/sla-status/clock-state?workItemId=%s&target=%s&source=%s", url.QueryEscape(caseID), url.QueryEscape(clockType), slaEngineSourceFilter)
	respBody, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return clockState{}, fmt.Errorf("slaengine: get clock state (case %s, %s): %w", caseID, clockType, err)
	}
	var resp clockState
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return clockState{}, fmt.Errorf("slaengine: decode clock state response (case %s, %s): %w", caseID, clockType, err)
	}
	return resp, nil
}
