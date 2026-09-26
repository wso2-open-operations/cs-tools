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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ScheduleRepository reads the Team Schedule tables. Every method is a plain
// data operation; what a day view does with the rows belongs one layer up.
type ScheduleRepository interface {
	Catalogue(ctx context.Context) (domain.ScheduleCatalogue, error)
	SearchAssignments(ctx context.Context, req domain.SearchScheduleAssignmentsRequest) ([]domain.ScheduleAssignment, error)
	SearchAbsences(ctx context.Context, req domain.SearchScheduleAbsencesRequest) ([]domain.ScheduleAbsence, error)
	OnDutyAt(ctx context.Context, at time.Time) ([]domain.ScheduleAssignment, error)
}

type scheduleRepository struct{ db *pgxpool.Pool }

// NewScheduleRepository constructs a ScheduleRepository over the given pool.
func NewScheduleRepository(db *pgxpool.Pool) ScheduleRepository {
	return &scheduleRepository{db: db}
}

// assignmentColumns is shared by every assignment read so the row scan below
// stays in one place -- three queries returning differently-shaped rows for
// the same struct is how scan bugs get in.
const assignmentColumns = `
    a.id, u.id, COALESCE(u.name, ''), COALESCE(u.email, ''),
    -- Lead-ness is looked up rather than joined, because schedule_assignment.team_id
    -- is nullable and routinely absent for a registry-only team. A LEFT JOIN on it
    -- silently returned FALSE for a real lead -- no error, just a missing badge.
    -- With no team on the row, any lead membership the engineer holds counts;
    -- with one, only that team's. bool_or keeps it a single row either way.
    COALESCE((SELECT bool_or(tm2.role = 'lead') FROM team_member tm2
               WHERE tm2.user_id = a.user_id
                 AND (a.team_id IS NULL OR tm2.team_id = a.team_id)), FALSE),
    a.team_key, s.code, z.code, a.tier::text, a.rota_date,
    a.starts_at, a.ends_at, a.is_on_call, a.source::text, a.note`

const assignmentFrom = `
  FROM schedule_assignment a
  JOIN "user" u          ON u.id = a.user_id
  JOIN schedule_shift s  ON s.id = a.shift_id
  LEFT JOIN schedule_zone z ON z.id = a.zone_id`

func scanAssignments(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]domain.ScheduleAssignment, error) {
	out := []domain.ScheduleAssignment{}
	for rows.Next() {
		var a domain.ScheduleAssignment
		var rotaDate time.Time
		if err := rows.Scan(
			&a.ID, &a.Engineer.UserID, &a.Engineer.Name, &a.Engineer.Email, &a.Engineer.IsLead,
			&a.TeamKey, &a.ShiftCode, &a.ZoneCode, &a.Tier, &rotaDate,
			&a.StartsAt, &a.EndsAt, &a.IsOnCall, &a.Source, &a.Note,
		); err != nil {
			return nil, fmt.Errorf("scan schedule assignment: %w", err)
		}
		a.RotaDate = rotaDate.Format("2006-01-02")
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schedule assignments: %w", err)
	}
	return out, nil
}

// Catalogue returns the zones, windows and absence kinds in one read. The UI
// needs all three to draw a single day, so serving them separately would only
// cost round trips.
func (r *scheduleRepository) Catalogue(ctx context.Context) (domain.ScheduleCatalogue, error) {
	cat := domain.ScheduleCatalogue{
		Zones:        []domain.ScheduleZone{},
		Shifts:       []domain.ScheduleShift{},
		AbsenceKinds: []domain.ScheduleAbsenceKind{},
	}

	zoneRows, err := r.db.Query(ctx, `
		SELECT z.id, z.code, z.label, w.code, z.sort_order
		FROM schedule_zone z
		LEFT JOIN schedule_zone w ON w.id = z.weekend_zone_id
		WHERE z.is_active ORDER BY z.sort_order`)
	if err != nil {
		return cat, fmt.Errorf("query schedule zones: %w", err)
	}
	defer zoneRows.Close()
	for zoneRows.Next() {
		var z domain.ScheduleZone
		if err := zoneRows.Scan(&z.ID, &z.Code, &z.Label, &z.WeekendZoneCode, &z.SortOrder); err != nil {
			return cat, fmt.Errorf("scan schedule zone: %w", err)
		}
		cat.Zones = append(cat.Zones, z)
	}
	if err := zoneRows.Err(); err != nil {
		return cat, fmt.Errorf("iterate schedule zones: %w", err)
	}

	shiftRows, err := r.db.Query(ctx, `
		SELECT s.id, s.code, s.short_code, s.label, s.family::text, z.code, s.tier::text,
		       s.day_scope::text, s.start_minute, s.end_minute, s.authoring_time_zone,
		       s.is_on_call, s.is_escalation, s.is_rotation, s.crosses_midnight, s.colour_token, s.sort_order
		FROM schedule_shift s
		LEFT JOIN schedule_zone z ON z.id = s.zone_id
		WHERE s.is_active ORDER BY s.family, s.sort_order`)
	if err != nil {
		return cat, fmt.Errorf("query schedule shifts: %w", err)
	}
	defer shiftRows.Close()
	for shiftRows.Next() {
		var s domain.ScheduleShift
		if err := shiftRows.Scan(&s.ID, &s.Code, &s.ShortCode, &s.Label, &s.Family, &s.ZoneCode, &s.Tier,
			&s.DayScope, &s.StartMinute, &s.EndMinute, &s.AuthoringTimeZone,
			&s.IsOnCall, &s.IsEscalation, &s.IsRotation, &s.CrossesMidnight, &s.ColourToken, &s.SortOrder); err != nil {
			return cat, fmt.Errorf("scan schedule shift: %w", err)
		}
		cat.Shifts = append(cat.Shifts, s)
	}
	if err := shiftRows.Err(); err != nil {
		return cat, fmt.Errorf("iterate schedule shifts: %w", err)
	}

	kindRows, err := r.db.Query(ctx, `
		SELECT id, code, short_code, label, bucket, colour_token, sort_order
		FROM schedule_absence_kind WHERE is_active ORDER BY sort_order`)
	if err != nil {
		return cat, fmt.Errorf("query schedule absence kinds: %w", err)
	}
	defer kindRows.Close()
	for kindRows.Next() {
		var k domain.ScheduleAbsenceKind
		if err := kindRows.Scan(&k.ID, &k.Code, &k.ShortCode, &k.Label, &k.Bucket, &k.ColourToken, &k.SortOrder); err != nil {
			return cat, fmt.Errorf("scan schedule absence kind: %w", err)
		}
		cat.AbsenceKinds = append(cat.AbsenceKinds, k)
	}
	if err := kindRows.Err(); err != nil {
		return cat, fmt.Errorf("iterate schedule absence kinds: %w", err)
	}

	return cat, nil
}

// SearchAssignments returns the rota over a date window.
//
// With IncludeOvernight, a block whose rota_date falls before From but which
// is still running into it is included too: that is the night crew a day view
// shows at the top, rostered for yesterday but working this morning. It is
// matched on the resolved instants rather than on rota_date, which is exactly
// what the window index is for.
func (r *scheduleRepository) SearchAssignments(ctx context.Context, req domain.SearchScheduleAssignmentsRequest) ([]domain.ScheduleAssignment, error) {
	args := []any{req.From, req.To}
	where := `WHERE a.rota_date BETWEEN $1::date AND $2::date`
	if req.IncludeOvernight {
		// The boundary is midnight in the shift's own office, not in the
		// database session's zone. `$1::date::timestamptz` resolves against
		// the session -- UTC in every deployment of this service -- so a
		// Colombo-authored block ending 00:30 IST (19:00Z the day before) was
		// not > the UTC midnight of the day it plainly runs into, and dropped
		// out of that day's view. Every other date boundary in this feature
		// already goes through authoring_time_zone; this one did not.
		where = `WHERE (a.rota_date BETWEEN $1::date AND $2::date
		          OR (a.rota_date < $1::date
		              AND a.ends_at > ($1::date::timestamp AT TIME ZONE s.authoring_time_zone)))`
	}
	if len(req.TeamKeys) > 0 {
		args = append(args, req.TeamKeys)
		where += fmt.Sprintf(" AND a.team_key = ANY($%d)", len(args))
	}
	if req.Family != "" {
		args = append(args, req.Family)
		where += fmt.Sprintf(" AND s.family = $%d::schedule_shift_family_enum", len(args))
	}
	if req.UserID != "" {
		args = append(args, req.UserID)
		where += fmt.Sprintf(" AND a.user_id = $%d", len(args))
	}
	if req.UserEmail != "" {
		args = append(args, req.UserEmail)
		where += fmt.Sprintf(" AND LOWER(u.email) = LOWER($%d)", len(args))
	}

	rows, err := r.db.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+" "+where+
		" ORDER BY a.rota_date, a.starts_at, s.sort_order, u.name", args...)
	if err != nil {
		return nil, fmt.Errorf("query schedule assignments: %w", err)
	}
	defer rows.Close()
	return scanAssignments(rows)
}

// OnDutyAt answers "who is responsible at this instant" -- the question an
// alert escalation asks. Matched against the resolved window as a range, so
// it is an index scan rather than a comparison over every row.
func (r *scheduleRepository) OnDutyAt(ctx context.Context, at time.Time) ([]domain.ScheduleAssignment, error) {
	// Somebody on leave is not on duty, whatever their assignment row says.
	//
	// The two facts are stored independently -- a rotation is generated weeks
	// ahead, leave is granted against it afterwards -- so the assignment
	// survives the absence and this query has to reconcile them. Without the
	// exclusion, "who do I page right now" answers with someone on annual
	// leave, which is the one thing it must never do.
	//
	// Every kind counts, not just leave. The catalogue's three buckets are
	// LEAVE, ALLOCATION and EXCLUDED, and none of them describes somebody who
	// is available: an allocation is work they are doing instead, and
	// "excluded from rota" is the plainest case of all. Filtering by bucket
	// would only reintroduce the bug for whichever bucket was left out.
	//
	// The span is closed at both ends because a leave day is a whole day, and
	// it is compared against the date in the shift's own authoring zone: leave
	// is granted as a calendar day by someone in that office, not as an
	// instant.
	rows, err := r.db.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
		WHERE tstzrange(a.starts_at, a.ends_at, '[)') @> $1::timestamptz
		  AND NOT EXISTS (
		        SELECT 1 FROM schedule_absence ab
		        WHERE ab.user_id = a.user_id
		          AND daterange(ab.starts_on, ab.ends_on, '[]')
		              @> ($1::timestamptz AT TIME ZONE s.authoring_time_zone)::date
		      )
		ORDER BY a.tier NULLS LAST, s.sort_order, u.name`, at)
	if err != nil {
		return nil, fmt.Errorf("query on-duty assignments: %w", err)
	}
	defer rows.Close()
	return scanAssignments(rows)
}

// SearchAbsences returns every absence overlapping the window. An absence
// with no end date is open-ended and overlaps any window that starts after it.
func (r *scheduleRepository) SearchAbsences(ctx context.Context, req domain.SearchScheduleAbsencesRequest) ([]domain.ScheduleAbsence, error) {
	args := []any{req.From, req.To}
	where := `WHERE daterange(ab.starts_on, ab.ends_on, '[]') && daterange($1::date, $2::date, '[]')`
	if len(req.TeamKeys) > 0 {
		args = append(args, req.TeamKeys)
		where += fmt.Sprintf(" AND ab.team_key = ANY($%d)", len(args))
	}
	if req.UserID != "" {
		args = append(args, req.UserID)
		where += fmt.Sprintf(" AND ab.user_id = $%d", len(args))
	}
	if req.UserEmail != "" {
		args = append(args, req.UserEmail)
		where += fmt.Sprintf(" AND LOWER(u.email) = LOWER($%d)", len(args))
	}

	rows, err := r.db.Query(ctx, `
		SELECT ab.id, u.id, COALESCE(u.name, ''), COALESCE(u.email, ''), FALSE,
		       ab.team_key, k.code, ab.starts_on, ab.ends_on, ab.note
		FROM schedule_absence ab
		JOIN "user" u ON u.id = ab.user_id
		JOIN schedule_absence_kind k ON k.id = ab.kind_id
		`+where+` ORDER BY ab.starts_on, u.name`, args...)
	if err != nil {
		return nil, fmt.Errorf("query schedule absences: %w", err)
	}
	defer rows.Close()

	out := []domain.ScheduleAbsence{}
	for rows.Next() {
		var ab domain.ScheduleAbsence
		var startsOn time.Time
		var endsOn *time.Time
		if err := rows.Scan(&ab.ID, &ab.Engineer.UserID, &ab.Engineer.Name, &ab.Engineer.Email,
			&ab.Engineer.IsLead, &ab.TeamKey, &ab.KindCode, &startsOn, &endsOn, &ab.Note); err != nil {
			return nil, fmt.Errorf("scan schedule absence: %w", err)
		}
		ab.StartsOn = startsOn.Format("2006-01-02")
		if endsOn != nil {
			s := endsOn.Format("2006-01-02")
			ab.EndsOn = &s
		}
		out = append(out, ab)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schedule absences: %w", err)
	}
	return out, nil
}
