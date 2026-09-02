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

// Package worker is the background retry/escalation loop: it periodically
// scans internal/store's alert_buffer for rows due for another delivery
// attempt against csm-integration-service, and escalates via Twilio once a
// row's retry budget is exhausted. This is the piece that makes buffering
// meaningful — without it, a persisted-but-undelivered alert would sit in
// Postgres forever.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/alertpayload"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/backoff"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/csmclient"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/store"
)

// Store is the subset of internal/store.Store the worker depends on.
type Store interface {
	PendingBatch(ctx context.Context, limit int) ([]store.AlertRecord, error)
	MarkDelivered(ctx context.Context, id, incidentID string) error
	MarkAttemptFailed(ctx context.Context, id, lastError string) error
	MarkEscalated(ctx context.Context, id, lastError string) error
	MarkFailed(ctx context.Context, id, lastError string) error
}

// IncidentCreator is the subset of internal/csmclient.Client the worker
// depends on: creating an incident from a buffered alert, the pre-retry
// dedup check before doing so again for a row that already failed once, and
// the incident-grouping lookup/confirm/record calls (see tryGroup). All live
// on the same interface (rather than separate constructor parameters)
// because internal/csmclient.Client always implements all of them, and
// there is no scenario in which the worker has some without the others.
type IncidentCreator interface {
	CreateIncident(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error)
	// SearchIncidentByTag looks up whether an incident tagged with tag
	// (csmclient.DedupTag) already exists. See attempt's doc comment for
	// when and why this is called, and the fail-open behavior on error.
	SearchIncidentByTag(ctx context.Context, tag string) (*csmclient.CreateIncidentResult, bool, error)
	// SearchOpenIncidentByNumber confirms whether the incident identified by
	// number is still open (not Resolved/Closed/Cancelled). See tryGroup for
	// when this is called and the fail-open behavior on error.
	SearchOpenIncidentByNumber(ctx context.Context, number string) (*csmclient.CreateIncidentResult, bool, error)
	// LookupAlertIncidentMappings finds any earlier alert(s) recorded against
	// (source, uniqueIdentifier), most-recent-first. See tryGroup.
	LookupAlertIncidentMappings(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error)
	// CreateAlertIncidentMapping records one alert against the incident it
	// ended up delivered to. See tryGroup and attempt's post-create call.
	CreateAlertIncidentMapping(ctx context.Context, req csmclient.CreateAlertIncidentMappingRequest) (*csmclient.AlertIncidentMappingView, error)
}

// Escalator is the subset of internal/notifications.TwilioClient the worker
// depends on.
type Escalator interface {
	Escalate(ctx context.Context, message string) error
}

// Config tunes the worker's polling and retry behavior.
type Config struct {
	// MaxRetries is the number of failed, retryable attempts a buffered
	// alert gets before the Twilio escalation call fires and the row is
	// marked escalated. Defaults to 3 if <= 0.
	MaxRetries int
	// BatchSize caps how many pending rows a single scan loads from the
	// store. Defaults to 50 if <= 0.
	BatchSize int
	// PollInterval is how often RunOnce is invoked by Run. Defaults to 15s
	// if <= 0. This is independent of internal/backoff's per-row delay —
	// PollInterval controls how often the worker *looks*, backoff controls
	// which rows it's willing to *act on* once it looks.
	PollInterval time.Duration
}

func (c Config) withDefaults() Config {
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 50
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 15 * time.Second
	}
	return c
}

// Worker runs the periodic buffer scan.
type Worker struct {
	store  Store
	csm    IncidentCreator
	twilio Escalator
	cfg    Config
	// now is overridden in tests for deterministic backoff-due checks.
	now func() time.Time
}

// New constructs a Worker. store, csm, and twilio must be non-nil.
func New(s Store, csm IncidentCreator, twilio Escalator, cfg Config) *Worker {
	return &Worker{
		store:  s,
		csm:    csm,
		twilio: twilio,
		cfg:    cfg.withDefaults(),
		now:    time.Now,
	}
}

// Run blocks, invoking RunOnce every cfg.PollInterval until ctx is
// cancelled. Intended to be started in its own goroutine from main.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce loads one batch of pending rows and attempts delivery for every
// one that's due per internal/backoff, given its RetryCount/LastAttemptAt.
// Exported (not just reachable via Run) specifically so tests can invoke a
// single scan synchronously without a ticker.
func (w *Worker) RunOnce(ctx context.Context) {
	rows, err := w.store.PendingBatch(ctx, w.cfg.BatchSize)
	if err != nil {
		slog.ErrorContext(ctx, "worker: failed to load pending batch", "err", err)
		return
	}

	now := w.now()
	for _, row := range rows {
		if !backoff.Due(now, row.LastAttemptAt, row.RetryCount) {
			continue
		}
		w.attempt(ctx, row)
	}
}

// attempt makes one delivery attempt for row and applies the resulting
// store transition — deliver, retry, escalate, or terminal-fail — per the
// classification in isRetryable.
func (w *Worker) attempt(ctx context.Context, row store.AlertRecord) {
	// Each attempt gets its own fresh correlation ID rather than reusing the
	// one (if any) from the original POST /alerts request: that request
	// completed and returned to its caller long ago (this service persists
	// before attempting delivery — see internal/handler), and an attempt
	// happening minutes or hours later, on the worker's own schedule, is a
	// distinct traceable event.
	attemptCtx := middleware.WithCorrelationID(ctx, middleware.NewCorrelationID())

	var bp alertpayload.Payload
	if err := json.Unmarshal(row.Payload, &bp); err != nil {
		// The buffered payload itself is corrupt. No retry can ever fix
		// this — it isn't a CSM-availability problem — so this is terminal,
		// and specifically does not reach Twilio escalation (that channel
		// exists for "CSM won't accept this", not "we can't even ask").
		slog.ErrorContext(attemptCtx, "worker: buffered payload is not valid JSON, marking failed", "id", row.ID, "alertNumber", row.AlertNumber, "err", err)
		if merr := w.store.MarkFailed(ctx, row.ID, "corrupt buffered payload: "+err.Error()); merr != nil {
			slog.ErrorContext(attemptCtx, "worker: MarkFailed failed", "id", row.ID, "err", merr)
		}
		return
	}
	req := bp.CreateIncidentRequest

	// Pre-retry dedup check: only on a retry (row.RetryCount > 0), never on
	// the first attempt — on attempt 1 nothing could possibly exist yet for
	// this row, so searching first would just be a wasted call. From the
	// second attempt onward, a previous CreateIncident call may have
	// actually succeeded upstream even though *this* service recorded it as
	// a failure: the request can complete on the far side while the
	// response is lost to a timeout or connection reset. Blindly retrying
	// in that case risks creating a second, duplicate incident for the same
	// alert. tag is the same csmclient.DedupTag(row.AlertNumber) value
	// internal/handler.buildSubject already stamped into this row's own
	// CreateIncidentRequest.Subject when it was first buffered — row.AlertNumber
	// is stable for the row's lifetime (this service's own Postgres
	// sequence, assigned once at Enqueue time), so it's always the right tag
	// to search for here, without needing to re-parse it out of req.Subject.
	//
	// Fail-open, deliberately: if the search call itself errors (including
	// the 401 this endpoint also currently always returns — see this
	// package's CLAUDE.md and internal/csmclient/search.go's doc comment),
	// that is "we couldn't confirm either way," not "assume a duplicate
	// exists." The safe default here is to proceed to attempt delivery, the
	// same as if this check didn't exist at all — never silently give up on
	// a buffered alert just because the confirmation step itself failed.
	if row.RetryCount > 0 {
		tag := csmclient.DedupTag(row.AlertNumber)
		if existing, found, serr := w.csm.SearchIncidentByTag(attemptCtx, tag); serr != nil {
			slog.WarnContext(attemptCtx, "worker: pre-retry dedup search failed, proceeding to attempt delivery (fail-open)", "id", row.ID, "alertNumber", row.AlertNumber, "err", serr)
		} else if found {
			slog.InfoContext(attemptCtx, "worker: found an existing incident for this alert from an earlier attempt, skipping duplicate create", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", existing.IncidentID, "incidentNumber", existing.IncidentNumber)
			if merr := w.store.MarkDelivered(ctx, row.ID, existing.IncidentID); merr != nil {
				slog.ErrorContext(attemptCtx, "worker: MarkDelivered (post-dedup) failed", "id", row.ID, "err", merr)
			}
			return
		}
	}

	// Incident-grouping check: only ever relevant for an alert whose inbound
	// payload carried a vendor-supplied UniqueIdentifier — nothing to group
	// on otherwise (see tryGroup's doc comment for the full contract and its
	// fail-open behavior). Deliberately run before the create-or-dedup-search
	// flow below: if an earlier alert's still-open incident is found, this
	// alert attaches to it and CreateIncident is never called at all.
	if bp.UniqueIdentifier != "" {
		if incidentID, _, grouped := w.tryGroup(attemptCtx, row, bp); grouped {
			if merr := w.store.MarkDelivered(ctx, row.ID, incidentID); merr != nil {
				slog.ErrorContext(attemptCtx, "worker: MarkDelivered (post-grouping) failed", "id", row.ID, "err", merr)
			}
			return
		}
	}

	result, err := w.csm.CreateIncident(attemptCtx, req)
	if err == nil {
		if merr := w.store.MarkDelivered(ctx, row.ID, result.IncidentID); merr != nil {
			slog.ErrorContext(attemptCtx, "worker: MarkDelivered failed", "id", row.ID, "err", merr)
			return
		}
		slog.InfoContext(attemptCtx, "worker: alert delivered", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", result.IncidentID, "incidentNumber", result.IncidentNumber)

		// Best-effort, non-blocking: record this alert against the incident
		// just created, so a future related alert (same source+uniqueIdentifier)
		// can find and group onto it via tryGroup. The primary goal — an
		// incident exists, and this row is marked delivered — is already
		// achieved above regardless of whether this call succeeds; see
		// recordMapping's doc comment.
		w.recordMapping(attemptCtx, row, bp, result.IncidentID, result.IncidentNumber)
		return
	}

	if !isRetryable(err) {
		slog.ErrorContext(attemptCtx, "worker: non-retryable error, marking failed", "id", row.ID, "alertNumber", row.AlertNumber, "err", err)
		if merr := w.store.MarkFailed(ctx, row.ID, err.Error()); merr != nil {
			slog.ErrorContext(attemptCtx, "worker: MarkFailed failed", "id", row.ID, "err", merr)
		}
		return
	}

	nextRetryCount := row.RetryCount + 1
	if nextRetryCount >= w.cfg.MaxRetries {
		slog.WarnContext(attemptCtx, "worker: retry budget exhausted, escalating", "id", row.ID, "alertNumber", row.AlertNumber, "retryCount", nextRetryCount, "maxRetries", w.cfg.MaxRetries, "err", err)
		if merr := w.store.MarkEscalated(ctx, row.ID, err.Error()); merr != nil {
			slog.ErrorContext(attemptCtx, "worker: MarkEscalated (store) failed", "id", row.ID, "err", merr)
		}
		// row.AlertNumber leads the message (it's this alert's externally-facing
		// identifier — see internal/store.PostgresStore.Enqueue); row.ID follows
		// for anyone cross-referencing this service's own logs/database directly.
		message := fmt.Sprintf(
			"SRE alert ingestion service: alert %s (id %s) could not be delivered to CSM after %d attempts. Last error: %s",
			row.AlertNumber, row.ID, nextRetryCount, truncate(err.Error(), 200),
		)
		if terr := w.twilio.Escalate(ctx, message); terr != nil {
			// The escalation call itself failing is the worst case this
			// service can be in — CSM is unreachable *and* the
			// CSM-independent notification channel just failed too. There
			// is no further fallback by design (see this service's
			// README/CLAUDE.md); log loudly and move on rather than retry
			// the call in a tight loop against Twilio.
			slog.ErrorContext(attemptCtx, "worker: twilio escalation call failed", "id", row.ID, "err", terr)
		}
		return
	}

	slog.WarnContext(attemptCtx, "worker: delivery attempt failed, will retry", "id", row.ID, "alertNumber", row.AlertNumber, "retryCount", nextRetryCount, "nextDelay", backoff.Delay(nextRetryCount-1).String(), "err", err)
	if merr := w.store.MarkAttemptFailed(ctx, row.ID, err.Error()); merr != nil {
		slog.ErrorContext(attemptCtx, "worker: MarkAttemptFailed failed", "id", row.ID, "err", merr)
	}
}

// tryGroup implements this alert's incident-grouping check: an earlier
// alert reporting the same (source, uniqueIdentifier) condition may already
// have an open incident, in which case this alert should attach to it (via
// a recorded alert-incident-mapping row) instead of a new one being
// created. Called only when bp.UniqueIdentifier is non-empty (see attempt).
//
// Returns grouped=true, plus the existing incident's id/number, only when
// both the lookup and the open-state confirmation succeed. Any failure
// anywhere along this path — the lookup call erroring, no mapping found, a
// mapping found but with no recorded incident number to confirm against, or
// the open-state confirmation call itself erroring or finding the incident
// no longer open — returns grouped=false, so attempt() falls through to the
// existing create-or-dedup-search flow unchanged. This mirrors the same
// fail-open philosophy as the pre-retry dedup check above: "we couldn't
// confirm this is groupable" is never treated as "assume it is."
//
// This is a known, accepted v1 limitation, not a bug: the open-state
// confirmation (SearchOpenIncidentByNumber) is ServiceNow-backed, like every
// other csmclient search call in this service, and 401s on every call today
// (see internal/csmclient/search.go and this service's CLAUDE.md) — so in
// production this always falls open to "not groupable, proceed as before"
// until that infrastructure gap is closed. The grouping feature itself is
// structurally complete and ready for that day; it does not attempt to work
// around the gap.
func (w *Worker) tryGroup(ctx context.Context, row store.AlertRecord, bp alertpayload.Payload) (incidentID, incidentNumber string, grouped bool) {
	mappings, err := w.csm.LookupAlertIncidentMappings(ctx, bp.Source, bp.UniqueIdentifier)
	if err != nil {
		slog.WarnContext(ctx, "worker: incident-grouping lookup failed, proceeding without grouping (fail-open)", "id", row.ID, "alertNumber", row.AlertNumber, "err", err)
		return "", "", false
	}
	if len(mappings) == 0 {
		return "", "", false
	}

	// mappings is most-recent-first per the lookup contract.
	latest := mappings[0]
	if latest.IncidentNumber == nil || *latest.IncidentNumber == "" {
		slog.WarnContext(ctx, "worker: incident-grouping match has no recorded incident number, cannot confirm open state, proceeding without grouping", "id", row.ID, "alertNumber", row.AlertNumber, "matchedMappingID", latest.ID)
		return "", "", false
	}

	existing, found, serr := w.csm.SearchOpenIncidentByNumber(ctx, *latest.IncidentNumber)
	if serr != nil {
		slog.WarnContext(ctx, "worker: incident-grouping open-state confirmation failed, proceeding without grouping (known limitation, fail-open — see tryGroup doc comment)", "id", row.ID, "alertNumber", row.AlertNumber, "err", serr)
		return "", "", false
	}
	if !found {
		slog.InfoContext(ctx, "worker: incident-grouping match's incident is not open (resolved/closed/cancelled), proceeding without grouping", "id", row.ID, "alertNumber", row.AlertNumber, "incidentNumber", *latest.IncidentNumber)
		return "", "", false
	}

	slog.InfoContext(ctx, "worker: grouping alert onto an earlier alert's still-open incident", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", existing.IncidentID, "incidentNumber", existing.IncidentNumber)
	w.recordMapping(ctx, row, bp, existing.IncidentID, existing.IncidentNumber)
	return existing.IncidentID, existing.IncidentNumber, true
}

// recordMapping calls csmclient.CreateAlertIncidentMapping to record row
// against incidentID/incidentNumber. Best-effort and non-blocking by
// design: every call site treats a failure here purely as a logged
// warning, never as a reason to fail the overall delivery or hold back
// Store.MarkDelivered — the primary goal (an incident exists, and this
// alert is attached to it) is already achieved by the time this is called.
// A missed mapping row just means a future related alert won't find this
// group and will create its own incident instead — a known, documented
// degradation, not a correctness bug (see this method's callers in attempt
// and tryGroup).
//
// A 409 (already recorded) surfaces from csmclient as (nil, nil), not an
// error — see CreateAlertIncidentMapping's doc comment — so it logs nothing
// here and is treated the same as a clean success.
func (w *Worker) recordMapping(ctx context.Context, row store.AlertRecord, bp alertpayload.Payload, incidentID, incidentNumber string) {
	req := csmclient.CreateAlertIncidentMappingRequest{
		AlertNumber:      row.AlertNumber,
		Source:           bp.Source,
		UniqueIdentifier: strOrNil(bp.UniqueIdentifier),
		Service:          strOrNil(bp.Service),
		MetricName:       strOrNil(bp.MetricName),
		AlertStatus:      bp.AlertStatus,
		IncidentID:       incidentID,
		IncidentNumber:   strOrNil(incidentNumber),
	}
	if _, err := w.csm.CreateAlertIncidentMapping(ctx, req); err != nil {
		slog.WarnContext(ctx, "worker: failed to record alert-incident-mapping (best-effort, non-blocking — incident delivery already succeeded)", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", incidentID, "err", err)
	}
}

// strOrNil returns nil for an empty string, else a pointer to s — for
// populating the optional *string fields on
// csmclient.CreateAlertIncidentMappingRequest from bp's plain string fields.
func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isRetryable classifies an error returned by IncidentCreator.CreateIncident.
//
// A 400 means CSM rejected the request payload itself as invalid — retrying
// the exact same payload can never succeed, so this is treated as a
// terminal, non-retryable failure (Store.MarkFailed), distinct from every
// other case.
//
// Every other error is retryable — critically including a 401, which this
// service treats as retryable *by design*, not by oversight. A 401 would
// normally signal "the caller isn't authorized, stop retrying" for a
// typical client. Here it means something different: csm-integration-service
// is M2M-only and the upstream entity-service incident-creation operation
// is ServiceNow-backed, requiring a forwarded end-user identity token this
// stack cannot currently supply (see internal/csmclient/incidents.go's
// CreateIncident doc comment, and this service's own CLAUDE.md). That is a
// known, currently-permanent state of CSM's own capability, not a
// per-request auth failure — and it's exactly the kind of
// "CSM-side-unavailability" condition this whole service exists to buffer
// and retry through, all the way to Twilio escalation if it doesn't
// resolve. Treating it as non-retryable would silently drop every alert
// this service ever ingests today.
//
// Network/transport-level errors (timeout, connection refused, DNS
// failure, TLS error — anything that never got an HTTP response to
// classify) are also always retryable: they are unambiguously
// CSM-side-unavailability signals.
func isRetryable(err error) bool {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode != http.StatusBadRequest
	}
	return true
}

// truncate bounds s to at most n runes-as-bytes for inclusion in a
// human-read-aloud Twilio message and in logs, appending "..." when
// truncated so it's visibly incomplete rather than silently cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
