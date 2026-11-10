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

// HolidayAvailabilityRepository against a real Postgres with every migration
// applied (0283 included). Skipped without ENTITY_TEST_DATABASE_URL:
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/repository/ -run HolidayAvailability

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	phTeamID      = "0f000000-0000-4000-8000-0000000000a1"
	phTeamKey     = "phtest-abt"
	phLeadID      = "0f000000-0000-4000-8000-0000000000b1"
	phMemberID    = "0f000000-0000-4000-8000-0000000000b2"
	phOnLeaveID   = "0f000000-0000-4000-8000-0000000000b3"
	phCountry     = "Holidayland (test)"
	phActor       = "holiday-availability-test"
	phLeadEmail   = "ph-lead@example.com"
	phMemberEmail = "ph-member@example.com"
)

func phPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedHolidayTeam writes a CRE ABT team with a lead, a member and a member on
// leave on day, all located in phCountry, which has a holiday on day; and a
// Morning turn for the member that day. Everything is removed afterwards.
func seedHolidayTeam(t *testing.T, pool *pgxpool.Pool, day string) {
	t.Helper()
	ctx := context.Background()
	users := []string{phLeadID, phMemberID, phOnLeaveID}
	cleanup := func() {
		for _, q := range []string{
			`DELETE FROM team_schedule_holiday_tag_placed WHERE user_id = ANY($1::uuid[])`,
			`DELETE FROM team_schedule_absence_activity WHERE user_id = ANY($1::uuid[])`,
			`DELETE FROM team_schedule_absence WHERE user_id = ANY($1::uuid[])`,
			`DELETE FROM team_schedule_assignment WHERE user_id = ANY($1::uuid[])`,
			`DELETE FROM team_schedule_rota_marker WHERE user_id = ANY($1::uuid[])`,
			`DELETE FROM team_schedule_person_location WHERE user_id = ANY($1::uuid[])`,
			`DELETE FROM team_member WHERE user_id = ANY($1::uuid[])`,
		} {
			if _, err := pool.Exec(ctx, q, users); err != nil {
				t.Fatalf("clean: %s: %v", q, err)
			}
		}
		for _, q := range []string{
			`DELETE FROM team_schedule_holiday_confirmation WHERE team_key = $1`,
			`DELETE FROM team_schedule_holiday_reminder_sent WHERE team_key = $1`,
			`DELETE FROM team WHERE key = $1`,
		} {
			if _, err := pool.Exec(ctx, q, phTeamKey); err != nil {
				t.Fatalf("clean: %s: %v", q, err)
			}
		}
		if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE id = ANY($1::uuid[])`, users); err != nil {
			t.Fatalf("clean users: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM team_schedule_public_holiday WHERE country = $1`, phCountry); err != nil {
			t.Fatalf("clean holiday: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	exec := func(q string, args ...any) {
		t.Helper()
		tag, err := pool.Exec(ctx, q, args...)
		if err != nil {
			t.Fatalf("seed: %s: %v", q, err)
		}
		if tag.RowsAffected() == 0 {
			t.Fatalf("seed wrote nothing: %s", q)
		}
	}
	now := time.Now()
	exec(`INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, type, key)
	      VALUES ($1, $2, $2, $3, $3, 'PH Test', 'CRE-ABT', $4)`, phTeamID, now, phActor, phTeamKey)
	for _, u := range []struct{ id, email, first, role string }{
		{phLeadID, phLeadEmail, "Lena", "lead"},
		{phMemberID, phMemberEmail, "Mo", "engineer"},
		{phOnLeaveID, "ph-leave@example.com", "Lia", "engineer"},
	} {
		exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, first_name, last_name, is_active)
		      VALUES ($1, $2, $2, $3, $3, $4, 'Test', TRUE)`, u.id, now, u.email, u.first)
		// user_type is derived from roles by a trigger on insert.
		exec(`UPDATE "user" SET user_type = 'INTERNAL'::user_type_enum WHERE id = $1`, u.id)
		exec(`INSERT INTO team_member (created_on, updated_on, created_by, updated_by, team_id, user_id, role)
		      VALUES ($1, $1, $2, $2, $3, $4, $5)`, now, phActor, phTeamID, u.id, u.role)
		exec(`INSERT INTO team_schedule_person_location (user_id, location) VALUES ($1, $2)`, u.id, phCountry)
	}
	exec(`INSERT INTO team_schedule_rota_marker (created_by, updated_by, user_id, team_key) VALUES ($1, $1, $2, $3)`,
		phActor, phMemberID, phTeamKey)
	exec(`INSERT INTO team_schedule_public_holiday (country, holiday_date, title) VALUES ($1, $2::date, 'Test Day')`, phCountry, day)
	exec(`INSERT INTO team_schedule_absence (created_by, updated_by, user_id, team_key, kind_id, starts_on, ends_on)
	      SELECT $1, $1, $2, $3, id, $4::date, $4::date FROM team_schedule_absence_kind WHERE code = 'ANNUAL_LEAVE'`,
		phActor, phOnLeaveID, phTeamKey, day)
	exec(`INSERT INTO team_schedule_assignment (created_by, updated_by, user_id, team_id, team_key, shift_id, rota_date, starts_at, ends_at)
	      SELECT $1, $1, $2, $3, $4, s.id, $5::date, ($5::date + time '06:00') AT TIME ZONE 'Asia/Colombo',
	             ($5::date + time '09:00') AT TIME ZONE 'Asia/Colombo'
	        FROM team_schedule_shift s WHERE s.code = 'CRE_MORNING'`,
		phActor, phMemberID, phTeamID, phTeamKey, day)
}

func TestHolidayAvailabilityRepository_PlaceReadConfirmRemind(t *testing.T) {
	pool := phPool(t)
	ctx := context.Background()
	day := time.Now().AddDate(0, 0, 10)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	d := day.Format(time.DateOnly)
	seedHolidayTeam(t, pool, d)
	repo := repository.NewHolidayAvailabilityRepository(pool)

	members, err := repo.CREMembers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, m := range members {
		if m.Location == phCountry && len(m.TeamKeys) > 0 && m.TeamKeys[0] == phTeamKey {
			found++
		}
	}
	if found != 3 {
		t.Fatalf("CRE members of the test team: %d, want 3", found)
	}

	place := func(user string) bool {
		t.Helper()
		ok, err := repo.PlaceHolidayTag(ctx, domain.HolidayTagPlacement{UserID: user, TeamKey: phTeamKey, Date: d, Country: phCountry, Title: "Test Day"})
		if err != nil {
			t.Fatalf("place %s: %v", user, err)
		}
		return ok
	}
	if !place(phLeadID) || !place(phMemberID) {
		t.Fatal("a member with no tag that day was not tagged")
	}
	if place(phMemberID) {
		t.Error("the same day was tagged twice")
	}
	if place(phOnLeaveID) {
		t.Error("a member on leave was tagged (leave wins)")
	}

	read := func() domain.HolidayPlacedRow {
		t.Helper()
		rows, err := repo.PlacedBetween(ctx, d, d, []string{phTeamKey})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.UserID == phMemberID {
				return r
			}
		}
		t.Fatalf("no placement for the member: %+v", rows)
		return domain.HolidayPlacedRow{}
	}
	mo := read()
	if mo.AbsenceID == "" || mo.Title != "Test Day" || mo.TeamName != "PH Test" {
		t.Fatalf("placed row %+v", mo)
	}
	turns, err := repo.TurnsBetween(ctx, d, d, []string{phTeamKey})
	if err != nil || len(turns) != 1 || !turns[0].Away {
		t.Fatalf("the tagged member's Morning turn: %+v, %v (want one, away)", turns, err)
	}

	// The lead makes Mo available: the tag goes, the turn is covered, and the
	// placement stays so the tag is never placed again.
	if _, err := pool.Exec(ctx, `DELETE FROM team_schedule_absence WHERE id = $1::uuid`, mo.AbsenceID); err != nil {
		t.Fatal(err)
	}
	if read().AbsenceID != "" {
		t.Error("a removed tag still reads as standing")
	}
	if turns, _ = repo.TurnsBetween(ctx, d, d, []string{phTeamKey}); turns[0].Away {
		t.Error("an available member's turn still reads as uncovered")
	}
	if place(phMemberID) {
		t.Error("a tag the lead removed was placed again")
	}

	if _, err := repo.Confirm(ctx, phTeamKey, d, phLeadEmail); err != nil {
		t.Fatal(err)
	}
	cs, err := repo.ConfirmationsBetween(ctx, d, d, []string{phTeamKey})
	if err != nil || len(cs) != 1 || cs[0].ConfirmedBy != phLeadEmail {
		t.Fatalf("confirmations %+v, %v", cs, err)
	}
	if ok, err := repo.Withdraw(ctx, phTeamKey, d); err != nil || !ok {
		t.Fatalf("withdraw: %v %v", ok, err)
	}

	who, err := repo.ReminderRecipients(ctx, phTeamKey)
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{}
	for _, w := range who {
		roles[w.Email] = w.Role
	}
	if roles[phLeadEmail] != "lead" || roles[phMemberEmail] != "rota_manager" || len(roles) != 2 {
		t.Fatalf("recipients %+v, want the lead and the rota manager", who)
	}

	if err := repo.RecordReminder(ctx, phTeamKey, d, domain.HolidayReminderNotice); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordReminder(ctx, phTeamKey, d, domain.HolidayReminderNotice); err != nil {
		t.Fatalf("recording twice: %v", err)
	}
	sent, err := repo.SentStages(ctx, d, d)
	if err != nil || len(sent[repository.HolidayStageKey(phTeamKey, d)]) != 1 {
		t.Fatalf("sent %v, %v", sent, err)
	}

	// The calendar drops the holiday: the lead's untouched tag goes, and the
	// placements are forgotten.
	if _, err := pool.Exec(ctx, `DELETE FROM team_schedule_public_holiday WHERE country = $1`, phCountry); err != nil {
		t.Fatal(err)
	}
	removed, err := repo.RemoveCancelledHolidayTags(ctx, d)
	if err != nil || removed != 1 {
		t.Fatalf("removed %d, %v; want the lead's one tag", removed, err)
	}
	if rows, _ := repo.PlacedBetween(ctx, d, d, []string{phTeamKey}); len(rows) != 0 {
		t.Errorf("placements of a cancelled holiday kept: %+v", rows)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM team_schedule_absence a JOIN team_schedule_absence_kind k ON k.id = a.kind_id
		WHERE a.user_id = $1 AND k.code = 'PH'`, phLeadID).Scan(&left); err != nil || left != 0 {
		t.Errorf("the lead's PH tag still there: %d, %v", left, err)
	}
}
