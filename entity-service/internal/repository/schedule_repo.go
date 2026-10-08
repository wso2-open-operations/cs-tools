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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ScheduleRepository reads the Team Schedule tables. Every method is a plain
// data operation; what a day view does with the rows belongs one layer up.
type ScheduleRepository interface {
	Catalogue(ctx context.Context) (domain.ScheduleCatalogue, error)
	SearchAssignments(ctx context.Context, req domain.SearchScheduleAssignmentsRequest) ([]domain.ScheduleAssignment, error)
	SearchAbsences(ctx context.Context, req domain.SearchScheduleAbsencesRequest) ([]domain.ScheduleAbsence, error)
	OnDutyAt(ctx context.Context, at time.Time) ([]domain.ScheduleAssignment, error)

	// AssignmentByID is what the service checks before it lets a lead touch a
	// row: which team the slot belongs to, so the lead's own team can be
	// compared against it.
	AssignmentByID(ctx context.Context, id string) (domain.ScheduleAssignment, error)

	// LeadsTeam reports whether this user leads this team. The whole of the
	// edit permission rests on it.
	LeadsTeam(ctx context.Context, userEmail, teamKey string) (bool, error)

	// UserInTeam reports whether this user is on this team at all, by id
	// rather than by email. Leading a team says what a lead may change; this
	// says whose rota is theirs to change it on.
	UserInTeam(ctx context.Context, userID, teamKey string) (bool, error)
	// MovedToTeamOver reports whether the person spends all of [from, to] on a
	// span filed under teamKey by a kind that moves people to it, and their
	// home team if so.
	MovedToTeamOver(ctx context.Context, userID, teamKey, from, to string) (homeTeamKey string, ok bool, err error)

	// The three writes. Each records its own activity row inside the same
	// transaction as the change -- an activity row without its change, or a
	// change without its row, is worse than either alone.
	CreateAssignment(ctx context.Context, req domain.CreateScheduleAssignmentRequest, actorEmail string) (domain.ScheduleAssignment, error)
	UpdateAssignment(ctx context.Context, id string, req domain.UpdateScheduleAssignmentRequest, actorEmail string) (domain.ScheduleAssignment, error)
	DeleteAssignment(ctx context.Context, id, actorEmail string, note *string) error

	// ActivityForTeam is "what changed on my team this week".
	ActivityForTeam(ctx context.Context, teamKey, from, to string) ([]domain.ScheduleAssignmentActivity, error)

	// LeadTeamsFor is every team this caller leads. The UI needs it to know
	// which rows to offer an edit control on; without it the page would have
	// to show the control to everyone and let the 403 explain.
	LeadTeamsFor(ctx context.Context, userEmail string) ([]string, error)

	// AbsenceByID reads one absence, so the service can check who may remove
	// it before anything is touched.
	AbsenceByID(ctx context.Context, id string) (domain.ScheduleAbsence, error)
	// DeleteAbsence removes one absence outright, open-ended ones included,
	// recording it in the absence history inside the same transaction.
	DeleteAbsence(ctx context.Context, id, actorEmail string, note *string) error
	// DeleteAbsenceKind removes a kind a lead added. The catalogue's own kinds
	// are refused, and so is one still in use.
	DeleteAbsenceKind(ctx context.Context, code, actorEmail string) error
	// CreateAbsenceKind adds a kind to the shared catalogue under the given
	// code. A code already taken is a ConflictError.
	CreateAbsenceKind(ctx context.Context, code string, req domain.CreateScheduleAbsenceKindRequest, actorEmail string) (domain.ScheduleAbsenceKind, error)
	// RotaAdminTeamsFor is every team this caller may edit by virtue of
	// holding a rota admin role, rather than by leading the team.
	RotaAdminTeamsFor(ctx context.Context, userEmail string) ([]string, error)

	// ApplyRange sets one engineer to one window across a span of days, which
	// is how the roster's picker edits.
	ApplyRange(ctx context.Context, req domain.ApplyScheduleRangeRequest, actorEmail string) (domain.ApplyScheduleRangeResponse, error)
	// EditMarkers is which cells in a window a person has changed, for the
	// roster to mark. One row per cell, not the changes themselves.
	EditMarkers(ctx context.Context, from, to string) ([]domain.ScheduleEditMarker, error)

	// ApplyAbsence marks one engineer away across a span, or clears it.
	ApplyAbsence(ctx context.Context, req domain.ApplyScheduleAbsenceRequest, actorEmail string) (domain.ApplyScheduleAbsenceResponse, error)
}

type scheduleRepository struct{ db *pgxpool.Pool }

// nameTheActor tells the database who is making this change, for the audit
// triggers (migration 0155) to record.
//
// The triggers can usually read it off the row's own updated_by, but not on a
// DELETE: there the row can only offer whoever last wrote it, which is not the
// person removing it. Set for the transaction, so it covers every statement in
// the change and is gone again afterwards.
func nameTheActor(ctx context.Context, tx pgx.Tx, actorEmail string) error {
	if actorEmail == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.actor', $1, true)`, actorEmail); err != nil {
		return fmt.Errorf("name the actor for the audit trail: %w", err)
	}
	return nil
}

// NewScheduleRepository constructs a ScheduleRepository over the given pool.
func NewScheduleRepository(db *pgxpool.Pool) ScheduleRepository {
	return &scheduleRepository{db: db}
}

// The engineer's name is the portal's usual display rule: the display name,
// else first + last, else the user name. Reading "user".name alone showed the
// rota sheet's nickname ("JaneD") for someone whose record has a real first
// and last name, and nothing at all for a synced row whose name is NULL.
const engineerName = `COALESCE(NULLIF(u.name, ''), NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), u.user_name, '')`

// assignmentColumns is shared by every assignment read so the row scan below
// stays in one place -- three queries returning differently-shaped rows for
// the same struct is how scan bugs get in.
const assignmentColumns = `
    a.id, u.id, ` + engineerName + `, COALESCE(u.email, ''),
    -- Lead-ness is looked up rather than joined, because team_schedule_assignment.team_id
    -- is nullable and routinely absent for a registry-only team. A LEFT JOIN on it
    -- silently returned FALSE for a real lead -- no error, just a missing badge.
    -- With no team on the row, any lead membership the engineer holds counts;
    -- with one, only that team's. bool_or keeps it a single row either way.
    COALESCE((SELECT bool_or(tm2.role IN ('lead', 'americas_team_lead')) FROM team_member tm2
               WHERE tm2.user_id = a.user_id
                 AND (a.team_id IS NULL OR tm2.team_id = a.team_id)), FALSE),
    a.team_key, s.code, z.code, a.tier::text, a.rota_date,
    a.starts_at, a.ends_at, a.is_on_call, a.source::text, a.note`

const assignmentFrom = `
  FROM team_schedule_assignment a
  JOIN "user" u          ON u.id = a.user_id
  JOIN team_schedule_shift s  ON s.id = a.shift_id
  LEFT JOIN team_schedule_zone z ON z.id = a.zone_id`

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

// Which teams the rota is run for, and which family each belongs to, both
// derived from team.type -- the registry spells it CRE-ABT / SRE-ABT / CRE, so
// the leading word is the group and an ABT is a team within it.
//
// Constants because two queries need them: the catalogue the roster colours
// itself by, and the rota admin lookup that decides whose rota somebody may
// edit. A second, drifted copy of the family test is precisely how an SRE
// admin would quietly gain a CRE team, and it would not look like a bug in
// either query on its own.
const (
	teamFamilyExpr = `CASE WHEN lower(t.type) LIKE 'sre%' THEN 'SRE' WHEN lower(t.type) LIKE 'sme%' THEN 'SME' ELSE 'CRE' END`
	// A leadership team (CRE Leadership) is management above the rota teams,
	// not a team that works one, so it is no team of the schedule's.
	//
	// And only a team that works a rota. A CRE-typed team is not one by its
	// type alone: the directory sync also brings in the change-request
	// approval boards (CAB, Devops approval and review) as teams, and once the
	// roster listed every member of every team, their members turned up on it
	// under those boards. A team counts when it is an ABT, its type has a rota
	// (the SRE and SME rotas), it holds rota history, a tag moves people to it
	// (Migration), or it has an ordinary-weekday window (Americas) -- the last
	// being how a new team is put on the rota before its first entry.
	rosteredTeamWhere = `t.type IS NOT NULL AND lower(t.type) LIKE ANY (ARRAY['cre%', 'sre%', 'sme%'])
		AND lower(t.type) NOT LIKE '%leadership%' AND lower(COALESCE(t.name, '')) NOT LIKE '%leadership%'
		AND (lower(t.type) LIKE '%-abt'
		     OR EXISTS (SELECT 1 FROM team_schedule_rota rr WHERE lower(rr.team_type) = lower(t.type) AND rr.is_active)
		     OR EXISTS (SELECT 1 FROM team_schedule_team_default_shift dd WHERE dd.team_key = t.key)
		     OR EXISTS (SELECT 1 FROM team_schedule_absence_kind kk WHERE lower(kk.moves_to_team_key) = lower(t.key))
		     OR EXISTS (SELECT 1 FROM team_schedule_assignment aa WHERE aa.team_key = t.key)
		     OR EXISTS (SELECT 1 FROM team_schedule_absence bb WHERE bb.team_key = t.key))`

	// notManagement leaves out anyone holding a management role -- the
	// Americas team's lead, the CRE and CS heads. They sit above the teams
	// rather than working a rota on one, so a team's view of its rota does not
	// list them. Applied to team reads only: somebody reading their own rota
	// by id or email still sees all of it, and the ladder's on-duty read is
	// not filtered at all.
	notManagement = `NOT EXISTS (SELECT 1 FROM team_member mgr
		WHERE mgr.user_id = %s AND mgr.role IN ('americas_team_lead', 'cre_head', 'cs_head'))`
	teamDisplayOrder = `(lower(t.type) LIKE '%abt') DESC, t.name`
)

// Catalogue returns the zones, windows and absence kinds in one read. The UI
// needs all three to draw a single day, so serving them separately would only
// cost round trips.
func (r *scheduleRepository) Catalogue(ctx context.Context) (domain.ScheduleCatalogue, error) {
	cat := domain.ScheduleCatalogue{
		Zones:        []domain.ScheduleZone{},
		Shifts:       []domain.ScheduleShift{},
		AbsenceKinds: []domain.ScheduleAbsenceKind{},
		Teams:        []domain.ScheduleTeam{},
		Rotas:        []domain.ScheduleRota{},
	}

	// The teams the rota is run for. type carries the family the registry
	// spells CRE-ABT / SRE-ABT / CRE, so the leading word is the group and an
	// ABT is a team within it.
	//
	// The ABTs come first, then the teams that hold no ABT rotation. Ordering
	// by name alone put Americas above Atlas and Migration in among the ABTs,
	// which is backwards for a reader scanning the roster for their own team:
	// the ABTs are the rota, and the others are the exceptions to it. Name
	// still orders within each group, so the position stays stable enough for
	// a client to colour by -- the same window function and ORDER BY, so a
	// team's sortOrder always matches where it actually appears.
	teamRows, err := r.db.Query(ctx, `
		SELECT t.key, t.name,
		       (SELECT d.shift_code FROM team_schedule_team_default_shift d WHERE d.team_key = t.key),
		       `+teamFamilyExpr+`,
		       (row_number() OVER (ORDER BY `+teamDisplayOrder+`))::int,
		       ro.code
		  FROM team t
		  LEFT JOIN team_schedule_rota ro ON lower(ro.team_type) = lower(t.type) AND ro.is_active
		 WHERE `+rosteredTeamWhere+`
		 ORDER BY `+teamDisplayOrder)
	if err != nil {
		return cat, fmt.Errorf("query schedule teams: %w", err)
	}
	defer teamRows.Close()
	for teamRows.Next() {
		var t domain.ScheduleTeam
		var key *string
		if err := teamRows.Scan(&key, &t.Name, &t.DefaultShiftCode, &t.Family, &t.SortOrder, &t.RotaCode); err != nil {
			return cat, fmt.Errorf("scan schedule team: %w", err)
		}
		t.Key = stringOrEmpty(key)
		cat.Teams = append(cat.Teams, t)
	}
	if err := teamRows.Err(); err != nil {
		return cat, fmt.Errorf("query schedule teams: %w", err)
	}
	teamRows.Close()

	// Each team's members, with their role. Reads team_member.role and
	// nothing newer, which every deployed schema has, so a database without
	// the later team_member columns still serves the catalogue.
	byKey := make(map[string]int, len(cat.Teams))
	for i := range cat.Teams {
		cat.Teams[i].Members = []domain.ScheduleTeamMember{}
		byKey[cat.Teams[i].Key] = i
	}
	memberRows, err := r.db.Query(ctx, `
		SELECT t.key, u.id::text, `+engineerName+`, COALESCE(u.email, ''), tm.role
		  FROM team_member tm
		  JOIN team t ON t.id = tm.team_id
		  JOIN "user" u ON u.id = tm.user_id
		 WHERE t.key IS NOT NULL
		   AND `+fmt.Sprintf(notManagement, "tm.user_id")+`
		 ORDER BY t.key, 3`)
	if err != nil {
		return cat, fmt.Errorf("query schedule team members: %w", err)
	}
	defer memberRows.Close()
	for memberRows.Next() {
		var key string
		var m domain.ScheduleTeamMember
		if err := memberRows.Scan(&key, &m.UserID, &m.Name, &m.Email, &m.Role); err != nil {
			return cat, fmt.Errorf("scan schedule team member: %w", err)
		}
		m.IsLead = m.Role == "lead" || m.Role == "americas_team_lead"
		if i, ok := byKey[key]; ok {
			cat.Teams[i].Members = append(cat.Teams[i].Members, m)
		}
	}
	if err := memberRows.Err(); err != nil {
		return cat, fmt.Errorf("query schedule team members: %w", err)
	}

	zoneRows, err := r.db.Query(ctx, `
		SELECT z.id, z.code, z.label, w.code, z.sort_order, ro.code
		FROM team_schedule_zone z
		LEFT JOIN team_schedule_zone w ON w.id = z.weekend_zone_id
		LEFT JOIN team_schedule_rota ro ON ro.id = z.rota_id
		WHERE z.is_active ORDER BY z.sort_order`)
	if err != nil {
		return cat, fmt.Errorf("query schedule zones: %w", err)
	}
	defer zoneRows.Close()
	for zoneRows.Next() {
		var z domain.ScheduleZone
		if err := zoneRows.Scan(&z.ID, &z.Code, &z.Label, &z.WeekendZoneCode, &z.SortOrder, &z.RotaCode); err != nil {
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
		FROM team_schedule_shift s
		LEFT JOIN team_schedule_zone z ON z.id = s.zone_id
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

	// The named rotations inside each family (SRE's SaaS and IaaS, SME's
	// products), with the rules their sheets state.
	rotaRows, err := r.db.Query(ctx, `
		SELECT code, label, family::text, rotates, escalation_minutes, source_sheet, sort_order
		FROM team_schedule_rota WHERE is_active ORDER BY family, sort_order, code`)
	if err != nil {
		return cat, fmt.Errorf("query schedule rotas: %w", err)
	}
	defer rotaRows.Close()
	for rotaRows.Next() {
		var ro domain.ScheduleRota
		if err := rotaRows.Scan(&ro.Code, &ro.Label, &ro.Family, &ro.Rotates, &ro.EscalationMinutes, &ro.SourceSheet, &ro.SortOrder); err != nil {
			return cat, fmt.Errorf("scan schedule rota: %w", err)
		}
		cat.Rotas = append(cat.Rotas, ro)
	}
	if err := rotaRows.Err(); err != nil {
		return cat, fmt.Errorf("iterate schedule rotas: %w", err)
	}

	kindRows, err := r.db.Query(ctx, `
		SELECT id, code, short_code, label, bucket, colour_token, sort_order,
		       created_by IS DISTINCT FROM 'migration', family::text, NOT is_active,
		       moves_to_team_key, works_rota_there, shows_as_shift_code
		FROM team_schedule_absence_kind ORDER BY sort_order`)
	if err != nil {
		return cat, fmt.Errorf("query schedule absence kinds: %w", err)
	}
	defer kindRows.Close()
	for kindRows.Next() {
		var k domain.ScheduleAbsenceKind
		if err := kindRows.Scan(&k.ID, &k.Code, &k.ShortCode, &k.Label, &k.Bucket, &k.ColourToken, &k.SortOrder, &k.Custom, &k.Family, &k.Retired, &k.MovesToTeamKey, &k.WorksRotaThere, &k.ShowsAsShiftCode); err != nil {
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
		//
		// Only the day before can run into $1: a window starts before
		// midnight of its own rota day and lasts at most 24 hours
		// (team_schedule_shift_end_minute_check), and every assignment's
		// instants are resolved from its shift, which cannot change once used.
		// Naming that one day, rather than every earlier one, keeps this on
		// the rota_date index instead of scanning the whole history.
		where = `WHERE (a.rota_date BETWEEN $1::date AND $2::date
		          OR (a.rota_date = $1::date - 1
		              AND a.ends_at > ($1::date::timestamp AT TIME ZONE s.authoring_time_zone)))`
	}
	if len(req.TeamKeys) > 0 {
		args = append(args, req.TeamKeys)
		where += fmt.Sprintf(" AND a.team_key = ANY($%d)", len(args))
	}
	if req.UserID == "" && req.UserEmail == "" {
		where += " AND " + fmt.Sprintf(notManagement, "a.user_id")
	}
	if req.Family != "" {
		args = append(args, req.Family)
		where += fmt.Sprintf(" AND s.family = $%d::team_schedule_shift_family_enum", len(args))
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
		        SELECT 1 FROM team_schedule_absence ab
		        JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
		        WHERE ab.user_id = a.user_id
		          AND daterange(ab.starts_on, ab.ends_on, '[]')
		              @> ($1::timestamptz AT TIME ZONE s.authoring_time_zone)::date
		          -- Away from every team but the one the span moved them to:
		          -- someone on the Brazil rotation is working the Americas
		          -- rota, and is on duty for its shifts.
		          AND NOT (k.moves_to_team_key IS NOT NULL
		                   AND lower(k.moves_to_team_key) = lower(a.team_key))
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
		// A span a kind moved somebody away on is read with their own team's
		// too: a lead looking at their team should still find the engineer
		// who is on the Brazil rotation, under the team they went to.
		where += fmt.Sprintf(" AND (ab.team_key = ANY($%d) OR ab.home_team_key = ANY($%d))", len(args), len(args))
	}
	if req.UserID == "" && req.UserEmail == "" {
		where += " AND " + fmt.Sprintf(notManagement, "ab.user_id")
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
		SELECT ab.id, u.id, `+engineerName+`, COALESCE(u.email, ''), FALSE,
		       ab.team_key, k.code, ab.starts_on, ab.ends_on, ab.note, ab.allocated_to, ab.home_team_key
		FROM team_schedule_absence ab
		JOIN "user" u ON u.id = ab.user_id
		JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
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
			&ab.Engineer.IsLead, &ab.TeamKey, &ab.KindCode, &startsOn, &endsOn, &ab.Note, &ab.AllocatedTo, &ab.HomeTeamKey); err != nil {
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

// ── lead edit ───────────────────────────────────────────────────────────────

// AssignmentByID returns one assignment, or a NotFoundError.
func (r *scheduleRepository) AssignmentByID(ctx context.Context, id string) (domain.ScheduleAssignment, error) {
	rows, err := r.db.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
		WHERE a.id = $1::uuid`, id)
	if err != nil {
		return domain.ScheduleAssignment{}, fmt.Errorf("query assignment by id: %w", err)
	}
	defer rows.Close()
	out, err := scanAssignments(rows)
	if err != nil {
		return domain.ScheduleAssignment{}, err
	}
	if len(out) == 0 {
		return domain.ScheduleAssignment{}, &apierror.NotFoundError{Msg: "no such assignment"}
	}
	return out[0], nil
}

// LeadsTeam reports whether the caller leads the given team.
//
// Matched on email rather than id because that is what a verified identity
// carries, and lowercased on both sides: an identity provider is free to return
// a different case from the one stored, and an exact compare would silently
// deny a real lead.
func (r *scheduleRepository) LeadsTeam(ctx context.Context, userEmail, teamKey string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		    FROM team_member tm
		    JOIN "user" u ON u.id = tm.user_id
		    JOIN team t    ON t.id = tm.team_id
		   WHERE lower(u.email) = lower($1)
		     AND tm.role IN ('lead', 'americas_team_lead')
		     AND t.key = lower($2)
		)`, userEmail, teamKey).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check team lead: %w", err)
	}
	return ok, nil
}

// UserInTeam implements ScheduleRepository.
func (r *scheduleRepository) UserInTeam(ctx context.Context, userID, teamKey string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM team_member tm
			  JOIN team t ON t.id = tm.team_id
			 WHERE tm.user_id = $1::uuid
			   AND t.key = lower($2))`, userID, teamKey).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check team membership: %w", err)
	}
	return ok, nil
}

// slotConflict turns the database refusing a second window for one person
// into a ConflictError the lead can act on, or returns nil for any other error.
//
// A person cannot be on two overlapping windows, whatever team each is for
// (team_schedule_assignment_no_overlap / _no_overlap_regular), nor twice on one
// window on one day (team_schedule_assignment_unique_slot). Replacing a day
// clears only the lead's own team's rows, so a window this person already
// holds on another team's rota is still there, and the insert collides with
// it. Unhandled, that surfaced as a 500 and the picker could only say that
// nothing had saved.
func slotConflict(err error, isoDate string) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	if pgErr.Code != "23P01" && !(pgErr.Code == "23505" && pgErr.ConstraintName == "team_schedule_assignment_unique_slot") {
		return nil
	}
	on := ""
	if isoDate != "" {
		on = " on " + isoDate
	}
	return &apierror.ConflictError{
		Msg: "this person already has a window that overlaps this one" + on +
			", possibly on another team's rota; clear that one first",
	}
}

// syncMoveShifts keeps a person's MOVE shifts in step with their spans over
// [from, to]: the shifts a span of a moving tag writes -- the Brazil rotation
// is worked as Americas cover, so each weekday of the span holds a real
// Americas cover shift on the Americas team. Those rows are what the day and
// week views and the escalation ladder read; a span that only drew them on the
// roster left the person missing from all three.
//
// Called inside the transaction that changed the spans, over the dates it
// touched. A MOVE shift no span covers any more (cut short, removed, or
// replaced by leave) is deleted; a day a span covers that has none is given
// one. A day already holding an overlapping shift is left alone -- ON CONFLICT
// DO NOTHING covers the no-overlap exclusion -- and shifts a lead placed by
// hand are never touched: only source MOVE is.
func syncMoveShifts(ctx context.Context, tx pgx.Tx, userID, from, to, actorEmail string) error {
	if _, err := tx.Exec(ctx, `
		DELETE FROM team_schedule_assignment a
		 WHERE a.user_id = $1::uuid
		   AND a.source = 'MOVE'
		   AND a.rota_date BETWEEN $2::date AND $3::date
		   AND NOT EXISTS (
		         SELECT 1 FROM team_schedule_absence ab
		           JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
		           JOIN team_schedule_shift s ON s.code = k.shows_as_shift_code
		          WHERE ab.user_id = a.user_id
		            AND k.works_rota_there
		            AND lower(ab.team_key) = lower(a.team_key)
		            AND s.id = a.shift_id
		            AND daterange(ab.starts_on, ab.ends_on, '[]') @> a.rota_date)`,
		userID, from, to); err != nil {
		return fmt.Errorf("remove move shifts: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO team_schedule_assignment
		  (id, created_on, updated_on, created_by, updated_by, user_id, team_id, team_key,
		   shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, note)
		SELECT gen_random_uuid(), now(), now(), $4, $4, ab.user_id,
		       (SELECT t.id FROM team t WHERE t.key = lower(ab.team_key)), ab.team_key,
		       s.id, s.zone_id, s.tier, d::date,
		       (d::date::timestamp + make_interval(mins => s.start_minute)) AT TIME ZONE s.authoring_time_zone,
		       (d::date::timestamp + make_interval(mins => s.end_minute))   AT TIME ZONE s.authoring_time_zone,
		       s.is_on_call, 'MOVE', NULL
		  FROM team_schedule_absence ab
		  JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
		  JOIN team_schedule_shift s ON s.code = k.shows_as_shift_code
		 CROSS JOIN LATERAL generate_series(
		        GREATEST(ab.starts_on, $2::date)::timestamp,
		        LEAST(COALESCE(ab.ends_on, $3::date), $3::date)::timestamp,
		        INTERVAL '1 day') AS d
		 WHERE ab.user_id = $1::uuid
		   AND k.works_rota_there
		   AND k.moves_to_team_key IS NOT NULL
		   AND lower(ab.team_key) = lower(k.moves_to_team_key)
		   AND (s.day_scope = 'ANY'
		        OR (s.day_scope = 'WEEKDAY' AND extract(isodow FROM d) < 6)
		        OR (s.day_scope = 'WEEKEND' AND extract(isodow FROM d) >= 6))
		ON CONFLICT DO NOTHING`,
		userID, from, to, actorEmail); err != nil {
		return fmt.Errorf("write move shifts: %w", err)
	}
	return nil
}

// MovedToTeamOver reports whether the person spends the whole of [from, to]
// on a span that moved them to teamKey -- the Brazil rotation, filed under the
// Americas team -- and if so which team they belong to. That is what lets the
// Americas lead roster somebody who is not an Americas member, and the home
// team's lead keep charge of the span itself.
func (r *scheduleRepository) MovedToTeamOver(ctx context.Context, userID, teamKey, from, to string) (string, bool, error) {
	var home *string
	err := r.db.QueryRow(ctx, `
		SELECT ab.home_team_key
		  FROM team_schedule_absence ab
		  JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
		 WHERE ab.user_id = $1::uuid
		   AND lower(ab.team_key) = lower($2)
		   AND k.moves_to_team_key IS NOT NULL
		   AND lower(k.moves_to_team_key) = lower($2)
		   AND daterange(ab.starts_on, ab.ends_on, '[]') @> daterange(LEAST($3::date, $4::date), GREATEST($3::date, $4::date), '[]')
		 LIMIT 1`, userID, teamKey, from, to).Scan(&home)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("check moved to team: %w", err)
	}
	return stringOrEmpty(home), true, nil
}

// recordActivity writes one row of history. Always called on the same tx as the
// change it describes.
func recordActivity(ctx context.Context, tx pgx.Tx, a domain.ScheduleAssignment,
	action, actorEmail string, field, oldV, newV, note *string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO team_schedule_assignment_activity
		  (id, created_on, created_by, assignment_id, user_id, team_key, rota_date,
		   shift_code, action, field_name, old_value, new_value, actor_email, note)
		VALUES (gen_random_uuid(), now(), $1, $2::uuid, $3::uuid, $4, $5::date, $6, $7, $8, $9, $10, $1, $11)`,
		actorEmail, a.ID, a.Engineer.UserID, a.TeamKey, a.RotaDate, a.ShiftCode,
		action, field, oldV, newV, note)
	if err != nil {
		return fmt.Errorf("record schedule activity: %w", err)
	}
	return nil
}

// CreateAssignment puts somebody on a window.
//
// The instants come from the shift, never from the caller: a hand-placed cover
// that claimed its own start and end could drift from the window it is supposed
// to be, and nothing downstream would notice.
func (r *scheduleRepository) CreateAssignment(ctx context.Context, req domain.CreateScheduleAssignmentRequest, actorEmail string) (domain.ScheduleAssignment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.ScheduleAssignment{}, fmt.Errorf("begin create assignment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return domain.ScheduleAssignment{}, err
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO team_schedule_assignment
		  (id, created_on, updated_on, created_by, updated_by, user_id, team_id, team_key,
		   shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, note)
		SELECT gen_random_uuid(), now(), now(), $1, $1, $2::uuid,
		       (SELECT t.id FROM team t WHERE t.key = lower($3)), $3,
		       s.id, s.zone_id, $4::team_schedule_tier_enum, $5::date,
		       ($5::date::timestamp + make_interval(mins => s.start_minute)) AT TIME ZONE s.authoring_time_zone,
		       ($5::date::timestamp + make_interval(mins => s.end_minute))   AT TIME ZONE s.authoring_time_zone,
		       COALESCE($6, s.is_on_call), 'MANUAL', $7
		  FROM team_schedule_shift s
		 WHERE s.code = $8
		RETURNING id`,
		actorEmail, req.UserID, req.TeamKey, req.Tier, req.RotaDate, req.IsOnCall, req.Note, req.ShiftCode).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// The INSERT selects from team_schedule_shift, so a code that is not in the
		// catalogue returns no rows rather than failing. Reported as the bad
		// request it is instead of a 500.
		return domain.ScheduleAssignment{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("no such shift %q", req.ShiftCode),
		}
	}
	if err != nil {
		if c := slotConflict(err, req.RotaDate); c != nil {
			return domain.ScheduleAssignment{}, c
		}
		return domain.ScheduleAssignment{}, fmt.Errorf("insert assignment: %w", err)
	}

	created, err := assignmentByIDTx(ctx, tx, id)
	if err != nil {
		return domain.ScheduleAssignment{}, err
	}
	if err := recordActivity(ctx, tx, created, "CREATED", actorEmail, nil, nil, nil, req.Note); err != nil {
		return domain.ScheduleAssignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ScheduleAssignment{}, fmt.Errorf("commit create assignment: %w", err)
	}
	return created, nil
}

// assignmentByIDTx is AssignmentByID against an open transaction, so a write can
// read back what it just wrote without leaving the transaction.
func assignmentByIDTx(ctx context.Context, tx pgx.Tx, id string) (domain.ScheduleAssignment, error) {
	rows, err := tx.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
		WHERE a.id = $1::uuid`, id)
	if err != nil {
		return domain.ScheduleAssignment{}, fmt.Errorf("query assignment in tx: %w", err)
	}
	defer rows.Close()
	out, err := scanAssignments(rows)
	if err != nil {
		return domain.ScheduleAssignment{}, err
	}
	if len(out) == 0 {
		return domain.ScheduleAssignment{}, &apierror.NotFoundError{Msg: "no such assignment"}
	}
	return out[0], nil
}

// UpdateAssignment changes who holds a slot, or its detail.
//
// One activity row per field changed, rather than one per call: "moved from
// Alice to Bob" and "marked on-call" are two different things to have done, and
// a reader of the history wants them separately.
func (r *scheduleRepository) UpdateAssignment(ctx context.Context, id string, req domain.UpdateScheduleAssignmentRequest, actorEmail string) (domain.ScheduleAssignment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.ScheduleAssignment{}, fmt.Errorf("begin update assignment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return domain.ScheduleAssignment{}, err
	}

	before, err := assignmentByIDTx(ctx, tx, id)
	if err != nil {
		return domain.ScheduleAssignment{}, err
	}

	// SWAP rather than MANUAL when the person changed: that is what the enum
	// distinguishes, and it is the case anyone auditing the rota looks for.
	source := "MANUAL"
	if req.UserID != nil && *req.UserID != before.Engineer.UserID {
		source = "SWAP"
	}

	_, err = tx.Exec(ctx, `
		UPDATE team_schedule_assignment
		   SET user_id    = COALESCE($2::uuid, user_id),
		       tier       = COALESCE($3::team_schedule_tier_enum, tier),
		       is_on_call = COALESCE($4, is_on_call),
		       note       = COALESCE($5, note),
		       source     = $6::team_schedule_source_enum,
		       updated_on = now(),
		       updated_by = $7
		 WHERE id = $1::uuid`,
		id, req.UserID, req.Tier, req.IsOnCall, req.Note, source, actorEmail)
	if err != nil {
		if c := slotConflict(err, ""); c != nil {
			return domain.ScheduleAssignment{}, c
		}
		return domain.ScheduleAssignment{}, fmt.Errorf("update assignment: %w", err)
	}

	after, err := assignmentByIDTx(ctx, tx, id)
	if err != nil {
		return domain.ScheduleAssignment{}, err
	}

	// Each change is detected on the value that identifies it and recorded as
	// the value a reader wants to see. For the engineer those are not the
	// same thing: two people on one rota can share a display name, and
	// comparing names let a slot move between them with nothing written down
	// -- the row changed, the history did not, and the history is the only
	// reason this table exists.
	for _, ch := range []struct{ field, was, now, old, new string }{
		{"user", before.Engineer.UserID, after.Engineer.UserID, before.Engineer.Name, after.Engineer.Name},
		{"tier", deref(before.Tier), deref(after.Tier), deref(before.Tier), deref(after.Tier)},
		{"isOnCall", boolText(before.IsOnCall), boolText(after.IsOnCall), boolText(before.IsOnCall), boolText(after.IsOnCall)},
		{"note", deref(before.Note), deref(after.Note), deref(before.Note), deref(after.Note)},
	} {
		if ch.was == ch.now {
			continue
		}
		f, o, n := ch.field, ch.old, ch.new
		if err := recordActivity(ctx, tx, after, "UPDATED", actorEmail, &f, &o, &n, nil); err != nil {
			return domain.ScheduleAssignment{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ScheduleAssignment{}, fmt.Errorf("commit update assignment: %w", err)
	}
	return after, nil
}

// DeleteAssignment takes somebody off a slot.
//
// The activity row is written first, while the assignment still exists to be
// described. That is also why team_schedule_assignment_activity has no foreign key
// to it: a cascade would erase exactly this record.
func (r *scheduleRepository) DeleteAssignment(ctx context.Context, id, actorEmail string, note *string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete assignment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return err
	}

	before, err := assignmentByIDTx(ctx, tx, id)
	if err != nil {
		return err
	}
	if err := recordActivity(ctx, tx, before, "DELETED", actorEmail, nil, nil, nil, note); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM team_schedule_assignment WHERE id = $1::uuid`, id); err != nil {
		return fmt.Errorf("delete assignment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete assignment: %w", err)
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ActivityForTeam is "what changed on my team this week", newest first.
func (r *scheduleRepository) ActivityForTeam(ctx context.Context, teamKey, from, to string) ([]domain.ScheduleAssignmentActivity, error) {
	// Leave marked from the roster is a change a lead made too, so its history
	// is read alongside the rota's -- a "recent changes" list without it would
	// miss half of what a lead does. A span is in the window when it overlaps
	// it at all.
	rows, err := r.db.Query(ctx, `
		SELECT id, assignment_id, user_id, team_key, rota_date, NULL::date AS ends_on,
		       'rota' AS subject, shift_code,
		       action, field_name, old_value, new_value, actor_email, note, created_on
		  FROM team_schedule_assignment_activity
		 WHERE team_key = $1
		   AND rota_date BETWEEN $2::date AND $3::date
		UNION ALL
		SELECT id, absence_id, user_id, team_key, starts_on, ends_on,
		       'leave', kind_code,
		       action, field_name, old_value, new_value, actor_email, note, created_on
		  FROM team_schedule_absence_activity
		 WHERE team_key = $1
		   AND starts_on <= $3::date
		   AND COALESCE(ends_on, 'infinity'::date) >= $2::date
		 ORDER BY created_on DESC`, teamKey, from, to)
	if err != nil {
		return nil, fmt.Errorf("query schedule activity: %w", err)
	}
	defer rows.Close()

	out := []domain.ScheduleAssignmentActivity{}
	for rows.Next() {
		var a domain.ScheduleAssignmentActivity
		var rota time.Time
		var endsOn *time.Time
		if err := rows.Scan(&a.ID, &a.AssignmentID, &a.UserID, &a.TeamKey, &rota, &endsOn, &a.Subject, &a.ShiftCode,
			&a.Action, &a.FieldName, &a.OldValue, &a.NewValue, &a.ActorEmail, &a.Note, &a.CreatedOn); err != nil {
			return nil, fmt.Errorf("scan schedule activity: %w", err)
		}
		a.RotaDate = rota.Format("2006-01-02")
		if endsOn != nil {
			s := endsOn.Format("2006-01-02")
			a.EndsOn = &s
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LeadTeamsFor returns the registry keys of every team this caller leads.
//
// team.name is the display name the registry maps a key onto, so it is
// lowercased here to give the frontend the same key it filters by everywhere
// else.
func (r *scheduleRepository) LeadTeamsFor(ctx context.Context, userEmail string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT t.key
		  FROM team_member tm
		  JOIN "user" u ON u.id = tm.user_id
		  JOIN team t    ON t.id = tm.team_id
		 WHERE lower(u.email) = lower($1)
		   AND tm.role IN ('lead', 'americas_team_lead')
		 ORDER BY 1`, userEmail)
	if err != nil {
		return nil, fmt.Errorf("query lead teams: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var k *string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan lead team: %w", err)
		}
		if k != nil {
			out = append(out, *k)
		}
	}
	return out, rows.Err()
}

// RotaAdminTeamsFor returns every team key the caller may edit because they
// hold a rota admin role: every rostered team in that role's own family.
//
// Deliberately not folded into LeadTeamsFor, and deliberately not an OR inside
// LeadsTeam. Leading a team and administering a family are two different
// facts, and combining them into one permission is the service's job -- this
// file's contract is plain data operations, and a permission expressed as a
// join is a permission nobody reviewing the policy will ever read.
//
// A rota admin is not a member of the teams they may edit, so there is no
// team_member row to go through; the grant is the role itself. Matched on
// email and lowercased on both sides for the same reason LeadsTeam is -- an
// identity provider is free to return a different case from the one stored.
//
// Only an INTERNAL holder counts. The role is a schedule permission for staff
// who are already internal; it never makes anyone internal (user_type is
// derived from the admin/internal roles alone), and a holder who is not
// internal -- a customer granted it by mistake -- is simply no rota admin.
// The service refuses a non-internal caller before it asks this at all; this
// holds the same line here so the answer can never be used without it.
func (r *scheduleRepository) RotaAdminTeamsFor(ctx context.Context, userEmail string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT t.key
		  FROM "user" u
		  JOIN user_role ur ON ur.user_id = u.id
		  JOIN role ro      ON ro.id = ur.role_id
		  JOIN team t       ON `+rosteredTeamWhere+`
		                   AND `+teamFamilyExpr+` =
		                       CASE ro.name WHEN 'sre_rota_admin' THEN 'SRE' WHEN 'sme_rota_admin' THEN 'SME' ELSE 'CRE' END
		 WHERE lower(u.email) = lower($1)
		   AND u.user_type = 'INTERNAL'::user_type_enum
		   AND ro.name IN ('cre_rota_admin', 'sre_rota_admin', 'sme_rota_admin')
		 ORDER BY 1`, userEmail)
	if err != nil {
		return nil, fmt.Errorf("query rota admin teams: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		// team.key is nullable (0169_team_key_nullable) -- same NULL-scan
		// panic risk this file's own LeadTeamsFor/SearchScheduleCatalogue
		// already fix for the identical `SELECT DISTINCT t.key` shape, missed
		// here. A keyless team is dropped from the result entirely rather
		// than included as "", the same choice LeadTeamsFor already made:
		// an empty string isn't a real registry key a caller could filter by.
		var k *string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan rota admin team: %w", err)
		}
		if k != nil {
			out = append(out, *k)
		}
	}
	return out, rows.Err()
}

// ApplyRange sets one engineer to one window across a span of days.
//
// One transaction for the whole span: a picker that says "Mon to Fri" and
// leaves Wednesday half-done because the fourth insert failed is worse than
// one that fails outright.
//
// A day the window is not worked on is skipped, not refused. Picking a weekday
// rotation across a week containing a Saturday should set the five weekdays --
// day_scope already knows which days each window runs, so this asks it rather
// than deciding again here.
//
// Each day replaces whatever that engineer held: the roster is one slot per
// person per day, and "put them on the evening shift" means instead of, not as
// well as, their regular hours.
func (r *scheduleRepository) ApplyRange(ctx context.Context, req domain.ApplyScheduleRangeRequest, actorEmail string) (domain.ApplyScheduleRangeResponse, error) {
	out := domain.ApplyScheduleRangeResponse{SkippedDates: []string{}}

	from, err := time.Parse("2006-01-02", req.From)
	if err != nil {
		return out, &apierror.ValidationError{Msg: "from must be YYYY-MM-DD"}
	}
	to, err := time.Parse("2006-01-02", req.To)
	if err != nil {
		return out, &apierror.ValidationError{Msg: "to must be YYYY-MM-DD"}
	}
	if to.Before(from) {
		from, to = to, from
	}
	if to.Sub(from) > 366*24*time.Hour {
		return out, &apierror.ValidationError{Msg: "a range longer than a year is almost certainly a mistake"}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return out, fmt.Errorf("begin apply range: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return out, err
	}

	// No shift code means "take them off over this span" -- the picker's own
	// clear option. There is no window to check a day against, so no day is
	// skipped: every day in the range is cleared.
	clearing := req.ShiftCode == ""

	// The window's own day scope decides which days it can be worked.
	var scope string
	// What a write takes off the day before it lands:
	//   day      -- everything the person holds that day on this team (a CRE
	//               window, any window with no zone, or a clear of the day);
	//   turn     -- an escalation turn: only turns in the same zone or that
	//               overlap it in time. A zone's regular hours stay -- an
	//               engineer can be on TZ1's regular hours and TZ1 L1, or TZ1
	//               L1 in the morning and TZ2 L2 in the afternoon;
	//   standing -- a zone's regular hours: only the regular hours they held
	//               that day, so their turns stay;
	//   zone     -- a clear of one zone's turns, leaving the rest of the day.
	displace := "day"
	var zoneID *string
	var isEscalation bool
	if clearing && req.ZoneCode != nil {
		var id string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM team_schedule_zone WHERE code = $1`, *req.ZoneCode).Scan(&id); err != nil {
			return out, &apierror.ValidationError{Msg: fmt.Sprintf("no such zone %q", *req.ZoneCode)}
		}
		zoneID, displace = &id, "zone"
	}
	if !clearing {
		var fixedTier *string
		var isRotation bool
		if err := tx.QueryRow(ctx,
			`SELECT day_scope::text, is_escalation, tier::text, zone_id::text, is_rotation FROM team_schedule_shift WHERE code = $1`,
			req.ShiftCode).Scan(&scope, &isEscalation, &fixedTier, &zoneID, &isRotation); err != nil {
			return out, &apierror.ValidationError{Msg: fmt.Sprintf("no such shift %q", req.ShiftCode)}
		}
		// Somebody moved to a team that works no rota -- Migration -- is not on
		// the rota for those days, so no rotation turn can be put on them. The
		// span has to end first, which moves them back to their own team.
		if isRotation {
			var kindLabel, until string
			err := tx.QueryRow(ctx, `
				SELECT k.label, COALESCE(ab.ends_on::text, 'further notice')
				  FROM team_schedule_absence ab
				  JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
				 WHERE ab.user_id = $1::uuid
				   AND k.moves_to_team_key IS NOT NULL
				   AND NOT k.works_rota_there
				   AND daterange(ab.starts_on, ab.ends_on, '[]') && daterange($2::date, $3::date, '[]')
				 ORDER BY ab.starts_on LIMIT 1`,
				req.UserID, from.Format("2006-01-02"), to.Format("2006-01-02")).Scan(&kindLabel, &until)
			if err == nil {
				return out, &apierror.ConflictError{Msg: fmt.Sprintf(
					"this person is on %s until %s, which is not rota work, so they cannot take a rotation; move them back to their team first",
					kindLabel, until)}
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return out, fmt.Errorf("check for a span off the rota: %w", err)
			}
		}
		if isEscalation && zoneID != nil {
			displace = "turn"
		} else if zoneID != nil {
			displace = "standing"
		}
		// Said here rather than left to the trigger, whose message names ids
		// rather than the choice the lead actually made.
		// An escalation window that leaves the tier to the person needs one:
		// without it the turn says nothing about the ladder, and it reads as
		// the zone's regular hours, which have a window of their own.
		if req.Tier == nil && isEscalation && fixedTier == nil {
			return out, &apierror.ValidationError{Msg: fmt.Sprintf("%s needs a tier: choose L1, L2 or L3, or the zone's regular hours", req.ShiftCode)}
		}
		if req.Tier != nil {
			if !isEscalation {
				return out, &apierror.ValidationError{Msg: fmt.Sprintf("%s is not an escalation window, so it holds no tier", req.ShiftCode)}
			}
			if fixedTier != nil && *fixedTier != *req.Tier {
				return out, &apierror.ValidationError{Msg: fmt.Sprintf("%s is an %s window; it cannot hold %s", req.ShiftCode, *fixedTier, *req.Tier)}
			}
		}
	}

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		weekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
		if (scope == "WEEKDAY" && weekend) || (scope == "WEEKEND" && !weekend) {
			out.Skipped++
			out.SkippedDates = append(out.SkippedDates, d.Format("2006-01-02"))
			continue
		}
		iso := d.Format("2006-01-02")

		// Record what is being displaced before it goes, so the history is not
		// a row appearing from nowhere.
		// Scoped to the team the caller leads, not just to the person. The
		// service has already checked the engineer is on that team, but an
		// engineer can be on two: without this, a lead of one could clear
		// the row the other team put them on that day, which is not theirs
		// to touch.
		//
		// Which of that day goes is decided by `displace` above. For a turn,
		// the new window's own instants are resolved here the same way the
		// insert below resolves them, so "overlaps" means what the no-overlap
		// constraint will mean a moment later.
		rows, err := tx.Query(ctx, `SELECT `+assignmentColumns+assignmentFrom+`
			WHERE a.user_id = $1::uuid AND a.rota_date = $2::date AND lower(a.team_key) = lower($3)
			  AND (   $4::text = 'day'
			       OR ($4 = 'zone' AND a.zone_id = $5::uuid AND a.is_rotation)
			       OR ($4 = 'standing' AND NOT a.is_rotation)
			       OR ($4 = 'turn' AND a.is_rotation AND (a.zone_id = $5::uuid OR EXISTS (
			              SELECT 1 FROM team_schedule_shift n
			               WHERE n.code = $6
			                 AND tstzrange(a.starts_at, a.ends_at, '[)') && tstzrange(
			                     ($2::date::timestamp + make_interval(mins => n.start_minute)) AT TIME ZONE n.authoring_time_zone,
			                     ($2::date::timestamp + make_interval(mins => n.end_minute))   AT TIME ZONE n.authoring_time_zone,
			                     '[)')))))`,
			req.UserID, iso, req.TeamKey, displace, zoneID, req.ShiftCode)
		if err != nil {
			return out, fmt.Errorf("read displaced assignments: %w", err)
		}
		displaced, err := scanAssignments(rows)
		rows.Close()
		if err != nil {
			return out, err
		}
		ids := make([]string, 0, len(displaced))
		for _, old := range displaced {
			if err := recordActivity(ctx, tx, old, "DELETED", actorEmail, nil, nil, nil, req.Note); err != nil {
				return out, err
			}
			ids = append(ids, old.ID)
		}
		if len(ids) > 0 {
			if _, err := tx.Exec(ctx,
				`DELETE FROM team_schedule_assignment WHERE id = ANY($1::uuid[])`, ids); err != nil {
				return out, fmt.Errorf("clear the day: %w", err)
			}
		}

		if clearing {
			// A day that held nothing is not a day that was changed, so it
			// does not count towards what the caller is told was applied.
			out.Applied += len(displaced)
			continue
		}

		var id string
		err = tx.QueryRow(ctx, `
			INSERT INTO team_schedule_assignment
			  (id, created_on, updated_on, created_by, updated_by, user_id, team_id, team_key,
			   shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, note)
			SELECT gen_random_uuid(), now(), now(), $1, $1, $2::uuid,
			       (SELECT t.id FROM team t WHERE t.key = lower($3)), $3,
			       s.id, s.zone_id, COALESCE($7::team_schedule_tier_enum, s.tier), $4::date,
			       ($4::date::timestamp + make_interval(mins => s.start_minute)) AT TIME ZONE s.authoring_time_zone,
			       ($4::date::timestamp + make_interval(mins => s.end_minute))   AT TIME ZONE s.authoring_time_zone,
			       s.is_on_call, 'MANUAL', $5
			  FROM team_schedule_shift s
			 WHERE s.code = $6
			RETURNING id`,
			actorEmail, req.UserID, req.TeamKey, iso, req.Note, req.ShiftCode, req.Tier).Scan(&id)
		if err != nil {
			if c := slotConflict(err, iso); c != nil {
				return out, c
			}
			return out, fmt.Errorf("insert assignment for %s: %w", iso, err)
		}

		created, err := assignmentByIDTx(ctx, tx, id)
		if err != nil {
			return out, err
		}
		if err := recordActivity(ctx, tx, created, "CREATED", actorEmail, nil, nil, nil, req.Note); err != nil {
			return out, err
		}
		out.Applied++
	}

	if err := tx.Commit(ctx); err != nil {
		return out, fmt.Errorf("commit apply range: %w", err)
	}
	return out, nil
}

// recordAbsenceActivity writes one row of absence history inside the caller's
// transaction, for the reason migration 0153 gives on that table.
func recordAbsenceActivity(ctx context.Context, tx pgx.Tx,
	id, userID, teamKey, kindCode, startsOn string, endsOn *string,
	action, actorEmail string, field, oldV, newV, note *string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO team_schedule_absence_activity
		  (id, created_on, created_by, absence_id, user_id, team_key, kind_code,
		   starts_on, ends_on, action, field_name, old_value, new_value, actor_email, note)
		VALUES (gen_random_uuid(), now(), $1, $2::uuid, $3::uuid, $4, $5, $6::date, $7::date,
		        $8, $9, $10, $11, $1, $12)`,
		actorEmail, id, userID, teamKey, kindCode, startsOn, endsOn,
		action, field, oldV, newV, note)
	if err != nil {
		return fmt.Errorf("record absence activity: %w", err)
	}
	return nil
}

// overlappingAbsence is one existing absence the span runs into, read before
// anything is changed so the history can say what it was.
type overlappingAbsence struct {
	id       string
	kindCode string
	startsOn time.Time
	endsOn   *time.Time
}

// ApplyAbsence implements ScheduleRepository.
//
// The hard part is not the insert, it is what the span does to the absences
// already there. A lead clearing three days out of a fortnight of leave means
// exactly that -- not that the fortnight is cancelled -- so an absence that
// overlaps one end of the span is shortened, one that straddles both ends is
// split in two, and only one that falls entirely inside it is removed.
func (r *scheduleRepository) ApplyAbsence(ctx context.Context, req domain.ApplyScheduleAbsenceRequest, actorEmail string) (domain.ApplyScheduleAbsenceResponse, error) {
	out := domain.ApplyScheduleAbsenceResponse{}

	const iso = "2006-01-02"
	from, err := time.Parse(iso, req.From)
	if err != nil {
		return out, &apierror.ValidationError{Msg: "from must be YYYY-MM-DD"}
	}
	to, err := time.Parse(iso, req.To)
	if err != nil {
		return out, &apierror.ValidationError{Msg: "to must be YYYY-MM-DD"}
	}
	if to.Before(from) {
		from, to = to, from
	}
	if to.Sub(from) > 366*24*time.Hour {
		return out, &apierror.ValidationError{Msg: "a range longer than a year is almost certainly a mistake"}
	}
	dayBefore := from.AddDate(0, 0, -1).Format(iso)
	dayAfter := to.AddDate(0, 0, 1).Format(iso)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return out, fmt.Errorf("begin apply absence: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return out, err
	}

	var kindID, bucket string
	var movesTo *string
	if req.KindCode != "" {
		// Active kinds only: a retired kind is kept so older absences stay
		// readable, not so new ones can be marked against it.
		if err := tx.QueryRow(ctx,
			`SELECT id::text, bucket::text, moves_to_team_key FROM team_schedule_absence_kind WHERE code = $1 AND is_active`,
			req.KindCode).Scan(&kindID, &bucket, &movesTo); err != nil {
			return out, &apierror.ValidationError{Msg: fmt.Sprintf("no such absence kind %q", req.KindCode)}
		}
	}
	// A kind that moves people to another team (the Brazil rotation, to the
	// Americas team) is filed under that team for its dates, with the team
	// the person belongs to beside it. Every other span is filed where it
	// was marked, and carries no home team.
	fileUnder, homeTeam := req.TeamKey, (*string)(nil)
	if movesTo != nil && *movesTo != "" {
		fileUnder = *movesTo
	}
	// Any span filed away from the person's own team keeps that team beside
	// it -- leave marked on someone while they are on the Brazil rotation as
	// much as the rotation itself -- so its lead can still see and change it.
	if h := req.HomeTeamKey; h != "" && !strings.EqualFold(h, fileUnder) {
		homeTeam = &h
	}
	// Who the time is for means something only for an allocation. Leave is
	// not "for" anybody, so a value sent with a leave kind is not stored.
	allocatedTo := req.AllocatedTo
	if bucket != "ALLOCATION" {
		allocatedTo = nil
	}

	// Everything of this engineer's that the span touches. An open-ended
	// absence (ends_on null) runs forever, so it overlaps anything at or after
	// its start -- which daterange's own unbounded upper end already means.
	rows, err := tx.Query(ctx, `
		SELECT a.id::text, k.code, a.starts_on, a.ends_on
		  FROM team_schedule_absence a
		  JOIN team_schedule_absence_kind k ON k.id = a.kind_id
		 WHERE a.user_id = $1::uuid
		   AND daterange(a.starts_on, a.ends_on, '[]') && daterange($2::date, $3::date, '[]')`,
		req.UserID, from.Format(iso), to.Format(iso))
	if err != nil {
		return out, fmt.Errorf("read overlapping absences: %w", err)
	}
	var hits []overlappingAbsence
	for rows.Next() {
		var h overlappingAbsence
		if err := rows.Scan(&h.id, &h.kindCode, &h.startsOn, &h.endsOn); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan overlapping absence: %w", err)
		}
		hits = append(hits, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("read overlapping absences: %w", err)
	}

	for _, h := range hits {
		startsBefore := h.startsOn.Before(from)
		endsAfter := h.endsOn == nil || h.endsOn.After(to)

		switch {
		case startsBefore && endsAfter:
			// The span is a hole in the middle. Shorten this one to the part
			// before it and add a second row for the part after, so the two
			// stretches that still stand are both kept.
			if _, err := tx.Exec(ctx,
				`UPDATE team_schedule_absence SET ends_on = $2::date, updated_on = now(), updated_by = $3 WHERE id = $1::uuid`,
				h.id, dayBefore, actorEmail); err != nil {
				return out, fmt.Errorf("trim absence: %w", err)
			}
			var tailEnds *string
			if h.endsOn != nil {
				e := h.endsOn.Format(iso)
				tailEnds = &e
			}
			var tailID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO team_schedule_absence
				  (id, created_on, updated_on, created_by, updated_by, user_id, team_key,
				   kind_id, starts_on, ends_on, note, allocated_to, home_team_key)
				SELECT gen_random_uuid(), now(), now(), $1, $1, a.user_id, a.team_key,
				       a.kind_id, $2::date, $3::date, a.note, a.allocated_to, a.home_team_key
				  FROM team_schedule_absence a WHERE a.id = $4::uuid
				RETURNING id::text`,
				actorEmail, dayAfter, tailEnds, h.id).Scan(&tailID); err != nil {
				return out, fmt.Errorf("split absence: %w", err)
			}
			if err := recordAbsenceActivity(ctx, tx, h.id, req.UserID, req.TeamKey, h.kindCode,
				h.startsOn.Format(iso), &dayBefore, "TRIMMED", actorEmail, nil, nil, nil, req.Note); err != nil {
				return out, err
			}
			if err := recordAbsenceActivity(ctx, tx, tailID, req.UserID, req.TeamKey, h.kindCode,
				dayAfter, tailEnds, "CREATED", actorEmail, nil, nil, nil, req.Note); err != nil {
				return out, err
			}
			out.Trimmed++

		case startsBefore:
			if _, err := tx.Exec(ctx,
				`UPDATE team_schedule_absence SET ends_on = $2::date, updated_on = now(), updated_by = $3 WHERE id = $1::uuid`,
				h.id, dayBefore, actorEmail); err != nil {
				return out, fmt.Errorf("trim absence: %w", err)
			}
			if err := recordAbsenceActivity(ctx, tx, h.id, req.UserID, req.TeamKey, h.kindCode,
				h.startsOn.Format(iso), &dayBefore, "TRIMMED", actorEmail, nil, nil, nil, req.Note); err != nil {
				return out, err
			}
			out.Trimmed++

		case endsAfter:
			if _, err := tx.Exec(ctx,
				`UPDATE team_schedule_absence SET starts_on = $2::date, updated_on = now(), updated_by = $3 WHERE id = $1::uuid`,
				h.id, dayAfter, actorEmail); err != nil {
				return out, fmt.Errorf("trim absence: %w", err)
			}
			var ends *string
			if h.endsOn != nil {
				e := h.endsOn.Format(iso)
				ends = &e
			}
			if err := recordAbsenceActivity(ctx, tx, h.id, req.UserID, req.TeamKey, h.kindCode,
				dayAfter, ends, "TRIMMED", actorEmail, nil, nil, nil, req.Note); err != nil {
				return out, err
			}
			out.Trimmed++

		default:
			// Entirely inside the span, so there is nothing of it left to keep.
			var ends *string
			if h.endsOn != nil {
				e := h.endsOn.Format(iso)
				ends = &e
			}
			if err := recordAbsenceActivity(ctx, tx, h.id, req.UserID, req.TeamKey, h.kindCode,
				h.startsOn.Format(iso), ends, "DELETED", actorEmail, nil, nil, nil, req.Note); err != nil {
				return out, err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM team_schedule_absence WHERE id = $1::uuid`, h.id); err != nil {
				return out, fmt.Errorf("remove absence: %w", err)
			}
			out.Removed++
		}
	}

	if req.KindCode != "" {
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO team_schedule_absence
			  (id, created_on, updated_on, created_by, updated_by, user_id, team_key,
			   kind_id, starts_on, ends_on, note, allocated_to, home_team_key)
			VALUES (gen_random_uuid(), now(), now(), $1, $1, $2::uuid, $3, $4::uuid, $5::date, $6::date, $7, $8, $9)
			RETURNING id::text`,
			actorEmail, req.UserID, fileUnder, kindID, from.Format(iso), to.Format(iso), req.Note, allocatedTo, homeTeam).Scan(&id); err != nil {
			return out, fmt.Errorf("insert absence: %w", err)
		}
		toIso := to.Format(iso)
		if err := recordAbsenceActivity(ctx, tx, id, req.UserID, req.TeamKey, req.KindCode,
			from.Format(iso), &toIso, "CREATED", actorEmail, nil, nil, nil, req.Note); err != nil {
			return out, err
		}
		out.Created = 1
	}

	if err := syncMoveShifts(ctx, tx, req.UserID, from.Format(iso), to.Format(iso), actorEmail); err != nil {
		return out, err
	}
	if err := tx.Commit(ctx); err != nil {
		return out, fmt.Errorf("commit apply absence: %w", err)
	}
	return out, nil
}

// AbsenceByID implements ScheduleRepository.
func (r *scheduleRepository) AbsenceByID(ctx context.Context, id string) (domain.ScheduleAbsence, error) {
	var a domain.ScheduleAbsence
	var starts time.Time
	var ends *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT a.id::text, a.user_id::text, a.team_key, k.code, a.starts_on, a.ends_on
		  FROM team_schedule_absence a
		  JOIN team_schedule_absence_kind k ON k.id = a.kind_id
		 WHERE a.id = $1::uuid`, id).Scan(&a.ID, &a.Engineer.UserID, &a.TeamKey, &a.KindCode, &starts, &ends)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, &apierror.NotFoundError{Msg: "no such absence"}
	}
	if err != nil {
		return a, fmt.Errorf("read absence: %w", err)
	}
	a.StartsOn = starts.Format("2006-01-02")
	if ends != nil {
		e := ends.Format("2006-01-02")
		a.EndsOn = &e
	}
	return a, nil
}

// DeleteAbsence implements ScheduleRepository.
//
// The history row is written first, while the absence still exists to be
// described -- the same order, and the same reason, as DeleteAssignment.
func (r *scheduleRepository) DeleteAbsence(ctx context.Context, id, actorEmail string, note *string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete absence: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return err
	}

	var userID, teamKey, kindCode string
	var starts time.Time
	var ends *time.Time
	err = tx.QueryRow(ctx, `
		SELECT a.user_id::text, a.team_key, k.code, a.starts_on, a.ends_on
		  FROM team_schedule_absence a
		  JOIN team_schedule_absence_kind k ON k.id = a.kind_id
		 WHERE a.id = $1::uuid
		   FOR UPDATE OF a`, id).Scan(&userID, &teamKey, &kindCode, &starts, &ends)
	if errors.Is(err, pgx.ErrNoRows) {
		return &apierror.NotFoundError{Msg: "no such absence"}
	}
	if err != nil {
		return fmt.Errorf("read absence for delete: %w", err)
	}
	var endsOn *string
	if ends != nil {
		e := ends.Format("2006-01-02")
		endsOn = &e
	}
	if err := recordAbsenceActivity(ctx, tx, id, userID, teamKey, kindCode,
		starts.Format("2006-01-02"), endsOn, "DELETED", actorEmail, nil, nil, nil, note); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM team_schedule_absence WHERE id = $1::uuid`, id); err != nil {
		return fmt.Errorf("delete absence: %w", err)
	}
	// A removed span takes the shifts it wrote with it. An open-ended one has
	// no end to stop at, and its shifts were written to different horizons
	// (a year from when it was marked, or from when 0205 ran), so the bound
	// is the person's last MOVE shift: nothing it wrote can lie past that.
	until := starts
	if ends != nil {
		until = *ends
	} else {
		var last *time.Time
		if err := tx.QueryRow(ctx,
			`SELECT max(rota_date) FROM team_schedule_assignment WHERE user_id = $1::uuid AND source = 'MOVE'`,
			userID).Scan(&last); err != nil {
			return fmt.Errorf("read last move shift: %w", err)
		}
		if last != nil && last.After(until) {
			until = *last
		}
	}
	if err := syncMoveShifts(ctx, tx, userID, starts.Format("2006-01-02"), until.Format("2006-01-02"), actorEmail); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete absence: %w", err)
	}
	return nil
}

// DeleteAbsenceKind implements ScheduleRepository.
//
// Only a kind nothing points at goes. One still in use is refused with how
// many absences use it, rather than retired quietly: a retired kind drops out
// of the catalogue, and the absences marked with it would lose their label
// and colour on every page that draws them.
func (r *scheduleRepository) DeleteAbsenceKind(ctx context.Context, code, actorEmail string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete absence kind: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return err
	}

	var id string
	var builtIn bool
	err = tx.QueryRow(ctx, `
		SELECT id::text, created_by IS NOT DISTINCT FROM 'migration'
		  FROM team_schedule_absence_kind WHERE code = $1 FOR UPDATE`, code).Scan(&id, &builtIn)
	if errors.Is(err, pgx.ErrNoRows) {
		return &apierror.NotFoundError{Msg: "no such tag"}
	}
	if err != nil {
		return fmt.Errorf("read absence kind: %w", err)
	}
	if builtIn {
		return &apierror.ForbiddenError{Msg: "a built-in tag cannot be deleted; only tags added from the portal can"}
	}
	var inUse int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM team_schedule_absence WHERE kind_id = $1::uuid`, id).Scan(&inUse); err != nil {
		return fmt.Errorf("count absences of kind: %w", err)
	}
	if inUse > 0 {
		return &apierror.ConflictError{
			Msg: fmt.Sprintf("the tag is still used by %d leave or allocation entries; remove those first", inUse),
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM team_schedule_absence_kind WHERE id = $1::uuid`, id); err != nil {
		return fmt.Errorf("delete absence kind: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete absence kind: %w", err)
	}
	return nil
}

// CreateAbsenceKind implements ScheduleRepository.
//
// A new kind sorts after the existing kinds in its own bucket, so it lands at
// the end of the right group in the picker and the legend rather than in the
// middle of another.
func (r *scheduleRepository) CreateAbsenceKind(ctx context.Context, code string, req domain.CreateScheduleAbsenceKindRequest, actorEmail string) (domain.ScheduleAbsenceKind, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.ScheduleAbsenceKind{}, fmt.Errorf("begin create absence kind: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return domain.ScheduleAbsenceKind{}, err
	}

	// A short code is what a roster cell draws, so two active kinds sharing
	// one would be indistinguishable on the grid.
	var clash bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM team_schedule_absence_kind WHERE is_active AND lower(short_code) = lower($1))`,
		req.ShortCode).Scan(&clash); err != nil {
		return domain.ScheduleAbsenceKind{}, fmt.Errorf("check short code: %w", err)
	}
	if clash {
		return domain.ScheduleAbsenceKind{}, &apierror.ConflictError{
			Msg: fmt.Sprintf("a tag with the short code %q already exists", req.ShortCode),
		}
	}

	k := domain.ScheduleAbsenceKind{Code: code, ShortCode: req.ShortCode, Label: req.Label, Bucket: req.Bucket, ColourToken: req.ColourToken, Custom: true}
	err = tx.QueryRow(ctx, `
		INSERT INTO team_schedule_absence_kind
		  (code, short_code, label, bucket, colour_token, sort_order, created_by, updated_by)
		SELECT $1, $2, $3, $4::team_schedule_absence_bucket_enum, $5,
		       COALESCE(MAX(sort_order), 0) + 1, $6, $6
		  FROM team_schedule_absence_kind
		 WHERE bucket = $4::team_schedule_absence_bucket_enum
		ON CONFLICT (code) DO NOTHING
		RETURNING id::text, sort_order`,
		code, req.ShortCode, req.Label, req.Bucket, req.ColourToken, actorEmail).Scan(&k.ID, &k.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		// The clash is on code, which is derived from the label (normalised
		// and cut to length), so two different labels can land on one code.
		// Name the tag that actually holds it: quoting the caller's own label
		// back would claim a tag by that name exists when it may not.
		return domain.ScheduleAbsenceKind{}, r.absenceKindCodeTaken(ctx, tx, code, req.Label)
	}
	if err != nil {
		return domain.ScheduleAbsenceKind{}, fmt.Errorf("insert absence kind: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ScheduleAbsenceKind{}, fmt.Errorf("commit create absence kind: %w", err)
	}
	return k, nil
}

// absenceKindCodeTaken explains a create refused because its derived code is
// already in use, naming the kind that holds it.
func (r *scheduleRepository) absenceKindCodeTaken(ctx context.Context, tx pgx.Tx, code, label string) error {
	var existing string
	var active bool
	err := tx.QueryRow(ctx,
		`SELECT label, is_active FROM team_schedule_absence_kind WHERE code = $1`, code).Scan(&existing, &active)
	if err != nil {
		// Gone between the insert and this read, or unreadable: say what is
		// known without guessing at a name.
		return &apierror.ConflictError{Msg: fmt.Sprintf("a tag like %q already exists; choose a more distinct name", label)}
	}
	retired := ""
	if !active {
		retired = " (retired)"
	}
	if strings.EqualFold(strings.TrimSpace(existing), strings.TrimSpace(label)) {
		return &apierror.ConflictError{Msg: fmt.Sprintf("a tag called %q already exists%s", existing, retired)}
	}
	return &apierror.ConflictError{
		Msg: fmt.Sprintf("%q is too close to the existing tag %q%s; choose a more distinct name", label, existing, retired),
	}
}

// EditMarkers implements ScheduleRepository.
//
// Reads the typed activity table rather than the audit trail. Both record the
// same edits, but this one is indexed by date and holds only what the service
// did -- which is exactly the set worth marking. The audit trail also carries
// every row the seed and the importer wrote, and marking twelve thousand
// generated cells as "changed" would say nothing at all.
//
// DISTINCT ON keeps the latest change per cell: the page marks a cell once and
// names whoever touched it last, not everyone who ever has.
func (r *scheduleRepository) EditMarkers(ctx context.Context, from, to string) ([]domain.ScheduleEditMarker, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (user_id, rota_date)
		       user_id::text, rota_date::text, actor_email, created_on, action
		  FROM team_schedule_assignment_activity
		 WHERE rota_date BETWEEN $1::date AND $2::date
		   -- People, not the seed or the importer: those wrote the rota, they
		   -- did not change somebody's day.
		   AND actor_email LIKE '%@%'
		 ORDER BY user_id, rota_date, created_on DESC`, from, to)
	if err != nil {
		return nil, fmt.Errorf("read edit markers: %w", err)
	}
	defer rows.Close()

	out := []domain.ScheduleEditMarker{}
	for rows.Next() {
		var m domain.ScheduleEditMarker
		if err := rows.Scan(&m.UserID, &m.RotaDate, &m.Actor, &m.ChangedAt, &m.Action); err != nil {
			return nil, fmt.Errorf("scan edit marker: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read edit markers: %w", err)
	}
	return out, nil
}
