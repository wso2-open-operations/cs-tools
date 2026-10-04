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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

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
}

// SLAEngineRepository defines the read/write operations backing the
// CSM-native SLA engine (internal/service/sla_policy_resolver.go,
// sla_engine_service.go) -- every write here is scoped to source='CSM' rows
// only (migration 0134): source='SERVICENOW' rows are exclusively owned
// by the ServiceNow sync and this repository never mutates one. Read-side
// consumers of "sla" (GET /sla-status, POST /task-slas/search) need no
// changes at all -- both source values look identical to them, which is the
// entire point of sharing the table (see migration 0134's own comment).
type SLAEngineRepository interface {
	// FindPolicyByName resolves the single active sla_policy row matching
	// name/target, preferring a source='SERVICENOW' row (the real
	// ServiceNow-synced policy) but falling back to a source='CSM' row (a
	// gap-filling policy this engine itself seeded, e.g. the P0 rows added
	// by migration 0136) when no synced row exists under that exact name.
	// Returns apierror.NotFoundError if neither exists.
	FindPolicyByName(ctx context.Context, name, target string) (SLAPolicyRef, error)

	// FindPolicyByPattern is sla_policy_resolver.go's last-resort fallback,
	// tried only once FindPolicyByName has failed under both plan labels --
	// see resolve's own doc comment for why. Real ServiceNow tenants outside
	// prod (confirmed on the dev instance's synced data) don't all
	// follow the "P{n} - {Type} ({Plan})" naming convention prod's policies
	// were verified against, e.g. "P2 - IR - Resolution (Open Source)"
	// instead of "P2 - Resolution (Open Source)" -- an exact-name lookup
	// finds nothing there even though a policy for that severity/clockType
	// clearly exists. Matches any name that starts with "<prefix> - " and
	// contains <label> anywhere after that, for the given target, preferring
	// the shortest matching name (closest to the canonical form) when more
	// than one qualifies. Returns apierror.NotFoundError if none match --
	// callers treat that exactly like FindPolicyByName's own NotFoundError.
	// derivedPlan is a preference, not a filter: a matching name containing
	// it is ranked first, but a policy that doesn't mention any plan at all
	// is still returned rather than treated as absent -- see this method's
	// own implementation doc comment for the full ordering rule.
	FindPolicyByPattern(ctx context.Context, prefix, label, target, derivedPlan string) (SLAPolicyRef, error)

	// RegisterClock inserts a new source='CSM' "sla" row for
	// (workItemID, policy.Target) and starts it running now, UNLESS an
	// active (see slaEngineActiveStageFilter) source='CSM' row already
	// exists for that (work_item, target) pair -- idempotent, so a retried
	// case-create hook never double-registers -- OR a row already exists in
	// a stage slaEngineRevisionBlockStages(policy.Target) names as blocking
	// for that target (per-target -- see that function's own doc comment for
	// why RESPONSE differs from WORKAROUND/RESOLUTION). A CANCELLED row does
	// NOT block a fresh insert -- cancellation deliberately frees that clock
	// type up for a genuinely new one (see ReviseClocks). Returns whether a
	// row was actually inserted.
	RegisterClock(ctx context.Context, workItemID string, policy SLAPolicyRef) (bool, error)

	// CompleteClock marks the source='CSM' clock for (workItemID, target)
	// ACHIEVED (end_on=now, business_elapsed_percentage set to the clock's
	// real, uncapped elapsed percentage at completion time, not
	// unconditionally 100 -- see this method's own implementation comment).
	// Matches a clock in ANY not-yet-final stage, including BREACHED -- a
	// clock whose window already ran out is still completable by its own
	// real finishing event, with whatever real overrun percentage that
	// event happened at (see slaEngineOpenStageFilter's own doc comment).
	// Returns whether a row was found and updated -- false (not an error) when no
	// such clock was ever registered, e.g. a LOW/Query-severity case, which
	// never gets a "response" clock's workaround/resolution siblings, or a
	// case whose policy lookup found nothing at create time.
	CompleteClock(ctx context.Context, workItemID, target string) (bool, error)

	// SetPaused pauses (true) or resumes (false) the source='CSM' clock for
	// (workItemID, target), matching any not-yet-final stage including
	// BREACHED (see slaEngineOpenStageFilter) -- so an already-breached
	// clock can still be paused while its case waits on the customer,
	// rather than continuing to silently accumulate elapsed time. Idempotent
	// and a no-op (not an error) when no such clock exists, same reasoning
	// as CompleteClock.
	SetPaused(ctx context.Context, workItemID, target string, paused bool) (bool, error)

	// RecomputeActive recomputes business_elapsed_percentage (uncapped --
	// it keeps climbing past 100 for as long as a clock remains BREACHED)
	// for every source='CSM' row currently IN_PROGRESS or BREACHED
	// (deliberately narrower than slaEngineOpenStageFilter -- see this
	// method's own doc comment on its implementation for why PAUSED is
	// excluded too), flips has_breached and stage to BREACHED once elapsed
	// time first reaches the policy duration, and returns how many rows
	// were touched. Including BREACHED here (not just IN_PROGRESS) is what
	// keeps a breached clock's elapsed time/percentage moving until its own
	// real completing event finalizes it, instead of freezing forever at
	// whatever value the tick that first crossed 100% happened to compute.
	RecomputeActive(ctx context.Context) (int, error)

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
}

// NewSLAEngineRepository constructs an SLAEngineRepository backed by the
// given Scoped connection. This engine runs on a process-startup background
// worker (cmd/api/main.go's slaEngineCtx), never an HTTP request, so that
// context must carry WithSystemIdentity(ctx) rather than inheriting nothing
// -- Scoped requires an identity on ctx even though sla itself has no RLS
// since migration 0153.
func NewSLAEngineRepository(db *Scoped) SLAEngineRepository {
	return &slaEngineRepo{db: db}
}

// FindPolicyByName implements SLAEngineRepository.
//
// ORDER BY source: 'SERVICENOW' sorts before 'CSM' lexically, so a plain
// ORDER BY source, then LIMIT 1, prefers the real synced policy whenever
// both a SERVICENOW and a CSM row happen to share a name -- which should
// never actually happen (this engine only ever seeds names ServiceNow's
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

// FindPolicyByPattern implements SLAEngineRepository.
//
// ORDER BY a plan-match rank first, then length(name), then source: a row
// whose name contains derivedPlan always sorts ahead of one that doesn't,
// regardless of length -- CodeRabbit correctly flagged that plain
// length(name) ordering alone can pick the wrong plan, e.g. preferring a
// shorter "P2 - IR - Resolution (Open Source)" over the derived plan's own
// "P2 - IR - Resolution (Managed Services)" purely because it's shorter.
// Within the same plan-match rank, shortest name first still prefers a
// plain "<prefix> - <label> (<plan>)" row over a longer, more qualified
// variant like "<prefix> - IR - <label> (<plan>)"; source is the final
// tiebreaker for the same reason FindPolicyByName uses it. derivedPlan is
// still only ever a preference, never a filter -- a policy that doesn't
// mention it at all is still returned (matching resolve()'s own two-plan
// fallback philosophy: a guessed-wrong plan must not silently drop SLA
// tracking).
func (r *slaEngineRepo) FindPolicyByPattern(ctx context.Context, prefix, label, target, derivedPlan string) (SLAPolicyRef, error) {
	const query = `
		SELECT id, name, target::TEXT, EXTRACT(EPOCH FROM duration)
		FROM sla_policy
		WHERE name ILIKE $1 || ' - %'
		  AND name ILIKE '%' || $2 || '%'
		  AND target = $3::sla_policy_target_enum
		  AND source IN ('SERVICENOW', 'CSM')
		  AND (is_active IS NULL OR is_active)
		  AND duration IS NOT NULL
		ORDER BY (CASE WHEN name ILIKE '%' || $4 || '%' THEN 0 ELSE 1 END), length(name), source
		LIMIT 1`

	var ref SLAPolicyRef
	var durationSeconds float64
	err := r.db.QueryRow(ctx, query, prefix, label, target, derivedPlan).Scan(&ref.ID, &ref.Name, &ref.Target, &durationSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return SLAPolicyRef{}, &apierror.NotFoundError{Msg: "no sla_policy found matching " + prefix + "/" + label + "/" + target}
	}
	if err != nil {
		return SLAPolicyRef{}, fmt.Errorf("find sla policy by pattern: %w", err)
	}
	ref.Duration = time.Duration(durationSeconds * float64(time.Second))
	return ref, nil
}

// slaEngineRegisterClockQuery inserts a new source='CSM' "sla" row for
// ($1=workItemID, $5=policy.Target) unless an existing row for that pair is
// either still active (slaEngineActiveStageFilter) or already reached a
// stage $6 names as blocking (slaEngineRevisionBlockStages(policy.Target),
// bound by registerClockExec -- per-target, see that function's own doc
// comment) -- a CANCELLED row blocks neither, deliberately, since
// cancellation is what frees a clock type up for a fresh registration (see
// SLAEngineRepository.ReviseClocks). Shared, identical SQL text between
// RegisterClock (run against the pool directly) and ReviseClocks (run
// inside its own transaction) -- sqlExecutor is satisfied by both
// *pgxpool.Pool and pgx.Tx.
const slaEngineRegisterClockQuery = `
	INSERT INTO sla (
		id, created_on, updated_on, created_by, updated_by,
		work_item_id, sla_policy_id, is_active, stage, start_on,
		duration, business_elapsed_percentage, business_duration,
		remaining_business_duration, has_breached, source
	)
	SELECT gen_random_uuid(), NOW(), NOW(), $3, $3,
	       $1::uuid, $2::uuid, TRUE, 'IN_PROGRESS'::sla_stage_enum, NOW(),
	       $4::interval, 0, INTERVAL '0', $4::interval, FALSE, 'CSM'::sla_source_enum
	WHERE NOT EXISTS (
		SELECT 1 FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $5
		  AND s.stage::TEXT ` + slaEngineActiveStageFilter + `
	)
	AND NOT EXISTS (
		SELECT 1 FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $5
		  AND s.stage::TEXT = ANY($6::text[])
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
func registerClockExec(ctx context.Context, exec sqlExecutor, workItemID string, policy SLAPolicyRef) (bool, error) {
	tag, err := exec.Exec(ctx, slaEngineRegisterClockQuery, workItemID, policy.ID, sqlActorLiteral, formatIntervalLiteral(policy.Duration), policy.Target, slaEngineRevisionBlockStages(policy.Target))
	if err != nil {
		return false, fmt.Errorf("register csm sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// lockWorkItemClocks serializes clock registration and revision for one work
// item for the rest of tx. slaEngineRegisterClockQuery's INSERT ... WHERE NOT
// EXISTS is not atomic under READ COMMITTED, and there is no unique index to
// back it up (target lives on sla_policy, not sla), so two registrations for
// the same (work item, target) racing each other -- or one racing a
// ReviseClocks -- could both see no open clock and both insert, leaving two
// active clocks that every SLA read and breach alert then double-counts.
// Holding this lock, each statement that follows sees the other side's
// committed rows. Same transaction-scoped pattern as case_repo.go's
// one-ongoing-case lock.
func lockWorkItemClocks(ctx context.Context, tx pgx.Tx, workItemID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('sla-clock:' || $1::text, 0))`, workItemID); err != nil {
		return fmt.Errorf("lock csm sla clocks: %w", err)
	}
	return nil
}

// RegisterClock implements SLAEngineRepository. Runs in its own short
// transaction so it can take lockWorkItemClocks.
func (r *slaEngineRepo) RegisterClock(ctx context.Context, workItemID string, policy SLAPolicyRef) (bool, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (bool, error) {
		if err := lockWorkItemClocks(ctx, tx, workItemID); err != nil {
			return false, err
		}
		return registerClockExec(ctx, tx, workItemID, policy)
	})
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
func (r *slaEngineRepo) CompleteClock(ctx context.Context, workItemID, target string) (bool, error) {
	const query = `
		UPDATE sla s
		SET stage = 'ACHIEVED'::sla_stage_enum, end_on = NOW(),
		    business_elapsed_percentage = GREATEST(0,
		        EXTRACT(EPOCH FROM (NOW() - s.start_on)) / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100
		    ),
		    business_duration = NOW() - s.start_on,
		    remaining_business_duration = GREATEST(s.duration - (NOW() - s.start_on), INTERVAL '0'),
		    updated_on = NOW(), updated_by = $3
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter

	tag, err := r.db.Exec(ctx, query, workItemID, target, sqlActorLiteral)
	if err != nil {
		return false, fmt.Errorf("complete csm sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SetPaused implements SLAEngineRepository.
//
// Matches on slaEngineOpenStageFilter, not slaEngineActiveStageFilter --
// a BREACHED clock must still be pausable (e.g. a workaround/resolution
// clock that breached while the case was still open, which then moves to
// AWAITING_INFO) so RecomputeActive actually stops touching it rather than
// continuing to climb its elapsed time while the case waits on the
// customer; see slaEngineOpenStageFilter's own doc comment.
func (r *slaEngineRepo) SetPaused(ctx context.Context, workItemID, target string, paused bool) (bool, error) {
	const query = `
		UPDATE sla s
		SET stage = CASE WHEN $3 THEN 'PAUSED'::sla_stage_enum ELSE 'IN_PROGRESS'::sla_stage_enum END,
		    paused_on = CASE WHEN $3 THEN NOW() ELSE NULL END,
		    updated_on = NOW(), updated_by = $4
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter

	tag, err := r.db.Exec(ctx, query, workItemID, target, paused, sqlActorLiteral)
	if err != nil {
		return false, fmt.Errorf("set csm sla clock paused: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// RecomputeActive implements SLAEngineRepository.
//
// Scoped to stage IN ('IN_PROGRESS', 'BREACHED') -- deliberately still
// narrower than slaEngineOpenStageFilter, which would also match PAUSED: a
// paused clock's whole point is to stop accumulating elapsed time, so
// recomputing its percentage against wall-clock "now" would silently undo
// SetPaused's own effect the very next tick. Excluding PAUSED here is what
// makes pause actually pause.
//
// BREACHED is included here -- unlike an earlier version of this query,
// which stopped touching a row the moment it first flipped to BREACHED --
// specifically so a clock that has already breached keeps accumulating
// real elapsed time/percentage until its own genuine completing event
// (CompleteClock) finally finalizes it, however much later that is. That
// earlier version was a real, reported bug: a response SLA observed live
// froze at its business_duration/business_elapsed_percentage from the
// exact tick it crossed 100% (e.g. stuck at "59m" forever), never
// reflecting that real time kept passing while the case sat unanswered.
// business_elapsed_percentage is no longer capped at 100 for the same
// reason CompleteClock's own cap was dropped -- see that method's doc
// comment; a clock still climbs past 100% here for as long as it remains
// BREACHED, and CompleteClock later reads the exact same uncapped value at
// whatever moment it actually finishes. Still floored at 0 (GREATEST(0,
// ...)) for the same clock-skew reason CompleteClock keeps that floor.
//
// No business-hours calendar: elapsed is flat wall-clock time since
// start_on, exactly as crude as the deleted sla_clocks map this replaces
// (see internal/service/sla_policy_resolver.go's own doc comment) -- and,
// same as that old code, resuming a paused clock does NOT extend start_on
// or duration to account for the paused interval, so time spent paused is
// silently counted once the clock resumes. A known, accepted gap, not
// something this method works around.
func (r *slaEngineRepo) RecomputeActive(ctx context.Context) (int, error) {
	const query = `
		UPDATE sla
		SET business_elapsed_percentage = GREATEST(0,
		        EXTRACT(EPOCH FROM (NOW() - start_on)) / NULLIF(EXTRACT(EPOCH FROM duration), 0) * 100
		    ),
		    business_duration = NOW() - start_on,
		    remaining_business_duration = GREATEST(duration - (NOW() - start_on), INTERVAL '0'),
		    has_breached = has_breached OR (EXTRACT(EPOCH FROM (NOW() - start_on)) >= EXTRACT(EPOCH FROM duration)),
		    stage = CASE
		        WHEN EXTRACT(EPOCH FROM (NOW() - start_on)) >= EXTRACT(EPOCH FROM duration)
		        THEN 'BREACHED'::sla_stage_enum
		        ELSE stage
		    END,
		    updated_on = NOW(), updated_by = $1
		WHERE source = 'CSM'
		  AND stage IN ('IN_PROGRESS', 'BREACHED')
		  AND start_on IS NOT NULL
		  AND duration IS NOT NULL`

	tag, err := r.db.Exec(ctx, query, sqlActorLiteral)
	if err != nil {
		return 0, fmt.Errorf("recompute csm sla clocks: %w", err)
	}
	return int(tag.RowsAffected()), nil
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
// it.
func (r *slaEngineRepo) ReviseClocks(ctx context.Context, workItemID string, policies []SLAPolicyRef) (int, error) {
	const cancelQuery = `
		UPDATE sla s
		SET stage = 'CANCELLED'::sla_stage_enum,
		    updated_on = NOW(), updated_by = $2
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND (
		    (sp.target::TEXT = 'RESPONSE' AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED'))
		    OR (sp.target::TEXT != 'RESPONSE' AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED', 'BREACHED'))
		  )`

	var cancelled int
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := lockWorkItemClocks(ctx, tx, workItemID); err != nil {
			return fmt.Errorf("revise csm sla clocks: %w", err)
		}
		tag, err := tx.Exec(ctx, cancelQuery, workItemID, sqlActorLiteral)
		if err != nil {
			return fmt.Errorf("revise csm sla clocks: cancel active: %w", err)
		}
		cancelled = int(tag.RowsAffected())

		for _, policy := range policies {
			if _, err := registerClockExec(ctx, tx, workItemID, policy); err != nil {
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
