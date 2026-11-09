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
)

// The CS Worklogs reports read three things and assemble them in the
// service layer rather than in one wide SQL statement: the team roster, the
// logged time, and the leave. They are kept apart because the mails need
// each independently -- the weekly roster includes people who logged
// nothing at all (vikt@ in the delivered mail), and worked days come
// from leave rather than from time cards.
//
// Everything here is SELECT-only against tables that already exist. No
// migration accompanies this feature.

// WorklogRosterRow is one person's membership of one team.
type WorklogRosterRow struct {
	TeamKey  string
	TeamName string
	TeamType string
	UserID   string
	Email    string
	Name     string
	Role     string
}

// WorklogTimeRow is one person's logged time on one work item, in one
// state, on one date. The grain is deliberately fine: the service needs to
// count distinct issues per person, sum hours per team and per type, and
// tell approved from pending, and collapsing any of that in SQL would make
// the remaining two impossible.
type WorklogTimeRow struct {
	UserID   string
	ItemID   string
	IssueKey string
	ItemType string
	WorkDate time.Time
	State    string
	// Severity is the case severity (S0-S4) of the work item, empty for
	// anything that is not a case. The reports split case time into
	// Incident and Query by it.
	Severity string
	Minutes  int
	Entries  int
}

// WorklogLeaveRow is one day-blanking absence overlapping the report
// range.
//
// ServiceNow blanks a day through two separate mechanisms, and both are
// reproduced here:
//
//   - cmn_schedule_span of type time_off, time_off_morning,
//     time_off_evening, exclude or ops_night -> our LEAVE and EXCLUDED
//     buckets.
//   - a Confirmed u_customer_engagement_allocation_resource at 100%
//     utilization -> the ALLOCATION bucket's customer-engagement kinds,
//     ENG and ALLO_EXT. Both are needed: ENG belongs to the SRE family,
//     while the teams these reports cover are cre-abt and cre, whose
//     customer allocations are recorded as ALLO_EXT. Matching ENG alone
//     meant the exclusion never fired for a single team in scope.
//
// Which ALLOCATION kinds are excluded is the tag's own
// excludes_from_timesheet (ENG, ALLO_EXT and ALLO_INT among the built-in
// ones; see worklogLeaveQuery). RnD and Migration are not: they are working
// time, and blanking them would understate everyone who does that work. We hold no
// utilization figure, so every ENG absence is treated as a full
// allocation; ServiceNow would keep a member allocated at less than 100%.
type WorklogLeaveRow struct {
	UserID    string
	ShortCode string
	Label     string
	Bucket    string
	StartsOn  time.Time
	// EndsOn is zero for an open-ended absence.
	EndsOn time.Time
}

// WorklogMoveRow is one Team Schedule span that moves a person to another
// team: a tag whose kind has moves_to_team_key (Brazil rotation onto
// Americas, Migration, India team). The person stays a member of their own
// team (migrations 0202, 0274), so the roster alone
// files them under the team they were moved away from. TeamKey, TeamName
// and TeamType describe the team the span moves them to.
type WorklogMoveRow struct {
	UserID   string
	TeamKey  string
	TeamName string
	TeamType string
	StartsOn time.Time
	// EndsOn is zero for an open-ended span.
	EndsOn time.Time
}

// WorklogReportRepository reads the three inputs of the CS Worklogs reports.
type WorklogReportRepository interface {
	// Roster returns every member of every team of the given types. A user
	// in two teams yields two rows; the service picks one.
	//
	// Heads of function are left out entirely, not merely excluded from
	// the averages: ServiceNow's __internal_processRoles DELETES a member
	// whose u_team_member_role is 'team_lead' from the roster, so they
	// appear in no team section and in no count.
	//
	// Our equivalent roles already exist (0170, 0185): engineer,
	// sub_lead, lead, americas_team_lead, cre_head, cs_head. Three of
	// them are that ServiceNow role:
	//
	//   cre_head / cs_head   -- heads of function.
	//   americas_team_lead   -- unique per team by index, and on staging
	//                           it is rosa@, who appears on the daily
	//                           mail's To line and in no team section.
	//                           A recipient, not a reported developer.
	//
	// 'lead' is the other kind: several per team, matching ServiceNow's
	// per-shift leads, who stay on the roster and are only taken out of
	// the averages. Nothing here writes or changes a role.
	//
	// Heads of function are left out entirely, not merely excluded from
	// the averages: ServiceNow's __internal_processRoles DELETES a member
	// whose u_team_member_role is 'team_lead' from the roster, so they
	// appear in no team section and in no count. Migration 0170 renamed
	// our roles to engineer / sub_lead / lead / cre_head / cs_head, and
	// the two head roles are that role.
	//
	// Heads of function are left out entirely, not merely excluded from
	// the averages. ServiceNow does the same -- __internal_processRoles
	// deletes a member whose u_team_member_role is 'team_lead' from the
	// roster outright, so they appear in no team section and in no count.
	// Migration 0170 renamed our roles to engineer / sub_lead / lead /
	// cre_head / cs_head, and the two head roles are that role.
	Roster(ctx context.Context, teamTypes, excludedTeamKeys []string) ([]WorklogRosterRow, error)
	// TimeRows returns logged time whose work_date falls in [from, to].
	// Rejected cards are included so the daily report can count
	// disapproved entries; the service excludes their hours.
	TimeRows(ctx context.Context, from, to time.Time) ([]WorklogTimeRow, error)
	// Leave returns the day-blanking absences overlapping [from, to].
	Leave(ctx context.Context, from, to time.Time) ([]WorklogLeaveRow, error)
	// RosteredUserIDs returns the users with a rota assignment on any date
	// in [from, to], whatever the shift. This is the WEEKLY report's
	// membership rule and it is deliberately rota-driven: ServiceNow's
	// WeeklyTimeReportUtil.getTeamInfo builds its roster from the shift
	// API's rows for the range, not from group membership, which is why
	// the delivered weekly mail lists people who logged nothing all week.
	RosteredUserIDs(ctx context.Context, from, to time.Time) ([]string, error)
	// WeekendCrewUserIDs is RosteredUserIDs narrowed to shifts whose
	// team_schedule_shift.day_scope is WEEKEND. The daily report needs
	// the narrower set, and only on a Saturday or Sunday: ServiceNow's
	// __internal_excludeNonWeekendMembers matches cmn_schedule_span rows
	// of type ops_weekend / ops_weekend_night, not any span at all. The
	// distinction is load-bearing -- SRE_TZ3 is a WEEKDAY shift that
	// nonetheless carries weekend assignments, so "has a rota row" would
	// keep people ServiceNow drops.
	//
	// ANY counts as well. ServiceNow had no equivalent -- a span was
	// weekend-typed or it was not -- but this schema uses ANY for the
	// round-the-clock SME and IaaS rotas, and somebody assigned to one
	// on a Saturday is on weekend duty by any reading.
	WeekendCrewUserIDs(ctx context.Context, from, to time.Time) ([]string, error)
	// TeamContacts returns, per team in scope, its group address and the
	// email of every member holding role 'lead'. It is a separate read
	// from Roster because Roster collapses to one row per person, which
	// would lose a team whose every member was attributed elsewhere.
	TeamContacts(ctx context.Context, teamTypes, excludedTeamKeys []string) ([]WorklogTeamContacts, error)
	// MoveSpans returns the spans overlapping [from, to] that move a person
	// to another team, whatever that team's type; the service decides
	// which of them it reports under.
	MoveSpans(ctx context.Context, from, to time.Time) ([]WorklogMoveRow, error)
}

// WorklogTeamContacts is one team's derived audience. Members and leads
// are both returned because the two reports draw on them differently: a
// leadership team contributes its whole membership, a delivery team only
// its leads.
type WorklogTeamContacts struct {
	TeamKey      string
	TeamName     string
	TeamType     string
	GroupEmail   string
	LeadEmails   []string
	MemberEmails []string
}

type worklogReportRepo struct {
	db *Scoped
}

// NewWorklogReportRepository constructs a WorklogReportRepository. It takes
// a *Scoped rather than a raw pool because time_card and work_item both
// carry row-level security (migrations 100005, 100008): these reports run
// as the system identity, and reading them through an unscoped pool would
// quietly bypass the policies.
func NewWorklogReportRepository(db *Scoped) WorklogReportRepository {
	return &worklogReportRepo{db: db}
}

// A member with role 'lead' in ANY team is left out of the roster entirely,
// in both reports: the previous system removes its team-lead role from the
// roster outright. Our 'lead' also covers per-shift leads that system keeps
// (a known, accepted conflation, no data changed). TeamContacts, and so the
// derived recipients, still use lead emails.
// A user can hold rows in more than one team -- team_member has no unique
// constraint on user_id -- but each person appears under exactly one team in
// the mails. DISTINCT ON picks one deterministically: a team the person
// leads first (a lead is a lead of their own team), then by team key, so the
// choice never varies between runs of the same data.
const worklogRosterQuery = `
SELECT DISTINCT ON (u.id)
       t.key,
       COALESCE(t.name, ''),
       COALESCE(t.type, ''),
       u.id::text,
       COALESCE(u.email, ''),
       TRIM(COALESCE(u.name, CONCAT_WS(' ', u.first_name, u.last_name))),
       m.role
  FROM team_member m
  JOIN team t ON t.id = m.team_id
  JOIN "user" u ON u.id = m.user_id
 WHERE t.key IS NOT NULL
   AND (cardinality($1::text[]) = 0 OR LOWER(COALESCE(t.type, '')) = ANY($1))
   AND NOT (t.key = ANY($2))
   AND m.role NOT IN ('cre_head', 'cs_head', 'americas_team_lead')
   AND NOT EXISTS (SELECT 1 FROM team_member lm WHERE lm.user_id = u.id AND lm.role = 'lead')
   AND COALESCE(u.is_active, TRUE)
 ORDER BY u.id, (m.role = 'lead') DESC, t.key`

func (r *worklogReportRepo) Roster(ctx context.Context, teamTypes, excludedTeamKeys []string) ([]WorklogRosterRow, error) {
	// Both queries compare the type case-insensitively, so the caller's
	// values are normalised to match. lowerAll never returns nil, which
	// matters: pgx will not bind a nil slice to text[].
	teamTypes = lowerAll(teamTypes)
	rows, err := r.db.Query(ctx, worklogRosterQuery, teamTypes, nonNil(excludedTeamKeys))
	if err != nil {
		return nil, fmt.Errorf("query worklog roster: %w", err)
	}
	defer rows.Close()

	var out []WorklogRosterRow
	for rows.Next() {
		var x WorklogRosterRow
		if err := rows.Scan(&x.TeamKey, &x.TeamName, &x.TeamType, &x.UserID, &x.Email, &x.Name, &x.Role); err != nil {
			return nil, fmt.Errorf("scan worklog roster: %w", err)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worklog roster: %w", err)
	}
	return out, nil
}

// Hours are the sum of the five minute columns; time_card has no total
// column. COALESCE each one -- they are individually nullable, and in
// Postgres a single NULL would null the whole sum.
const worklogTimeQuery = `
SELECT tc.user_id::text,
       wi.id::text,
       COALESCE(NULLIF(wi.wso2_id, ''), wi.number),
       COALESCE(wi.type::TEXT, ''),
       tc.work_date,
       COALESCE(tc.state::TEXT, 'UNKNOWN'),
       COALESCE(c.severity::TEXT, ''),
       SUM(COALESCE(tc.analyzing_minutes, 0) + COALESCE(tc.setting_up_minutes, 0) +
           COALESCE(tc.reproducing_debugging_minutes, 0) + COALESCE(tc.providing_solution_minutes, 0) +
           COALESCE(tc.patching_minutes, 0))::int,
       COUNT(*)::int
  FROM time_card tc
  JOIN work_item wi ON wi.id = tc.case_id
  LEFT JOIN "case" c ON c.id = wi.id
 WHERE tc.work_date IS NOT NULL
   AND tc.work_date >= $1
   AND tc.work_date <= $2
 GROUP BY tc.user_id, wi.id, wi.wso2_id, wi.number, wi.type, tc.work_date, tc.state, c.severity
 ORDER BY tc.user_id, tc.work_date DESC, wi.id`

func (r *worklogReportRepo) TimeRows(ctx context.Context, from, to time.Time) ([]WorklogTimeRow, error) {
	rows, err := r.db.Query(ctx, worklogTimeQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("query worklog time rows: %w", err)
	}
	defer rows.Close()

	var out []WorklogTimeRow
	for rows.Next() {
		var x WorklogTimeRow
		if err := rows.Scan(&x.UserID, &x.ItemID, &x.IssueKey, &x.ItemType,
			&x.WorkDate, &x.State, &x.Severity, &x.Minutes, &x.Entries); err != nil {
			return nil, fmt.Errorf("scan worklog time row: %w", err)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worklog time rows: %w", err)
	}
	return out, nil
}

// An absence with a NULL ends_on is open-ended and overlaps anything that
// starts on or after it.
//
// WHICH ALLOCATION KINDS REMOVE SOMEBODY FROM THE REPORT.
//
// The ones whose tag says so: excludes_from_timesheet (migration 0281). A
// lead sets it when adding a tag (an SRE rotation tag does, a work-from-home
// tag does not), and 0281 set it on the three kinds this report used to name
// by code: ALLO_INT and ALLO_EXT, a pair -- "Allocated — internal" and
// "Allocated — external" (migration 0154) -- and ENG, the customer
// engagement added by 0215. Someone allocated is not available for support
// either way, so all three count. Listing EXT without INT was an oversight:
// it inflated every team's member count and its average's divisor, which is
// what the CS leads reported against the 27 Oct mail.
//
// The other built-in ALLOCATION kinds deliberately do not have it:
//
//	MIGRATION  -- the CS Migrations Team's own work. The delivered mail
//	              lists those people with real approved hours, so
//	              excluding them would empty the team that logged the
//	              most time.
//	RND        -- SRE-scoped, and the reports cover CRE teams.
//	ALLO_BR    -- Brazil rotation, a working rotation rather than an
//	              absence from support.
//
// Add one only with evidence from a delivered mail that it should be
// absent; the cost of guessing wrong is a person silently vanishing from
// their team's figures.
//
// CONFIRMED ENGAGEMENT ALLOCATIONS are read straight from
// customer_engagement_allocation_resource and returned as ALLOCATION rows
// (short code 'ENG_ALLOC'), because that table, not the schedule
// absences, is what the previous system's report consults: state
// CONFIRMED, utilization at least 100, dates overlapping the range, and
// the person must be a resource with both dates set. Any allocation that
// meets that blanks the covered days, whatever its engagement type.
const worklogLeaveQuery = `
SELECT user_id, short_code, label, bucket, starts_on, ends_on
  FROM (
    SELECT a.user_id::text AS user_id,
           k.short_code,
           k.label,
           k.bucket,
           a.starts_on,
           a.ends_on
      FROM team_schedule_absence a
      JOIN team_schedule_absence_kind k ON k.id = a.kind_id
     WHERE (k.bucket IN ('LEAVE', 'EXCLUDED')
            OR (k.bucket = 'ALLOCATION' AND k.excludes_from_timesheet))
       AND a.starts_on <= $2
       AND (a.ends_on IS NULL OR a.ends_on >= $1)
    UNION ALL
    SELECT ar.resource_id::text,
           'ENG_ALLOC',
           'Engagement allocation',
           'ALLOCATION',
           ar.start_date,
           ar.end_date
      FROM customer_engagement_allocation_resource ar
     WHERE ar.resource_id IS NOT NULL
       AND ar.state = 'CONFIRMED'
       AND ar.utilization >= 100
       AND ar.start_date IS NOT NULL
       AND ar.end_date IS NOT NULL
       AND ar.start_date <= $2
       AND ar.end_date >= $1
  ) x
 ORDER BY user_id, starts_on`

// The destination is the kind's moves_to_team_key, the team the tag moves
// people to (0202 files the span itself under that team as well).
const worklogMoveSpansQuery = `
SELECT a.user_id::text,
       t.key,
       COALESCE(t.name, ''),
       COALESCE(t.type, ''),
       a.starts_on,
       a.ends_on
  FROM team_schedule_absence a
  JOIN team_schedule_absence_kind k ON k.id = a.kind_id
  JOIN team t ON LOWER(t.key) = LOWER(k.moves_to_team_key)
 WHERE k.moves_to_team_key IS NOT NULL
   AND a.starts_on <= $2
   AND (a.ends_on IS NULL OR a.ends_on >= $1)
 ORDER BY a.user_id, a.starts_on, t.key`

const worklogTeamContactsQuery = `
SELECT t.key,
       COALESCE(t.name, ''),
       LOWER(COALESCE(t.type, '')),
       COALESCE(t.group_email, ''),
       COALESCE(
         ARRAY_AGG(DISTINCT u.email)
           FILTER (WHERE m.role = 'lead' AND COALESCE(u.email, '') <> ''),
         '{}'
       ),
       COALESCE(
         ARRAY_AGG(DISTINCT u.email) FILTER (WHERE COALESCE(u.email, '') <> ''),
         '{}'
       )
  FROM team t
  LEFT JOIN team_member m ON m.team_id = t.id
  LEFT JOIN "user" u ON u.id = m.user_id AND COALESCE(u.is_active, TRUE)
 WHERE t.key IS NOT NULL
   AND (cardinality($1::text[]) = 0 OR LOWER(COALESCE(t.type, '')) = ANY($1))
   AND NOT (t.key = ANY($2))
 GROUP BY t.key, t.name, t.type, t.group_email
 ORDER BY t.key`

func (r *worklogReportRepo) TeamContacts(ctx context.Context, teamTypes, excludedTeamKeys []string) ([]WorklogTeamContacts, error) {
	// Both queries compare the type case-insensitively, so the caller's
	// values are normalised to match. lowerAll never returns nil, which
	// matters: pgx will not bind a nil slice to text[].
	teamTypes = lowerAll(teamTypes)
	rows, err := r.db.Query(ctx, worklogTeamContactsQuery, teamTypes, nonNil(excludedTeamKeys))
	if err != nil {
		return nil, fmt.Errorf("query worklog team contacts: %w", err)
	}
	defer rows.Close()

	var out []WorklogTeamContacts
	for rows.Next() {
		var x WorklogTeamContacts
		if err := rows.Scan(&x.TeamKey, &x.TeamName, &x.TeamType, &x.GroupEmail,
			&x.LeadEmails, &x.MemberEmails); err != nil {
			return nil, fmt.Errorf("scan worklog team contacts: %w", err)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worklog team contacts: %w", err)
	}
	return out, nil
}

const worklogRosteredQuery = `
SELECT DISTINCT a.user_id::text
  FROM team_schedule_assignment a
 WHERE a.rota_date >= $1
   AND a.rota_date <= $2`

// The weekend variant. day_scope lives on the shift, not the assignment,
// so this has to join -- and the join is the point: filtering on the
// DATE being a weekend would keep a weekday shift that happens to run on
// a Saturday, which is exactly what ServiceNow's span-type match excludes.
const worklogWeekendCrewQuery = `
SELECT DISTINCT a.user_id::text
  FROM team_schedule_assignment a
  JOIN team_schedule_shift s ON s.id = a.shift_id
 WHERE a.rota_date >= $1
   AND a.rota_date <= $2
   AND s.day_scope IN ('WEEKEND', 'ANY')`

func (r *worklogReportRepo) RosteredUserIDs(ctx context.Context, from, to time.Time) ([]string, error) {
	return r.rotaUserIDs(ctx, worklogRosteredQuery, from, to)
}

func (r *worklogReportRepo) WeekendCrewUserIDs(ctx context.Context, from, to time.Time) ([]string, error) {
	return r.rotaUserIDs(ctx, worklogWeekendCrewQuery, from, to)
}

func (r *worklogReportRepo) rotaUserIDs(ctx context.Context, query string, from, to time.Time) ([]string, error) {
	rows, err := r.db.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("query worklog rostered users: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan worklog rostered user: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worklog rostered users: %w", err)
	}
	return out, nil
}

func (r *worklogReportRepo) Leave(ctx context.Context, from, to time.Time) ([]WorklogLeaveRow, error) {
	rows, err := r.db.Query(ctx, worklogLeaveQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("query worklog leave: %w", err)
	}
	defer rows.Close()

	var out []WorklogLeaveRow
	for rows.Next() {
		var x WorklogLeaveRow
		var ends *time.Time
		if err := rows.Scan(&x.UserID, &x.ShortCode, &x.Label, &x.Bucket, &x.StartsOn, &ends); err != nil {
			return nil, fmt.Errorf("scan worklog leave: %w", err)
		}
		if ends != nil {
			x.EndsOn = *ends
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worklog leave: %w", err)
	}
	return out, nil
}

// MoveSpans implements WorklogReportRepository: every span overlapping
// [from, to] whose kind moves people to another team, with that team.
func (r *worklogReportRepo) MoveSpans(ctx context.Context, from, to time.Time) ([]WorklogMoveRow, error) {
	rows, err := r.db.Query(ctx, worklogMoveSpansQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("query worklog move spans: %w", err)
	}
	defer rows.Close()

	var out []WorklogMoveRow
	for rows.Next() {
		var x WorklogMoveRow
		var ends *time.Time
		if err := rows.Scan(&x.UserID, &x.TeamKey, &x.TeamName, &x.TeamType, &x.StartsOn, &ends); err != nil {
			return nil, fmt.Errorf("scan worklog move span: %w", err)
		}
		if ends != nil {
			x.EndsOn = *ends
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worklog move spans: %w", err)
	}
	return out, nil
}
