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

// These tests exercise the real SQL in schedule_repo.go against a live
// PostgreSQL instance.
//
// The service-level tests run against an in-memory fake, so everything that
// makes this feature work is invisible to them: the GiST range index that
// answers who is on call at an instant, the absence exclusion that keeps
// somebody on leave out of that answer, the day-scope skip that lets a
// weekday rotation cross a weekend, and the trim-and-split that shortens a
// stretch of leave instead of cancelling it. None of that has behaviour a
// fake can reproduce -- it is all SQL -- and this file is where it is checked.
//
// Skipped unless ENTITY_TEST_DATABASE_URL is set, so `go test ./...` on a
// machine with no database stays green. Apply every migration in order first;
// the tables this file reads are created by 0152-0155:
//
//	createdb entity_test
//	for f in migrations/*.sql; do psql -v ON_ERROR_STOP=1 -d entity_test -f "$f"; done
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestScheduleIntegration ./internal/repository/

package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Fixture ids, distinct from every other integration file's so the two can run
// in the same database without clearing each other's rows.
const (
	schedTeamID   = "5c8e0000-0000-4000-8000-000000000001"
	schedLeadID   = "5c8e0000-0000-4000-8000-000000000002"
	schedMemberID = "5c8e0000-0000-4000-8000-000000000003"
	schedOtherID  = "5c8e0000-0000-4000-8000-000000000004"
	// A team in the other family, and somebody holding the CRE rota admin
	// role, so the family split can be tested rather than assumed.
	schedSreTeamID = "5c8e0000-0000-4000-8000-000000000005"
	schedAdminID   = "5c8e0000-0000-4000-8000-000000000006"
	// schedOtherTeam was only ever a string in a WHERE clause until 000101
	// gave team_schedule_assignment.team_key a foreign key into team(key).
	// It needs a real row now, or anything seeding a slot on it fails.
	schedOtherTeamID = "5c8e0000-0000-4000-8000-000000000007"
	// Holds the CRE rota admin role but is not internal staff, so it can be
	// shown that the role alone grants nothing.
	schedOutsiderID = "5c8e0000-0000-4000-8000-000000000008"

	schedLeadEmail     = "sched.lead@example.test"
	schedMemberEmail   = "sched.member@example.test"
	schedOtherEmail    = "sched.other@example.test"
	schedAdminEmail    = "sched.admin@example.test"
	schedOutsiderEmail = "sched.outsider@example.test"
	schedTeamKey       = "schedfixture"
	schedOtherTeam     = "schedother"
	schedSreTeamKey    = "schedsrefixture"
	schedLeadershipKey = "schedleadership"
	schedBoardKey      = "schedboard"

	// A Monday, so the weekday/weekend arithmetic below reads plainly.
	schedMonday = "2026-09-21"
)

// newScheduleIntegrationRepo connects, rebuilds this file's fixtures from
// scratch so the tests are order-independent and rerunnable, and returns a
// repository over a real pool.
func newScheduleIntegrationRepo(t *testing.T) (ScheduleRepository, *pgxpool.Pool) {
	t.Helper()

	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Torn down first, not after: a test that fails half way should still
	// leave the next run a clean slate.
	for _, stmt := range []string{
		`DELETE FROM team_schedule_assignment_activity WHERE team_key IN ($1, $2)`,
		`DELETE FROM team_schedule_absence_activity WHERE team_key IN ($1, $2)`,
		`DELETE FROM team_schedule_assignment WHERE team_key IN ($1, $2)`,
		`DELETE FROM team_schedule_absence WHERE team_key IN ($1, $2)`,
	} {
		mustExec(t, pool, stmt, schedTeamKey, schedOtherTeam)
	}
	mustExec(t, pool, `DELETE FROM team_member WHERE user_id IN ($1, $2, $3, $4)`,
		schedLeadID, schedMemberID, schedOtherID, schedAdminID)
	mustExec(t, pool, `DELETE FROM user_role WHERE user_id IN ($1, $2)`, schedAdminID, schedOutsiderID)

	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $2, $2, 'cre-abt')
		ON CONFLICT (id) DO NOTHING`, schedTeamID, schedTeamKey)
	// The same fixture in the other family. Without it, "a CRE admin does not
	// reach SRE" could only be asserted against teams this file does not own,
	// which is a test that passes for the wrong reason on an empty database.
	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $2, $2, 'sre-abt')
		ON CONFLICT (id) DO NOTHING`, schedSreTeamID, schedSreTeamKey)
	// The second CRE team, for the tests that check one team's edit does not
	// reach another's rows.
	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $2, $2, 'cre-abt')
		ON CONFLICT (id) DO NOTHING`, schedOtherTeamID, schedOtherTeam)

	// Two of these deliberately share a display name. A rota can carry two
	// people called the same thing, and the history has to survive it.
	for _, u := range []struct{ id, email, first, last string }{
		{schedLeadID, schedLeadEmail, "Sched", "Lead"},
		{schedMemberID, schedMemberEmail, "Chamara", "Perera"},
		{schedOtherID, schedOtherEmail, "Chamara", "Perera"},
		{schedAdminID, schedAdminEmail, "Sched", "Admin"},
		{schedOutsiderID, schedOutsiderEmail, "Sched", "Outsider"},
	} {
		mustExec(t, pool, `
			INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
			                    user_name, first_name, last_name, email)
			VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $2, $3, $4, $2)
			ON CONFLICT (id) DO UPDATE SET first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name`,
			u.id, u.email, u.first, u.last)
	}

	mustExec(t, pool, `
		INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', $1, $2, 'lead')`,
		schedTeamID, schedLeadID)
	mustExec(t, pool, `
		INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', $1, $2, $3)`,
		schedTeamID, schedMemberID, ordinaryMemberRole(t, pool))

	// The rota admin gets the role and NO team_member row at all -- that
	// combination is the whole point of the grant, and a fixture that also
	// made them a lead somewhere would not be testing it.
	//
	// The role row comes from migration 0156. Resolved by name rather than
	// by a fixed id, because that migration creates it with gen_random_uuid()
	// and the ServiceNow sync may have seeded its own under a different id.
	//
	// A rota admin is internal staff first: the role is a schedule permission
	// for someone who already is, and never makes anyone internal. So the
	// admin also gets 'internal', which a database built from migrations alone
	// does not have -- the ServiceNow sync seeds it -- hence the insert.
	// The outsider gets the rota admin role and nothing else.
	mustExec(t, pool, `
		INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name, description)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', 'internal', 'Internal staff')
		ON CONFLICT (name) DO NOTHING`)
	mustExec(t, pool, `
		INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
		SELECT gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', $1, r.id
		  FROM role r WHERE r.name IN ('cre_rota_admin', 'internal')`, schedAdminID)
	mustExec(t, pool, `
		INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
		SELECT gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', $1, r.id
		  FROM role r WHERE r.name = 'cre_rota_admin'`, schedOutsiderID)

	// And torn down after, too: these tests run against a developer's local
	// database, where a fixture team left behind turns up on the real rota
	// page -- its lead heading a team called "schedfixture".
	t.Cleanup(func() { removeScheduleFixtures(t, pool) })

	return NewScheduleRepository(pool), pool
}

// removeScheduleFixtures deletes every row the fixtures above create: their
// rota history, assignments and absences, memberships, role grants, users and
// teams, children first.
func removeScheduleFixtures(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	teams := []string{schedTeamKey, schedOtherTeam, schedSreTeamKey, schedLeadershipKey, schedBoardKey}
	users := []string{schedLeadID, schedMemberID, schedOtherID, schedAdminID, schedOutsiderID}
	for _, stmt := range []string{
		`DELETE FROM team_schedule_assignment_activity WHERE team_key = ANY($1) OR user_id = ANY($2::uuid[])`,
		`DELETE FROM team_schedule_absence_activity WHERE team_key = ANY($1) OR user_id = ANY($2::uuid[])`,
		`DELETE FROM team_schedule_assignment WHERE team_key = ANY($1) OR user_id = ANY($2::uuid[])`,
		`DELETE FROM team_schedule_absence WHERE team_key = ANY($1) OR user_id = ANY($2::uuid[])`,
		`DELETE FROM team_member WHERE user_id = ANY($2::uuid[]) OR team_id IN (SELECT id FROM team WHERE key = ANY($1))`,
		`DELETE FROM user_role WHERE user_id = ANY($2::uuid[]) AND cardinality($1::text[]) >= 0`,
		`DELETE FROM "user" WHERE id = ANY($2::uuid[]) AND cardinality($1::text[]) >= 0`,
		`DELETE FROM team WHERE key = ANY($1) AND cardinality($2::uuid[]) >= 0`,
	} {
		mustExec(t, pool, stmt, teams, users)
	}
}

// ordinaryMemberRole returns a team_member.role this database will accept for
// somebody who is on a team but does not lead it.
//
// Asked rather than hardcoded because the vocabulary is being changed by work
// in flight: 0034 constrains it to ('member', 'lead'), and the escalation
// roster branch replaces 'member' with a rung set of its own
// ('engineer', 'sub_lead', 'lead', 'cre_head', 'cs_head'). A fixture naming
// either one fails outright against the other's schema -- and it fails in
// setUp, so every test in this file goes red at once and none of them is
// about team_member roles at all. 'lead' is the only value common to both,
// and it is the one value this fixture must not use.
//
// Nothing here depends on which name comes back: these tests only ever assert
// that a non-lead is not treated as a lead.
func ordinaryMemberRole(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var def string
	err := pool.QueryRow(context.Background(), `
		SELECT pg_get_constraintdef(oid) FROM pg_constraint
		 WHERE conname = 'team_member_role_check'`).Scan(&def)
	if err != nil {
		// No such constraint: nothing is restricting the column, so the
		// original value is as good as any.
		return "member"
	}
	if strings.Contains(def, `'member'`) {
		return "member"
	}
	return "engineer"
}

// weekdayShift returns a shift code the catalogue actually has for the given
// day scope, so these tests bind to the seeded catalogue rather than inventing
// codes that a later migration might rename.
func shiftWithScope(t *testing.T, pool *pgxpool.Pool, family, scope string) string {
	t.Helper()
	var code string
	err := pool.QueryRow(context.Background(), `
		SELECT code FROM team_schedule_shift
		 WHERE family::text = $1 AND day_scope::text = $2
		 ORDER BY sort_order LIMIT 1`, family, scope).Scan(&code)
	if err != nil {
		t.Fatalf("no %s %s shift in the catalogue: %v", family, scope, err)
	}
	return code
}

func countRows(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// ── the catalogue ─────────────────────────────────────────────────────────

func TestScheduleIntegration_CatalogueServesAllThreeParts(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)

	cat, err := repo.Catalogue(context.Background())
	if err != nil {
		t.Fatalf("Catalogue: %v", err)
	}
	// One payload because a client needs all of it to draw a single day.
	if len(cat.Zones) == 0 || len(cat.Shifts) == 0 || len(cat.AbsenceKinds) == 0 {
		t.Fatalf("got %d zones, %d shifts, %d absence kinds; want all three populated",
			len(cat.Zones), len(cat.Shifts), len(cat.AbsenceKinds))
	}

	// The teams too, since the frontend no longer holds that list: if this
	// comes back empty the team picker renders empty and every team draws in
	// the same fallback grey.
	var fixture *domain.ScheduleTeam
	for i := range cat.Teams {
		if cat.Teams[i].Key == schedTeamKey {
			fixture = &cat.Teams[i]
		}
	}
	if fixture == nil {
		t.Fatalf("the fixture team is missing from %d served teams", len(cat.Teams))
	}
	if fixture.Family != "CRE" {
		t.Fatalf("fixture team family %q, want CRE from its type", fixture.Family)
	}
	// Positions are what colour a team, so they have to be 1-based and
	// distinct -- a zero or a repeat puts two teams in one colour.
	seen := map[int]string{}
	for _, tm := range cat.Teams {
		if tm.SortOrder < 1 {
			t.Fatalf("team %s has sortOrder %d, want 1 or more", tm.Key, tm.SortOrder)
		}
		if other, dup := seen[tm.SortOrder]; dup {
			t.Fatalf("teams %s and %s share sortOrder %d", other, tm.Key, tm.SortOrder)
		}
		seen[tm.SortOrder] = tm.Key
	}
}

// Rotas (migrations 0199-0200): the catalogue names SRE's SaaS and IaaS and
// the SME product rotations, ties each zone to its rota, and reads a team's
// rota -- and the SME family -- from its type, the way family has always been
// read. SaaS SRE's own zones and teams must come out on the SaaS rota, so the
// rota a live team is on is exactly the one it was on before.
func TestScheduleIntegration_CatalogueServesRotas(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	const smeTeamID, smeTeamKey = "8c1f6d3e-5b7a-4e2c-9a0d-3f4e5d6c7b8a", "fixture-sme-moesif"
	mustExec(t, pool, `DELETE FROM team WHERE id = $1`, smeTeamID)
	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $2, $2, 'SME-Moesif')`, smeTeamID, smeTeamKey)
	t.Cleanup(func() { mustExec(t, pool, `DELETE FROM team WHERE id = $1`, smeTeamID) })

	cat, err := repo.Catalogue(ctx)
	if err != nil {
		t.Fatalf("Catalogue: %v", err)
	}

	rotas := map[string]domain.ScheduleRota{}
	for _, ro := range cat.Rotas {
		rotas[ro.Code] = ro
	}
	for code, family := range map[string]string{"SRE_SAAS": "SRE", "SRE_IAAS": "SRE", "SME_ASGARDEO": "SME", "SME_MOESIF": "SME"} {
		if rotas[code].Family != family {
			t.Fatalf("rota %s: family %q, want %q (served rotas: %v)", code, rotas[code].Family, family, cat.Rotas)
		}
	}
	if m := rotas["SME_MOESIF"].EscalationMinutes; m == nil || *m != 30 {
		t.Fatalf("Moesif escalates after %v minutes, want 30", m)
	}
	if rotas["SME_ASGARDEO"].Rotates != "DAILY" || rotas["SME_MOESIF"].Rotates != "WEEKLY" {
		t.Fatalf("rotation frequency: Asgardeo %q, Moesif %q", rotas["SME_ASGARDEO"].Rotates, rotas["SME_MOESIF"].Rotates)
	}

	zoneRota := map[string]string{}
	for _, z := range cat.Zones {
		if z.RotaCode != nil {
			zoneRota[z.Code] = *z.RotaCode
		}
	}
	for zone, rota := range map[string]string{"TZ1": "SRE_SAAS", "TZ3": "SRE_SAAS", "IAAS_D": "SRE_IAAS", "MOE_N": "SME_MOESIF"} {
		if zoneRota[zone] != rota {
			t.Fatalf("zone %s is on rota %q, want %q", zone, zoneRota[zone], rota)
		}
	}

	var night *domain.ScheduleShift
	for i := range cat.Shifts {
		if cat.Shifts[i].Code == "SME_MOE_NIGHT" {
			night = &cat.Shifts[i]
		}
	}
	if night == nil || night.Family != "SME" || night.ZoneCode == nil || *night.ZoneCode != "MOE_N" ||
		night.StartMinute != 1320 || night.EndMinute != 2040 || !night.IsEscalation || night.DayScope != "ANY" {
		t.Fatalf("Moesif night window = %+v, want SME, zone MOE_N, 22:00-10:00 every day, escalation", night)
	}

	teams := map[string]domain.ScheduleTeam{}
	for _, tm := range cat.Teams {
		teams[tm.Key] = tm
	}
	if tm := teams[smeTeamKey]; tm.Family != "SME" || tm.RotaCode == nil || *tm.RotaCode != "SME_MOESIF" {
		t.Fatalf("an SME-Moesif team reads as family %q, rota %v; want SME on SME_MOESIF", tm.Family, tm.RotaCode)
	}
	if tm := teams[schedSreTeamKey]; tm.Family != "SRE" || tm.RotaCode == nil || *tm.RotaCode != "SRE_SAAS" {
		t.Fatalf("an sre-abt team reads as family %q, rota %v; want SRE on SRE_SAAS", tm.Family, tm.RotaCode)
	}
	if tm := teams[schedTeamKey]; tm.Family != "CRE" || tm.RotaCode != nil {
		t.Fatalf("a cre-abt team reads as family %q, rota %v; want CRE on no named rota", tm.Family, tm.RotaCode)
	}
}

// ── who may edit ──────────────────────────────────────────────────────────

func TestScheduleIntegration_LeadsTeamIsTrueOnlyForTheLead(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	lead, err := repo.LeadsTeam(ctx, schedLeadEmail, schedTeamKey)
	if err != nil {
		t.Fatalf("LeadsTeam(lead): %v", err)
	}
	if !lead {
		t.Fatal("the team's lead was not recognised as its lead")
	}

	// A member of the same team is not a lead of it: the whole edit
	// permission rests on this distinction.
	member, err := repo.LeadsTeam(ctx, schedMemberEmail, schedTeamKey)
	if err != nil {
		t.Fatalf("LeadsTeam(member): %v", err)
	}
	if member {
		t.Fatal("an ordinary member was reported as leading the team")
	}

	// Leading one team says nothing about another.
	elsewhere, err := repo.LeadsTeam(ctx, schedLeadEmail, schedOtherTeam)
	if err != nil {
		t.Fatalf("LeadsTeam(other team): %v", err)
	}
	if elsewhere {
		t.Fatal("a lead was reported as leading a team they are not on")
	}
}

func TestScheduleIntegration_LeadTeamsForListsOnlyLedTeams(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	teams, err := repo.LeadTeamsFor(ctx, schedLeadEmail)
	if err != nil {
		t.Fatalf("LeadTeamsFor: %v", err)
	}
	var found bool
	for _, k := range teams {
		if k == schedTeamKey {
			found = true
		}
	}
	if !found {
		t.Fatalf("the lead's own team is missing from %v", teams)
	}

	member, err := repo.LeadTeamsFor(ctx, schedMemberEmail)
	if err != nil {
		t.Fatalf("LeadTeamsFor(member): %v", err)
	}
	for _, k := range member {
		if k == schedTeamKey {
			t.Fatalf("a member was told they lead %s", k)
		}
	}
}

// A rota admin reaches every team in their own family and none in the other.
//
// Asserted by containment rather than by comparing the whole list: the query
// answers for every rostered team in the database, so a seeded database
// legitimately returns more than this file's own fixtures. What must hold is
// that the CRE fixture is in and the SRE fixture is out.
func TestScheduleIntegration_RotaAdminReachesOneFamilyOnly(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	teams, err := repo.RotaAdminTeamsFor(ctx, schedAdminEmail)
	if err != nil {
		t.Fatalf("RotaAdminTeamsFor: %v", err)
	}
	var sawCRE, sawSRE bool
	for _, k := range teams {
		switch k {
		case schedTeamKey:
			sawCRE = true
		case schedSreTeamKey:
			sawSRE = true
		}
	}
	if !sawCRE {
		t.Fatalf("a CRE rota admin did not reach the CRE fixture team; got %v", teams)
	}
	if sawSRE {
		t.Fatalf("a CRE rota admin reached an SRE team; got %v", teams)
	}

	// Holding the role is the whole grant, so somebody without it reaches
	// nothing this way -- including the lead, whose own access comes from
	// team_member and must not leak into this answer.
	lead, err := repo.RotaAdminTeamsFor(ctx, schedLeadEmail)
	if err != nil {
		t.Fatalf("RotaAdminTeamsFor(lead): %v", err)
	}
	if len(lead) != 0 {
		t.Fatalf("a lead with no rota admin role was given %v", lead)
	}
}

// A rota admin is not a lead. LeadsTeam answers about team_member alone, and
// folding the role into it would have made the two indistinguishable -- which
// is exactly what the service needs to keep apart to report a refusal
// accurately.
func TestScheduleIntegration_RotaAdminIsNotReportedAsALead(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	leads, err := repo.LeadsTeam(ctx, schedAdminEmail, schedTeamKey)
	if err != nil {
		t.Fatalf("LeadsTeam(admin): %v", err)
	}
	if leads {
		t.Fatal("a rota admin was reported as leading the team")
	}
	teams, err := repo.LeadTeamsFor(ctx, schedAdminEmail)
	if err != nil {
		t.Fatalf("LeadTeamsFor(admin): %v", err)
	}
	if len(teams) != 0 {
		t.Fatalf("a rota admin was listed as leading %v", teams)
	}
}

// The catalogue lists each team's members with their role, so the roster can
// show every one of them on a month they hold no window -- the lead heading
// the team, the engineer whose only entry was cleared still there to mark.
func TestScheduleIntegration_CatalogueListsEachTeamsMembers(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)
	cat, err := repo.Catalogue(context.Background())
	if err != nil {
		t.Fatalf("Catalogue: %v", err)
	}
	for _, team := range cat.Teams {
		if team.Key != schedTeamKey {
			continue
		}
		roles := map[string]string{}
		for _, m := range team.Members {
			roles[m.UserID] = m.Role
			if m.IsLead != (m.Role == "lead") {
				t.Errorf("member %s: role %q but isLead %v", m.UserID, m.Role, m.IsLead)
			}
		}
		if roles[schedLeadID] != "lead" {
			t.Errorf("the fixture lead is listed as %q, want lead", roles[schedLeadID])
		}
		if _, ok := roles[schedMemberID]; !ok {
			t.Error("the fixture member is not listed")
		}
		if _, ok := roles[schedOtherID]; ok {
			t.Error("a member of another team is listed")
		}
		return
	}
	t.Fatalf("catalogue has no team %s", schedTeamKey)
}

// Management is above the rota: a leadership team is no team of the
// schedule's, and somebody holding a management role is left out of a team's
// view of its rota -- but still sees their own, read by id.
func TestScheduleIntegration_ManagementIsNotOnTheRota(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', 'Sched Leadership', $1, 'cre-leadership')
		ON CONFLICT DO NOTHING`, schedLeadershipKey)

	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code, From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// The member also leads the Americas team as a whole.
	mustExec(t, pool, `
		INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', $1, $2, 'americas_team_lead')`,
		schedOtherTeamID, schedMemberID)

	cat, err := repo.Catalogue(ctx)
	if err != nil {
		t.Fatalf("Catalogue: %v", err)
	}
	for _, team := range cat.Teams {
		if team.Key == schedLeadershipKey {
			t.Error("a leadership team is served as a rota team")
		}
		for _, m := range team.Members {
			if m.UserID == schedMemberID && m.Role == "americas_team_lead" {
				t.Error("a management role is served as a team member")
			}
		}
	}

	byTeam, err := repo.SearchAssignments(ctx, domain.SearchScheduleAssignmentsRequest{
		From: schedMonday, To: schedMonday, TeamKeys: []string{schedTeamKey},
	})
	if err != nil {
		t.Fatalf("SearchAssignments(team): %v", err)
	}
	for _, a := range byTeam {
		if a.Engineer.UserID == schedMemberID {
			t.Error("the team's rota lists somebody holding a management role")
		}
	}
	own, err := repo.SearchAssignments(ctx, domain.SearchScheduleAssignmentsRequest{
		From: schedMonday, To: schedMonday, UserID: schedMemberID,
	})
	if err != nil {
		t.Fatalf("SearchAssignments(own): %v", err)
	}
	if len(own) != 1 {
		t.Errorf("their own rota read = %d rows, want 1", len(own))
	}
}

// A CRE-typed team that works no rota -- a change-request approval board the
// directory sync brought in as a team -- is no team of the schedule's, so its
// members are not listed on the roster under it. Given an ordinary-weekday
// window it is one, which is how a new team is put on the rota.
func TestScheduleIntegration_OnlyTeamsThatWorkARotaAreInTheCatalogue(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', 'Sched Approval Board', $1, 'cre')
		ON CONFLICT DO NOTHING`, schedBoardKey)
	mustExec(t, pool, `
		INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
		SELECT gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', t.id, $2, 'engineer' FROM team t WHERE t.key = $1`,
		schedBoardKey, schedOtherID)

	listed := func() (board, abt bool) {
		t.Helper()
		cat, err := repo.Catalogue(ctx)
		if err != nil {
			t.Fatalf("Catalogue: %v", err)
		}
		for _, team := range cat.Teams {
			board = board || team.Key == schedBoardKey
			abt = abt || team.Key == schedTeamKey
		}
		return board, abt
	}
	if board, abt := listed(); board || !abt {
		t.Fatalf("board listed=%v (want false), ABT listed=%v (want true)", board, abt)
	}

	mustExec(t, pool, `INSERT INTO team_schedule_team_default_shift (team_key, shift_code, created_by)
		VALUES ($1, $2, 'fixture')`, schedBoardKey, shiftWithScope(t, pool, "CRE", "WEEKDAY"))
	if board, _ := listed(); !board {
		t.Fatal("a team given an ordinary-weekday window is still not listed")
	}
}

// ── ApplyRange ────────────────────────────────────────────────────────────

// The headline behaviour: a weekday rotation asked for across a week sets the
// five weekdays and skips the Saturday and Sunday rather than refusing the
// whole call or forcing them.
func TestScheduleIntegration_ApplyRangeSkipsDaysTheWindowIsNotWorkedOn(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")

	res, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID:    schedMemberID,
		TeamKey:   schedTeamKey,
		ShiftCode: code,
		From:      schedMonday,  // Mon 21 Sept
		To:        "2026-09-27", // Sun 27 Sept
	}, schedLeadEmail)
	if err != nil {
		t.Fatalf("ApplyRange: %v", err)
	}
	if res.Applied != 5 {
		t.Fatalf("applied %d days, want the 5 weekdays", res.Applied)
	}
	if res.Skipped != 2 {
		t.Fatalf("skipped %d days, want the 2 weekend days", res.Skipped)
	}
	// It reports which, not only how many, so a caller can say why.
	if len(res.SkippedDates) != 2 ||
		res.SkippedDates[0] != "2026-09-26" || res.SkippedDates[1] != "2026-09-27" {
		t.Fatalf("skipped dates %v, want the Saturday and the Sunday", res.SkippedDates)
	}

	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid AND rota_date BETWEEN $2::date AND $3::date`,
		schedMemberID, schedMonday, "2026-09-27"); n != 5 {
		t.Fatalf("%d assignment rows in the range, want 5", n)
	}
}

// Every day is one slot per person, so applying over a day that already holds
// something replaces it -- and the replaced row is recorded before it goes,
// or the history would show a row appearing from nowhere.
func TestScheduleIntegration_ApplyRangeReplacesAndRecordsWhatItDisplaced(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	first := shiftWithScope(t, pool, "CRE", "WEEKDAY")

	base := domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: first,
		From: schedMonday, To: schedMonday,
	}
	if _, err := repo.ApplyRange(ctx, base, schedLeadEmail); err != nil {
		t.Fatalf("first ApplyRange: %v", err)
	}
	if _, err := repo.ApplyRange(ctx, base, schedLeadEmail); err != nil {
		t.Fatalf("second ApplyRange: %v", err)
	}

	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid AND rota_date = $2::date`,
		schedMemberID, schedMonday); n != 1 {
		t.Fatalf("%d rows on the day, want exactly 1 -- the day is one slot per person", n)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment_activity WHERE user_id = $1::uuid AND action = 'DELETED'`,
		schedMemberID); n != 1 {
		t.Fatalf("%d DELETED activity rows, want 1 for the displaced assignment", n)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment_activity WHERE user_id = $1::uuid AND action = 'CREATED'`,
		schedMemberID); n != 2 {
		t.Fatalf("%d CREATED activity rows, want 2", n)
	}
}

// A person already on a window for another team cannot be put on an
// overlapping one: the day's replace clears only the lead's own team's rows.
// The database refuses the second window, and that has to come back as a
// conflict the picker can show, not as a 500 that says only "not saved".
func TestScheduleIntegration_ApplyRangeOverAnotherTeamsWindowIsAConflict(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")

	other := domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedOtherTeam, ShiftCode: shift,
		From: schedMonday, To: schedMonday,
	}
	if _, err := repo.ApplyRange(ctx, other, schedLeadEmail); err != nil {
		t.Fatalf("ApplyRange on the other team: %v", err)
	}

	mine := other
	mine.TeamKey = schedTeamKey
	_, err := repo.ApplyRange(ctx, mine, schedLeadEmail)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("ApplyRange over another team's window: got %v, want a ConflictError", err)
	}
	if !strings.Contains(conflict.Msg, schedMonday) {
		t.Errorf("conflict message %q does not name the day", conflict.Msg)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid AND rota_date = $2::date`,
		schedMemberID, schedMonday); n != 1 {
		t.Fatalf("%d rows on the day, want the other team's 1 left alone", n)
	}
}

// An empty shift code is the picker's clear: it takes the days off the rota
// rather than putting anybody on a window.
func TestScheduleIntegration_ApplyRangeWithNoShiftCodeClearsTheSpan(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")

	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code,
		From: schedMonday, To: "2026-09-23",
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed ApplyRange: %v", err)
	}

	res, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "",
		From: schedMonday, To: "2026-09-23",
	}, schedLeadEmail)
	if err != nil {
		t.Fatalf("clearing ApplyRange: %v", err)
	}
	// Nothing is skipped when clearing: there is no window whose day scope
	// could rule a day out.
	if res.Skipped != 0 {
		t.Fatalf("skipped %d days while clearing, want 0", res.Skipped)
	}
	if res.Applied != 3 {
		t.Fatalf("cleared %d days, want the 3 that held something", res.Applied)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid`, schedMemberID); n != 0 {
		t.Fatalf("%d assignments left after clearing, want 0", n)
	}
}

func TestScheduleIntegration_ApplyRangeRefusesAnUnknownShift(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)

	_, err := repo.ApplyRange(context.Background(), domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "NO_SUCH_WINDOW",
		From: schedMonday, To: schedMonday,
	}, schedLeadEmail)
	if err == nil {
		t.Fatal("a shift code that is not in the catalogue was accepted")
	}
}

// A backwards range is a slip, not a refusal: the ends are swapped.
func TestScheduleIntegration_ApplyRangeAcceptsABackwardsSpan(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")

	res, err := repo.ApplyRange(context.Background(), domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code,
		From: "2026-09-23", To: schedMonday,
	}, schedLeadEmail)
	if err != nil {
		t.Fatalf("ApplyRange: %v", err)
	}
	if res.Applied != 3 {
		t.Fatalf("applied %d days, want 3", res.Applied)
	}
}

// ── ApplyAbsence ──────────────────────────────────────────────────────────

func absenceKind(t *testing.T, pool *pgxpool.Pool, bucket string) string {
	t.Helper()
	var code string
	if err := pool.QueryRow(context.Background(),
		`SELECT code FROM team_schedule_absence_kind WHERE bucket = $1 AND is_active ORDER BY sort_order LIMIT 1`,
		bucket).Scan(&code); err != nil {
		t.Fatalf("no %s absence kind: %v", bucket, err)
	}
	return code
}

// Leave is a span, not a day at a time: a weekend inside it is covered too,
// which is the opposite of how a weekday rotation behaves.
func TestScheduleIntegration_ApplyAbsenceMarksTheWholeSpanIncludingTheWeekend(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	kind := absenceKind(t, pool, "LEAVE")

	res, err := repo.ApplyAbsence(context.Background(), domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: kind,
		From: "2026-09-25", To: "2026-09-28", // Fri through Mon
	}, schedLeadEmail)
	if err != nil {
		t.Fatalf("ApplyAbsence: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("created %d rows, want 1 span", res.Created)
	}
	var starts, ends time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT starts_on, ends_on FROM team_schedule_absence WHERE user_id = $1::uuid`,
		schedMemberID).Scan(&starts, &ends); err != nil {
		t.Fatalf("read the absence back: %v", err)
	}
	if starts.Format("2006-01-02") != "2026-09-25" || ends.Format("2006-01-02") != "2026-09-28" {
		t.Fatalf("stored %s..%s, want the span as asked for -- weekend included",
			starts.Format("2006-01-02"), ends.Format("2006-01-02"))
	}
}

// Clearing three days out of a fortnight of leave means exactly that. The
// stretch either side still stands, so one row becomes two.
func TestScheduleIntegration_ClearingTheMiddleOfALeaveSpanSplitsIt(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	kind := absenceKind(t, pool, "LEAVE")

	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: kind,
		From: "2026-09-01", To: "2026-09-14",
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed ApplyAbsence: %v", err)
	}

	res, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: "",
		From: "2026-09-06", To: "2026-09-08",
	}, schedLeadEmail)
	if err != nil {
		t.Fatalf("clearing ApplyAbsence: %v", err)
	}
	if res.Trimmed != 1 {
		t.Fatalf("trimmed %d, want 1 -- the span straddles both ends of the hole", res.Trimmed)
	}
	if res.Removed != 0 {
		t.Fatalf("removed %d, want 0 -- most of the leave still stands", res.Removed)
	}

	rows, err := pool.Query(ctx,
		`SELECT starts_on, ends_on FROM team_schedule_absence WHERE user_id = $1::uuid ORDER BY starts_on`,
		schedMemberID)
	if err != nil {
		t.Fatalf("read the absences back: %v", err)
	}
	defer rows.Close()
	var spans [][2]string
	for rows.Next() {
		var s, e time.Time
		if err := rows.Scan(&s, &e); err != nil {
			t.Fatalf("scan: %v", err)
		}
		spans = append(spans, [2]string{s.Format("2006-01-02"), e.Format("2006-01-02")})
	}
	if len(spans) != 2 {
		t.Fatalf("got %d spans %v, want the two stretches either side of the hole", len(spans), spans)
	}
	if spans[0] != [2]string{"2026-09-01", "2026-09-05"} {
		t.Fatalf("first span %v, want 01..05", spans[0])
	}
	if spans[1] != [2]string{"2026-09-09", "2026-09-14"} {
		t.Fatalf("second span %v, want 09..14", spans[1])
	}
}

// An absence entirely inside the cleared span has nothing left to keep.
func TestScheduleIntegration_ClearingOverALeaveSpanRemovesIt(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	kind := absenceKind(t, pool, "LEAVE")

	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: kind,
		From: "2026-09-10", To: "2026-09-11",
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed ApplyAbsence: %v", err)
	}

	res, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: "",
		From: "2026-09-01", To: "2026-09-30",
	}, schedLeadEmail)
	if err != nil {
		t.Fatalf("clearing ApplyAbsence: %v", err)
	}
	if res.Removed != 1 || res.Trimmed != 0 {
		t.Fatalf("removed %d trimmed %d, want removed 1 trimmed 0", res.Removed, res.Trimmed)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_absence WHERE user_id = $1::uuid`, schedMemberID); n != 0 {
		t.Fatalf("%d absences left, want 0", n)
	}
	// The removal is recorded, since the row it describes is gone.
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_absence_activity WHERE user_id = $1::uuid AND action = 'DELETED'`,
		schedMemberID); n != 1 {
		t.Fatalf("%d DELETED absence activity rows, want 1", n)
	}
}

// Marking leave over a stretch that already holds some replaces it rather
// than leaving two overlapping spans behind.
func TestScheduleIntegration_MarkingOverExistingLeaveLeavesOneSpan(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	leave := absenceKind(t, pool, "LEAVE")

	for _, span := range [][2]string{{"2026-09-02", "2026-09-03"}, {"2026-09-01", "2026-09-05"}} {
		if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
			UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: leave,
			From: span[0], To: span[1],
		}, schedLeadEmail); err != nil {
			t.Fatalf("ApplyAbsence %v: %v", span, err)
		}
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_absence WHERE user_id = $1::uuid`, schedMemberID); n != 1 {
		t.Fatalf("%d absence rows, want 1 -- the second span swallowed the first", n)
	}
}

func TestScheduleIntegration_ApplyAbsenceRefusesAnUnknownKind(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)

	_, err := repo.ApplyAbsence(context.Background(), domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: "NO_SUCH_KIND",
		From: schedMonday, To: schedMonday,
	}, schedLeadEmail)
	if err == nil {
		t.Fatal("an absence kind that is not in the catalogue was accepted")
	}
}

// An allocation says who the time is for; leave is not for anybody, so the
// same value sent with a leave kind is not stored.
func TestScheduleIntegration_ApplyAbsenceKeepsWhoAnAllocationIsFor(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	customer := "Acme Corp"

	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: absenceKind(t, pool, "ALLOCATION"),
		From: "2026-09-01", To: "2026-09-04", AllocatedTo: &customer,
	}, schedLeadEmail); err != nil {
		t.Fatalf("ApplyAbsence (allocation): %v", err)
	}
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: absenceKind(t, pool, "LEAVE"),
		From: "2026-09-21", To: "2026-09-22", AllocatedTo: &customer,
	}, schedLeadEmail); err != nil {
		t.Fatalf("ApplyAbsence (leave): %v", err)
	}

	got := map[string]*string{}
	rows, err := pool.Query(ctx,
		`SELECT k.bucket::text, a.allocated_to FROM team_schedule_absence a
		   JOIN team_schedule_absence_kind k ON k.id = a.kind_id
		  WHERE a.user_id = $1::uuid`, schedMemberID)
	if err != nil {
		t.Fatalf("read the absences back: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var bucket string
		var to *string
		if err := rows.Scan(&bucket, &to); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[bucket] = to
	}
	if v := got["ALLOCATION"]; v == nil || *v != customer {
		t.Errorf("allocation stored allocated_to %v, want %q", v, customer)
	}
	if v, ok := got["LEAVE"]; !ok || v != nil {
		t.Errorf("leave stored allocated_to %v, want it left empty", v)
	}
}

// A retired kind stays in the catalogue so older absences read correctly, but
// nothing new can be marked against it.
func TestScheduleIntegration_ApplyAbsenceRefusesARetiredKind(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	var retired string
	if err := pool.QueryRow(context.Background(),
		`SELECT code FROM team_schedule_absence_kind WHERE NOT is_active LIMIT 1`).Scan(&retired); err != nil {
		t.Skipf("no retired kind in this catalogue: %v", err)
	}
	if _, err := repo.ApplyAbsence(context.Background(), domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: retired,
		From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err == nil {
		t.Fatalf("retired kind %s was accepted", retired)
	}
}

// Removing an absence removes all of it, including one with no end date --
// which clearing a date range cannot do, since there is no range to name.
func TestScheduleIntegration_DeleteAbsenceRemovesAnOpenEndedSpan(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	var id string
	if err := pool.QueryRow(ctx, `
		INSERT INTO team_schedule_absence (user_id, team_key, kind_id, starts_on, ends_on)
		SELECT $1::uuid, $2, id, '2026-09-01', NULL FROM team_schedule_absence_kind WHERE code = $3
		RETURNING id::text`, schedMemberID, schedTeamKey, absenceKind(t, pool, "ALLOCATION")).Scan(&id); err != nil {
		t.Fatalf("seed an open-ended allocation: %v", err)
	}

	got, err := repo.AbsenceByID(ctx, id)
	if err != nil {
		t.Fatalf("AbsenceByID: %v", err)
	}
	if got.TeamKey != schedTeamKey || got.Engineer.UserID != schedMemberID || got.EndsOn != nil {
		t.Fatalf("read back %+v, want the member's open-ended span on %s", got, schedTeamKey)
	}

	if err := repo.DeleteAbsence(ctx, id, schedLeadEmail, nil); err != nil {
		t.Fatalf("DeleteAbsence: %v", err)
	}
	var left, history int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM team_schedule_absence WHERE id = $1::uuid`, id).Scan(&left); err != nil {
		t.Fatalf("count: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM team_schedule_absence_activity WHERE absence_id = $1::uuid AND action = 'DELETED'`,
		id).Scan(&history); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if left != 0 || history != 1 {
		t.Fatalf("after delete: %d rows left and %d DELETED history rows, want 0 and 1", left, history)
	}

	var notFound *apierror.NotFoundError
	if err := repo.DeleteAbsence(ctx, id, schedLeadEmail, nil); !errors.As(err, &notFound) {
		t.Fatalf("deleting it again: want NotFoundError, got %v", err)
	}
}

// A new tag lands at the end of its own bucket, and can be marked at once.
// The same label, or a short code another active kind already draws, is
// refused rather than creating a twin.
func TestScheduleIntegration_CreateAbsenceKindJoinsTheEndOfItsBucket(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM team_schedule_absence WHERE kind_id IN (SELECT id FROM team_schedule_absence_kind WHERE code = 'INTEGRATION_TEST_TAG')`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM team_schedule_absence_kind WHERE code = 'INTEGRATION_TEST_TAG'`)
	})

	var maxAllocation int
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), 0) FROM team_schedule_absence_kind WHERE bucket = 'ALLOCATION'`).Scan(&maxAllocation); err != nil {
		t.Fatalf("read sort order: %v", err)
	}

	req := domain.CreateScheduleAbsenceKindRequest{ShortCode: "ITT", Label: "Integration test tag", Bucket: "ALLOCATION", ColourToken: "INT"}
	k, err := repo.CreateAbsenceKind(ctx, "INTEGRATION_TEST_TAG", req, schedLeadEmail)
	if err != nil {
		t.Fatalf("CreateAbsenceKind: %v", err)
	}
	if k.ID == "" || k.SortOrder != maxAllocation+1 {
		t.Fatalf("created %+v, want an id and sort order %d", k, maxAllocation+1)
	}

	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: k.Code, From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("marking the new tag: %v", err)
	}

	var conflict *apierror.ConflictError
	if _, err := repo.CreateAbsenceKind(ctx, "INTEGRATION_TEST_TAG", domain.CreateScheduleAbsenceKindRequest{
		ShortCode: "IT2", Label: "Integration test tag", Bucket: "ALLOCATION", ColourToken: "INT",
	}, schedLeadEmail); !errors.As(err, &conflict) {
		t.Fatalf("the same label again: want ConflictError, got %v", err)
	}
	if _, err := repo.CreateAbsenceKind(ctx, "SOMETHING_ELSE", domain.CreateScheduleAbsenceKindRequest{
		ShortCode: "al", Label: "Something else", Bucket: "LEAVE", ColourToken: "AL",
	}, schedLeadEmail); !errors.As(err, &conflict) {
		t.Fatalf("annual leave's short code: want ConflictError, got %v", err)
	}
}

// Any tier can be rostered on a zone's escalation window: the window leaves
// the tier to the person. A window that fixes a tier accepts only that one,
// and a window that is not an escalation window holds none.
func TestScheduleIntegration_ApplyRangeRostersAnyTierOnAZonesEscalationWindow(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	l3 := "L3"

	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1",
		From: schedMonday, To: schedMonday, Tier: &l3,
	}, schedLeadEmail); err != nil {
		t.Fatalf("L3 on SRE_TZ1: %v", err)
	}
	var tier, zone string
	if err := pool.QueryRow(ctx, `
		SELECT a.tier::text, z.code FROM team_schedule_assignment a
		  JOIN team_schedule_zone z ON z.id = a.zone_id
		 WHERE a.user_id = $1::uuid AND a.rota_date = $2::date`, schedMemberID, schedMonday).Scan(&tier, &zone); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if tier != "L3" || zone != "TZ1" {
		t.Fatalf("stored %s in %s, want L3 in TZ1", tier, zone)
	}

	var invalid *apierror.ValidationError
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1",
		From: schedMonday, To: schedMonday,
	}, schedLeadEmail); !errors.As(err, &invalid) {
		t.Fatalf("SRE_TZ1 with no tier: want ValidationError, got %v", err)
	}
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1_L1",
		From: schedMonday, To: schedMonday, Tier: &l3,
	}, schedLeadEmail); !errors.As(err, &invalid) {
		t.Fatalf("L3 on the L1-only window: want ValidationError, got %v", err)
	}
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1_REGULAR",
		From: schedMonday, To: schedMonday, Tier: &l3,
	}, schedLeadEmail); !errors.As(err, &invalid) {
		t.Fatalf("a tier on regular hours: want ValidationError, got %v", err)
	}
}

// A lead may delete a tag a lead added, once nothing uses it -- never one of
// the catalogue's own.
func TestScheduleIntegration_DeleteAbsenceKindOnlyRemovesAnUnusedCustomTag(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM team_schedule_absence WHERE kind_id IN (SELECT id FROM team_schedule_absence_kind WHERE code = 'INTEGRATION_DELETE_TAG')`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM team_schedule_absence_kind WHERE code = 'INTEGRATION_DELETE_TAG'`)
	})

	var forbidden *apierror.ForbiddenError
	if err := repo.DeleteAbsenceKind(ctx, "ANNUAL_LEAVE", schedLeadEmail); !errors.As(err, &forbidden) {
		t.Fatalf("deleting annual leave: want ForbiddenError, got %v", err)
	}

	k, err := repo.CreateAbsenceKind(ctx, "INTEGRATION_DELETE_TAG", domain.CreateScheduleAbsenceKindRequest{
		ShortCode: "IDT", Label: "Integration delete tag", Bucket: "ALLOCATION", ColourToken: "INT",
	}, schedLeadEmail)
	if err != nil {
		t.Fatalf("CreateAbsenceKind: %v", err)
	}
	if !k.Custom {
		t.Fatal("a tag a lead added did not come back marked custom")
	}
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: k.Code, From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("marking the tag: %v", err)
	}
	var conflict *apierror.ConflictError
	if err := repo.DeleteAbsenceKind(ctx, k.Code, schedLeadEmail); !errors.As(err, &conflict) {
		t.Fatalf("deleting a tag in use: want ConflictError, got %v", err)
	}

	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("clearing the day: %v", err)
	}
	if err := repo.DeleteAbsenceKind(ctx, k.Code, schedLeadEmail); err != nil {
		t.Fatalf("deleting the unused tag: %v", err)
	}
	cat, err := repo.Catalogue(ctx)
	if err != nil {
		t.Fatalf("Catalogue: %v", err)
	}
	for _, kind := range cat.AbsenceKinds {
		if kind.Code == k.Code {
			t.Fatal("the deleted tag is still in the catalogue")
		}
		if kind.Code == "ANNUAL_LEAVE" && kind.Custom {
			t.Fatal("annual leave is marked custom")
		}
	}
}

// One engineer can hold turns in two zones on the same day -- TZ1 L1 in the
// morning, TZ2 L2 in the afternoon. A new turn only displaces what is in its
// own zone or overlaps it; a clear can be narrowed to one zone; a regular
// window still replaces the whole day.
func TestScheduleIntegration_ApplyRangeKeepsTurnsInOtherZones(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	l2 := "L2"
	apply := func(code string, tier *string, zone *string) {
		t.Helper()
		if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
			UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code,
			From: schedMonday, To: schedMonday, Tier: tier, ZoneCode: zone,
		}, schedLeadEmail); err != nil {
			t.Fatalf("apply %q: %v", code, err)
		}
	}
	held := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT s.code || ':' || COALESCE(a.tier::text, '-') FROM team_schedule_assignment a
			  JOIN team_schedule_shift s ON s.id = a.shift_id
			 WHERE a.user_id = $1::uuid AND a.rota_date = $2::date ORDER BY s.code`, schedMemberID, schedMonday)
		if err != nil {
			t.Fatalf("read the day: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				t.Fatalf("scan: %v", err)
			}
			out = append(out, v)
		}
		return out
	}
	want := func(label string, expect ...string) {
		t.Helper()
		got := held()
		if fmt.Sprint(got) != fmt.Sprint(expect) {
			t.Fatalf("%s: holds %v, want %v", label, got, expect)
		}
	}

	apply("SRE_TZ1_L1", nil, nil)
	apply("SRE_TZ2", &l2, nil)
	want("TZ1 L1 then TZ2 L2", "SRE_TZ1_L1:L1", "SRE_TZ2:L2")

	apply("SRE_TZ1", &l2, nil)
	want("TZ1 L2 replaces TZ1 L1", "SRE_TZ1:L2", "SRE_TZ2:L2")

	tz2 := "TZ2"
	apply("", nil, &tz2)
	want("clearing TZ2 only", "SRE_TZ1:L2")

	// A zone's regular hours sit under the turns rather than replacing them:
	// on TZ1's regular hours and TZ1 L2, both are true.
	apply("SRE_TZ1_REGULAR", nil, nil)
	want("regular hours keep the turn", "SRE_TZ1:L2", "SRE_TZ1_REGULAR:-")

	// Clearing TZ1 takes its turn off and leaves the regular hours.
	tz1 := "TZ1"
	apply("", nil, &tz1)
	want("clearing TZ1's turn", "SRE_TZ1_REGULAR:-")

	// A turn added to a day of regular hours keeps them too.
	apply("SRE_TZ1_L1", nil, nil)
	want("a turn keeps the regular hours", "SRE_TZ1_L1:L1", "SRE_TZ1_REGULAR:-")

	// Another zone's regular hours replace the first zone's, not the turn.
	apply("SRE_TZ2_REGULAR", nil, nil)
	want("one zone's regular hours at a time", "SRE_TZ1_L1:L1", "SRE_TZ2_REGULAR:-")
}

// ── reads ─────────────────────────────────────────────────────────────────

func TestScheduleIntegration_SearchAssignmentsFiltersByTeamAndWindow(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")

	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code,
		From: schedMonday, To: "2026-09-23",
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := repo.SearchAssignments(ctx, domain.SearchScheduleAssignmentsRequest{
		From: schedMonday, To: "2026-09-23", TeamKeys: []string{schedTeamKey},
	})
	if err != nil {
		t.Fatalf("SearchAssignments: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d assignments, want 3", len(got))
	}

	// A window that ends before the rota starts finds nothing.
	none, err := repo.SearchAssignments(ctx, domain.SearchScheduleAssignmentsRequest{
		From: "2026-08-01", To: "2026-08-02", TeamKeys: []string{schedTeamKey},
	})
	if err != nil {
		t.Fatalf("SearchAssignments(empty window): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("got %d assignments outside the window, want 0", len(none))
	}

	// Asking by email resolves to the same person, which is what saves every
	// client its own lookup.
	byEmail, err := repo.SearchAssignments(ctx, domain.SearchScheduleAssignmentsRequest{
		From: schedMonday, To: "2026-09-23", UserEmail: schedMemberEmail,
	})
	if err != nil {
		t.Fatalf("SearchAssignments(by email): %v", err)
	}
	if len(byEmail) != 3 {
		t.Fatalf("got %d assignments by email, want 3", len(byEmail))
	}
}

// ── a span that moves somebody to another team ─────────────────────────────

// moveKind is a fixture tag that moves people to the fixture's other team, the
// way the Brazil rotation moves them to the Americas team.
func moveKind(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	const code = "FIXTURE_MOVE"
	cleanup := func() {
		mustExec(t, pool, `DELETE FROM team_schedule_absence_activity WHERE kind_code = $1`, code)
		mustExec(t, pool, `DELETE FROM team_schedule_absence WHERE kind_id IN (SELECT id FROM team_schedule_absence_kind WHERE code = $1)`, code)
		mustExec(t, pool, `DELETE FROM team_schedule_absence_kind WHERE code = $1`, code)
	}
	cleanup()
	t.Cleanup(cleanup)
	mustExec(t, pool, `
		INSERT INTO team_schedule_absence_kind
		  (code, short_code, label, bucket, colour_token, sort_order, is_active,
		   created_by, updated_by, moves_to_team_key, works_rota_there)
		VALUES ($1, 'FXM', 'Fixture move', 'ALLOCATION', 'BR', 999, TRUE,
		        'fixture', 'fixture', $2, TRUE)`, code, schedOtherTeam)
	return code
}

// The span is filed under the team it moves the person to, with their own
// team beside it; the move covers exactly its dates; and the ladder pages
// them for that team's shifts while still treating them as away from their own.
func TestScheduleIntegration_AMovingSpanIsFiledUnderItsTeamAndPagedThere(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	kind := moveKind(t, pool)
	friday := "2026-09-25"

	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, HomeTeamKey: schedTeamKey,
		KindCode: kind, From: schedMonday, To: friday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("ApplyAbsence: %v", err)
	}

	var teamKey, home string
	if err := pool.QueryRow(ctx,
		`SELECT team_key, COALESCE(home_team_key, '') FROM team_schedule_absence WHERE user_id = $1::uuid`,
		schedMemberID).Scan(&teamKey, &home); err != nil {
		t.Fatalf("read the span back: %v", err)
	}
	if teamKey != schedOtherTeam || home != schedTeamKey {
		t.Fatalf("span filed under %q with home %q, want %q with home %q", teamKey, home, schedOtherTeam, schedTeamKey)
	}

	got, err := repo.SearchAbsences(ctx, domain.SearchScheduleAbsencesRequest{
		From: schedMonday, To: friday, TeamKeys: []string{schedOtherTeam},
	})
	if err != nil {
		t.Fatalf("SearchAbsences: %v", err)
	}
	if len(got) != 1 || got[0].HomeTeamKey == nil || *got[0].HomeTeamKey != schedTeamKey {
		t.Fatalf("the other team's read = %+v, want the span with its home team", got)
	}
	// And their own team's read still finds it, under the team they went to.
	fromHome, err := repo.SearchAbsences(ctx, domain.SearchScheduleAbsencesRequest{
		From: schedMonday, To: friday, TeamKeys: []string{schedTeamKey},
	})
	if err != nil {
		t.Fatalf("SearchAbsences(home): %v", err)
	}
	if len(fromHome) != 1 || fromHome[0].TeamKey != schedOtherTeam {
		t.Fatalf("the home team's read = %+v, want the moved span", fromHome)
	}

	for _, c := range []struct {
		team, from, to string
		want           bool
	}{
		{schedOtherTeam, "2026-09-22", "2026-09-23", true},
		{schedOtherTeam, schedMonday, "2026-09-28", false}, // runs past the span
		{schedTeamKey, "2026-09-22", "2026-09-23", false},  // not the team it moved them to
	} {
		gotHome, ok, err := repo.MovedToTeamOver(ctx, schedMemberID, c.team, c.from, c.to)
		if err != nil {
			t.Fatalf("MovedToTeamOver: %v", err)
		}
		if ok != c.want || (ok && gotHome != schedTeamKey) {
			t.Errorf("MovedToTeamOver(%s, %s..%s) = %q, %v; want %v", c.team, c.from, c.to, gotHome, ok, c.want)
		}
	}

	// A shift for each team: Monday for the team the span moved them to,
	// Tuesday for their own.
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	for _, a := range []struct{ team, day string }{{schedOtherTeam, schedMonday}, {schedTeamKey, "2026-09-22"}} {
		if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
			UserID: schedMemberID, TeamKey: a.team, ShiftCode: code, From: a.day, To: a.day,
		}, schedLeadEmail); err != nil {
			t.Fatalf("seed %s shift: %v", a.team, err)
		}
	}
	onDutyFor := func(team string) bool {
		var startsAt, endsAt time.Time
		if err := pool.QueryRow(ctx,
			`SELECT starts_at, ends_at FROM team_schedule_assignment WHERE user_id = $1::uuid AND team_key = $2`,
			schedMemberID, team).Scan(&startsAt, &endsAt); err != nil {
			t.Fatalf("read the %s window back: %v", team, err)
		}
		rows, err := repo.OnDutyAt(ctx, startsAt.Add(endsAt.Sub(startsAt)/2))
		if err != nil {
			t.Fatalf("OnDutyAt: %v", err)
		}
		for _, a := range rows {
			if a.Engineer.UserID == schedMemberID {
				return true
			}
		}
		return false
	}
	if !onDutyFor(schedOtherTeam) {
		t.Error("not paged for a shift on the team the span moved them to")
	}
	if onDutyFor(schedTeamKey) {
		t.Error("paged for their own team's shift while the span has them working elsewhere")
	}
}

// A span of a tag worked as another team's normal hours writes those hours as
// real shifts on that team, and keeps them in step with the span: cut short,
// overlaid by leave, or removed, the shifts it no longer covers go -- and a
// shift a lead placed by hand is never touched.
func TestScheduleIntegration_AMoveWritesRealShiftsAndKeepsThemInStep(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	kind := moveKind(t, pool)
	shown := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	mustExec(t, pool, `UPDATE team_schedule_absence_kind SET shows_as_shift_code = $2 WHERE code = $1`, kind, shown)
	friday := "2026-09-25"

	moveShifts := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT rota_date::text FROM team_schedule_assignment
			 WHERE user_id = $1::uuid AND source = 'MOVE' AND team_key = $2 ORDER BY rota_date`,
			schedMemberID, schedOtherTeam)
		if err != nil {
			t.Fatalf("read move shifts: %v", err)
		}
		defer rows.Close()
		var days []string
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				t.Fatalf("scan: %v", err)
			}
			days = append(days, d)
		}
		return days
	}
	apply := func(kindCode, from, to string) {
		t.Helper()
		if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
			UserID: schedMemberID, TeamKey: schedTeamKey, HomeTeamKey: schedTeamKey,
			KindCode: kindCode, From: from, To: to,
		}, schedLeadEmail); err != nil {
			t.Fatalf("ApplyAbsence(%q %s..%s): %v", kindCode, from, to, err)
		}
	}

	// A shift placed by hand on the other team, which no span may touch.
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedOtherTeam, ShiftCode: shown, From: "2026-09-28", To: "2026-09-28",
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed manual shift: %v", err)
	}

	apply(kind, schedMonday, friday)
	if got := moveShifts(); len(got) != 5 {
		t.Fatalf("after marking Mon-Fri: %v, want five weekdays", got)
	}
	apply("", "2026-09-24", friday) // back from Thursday
	if got := moveShifts(); len(got) != 3 {
		t.Fatalf("after cutting it to Mon-Wed: %v, want three days", got)
	}
	apply(absenceKind(t, pool, "LEAVE"), "2026-09-22", "2026-09-22") // leave on the Tuesday
	if got := moveShifts(); len(got) != 2 || got[0] != schedMonday || got[1] != "2026-09-23" {
		t.Fatalf("after leave on Tuesday: %v, want Mon and Wed", got)
	}

	var spanID string
	if err := pool.QueryRow(ctx, `
		SELECT ab.id::text FROM team_schedule_absence ab JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
		 WHERE ab.user_id = $1::uuid AND k.code = $2 ORDER BY ab.starts_on LIMIT 1`, schedMemberID, kind).Scan(&spanID); err != nil {
		t.Fatalf("find span: %v", err)
	}
	if err := repo.DeleteAbsence(ctx, spanID, schedLeadEmail, nil); err != nil {
		t.Fatalf("DeleteAbsence: %v", err)
	}
	if got := moveShifts(); len(got) != 1 || got[0] != "2026-09-23" {
		t.Fatalf("after removing the Monday span: %v, want only Wednesday's, from the span after the leave", got)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_assignment
		WHERE user_id = $1::uuid AND source = 'MANUAL' AND rota_date = '2026-09-28'`, schedMemberID); n != 1 {
		t.Fatalf("the hand-placed shift was touched: %d rows", n)
	}
}

// Removing an open-ended move takes every shift it wrote, however far past
// its start they run -- the ladder would otherwise go on paging the person
// for the team they moved to.
func TestScheduleIntegration_RemovingAnOpenEndedMoveLeavesNoShiftBehind(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	kind := moveKind(t, pool)
	shown := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	mustExec(t, pool, `UPDATE team_schedule_absence_kind SET shows_as_shift_code = $2 WHERE code = $1`, kind, shown)

	// Open-ended, started well over a year before the shifts below.
	var spanID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO team_schedule_absence (id, created_on, updated_on, created_by, updated_by, user_id, team_key, kind_id, starts_on, ends_on, home_team_key)
		SELECT gen_random_uuid(), now(), now(), 'fixture', 'fixture', $1::uuid, $2, k.id, DATE '2024-01-01', NULL, $3
		  FROM team_schedule_absence_kind k WHERE k.code = $4
		RETURNING id::text`, schedMemberID, schedOtherTeam, schedTeamKey, kind).Scan(&spanID); err != nil {
		t.Fatalf("seed open-ended span: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := syncMoveShifts(ctx, tx, schedMemberID, schedMonday, "2026-09-25", schedLeadEmail); err != nil {
		t.Fatalf("syncMoveShifts: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	moves := func() int {
		return countRows(t, pool, `SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid AND source = 'MOVE'`, schedMemberID)
	}
	if n := moves(); n != 5 {
		t.Fatalf("seeded %d move shifts, want 5", n)
	}

	if err := repo.DeleteAbsence(ctx, spanID, schedLeadEmail, nil); err != nil {
		t.Fatalf("DeleteAbsence: %v", err)
	}
	if n := moves(); n != 0 {
		t.Fatalf("%d move shifts left behind after removing the open-ended span", n)
	}
}

// Somebody moved to a team that works no rota (Migration) takes no rotation
// turn while the span lasts -- and their standing hours are still theirs.
func TestScheduleIntegration_NoRotationWhileMovedOffTheRota(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	kind := moveKind(t, pool)
	mustExec(t, pool, `UPDATE team_schedule_absence_kind SET works_rota_there = FALSE WHERE code = $1`, kind)
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, HomeTeamKey: schedTeamKey,
		KindCode: kind, From: schedMonday, To: "2026-09-25",
	}, schedLeadEmail); err != nil {
		t.Fatalf("ApplyAbsence: %v", err)
	}

	var rotation, standing string
	if err := pool.QueryRow(ctx, `SELECT code FROM team_schedule_shift
		WHERE family::text = 'CRE' AND is_rotation AND day_scope::text <> 'WEEKEND' AND is_active ORDER BY sort_order LIMIT 1`).Scan(&rotation); err != nil {
		t.Fatalf("no CRE weekday rotation: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT code FROM team_schedule_shift
		WHERE family::text = 'CRE' AND NOT is_rotation AND day_scope::text = 'WEEKDAY' AND is_active ORDER BY sort_order LIMIT 1`).Scan(&standing); err != nil {
		t.Fatalf("no CRE standing window: %v", err)
	}

	_, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedOtherTeam, ShiftCode: rotation, From: "2026-09-23", To: "2026-09-23",
	}, schedLeadEmail)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("a rotation on a day moved off the rota: got %v, want a ConflictError", err)
	}
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedOtherTeam, ShiftCode: standing, From: "2026-09-23", To: "2026-09-23",
	}, schedLeadEmail); err != nil {
		t.Fatalf("standing hours on a day moved off the rota were refused: %v", err)
	}
	// After the span, rotations are theirs again.
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: rotation, From: "2026-09-28", To: "2026-09-28",
	}, schedLeadEmail); err != nil {
		t.Fatalf("a rotation after the span was refused: %v", err)
	}
}

// The point of the GiST range index: on-call is a containment question, and
// the answer has to exclude anyone on leave or it names somebody who is away.
func TestScheduleIntegration_OnDutyAtExcludesSomebodyOnLeave(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")

	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code,
		From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var startsAt, endsAt time.Time
	if err := pool.QueryRow(ctx,
		`SELECT starts_at, ends_at FROM team_schedule_assignment WHERE user_id = $1::uuid`,
		schedMemberID).Scan(&startsAt, &endsAt); err != nil {
		t.Fatalf("read the window back: %v", err)
	}
	middle := startsAt.Add(endsAt.Sub(startsAt) / 2)

	onDuty := func() bool {
		rows, err := repo.OnDutyAt(ctx, middle)
		if err != nil {
			t.Fatalf("OnDutyAt: %v", err)
		}
		for _, a := range rows {
			if a.Engineer.UserID == schedMemberID {
				return true
			}
		}
		return false
	}

	if !onDuty() {
		t.Fatal("an engineer inside their own window was not reported on duty")
	}

	// Just outside it, they are not.
	if rows, err := repo.OnDutyAt(ctx, endsAt.Add(time.Hour)); err != nil {
		t.Fatalf("OnDutyAt(after): %v", err)
	} else {
		for _, a := range rows {
			if a.Engineer.UserID == schedMemberID {
				t.Fatal("an engineer was reported on duty an hour after their window ended")
			}
		}
	}

	// Now book leave over the same day. The assignment is untouched -- leave
	// covers the rota rather than deleting it -- so this is exactly the case
	// a query that forgot to reconcile the two would get wrong.
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey,
		KindCode: absenceKind(t, pool, "LEAVE"),
		From:     schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("ApplyAbsence: %v", err)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid`, schedMemberID); n != 1 {
		t.Fatalf("the assignment went away when leave was booked; %d rows left", n)
	}
	if onDuty() {
		t.Fatal("somebody on leave was still reported on duty")
	}
}

func TestScheduleIntegration_SearchAbsencesFindsAnOverlappingSpan(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey,
		KindCode: absenceKind(t, pool, "LEAVE"),
		From:     "2026-09-10", To: "2026-09-20",
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A window touching only the tail of the span still finds it: the search
	// is an overlap, not a containment.
	got, err := repo.SearchAbsences(ctx, domain.SearchScheduleAbsencesRequest{
		From: "2026-09-19", To: "2026-09-25", TeamKeys: []string{schedTeamKey},
	})
	if err != nil {
		t.Fatalf("SearchAbsences: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d absences overlapping the tail, want 1", len(got))
	}

	none, err := repo.SearchAbsences(ctx, domain.SearchScheduleAbsencesRequest{
		From: "2026-09-21", To: "2026-09-25", TeamKeys: []string{schedTeamKey},
	})
	if err != nil {
		t.Fatalf("SearchAbsences(after): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("got %d absences after the span ended, want 0", len(none))
	}
}

// ── the single-row writes, and the history they leave ─────────────────────

func TestScheduleIntegration_CreateUpdateDeleteLeaveATrail(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")

	made, err := repo.CreateAssignment(ctx, domain.CreateScheduleAssignmentRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code, RotaDate: schedMonday,
	}, schedLeadEmail)
	if err != nil {
		t.Fatalf("CreateAssignment: %v", err)
	}
	// StartsAt/EndsAt are resolved from the window rather than accepted from
	// the caller, so a hand-placed cover cannot drift from what it claims.
	if made.StartsAt.IsZero() || !made.EndsAt.After(made.StartsAt) {
		t.Fatalf("resolved window %s..%s is not a forward span", made.StartsAt, made.EndsAt)
	}

	back, err := repo.AssignmentByID(ctx, made.ID)
	if err != nil {
		t.Fatalf("AssignmentByID: %v", err)
	}
	if back.TeamKey != schedTeamKey {
		t.Fatalf("read back team %q, want %q -- the service checks this to decide who may edit",
			back.TeamKey, schedTeamKey)
	}

	moved := schedOtherID
	if _, err := repo.UpdateAssignment(ctx, made.ID,
		domain.UpdateScheduleAssignmentRequest{UserID: &moved}, schedLeadEmail); err != nil {
		t.Fatalf("UpdateAssignment: %v", err)
	}
	// Handing a slot to somebody else is a swap, not a plain edit, and the
	// row says so.
	var source string
	if err := pool.QueryRow(ctx,
		`SELECT source FROM team_schedule_assignment WHERE id = $1::uuid`, made.ID).Scan(&source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if source != "SWAP" {
		t.Fatalf("source %q after moving the slot to another engineer, want SWAP", source)
	}

	note := "done with it"
	if err := repo.DeleteAssignment(ctx, made.ID, schedLeadEmail, &note); err != nil {
		t.Fatalf("DeleteAssignment: %v", err)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE id = $1::uuid`, made.ID); n != 0 {
		t.Fatalf("%d rows left after the delete, want 0", n)
	}

	// Recorded even though both engineers read as "Chamara Perera": the
	// change is detected on who they are, not on what they are called.
	var field, oldV, newV string
	if err := pool.QueryRow(ctx, `
		SELECT field_name, old_value, new_value FROM team_schedule_assignment_activity
		 WHERE assignment_id = $1::uuid AND action = 'UPDATED' AND field_name = 'user'`,
		made.ID).Scan(&field, &oldV, &newV); err != nil {
		t.Fatalf("no UPDATED row for the engineer change: %v", err)
	}
	if oldV != newV {
		t.Fatalf("recorded %q -> %q; this fixture's two engineers share a name, so both sides should read alike", oldV, newV)
	}

	// The whole point of the activity table: the row is gone and its history
	// is not.
	for _, action := range []string{"CREATED", "UPDATED", "DELETED"} {
		if n := countRows(t, pool,
			`SELECT count(*) FROM team_schedule_assignment_activity WHERE assignment_id = $1::uuid AND action = $2`,
			made.ID, action); n == 0 {
			t.Fatalf("no %s activity row survived for the deleted assignment", action)
		}
	}

	acts, err := repo.ActivityForTeam(ctx, schedTeamKey, schedMonday, schedMonday)
	if err != nil {
		t.Fatalf("ActivityForTeam: %v", err)
	}
	if len(acts) < 3 {
		t.Fatalf("got %d activity rows for the team, want at least the 3 changes made", len(acts))
	}
	for _, a := range acts {
		if a.ActorEmail != schedLeadEmail {
			t.Fatalf("activity attributed to %q, want the lead who made the change", a.ActorEmail)
		}
	}
}

// An engineer can be on two teams. A lead of one may clear their day on that
// team; the row the other team put them on is not theirs to touch.
func TestScheduleIntegration_ApplyRangeOnlyClearsTheCallersOwnTeam(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	// Two windows that do not overlap: one person cannot hold the same shift
	// twice on a day, nor two windows sharing hours, so the two teams have to
	// have put them on genuinely different parts of the day.
	morning, evening := "CRE_MORNING", "CRE_EVENING"

	// A second team, and the same engineer rostered on it the same day.
	otherTeamID := "5c8e0000-0000-4000-8000-000000000005"
	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $2, $2, 'cre-abt')
		ON CONFLICT (id) DO NOTHING`, otherTeamID, schedOtherTeam)
	t.Cleanup(func() {
		mustExec(t, pool, `DELETE FROM team_schedule_assignment WHERE team_key = $1`, schedOtherTeam)
	})

	for _, seed := range []struct{ team, code string }{
		{schedTeamKey, morning},
		{schedOtherTeam, evening},
	} {
		if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
			UserID: schedMemberID, TeamKey: seed.team, ShiftCode: seed.code,
			From: schedMonday, To: schedMonday,
		}, schedLeadEmail); err != nil {
			t.Fatalf("seed %s: %v", seed.team, err)
		}
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid AND rota_date = $2::date`,
		schedMemberID, schedMonday); n != 2 {
		t.Fatalf("%d rows on the day, want one per team", n)
	}

	// Clearing on one team leaves the other standing.
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "",
		From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid AND team_key = $2`,
		schedMemberID, schedTeamKey); n != 0 {
		t.Fatalf("%d rows left on the caller's own team, want 0", n)
	}
	if n := countRows(t, pool,
		`SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1::uuid AND team_key = $2`,
		schedMemberID, schedOtherTeam); n != 1 {
		t.Fatalf("%d rows left on the other team, want the 1 it put there", n)
	}
}

// mustExec runs one seeding statement, failing the test on error. It used to
// be borrowed from the project-consumption integration test, which has since
// been removed.
func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// Each rota is offered its own allocations: RnD is SRE's, Migration is CRE's,
// and the rest (Allo-INT, Allo-EXT, the Brazil rotation, all leave) are both
// rotas'. A retired kind is still served, marked, so the days already marked
// with it keep their label.
func TestScheduleIntegration_CatalogueKindsCarryFamilyAndRetired(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)
	cat, err := repo.Catalogue(context.Background())
	if err != nil {
		t.Fatalf("Catalogue: %v", err)
	}
	byCode := map[string]domain.ScheduleAbsenceKind{}
	for _, k := range cat.AbsenceKinds {
		byCode[k.Code] = k
	}
	family := func(code string) string {
		if f := byCode[code].Family; f != nil {
			return *f
		}
		return ""
	}
	for code, want := range map[string]string{
		"RND": "SRE", "MIGRATION": "CRE", "ALLO_INT": "", "ALLO_EXT": "", "ALLO_BR": "", "ANNUAL_LEAVE": "",
	} {
		if _, ok := byCode[code]; !ok {
			t.Fatalf("%s is not in the catalogue", code)
		}
		if got := family(code); got != want {
			t.Errorf("%s family = %q, want %q", code, got, want)
		}
		if byCode[code].Retired {
			t.Errorf("%s is retired, want offered", code)
		}
	}
	for _, code := range []string{"CUSTOMER_ONSITE", "CUSTOMER_OFFSITE", "ONBOARDING"} {
		k, ok := byCode[code]
		if !ok {
			t.Fatalf("retired %s is not served, so days marked with it lose their label", code)
		}
		if !k.Retired {
			t.Errorf("%s is offered, want retired", code)
		}
	}
	if byCode["ALLO_INT"].ShortCode != "Allo-INT" || byCode["ALLO_EXT"].ShortCode != "Allo-EXT" {
		t.Errorf("short codes = %q, %q, want Allo-INT, Allo-EXT",
			byCode["ALLO_INT"].ShortCode, byCode["ALLO_EXT"].ShortCode)
	}
}

// The rota admin role is a schedule permission for internal staff, not a way
// to become internal. Somebody holding it without already being INTERNAL is no
// rota admin, and holding it does not change their user_type.
func TestScheduleIntegration_RotaAdminRoleAloneGrantsNothing(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	teams, err := repo.RotaAdminTeamsFor(ctx, schedOutsiderEmail)
	if err != nil {
		t.Fatalf("RotaAdminTeamsFor: %v", err)
	}
	if len(teams) != 0 {
		t.Fatalf("a non-internal holder of the rota admin role was given %v", teams)
	}
	var userType *string
	if err := pool.QueryRow(ctx, `SELECT user_type::text FROM "user" WHERE id = $1`, schedOutsiderID).Scan(&userType); err != nil {
		t.Fatalf("read user_type: %v", err)
	}
	if userType != nil && *userType == "INTERNAL" {
		t.Fatal("holding cre_rota_admin alone made the user INTERNAL")
	}
}

// Deleting a user must not erase their rota. The roster is a record of who was
// responsible, so the database refuses the delete rather than cascading it.
func TestScheduleIntegration_DeletingAUserWithRotaHistoryIsRefused(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code, From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("ApplyRange: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, schedMemberID)
	if err == nil {
		t.Fatal("deleting a user with rota history succeeded; the history would be gone")
	}
	if !strings.Contains(err.Error(), "23503") && !strings.Contains(err.Error(), "foreign key") {
		t.Fatalf("delete failed for another reason: %v", err)
	}
}

// Two sets of regular hours for one engineer at the same time are refused, the
// same way two overlapping turns are. A turn over regular hours still is not.
func TestScheduleIntegration_OverlappingRegularHoursAreRefused(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	place := func(code string) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO team_schedule_assignment
			  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, created_by, updated_by)
			SELECT $1::uuid, $2::uuid, $3, s.id, s.zone_id, s.tier, $4::date,
			       ($4::date::timestamp + make_interval(mins => s.start_minute)) AT TIME ZONE s.authoring_time_zone,
			       ($4::date::timestamp + make_interval(mins => s.end_minute))   AT TIME ZONE s.authoring_time_zone,
			       'fixture', 'fixture'
			  FROM team_schedule_shift s WHERE s.code = $5`,
			schedMemberID, schedTeamID, schedTeamKey, schedMonday, code)
		return err
	}
	if err := place("SRE_TZ1_REGULAR"); err != nil {
		t.Fatalf("first regular hours: %v", err)
	}
	if err := place("SRE_TZ1_L1"); err != nil {
		t.Fatalf("a turn over regular hours was refused: %v", err)
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT regular_clash"); err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	err = place("SRE_TZ2_REGULAR") // 12:00-21:00, overlapping TZ1's 06:00-15:00
	if err == nil {
		t.Fatal("two overlapping sets of regular hours were accepted")
	}
	if !strings.Contains(err.Error(), "no_overlap_regular") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// A shift that assignments already use cannot have its hours changed out from
// under them; a label edit is fine, and a migration that recomputes the rows
// itself may say so explicitly.
func TestScheduleIntegration_AUsedShiftsHoursAreFrozen(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	code := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: code, From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("ApplyRange: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE team_schedule_shift SET label = label || ' (renamed)' WHERE code = $1`, code); err != nil {
		t.Fatalf("a label edit on a used shift was refused: %v", err)
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT hours"); err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	_, err = tx.Exec(ctx, `UPDATE team_schedule_shift SET end_minute = end_minute - 30 WHERE code = $1`, code)
	if err == nil {
		t.Fatal("the hours of a used shift changed; its assignments now disagree with it")
	}
	if !strings.Contains(err.Error(), "used by existing assignments") {
		t.Fatalf("refused for another reason: %v", err)
	}
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT hours"); err != nil {
		t.Fatalf("rollback to savepoint: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL team_schedule.allow_shift_rewrite = 'on'`); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE team_schedule_shift SET end_minute = end_minute - 30 WHERE code = $1`, code); err != nil {
		t.Fatalf("an explicitly allowed rewrite was refused: %v", err)
	}
}

// A tag whose derived code is already taken is refused naming the tag that
// actually holds it, not the caller's own label.
func TestScheduleIntegration_CreateAbsenceKindConflictNamesTheExistingTag(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM team_schedule_absence_kind WHERE code = 'INTEGRATION_CLASH_TAG'`)
	})
	_, _ = pool.Exec(ctx, `DELETE FROM team_schedule_absence_kind WHERE code = 'INTEGRATION_CLASH_TAG'`)

	first := domain.CreateScheduleAbsenceKindRequest{Label: "Integration clash tag", ShortCode: "ICT1", Bucket: "ALLOCATION", ColourToken: "INT"}
	if _, err := repo.CreateAbsenceKind(ctx, "INTEGRATION_CLASH_TAG", first, schedLeadEmail); err != nil {
		t.Fatalf("first create: %v", err)
	}
	second := domain.CreateScheduleAbsenceKindRequest{Label: "Integration-clash tag!", ShortCode: "ICT2", Bucket: "ALLOCATION", ColourToken: "INT"}
	_, err := repo.CreateAbsenceKind(ctx, "INTEGRATION_CLASH_TAG", second, schedLeadEmail)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want a ConflictError, got %v", err)
	}
	if !strings.Contains(conflict.Msg, "Integration clash tag") || !strings.Contains(conflict.Msg, "too close") {
		t.Fatalf("the message should name the existing tag: %q", conflict.Msg)
	}
}

// With IncludeOvernight a day's view carries the night block rostered for the
// day before -- it is still running this morning -- and nothing from earlier,
// since no window lasts past the day after its own. Pins the narrowed
// `rota_date = From - 1` to the behaviour it replaced.
func TestScheduleIntegration_OvernightViewCarriesOnlyTheNightBefore(t *testing.T) {
	repo, _ := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	// SRE_TZ3_REGULAR runs 21:00-06:00, so Monday's block ends on Tuesday.
	if _, err := repo.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ3_REGULAR", From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("ApplyRange: %v", err)
	}
	found := func(day string) bool {
		rows, err := repo.SearchAssignments(ctx, domain.SearchScheduleAssignmentsRequest{
			From: day, To: day, TeamKeys: []string{schedTeamKey}, UserID: schedMemberID, IncludeOvernight: true,
		})
		if err != nil {
			t.Fatalf("SearchAssignments(%s): %v", day, err)
		}
		for _, a := range rows {
			if a.RotaDate == schedMonday && a.ShiftCode == "SRE_TZ3_REGULAR" {
				return true
			}
		}
		return false
	}
	if !found("2026-09-22") {
		t.Fatal("Tuesday's view dropped Monday's night block, which runs until 06:00 Tuesday")
	}
	if found("2026-09-23") {
		t.Fatal("Wednesday's view carried Monday's block, which ended on Tuesday")
	}
}
