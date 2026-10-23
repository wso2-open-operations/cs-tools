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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// rotaImportReplaceNote is the note on the activity row of an assignment, or time away, a replace cleared.
const rotaImportReplaceNote = "Cleared by a rota import (replace)"

// RotaImportWrite is a rota import file, resolved and authorised, ready to write.
type RotaImportWrite struct {
	RotaCode string
	// TeamKeys and From..To bound what a replace clears: every assignment of these teams on these days.
	TeamKeys []string
	From     string
	To       string
	Replace  bool
	// DryRun runs every write and rolls the transaction back, so the answer is exactly what a real import would do.
	DryRun      bool
	Assignments []domain.ApplyScheduleRangeRequest
	Absences    []domain.ApplyScheduleAbsenceRequest
}

// RotaImportResult is what each entry did, in the file's order.
type RotaImportResult struct {
	Cleared int
	// LeaveWrittenOver is the earlier time away a replace removed, shortened or split under the file's assignments.
	LeaveWrittenOver int
	Assignments      []domain.ApplyScheduleRangeResponse
	Absences         []domain.ApplyScheduleAbsenceResponse
}

// ImportRota implements ScheduleRepository: the whole file in one transaction, all or nothing.
func (r *scheduleRepository) ImportRota(ctx context.Context, w RotaImportWrite, actorEmail string) (RotaImportResult, error) {
	var out RotaImportResult
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return out, fmt.Errorf("begin rota import: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The generator's own lock, so an import and a "Generate month" of the same rota never interleave.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, RotaGeneratorOwner(w.RotaCode)); err != nil {
		return out, fmt.Errorf("lock rota import: %w", err)
	}
	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return out, err
	}

	if w.Replace {
		// A move's shifts (source MOVE) follow its absence, which a replace keeps, so they stay too.
		// Each cleared row leaves a DELETED activity row, as the roster's own delete does, written from the row as it was.
		tag, err := tx.Exec(ctx, `
			WITH cleared AS (
			  DELETE FROM team_schedule_assignment
			   WHERE team_key = ANY($1::text[])
			     AND rota_date BETWEEN $2::date AND $3::date
			     AND source <> 'MOVE'
			  RETURNING id, user_id, team_key, rota_date, shift_id)
			INSERT INTO team_schedule_assignment_activity
			  (id, created_on, created_by, assignment_id, user_id, team_key, rota_date,
			   shift_code, action, actor_email, note)
			SELECT gen_random_uuid(), now(), $4, c.id, c.user_id, c.team_key, c.rota_date,
			       s.code, 'DELETED', $4, $5
			  FROM cleared c
			  JOIN team_schedule_shift s ON s.id = c.shift_id`,
			w.TeamKeys, w.From, w.To, actorEmail, rotaImportReplaceNote)
		if err != nil {
			return out, fmt.Errorf("clear rota for import: %w", err)
		}
		out.Cleared = int(tag.RowsAffected())
	}

	scopes := map[string]string{}
	for i, req := range w.Assignments {
		if w.Replace {
			n, err := cutLeaveUnderAssignment(ctx, tx, req, w.TeamKeys, scopes, actorEmail)
			if err != nil {
				return RotaImportResult{}, apierror.Prefixed(fmt.Sprintf("assignments[%d]", i), err)
			}
			out.LeaveWrittenOver += n
		}
		res, err := applyRangeTx(ctx, tx, req, actorEmail)
		if err != nil {
			return RotaImportResult{}, apierror.Prefixed(fmt.Sprintf("assignments[%d]", i), err)
		}
		out.Assignments = append(out.Assignments, res)
	}
	for i, req := range w.Absences {
		res, err := applyAbsenceTx(ctx, tx, req, actorEmail)
		if err != nil {
			return RotaImportResult{}, apierror.Prefixed(fmt.Sprintf("absences[%d]", i), err)
		}
		out.Absences = append(out.Absences, res)
	}

	if w.DryRun {
		return out, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return RotaImportResult{}, fmt.Errorf("commit rota import: %w", err)
	}
	return out, nil
}

// cutLeaveUnderAssignment takes the days an entry's window is worked out of the person's time away filed on the rota's teams, a run of consecutive days at a time.
// A move (the Night cover) is never cut: it is where they work, not time off. The file's own absences go in after every assignment, so none of them is cut.
func cutLeaveUnderAssignment(ctx context.Context, tx pgx.Tx, req domain.ApplyScheduleRangeRequest,
	teams []string, scopes map[string]string, actorEmail string) (int, error) {
	from, ferr := time.Parse(time.DateOnly, req.From)
	to, terr := time.Parse(time.DateOnly, req.To)
	if ferr != nil || terr != nil || req.ShiftCode == "" {
		// applyRangeTx refuses it in its own words.
		return 0, nil
	}
	scope, ok := scopes[req.ShiftCode]
	if !ok {
		err := tx.QueryRow(ctx, `SELECT day_scope::text FROM team_schedule_shift WHERE code = $1`, req.ShiftCode).Scan(&scope)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		if err != nil {
			return 0, fmt.Errorf("read the window's day scope: %w", err)
		}
		scopes[req.ShiftCode] = scope
	}

	cut := 0
	var runFrom *time.Time
	flush := func(end time.Time) error {
		if runFrom == nil {
			return nil
		}
		removed, trimmed, err := cutAbsencesTx(ctx, tx, req.UserID, req.TeamKey, *runFrom, end, actorEmail, ptrTo(rotaImportReplaceNote), teams)
		cut, runFrom = cut+removed+trimmed, nil
		return err
	}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		weekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
		if (scope == "WEEKDAY" && weekend) || (scope == "WEEKEND" && !weekend) {
			if err := flush(d.AddDate(0, 0, -1)); err != nil {
				return cut, err
			}
			continue
		}
		if runFrom == nil {
			day := d
			runFrom = &day
		}
	}
	return cut, flush(to)
}

// ptrTo is a pointer to v.
func ptrTo(v string) *string { return &v }
