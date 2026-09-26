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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// firstRecompute is the previousState sentinel for a project with no stored
// position yet. Distinct from state 0 ("computed, below every threshold"),
// which is a real state that a later crossing can rise from.
const firstRecompute = -1

// defaultSweepLimit caps one scheduled sweep. The sweep is resumable — it
// orders by staleness — so a cap costs latency, never coverage.
const defaultSweepLimit = 50

// sweepBudget is how long Sweep keeps starting new projects.
//
// Sized against the SERVER'S WRITE DEADLINE, not the request context.
// server.go sets WriteTimeout to 15s while routes.go wraps the mux in
// middleware.Timeout(30s), so the connection's write deadline fires FIRST: a
// sweep that stopped only on ctx.Err() could do 30 seconds of work and then be
// unable to deliver any of it. Stopping at 10s leaves headroom to serialise
// and write the partial result inside the 15s the connection actually has.
//
// Whatever is left unprocessed stays the stalest, so the next hourly run takes
// it first. Stopping early costs latency, never coverage.
//
// A var, not a const, only so tests can shrink it — the same reason
// operations/csm-scheduled-tasks/internal/queryhours keeps tokenFetchTimeout a
// var. Nothing in production reassigns it.
var sweepBudget = 10 * time.Second

// maxSweepLimit bounds what a caller may ask for in one request, so a typo in
// the scheduled task's config cannot turn into an unbounded scan.
const maxSweepLimit = 2000

// SubscriptionClosureNotifier pushes a project's consumption to Choreo Sales
// Operations. Ported from the `Consumed Query Hour Update` business rule,
// which calls REST message "Choreo API Sales Operations" /
// "Update Subscription Closure State" keyed by the project's Salesforce id.
//
// A nil notifier means pushing is disabled; the recompute still runs and is
// still recorded. Losing the outbound call is strictly better than refusing
// to record the position, and last_pushed_state makes the next recompute
// retry it.
type SubscriptionClosureNotifier interface {
	// NotifyClosureState pushes one project's position. subscriptionID is the
	// project's Salesforce id (project.sf_id / SN's u_project_id).
	NotifyClosureState(ctx context.Context, subscriptionID string, payload domain.SubscriptionClosureUpdate) error
}

// queryHourCcGroups are the standing internal recipients of every threshold
// notice, hardcoded in ServiceNow's flow script and kept hardcoded here for
// the same reason: they are a policy about who watches query-hour burn, not a
// per-deployment setting, and moving them to config would make it possible to
// ship a build that quietly tells nobody.
var queryHourCcGroups = []string{
	"cs-management-group@wso2.com",
	"bizdev@wso2.com",
	"cs-tooling-notification-group@wso2.com",
}

// queryHourExceededCc is added only at state 3. ServiceNow did exactly this
// (`cc_list.push("ruwan@wso2.com")` inside the state==3 branch).
const queryHourExceededCc = "ruwan@wso2.com"

// internalEmailDomain gates the recipient list. ServiceNow checked
// `ref_email.includes("@wso2.com")` on each resolved address before adding it.
const internalEmailDomain = "@wso2.com"

type queryHourService struct {
	repo     repository.QueryHourRepository
	notifier SubscriptionClosureNotifier
	// access scopes every by-id read and write. The project id arrives from
	// the request path, so unlike the scoped list endpoints — which fold the
	// scope into their WHERE clause — it needs an explicit check, exactly as
	// ProjectStatsService does for its own by-id stats read.
	access AccessService
	// publisher is nil when Event Hub is not configured, in which case the
	// position is still recomputed and stored and only the email is skipped —
	// the same convention as engagementAllocationService.publisher.
	publisher EventPublisherService
	// notificationsEnabled gates the threshold email separately from Event
	// Hub. Off by default: see config.QueryHourNotificationsEnabled for why
	// the Choreo kill switch alone was not enough.
	notificationsEnabled bool
}

// NewQueryHourService constructs a QueryHourService. notifier may be nil —
// see SubscriptionClosureNotifier.
func NewQueryHourService(
	repo repository.QueryHourRepository,
	notifier SubscriptionClosureNotifier,
	publisher EventPublisherService,
	access AccessService,
	notificationsEnabled bool,
) QueryHourService {
	return &queryHourService{
		repo:                 repo,
		notifier:             notifier,
		publisher:            publisher,
		access:               access,
		notificationsEnabled: notificationsEnabled,
	}
}

// requireProjectAccess rejects a caller who may not see this project.
//
// An unrestricted (internal) caller passes. Everyone else must have the
// project in their resolved scope — for an EXTERNAL customer that is the set
// of projects they are a REGISTERED project_contact of.
//
// The refusal is a NotFoundError, not Forbidden: telling an outsider that a
// project id exists but is off-limits is itself a disclosure, and every by-id
// read here would otherwise become a membership oracle. Callers who legitimately
// hold the id see no difference.
func (s *queryHourService) requireProjectAccess(ctx context.Context, projectID string) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if scope.Unrestricted {
		return nil
	}
	for _, id := range scope.ProjectIDs {
		if id == projectID {
			return nil
		}
	}
	return &apierror.NotFoundError{Msg: fmt.Sprintf("project %s not found", projectID)}
}

// requireInternalCaller gates the estate-wide operations — the sweep and the
// weekly report. Neither has a single project to scope
// against — it recomputes whatever is stalest across the estate and pushes to
// Choreo — so it is restricted to allow-listed internal services, the same
// treatment onboardingStepService gives its own estate-wide operations.
func (s *queryHourService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "this query-hour operation is only available to internal services"}
	}
	return nil
}

// QueryHourStateFor maps percent-consumed to ServiceNow's u_query_hour_state.
//
// Unlike ServiceNow's `Set Project Query Hour State`, which guards its write
// with `if (newState > 0 && ...)` and therefore only ever RAISES the state,
// this returns the honest state for the current numbers. A recalled or
// corrected time card lowers it again. That is a deliberate divergence.
func QueryHourStateFor(percentConsumed float64) int {
	for _, t := range domain.QueryHourThresholds {
		if percentConsumed >= t.MinPercent {
			return t.State
		}
	}
	return domain.QueryHourStateNormal
}

// percentConsumed is consumed as a percentage of entitlement, or 0 when there
// is no entitlement to divide by. ServiceNow skipped such projects outright
// (`if (totalHours <= 0) continue`) to avoid a divide-by-zero; recording a
// zero-percent row instead means the project still appears in the data with
// an explicit "no entitlement" reading rather than silently missing.
func percentConsumed(consumed, entitlement int) float64 {
	if entitlement <= 0 {
		return 0
	}
	return (float64(consumed) / float64(entitlement)) * 100
}

func (s *queryHourService) Recompute(ctx context.Context, projectID string) (domain.RecomputeQueryHoursResponse, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.RecomputeQueryHoursResponse{}, &apierror.ValidationError{Msg: "projectId is required"}
	}
	// Before the access check, not after: a malformed id is the caller's
	// mistake and deserves a 400. Left to reach Postgres it becomes "invalid
	// input syntax for type uuid" and surfaces as a 500, which reads as our
	// fault rather than theirs. This is the repository convention
	// (CLAUDE.md, "Service conventions") applied to the one path that skipped
	// it — requireProjectAccess returns early for an unrestricted caller and
	// never inspects the id's shape.
	if err := validateUUIDs("projectId", []string{projectID}); err != nil {
		return domain.RecomputeQueryHoursResponse{}, err
	}
	if err := s.requireProjectAccess(ctx, projectID); err != nil {
		return domain.RecomputeQueryHoursResponse{}, err
	}

	consumption, err := s.repo.Consumption(ctx, projectID)
	if err != nil {
		return domain.RecomputeQueryHoursResponse{}, err
	}

	consumed := consumption.ConsumedMinutes()
	pct := percentConsumed(consumed, consumption.EntitlementMinutes)
	state := QueryHourStateFor(pct)

	// The state this replaced comes back from the upsert itself rather than a
	// separate read. Two recomputes can run for one project at the same time —
	// the hourly sweep and a time-card-triggered one — and a read-then-write
	// pair lets both see the same old state, both conclude they caused the
	// crossing, and both publish. Reporting it from inside the write makes
	// exactly one of them the one that moved it. See upsertSQL.
	//
	// A nil previous state means the project had no row at all: a first
	// computation, which is deliberately silent (see below).
	stored, previous, err := s.repo.Upsert(ctx, consumption, state)
	if err != nil {
		return domain.RecomputeQueryHoursResponse{}, err
	}
	previousState := firstRecompute
	if previous != nil {
		previousState = *previous
	}
	stored.PercentConsumed = pct
	stored.RemainingMinutes = consumption.EntitlementMinutes - consumed
	stored.EntitlementSource = consumption.EntitlementSource
	stored.SyncedEntitlementMinutes = consumption.SyncedEntitlementMinutes
	stored.UnmatchedLineCount = consumption.UnmatchedLineCount

	// A product line the entitlement rule does not recognise contributes zero
	// hours and says nothing about it — ServiceNow's behaviour, faithfully
	// kept. Log it so the silence is at least visible in one place.
	if consumption.UnmatchedLineCount > 0 {
		slog.Warn("query hours: active opportunity lines with an unrecognised product name contributed zero entitlement",
			"projectId", projectID, "unmatchedLines", consumption.UnmatchedLineCount,
			"activeLines", consumption.ActiveLineCount)
	}
	// During the parallel run, a derived figure that disagrees with
	// ServiceNow's is the thing worth seeing before cutover.
	if consumption.EntitlementSource == domain.EntitlementSourceOpportunityLines &&
		consumption.SyncedEntitlementMinutes != consumption.EntitlementMinutes {
		slog.Info("query hours: derived entitlement differs from the ServiceNow figure",
			"projectId", projectID,
			"derivedMinutes", consumption.EntitlementMinutes,
			"serviceNowMinutes", consumption.SyncedEntitlementMinutes)
	}

	resp := domain.RecomputeQueryHoursResponse{
		ProjectQueryHours: stored,
		StateChanged:      previousState >= 0 && previousState != state,
	}

	// Notify only on an UPWARD crossing into a real threshold. Going down is
	// not news anyone needs mailing about, and state 0 has no message at all —
	// ServiceNow built an email for it anyway, with the literal word
	// "undefined" in the body, because its `internal_message` variable was
	// never assigned on that path.
	//
	// A FIRST computation is never a crossing. Without this guard the very
	// first sweep after deploy would email the owners and the cc groups for
	// every project already past 75% — notices ServiceNow has already sent.
	// The first recompute records a baseline silently; the second one onwards
	// can notify.
	if previousState != firstRecompute && state > previousState && state >= domain.QueryHourStateWarning {
		s.publishThresholdReached(ctx, projectID, state, previousState, consumption, stored)
	} else if previousState == firstRecompute && state >= domain.QueryHourStateWarning {
		slog.InfoContext(ctx, "query hours: first computation recorded as a baseline, not notified",
			"projectId", projectID, "state", state)
	}

	// Push only when the state Choreo last accepted differs from the current
	// one. That covers both "it moved" and "a previous push failed", and
	// makes a repeated recompute with unchanged numbers a no-op.
	if stored.LastPushedState != nil && *stored.LastPushedState == state {
		return resp, nil
	}
	if s.notifier == nil {
		slog.Debug("query-hour push skipped: notifier not configured",
			"projectId", projectID, "state", state)
		return resp, nil
	}
	if consumption.ProjectSFID == "" {
		// SN keyed the call on u_project_id and would have sent `undefined`
		// for a project without one. Skipping is the honest equivalent.
		slog.Warn("query-hour push skipped: project has no Salesforce id",
			"projectId", projectID, "state", state)
		resp.PushError = "project has no Salesforce id"
		return resp, nil
	}

	pushErr := s.notifier.NotifyClosureState(ctx, consumption.ProjectSFID, domain.SubscriptionClosureUpdate{
		ConsumedQueryTime: consumed,
		TotalQueryTime:    consumption.EntitlementMinutes,
	})
	if pushErr != nil {
		// A failed push must not fail the recompute: the position is already
		// stored, and last_pushed_state still differs, so the next sweep
		// retries. Same reasoning as the nil publisher in
		// engagementAllocationService.
		slog.Error("query-hour push to Choreo failed",
			"projectId", projectID, "sfId", consumption.ProjectSFID,
			"state", state, "error", pushErr)
		resp.PushError = pushErr.Error()
		return resp, nil
	}

	now := time.Now().UTC()
	if err := s.repo.MarkPushed(ctx, projectID, state, now); err != nil {
		// The push landed but we failed to record it. Report success — the
		// worst case is one duplicate push next sweep, which the receiving
		// Choreo service treats as idempotent (it sets a state, not a delta).
		slog.Error("query-hour push succeeded but could not be recorded",
			"projectId", projectID, "state", state, "error", err)
	} else {
		resp.LastPushedState = &state
		resp.LastPushedAt = &now
	}
	resp.Pushed = true
	return resp, nil
}

func (s *queryHourService) RecomputeForTimeCard(ctx context.Context, timeCardID string) (domain.RecomputeQueryHoursResponse, error) {
	timeCardID = strings.TrimSpace(timeCardID)
	if timeCardID == "" {
		return domain.RecomputeQueryHoursResponse{}, &apierror.ValidationError{Msg: "timeCardId is required"}
	}
	projectID, err := s.repo.ProjectIDForTimeCard(ctx, timeCardID)
	if err != nil {
		return domain.RecomputeQueryHoursResponse{}, err
	}
	// Scoped on the resolved PROJECT, not the time card: the card is only a
	// route to it, and Recompute re-checks anyway. Checking here as well keeps
	// the refusal a NotFound on the time card's own id rather than leaking
	// which project it belongs to.
	if err := s.requireProjectAccess(ctx, projectID); err != nil {
		return domain.RecomputeQueryHoursResponse{}, &apierror.NotFoundError{
			Msg: fmt.Sprintf("time card %s not found", timeCardID)}
	}
	// Scoped to the time card's OWN project. ServiceNow's flow instead looked
	// every project under the case's ACCOUNT up (max 1000) and recomputed all
	// of them on every approval; that fan-out is not reproduced.
	return s.Recompute(ctx, projectID)
}

func (s *queryHourService) Get(ctx context.Context, projectID string) (domain.ProjectQueryHours, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.ProjectQueryHours{}, &apierror.ValidationError{Msg: "projectId is required"}
	}
	if err := s.requireProjectAccess(ctx, projectID); err != nil {
		return domain.ProjectQueryHours{}, err
	}
	q, err := s.repo.Get(ctx, projectID)
	if err != nil {
		return domain.ProjectQueryHours{}, err
	}
	q.PercentConsumed = percentConsumed(q.ConsumedMinutes, q.EntitlementMinutes)
	q.RemainingMinutes = q.EntitlementMinutes - q.ConsumedMinutes
	return q, nil
}

func (s *queryHourService) Sweep(ctx context.Context, staleFor time.Duration, limit int) (domain.RecomputeQueryHoursBatchResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.RecomputeQueryHoursBatchResponse{}, err
	}
	if limit <= 0 {
		limit = defaultSweepLimit
	}
	if limit > maxSweepLimit {
		return domain.RecomputeQueryHoursBatchResponse{}, &apierror.ValidationError{
			Msg: "limit exceeds the maximum of 2000"}
	}
	if staleFor < 0 {
		return domain.RecomputeQueryHoursBatchResponse{}, &apierror.ValidationError{
			Msg: "staleFor must not be negative"}
	}

	started := time.Now()
	cutoff := time.Now().UTC().Add(-staleFor)
	ids, err := s.repo.StaleProjectIDs(ctx, cutoff, limit)
	if err != nil {
		return domain.RecomputeQueryHoursBatchResponse{}, err
	}

	out := domain.RecomputeQueryHoursBatchResponse{Requested: len(ids)}
	for _, id := range ids {
		// Stop on the elapsed budget or a cancelled context, whichever comes
		// first. The budget is the one that matters: the connection's 15s
		// write deadline expires long before the request context's 30s, so a
		// sweep that ran to the context deadline could not deliver its result
		// at all. See sweepBudget.
		//
		// Stopping cleanly leaves the unprocessed projects untouched, so they
		// stay the stalest and the next run picks them up first. Requested is
		// corrected to what was actually attempted, so a caller can see the
		// sweep was cut short rather than having to infer it.
		if elapsed := time.Since(started); elapsed > sweepBudget || ctx.Err() != nil {
			attempted := out.Succeeded + out.Failed
			reason := "time budget exhausted"
			if ctx.Err() != nil {
				reason = "request context cancelled"
			}
			slog.WarnContext(ctx, "query-hour sweep stopped early",
				"reason", reason, "elapsed", elapsed,
				"succeeded", out.Succeeded, "failed", out.Failed,
				"notAttempted", len(ids)-attempted)
			out.Requested = attempted
			return out, nil
		}
		// One project's failure must not abandon the rest of the sweep: the
		// next run would hit the same project first (it stays stalest) and
		// stall forever. Record and continue.
		// Bound ONE project, not just the gap between projects. The loop
		// already stops when the overall budget is spent, but that check
		// happens between iterations — a single slow Choreo push inside
		// Recompute could overrun it by any amount, and the connection's own
		// write deadline expires well before the request context's, so the
		// sweep would finish with a result it could no longer deliver.
		// Whatever remains of the budget is all any one project may have.
		remaining := sweepBudget - time.Since(started)
		if remaining <= 0 {
			continue
		}
		projectCtx, cancel := context.WithTimeout(ctx, remaining)
		res, err := s.Recompute(projectCtx, id)
		cancel()
		if err != nil {
			out.Failed++
			if out.Errors == nil {
				out.Errors = make(map[string]string)
			}
			out.Errors[id] = err.Error()
			slog.Error("query-hour sweep: project failed", "projectId", id, "error", err)
			continue
		}
		out.Succeeded++
		out.Results = append(out.Results, res)
	}
	return out, nil
}

// publishThresholdReached emails the account manager and technical owner that
// a project has crossed 75%, 90% or 100%.
//
// Port of ServiceNow's `[WSO2][Query Hour] Usage Notifications - Project`. All
// recipient resolution and subject rendering happen here, before the event is
// published, so csm-notification-service formats and sends and decides
// nothing — the division every other notification in this service uses.
//
// A failure anywhere in here is logged and swallowed: the recompute has
// already stored the position and already pushed to Choreo, and losing one
// email is strictly better than failing a run that otherwise succeeded.
func (s *queryHourService) publishThresholdReached(
	ctx context.Context,
	projectID string,
	state, previousState int,
	consumption domain.QueryHourConsumption,
	stored domain.ProjectQueryHours,
) {
	if !s.notificationsEnabled {
		slog.InfoContext(ctx, "query-hour threshold reached but notifications are disabled; nothing emailed",
			"projectId", projectID, "state", state, "previousState", previousState)
		return
	}
	if s.publisher == nil {
		slog.WarnContext(ctx, "no event publisher configured; query-hour threshold reached but not emailed",
			"projectId", projectID, "state", state)
		return
	}

	nctx, err := s.repo.NotificationContext(ctx, projectID)
	if err != nil {
		slog.ErrorContext(ctx, "resolve query-hour notification context", "err", err, "projectId", projectID)
		return
	}

	// ServiceNow added each address only if it ended @wso2.com, then
	// de-duplicated with ArrayUtil.unique(). Same here.
	var to []string
	seen := map[string]bool{}
	for _, addr := range []string{nctx.AccountManagerEmail, nctx.TechnicalOwnerEmail} {
		addr = strings.ToLower(strings.TrimSpace(addr))
		if addr == "" || !strings.HasSuffix(addr, internalEmailDomain) || seen[addr] {
			continue
		}
		seen[addr] = true
		to = append(to, addr)
	}
	if len(to) == 0 {
		// ServiceNow fell back to a single hardcoded address here
		// (kalanad@wso2.com) when the account had no owner. That is one
		// person's inbox standing in for a data problem, and it is not
		// reproduced: the cc groups still receive the notice, so nothing is
		// lost, and the missing owner stays visible instead of being absorbed.
		slog.WarnContext(ctx, "query-hour threshold: no internal owner resolved, sending to the cc groups only",
			"projectId", projectID, "state", state, "accountName", nctx.AccountName)
	}

	cc := append([]string{}, queryHourCcGroups...)
	if state == domain.QueryHourStateExceeded {
		cc = append(cc, queryHourExceededCc)
	}

	accountName := nctx.AccountName
	if accountName == "" {
		accountName = consumption.ProjectKey
	}

	var subject string
	switch state {
	case domain.QueryHourStateExceeded:
		subject = "Query Hour Exceeded in " + accountName
	case domain.QueryHourStateCritical:
		subject = "90% of Query Hours Utilized"
	default:
		subject = "75% of Query Hours Utilized"
	}

	payload, err := json.Marshal(events.QueryHourThresholdReachedPayload{
		ProjectID:          projectID,
		ProjectKey:         consumption.ProjectKey,
		ProjectName:        nctx.ProjectName,
		AccountName:        accountName,
		State:              state,
		PreviousState:      previousState,
		TotalQueryHours:    domain.FormatHoursMinutes(stored.EntitlementMinutes),
		ConsumedHours:      domain.FormatHoursMinutes(stored.ConsumedMinutes),
		RemainingHours:     domain.FormatHoursMinutes(stored.RemainingMinutes),
		EntitlementMinutes: stored.EntitlementMinutes,
		ConsumedMinutes:    stored.ConsumedMinutes,
		RemainingMinutes:   stored.RemainingMinutes,
		PercentConsumed:    stored.PercentConsumed,
		OwnerName:          nctx.AccountManagerName,
		Subject:            subject,
		Recipients:         to,
		CcRecipients:       cc,
	})
	if err != nil {
		slog.ErrorContext(ctx, "marshal query-hour threshold payload", "err", err, "projectId", projectID)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeQueryHourThresholdReached, projectID, payload); err != nil {
		slog.ErrorContext(ctx, "publish query-hour threshold event", "err", err, "projectId", projectID)
		return
	}
	slog.InfoContext(ctx, "query-hour threshold notice published",
		"projectId", projectID, "state", state, "previousState", previousState,
		"to", len(to), "cc", len(cc))
}
