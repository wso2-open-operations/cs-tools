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

package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// The CS Worklogs daily report, reproduced against a real Postgres from the
// mail WSO2 Customer Service actually delivered for Sunday 2026-10-18.
//
// This is the whole point of the feature's test strategy: there is no
// ServiceNow original to diff against, so the delivered mail is the oracle.
// Every figure asserted below is quoted from it.
//
// Skipped unless ENTITY_TEST_DATABASE_URL is set, like the repository
// integration tests:
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" \
//	  go test -run TestWorklogDailyReportMatchesDeliveredMail ./internal/service/
//
// The seed reproduces the mail's hours exactly in whole minutes, which is
// how the real data is stored -- 1.42 h is 85 minutes, not 85.2.
func TestWorklogDailyReportMatchesDeliveredMail(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)

	// The reports run as the system identity: they report on every team,
	// so a viewer-scoped read would silently drop rows.
	ctx = repository.WithSystemIdentity(ctx)
	svc := service.NewWorklogReportService(
		repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, nil)
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	got, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily,
		From:    day,
		To:      day,
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}

	// WORKLOG_REPORT_JSON_OUT writes the built report out so the
	// csm-scheduled-tasks renderer can be driven from real output rather
	// than a second, independently-drifting fixture. Purely a test aid.
	if out := os.Getenv("WORKLOG_REPORT_JSON_OUT"); out != "" {
		blob, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}
		if err := os.WriteFile(out, blob, 0o600); err != nil {
			t.Fatalf("write %s: %v", out, err)
		}
		t.Logf("report written to %s", out)
	}

	// --- Summary block, verbatim from the mail ---------------------------
	if got.Totals.Members != 6 {
		t.Errorf("Total Members = %d, mail says 6", got.Totals.Members)
	}
	if got.Totals.Issues != 10 {
		t.Errorf("Total Issues = %d, mail says 10", got.Totals.Issues)
	}
	if got.Totals.TotalHours != 17.42 {
		t.Errorf("Total Support Time = %.2f, mail says 17.42", got.Totals.TotalHours)
	}
	if got.Totals.AvgHoursPerPerson != 2.90 {
		t.Errorf("Average Support Time per person = %.2f, mail says 2.90", got.Totals.AvgHoursPerPerson)
	}
	if got.Totals.ApprovedHours != 0.00 {
		t.Errorf("Approved = %.2f, mail says 0.00", got.Totals.ApprovedHours)
	}
	if got.Totals.PendingHours != 17.42 {
		t.Errorf("Pending = %.2f, mail says 17.42", got.Totals.PendingHours)
	}
	if got.Totals.NonLeaveMembers != 6 {
		t.Errorf("Non Leave Members = %v, mail says 6", got.Totals.NonLeaveMembers)
	}
	if got.Totals.QueryIssues != 2 {
		t.Errorf("Queries = %d, mail says 2", got.Totals.QueryIssues)
	}
	if got.Totals.IncidentIssues != 7 {
		t.Errorf("Incidents = %d, mail says 7", got.Totals.IncidentIssues)
	}

	// The head of function logged 10 hours that day and must appear
	// nowhere: not in the headcount, not in the hours, not in a team.
	for _, tr := range got.Teams {
		for _, d := range tr.Developers {
			if d.Email == "headof@example.com" {
				t.Errorf("headof@ is a cs_head and must not appear in %s", tr.TeamName)
			}
		}
	}

	// --- Derived recipients ----------------------------------------------
	// Every team lead on To, every team group address on Cc, each once
	// and sorted. Nothing here comes from configuration.
	// Daily: management by membership, plus every delivery team's lead.
	wantTo := []string{"americaslead@example.com", "atlaslead@example.com", "castorlead@example.com",
		"dracolead@example.com", "mara@example.com", "nils@example.com", "vegalead@example.com"}
	if strings.Join(got.Recipients.To, ",") != strings.Join(wantTo, ",") {
		t.Errorf("derived To = %v, want %v", got.Recipients.To, wantTo)
	}
	wantCc := []string{"americas-list@example.com", "atlas-list@example.com",
		"castor-list@example.com", "draco-list@example.com", "leads-list@example.com",
		"vega-list@example.com"}
	if strings.Join(got.Recipients.Cc, ",") != strings.Join(wantCc, ",") {
		t.Errorf("derived Cc = %v, want %v", got.Recipients.Cc, wantCc)
	}
	// bruno@ is a plain member and must not be on the To line.
	for _, addr := range got.Recipients.To {
		if addr == "bruno@example.com" {
			t.Error("bruno@ is a member, not a lead, and must not be a recipient")
		}
	}

	// --- By type ---------------------------------------------------------
	wantType := map[string]struct {
		issues int
		hours  float64
	}{
		"Query":           {2, 2.75},
		"Incident":        {7, 13.33},
		"Service Request": {1, 1.33},
	}
	if len(got.ByType) != len(wantType) {
		t.Errorf("by-type buckets = %d, mail shows %d", len(got.ByType), len(wantType))
	}
	for _, b := range got.ByType {
		w, ok := wantType[b.Label]
		if !ok {
			t.Errorf("unexpected type bucket %q", b.Label)
			continue
		}
		if b.Issues != w.issues {
			t.Errorf("%s issues = %d, mail says %d", b.Label, b.Issues, w.issues)
		}
		if b.TotalHours != w.hours {
			t.Errorf("%s hours = %.2f, mail says %.2f", b.Label, b.TotalHours, w.hours)
		}
	}

	// --- Per team --------------------------------------------------------
	wantTeam := map[string]struct {
		members int
		issues  int
		pending float64
		avg     float64
	}{
		"Americas CS Team - Integraion": {2, 2, 1.42, 0.71},
		"Atlas":                         {1, 1, 2.17, 2.17},
		"Castor":                        {1, 0, 0.00, 0.00},
		"Draco":                         {1, 3, 7.33, 7.33},
		"Vega":                          {1, 4, 6.50, 6.50},
	}
	if len(got.Teams) != len(wantTeam) {
		t.Fatalf("teams = %d, mail shows %d", len(got.Teams), len(wantTeam))
	}
	for _, tr := range got.Teams {
		w, ok := wantTeam[tr.TeamName]
		if !ok {
			t.Errorf("unexpected team %q", tr.TeamName)
			continue
		}
		if tr.Members != w.members {
			t.Errorf("%s members = %d, mail says %d", tr.TeamName, tr.Members, w.members)
		}
		if tr.Issues != w.issues {
			t.Errorf("%s issues = %d, mail says %d", tr.TeamName, tr.Issues, w.issues)
		}
		if tr.PendingHours != w.pending {
			t.Errorf("%s pending hrs = %.2f, mail says %.2f", tr.TeamName, tr.PendingHours, w.pending)
		}
		if tr.AvgTotalHoursPerPerson != w.avg {
			t.Errorf("%s avg total hrs/person = %.2f, mail says %.2f",
				tr.TeamName, tr.AvgTotalHoursPerPerson, w.avg)
		}
	}
}

// worklogFixtureUser is one seeded person: their team, and the issues they
// logged on 2026-10-18 with the work item's type and the minutes logged.
type worklogFixtureUser struct {
	email    string
	name     string
	teamKey  string
	teamName string
	issues   []worklogFixtureIssue
}

// worklogFixtureGroupEmails is each team's group address, the derived Cc
// line. A team with none is simply left out of it.
var worklogFixtureGroupEmails = map[string]string{
	"americas": "americas-list@example.com",
	"atlas":    "atlas-list@example.com",
	"castor":   "castor-list@example.com",
	"draco":    "draco-list@example.com",
	"vega":     "vega-list@example.com",
}

type worklogFixtureIssue struct {
	key      string
	itemType string
	minutes  int
}

// The mail prints hours; these are the whole minutes that round to them.
// 85 -> 1.42, 130 -> 2.17, 440 -> 7.33, 35 -> 0.58, 80 -> 1.33, 275 -> 4.58.
var worklogDailyFixture = []worklogFixtureUser{
	{email: "asha@example.com", name: "Sumudu", teamKey: "americas",
		teamName: "Americas CS Team - Integraion", issues: []worklogFixtureIssue{
			{"ACME-515", "INCIDENT", 85},
			{"ACME-516", "INCIDENT", 0},
		}},
	{email: "bruno@example.com", name: "Mihaly", teamKey: "americas",
		teamName: "Americas CS Team - Integraion"},
	{email: "chen@example.com", name: "Kaushalya", teamKey: "atlas",
		teamName: "Atlas", issues: []worklogFixtureIssue{
			{"BOREAL-317", "CASE", 130},
		}},
	{email: "dara@example.com", name: "Asela", teamKey: "castor",
		teamName: "Castor"},
	{email: "elif@example.com", name: "Pasindu", teamKey: "draco",
		teamName: "Draco", issues: []worklogFixtureIssue{
			{"CEDAR-1549", "INCIDENT", 200},
			{"DELTA-2157", "INCIDENT", 140},
			{"ECHO-214", "INCIDENT", 100},
		}},
	{email: "farid@example.com", name: "Lumbini", teamKey: "vega",
		teamName: "Vega", issues: []worklogFixtureIssue{
			{"FABLE-341", "CASE", 35},
			{"CEDAR-1549-V", "SERVICE_REQUEST", 80},
			{"GROVE-270", "INCIDENT", 175},
			{"HAVEN-299", "INCIDENT", 100},
		}},
}

func seedWorklogDailyFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}

	// Clear only the rows this fixture owns.
	//
	// TRUNCATE ... team ... CASCADE would reach further than it looks:
	// migration 0202 gives team_schedule_absence_kind a
	// moves_to_team_key referencing team(key), so the cascade empties
	// the leave-kind CATALOGUE as well -- twenty rows of reference data
	// the migrations seed once. Every test that looks up ANNUAL_LEAVE
	// then fails on a freshly migrated database, which is to say the
	// leave paths were never actually being verified, and any other
	// suite sharing the database loses the catalogue too.
	fixtureTeams := []string{"americas", "atlas", "castor", "draco", "vega", "cs-leadership"}
	exec(`DELETE FROM time_card WHERE created_by = 'seed'`)
	exec(`DELETE FROM team_schedule_assignment WHERE team_key = ANY($1)`, fixtureTeams)
	exec(`DELETE FROM team_schedule_absence WHERE team_key = ANY($1)`, fixtureTeams)
	exec(`DELETE FROM team_member WHERE team_id IN (SELECT id FROM team WHERE key = ANY($1))`, fixtureTeams)
	exec(`DELETE FROM work_item WHERE created_by = 'seed'`)
	exec(`DELETE FROM team WHERE key = ANY($1)`, fixtureTeams)
	exec(`DELETE FROM "user" WHERE email LIKE '%@example.com'`)

	// Management: its own team, type cre-leadership, contributing its
	// whole membership rather than just a lead. It appears in no team
	// section of either report -- nobody on it logs time -- but it is on
	// both audiences.
	exec(`INSERT INTO team (id, created_on, updated_on, name, type, key, group_email)
	      VALUES ('00000000-0000-0000-0000-00000000cafe', NOW(), NOW(),
	              'CS Leadership', 'cre-leadership', 'cs-leadership',
	              'leads-list@example.com')`)
	for i, mgr := range []string{"mara@example.com", "nils@example.com"} {
		mgrID := fmt.Sprintf("00000000-0000-0000-0000-0000000cafe%d", i)
		exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email)
		      VALUES ($1, NOW(), NOW(), $2, $2, $2)`, mgrID, mgr)
		exec(`INSERT INTO team_member (id, created_on, updated_on, team_id, user_id, role)
		      VALUES (gen_random_uuid(), NOW(), NOW(),
		              '00000000-0000-0000-0000-00000000cafe', $1, 'engineer')`, mgrID)
	}

	// A head of function sitting on a delivery team, rostered that
	// Sunday and with time logged. ServiceNow deletes such a member from
	// the roster outright, so none of this may surface anywhere: not in
	// Total Members, not in the team's member count, not in its hours.
	exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email)
	      VALUES ('00000000-0000-0000-0000-0000000000ad', NOW(), NOW(),
	              'headof@example.com', 'CS Head', 'headof@example.com')`)

	// day_scope is WEEKEND, not the column's WEEKDAY default. It is what
	// makes these six count as the weekend crew: a daily report for a
	// Saturday or Sunday takes its membership from WEEKEND/ANY-scoped
	// assignments alone, mirroring ServiceNow's ops_weekend span match.
	exec(`INSERT INTO team_schedule_shift (id, code, short_code, label, colour_token, family, start_minute, end_minute, day_scope)
	      VALUES ('00000000-0000-0000-0000-0000000000ff', 'WKND', 'WK', 'Weekend', 'WK', 'CRE', 540, 1080, 'WEEKEND')
	      ON CONFLICT (id) DO UPDATE SET day_scope = EXCLUDED.day_scope`)

	seenTeam := map[string]bool{}
	for i, u := range worklogDailyFixture {
		userID := fmt.Sprintf("00000000-0000-0000-0000-00000000%04d", i+1)
		teamID := fmt.Sprintf("00000000-0000-0000-0000-0000000%05d", 900+len(seenTeam))

		isNewTeam := !seenTeam[u.teamKey]
		if isNewTeam {
			seenTeam[u.teamKey] = true
			exec(`INSERT INTO team (id, created_on, updated_on, name, type, key, group_email)
			      VALUES ($1, NOW(), NOW(), $2, 'cre', $3, $4)`,
				teamID, u.teamName, u.teamKey, worklogFixtureGroupEmails[u.teamKey])
		}
		var realTeamID string
		if err := pool.QueryRow(ctx, `SELECT id::text FROM team WHERE key = $1`, u.teamKey).Scan(&realTeamID); err != nil {
			t.Fatalf("team lookup %s: %v", u.teamKey, err)
		}

		exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email)
		      VALUES ($1, NOW(), NOW(), $2, $3, $2)`, userID, u.email, u.name)
		// Migration 0170 renamed 'member' to 'engineer' and added the
		// sub_lead / cre_head / cs_head roles. 'member' no longer passes
		// the CHECK constraint.
		exec(`INSERT INTO team_member (id, created_on, updated_on, team_id, user_id, role)
		      VALUES (gen_random_uuid(), NOW(), NOW(), $1, $2, 'engineer')`, realTeamID, userID)

		// Every one of the six was on the rota that Sunday. This is what
		// puts bruno@ and dara@ in the mail with nothing logged.
		exec(`INSERT INTO team_schedule_assignment
		        (user_id, team_key, shift_id, rota_date, starts_at, ends_at)
		      VALUES ($1, $2, '00000000-0000-0000-0000-0000000000ff', DATE '2026-10-18',
		              TIMESTAMPTZ '2026-10-18 09:00+05:30', TIMESTAMPTZ '2026-10-18 18:00+05:30')`,
			userID, u.teamKey)

		// The head joins the first team seeded, with a logged time card,
		// so the exclusion is tested against real data rather than an
		// absence of it.
		if i == 0 {
			exec(`INSERT INTO team_member (id, created_on, updated_on, team_id, user_id, role)
			      VALUES (gen_random_uuid(), NOW(), NOW(), $1,
			              '00000000-0000-0000-0000-0000000000ad', 'cs_head')`, realTeamID)
			exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by,
			        number, wso2_id, subject, type)
			      VALUES ('00000000-0000-0000-0000-0000000000be', NOW(), NOW(), 'seed', 'seed',
			              'LOTUS-1', 'LOTUS-1', 'LOTUS-1', 'INCIDENT')`)
			exec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
			        case_id, user_id, work_date, state, analyzing_minutes)
			      VALUES (gen_random_uuid(), NOW(), NOW(), 'seed', 'seed',
			              '00000000-0000-0000-0000-0000000000be',
			              '00000000-0000-0000-0000-0000000000ad',
			              DATE '2026-10-18', 'SUBMITTED', 600)`)
		}

		// Each team's lead, who is NOT on the rota and logs nothing, so
		// they never enter a report. They exist because the derived
		// audience is built from the team registry rather than from the
		// report's own membership -- and because the delivered daily
		// mail's averages only reconcile if none of the six people it
		// lists is a lead.
		if isNewTeam {
			leadID := fmt.Sprintf("00000000-0000-0000-0000-00000000c%03d", 100+i)
			leadEmail := u.teamKey + "lead@example.com"
			exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email)
			      VALUES ($1, NOW(), NOW(), $2, $2, $2)`, leadID, leadEmail)
			exec(`INSERT INTO team_member (id, created_on, updated_on, team_id, user_id, role)
			      VALUES (gen_random_uuid(), NOW(), NOW(), $1, $2, 'lead')`, realTeamID, leadID)
		}

		for j, iss := range u.issues {
			itemID := fmt.Sprintf("00000000-0000-0000-0000-%04d%08d", i+1, j+1)
			exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by,
			        number, wso2_id, subject, type)
			      VALUES ($1, NOW(), NOW(), 'seed', 'seed', $2, $2, $2, $3::work_item_type_enum)`,
				itemID, iss.key, iss.itemType)
			exec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
			        case_id, user_id, work_date, state, analyzing_minutes)
			      VALUES (gen_random_uuid(), NOW(), NOW(), 'seed', 'seed',
			              $1, $2, DATE '2026-10-18', 'SUBMITTED', $3)`,
				itemID, userID, iss.minutes)
		}
	}
}

// The weekly report over the week containing that Sunday, on the same
// seed. There is no delivered weekly mail for this week to diff against,
// so this asserts the quantities the daily report does not exercise at
// all: worked days (weekdays minus leave, not days with time cards), the
// per-day average derived from them, and average people per day.
//
// One person is given a Wednesday leave day so the worked-day arithmetic
// is actually tested rather than being 5 for everybody.
func TestWorklogWeeklyReportDerivedQuantities(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)

	// Annual leave on Wednesday 2026-10-14 for bruno@, who is user 2.
	var kindID string
	if err := pool.QueryRow(ctx,
		`SELECT id::text FROM team_schedule_absence_kind WHERE code = 'ANNUAL_LEAVE'`).Scan(&kindID); err != nil {
		t.Fatalf("annual leave kind: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO team_schedule_absence (user_id, team_key, starts_on, ends_on, kind_id)
		 VALUES ($1, 'americas', DATE '2026-10-14', DATE '2026-10-14', $2)`,
		"00000000-0000-0000-0000-000000000002", kindID); err != nil {
		t.Fatalf("seed leave: %v", err)
	}

	// Time spread across the working week, so the weekly report is not
	// just the Sunday's figures with different column headers. Minutes
	// again, since that is how the data is stored.
	seedWorklogWeekdayTime(ctx, t, pool)

	// asha@ leads Americas. The weekly average must then drop both
	// their hours and one off the divisor, leaving bruno@'s 1.00 hour
	// over one person.
	if _, err := pool.Exec(ctx,
		`UPDATE team_member SET role = 'lead' WHERE user_id = $1`,
		"00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("mark lead: %v", err)
	}

	got, err := svcWeekly(ctx, pool)
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}

	if out := os.Getenv("WORKLOG_WEEKLY_JSON_OUT"); out != "" {
		blob, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}
		if err := os.WriteFile(out, blob, 0o600); err != nil {
			t.Fatalf("write %s: %v", out, err)
		}
	}

	// Mon 12th to Sun 18th is five weekdays. Everybody worked all five
	// except bruno@, who took the Wednesday.
	byEmail := map[string]domain.WorklogDeveloper{}
	for _, tr := range got.Teams {
		for _, d := range tr.Developers {
			byEmail[d.Email] = d
		}
	}
	if d := byEmail["farid@example.com"]; d.WorkedDays != 5 {
		t.Errorf("lumbini worked days = %v, want 5", d.WorkedDays)
	}
	if d := byEmail["bruno@example.com"]; d.WorkedDays != 4 {
		t.Errorf("mihaly worked days = %v, want 4 (one day's annual leave)", d.WorkedDays)
	}
	// dara@ logged nothing all week and is still listed, with five
	// worked days -- the case that proves worked days are not derived
	// from time cards.
	if d, ok := byEmail["dara@example.com"]; !ok {
		t.Error("dara@ is missing; the weekly report lists rostered people who logged nothing")
	} else if d.WorkedDays != 5 || d.TotalHours != 0 {
		t.Errorf("dara@ worked days = %v, hours = %v; want 5 and 0", d.WorkedDays, d.TotalHours)
	}
	// lumbini logged 390 minutes on the Sunday plus 90 and 200 in the
	// week: 680 minutes, 11.33 hours, over five worked days.
	if d := byEmail["farid@example.com"]; d.TotalHours != 11.33 {
		t.Errorf("lumbini total hours = %v, want 11.33", d.TotalHours)
	}
	if d := byEmail["farid@example.com"]; d.AvgTimePerDay != 2.27 {
		t.Errorf("lumbini avg hrs/day = %v, want 2.27 (11.33 over 5 days)", d.AvgTimePerDay)
	}
	// mihaly logged 60 minutes and lost the Wednesday to leave, so the
	// average is over four days, not five.
	if d := byEmail["bruno@example.com"]; d.AvgTimePerDay != 0.25 {
		t.Errorf("mihaly avg hrs/day = %v, want 0.25 (1.00 over 4 days)", d.AvgTimePerDay)
	}
	// Average people per day counts AVAILABILITY, not activity:
	// summerizeTimeReport pushes one entry per (member, day still in
	// teamInfo). Five members are available all five weekdays and
	// bruno@ four, so 29 member-days, over a span of 6 -- ServiceNow
	// divides by getRoundedDayPart(from, to), the gap between Monday and
	// Sunday, not by the seven days of the range nor its five weekdays.
	if got.Totals.AveragePeoplePerDay != 4.83 {
		t.Errorf("average people per day = %v, want 4.83 (29 member-days over a span of 6)",
			got.Totals.AveragePeoplePerDay)
	}
	// 2145 minutes in all.
	if got.Totals.TotalHours != 35.75 {
		t.Errorf("weekly total hours = %.2f, want 35.75", got.Totals.TotalHours)
	}
	if got.Totals.AvgHoursPerPerson != 5.96 {
		t.Errorf("weekly avg hrs/person = %.2f, want 5.96", got.Totals.AvgHoursPerPerson)
	}
	// Ten issues on the Sunday plus eight more across the week.
	if got.Totals.Issues != 18 {
		t.Errorf("weekly issues = %d, want 18", got.Totals.Issues)
	}

	// --- Lead exclusion, the weekly report's defining quirk -----------
	// ServiceNow's nMembers_Cal / time_exclude_lead: the per-team average
	// drops leads from BOTH the divisor and the hours. Americas has two
	// members -- asha@ who leads it and logged 5.92 hours, and bruno@
	// who logged 1.00 -- so the average is 1.00 over one person, not
	// 6.92 over two.
	byTeam := map[string]domain.WorklogTeamReport{}
	for _, tr := range got.Teams {
		byTeam[tr.TeamName] = tr
	}
	americas := byTeam["Americas CS Team - Integraion"]
	if americas.Members != 2 {
		t.Errorf("Americas members = %d, want 2 (leads still COUNT as members)", americas.Members)
	}
	if americas.MembersForAverage != 1 {
		t.Errorf("Americas membersForAverage = %v, want 1 (nMembers - 1, flat)", americas.MembersForAverage)
	}
	if americas.TotalHours != 6.92 {
		t.Errorf("Americas total hours = %.2f, want 6.92 (the lead's hours still COUNT)", americas.TotalHours)
	}
	if americas.AvgTotalHoursPerPerson != 1.00 {
		t.Errorf("Americas avg total hrs/person = %.2f, want 1.00 (mihaly's 1.00 over one non-lead)",
			americas.AvgTotalHoursPerPerson)
	}

	// Castor is dara@ alone, and they lead it. Excluding the only
	// member leaves nothing to divide by -- this is the row that prints
	// NaN and Infinity in the delivered mail. We report 0.00 instead,
	// the one place we knowingly differ.
	castor := byTeam["Castor"]
	if castor.MembersForAverage != 0 {
		t.Errorf("Castor membersForAverage = %v, want 0 (one member, less one)", castor.MembersForAverage)
	}
	if castor.AvgTotalHoursPerPerson != 0 {
		t.Errorf("Castor avg = %v, want 0 rather than Infinity", castor.AvgTotalHoursPerPerson)
	}
}

// The daily report does NOT exclude leads: its delivered mail divides
// Americas' 1.42 hours by both members for 0.71, which no exclusion
// produces. The two cadences really do differ here, so assert it.
func TestWorklogDailyDoesNotExcludeLeads(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)
	svc := service.NewWorklogReportService(
		repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, nil)
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	got, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily, From: day, To: day})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	// Nobody the delivered mail lists is a lead, so presence equals the
	// member count and nothing is deducted.
	for _, tr := range got.Teams {
		if tr.MembersForAverage != float64(tr.Members) {
			t.Errorf("%s: membersForAverage = %v, want %d -- none of these six is a lead",
				tr.TeamName, tr.MembersForAverage, tr.Members)
		}
	}

	// Marking one of them a lead must drop both the divisor and their
	// hours, which is what summerizeTimeReport's nMembers_Cal and
	// time_wl do on the daily report too.
	if _, err := pool.Exec(ctx,
		`UPDATE team_member SET role = 'lead' WHERE user_id = $1`,
		"00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("mark lead: %v", err)
	}
	withLead, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily, From: day, To: day})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	for _, tr := range withLead.Teams {
		if tr.TeamName != "Americas CS Team - Integraion" {
			continue
		}
		if tr.MembersForAverage != 1 {
			t.Errorf("Americas membersForAverage = %v, want 1 (two present, one now a lead)",
				tr.MembersForAverage)
		}
		// asha@ held all 1.42 of the team's hours, so excluding them
		// leaves bruno@'s nothing.
		if tr.AvgTotalHoursPerPerson != 0 {
			t.Errorf("Americas avg = %v, want 0.00 once the only contributor is excluded",
				tr.AvgTotalHoursPerPerson)
		}
	}
}

func svcWeekly(ctx context.Context, pool *pgxpool.Pool) (domain.WorklogReport, error) {
	svc := service.NewWorklogReportService(
		repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, nil)
	return svc.BuildReport(repository.WithSystemIdentity(ctx), domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceWeekly,
		From:    time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC),
	})
}

// weekdayWorklogEntry is one extra time card seeded into the working week
// so the weekly report differs from the daily one in substance, not just
// in which columns it prints.
type weekdayWorklogEntry struct {
	userIdx  int
	date     string
	key      string
	itemType string
	minutes  int
}

var worklogWeekdayFixture = []weekdayWorklogEntry{
	{1, "2026-10-12", "ACME-520", "INCIDENT", 120},
	{3, "2026-10-12", "BOREAL-330", "CASE", 60},
	{5, "2026-10-13", "DELTA-2160", "INCIDENT", 180},
	{6, "2026-10-13", "FABLE-350", "CASE", 90},
	// Deliberately not dara@ (index 4): that person logs nothing all
	// week, which is what makes the worked-days assertion meaningful.
	{5, "2026-10-14", "IRIS-520", "SERVICE_REQUEST", 240},
	{1, "2026-10-15", "ACME-521", "CASE", 150},
	{2, "2026-10-15", "JUNO-25", "INCIDENT", 60},
	{6, "2026-10-16", "HAVEN-310", "INCIDENT", 200},
}

func seedWorklogWeekdayTime(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for i, e := range worklogWeekdayFixture {
		userID := fmt.Sprintf("00000000-0000-0000-0000-00000000%04d", e.userIdx)
		itemID := fmt.Sprintf("00000000-0000-0000-0000-9999%08d", i+1)
		if _, err := pool.Exec(ctx,
			`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by,
			   number, wso2_id, subject, type)
			 VALUES ($1, NOW(), NOW(), 'seed', 'seed', $2, $2, $2, $3::work_item_type_enum)`,
			itemID, e.key, e.itemType); err != nil {
			t.Fatalf("seed weekday work item %s: %v", e.key, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
			   case_id, user_id, work_date, state, analyzing_minutes)
			 VALUES (gen_random_uuid(), NOW(), NOW(), 'seed', 'seed', $1, $2, $3::date, 'SUBMITTED', $4)`,
			itemID, userID, e.date, e.minutes); err != nil {
			t.Fatalf("seed weekday time card %s: %v", e.key, err)
		}
	}
}

// The two reports must not share an audience. The delivered weekly mail
// goes to management and the org-wide list only; the delivered daily mail
// adds every delivery team's lead and their per-area lists. A change that
// quietly unified them would mail eleven team leads a weekly report they
// have never received, so this asserts the difference directly rather
// than only asserting each list.
func TestWorklogWeeklyAndDailyAudiencesDiffer(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)

	ctx = repository.WithSystemIdentity(ctx)
	svc := service.NewWorklogReportService(
		repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, nil)
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)

	daily, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily, From: day, To: day})
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	weekly, err := svcWeekly(ctx, pool)
	if err != nil {
		t.Fatalf("weekly: %v", err)
	}

	// Weekly: management only, and the org-wide list only.
	wantWeeklyTo := "mara@example.com,nils@example.com"
	if got := strings.Join(weekly.Recipients.To, ","); got != wantWeeklyTo {
		t.Errorf("weekly To = %q, want %q (management only, no team leads)", got, wantWeeklyTo)
	}
	wantWeeklyCc := "leads-list@example.com"
	if got := strings.Join(weekly.Recipients.Cc, ","); got != wantWeeklyCc {
		t.Errorf("weekly Cc = %q, want %q", got, wantWeeklyCc)
	}

	// No team lead may appear on the weekly audience.
	for _, lead := range []string{"americaslead@example.com", "atlaslead@example.com",
		"castorlead@example.com", "dracolead@example.com", "vegalead@example.com"} {
		for _, got := range weekly.Recipients.To {
			if got == lead {
				t.Errorf("%s is a team lead and must not receive the weekly report", lead)
			}
		}
	}

	// Daily must be strictly larger, and must still carry management.
	if len(daily.Recipients.To) <= len(weekly.Recipients.To) {
		t.Errorf("daily To (%d) must be larger than weekly To (%d)",
			len(daily.Recipients.To), len(weekly.Recipients.To))
	}
	if strings.Join(daily.Recipients.To, ",") == strings.Join(weekly.Recipients.To, ",") {
		t.Error("the daily and weekly audiences are identical; they must differ")
	}
	for _, mgr := range []string{"mara@example.com", "nils@example.com"} {
		if !strings.Contains(strings.Join(daily.Recipients.To, ","), mgr) {
			t.Errorf("%s is management and must receive the daily report too", mgr)
		}
	}
}

// Leave and exclusions must apply on a WEEKEND daily report. Worked days
// count only weekdays, so a weekend absence adds nothing to the weekly
// total -- but the daily report runs every day, and the mail it is
// modelled on is a Sunday. An earlier version tested for a weekend
// before deciding whether an absence covered the reported day, so a
// Saturday or Sunday report applied no leave and honoured no exclusion:
// somebody away, or explicitly excluded, appeared as present.
func TestWorklogDailyAppliesLeaveOnAWeekend(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)

	// 2026-10-18 is a Sunday, the day the delivered daily mail covers.
	const sunday = "2026-10-18"
	kind := func(code string) string {
		var id string
		if err := pool.QueryRow(ctx,
			`SELECT id::text FROM team_schedule_absence_kind WHERE code = $1`, code).Scan(&id); err != nil {
			t.Fatalf("kind %s: %v", code, err)
		}
		return id
	}
	absence := func(userID, teamKey, code string) {
		if _, err := pool.Exec(ctx,
			`INSERT INTO team_schedule_absence (user_id, team_key, starts_on, ends_on, kind_id)
			 VALUES ($1, $2, $3::date, $3::date, $4)`,
			userID, teamKey, sunday, kind(code)); err != nil {
			t.Fatalf("seed absence %s: %v", code, err)
		}
	}
	// chen@ (Atlas) is on annual leave; dara@ (Castor) is
	// excluded from the rota outright.
	absence("00000000-0000-0000-0000-000000000003", "atlas", "ANNUAL_LEAVE")
	absence("00000000-0000-0000-0000-000000000004", "castor", "EXCLUDED")

	svc := service.NewWorklogReportService(
		repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, nil)
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	got, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily, From: day, To: day})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}

	var sawLeave, sawExcluded bool
	for _, tr := range got.Teams {
		for _, d := range tr.Developers {
			switch d.Email {
			case "chen@example.com":
				sawLeave = true
				if d.Leave == "" {
					t.Error("chen@ is on annual leave on this Sunday and must be marked")
				}
			case "dara@example.com":
				sawExcluded = true
			}
		}
	}
	if !sawLeave {
		t.Error("chen@ is on leave, not absent from the report")
	}
	if sawExcluded {
		t.Error("dara@ carries an EXCLUDED absence and must not appear at all")
	}
	// One of the six is excluded outright, so five remain; the one on
	// full-day leave counts as 0 present, leaving four.
	if got.Totals.Members != 5 {
		t.Errorf("Total Members = %d, want 5 (one of the six excluded)", got.Totals.Members)
	}
	if got.Totals.NonLeaveMembers != 4 {
		t.Errorf("Non Leave Members = %v, want 4 (five present, one on full-day leave)",
			got.Totals.NonLeaveMembers)
	}
}

// *** THE 791-AGAINST-574 GAP, REPRODUCED. ***
//
// ServiceNow increments nCases once per case but caseTypes[type] once
// per matching DATE, so a case worked on more than one day adds 1 to
// the issue total and one per day to its type bucket. The delivered
// weekly mail's type counts sum to 791 against its stated 574 for that
// reason alone, and the daily mail shows no gap only because one day
// cannot contain a second.
//
// The fixture elsewhere gives every case a single day, so the two
// counters agree and the behaviour goes untested. This logs a second
// day against a case that already has one.
func TestWorklogTypeCountsAreCaseDaysNotCases(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)

	svc := service.NewWorklogReportService(
		repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, nil)
	week := func() domain.WorklogReport {
		r, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
			Cadence: domain.WorklogReportCadenceWeekly,
			From:    time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
			To:      time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			t.Fatalf("BuildReport: %v", err)
		}
		return r
	}
	sum := func(r domain.WorklogReport) int {
		n := 0
		for _, b := range r.ByType {
			n += b.Issues
		}
		return n
	}

	before := week()
	if sum(before) != before.Totals.Issues {
		t.Fatalf("with one day per case the counters should agree: byType %d, issues %d",
			sum(before), before.Totals.Issues)
	}

	// A SECOND day on a case asha@ already logged against on the 18th.
	if _, err := pool.Exec(ctx,
		`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
		   case_id, user_id, work_date, state, analyzing_minutes)
		 SELECT gen_random_uuid(), NOW(), NOW(), 'seed', 'seed',
		        wi.id, $1, DATE '2026-10-16', 'SUBMITTED', 60
		   FROM work_item wi WHERE wi.wso2_id = 'ACME-515'`,
		"00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("seed second day: %v", err)
	}

	after := week()
	if after.Totals.Issues != before.Totals.Issues {
		t.Errorf("issues = %d, want %d unchanged -- a second day is not a second issue",
			after.Totals.Issues, before.Totals.Issues)
	}
	if sum(after) != sum(before)+1 {
		t.Errorf("byType = %d, want %d -- the second day is a second case-day",
			sum(after), sum(before)+1)
	}
	if sum(after) == after.Totals.Issues {
		t.Error("byType still reconciles with the total; the case-day counter is not case-days")
	}
}

// Which teams appear is deployment configuration. The type filter is
// broad -- 'cre' catches approval and review teams alongside the real
// delivery ones -- so a deployment must be able to drop particular
// teams without narrowing the types until they are brittle.
//
// An excluded team must vanish completely: no section, no members in
// the totals, and no lead on the derived audience.
func TestWorklogExcludedTeamsDisappearEntirely(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)

	build := func(excluded []string) domain.WorklogReport {
		svc := service.NewWorklogReportService(
			repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, excluded)
		r, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
			Cadence: domain.WorklogReportCadenceDaily, From: day, To: day})
		if err != nil {
			t.Fatalf("BuildReport: %v", err)
		}
		return r
	}

	before := build(nil)
	after := build([]string{"draco", "vega"})

	if len(after.Teams) != len(before.Teams)-2 {
		t.Errorf("teams = %d, want %d -- two were excluded", len(after.Teams), len(before.Teams)-2)
	}
	for _, tr := range after.Teams {
		if tr.TeamName == "Draco" || tr.TeamName == "Vega" {
			t.Errorf("%s is excluded and must not have a section", tr.TeamName)
		}
	}
	// Their members leave the totals with them.
	if after.Totals.Members >= before.Totals.Members {
		t.Errorf("members = %d, want fewer than %d", after.Totals.Members, before.Totals.Members)
	}
	// And their leads leave the derived audience.
	for _, addr := range after.Recipients.To {
		if addr == "dracolead@example.com" || addr == "vegalead@example.com" {
			t.Errorf("%s leads an excluded team and must not be a recipient", addr)
		}
	}
	if len(after.Recipients.To) >= len(before.Recipients.To) {
		t.Errorf("derived To = %d, want fewer than %d", len(after.Recipients.To), len(before.Recipients.To))
	}
}

// A WEEKDAY daily report is not rota-gated at all. This is the one rule
// the two cadences do not share, and getting it wrong is invisible --
// the report still renders, just short.
//
// ServiceNow's DailyTimeReportUtil_V2.__internal_getTeamInfo starts from
// every member of the operational groups and narrows to the shift crew
// only inside an isWeekend branch, so a Wednesday report lists the whole
// roster and most of it shows zeros. An earlier version of this port
// applied the rota filter on every day of the week, which silently
// dropped whoever the rota did not cover: on staging for Wednesday
// 2026-10-21, 64 of team cre-abt's 98 members.
//
// The fixture rosters its six people for Sunday 2026-10-18 only, so
// nobody has a rota row on the Wednesday three days earlier. Under the
// old rule that report was empty; under ServiceNow's it is everyone.
func TestWorklogWeekdayDailyIgnoresTheRota(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)
	svc := service.NewWorklogReportService(
		repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, nil)

	// 2026-10-14 is a Wednesday. No fixture rota row falls on it.
	wednesday := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	weekday, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily, From: wednesday, To: wednesday})
	if err != nil {
		t.Fatalf("weekday daily: %v", err)
	}

	// Sunday, the same fixture, where the rota does apply.
	sunday := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	weekend, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily, From: sunday, To: sunday})
	if err != nil {
		t.Fatalf("weekend daily: %v", err)
	}

	if weekday.Totals.Members == 0 {
		t.Fatal("the weekday daily report is empty; it must list the roster " +
			"regardless of the rota")
	}
	if weekday.Totals.Members < weekend.Totals.Members {
		t.Errorf("weekday members = %d, weekend members = %d: the weekday report "+
			"must not be narrower than the weekend crew",
			weekday.Totals.Members, weekend.Totals.Members)
	}

	// Nobody logged time on the Wednesday, so every member of it is
	// there on roster membership alone -- precisely the people the old
	// rule dropped.
	if weekday.Totals.Issues != 0 {
		t.Errorf("TotalIssues = %d, want 0 (the fixture logs no time that day)",
			weekday.Totals.Issues)
	}

	// The head of function stays out: dropping the rota gate must not
	// quietly readmit a role the roster query excludes.
	for _, team := range weekday.Teams {
		for _, dev := range team.Developers {
			if dev.Email == "headof@example.com" {
				t.Error("headof@example.com holds cs_head and must appear in no team section")
			}
		}
	}
}

// On a weekend, being on the rota is not enough -- the SHIFT has to be a
// weekend one. ServiceNow matched cmn_schedule_span rows of type
// ops_weekend / ops_weekend_night, never any span at all, and the
// distinction is live in this schema: on staging, SRE_TZ3 is a WEEKDAY
// shift that still carried 28 weekend assignments over September and
// October. Gating on "has a rota row" would put all of them in the
// Sunday mail.
func TestWorklogWeekendDailyTakesWeekendShiftsOnly(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	seedWorklogDailyFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}

	// A seventh person, on the roster of a fixture team, rostered that
	// Sunday on a WEEKDAY-scoped shift. They logged nothing and took no
	// leave, so the rota row is the only thing that could list them.
	exec(`INSERT INTO team_schedule_shift (id, code, short_code, label, colour_token, family, start_minute, end_minute, day_scope)
	      VALUES ('00000000-0000-0000-0000-0000000000fe', 'WKDY', 'WD', 'Weekday', 'WD', 'SRE', 540, 1080, 'WEEKDAY')
	      ON CONFLICT (id) DO UPDATE SET day_scope = EXCLUDED.day_scope`)
	exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email)
	      VALUES ('00000000-0000-0000-0000-0000000000fd', NOW(), NOW(),
	              'weekdayshift@example.com', 'Weekday Shift', 'weekdayshift@example.com')`)
	exec(`INSERT INTO team_member (id, created_on, updated_on, team_id, user_id, role)
	      VALUES (gen_random_uuid(), NOW(), NOW(),
	              (SELECT id FROM team WHERE key = 'americas'),
	              '00000000-0000-0000-0000-0000000000fd', 'engineer')`)
	exec(`INSERT INTO team_schedule_assignment (user_id, team_key, shift_id, rota_date, starts_at, ends_at)
	      VALUES ('00000000-0000-0000-0000-0000000000fd', 'americas',
	              '00000000-0000-0000-0000-0000000000fe', DATE '2026-10-18',
	              TIMESTAMPTZ '2026-10-18 09:00+05:30', TIMESTAMPTZ '2026-10-18 18:00+05:30')`)

	svc := service.NewWorklogReportService(
		repository.NewWorklogReportRepository(repository.NewScoped(pool)), nil, nil)
	sunday := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	report, err := svc.BuildReport(ctx, domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily, From: sunday, To: sunday})
	if err != nil {
		t.Fatalf("weekend daily: %v", err)
	}

	for _, team := range report.Teams {
		for _, dev := range team.Developers {
			if dev.Email == "weekdayshift@example.com" {
				t.Error("a WEEKDAY-scoped shift is not weekend duty; " +
					"weekdayshift@example.com must not appear in a Sunday report")
			}
		}
	}
	if report.Totals.Members != 6 {
		t.Errorf("Members = %d, want 6 (the weekend crew, unchanged by the extra rota row)",
			report.Totals.Members)
	}
}
