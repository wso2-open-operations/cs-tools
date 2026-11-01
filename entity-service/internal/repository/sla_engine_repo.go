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

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/businessdays"
)

// slaScheduleWSO2BusinessDays is the one non-24x7 schedule this engine
// actually implements -- see internal/businessdays' own package doc comment.
// Every other schedule value (including "8-5 weekdays"/"8-5 weekdays
// excluding holidays", seen on a handful of real rows this engine never
// resolves a case policy to -- see sla_policy_resolver.go's own case-policy
// classification) is treated as plain 24x7 flat wall-clock time, same as a
// NULL/empty schedule always has been, rather than guessed at.
const slaScheduleWSO2BusinessDays = "WSO2 Business days"

// businessDayElapsedSnapshot computes a 'WSO2 Business days' clock's
// elapsed/remaining/percentage/breached figures as of instant `at`, given
// its stored start_on and the business-day pause time already accumulated
// from every COMPLETED pause/resume cycle (businessPauseSoFar never
// includes an open, still-running pause segment -- a caller finalizing one
// of those into the total before calling, e.g. completeClockBusinessDays
// folding in the segment up to its own `at`, is what keeps this one
// function correct for completion, pause and a live read alike). Shared by
// completeClockBusinessDays, setPausedBusinessDays, recomputeActiveBusinessDays
// and SLAStatusRepository.GetClockState's own live business-day branch, so
// the one real elapsed-time formula is written once.
func businessDayElapsedSnapshot(startOn, at time.Time, businessPauseSoFar, duration time.Duration) (elapsed, remaining time.Duration, percentage float64, breached bool) {
	elapsed = businessdays.Elapsed(startOn, at) - businessPauseSoFar
	if elapsed < 0 {
		elapsed = 0
	}
	remaining = duration - elapsed
	if remaining < 0 {
		remaining = 0
	}
	if duration > 0 {
		percentage = elapsed.Seconds() / duration.Seconds() * 100
	}
	breached = elapsed >= duration
	return elapsed, remaining, percentage, breached
}

// businessDayCorrectedLiveState recomputes (stage, hasBreached,
// elapsedPercent) for a business-day-scheduled clock currently in a live
// (IN_PROGRESS/BREACHED) stage, consistent with businessDayElapsedSnapshot's
// own corrected percentage -- the one helper every read path (GetClockState,
// SearchActiveSLAStatuses, TaskSlaRepository) calls instead of trusting
// sla_live's own flat-time-computed stage/has_breached for a 'WSO2 Business
// days' row.
//
// Before this helper existed, only the elapsed PERCENTAGE was corrected on
// the business-day path -- stage/has_breached still came straight from
// sla_live, which has no business-day calendar of its own and marks a clock
// BREACHED purely from flat wall-clock time. A real, confirmed consequence:
// a 1-business-day clock started Friday 10:00 ET and read Saturday 11:00
// showed BREACHED/has_breached=true (25 flat hours elapsed) right next to a
// corrected percentage of 58% (14 real business hours elapsed) -- one
// response disagreeing with itself, and (for csm-notification-service,
// which reads GetClockState before alerting) a possible false breach alert
// sent across a weekend the clock was never actually counting against.
//
// ok=false (nothing to correct, caller keeps sla_live's own value) for a
// non-business-day schedule, a nil startedOn, or a stage that isn't live --
// a PAUSED or terminal row's stored figures were already frozen correctly,
// business-day-aware, at the moment they were written (see SetPaused/
// CompleteClock's own doc comments), so recomputing them here from "now"
// would be wrong, not just redundant.
func businessDayCorrectedLiveState(schedule, stage string, startedOn *time.Time, businessPauseSeconds, durationSeconds float64) (correctedStage string, correctedHasBreached bool, correctedPercent float64, ok bool) {
	if schedule != slaScheduleWSO2BusinessDays || startedOn == nil {
		return "", false, 0, false
	}
	if stage != "IN_PROGRESS" && stage != "BREACHED" {
		return "", false, 0, false
	}
	if durationSeconds <= 0 {
		// businessDayElapsedSnapshot's own breached = elapsed >= duration
		// would be vacuously true against a zero/negative duration (elapsed
		// is clamped to >= 0), reporting BREACHED at 0% -- a missing or
		// not-yet-set duration must leave sla_live's own value alone rather
		// than correct it into something worse.
		return "", false, 0, false
	}
	duration := time.Duration(durationSeconds * float64(time.Second))
	businessPause := time.Duration(businessPauseSeconds * float64(time.Second))
	_, _, pct, breached := businessDayElapsedSnapshot(*startedOn, time.Now().UTC(), businessPause, duration)
	newStage := "IN_PROGRESS"
	if breached {
		newStage = "BREACHED"
	}
	return newStage, breached, pct, true
}

// slaEngineActiveStageFilter names the terminal sla.stage values a "sla" row
// can never leave -- shared by RegisterClock/ReviseClocks' own insert guard
// so a row this engine has already finished registering against is never
// resurrected by a later, out-of-order call (e.g. a retried case-create hook
// after a slow first attempt already registered the clock). Used only for
// that "is there already a clock for this (work_item, target) pair" check --
// NOT by CompleteClock/SetPaused, which use the narrower
// slaEngineOpenStageFilter below, since BREACHED is very much not finished
// from their point of view.
const slaEngineActiveStageFilter = `NOT IN ('ACHIEVED', 'BREACHED', 'CANCELLED', 'COMPLETED')`

// slaEngineOpenStageFilter names the only truly final sla.stage values --
// the ones a clock can never leave because its own disposition already
// happened (a reply came in, a workaround was provided, a case closed, or
// the clock was deliberately cancelled on a severity change). Deliberately
// excludes BREACHED, unlike slaEngineActiveStageFilter above: a clock whose
// wall-clock duration merely ran out without yet being satisfied is NOT
// finished -- it must keep being recomputed (RecomputeActive) and must
// still be completable or pausable by its own real event, whenever that
// event finally happens, with the TRUE elapsed time at that moment however
// far past 100% it has climbed. Treating BREACHED as terminal here was a
// real, reported bug: once a response SLA breached, a later qualifying
// comment's CompleteResponseClock call silently matched zero rows, leaving
// the clock -- and its business_elapsed_percentage/business_duration --
// frozen forever at whatever RecomputeActive last wrote before excluding it
// too (see RecomputeActive's own doc comment for its matching half of this
// fix).
const slaEngineOpenStageFilter = `NOT IN ('ACHIEVED', 'CANCELLED', 'COMPLETED')`

// slaEngineRevisionBlockStages names the stages that must block a fresh
// registration for the given clock TARGET on a severity revision
// (ReviseClocks) -- deliberately per-target, per explicit product
// direction: RESPONSE and WORKAROUND/RESOLUTION disagree on whether a mere
// BREACHED (the wall clock ran out without the clock ever being satisfied)
// counts as done.
//
//   - RESPONSE: "did a support engineer reply at all" is a fact about the
//     past that a later severity change cannot un-happen or un-miss --
//     ACHIEVED (a reply came in) and BREACHED (a reply never came, and the
//     window for a first reply has already closed) both permanently retire
//     this clock type for the case. A severity change must not re-open "are
//     we still waiting for a first reply" once that window has already
//     closed one way or the other.
//   - WORKAROUND/RESOLUTION: these track ongoing remediation work, which
//     genuinely restarts under a new severity's own duration regardless of
//     whether the OLD severity's clock ran out first -- only a REAL
//     completion (a workaround actually provided, a case actually closed,
//     via CompleteClock) means there is nothing left to track. Only
//     ACHIEVED/COMPLETED block a fresh registration for these two; BREACHED
//     does not -- a real, reported bug had this treated identically to
//     RESPONSE, leaving a case's workaround/resolution tracking permanently
//     stuck on a stale, timed-out clock instead of starting over under the
//     new severity.
//
// CANCELLED never blocks either way -- it means "this clock was
// deliberately retired and its slot is free for a fresh one" (see
// ReviseClocks).
func slaEngineRevisionBlockStages(target string) []string {
	if target == "RESPONSE" {
		return []string{"ACHIEVED", "BREACHED", "COMPLETED"}
	}
	return []string{"ACHIEVED", "COMPLETED"}
}

// ReviseClocks' own cancellation query encodes this same per-target split
// directly in SQL (RESPONSE never cancels a BREACHED row; WORKAROUND/
// RESOLUTION do) rather than calling back into this function -- see that
// method's own doc comment. Kept as one written-out rule rather than two
// (this function plus a SQL mirror) since ReviseClocks' cancellation scans
// every CSM row on the work item by its own target column, not a per-policy
// Go-side loop, so there is no natural call site for a Go-side helper here.

// SLAPolicyRef is the subset of an sla_policy row the engine's resolver
// needs: enough to register a new "sla" row against it, nothing this
// service would otherwise have to re-derive (target, duration) or display
// (name, for logging).
type SLAPolicyRef struct {
	ID       string
	Name     string
	Target   string
	Duration time.Duration
	// Schedule is sla_policy.schedule verbatim ("24 x 7", "WSO2 Business
	// days", or one of the unsupported values -- see
	// slaScheduleWSO2BusinessDays's own doc comment). Empty for a policy
	// with no schedule recorded, treated the same as "24 x 7".
	Schedule string
	// Source is sla_policy.source verbatim ("SERVICENOW" for every row the
	// external sync job writes today, "CSM" reserved for a future
	// admin-authored policy -- see the SLA admin-configuration work this
	// makes room for). Gates whether service.translateSLACondition runs on
	// this policy's condition columns at all: a SERVICENOW row is raw,
	// vendor-encoded text that needs translating; any other source is
	// assumed to already be in this engine's own neutral predicate
	// vocabulary and is parsed as-is.
	Source string
	// StartCondition/StopCondition/PauseCondition/ResumeCondition/
	// CancelCondition are the raw sla_policy condition columns, exactly as
	// the external sync job writes them -- translated by
	// service.translateSLACondition (when Source warrants it) before
	// internal/slacondition ever parses them; never interpreted directly
	// here. Empty when the column is NULL.
	StartCondition  string
	StopCondition   string
	PauseCondition  string
	ResumeCondition string
	CancelCondition string
	// Retroactive is sla_policy.retroactive: when true, a clock newly
	// registered for this policy starts at the case's own creation instant
	// (or, on a later re-evaluation, the instant that triggered it) instead
	// of the moment this engine happens to process the event -- see
	// SLAEngineService.RegisterCaseClocks' own doc comment for exactly how
	// the start instant is chosen.
	Retroactive bool
	// WhenToCancel is sla_policy.when_to_cancel verbatim
	// ("CANCEL_CONDITIONS_ARE_MET", "START_CONDITIONS_ARE_NOT_MET", or
	// "NEVER") -- see SLAEngineService.ApplyCaseStateEffects' own doc
	// comment for how each value changes cancellation.
	WhenToCancel string
	// WhenToResume is sla_policy.when_to_resume verbatim
	// ("RESUME_CONDITIONS_ARE_MET" or "PAUSE_CONDITIONS_ARE_NOT_MET") -- see
	// SLAEngineService.ApplyCaseStateEffects' own doc comment for how each
	// value changes resumption.
	WhenToResume string
}

// CaseFacts is the subset of a case's current field values the condition
// engine (internal/slacondition, via service.caseConditionFacts) needs to
// evaluate an sla_policy condition against. State/Severity are the raw
// case_state_enum/case_severity_enum text Postgres stores (e.g. "OPEN",
// "S1") -- these already match sla_policy's own decoded condition
// vocabulary verbatim (migrations 0250/0252), so no further translation
// happens in this package; domain.CaseState/domain.CaseSeverity only enter
// the picture at the service layer, when a caller's own in-flight value
// (not yet necessarily visible to a fresh read -- see
// GetCaseFacts' own doc comment) needs to override what's stored.
type CaseFacts struct {
	// State is "" when the case has no state recorded at all (a real,
	// non-trivial share of rows -- see domain.CaseView's own doc comment).
	State string
	// Severity is "" when the case has no severity recorded (case-type
	// work items only ever carry one; every other case-like type is
	// always "").
	Severity string
	// CaseType is "case"/"engagement"/"service_request"/
	// "security_report_analysis"/"announcement" (lowercased
	// caseLikeRowTypeColumn) -- "hosting"/"hosting_query"/"hosting_task"
	// are part of sla_policy's own decoded vocabulary (previous-system-only
	// case types, migration 0252) but can never appear here: this
	// Postgres schema's work_item_type_enum has no matching value at all,
	// so no work_item row can ever carry one. A policy conditioned on one
	// of those three case types simply never matches on this data source
	// -- not a bug, a real, structural gap in what this schema can
	// represent today (see sla_policy_resolver.go's own doc comment).
	CaseType string
	// ProjectType is the case's project's resolved project_type.name, ""
	// when the case has no project or the project has no type.
	ProjectType string
	// Product is the case's deployed product's resolved product.name, ""
	// when the case has no deployed product.
	Product string
	// CSEngineerResponseComment answers sla_policy's own
	// "cs_engineer_response_comment" field: whether a qualifying
	// support-engineer reply has ever been posted on this case. Computed
	// live by GetCaseFacts' own query -- an EXISTS over "comment" joined to
	// "user" (by email) and from there to user_role/role, for a public
	// (type = 'COMMENT', never WORK_NOTE) row whose author holds the
	// CS_ENGINEER_ROLE role (slaEngineRepo.csEngineerRole,
	// config.Config.CSEngineerRole) -- the SAME role
	// caseService.isSupportEngineerAuthor checks before ever calling
	// SLAEngineService.CompleteResponseClock, not the broader
	// "user".user_type = 'INTERNAL' an earlier revision used. The two checks
	// must agree: a broader fact here would report "already responded" for
	// an internal commenter (e.g. an account manager) who can never actually
	// complete the clock, letting a later severity change skip registering a
	// response clock the case genuinely still needs. Live query, not a
	// stored column: no schema change needed, and it already reflects every
	// reply ever made, including ones posted before this engine existed,
	// with no backfill required. A created_by that matches no "user" row, or
	// a user with no CS_ENGINEER_ROLE grant, simply never joins, so it's
	// never counted -- never an error. CS_ENGINEER_ROLE unset makes this
	// always false (see NewSLAEngineRepository's own doc comment).
	//
	// This is deliberately NOT derived from the response clock's own stage
	// reaching ACHIEVED, which an earlier version of this field did -- a
	// confirmed, reported bug: a response clock can be CANCELLED by a
	// severity change (ReviseClocks) without ever having been ACHIEVED, or
	// may never have existed at all (an S0/S4 case has no response policy),
	// so "the response clock is ACHIEVED" was false in cases where a reply
	// had, in fact, already happened -- a later severity change that starts
	// a fresh response policy would then see CSEngineerResponseComment=false
	// and start a brand-new response clock on a case an engineer had
	// already answered, which would go on to breach. Deriving this from the
	// case's own comment history instead means it survives any number of
	// clock cancellations/revisions, exactly like the real-world fact it
	// represents.
	CSEngineerResponseComment bool
	// WorkaroundProvided answers "u_workaround_providedISNOTEMPTY" --
	// backed by the existing work_item.workaround_provided_on column.
	WorkaroundProvided bool
	// FixEtaShared answers "fix_eta_sharedISNOTEMPTY" -- backed by the
	// existing work_item.eta_shared_on column.
	FixEtaShared bool
	// HasAccount answers "accountISNOTEMPTY".
	HasAccount bool
	// CreatedOn answers "opened_at"/"sys_created_on" (both the previous system's
	// own names for the same fact) -- work_item.created_on.
	CreatedOn time.Time
}

// SLAEngineRepository defines the read/write operations backing this
// service's own in-process SLA engine (internal/service/sla_policy_resolver.go,
// sla_engine_service.go). Every clock it writes is tagged source='CSM'
// (migration 0134's enum) -- this is migration 0134's own original intent,
// unchanged: 'SERVICENOW' means a row the external sync job that populates
// sla_policy/sla wrote, 'CSM' means a row this in-process engine wrote, and
// the two are NEVER supposed to share ownership of the same (work_item,
// target) pair -- that sync job runs continuously and real-time, tagging
// every row it writes 'SERVICENOW', so tagging this engine's own clocks the
// same way would let the two collide on the same case (a dual-write-mode
// case has rows from both), each silently overwriting state the other owns.
// `'SERVICENOW'` must never be used for an engine-managed clock.
//
// This repository still only ever writes/mutates rows it itself registered
// via RegisterClock (every method here scopes its WHERE clause to a
// specific work_item_id/target pair it just resolved a policy for, AND to
// source='CSM'), so it can never touch a row the external sync job owns.
// Read-side consumers of "sla" (GET /sla-status, POST /task-slas/search)
// need no changes at all -- every row already looks the same to them
// regardless of who wrote it, and both GetClockState (single-clock) and
// SearchActiveSLAStatuses (bulk) already accept the identical source query
// parameter, so a caller that wants only this engine's own clocks (e.g.
// csm-notification-service's pre-alert check, see that repo's own
// CLAUDE.md) scopes to source=csm on either call, not just the bulk one.
//
// The `sla_policy` table's own `source` column is a SEPARATE, unrelated
// concept -- it distinguishes a real previous-system-synced POLICY definition
// from the 16 `source='CSM'` policy rows this engine used to seed itself
// (migration 0203, deleted by migration 0259 once this engine started
// evaluating the real synced policies directly). That policy-table cleanup
// was correct and stays; only the `sla` (clock) table's own tagging was
// ever wrongly touched.
type SLAEngineRepository interface {
	// FindPolicyByName resolves the single active sla_policy row matching
	// name/target, preferring a source='SERVICENOW' row over a source='CSM'
	// one should both ever exist under the same name (they shouldn't in
	// practice: this engine's own resolver only ever looks up the
	// deterministic "<severity> - <target> (CSM)" names migration 0203
	// seeds, e.g. "S4 - RESPONSE (CSM)", a name the previous system's own sync would
	// never produce). Returns apierror.NotFoundError if no row matches.
	FindPolicyByName(ctx context.Context, name, target string) (SLAPolicyRef, error)

	// ListCandidatePolicies returns every active, source='SERVICENOW'
	// sla_policy row for target (RESPONSE/WORKAROUND/RESOLUTION), condition
	// columns included -- the raw material service.slaPolicyResolver
	// evaluates against a case's CaseFacts to pick the one real policy that
	// actually applies. Deliberately returns every row (case policies,
	// incident/escalation policies, inactive rows already excluded) rather
	// than filtering to "case policies" here -- that classification is
	// business logic (see sla_policy_resolver.go's own isCasePolicy), not a
	// SQL concern. Ordered by id for a deterministic tie-break when more
	// than one policy's start_condition matches the same case (see
	// slaPolicyResolver.resolve's own doc comment).
	ListCandidatePolicies(ctx context.Context, target string) ([]SLAPolicyRef, error)

	// GetCaseFacts reads the current field values of workItemID needed to
	// evaluate an sla_policy condition against it (internal/slacondition).
	// Returns apierror.NotFoundError if no such work item exists.
	//
	// Every field is read fresh from Postgres at call time -- there is no
	// in-flight override here. A caller that already knows a more
	// authoritative value than what a fresh read would see right now (e.g.
	// ApplyCaseStateEffects is handed the case's brand new state as a
	// parameter, from a PATCH that may not have committed its own mirror
	// write yet on the previous system's dual-write path) is expected to override
	// that one field on the returned CaseFacts itself -- see
	// slaEngineService's own call sites.
	GetCaseFacts(ctx context.Context, workItemID string) (CaseFacts, error)

	// CancelClock marks every open (slaEngineOpenStageFilter) source='CSM'
	// clock for (workItemID, target) CANCELLED, regardless of which policy
	// backs it -- used by a caller that knows "every clock of this target
	// should stop" with no per-policy distinction to make. Returns whether
	// any row was found and updated.
	CancelClock(ctx context.Context, workItemID, target string) (bool, error)

	// CancelClockByPolicy is CancelClock scoped to one specific policy's own
	// clock -- the condition-driven engine's own per-policy cancel_condition
	// outcome (ApplyCaseStateEffects), which must only ever act on the ONE
	// clock it evaluated that policy's conditions against, never on a
	// sibling clock backed by a different policy of the same target (see
	// ActiveClockPolicies' own doc comment on why more than one can be open
	// at once). Matches slaEngineOpenStageFilter, same as SetPaused/
	// CompleteClock -- an already-BREACHED clock can still be explicitly
	// cancelled (e.g. a case whose case_type changed away from what the
	// breached clock's own policy required). Returns whether a row was
	// found and updated.
	CancelClockByPolicy(ctx context.Context, workItemID, target, policyID string) (bool, error)

	// ActiveClockPolicies returns every sla_policy row currently backing one
	// of workItemID's open (slaEngineOpenStageFilter) clocks for target, so
	// the condition-driven engine (ApplyCaseStateEffects) can evaluate EACH
	// one's own stop/pause/resume/cancel conditions independently -- never a
	// fresh re-resolution of "which policy should apply now" (facts can
	// shift in ways that would otherwise flip a running clock onto a
	// different policy's numbers mid-flight). Empty (no error) when the case
	// has no open clock for target at all.
	ActiveClockPolicies(ctx context.Context, workItemID, target string) ([]SLAPolicyRef, error)

	// RegisterClock inserts a new source='CSM' "sla" row for
	// (workItemID, policy.Target, policy.ID) and starts it running now,
	// UNLESS an active (see slaEngineActiveStageFilter) source='CSM' row
	// already exists for that exact (work_item, target, policy) triple --
	// idempotent, so a retried case-create hook never double-registers --
	// OR a row for that triple already exists in a stage
	// slaEngineRevisionBlockStages(policy.Target) names as blocking (per-
	// target -- see that function's own doc comment for why RESPONSE
	// differs from WORKAROUND/RESOLUTION). A CANCELLED row does NOT block a
	// fresh insert -- cancellation deliberately frees that slot up for a
	// genuinely new one (see ReviseClocks).
	//
	// Scoped by policy, not just target, DELIBERATELY: the real data has a
	// target (RESOLUTION, for a Hosting case) that two distinct policies can
	// both legitimately match at once -- an open-source-track and a
	// managed-services-track resolution clock, running side by side (see
	// service.slaPolicyResolver.resolve's own doc comment). Scoping the
	// uniqueness check to target alone, as an earlier version of this query
	// did, would have let the first policy's own active row silently block
	// the second's registration, even though they are two entirely
	// different clocks.
	//
	// start_on is policy.Retroactive's own startOn parameter, not always
	// NOW() -- see SLAEngineService.RegisterCaseClocks' own doc comment for
	// how that instant is chosen.
	RegisterClock(ctx context.Context, workItemID string, policy SLAPolicyRef, startOn time.Time) (bool, error)

	// CompleteClock marks EVERY open (slaEngineOpenStageFilter) source='CSM'
	// clock for (workItemID, target) ACHIEVED, regardless of which policy
	// backs it -- used by a caller that has no notion of "which policy" at
	// all (a qualifying comment, a workaround being provided, a fix ETA
	// being shared: these complete whatever response/workaround/resolution
	// clock(s) happen to be open on the case, full stop). end_on=now,
	// business_elapsed_percentage set to the clock's real, uncapped elapsed
	// percentage at completion time, not unconditionally 100 -- see this
	// method's own implementation comment), and sets is_active = FALSE -- a
	// real, reported bug this fixes: is_active was previously only ever set
	// at RegisterClock's own INSERT and never cleared again, so GET
	// /sla-status (and csm-notification-service's own Redis-recovery
	// Reconcile, which reads it) kept reporting a clock as "currently
	// active" forever after it had genuinely, cleanly completed. Matches a
	// clock in ANY not-yet-final stage, including BREACHED -- a clock whose
	// window already ran out is still completable by its own real
	// finishing event, with whatever real overrun percentage that event
	// happened at (see slaEngineOpenStageFilter's own doc comment). Returns
	// whether any row was found and updated -- false (not an error) when no
	// such clock was ever registered, e.g. a LOW/Query-severity case, which
	// never gets a "response" clock's workaround/resolution siblings, or a
	// case whose policy lookup found nothing at create time.
	CompleteClock(ctx context.Context, workItemID, target string) (bool, error)

	// CompleteClockByPolicy is CompleteClock scoped to one specific policy's
	// own clock -- used by the condition-driven engine
	// (ApplyCaseStateEffects), which evaluates each open clock's stop
	// condition independently and must only ever complete the ONE clock it
	// evaluated, never a sibling clock backed by a different policy of the
	// same target.
	CompleteClockByPolicy(ctx context.Context, workItemID, target, policyID string) (bool, error)

	// SetPaused pauses (true) or resumes (false) EVERY open (not-yet-final,
	// including BREACHED -- see slaEngineOpenStageFilter) source='CSM'
	// clock for (workItemID, target), regardless of which policy backs it.
	// Idempotent and a no-op (not an error) when no such clock exists.
	SetPaused(ctx context.Context, workItemID, target string, paused bool) (bool, error)

	// SetPausedByPolicy is SetPaused scoped to one specific policy's own
	// clock -- used by the condition-driven engine (ApplyCaseStateEffects),
	// which evaluates each open clock's pause/resume conditions
	// independently and must only ever act on the ONE clock it evaluated,
	// never a sibling clock backed by a different policy of the same
	// target.
	SetPausedByPolicy(ctx context.Context, workItemID, target, policyID string, paused bool) (bool, error)

	// ReviseClocks marks every source='CSM' clock for workItemID CANCELLED,
	// per-target (RESPONSE excludes BREACHED from cancellation; WORKAROUND/
	// RESOLUTION include it -- see the implementation's own doc comment),
	// then registers a fresh row for each given policy (same insert shape
	// and per-target blocking guard as RegisterClock) -- both in ONE
	// database transaction. Used when a case's severity changes: per
	// explicit product direction, the old severity's clocks must not be
	// revised or carried forward in any way, they run into a terminal
	// CANCELLED state, and the new severity's clocks start completely fresh
	// with no relation to the old numbers (see SLAEngineService.
	// ReviseCaseClocks).
	//
	// The whole operation is one transaction, not two independent
	// statements, specifically so a failure partway through registering the
	// new clocks rolls back the cancellation too -- the case is left with
	// its OLD clocks exactly as they were, never with the old ones
	// cancelled and no replacement in their place. A clock already in a
	// stage slaEngineRevisionBlockStages names as blocking for its own
	// target (e.g. a response clock CompleteResponseClock already marked
	// ACHIEVED, or -- for RESPONSE only -- already BREACHED) is left
	// untouched by the cancellation step and never resurrected by the
	// registration step either (RegisterClock's own guard applies here
	// too). policies may be empty (e.g. a nil/unresolvable severity) -- the
	// cancellation still runs, nothing gets registered. Returns how many
	// rows were cancelled.
	ReviseClocks(ctx context.Context, workItemID string, policies []SLAPolicyRef) (int, error)
}

type slaEngineRepo struct {
	db *Scoped
	// csEngineerRole is config.Config.CSEngineerRole (CS_ENGINEER_ROLE) --
	// see GetCaseFacts' own doc comment for why CaseFacts.CSEngineerResponseComment
	// must use this exact role, not a looser "any internal user" check.
	csEngineerRole string
}

// NewSLAEngineRepository constructs an SLAEngineRepository backed by the
// given Scoped connection and the CS_ENGINEER_ROLE role name. This engine
// runs on a process-startup background worker (cmd/api/main.go's
// slaEngineCtx), never an HTTP request, so that context must carry
// WithSystemIdentity(ctx) rather than inheriting nothing -- Scoped requires
// an identity on ctx even though sla itself has no RLS since migration 0153.
//
// An empty csEngineerRole (CS_ENGINEER_ROLE unset) makes
// CaseFacts.CSEngineerResponseComment always false -- the same "can't
// confirm, not an error" posture caseService.isSupportEngineerAuthor already
// has for the identical gap (see that method's own doc comment): a role name
// that's never a real row in the "role" table simply never matches, so no
// special-casing is needed in the query itself.
func NewSLAEngineRepository(db *Scoped, csEngineerRole string) SLAEngineRepository {
	return &slaEngineRepo{db: db, csEngineerRole: csEngineerRole}
}

// FindPolicyByName implements SLAEngineRepository.
//
// ORDER BY source: 'SERVICENOW' sorts before 'CSM' lexically, so a plain
// ORDER BY source, then LIMIT 1, prefers the real synced policy whenever
// both a SERVICENOW and a CSM row happen to share a name -- which should
// never actually happen (this engine only ever seeds names the previous system's
// own real policy set is confirmed NOT to define, e.g. the P0 rows from
// migration 0136), but preferring the synced row costs nothing and
// removes any doubt about which one wins if it ever did.
func (r *slaEngineRepo) FindPolicyByName(ctx context.Context, name, target string) (SLAPolicyRef, error) {
	// EXTRACT(EPOCH FROM duration) rather than scanning the INTERVAL column
	// directly into time.Duration -- pgx v5 has no default scan plan from
	// PostgreSQL INTERVAL to time.Duration (it scans into pgtype.Interval,
	// whose Months/Days fields have no fixed conversion), so a direct scan
	// errors at query time. Same pattern task_sla_repo.go's
	// scanTaskSlaView already uses for its own INTERVAL columns.
	const query = `
		SELECT id, name, target::TEXT, EXTRACT(EPOCH FROM duration)
		FROM sla_policy
		WHERE name = $1 AND target = $2::sla_policy_target_enum
		  AND source IN ('SERVICENOW', 'CSM')
		  AND (is_active IS NULL OR is_active)
		  AND duration IS NOT NULL
		ORDER BY source
		LIMIT 1`

	var ref SLAPolicyRef
	var durationSeconds float64
	err := r.db.QueryRow(ctx, query, name, target).Scan(&ref.ID, &ref.Name, &ref.Target, &durationSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return SLAPolicyRef{}, &apierror.NotFoundError{Msg: "no sla_policy found named " + name}
	}
	if err != nil {
		return SLAPolicyRef{}, fmt.Errorf("find sla policy by name: %w", err)
	}
	ref.Duration = time.Duration(durationSeconds * float64(time.Second))
	return ref, nil
}

// slaPolicyRefColumns/scanSLAPolicyRef are shared by every query in this
// file that reads a full SLAPolicyRef -- one column list and one scan
// function, so a field added to the type is never forgotten at one call
// site while another keeps working.
const slaPolicyRefColumns = `
	id, name, target::TEXT, EXTRACT(EPOCH FROM duration), COALESCE(schedule, ''), source::TEXT,
	COALESCE(start_condition, ''), COALESCE(stop_condition, ''),
	COALESCE(pause_condition, ''), COALESCE(resume_condition, ''), COALESCE(cancel_condition, ''),
	COALESCE(retroactive, FALSE), COALESCE(when_to_cancel::TEXT, 'CANCEL_CONDITIONS_ARE_MET'),
	COALESCE(when_to_resume::TEXT, 'RESUME_CONDITIONS_ARE_MET')`

func scanSLAPolicyRef(row interface{ Scan(...any) error }) (SLAPolicyRef, error) {
	var ref SLAPolicyRef
	var durationSeconds float64
	if err := row.Scan(&ref.ID, &ref.Name, &ref.Target, &durationSeconds, &ref.Schedule, &ref.Source,
		&ref.StartCondition, &ref.StopCondition, &ref.PauseCondition, &ref.ResumeCondition, &ref.CancelCondition,
		&ref.Retroactive, &ref.WhenToCancel, &ref.WhenToResume); err != nil {
		return SLAPolicyRef{}, err
	}
	ref.Duration = time.Duration(durationSeconds * float64(time.Second))
	return ref, nil
}

// ListCandidatePolicies implements SLAEngineRepository.
func (r *slaEngineRepo) ListCandidatePolicies(ctx context.Context, target string) ([]SLAPolicyRef, error) {
	query := `
		SELECT ` + slaPolicyRefColumns + `
		FROM sla_policy
		WHERE target = $1::sla_policy_target_enum
		  AND source = 'SERVICENOW'
		  AND (is_active IS NULL OR is_active)
		  AND duration IS NOT NULL
		ORDER BY id`

	rows, err := r.db.Query(ctx, query, target)
	if err != nil {
		return nil, fmt.Errorf("list candidate sla policies: %w", err)
	}
	defer rows.Close()

	var refs []SLAPolicyRef
	for rows.Next() {
		ref, err := scanSLAPolicyRef(rows)
		if err != nil {
			return nil, fmt.Errorf("list candidate sla policies: scan: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list candidate sla policies: rows: %w", err)
	}
	return refs, nil
}

// GetCaseFacts implements SLAEngineRepository.
//
// Reuses case_repo.go's own caseLikeRowTypeColumn/caseLikeStateColumn/
// caseLikeJoins (same package) rather than a second, independent
// case-like/state resolution -- see those constants' own doc comments for
// why a case-like work item's state/type can't be read off a single column.
// severity is read from "case" directly (c.severity): only a CASE-typed
// work item ever has one (domain.CaseView's own doc comment).
//
// CaseFacts.CSEngineerResponseComment checks the commenter's OWN
// CS_ENGINEER_ROLE role membership (user_role/role joined on r.name =
// r.csEngineerRole), not the looser "any user_type = 'INTERNAL'" an earlier
// revision used -- caseService.completeResponseSLAOnComment (the one place a
// response clock is ever actually completed) only fires for that same exact
// role, via isSupportEngineerAuthor. The two must agree: a looser fact would
// report "already responded" for an internal commenter (e.g. an account
// manager) who can never actually complete the clock, so a later severity
// change would skip registering a response clock the case genuinely still
// needs.
func (r *slaEngineRepo) GetCaseFacts(ctx context.Context, workItemID string) (CaseFacts, error) {
	const query = `
		SELECT LOWER(` + caseLikeRowTypeColumn + `), ` + caseLikeStateColumn + `, c.severity::TEXT,
		       wi.created_on, COALESCE(pt.name, ''), COALESCE(prod.name, ''),
		       (wi.account_id IS NOT NULL),
		       (wi.workaround_provided_on IS NOT NULL),
		       (wi.eta_shared_on IS NOT NULL),
		       EXISTS (
		           SELECT 1 FROM comment cm
		           JOIN "user" u ON LOWER(u.email) = LOWER(cm.created_by)
		           JOIN user_role ur ON ur.user_id = u.id
		           JOIN role r ON r.id = ur.role_id
		           WHERE cm.work_item_id = wi.id
		             AND cm.type = 'COMMENT'
		             AND r.name = $2
		       )
		FROM work_item wi
		LEFT JOIN "case" c ON c.id = wi.id
		` + caseLikeJoins + `
		LEFT JOIN project proj ON proj.id = wi.project_id
		LEFT JOIN project_type pt ON pt.id = proj.project_type_id
		LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
		LEFT JOIN product prod ON prod.id = dp.product_id
		WHERE wi.id = $1`

	var facts CaseFacts
	var caseType string
	var state, severity *string
	err := r.db.QueryRow(ctx, query, workItemID, r.csEngineerRole).Scan(
		&caseType, &state, &severity, &facts.CreatedOn, &facts.ProjectType, &facts.Product,
		&facts.HasAccount, &facts.WorkaroundProvided, &facts.FixEtaShared, &facts.CSEngineerResponseComment,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return CaseFacts{}, &apierror.NotFoundError{Msg: "work item not found: " + workItemID}
	}
	if err != nil {
		return CaseFacts{}, fmt.Errorf("get case facts: %w", err)
	}
	facts.CaseType = caseType
	if state != nil {
		facts.State = *state
	}
	if severity != nil {
		facts.Severity = *severity
	}
	return facts, nil
}

// CancelClock implements SLAEngineRepository.
func (r *slaEngineRepo) CancelClock(ctx context.Context, workItemID, target string) (bool, error) {
	return r.cancelClock(ctx, workItemID, target, "")
}

// CancelClockByPolicy implements SLAEngineRepository.
func (r *slaEngineRepo) CancelClockByPolicy(ctx context.Context, workItemID, target, policyID string) (bool, error) {
	return r.cancelClock(ctx, workItemID, target, policyID)
}

// cancelClock is CancelClock/CancelClockByPolicy's shared implementation --
// same both-paths-when-bulk reasoning as completeClock/setPaused above.
//
// Also computes has_breached, same as ReviseClocks' own cancellation and
// CompleteClock already do: a clock cancelled by its own policy's cancel
// condition (applyCaseStateEffectsForPolicy, the live caller of
// CancelClockByPolicy) can genuinely have run past its duration before the
// cancelling event happened (e.g. a workaround/resolution clock that
// breached while the case sat untouched, then got a late severity change);
// without this, that breach silently disappeared the moment the clock was
// cancelled -- a real, reported gap this method shared with ReviseClocks
// before that one was fixed.
func (r *slaEngineRepo) cancelClock(ctx context.Context, workItemID, target, policyID string) (bool, error) {
	ok, err := r.cancelClockFlat(ctx, workItemID, target, policyID)
	if err != nil {
		return false, err
	}
	if ok && policyID != "" {
		return true, nil
	}
	bdOK, err := r.cancelClockBusinessDays(ctx, workItemID, target, policyID)
	return ok || bdOK, err
}

func (r *slaEngineRepo) cancelClockFlat(ctx context.Context, workItemID, target, policyID string) (bool, error) {
	policyFilter := ""
	args := []any{workItemID, target, sqlActorLiteral}
	if policyID != "" {
		policyFilter = " AND sp.id = $4::uuid"
		args = append(args, policyID)
	}
	query := `
		UPDATE sla s
		SET stage = 'CANCELLED'::sla_stage_enum, is_active = FALSE,
		    has_breached = (` + completedElapsedExpr + ` >= s.duration),
		    updated_on = NOW(), updated_by = $3
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND COALESCE(sp.schedule, '') != '` + slaScheduleWSO2BusinessDays + `'
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter + policyFilter

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("cancel csm sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// cancelClockBusinessDays is cancelClock's 'WSO2 Business days' branch --
// same pause-aware elapsed-vs-duration computation as
// completeClockBusinessDays, just writing CANCELLED instead of ACHIEVED and
// leaving the clock's displayed elapsed/remaining/percentage columns alone
// (a cancelled clock, unlike a completed one, was never actually satisfied,
// so there's no "real outcome" figure to freeze the way completion does --
// only has_breached, a simple past/future-tense fact, needs recording here).
func (r *slaEngineRepo) cancelClockBusinessDays(ctx context.Context, workItemID, target, policyID string) (bool, error) {
	var found bool
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		policyFilter := ""
		args := []any{workItemID, target}
		if policyID != "" {
			policyFilter = " AND sp.id = $3::uuid"
			args = append(args, policyID)
		}
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.start_on, s.paused_on,
			       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0),
			       EXTRACT(EPOCH FROM s.duration)
			FROM sla s
			JOIN sla_policy sp ON sp.id = s.sla_policy_id
			WHERE s.work_item_id = $1::uuid
			  AND s.source = 'CSM'
			  AND sp.target::TEXT = $2
			  AND sp.schedule = '`+slaScheduleWSO2BusinessDays+`'
			  AND s.stage::TEXT `+slaEngineOpenStageFilter+policyFilter+`
			FOR UPDATE OF s`, args...)
		if err != nil {
			return fmt.Errorf("cancel business-day sla clock: select: %w", err)
		}
		type openRow struct {
			id                                    string
			startOn                               time.Time
			pausedOn                              *time.Time
			businessPauseSeconds, durationSeconds float64
		}
		var open []openRow
		for rows.Next() {
			var o openRow
			if err := rows.Scan(&o.id, &o.startOn, &o.pausedOn, &o.businessPauseSeconds, &o.durationSeconds); err != nil {
				rows.Close()
				return fmt.Errorf("cancel business-day sla clock: scan: %w", err)
			}
			open = append(open, o)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("cancel business-day sla clock: rows: %w", err)
		}

		now := time.Now().UTC()
		for _, o := range open {
			businessPause := time.Duration(o.businessPauseSeconds * float64(time.Second))
			if o.pausedOn != nil {
				businessPause += businessdays.Elapsed(*o.pausedOn, now)
			}
			elapsed := businessdays.Elapsed(o.startOn, now) - businessPause
			if elapsed < 0 {
				elapsed = 0
			}
			duration := time.Duration(o.durationSeconds * float64(time.Second))
			hasBreached := elapsed >= duration

			if _, err := tx.Exec(ctx, `
				UPDATE sla SET stage = 'CANCELLED'::sla_stage_enum, is_active = FALSE,
				    has_breached = $2,
				    updated_on = NOW(), updated_by = $3
				WHERE id = $1::uuid`,
				o.id, hasBreached, sqlActorLiteral,
			); err != nil {
				return fmt.Errorf("cancel business-day sla clock: update: %w", err)
			}
			found = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// ActiveClockPolicies implements SLAEngineRepository.
//
// Returns every open clock's own bound policy for (workItemID, target), not
// just one -- a target can genuinely have more than one simultaneously open
// clock since RegisterClock/ReviseClocks register one clock per matching
// policy rather than per target (see those methods' own doc comments on why:
// the real data has a Hosting case legitimately running both an
// open-source-track and a managed-services-track resolution clock at once).
// An earlier version of this method (ActiveClockPolicy, singular) picked
// only the most-recently-started row and silently ignored any other open
// clock of the same target -- ApplyCaseStateEffects now evaluates every
// policy this returns independently, each against its OWN condition
// columns, rather than applying one policy's decision to clocks it was never
// resolved for.
func (r *slaEngineRepo) ActiveClockPolicies(ctx context.Context, workItemID, target string) ([]SLAPolicyRef, error) {
	query := `
		SELECT ` + slaPolicyRefColumnsPrefixed("sp.") + `
		FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter + `
		ORDER BY s.created_on`

	rows, err := r.db.Query(ctx, query, workItemID, target)
	if err != nil {
		return nil, fmt.Errorf("active clock policies: %w", err)
	}
	defer rows.Close()

	var refs []SLAPolicyRef
	for rows.Next() {
		ref, err := scanSLAPolicyRef(rows)
		if err != nil {
			return nil, fmt.Errorf("active clock policies: scan: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("active clock policies: rows: %w", err)
	}
	return refs, nil
}

// slaPolicyRefColumnsPrefixed is slaPolicyRefColumns qualified with alias --
// needed wherever the query joins "sla_policy" under an alias alongside
// another table that also has an "id"/"name" column (e.g. ActiveClockPolicies'
// own join against "sla").
func slaPolicyRefColumnsPrefixed(alias string) string {
	cols := []string{
		"id", "name", "target::TEXT", "EXTRACT(EPOCH FROM duration)", "COALESCE(schedule, '')", "source::TEXT",
		"COALESCE(start_condition, '')", "COALESCE(stop_condition, '')",
		"COALESCE(pause_condition, '')", "COALESCE(resume_condition, '')", "COALESCE(cancel_condition, '')",
		"COALESCE(retroactive, FALSE)", "COALESCE(when_to_cancel::TEXT, 'CANCEL_CONDITIONS_ARE_MET')",
		"COALESCE(when_to_resume::TEXT, 'RESUME_CONDITIONS_ARE_MET')",
	}
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = alias + c
	}
	return strings.Join(out, ", ")
}

// slaEngineRegisterClockQuery inserts a new source='CSM' "sla" row for
// ($1=workItemID, $2=policy.ID) unless an existing row for that exact
// (work_item, policy) pair is either still active (slaEngineActiveStageFilter)
// or already reached a stage $5 names as blocking
// (slaEngineRevisionBlockStages(policy.Target), bound by registerClockExec --
// per-target, see that function's own doc comment) -- a CANCELLED row blocks
// neither, deliberately, since cancellation is what frees that slot up for a
// fresh registration (see SLAEngineRepository.ReviseClocks). Scoped by
// sp.id = $2, not sp.target, so two distinct policies that both resolve for
// the same target (the real Hosting-case shape RegisterClock's own doc
// comment describes) can each register their own independent clock, rather
// than the first one's active row blocking the second's insert -- policy.Target
// itself has no place in this query at all (registerClockExec only needs it
// Go-side, to compute the $5 blocking-stage list). Shared, identical SQL text
// between RegisterClock (run against the pool directly) and ReviseClocks (run
// inside its own transaction) -- sqlExecutor is satisfied by both
// *pgxpool.Pool and pgx.Tx. $7 is start_on -- NOW() for an ordinary
// registration, or an explicit earlier instant for policy.Retroactive (see
// SLAEngineService.RegisterCaseClocks' own doc comment).
//
// Regression, caught post-merge-review: an earlier revision of this query
// used to reference policy.Target directly (hence the "$5=policy.Target"
// this comment used to say) and was refactored to the sp.id-scoped NOT EXISTS
// clauses above instead -- but registerClockExec's own Exec call kept passing
// policy.Target as a positional argument nobody renumbered or removed. A
// parameter pgx sends that the query text never references anywhere has no
// context for Postgres to infer its type from during Parse, which fails
// every single call with 42P18 (indeterminate_datatype) against a real
// database -- invisible here because the only tests exercising the real SQL
// (sla_engine_repo_integration_test.go) are skipped unless
// CASE_STATS_TEST_DSN is set, so this shipped with the engine registering
// nothing at all. Fixed by dropping the unused argument and renumbering.
// registration_lock serializes this exact (work_item_id, sla_policy_id) pair
// before the NOT EXISTS checks below ever run -- without it, two concurrent
// calls (RegisterClock has no transaction/row lock of its own; ReviseClocks
// registers from inside a transaction, but a separate RegisterClock call
// racing it is still possible) could both see no existing row and both
// insert, leaving two active IN_PROGRESS clocks for one policy. pg_advisory_xact_lock
// is released at the end of the current transaction -- for RegisterClock's
// own un-transacted Exec call, Postgres' implicit per-statement transaction
// is that scope, so the lock is held for exactly this INSERT's duration;
// inside ReviseClocks' explicit transaction it's held until that commits.
// Keyed on both ids (not just work_item_id) so two different policies for
// the same work item never serialize against each other needlessly.
const slaEngineRegisterClockQuery = `
	WITH registration_lock AS (
		SELECT pg_advisory_xact_lock(hashtextextended($1::text || '|' || $2::text, 0))
	)
	INSERT INTO sla (
		id, created_on, updated_on, created_by, updated_by,
		work_item_id, sla_policy_id, schedule, is_active, stage, start_on,
		duration, business_elapsed_percentage, business_duration,
		remaining_business_duration, has_breached, source
	)
	SELECT gen_random_uuid(), NOW(), NOW(), $3, $3,
	       $1::uuid, $2::uuid, $6, TRUE, 'IN_PROGRESS'::sla_stage_enum, $7::timestamptz,
	       $4::interval, 0, INTERVAL '0', $4::interval, FALSE, 'CSM'::sla_source_enum
	FROM registration_lock
	WHERE NOT EXISTS (
		SELECT 1 FROM sla s
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND s.sla_policy_id = $2::uuid
		  AND s.stage::TEXT ` + slaEngineActiveStageFilter + `
	)
	AND NOT EXISTS (
		SELECT 1 FROM sla s
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND s.sla_policy_id = $2::uuid
		  AND s.stage::TEXT = ANY($5::text[])
	)`

// sqlExecutor is the subset of *pgxpool.Pool/pgx.Tx this file's queries
// need -- lets slaEngineRegisterClockQuery run identically against either.
type sqlExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// registerClockExec runs slaEngineRegisterClockQuery against any sqlExecutor
// -- the pool directly for RegisterClock's own standalone call, or a
// transaction for ReviseClocks, so both share one query and one insert
// decision instead of two copies that could drift apart. The blocking-stage
// list ($6) is resolved per policy.Target via slaEngineRevisionBlockStages
// -- see that function's own doc comment for why RESPONSE differs from
// WORKAROUND/RESOLUTION.
func registerClockExec(ctx context.Context, exec sqlExecutor, workItemID string, policy SLAPolicyRef, startOn time.Time) (bool, error) {
	if startOn.IsZero() {
		startOn = time.Now().UTC()
	}
	tag, err := exec.Exec(ctx, slaEngineRegisterClockQuery, workItemID, policy.ID, sqlActorLiteral, formatIntervalLiteral(policy.Duration), slaEngineRevisionBlockStages(policy.Target), nullableSchedule(policy.Schedule), startOn)
	if err != nil {
		return false, fmt.Errorf("register csm sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// RegisterClock implements SLAEngineRepository.
func (r *slaEngineRepo) RegisterClock(ctx context.Context, workItemID string, policy SLAPolicyRef, startOn time.Time) (bool, error) {
	return registerClockExec(ctx, r.db, workItemID, policy, startOn)
}

// CompleteClock implements SLAEngineRepository.
//
// business_elapsed_percentage, business_duration and remaining_business_duration
// are all set from the clock's REAL elapsed time at completion (same flat
// wall-clock formula RecomputeActive uses), not a hardcoded 100%/zeroed-out
// pair -- an earlier version of this query always wrote
// business_elapsed_percentage=100 regardless of how much of the clock's
// duration had actually passed, which made a response answered within
// minutes look identical, on read, to one that ran the entire window and
// barely made it. That false 100% had a real, live-observed downstream
// effect beyond just a misleading UI number: csm-notification-service's
// SLA-breach poller (internal/slaengine.tierForStatus) treats
// businessElapsedPercent >= 100 as tier 100 on its own, with no regard for
// stage -- so a same-clock ACHIEVED reading at a fabricated 100% was
// indistinguishable from a genuine breach, and fired a spurious "Response
// SLA Violation" Chat alert for a case answered well within its window.
// business_duration/remaining_business_duration were never written by this
// query at all before -- both columns stayed NULL forever for every
// source='CSM' row, which task_sla_repo.go's read renders as an absent
// "Business elapsed time"/"Business time left" ("--" in the webapp) even
// for a clock that has since completed. Computing all three here means an
// early completion reads (and alerts) as what it actually was, on every
// column the UI shows.
//
// business_elapsed_percentage is no longer capped at 100 -- a clock
// completed well past its deadline (e.g. a support engineer replying long
// after a response SLA breached) now shows its real overrun (e.g. 134%)
// rather than an identical-looking 100%, matching RecomputeActive's own
// uncapped formula below so the two never disagree on a clock that was
// BREACHED right up until this call finalized it. Still floored at 0 --
// GREATEST(0, ...) -- for the same reason it always was: a clock processed
// a moment before its own start_on (clock skew, or a near-simultaneous
// register+complete) must not show a negative percentage.
//
// Matches on slaEngineOpenStageFilter, not slaEngineActiveStageFilter --
// see that constant's own doc comment: a BREACHED clock is exactly the
// case this needs to still match, so its real completing event (whenever
// it finally happens) finalizes it instead of silently matching zero rows.
// completedElapsedExpr is the wall-clock time between a clock's start and
// its completion MINUS every moment it spent paused along the way -- the
// same pause-subtraction sla_live and SetPaused's own pause-time freeze
// already apply, just evaluated at completion instead of at read time or
// pause time. Before this, CompleteClock used a bare "NOW() - s.start_on",
// completely ignoring s.pause_duration -- a real, reported bug: a clock
// paused for, say, two days and then resumed and completed showed a final
// business_duration/business_elapsed_percentage that counted those two
// paused days as elapsed working time. COALESCE(s.pause_duration, INTERVAL
// '0') covers every already-closed pause segment; the CASE covers the rare
// defensive case of a clock somehow still marked paused (paused_on IS NOT
// NULL) at the exact moment it's force-completed -- every real caller
// (ApplyStateEffects' CLOSED branch) already resumes a clock before
// completing it, but this makes the formula correct regardless of calling
// order, the same way reconcileClock's own pauseOffset does on the
// csm-notification-service side for the identical reason.
const completedElapsedExpr = `(NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0') -
		    CASE WHEN s.paused_on IS NOT NULL THEN NOW() - s.paused_on ELSE INTERVAL '0' END)`

// CompleteClock also finalizes any open pause segment (paused_on IS NOT
// NULL) into pause_duration and clears paused_on. ApplyStateEffects'
// CLOSED branch always resumes a clock before completing it, but that
// resume can itself fail and be logged-and-ignored -- this keeps a clock
// that reaches CompleteClock still paused from ending up ACHIEVED with
// paused_on left set, which would make GetClockState report a finished
// clock as paused with an incomplete pause total.
// CompleteClock dispatches to the flat wall-clock SQL path above for every
// 24x7 (or NULL/unsupported-schedule) clock, and to completeClockBusinessDays
// (internal/businessdays) for a clock backed by a 'WSO2 Business days'
// policy -- see that method's own doc comment for why this is done in Go
// rather than hand-rolled as a second SQL formula. completeClockFlat's own
// WHERE clause excludes 'WSO2 Business days' rows so the two paths can never
// both match the same row.
func (r *slaEngineRepo) CompleteClock(ctx context.Context, workItemID, target string) (bool, error) {
	return r.completeClock(ctx, workItemID, target, "")
}

// CompleteClockByPolicy implements SLAEngineRepository.
func (r *slaEngineRepo) CompleteClockByPolicy(ctx context.Context, workItemID, target, policyID string) (bool, error) {
	return r.completeClock(ctx, workItemID, target, policyID)
}

// completeClock is CompleteClock/CompleteClockByPolicy's shared
// implementation -- policyID == "" completes every open clock of
// (workItemID, target) regardless of policy (CompleteClock's own bulk
// behavior); a non-empty policyID additionally restricts to that one
// policy's own row (CompleteClockByPolicy).
// A non-empty policyID targets exactly one row, which can only ever be flat
// or business-day, so the first path to report a hit is authoritative and the
// other is skipped. policyID == "" is CompleteClock's bulk case: two policies
// can match the same (workItemID, target) at once with different schedules
// (e.g. a Hosting case's OS and managed-services resolution clocks), so both
// paths must always run and their results combined -- stopping at the first
// hit would leave a same-target sibling clock on the other schedule open
// forever.
func (r *slaEngineRepo) completeClock(ctx context.Context, workItemID, target, policyID string) (bool, error) {
	ok, err := r.completeClockFlat(ctx, workItemID, target, policyID)
	if err != nil {
		return false, err
	}
	if ok && policyID != "" {
		return true, nil
	}
	bdOK, err := r.completeClockBusinessDays(ctx, workItemID, target, policyID)
	return ok || bdOK, err
}

func (r *slaEngineRepo) completeClockFlat(ctx context.Context, workItemID, target, policyID string) (bool, error) {
	policyFilter := ""
	args := []any{workItemID, target, sqlActorLiteral}
	if policyID != "" {
		policyFilter = " AND sp.id = $4::uuid"
		args = append(args, policyID)
	}
	query := `
		UPDATE sla s
		SET stage = 'ACHIEVED'::sla_stage_enum, end_on = NOW(), is_active = FALSE,
		    business_elapsed_percentage = GREATEST(0,
		        EXTRACT(EPOCH FROM ` + completedElapsedExpr + `) / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100
		    ),
		    business_duration = ` + completedElapsedExpr + `,
		    remaining_business_duration = GREATEST(s.duration - ` + completedElapsedExpr + `, INTERVAL '0'),
		    has_breached = (` + completedElapsedExpr + ` >= s.duration),
		    pause_duration = COALESCE(s.pause_duration, INTERVAL '0') +
		        CASE WHEN s.paused_on IS NOT NULL THEN NOW() - s.paused_on ELSE INTERVAL '0' END,
		    paused_on = NULL,
		    updated_on = NOW(), updated_by = $3
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND COALESCE(sp.schedule, '') != '` + slaScheduleWSO2BusinessDays + `'
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter + policyFilter

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("complete csm sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// completeClockBusinessDays is CompleteClock's 'WSO2 Business days' branch:
// reads the one open row's raw instants under a row lock, computes its real
// elapsed/remaining business time in Go (internal/businessdays.Elapsed),
// and writes the result back -- the same final shape completeClockFlat
// writes via SQL, just computed with a calendar SQL has no notion of.
// Finalizes an open pause segment into both pause_duration (flat) and
// business_pause_duration (business-day), mirroring completedElapsedExpr's
// own CASE for the flat path.
// completeClockBusinessDays locks and completes EVERY open business-day row
// matching (workItemID, target[, policyID]), not just one -- a target with
// two simultaneously-open business-day policies (not just one flat/one
// business-day) must have both finalized by a single bulk CompleteClock call.
func (r *slaEngineRepo) completeClockBusinessDays(ctx context.Context, workItemID, target, policyID string) (bool, error) {
	var found bool
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		policyFilter := ""
		args := []any{workItemID, target}
		if policyID != "" {
			policyFilter = " AND sp.id = $3::uuid"
			args = append(args, policyID)
		}
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.start_on, s.paused_on,
			       COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0),
			       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0),
			       EXTRACT(EPOCH FROM s.duration)
			FROM sla s
			JOIN sla_policy sp ON sp.id = s.sla_policy_id
			WHERE s.work_item_id = $1::uuid
			  AND s.source = 'CSM'
			  AND sp.target::TEXT = $2
			  AND sp.schedule = '`+slaScheduleWSO2BusinessDays+`'
			  AND s.stage::TEXT `+slaEngineOpenStageFilter+policyFilter+`
			FOR UPDATE OF s`, args...)
		if err != nil {
			return fmt.Errorf("complete business-day sla clock: select: %w", err)
		}
		type openRow struct {
			id                                                  string
			startOn                                             time.Time
			pausedOn                                            *time.Time
			pauseSeconds, businessPauseSeconds, durationSeconds float64
		}
		var open []openRow
		for rows.Next() {
			var o openRow
			if err := rows.Scan(&o.id, &o.startOn, &o.pausedOn, &o.pauseSeconds, &o.businessPauseSeconds, &o.durationSeconds); err != nil {
				rows.Close()
				return fmt.Errorf("complete business-day sla clock: scan: %w", err)
			}
			open = append(open, o)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("complete business-day sla clock: rows: %w", err)
		}

		now := time.Now().UTC()
		for _, o := range open {
			flatPause := time.Duration(o.pauseSeconds * float64(time.Second))
			businessPause := time.Duration(o.businessPauseSeconds * float64(time.Second))
			if o.pausedOn != nil {
				// An open pause segment at completion time -- finalize it
				// into both totals, same as completedElapsedExpr's CASE does
				// for the flat path.
				flatPause += now.Sub(*o.pausedOn)
				businessPause += businessdays.Elapsed(*o.pausedOn, now)
			}

			elapsed := businessdays.Elapsed(o.startOn, now) - businessPause
			if elapsed < 0 {
				elapsed = 0
			}
			duration := time.Duration(o.durationSeconds * float64(time.Second))
			remaining := duration - elapsed
			if remaining < 0 {
				remaining = 0
			}
			pct := 0.0
			if o.durationSeconds > 0 {
				pct = elapsed.Seconds() / o.durationSeconds * 100
			}

			hasBreached := elapsed >= duration
			if _, err := tx.Exec(ctx, `
				UPDATE sla SET stage = 'ACHIEVED'::sla_stage_enum, end_on = NOW(), is_active = FALSE,
				    business_elapsed_percentage = $2,
				    business_duration = $3::interval,
				    remaining_business_duration = $4::interval,
				    has_breached = $8,
				    pause_duration = $5::interval,
				    business_pause_duration = $6::interval,
				    paused_on = NULL,
				    updated_on = NOW(), updated_by = $7
				WHERE id = $1::uuid`,
				o.id, pct, formatIntervalLiteral(elapsed), formatIntervalLiteral(remaining),
				formatIntervalLiteral(flatPause), formatIntervalLiteral(businessPause), sqlActorLiteral, hasBreached,
			); err != nil {
				return fmt.Errorf("complete business-day sla clock: update: %w", err)
			}
			found = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// SetPaused implements SLAEngineRepository.
//
// Matches on slaEngineOpenStageFilter, not slaEngineActiveStageFilter --
// a BREACHED clock must still be pausable (e.g. a workaround/resolution
// clock that breached while the case was still open, which then moves to
// AWAITING_INFO) so RecomputeActive actually stops touching it rather than
// continuing to climb its elapsed time while the case waits on the
// customer; see slaEngineOpenStageFilter's own doc comment.
//
// paused_on/pause_duration are now real, accumulated state -- not just a
// "currently paused" marker. Until this fix, pausing only ever reset
// paused_on to the current instant and resuming only ever cleared it back
// to NULL; pause_duration (the sla table's own column, since migration
// 0048 -- previous-system-synced rows already use it) was never written for a
// source='CSM' row at all, so there was no durable record of how long a
// clock had actually spent paused across one or more pause/resume cycles --
// only this process's own fleeting "is it paused right now," gone on
// restart, and nothing at all on csm-notification-service's side either
// (its own Redis clock hash has even less: a bare boolean, no timestamp --
// see that repo's own CLAUDE.md, "SLA breach-alerting engine"). One UPDATE
// now behaves correctly for every transition:
//   - pause -> pause (already PAUSED, paused again): paused_on is left as
//     it was (COALESCE keeps the existing value) rather than reset to NOW()
//     -- a duplicate/idempotent pause call (e.g. a redelivered
//     case.status_changed event) must not shrink the apparent pause
//     duration by silently moving the pause's own start time forward.
//   - resume (paused -> running): the elapsed pause interval
//     (NOW() - paused_on) is added to the running pause_duration total,
//     then paused_on is cleared -- both read the row's own pre-update
//     values, so there is no ordering hazard within the single statement.
//   - resume called on a row that was not actually paused (paused_on
//     already NULL): adds zero (NOW() - NULL is NULL, COALESCE falls back).
//
// business_pause_duration is deliberately left untouched -- this engine has
// no business-hours calendar (see RecomputeActive's own doc comment on its
// "flat wall-clock, no business hours" crudeness), so there is nothing
// correct to compute for it; it stays NULL for every source='CSM' row, same
// as before this fix.
//
// business_elapsed_percentage/business_duration/remaining_business_duration
// are now frozen at their REAL value the instant a clock pauses -- a real,
// reported gap with the identical shape CompleteClock's own doc comment
// already describes fixing for completion: this UPDATE used to leave all
// three completely untouched on pause, same as it always had, so a paused
// clock's displayed elapsed/remaining fell back to whatever was stored at
// registration (effectively 0%/the full duration) rather than the true
// elapsed time up to the moment it was paused -- confirmed live, a case
// moved to Awaiting Info minutes after its workaround/resolution clocks
// started showed "0 Minutes elapsed" / the full target duration still
// "remaining" for both, even though real working time had passed.
// sla_live only computes these three live for an IN_PROGRESS/BREACHED row
// (see that view's own doc comment); for PAUSED it just passes the stored
// columns through unchanged, so whatever this UPDATE writes here is exactly
// what the case detail's SLA tab, GET /sla-status and task_sla_repo.go's
// own reads will show for as long as the clock stays paused.
//
// The freeze point is COALESCE(s.paused_on, NOW()), not a bare NOW() --
// deliberately the SAME expression paused_on itself is computed from just
// above. A genuinely new pause (paused_on currently NULL) freezes at NOW(),
// same instant paused_on is set to. A duplicate/idempotent pause call on an
// already-PAUSED clock (e.g. a redelivered case.status_changed event,
// arbitrarily later in wall-clock time) must re-derive the IDENTICAL frozen
// values, not silently advance them as if the clock kept accumulating
// elapsed time while paused -- using the row's own already-set paused_on
// (now non-NULL, so COALESCE no longer falls back to NOW()) gives exactly
// that: the same frozen snapshot every time, however many times pause is
// redelivered. The elapsed math itself mirrors sla_live's own live_*
// formula, just evaluated at the pause instant instead of at read time:
// (freeze point) - start_on - the pause total ALREADY completed before this
// pause began (s.pause_duration, left unmodified by the pause branch above)
// -- never the segment this pause is itself only just starting.
func (r *slaEngineRepo) SetPaused(ctx context.Context, workItemID, target string, paused bool) (bool, error) {
	return r.setPaused(ctx, workItemID, target, "", paused)
}

// SetPausedByPolicy implements SLAEngineRepository.
func (r *slaEngineRepo) SetPausedByPolicy(ctx context.Context, workItemID, target, policyID string, paused bool) (bool, error) {
	return r.setPaused(ctx, workItemID, target, policyID, paused)
}

// setPaused is SetPaused/SetPausedByPolicy's shared implementation --
// policyID == "" acts on every open clock of (workItemID, target) regardless
// of policy (SetPaused's own bulk behavior); a non-empty policyID
// additionally restricts to that one policy's own row (SetPausedByPolicy).
// Same both-paths-when-bulk reasoning as completeClock above: policyID == ""
// must run both the flat and business-day branches and combine their
// results, since two policies on different schedules can both be open for
// the same (workItemID, target) at once.
func (r *slaEngineRepo) setPaused(ctx context.Context, workItemID, target, policyID string, paused bool) (bool, error) {
	ok, err := r.setPausedFlat(ctx, workItemID, target, policyID, paused)
	if err != nil {
		return false, err
	}
	if ok && policyID != "" {
		return true, nil
	}
	bdOK, err := r.setPausedBusinessDays(ctx, workItemID, target, policyID, paused)
	return ok || bdOK, err
}

func (r *slaEngineRepo) setPausedFlat(ctx context.Context, workItemID, target, policyID string, paused bool) (bool, error) {
	policyFilter := ""
	args := []any{workItemID, target, paused, sqlActorLiteral}
	if policyID != "" {
		policyFilter = " AND sp.id = $5::uuid"
		args = append(args, policyID)
	}
	query := `
		UPDATE sla s
		SET stage = CASE WHEN $3 THEN 'PAUSED'::sla_stage_enum ELSE 'IN_PROGRESS'::sla_stage_enum END,
		    pause_duration = CASE WHEN $3 THEN s.pause_duration
		                           ELSE COALESCE(s.pause_duration, INTERVAL '0') + COALESCE(NOW() - s.paused_on, INTERVAL '0')
		                      END,
		    paused_on = CASE WHEN $3 THEN COALESCE(s.paused_on, NOW()) ELSE NULL END,
		    business_elapsed_percentage = CASE WHEN $3 THEN GREATEST(0,
		            EXTRACT(EPOCH FROM (COALESCE(s.paused_on, NOW()) - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')))
		            / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100
		        )
		        ELSE s.business_elapsed_percentage
		    END,
		    business_duration = CASE WHEN $3 THEN GREATEST(
		            COALESCE(s.paused_on, NOW()) - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'), INTERVAL '0'
		        )
		        ELSE s.business_duration
		    END,
		    remaining_business_duration = CASE WHEN $3 THEN GREATEST(
		            s.duration - (COALESCE(s.paused_on, NOW()) - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')), INTERVAL '0'
		        )
		        ELSE s.remaining_business_duration
		    END,
		    updated_on = NOW(), updated_by = $4
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND COALESCE(sp.schedule, '') != '` + slaScheduleWSO2BusinessDays + `'
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter + policyFilter

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("set csm sla clock paused: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// setPausedBusinessDays is SetPaused's 'WSO2 Business days' branch -- same
// transition shape as setPausedFlat (pause freezes the live figures at the
// pause instant and leaves them alone on every subsequent idempotent pause
// call; resume accumulates this segment's real elapsed business time into
// business_pause_duration/pause_duration and leaves the frozen figures
// alone), computed with internal/businessdays.Elapsed instead of flat SQL
// arithmetic.
// setPausedBusinessDays locks and transitions EVERY open business-day row
// matching (workItemID, target[, policyID]) -- same bulk reasoning as
// completeClockBusinessDays above.
func (r *slaEngineRepo) setPausedBusinessDays(ctx context.Context, workItemID, target, policyID string, paused bool) (bool, error) {
	var found bool
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		policyFilter := ""
		args := []any{workItemID, target}
		if policyID != "" {
			policyFilter = " AND sp.id = $3::uuid"
			args = append(args, policyID)
		}
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.start_on, s.paused_on,
			       COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0),
			       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0),
			       EXTRACT(EPOCH FROM s.duration)
			FROM sla s
			JOIN sla_policy sp ON sp.id = s.sla_policy_id
			WHERE s.work_item_id = $1::uuid
			  AND s.source = 'CSM'
			  AND sp.target::TEXT = $2
			  AND sp.schedule = '`+slaScheduleWSO2BusinessDays+`'
			  AND s.stage::TEXT `+slaEngineOpenStageFilter+policyFilter+`
			FOR UPDATE OF s`, args...)
		if err != nil {
			return fmt.Errorf("set business-day sla clock paused: select: %w", err)
		}
		type openRow struct {
			id                                                  string
			startOn                                             time.Time
			pausedOn                                            *time.Time
			pauseSeconds, businessPauseSeconds, durationSeconds float64
		}
		var open []openRow
		for rows.Next() {
			var o openRow
			if err := rows.Scan(&o.id, &o.startOn, &o.pausedOn, &o.pauseSeconds, &o.businessPauseSeconds, &o.durationSeconds); err != nil {
				rows.Close()
				return fmt.Errorf("set business-day sla clock paused: scan: %w", err)
			}
			open = append(open, o)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("set business-day sla clock paused: rows: %w", err)
		}

		now := time.Now().UTC()
		for _, o := range open {
			flatPause := time.Duration(o.pauseSeconds * float64(time.Second))
			businessPause := time.Duration(o.businessPauseSeconds * float64(time.Second))
			duration := time.Duration(o.durationSeconds * float64(time.Second))

			var (
				newStage                            string
				newPausedOn                         *time.Time
				newFlatPause, newBusinessPause      time.Duration
				pct                                 *float64
				businessDuration, remainingDuration *time.Duration
			)
			if paused {
				newStage = "PAUSED"
				freeze := now
				if o.pausedOn != nil {
					freeze = *o.pausedOn // idempotent re-pause: re-derive the identical frozen snapshot
				}
				newPausedOn = &freeze
				newFlatPause = flatPause
				newBusinessPause = businessPause
				elapsed := businessdays.Elapsed(o.startOn, freeze) - businessPause
				if elapsed < 0 {
					elapsed = 0
				}
				remaining := duration - elapsed
				if remaining < 0 {
					remaining = 0
				}
				p := 0.0
				if o.durationSeconds > 0 {
					p = elapsed.Seconds() / o.durationSeconds * 100
				}
				pct = &p
				businessDuration = &elapsed
				remainingDuration = &remaining
			} else {
				newStage = "IN_PROGRESS"
				newPausedOn = nil
				newFlatPause = flatPause
				newBusinessPause = businessPause
				if o.pausedOn != nil {
					newFlatPause += now.Sub(*o.pausedOn)
					newBusinessPause += businessdays.Elapsed(*o.pausedOn, now)
				}
				// business_elapsed_percentage/business_duration/
				// remaining_business_duration are left untouched on resume --
				// same as setPausedFlat's own ELSE branches.
			}

			if _, err := tx.Exec(ctx, `
				UPDATE sla SET stage = $2::sla_stage_enum,
				    paused_on = $3,
				    pause_duration = $4::interval,
				    business_pause_duration = $5::interval,
				    business_elapsed_percentage = COALESCE($6, business_elapsed_percentage),
				    business_duration = COALESCE($7::interval, business_duration),
				    remaining_business_duration = COALESCE($8::interval, remaining_business_duration),
				    updated_on = NOW(), updated_by = $9
				WHERE id = $1::uuid`,
				o.id, newStage, newPausedOn, formatIntervalLiteral(newFlatPause), formatIntervalLiteral(newBusinessPause),
				pct, formatIntervalOrNil(businessDuration), formatIntervalOrNil(remainingDuration), sqlActorLiteral,
			); err != nil {
				return fmt.Errorf("set business-day sla clock paused: update: %w", err)
			}
			found = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// formatIntervalOrNil is formatIntervalLiteral for an optional *time.Duration
// -- nil passes through as a real SQL NULL so the UPDATE's own
// COALESCE($n, column) keeps the column's existing value, used by
// setPausedBusinessDays' resume branch (which must not touch these three
// columns at all).
func formatIntervalOrNil(d *time.Duration) any {
	if d == nil {
		return nil
	}
	return formatIntervalLiteral(*d)
}

// ReviseClocks implements SLAEngineRepository.
//
// Cancels every clock whose CURRENT stage is cancel-eligible for its own
// target (the cancelQuery's own OR branches below) -- per-target, unlike
// slaEngineActiveStageFilter (which excludes BREACHED for every target
// uniformly): RESPONSE never cancels a BREACHED row (a first-reply window
// that already closed unanswered is permanent, exactly like ACHIEVED is
// permanent -- see slaEngineRevisionBlockStages' own doc comment), while
// WORKAROUND/RESOLUTION do cancel a BREACHED row, since ongoing remediation
// work genuinely restarts under the new severity's own duration. A
// severity change means every OTHER clock type starts over "as if the case
// had just been created at the new severity" (see SLAEngineService.
// ReviseCaseClocks's own doc comment) -- this scans every CSM row on the
// work item, not just the targets in `policies`, so a clock type no longer
// applicable after a severity DOWNGRADE (e.g. losing "workaround"/
// "resolution") is still cancelled even though `policies` won't re-register
// it. Also sets is_active = FALSE on every row it cancels -- see
// CompleteClock's own doc comment for the is_active bug this is the other
// half of the fix for.
// it.
func (r *slaEngineRepo) ReviseClocks(ctx context.Context, workItemID string, policies []SLAPolicyRef) (int, error) {
	// Split flat vs. 'WSO2 Business days' the same way CompleteClock/SetPaused
	// do: has_breached must reflect real, pause-aware elapsed time at the
	// moment of cancellation (see this method's own doc comment on the
	// RESPONSE/BREACHED guard, which depends on a live-breached row actually
	// being recorded as such) -- completedElapsedExpr's flat wall-clock SQL
	// can express that for a 24x7 clock directly, but a business-day
	// calendar has no SQL formula, so those rows are read and updated
	// row-by-row in Go (reviseClocksCancelBusinessDays), same as
	// completeClockBusinessDays/setPausedBusinessDays already do.
	const flatCancelQuery = `
		UPDATE sla s
		SET stage = 'CANCELLED'::sla_stage_enum, is_active = FALSE,
		    has_breached = (` + completedElapsedExpr + ` >= s.duration),
		    updated_on = NOW(), updated_by = $2
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND COALESCE(sp.schedule, '') != '` + slaScheduleWSO2BusinessDays + `'
		  AND (
		    (sp.target::TEXT = 'RESPONSE' AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED'))
		    OR (sp.target::TEXT != 'RESPONSE' AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED', 'BREACHED'))
		  )`

	var cancelled int
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, flatCancelQuery, workItemID, sqlActorLiteral)
		if err != nil {
			return fmt.Errorf("revise csm sla clocks: cancel active: %w", err)
		}
		cancelled = int(tag.RowsAffected())

		bdCancelled, err := reviseClocksCancelBusinessDays(ctx, tx, workItemID)
		if err != nil {
			return fmt.Errorf("revise csm sla clocks: cancel active (business-day): %w", err)
		}
		cancelled += bdCancelled

		for _, policy := range policies {
			// A zero Time here means "start now" -- see registerClockExec's
			// own doc comment. A severity revision's own replacement clocks
			// always start at the instant of the triggering update, which
			// NOW() already approximates: this runs synchronously, inside
			// the same request that changed the severity, so there is no
			// meaningfully earlier "triggering update" instant to backdate
			// to even for a policy.Retroactive policy -- unlike a brand-new
			// case's own RegisterCaseClocks call, which can genuinely lag
			// the case's own creation instant.
			if _, err := registerClockExec(ctx, tx, workItemID, policy, time.Time{}); err != nil {
				return fmt.Errorf("revise csm sla clocks: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return cancelled, nil
}

// reviseClocksCancelBusinessDays is ReviseClocks' 'WSO2 Business days'
// branch -- same cancel-eligibility predicate as the flat cancel query above
// (RESPONSE never cancels a BREACHED row; WORKAROUND/RESOLUTION do), just
// computed with internal/businessdays.Elapsed, which a single bulk SQL
// statement has no way to express (the flat branch's completedElapsedExpr is
// pure wall-clock arithmetic SQL can do directly; a business-day calendar
// can't be). Returns the number of rows cancelled, added into ReviseClocks'
// own cancelled count.
func reviseClocksCancelBusinessDays(ctx context.Context, tx pgx.Tx, workItemID string) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id, s.start_on, s.paused_on,
		       COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0),
		       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0),
		       EXTRACT(EPOCH FROM s.duration)
		FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.schedule = '`+slaScheduleWSO2BusinessDays+`'
		  AND (
		    (sp.target::TEXT = 'RESPONSE' AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED'))
		    OR (sp.target::TEXT != 'RESPONSE' AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED', 'BREACHED'))
		  )
		FOR UPDATE OF s`, workItemID)
	if err != nil {
		return 0, fmt.Errorf("select: %w", err)
	}
	type openRow struct {
		id                                                  string
		startOn                                             time.Time
		pausedOn                                            *time.Time
		pauseSeconds, businessPauseSeconds, durationSeconds float64
	}
	var open []openRow
	for rows.Next() {
		var o openRow
		if err := rows.Scan(&o.id, &o.startOn, &o.pausedOn, &o.pauseSeconds, &o.businessPauseSeconds, &o.durationSeconds); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan: %w", err)
		}
		open = append(open, o)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows: %w", err)
	}

	now := time.Now().UTC()
	for _, o := range open {
		businessPause := time.Duration(o.businessPauseSeconds * float64(time.Second))
		if o.pausedOn != nil {
			businessPause += businessdays.Elapsed(*o.pausedOn, now)
		}
		elapsed := businessdays.Elapsed(o.startOn, now) - businessPause
		if elapsed < 0 {
			elapsed = 0
		}
		duration := time.Duration(o.durationSeconds * float64(time.Second))
		hasBreached := elapsed >= duration

		if _, err := tx.Exec(ctx, `
			UPDATE sla SET stage = 'CANCELLED'::sla_stage_enum, is_active = FALSE,
			    has_breached = $2,
			    updated_on = NOW(), updated_by = $3
			WHERE id = $1::uuid`,
			o.id, hasBreached, sqlActorLiteral,
		); err != nil {
			return 0, fmt.Errorf("update: %w", err)
		}
	}
	return len(open), nil
}

// sqlActorLiteral is created_by/updated_by for every row this repository
// writes -- see domain.SLAEngineActor's own doc comment.
const sqlActorLiteral = "sla-engine"

// formatIntervalLiteral renders a time.Duration as a Postgres INTERVAL
// literal ("3600 seconds") -- binding a Go time.Duration directly as
// ::interval has no pgx encode plan registered in this codebase (same
// class of issue as comment_repo.go's ::text[]::comment_type_enum[] cast
// note), so this passes a plain numeric-seconds string instead, which
// Postgres's own ::interval cast parses natively.
func formatIntervalLiteral(d time.Duration) string {
	return fmt.Sprintf("%d seconds", int64(d.Seconds()))
}

// nullableSchedule returns schedule as a bindable value, or a real SQL NULL
// for an empty string -- sla.schedule is nullable and an empty string isn't
// a meaningful schedule value to store (every real sla_policy row's
// schedule is either a real value or NULL, never "").
func nullableSchedule(schedule string) any {
	if schedule == "" {
		return nil
	}
	return schedule
}
