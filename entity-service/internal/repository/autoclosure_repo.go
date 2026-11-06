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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AutoclosureRowType says which extension table owns a candidate's workflow
// columns. Only "case" and security_report_analysis take part in the workflow
// (Query and Incident are both a "case", see the CLAUDE.md type-transfer notes).
type AutoclosureRowType string

const (
	AutoclosureRowCase AutoclosureRowType = "CASE"
	AutoclosureRowSRA  AutoclosureRowType = "SECURITY_REPORT_ANALYSIS"
)

// AutoclosureCandidateRow is a candidate plus what ClaimStep needs to claim it.
type AutoclosureCandidateRow struct {
	domain.AutoclosureCandidate
	RowType AutoclosureRowType
	// StateOn is autoclosure_state_on as read; ClaimStep compares against it.
	StateOn *time.Time
}

// AutoclosureCandidateQuery selects tickets due for one stage.
type AutoclosureCandidateQuery struct {
	Stage        domain.AutoclosureStage
	Now          time.Time
	IntervalDays int
	Limit        int
	// States are the upper-snake state labels to consider (AWAITING_INFO,
	// SOLUTION_PROPOSED).
	States []string
	// OnlyID restricts the query to one ticket: how Advance re-checks that the
	// ticket is still due, with the very predicate that listed it.
	OnlyID string
	// SystemEmail is the system user's email. Its comments (the reminders) are
	// never counted as a staff reply, so a reminder cannot restart the clock.
	SystemEmail string
	// MaxFailures and FailureWindowDays keep a ticket that fails the same stage
	// again and again out of the list: one with MaxFailures or more FAILED audit
	// rows for this stage inside the window is skipped until the oldest ages out
	// of it. Zero MaxFailures turns the rule off.
	MaxFailures       int
	FailureWindowDays int
}

// AutoclosureEvent is one audit row.
type AutoclosureEvent struct {
	WorkItemID string
	Stage      domain.AutoclosureStage
	FromStep   *string
	ToStep     *string
	Outcome    string
	Detail     string
}

// AutoclosureRepository persists the auto-closure workflow's own state. The
// workflow position lives on the case-like tables (autoclosure_step /
// autoclosure_state_on); this repository is the only writer of it besides the
// customer-requested hold (setAutocloseHoldTx) and the reset trigger in
// migration 0249.
type AutoclosureRepository interface {
	// FindCandidates lists the tickets due for q.Stage, oldest reference time
	// first.
	FindCandidates(ctx context.Context, q AutoclosureCandidateQuery) ([]AutoclosureCandidateRow, error)
	// ClaimStep moves a ticket from (expectedStep, expectedStateOn) to
	// (newStep, now) only if it still holds exactly those values and is still
	// in expectedState. That compare-and-set is what keeps two overlapping runs
	// from both acting on one ticket. Reports whether this caller won.
	ClaimStep(ctx context.Context, c AutoclosureCandidateRow, newStep string, now time.Time) (bool, error)
	// RestoreStep undoes a claim after the action it guarded failed, only if
	// the claim is still the latest write.
	RestoreStep(ctx context.Context, c AutoclosureCandidateRow, claimedAt time.Time) error
	// RecordEvent appends an audit row.
	RecordEvent(ctx context.Context, ev AutoclosureEvent) error
	// CountQuarantined reports how many tickets FindCandidates is skipping for
	// stage because of repeated failures (see AutoclosureCandidateQuery.MaxFailures).
	// It is approximate: it counts tickets with that many recent failures
	// whether or not they are still due, and exists so the skipping is visible
	// in the logs instead of silent.
	CountQuarantined(ctx context.Context, stage domain.AutoclosureStage, now time.Time, windowDays, maxFailures int) (int, error)
	// CountAdvancedSince counts the reminders and closures performed since
	// the given instant (hold releases excluded: they notify nobody). It is
	// what makes the daily limit hold across retried runs.
	CountAdvancedSince(ctx context.Context, since time.Time) (int, error)
	// DisplayNameOf returns how a user is shown to people (the "name" column,
	// else first and last name), or the email itself when the user has none
	// or does not exist. It is the same name the case timeline shows for a
	// comment the user wrote.
	DisplayNameOf(ctx context.Context, email string) (string, error)
}

type autoclosureRepo struct {
	db *Scoped
}

// NewAutoclosureRepository constructs the repository.
func NewAutoclosureRepository(db *Scoped) AutoclosureRepository {
	return &autoclosureRepo{db: db}
}

// autoclosureBaseCTE reads every ticket of the two workflow tables that could
// possibly be eligible: enabled, in one of the waiting states, and a parent
// (a child never has a workflow of its own).
const autoclosureBaseCTE = `
	WITH base AS (
		SELECT wi.id, wi.number, 'CASE'::text AS row_type, c.state::text AS state,
		       c.autoclosure_step AS step, c.autoclosure_state_on AS state_on,
		       c.resolved_on, wi.updated_on
		FROM work_item wi
		JOIN "case" c ON c.id = wi.id
		WHERE wi.type = 'CASE'
		  AND c.autoclose_enabled
		  AND c.state::text = ANY($1::text[])
		  AND wi.parent_id IS NULL
		UNION ALL
		SELECT wi.id, wi.number, 'SECURITY_REPORT_ANALYSIS'::text, s.state::text,
		       s.autoclosure_step, s.autoclosure_state_on,
		       s.resolved_on, wi.updated_on
		FROM work_item wi
		JOIN security_report_analysis s ON s.id = wi.id
		WHERE wi.type = 'SECURITY_REPORT_ANALYSIS'
		  AND s.autoclose_enabled
		  AND s.state::text = ANY($1::text[])
		  AND wi.parent_id IS NULL
	)`

// autoclosureNoOpenChild is true for a ticket whose children are all closed.
// Children are case-like work items (the same state rule UpdateCase applies
// before it lets a parent close) and change requests (closed, canceled or
// rolled back count as done).
const autoclosureNoOpenChild = `
	AND NOT EXISTS (
		SELECT 1
		FROM work_item wi
		LEFT JOIN "case" c ON c.id = wi.id
		` + caseLikeJoins + `
		WHERE wi.parent_id = b.id
		  AND wi.type = ANY(` + caseLikeWorkItemTypes + `)
		  AND COALESCE(` + caseLikeStateColumn + `, '') <> 'CLOSED')
	AND NOT EXISTS (
		SELECT 1
		FROM work_item wcr
		JOIN change_request cr ON cr.id = wcr.id
		WHERE wcr.parent_id = b.id
		  AND COALESCE(cr.state::text, 'NEW') NOT IN ('CLOSED', 'CANCELED', 'ROLLBACK'))`

// autoclosureFirstReminderRef is when the first reminder's interval starts.
//
//   - Solution Proposed: when the solution was proposed (the later of the
//     resolved-on stamp and the activity entry for entering the state).
//   - Awaiting Info: the later of the last reply from a WSO2 user and entering
//     the state. Only replies by an INTERNAL user count, never the system
//     user's own reminders, and never work notes.
//   - A ticket whose workflow was reset (step DEFAULT) also waits out the
//     interval from the reset, so a ticket released from a hold, or one that
//     just came back from the customer, gets a fresh clock instead of an
//     immediate reminder for something that is old only on paper.
//   - A ticket with none of these falls back to its last update.
const autoclosureFirstReminderRef = `
	COALESCE(GREATEST(
		CASE WHEN b.state = 'SOLUTION_PROPOSED' THEN b.resolved_on END,
		(SELECT MAX(a.created_on) FROM work_item_activity a
		  WHERE a.work_item_id = b.id AND a.field_name = 'state'
		    AND a.new_value = CASE b.state WHEN 'SOLUTION_PROPOSED' THEN 'Solution Proposed' ELSE 'Awaiting Info' END),
		CASE WHEN b.state = 'AWAITING_INFO' THEN (
			SELECT MAX(cm.created_on) FROM comment cm
			 WHERE cm.work_item_id = b.id AND cm.type = 'COMMENT'
			   AND LOWER(cm.created_by) <> LOWER($6::text)
			   AND EXISTS (SELECT 1 FROM "user" u
			                WHERE LOWER(u.email) = LOWER(cm.created_by) AND u.user_type = 'INTERNAL')) END,
		CASE WHEN b.step = 'DEFAULT' THEN b.state_on END
	), b.updated_on)`

// FindCandidates implements AutoclosureRepository.
func (r *autoclosureRepo) FindCandidates(ctx context.Context, q AutoclosureCandidateQuery) ([]AutoclosureCandidateRow, error) {
	var refExpr, stepFilter, childFilter string
	switch q.Stage {
	case domain.AutoclosureStageFirstReminder:
		refExpr = autoclosureFirstReminderRef
		stepFilter = `AND COALESCE(NULLIF(b.step, ''), 'DEFAULT') = 'DEFAULT'`
		childFilter = autoclosureNoOpenChild
	case domain.AutoclosureStageSecondReminder:
		refExpr = `b.state_on`
		stepFilter = `AND b.step = 'FIRST_COMMENT' AND b.state_on IS NOT NULL`
		childFilter = autoclosureNoOpenChild
	case domain.AutoclosureStageClose:
		refExpr = `b.state_on`
		stepFilter = `AND b.step = 'SECOND_COMMENT' AND b.state_on IS NOT NULL`
		childFilter = autoclosureNoOpenChild
	case domain.AutoclosureStageReleaseHold:
		// A hold names the last day it protects (the UTC day stored in
		// autoclosure_state_on), so it ends when that day is over.
		refExpr = `b.state_on + INTERVAL '1 day'`
		stepFilter = `AND b.step = 'ON_HOLD' AND b.state_on IS NOT NULL`
	default:
		return nil, fmt.Errorf("autoclosure candidates: unknown stage %q", q.Stage)
	}

	// release_hold is due when the hold is over (ref <= now); the others when
	// the interval has elapsed since their reference time.
	dueFilter := `q.ref_time <= $2::timestamptz - make_interval(days => $3::int)`
	days := q.IntervalDays
	if q.Stage == domain.AutoclosureStageReleaseHold {
		days = 0
	}

	query := autoclosureBaseCTE + `
	SELECT q.id, q.number, q.row_type, q.state, q.step, q.state_on, q.ref_time
	FROM (
		SELECT b.id, b.number, b.row_type, b.state, b.step, b.state_on,
		       ` + refExpr + ` AS ref_time
		FROM base b
		WHERE ($5::text = '' OR b.id = $5::uuid)
		  AND $6::text IS NOT NULL -- Postgres rejects a parameter no stage references
		  AND ($9::int <= 0 OR (
		        SELECT COUNT(*) FROM case_autoclosure_event fe
		         WHERE fe.work_item_id = b.id AND fe.stage = $7::text AND fe.outcome = 'FAILED'
		           AND fe.created_on > $2::timestamptz - make_interval(days => $8::int)
		      ) < $9::int)
		  ` + stepFilter + childFilter + `
	) q
	WHERE ` + dueFilter + `
	ORDER BY q.ref_time ASC, q.id ASC
	LIMIT $4::int`

	rows, err := r.db.Query(ctx, query, q.States, q.Now, days, q.Limit, q.OnlyID, q.SystemEmail,
		string(q.Stage), q.FailureWindowDays, q.MaxFailures)
	if err != nil {
		return nil, fmt.Errorf("autoclosure candidates: %w", err)
	}
	defer rows.Close()

	var out []AutoclosureCandidateRow
	for rows.Next() {
		var (
			c       AutoclosureCandidateRow
			rowType string
			state   string
			step    *string
		)
		if err := rows.Scan(&c.ID, &c.Number, &rowType, &state, &step, &c.StateOn, &c.ReferenceTime); err != nil {
			return nil, fmt.Errorf("autoclosure candidates: scan: %w", err)
		}
		c.RowType = AutoclosureRowType(rowType)
		c.State = domain.CaseState(strings.ToLower(state))
		c.Step = step
		c.ReferenceTime = c.ReferenceTime.UTC()
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("autoclosure candidates: %w", err)
	}
	return out, nil
}

// autoclosureTable is the extension table that owns a candidate's workflow
// columns. Chosen from a closed set, never from request text.
func autoclosureTable(t AutoclosureRowType) (string, error) {
	switch t {
	case AutoclosureRowCase:
		return `"case"`, nil
	case AutoclosureRowSRA:
		return `security_report_analysis`, nil
	default:
		return "", fmt.Errorf("autoclosure: unsupported row type %q", t)
	}
}

// ClaimStep implements AutoclosureRepository.
func (r *autoclosureRepo) ClaimStep(ctx context.Context, c AutoclosureCandidateRow, newStep string, now time.Time) (bool, error) {
	table, err := autoclosureTable(c.RowType)
	if err != nil {
		return false, err
	}
	tag, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		   SET autoclosure_step = $3::text, autoclosure_state_on = $4::timestamptz
		 WHERE id = $1::uuid
		   AND autoclosure_step IS NOT DISTINCT FROM $2::text
		   AND autoclosure_state_on IS NOT DISTINCT FROM $5::timestamptz
		   AND state::text = $6::text`, table),
		c.ID, c.Step, newStep, now, c.StateOn, strings.ToUpper(string(c.State)))
	if err != nil {
		return false, fmt.Errorf("autoclosure claim: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// RestoreStep implements AutoclosureRepository.
func (r *autoclosureRepo) RestoreStep(ctx context.Context, c AutoclosureCandidateRow, claimedAt time.Time) error {
	table, err := autoclosureTable(c.RowType)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		   SET autoclosure_step = $2::text, autoclosure_state_on = $3::timestamptz
		 WHERE id = $1::uuid AND autoclosure_state_on = $4::timestamptz`, table),
		c.ID, c.Step, c.StateOn, claimedAt); err != nil {
		return fmt.Errorf("autoclosure restore: %w", err)
	}
	return nil
}

// RecordEvent implements AutoclosureRepository.
func (r *autoclosureRepo) RecordEvent(ctx context.Context, ev AutoclosureEvent) error {
	var detail *string
	if ev.Detail != "" {
		detail = &ev.Detail
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO case_autoclosure_event (work_item_id, stage, from_step, to_step, outcome, detail)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)`,
		ev.WorkItemID, string(ev.Stage), ev.FromStep, ev.ToStep, ev.Outcome, detail); err != nil {
		return fmt.Errorf("autoclosure event: %w", err)
	}
	return nil
}

// CountAdvancedSince implements AutoclosureRepository.
func (r *autoclosureRepo) CountAdvancedSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM case_autoclosure_event
		 WHERE outcome = 'ADVANCED' AND created_on >= $1::timestamptz
		   AND stage IN ('first_reminder', 'second_reminder', 'close')`, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("autoclosure count: %w", err)
	}
	return n, nil
}

// DisplayNameOf implements AutoclosureRepository.
func (r *autoclosureRepo) DisplayNameOf(ctx context.Context, email string) (string, error) {
	var name *string
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(TRIM(name), ''), NULLIF(TRIM(CONCAT_WS(' ', first_name, last_name)), ''))
		  FROM "user"
		 WHERE LOWER(email) = LOWER($1::text)
		 ORDER BY id
		 LIMIT 1`, email).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return email, nil
		}
		return "", fmt.Errorf("autoclosure display name: %w", err)
	}
	if name == nil || *name == "" {
		return email, nil
	}
	return *name, nil
}

// CountQuarantined implements AutoclosureRepository.
func (r *autoclosureRepo) CountQuarantined(ctx context.Context, stage domain.AutoclosureStage, now time.Time, windowDays, maxFailures int) (int, error) {
	if maxFailures <= 0 {
		return 0, nil
	}
	var n int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT work_item_id FROM case_autoclosure_event
			 WHERE stage = $1::text AND outcome = 'FAILED'
			   AND created_on > $2::timestamptz - make_interval(days => $3::int)
			 GROUP BY work_item_id HAVING COUNT(*) >= $4::int
		) q`, string(stage), now, windowDays, maxFailures).Scan(&n); err != nil {
		return 0, fmt.Errorf("autoclosure quarantined count: %w", err)
	}
	return n, nil
}
