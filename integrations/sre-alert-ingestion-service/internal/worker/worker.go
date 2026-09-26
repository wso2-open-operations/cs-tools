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
	"strings"
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
	RecordIncidentID(ctx context.Context, id, incidentID string) error
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
	// SearchOpenIncidentByGroupTag looks up whether an earlier alert
	// reporting the same (source, uniqueIdentifier) condition already has a
	// still-open incident created within Config.GroupWindow. See tryGroup
	// for when this is called and the fail-open behavior on error.
	SearchOpenIncidentByGroupTag(ctx context.Context, tag string, since time.Time) (*csmclient.CreateIncidentResult, bool, error)
	// CreateAlertIncidentMapping records one alert against the incident it
	// ended up delivered to — a best-effort CSM-side audit trail of the
	// alert->incident relationship, kept for visibility even though the
	// grouping *decision* itself (tryGroup, above) no longer depends on
	// this being readable. See tryGroup and attempt's post-create call.
	CreateAlertIncidentMapping(ctx context.Context, req csmclient.CreateAlertIncidentMappingRequest) (*csmclient.AlertIncidentMappingView, error)
	// SearchServices looks up a CMDB service UUID by exact-match label. See
	// resolveServiceID for when and why this is called, and
	// csmclient.Client.SearchServices's own doc comment for the full
	// contract (an empty, error-free result is a confirmed zero-result
	// search, not a failure).
	SearchServices(ctx context.Context, label string) ([]csmclient.ITService, error)
	// UpdateIncident pushes workNotes onto an already-existing incident. See
	// pushGroupAttachWorkNote's doc comment for when and why this is called,
	// and its best-effort, non-blocking contract.
	UpdateIncident(ctx context.Context, incidentID, workNotes string) error
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
	// GroupWindow bounds how far back tryGroup's incident search looks for
	// an earlier alert's still-open incident to attach to — an alert whose
	// matching incident was created before now-GroupWindow is treated as
	// not groupable, even if it's still open. Defaults to 15 minutes if
	// <= 0, the one concrete parameter carried over from a ServiceNow prod
	// flow design this mirrors in spirit (see csmclient.GroupTag's doc
	// comment for what was and wasn't actually portable from it).
	GroupWindow time.Duration
	// UnknownServiceID is the operator-configured CMDB "Unclassified"/
	// catch-all service UUID (SRE_ALERT_UNKNOWN_SERVICE_ID) resolveServiceID
	// falls back to when a live /services/search for a row's raw Service
	// label returns a confirmed zero-result match. Required, non-empty
	// config — mustEnv'd at startup in cmd/server/main.go, matching
	// SRE_ALERT_CALLER_ID's precedent (see AlertHandler.callerID's doc
	// comment): a deployment that never expects an unmapped service can
	// still set SRE_ALERT_SERVICE_MAP to cover every label it sends, making
	// this purely a safety net, not an operational burden.
	UnknownServiceID string
	// ServiceCacheTTL bounds how long a successful resolveServiceID result
	// is reused before the next alert for the same label triggers a fresh
	// /services/search call. Defaults to 15 minutes if <= 0. See
	// serviceCache's doc comment for why only a successful resolution is
	// ever cached.
	ServiceCacheTTL time.Duration
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
	if c.GroupWindow <= 0 {
		c.GroupWindow = 15 * time.Minute
	}
	if c.ServiceCacheTTL <= 0 {
		c.ServiceCacheTTL = 15 * time.Minute
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
	// svcCache is resolveServiceID's label->UUID cache. See serviceCache's
	// own doc comment.
	svcCache *serviceCache
}

// New constructs a Worker. store, csm, and twilio must be non-nil.
func New(s Store, csm IncidentCreator, twilio Escalator, cfg Config) *Worker {
	cfg = cfg.withDefaults()
	return &Worker{
		store:    s,
		csm:      csm,
		twilio:   twilio,
		cfg:      cfg,
		now:      time.Now,
		svcCache: newServiceCache(cfg.ServiceCacheTTL),
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

	// Durable short-circuit: if a prior attempt's CreateIncident already
	// succeeded and recorded this row's IncidentID (via RecordIncidentID,
	// see below and that method's doc comment) but MarkDelivered failed
	// before it could mark the row terminal, do NOT call CreateIncident
	// again — the incident is confirmed to already exist, durably, in this
	// service's own database, independent of CSM's or SearchIncidentByTag's
	// availability. Just retry MarkDelivered directly. This is the actual
	// fix for the gap the fallback-to-MarkAttemptFailed path further below
	// only partially mitigated (that path still relied on SearchIncidentByTag,
	// which 401s today and fails open to creating a second incident) — this
	// check needs no network call to CSM at all, so it cannot fail open.
	if row.IncidentID != "" {
		if merr := w.store.MarkDelivered(ctx, row.ID, row.IncidentID); merr != nil {
			slog.ErrorContext(attemptCtx, "worker: retrying MarkDelivered for an already-recorded incident failed; will retry again next scan", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", row.IncidentID, "err", merr)
			// Bump RetryCount so this doesn't retry MarkDelivered forever
			// with no bound: if this service's own database is genuinely
			// unable to complete the update, that's a real operational
			// problem distinct from CSM's availability, and should still
			// eventually reach the normal retry-budget/escalation path
			// rather than loop indefinitely.
			//
			// That budget check has to happen HERE, not by falling through
			// to the normal path below: this branch always returns, so
			// nothing past it (including the nextRetryCount check further
			// down) ever runs for this row. Without this, a row stuck here
			// retries MarkDelivered forever and never escalates, contrary
			// to the comment above -- CreateIncident is never called again
			// for a row with IncidentID already set, so it can't reach the
			// budget check any other way.
			if row.RetryCount+1 >= w.cfg.MaxRetries {
				slog.WarnContext(attemptCtx, "worker: retry budget exhausted retrying MarkDelivered for an already-recorded incident, escalating", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", row.IncidentID, "retryCount", row.RetryCount+1, "maxRetries", w.cfg.MaxRetries, "err", merr)
				message := fmt.Sprintf(
					"SRE alert ingestion service: alert %s (id %s) has incident %s created, but this service could not mark it delivered after %d attempts. Last error: %s",
					row.AlertNumber, row.ID, row.IncidentID, row.RetryCount+1, truncate(merr.Error(), 200),
				)
				if terr := w.twilio.Escalate(ctx, message); terr != nil {
					slog.ErrorContext(attemptCtx, "worker: twilio escalation call failed", "id", row.ID, "err", terr)
				}
				if eerr := w.store.MarkEscalated(ctx, row.ID, merr.Error()); eerr != nil {
					slog.ErrorContext(attemptCtx, "worker: MarkEscalated (post-MarkDelivered-retry) failed", "id", row.ID, "err", eerr)
				}
				return
			}
			if aerr := w.store.MarkAttemptFailed(ctx, row.ID, fmt.Sprintf("incident %s already recorded but retrying MarkDelivered failed: %v", row.IncidentID, merr)); aerr != nil {
				slog.ErrorContext(attemptCtx, "worker: MarkAttemptFailed (post-MarkDelivered-retry) also failed", "id", row.ID, "err", aerr)
			}
			return
		}
		slog.InfoContext(attemptCtx, "worker: alert delivered (MarkDelivered retried for an already-recorded incident)", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", row.IncidentID)
		// The original attempt failed before ever reaching the mapping call
		// (it failed at MarkDelivered, one step earlier) — fire it now,
		// same best-effort/non-blocking contract as the normal create path.
		// incidentNumber is unavailable here (only IncidentID is persisted
		// on the row); recordMapping/strOrNil already treat that as
		// optional.
		w.recordMapping(attemptCtx, row, bp, row.IncidentID, "")
		return
	}

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

	// Live service-UUID resolution: only for a row whose stored ServiceID is
	// still the sentinel internal/handler.MapToIncident writes when its
	// static SRE_ALERT_SERVICE_MAP lookup had no entry for this alert's raw
	// Service label at buffering time (see that function's doc comment for
	// the full hybrid-resolution design). A row with a real, statically- or
	// previously-resolved ServiceID skips this entirely — req.ServiceID is
	// mutated in place here, in this function's own local copy of
	// bp.CreateIncidentRequest, never written back to the buffered row (see
	// resolveServiceID's doc comment for why re-resolving on every attempt
	// is fine, even desirable).
	if req.ServiceID == csmclient.UnresolvedServiceIDSentinel {
		resolvedID, rerr := w.resolveServiceID(attemptCtx, row, bp)
		if rerr != nil {
			slog.WarnContext(attemptCtx, "worker: service resolution failed, treating as a retryable delivery failure", "id", row.ID, "alertNumber", row.AlertNumber, "service", bp.Service, "err", rerr)
			w.handleDeliveryFailure(ctx, attemptCtx, row, rerr)
			return
		}
		req.ServiceID = resolvedID
	}

	result, err := w.csm.CreateIncident(attemptCtx, req)
	if err == nil {
		// Durably record the incident id BEFORE attempting MarkDelivered,
		// not after: this is what closes the window a bare
		// CreateIncident-then-MarkDelivered sequence leaves open. If
		// MarkDelivered fails below, this row's IncidentID is already
		// persisted, so the short-circuit at the top of this function will
		// retry MarkDelivered directly on the next attempt — never
		// CreateIncident again — regardless of whether SearchIncidentByTag
		// is reachable. RecordIncidentID failing here (this service's own
		// database, not CSM) is logged but not fatal: MarkDelivered below
		// would also persist incident_id if it succeeds despite this call
		// failing, and the fallback path after it is unchanged for the
		// remaining edge case where both fail.
		if rerr := w.store.RecordIncidentID(ctx, row.ID, result.IncidentID); rerr != nil {
			slog.ErrorContext(attemptCtx, "worker: RecordIncidentID failed after a successful create (this service's own database, not CSM) — proceeding to MarkDelivered anyway", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", result.IncidentID, "err", rerr)
		}
		if merr := w.store.MarkDelivered(ctx, row.ID, result.IncidentID); merr != nil {
			// Residual case only: RecordIncidentID above already persisted
			// IncidentID in the common case (MarkDelivered failing here is
			// a *second* independent failure against this service's own
			// database), so the top-of-function short-circuit will retry
			// MarkDelivered directly next scan without ever calling
			// CreateIncident again. MarkAttemptFailed still runs so this
			// keeps advancing toward the retry budget/escalation path
			// rather than looping on a broken database forever.
			slog.ErrorContext(attemptCtx, "worker: MarkDelivered failed after a successful create; IncidentID is durably recorded regardless, next attempt retries MarkDelivered directly", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", result.IncidentID, "err", merr)
			if aerr := w.store.MarkAttemptFailed(ctx, row.ID, fmt.Sprintf("incident %s was created but MarkDelivered failed: %v", result.IncidentID, merr)); aerr != nil {
				slog.ErrorContext(attemptCtx, "worker: MarkAttemptFailed (post-create MarkDelivered fallback) also failed", "id", row.ID, "err", aerr)
			}
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

	w.handleDeliveryFailure(ctx, attemptCtx, row, err)
}

// handleDeliveryFailure applies the non-retryable/retry/escalate
// classification and resulting store transition for a failed delivery
// attempt — err is either a CreateIncident error, or a service-resolution
// error from resolveServiceID (see attempt's call sites). Both are folded
// into this exact same path deliberately: a resolution failure is, from the
// worker's perspective, just another reason this attempt couldn't reach a
// successful CreateIncident call, and gets exactly the same
// retry-budget/escalation treatment. ctx is the caller's own (unmodified)
// context, used for store/escalator calls; attemptCtx carries this
// attempt's own correlation ID, used only for logging — see attempt's doc
// comment for why the two are kept distinct.
func (w *Worker) handleDeliveryFailure(ctx, attemptCtx context.Context, row store.AlertRecord, err error) {
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
		// row.AlertNumber leads the message (it's this alert's externally-facing
		// identifier — see internal/store.PostgresStore.Enqueue); row.ID follows
		// for anyone cross-referencing this service's own logs/database directly.
		message := fmt.Sprintf(
			"SRE alert ingestion service: alert %s (id %s) could not be delivered to CSM after %d attempts. Last error: %s",
			row.AlertNumber, row.ID, nextRetryCount, truncate(err.Error(), 200),
		)
		// Call before marking the row terminal, not after: if this attempt
		// call was the terminal marker, a process exit between MarkEscalated
		// and Escalate would leave the row permanently 'escalated' with no
		// call ever having gone out — the exact failure this service exists
		// to prevent (CSM down *and* the independent channel silently
		// skipped). Reordered so the worst case on a mid-attempt crash is
		// instead a possible duplicate call on the next scan (MarkEscalated
		// succeeds after a call that already went out, or the process dies
		// between the two and a future scan repeats it) — a second phone
		// call is a far smaller cost than zero calls. A complete fix needs a
		// durable, idempotent notification identifier (an outbox record
		// Twilio's own call SID confirms against); tracked as follow-up, not
		// done here.
		if terr := w.twilio.Escalate(ctx, message); terr != nil {
			// The escalation call itself failing is the worst case this
			// service can be in — CSM is unreachable *and* the
			// CSM-independent notification channel just failed too. There
			// is no further fallback by design (see this service's
			// README/CLAUDE.md); log loudly and move on rather than retry
			// the call in a tight loop against Twilio. Still mark the row
			// escalated below: MaxRetries is already exhausted, and a bare
			// retry loop against a failing Twilio call is not this
			// service's job to run.
			slog.ErrorContext(attemptCtx, "worker: twilio escalation call failed", "id", row.ID, "err", terr)
		}
		if merr := w.store.MarkEscalated(ctx, row.ID, err.Error()); merr != nil {
			slog.ErrorContext(attemptCtx, "worker: MarkEscalated (store) failed", "id", row.ID, "err", merr)
		}
		return
	}

	slog.WarnContext(attemptCtx, "worker: delivery attempt failed, will retry", "id", row.ID, "alertNumber", row.AlertNumber, "retryCount", nextRetryCount, "nextDelay", backoff.Delay(nextRetryCount-1).String(), "err", err)
	if merr := w.store.MarkAttemptFailed(ctx, row.ID, err.Error()); merr != nil {
		slog.ErrorContext(attemptCtx, "worker: MarkAttemptFailed failed", "id", row.ID, "err", merr)
	}
}

// tryGroup implements this alert's incident-grouping check: an earlier
// alert reporting the same (source, uniqueIdentifier) condition, within
// Config.GroupWindow, may already have a still-open incident — found by
// searching for csmclient.GroupTag(bp.Source, bp.UniqueIdentifier), the tag
// internal/handler.buildSubject embeds in every such incident's Subject at
// creation time. If found, this alert attaches to it instead of a new one
// being created. Called only when bp.UniqueIdentifier is non-empty (see
// attempt).
//
// This needs a single search call, not the lookup-then-confirm two-step an
// earlier design used (a separate alert-incident-mapping lookup, then a
// second call to confirm the matched incident was still open) — the state
// and time-window filters are sent as part of the same search, so a match
// is only ever returned already-confirmed-open-and-in-window.
//
// Returns grouped=false, falling through to the existing create-or-dedup-
// search flow unchanged, on any failure: the search call itself erroring,
// or no match found. This mirrors the same fail-open philosophy as the
// pre-retry dedup check above: "we couldn't confirm this is groupable" is
// never treated as "assume it is."
//
// This is a known, accepted v1 limitation, not a bug: the search
// (SearchOpenIncidentByGroupTag) is ServiceNow-backed, like every other
// csmclient search call in this service, and 401s on every call today (see
// internal/csmclient/search.go and this service's CLAUDE.md) — so in
// production this always falls open to "not groupable, proceed as before"
// until that infrastructure gap is closed. The grouping feature itself is
// structurally complete and ready for that day; it does not attempt to work
// around the gap.
func (w *Worker) tryGroup(ctx context.Context, row store.AlertRecord, bp alertpayload.Payload) (incidentID, incidentNumber string, grouped bool) {
	// handler.AlertRequest.validate rejects csmclient.TagDelimiterChars in
	// Source/UniqueIdentifier on ingress, but that check postdates rows
	// already buffered by then — a legacy row's persisted payload can still
	// carry one. Building GroupTag from an unvalidated field lets distinct
	// (source, uniqueIdentifier) pairs collide on the same tag (e.g.
	// ("a", "b:c") and ("a:b", "c")), grouping this alert onto the wrong
	// incident and skipping CreateIncident. Bypass grouping instead —
	// falling through to the normal create/dedup path is always safe.
	if strings.ContainsAny(bp.Source, csmclient.TagDelimiterChars) || strings.ContainsAny(bp.UniqueIdentifier, csmclient.TagDelimiterChars) {
		slog.WarnContext(ctx, "worker: persisted source/uniqueIdentifier contains a tag delimiter, skipping grouping for this alert", "id", row.ID, "alertNumber", row.AlertNumber)
		return "", "", false
	}

	tag := csmclient.GroupTag(bp.Source, bp.UniqueIdentifier)
	since := w.now().Add(-w.cfg.GroupWindow)
	existing, found, serr := w.csm.SearchOpenIncidentByGroupTag(ctx, tag, since)
	if serr != nil {
		slog.WarnContext(ctx, "worker: incident-grouping search failed, proceeding without grouping (known limitation, fail-open — see tryGroup doc comment)", "id", row.ID, "alertNumber", row.AlertNumber, "err", serr)
		return "", "", false
	}
	if !found {
		return "", "", false
	}

	slog.InfoContext(ctx, "worker: grouping alert onto an earlier alert's still-open incident within the group window", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", existing.IncidentID, "incidentNumber", existing.IncidentNumber, "groupWindow", w.cfg.GroupWindow.String())
	w.pushGroupAttachWorkNote(ctx, row, bp, existing.IncidentID)
	w.recordMapping(ctx, row, bp, existing.IncidentID, existing.IncidentNumber)
	return existing.IncidentID, existing.IncidentNumber, true
}

// buildGroupAttachWorkNotes composes the work note pushGroupAttachWorkNote
// pushes onto an already-existing incident when row/bp — a new alert — is
// found (via tryGroup) to report the same condition: so an engineer looking
// at the incident sees "this condition fired again" history, not silence.
//
// Reuses bp.CreateIncidentRequest.WorkNotes/AdditionalComments — exactly
// what internal/handler.buildWorkNotes and this alert's own Description
// already produced when this row was buffered — rather than re-deriving the
// same source/severity/metric/environment/identifier fields a second time;
// only the leading line (row.AlertNumber, row.ReceivedAt — neither available
// to buildWorkNotes, which only ever sees the inbound AlertRequest) is new
// here. The leading line alone means this never returns "" in practice —
// pushGroupAttachWorkNote still guards on an empty result defensively,
// treating it the same as any other "nothing worth pushing" case rather than
// assuming this function can never produce one.
func buildGroupAttachWorkNotes(row store.AlertRecord, bp alertpayload.Payload) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This condition fired again — alert %s (received %s).\n", row.AlertNumber, row.ReceivedAt.UTC().Format(time.RFC3339))
	if bp.CreateIncidentRequest.WorkNotes != nil && *bp.CreateIncidentRequest.WorkNotes != "" {
		b.WriteString(*bp.CreateIncidentRequest.WorkNotes)
		b.WriteString("\n")
	}
	if bp.CreateIncidentRequest.AdditionalComments != nil && *bp.CreateIncidentRequest.AdditionalComments != "" {
		fmt.Fprintf(&b, "Description: %s\n", *bp.CreateIncidentRequest.AdditionalComments)
	}
	return strings.TrimRight(b.String(), "\n")
}

// pushGroupAttachWorkNote calls csmclient.UpdateIncident to push a work note
// summarizing row/bp onto incidentID — the incident tryGroup just attached
// this alert to, instead of creating a new one. Without this, that attach
// was silent: csmclient.CreateAlertIncidentMapping (see recordMapping below)
// records the relationship in this service's own Postgres, but nothing ever
// showed up on the incident itself for an engineer to see.
//
// Best-effort and non-blocking by design, matching recordMapping's own exact
// philosophy immediately below (see that method's doc comment): every call
// site treats a failure here purely as a logged warning, never as a reason
// to fail the overall delivery or hold back Store.MarkDelivered — the
// primary goal (this alert is attached to the right incident) is already
// achieved by the time this is called. A missed work note just means an
// engineer sees one fewer line of history on the incident, not a
// correctness bug.
func (w *Worker) pushGroupAttachWorkNote(ctx context.Context, row store.AlertRecord, bp alertpayload.Payload, incidentID string) {
	notes := buildGroupAttachWorkNotes(row, bp)
	if notes == "" {
		return
	}
	if err := w.csm.UpdateIncident(ctx, incidentID, notes); err != nil {
		slog.WarnContext(ctx, "worker: failed to push work note onto the incident this alert attached to (best-effort, non-blocking — attach already succeeded)", "id", row.ID, "alertNumber", row.AlertNumber, "incidentID", incidentID, "err", err)
	}
}

// resolveServiceID resolves the CMDB service UUID for a row whose stored
// CreateIncidentRequest.ServiceID is still
// csmclient.UnresolvedServiceIDSentinel — i.e. internal/handler.MapToIncident's
// static SRE_ALERT_SERVICE_MAP lookup had no entry for this alert's raw
// Service label at buffering time. Called once per delivery attempt,
// immediately before CreateIncident (see attempt), never inline in the
// request path — this is the live half of this service's hybrid
// service-UUID resolution design, deliberately kept off the fast path that
// runs before the 202 response (see MapToIncident's doc comment).
//
// Resolution order:
//  1. w.svcCache — a prior successful resolution for this exact label,
//     still within its TTL. No network call.
//  2. w.csm.SearchServices — a live exact-match query against
//     csm-integration-service's POST /services/search proxy. On a match, the
//     result is cached (see serviceCache's doc comment for why only a
//     success is ever cached) and returned.
//  3. A confirmed zero-result search (SearchServices returns an empty
//     slice, no error) falls back to w.cfg.UnknownServiceID
//     (SRE_ALERT_UNKNOWN_SERVICE_ID) — logged, not cached, and not treated
//     as an error: it is CSM's own signal that this label genuinely has no
//     matching CMDB service today, and the unknown-service fallback exists
//     precisely so that alert still gets an incident rather than being
//     stuck unresolved forever.
//
// A transient error from the search call itself (non-2xx, timeout, etc.) is
// returned to the caller as-is, NOT translated into the unknown-service
// fallback: attempt folds it into the exact same retryable-delivery-failure
// path (handleDeliveryFailure) a CreateIncident error takes, so a search
// hiccup gets retried like any other CSM-side-unavailability signal instead
// of silently bucketing a possibly-resolvable label as "unknown" — see
// csmclient.Client.SearchServices's own doc comment for the same contract
// from the client's side.
func (w *Worker) resolveServiceID(ctx context.Context, row store.AlertRecord, bp alertpayload.Payload) (string, error) {
	label := bp.Service

	if id, ok := w.svcCache.get(label, w.now()); ok {
		return id, nil
	}

	results, err := w.csm.SearchServices(ctx, label)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		slog.InfoContext(ctx, "worker: no CMDB service matched alert's service label, using unknown-service fallback", "id", row.ID, "alertNumber", row.AlertNumber, "service", label, "unknownServiceID", w.cfg.UnknownServiceID)
		return w.cfg.UnknownServiceID, nil
	}

	// Limit:1 on the request (see csmclient.Client.SearchServices) already
	// bounds this to at most one result — results[0] is simply "the match,"
	// not a "first of several" choice made here.
	resolved := results[0].ID
	slog.InfoContext(ctx, "worker: resolved service label to a CMDB service via live search, caching for reuse", "id", row.ID, "alertNumber", row.AlertNumber, "service", label, "serviceID", resolved, "cacheTTL", w.cfg.ServiceCacheTTL.String())
	w.svcCache.set(label, resolved, w.now())
	return resolved, nil
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
