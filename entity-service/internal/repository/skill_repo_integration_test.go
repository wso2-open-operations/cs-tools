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

// The skills SQL against a live PostgreSQL: the profile writes and their
// history, the duplicate guard, the inherited rows, the manager's filters and
// the catalogue edits, read back from the tables rather than taken from a
// return value. Skipped unless ENTITY_TEST_DATABASE_URL is set; apply every
// migration first (see schedule_repo_integration_test.go):
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run SkillIntegration ./internal/repository/

package repository_test

import (
	"context"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// Fixture ids, distinct from every other integration file's.
const (
	skScaleA   = "5e1e0000-0000-4000-8000-0000000000a1"
	skScaleB   = "5e1e0000-0000-4000-8000-0000000000a2"
	skLvlA20   = "5e1e0000-0000-4000-8000-0000000000b1"
	skLvlA50   = "5e1e0000-0000-4000-8000-0000000000b2"
	skLvlA100  = "5e1e0000-0000-4000-8000-0000000000b3"
	skLvlB20   = "5e1e0000-0000-4000-8000-0000000000b4"
	skCatTop   = "5e1e0000-0000-4000-8000-0000000000c1"
	skCatLeaf  = "5e1e0000-0000-4000-8000-0000000000c2"
	skCatOther = "5e1e0000-0000-4000-8000-0000000000c3"
	skGo       = "5e1e0000-0000-4000-8000-0000000000d1"
	skRust     = "5e1e0000-0000-4000-8000-0000000000d2"
	skOld      = "5e1e0000-0000-4000-8000-0000000000d3"
	skNoScale  = "5e1e0000-0000-4000-8000-0000000000d4"
	skJane     = "5e1e0000-0000-4000-8000-0000000000e1"
	skLead     = "5e1e0000-0000-4000-8000-0000000000e2"
	skExt      = "5e1e0000-0000-4000-8000-0000000000e3"
	skGone     = "5e1e0000-0000-4000-8000-0000000000e4"
	skSreLead  = "5e1e0000-0000-4000-8000-0000000000e5"
	skOddLead  = "5e1e0000-0000-4000-8000-0000000000e6"
	skTeamCRE  = "5e1e0000-0000-4000-8000-0000000000f1"
	skTeamSRE  = "5e1e0000-0000-4000-8000-0000000000f2"
	skTeamOdd  = "5e1e0000-0000-4000-8000-0000000000f3"

	skJaneEmail = "jane.doe.skills@example.test"
	skLeadEmail = "lead.skills@example.test"
	skActor     = "skills.actor@example.test"
)

var skUsers = []string{skJane, skLead, skExt, skGone, skSreLead, skOddLead}
var skSkills = []string{skGo, skRust, skOld, skNoScale}

func skExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("seed (%.60s): %v", sql, err)
	}
}

func skCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count (%.60s): %v", sql, err)
	}
	return n
}

func skRemoveFixtures(pool *pgxpool.Pool) {
	ctx := context.Background()
	cats := `('` + skCatTop + `','` + skCatLeaf + `','` + skCatOther + `')`
	scales := `('` + skScaleA + `','` + skScaleB + `')`
	teams := `('` + skTeamCRE + `','` + skTeamSRE + `','` + skTeamOdd + `')`
	// Each statement names only the parameters it uses: the driver refuses
	// extra ones, and a swallowed error would leave the fixtures behind.
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM user_skill_history WHERE user_id = ANY($1::uuid[]) OR skill_id = ANY($2::uuid[]) OR skill_id IN (SELECT id FROM skill WHERE name LIKE 'sktest %')`, []any{skUsers, skSkills}},
		{`DELETE FROM user_skill WHERE user_id = ANY($1::uuid[]) OR skill_id = ANY($2::uuid[]) OR skill_id IN (SELECT id FROM skill WHERE name LIKE 'sktest %')`, []any{skUsers, skSkills}},
		{`DELETE FROM skill WHERE id = ANY($1::uuid[]) OR name LIKE 'sktest %'`, []any{skSkills}},
		{`DELETE FROM skill_level WHERE level_type_id IN ` + scales, nil},
		{`DELETE FROM skill_level_type WHERE id IN ` + scales, nil},
		{`DELETE FROM skill_category WHERE id IN ` + cats, nil},
		{`DELETE FROM team_member WHERE user_id = ANY($1::uuid[])`, []any{skUsers}},
		{`DELETE FROM user_role WHERE user_id = ANY($1::uuid[])`, []any{skUsers}},
		{`DELETE FROM team WHERE id IN ` + teams, nil},
		{`DELETE FROM "user" WHERE id = ANY($1::uuid[])`, []any{skUsers}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			panic("skill fixture cleanup: " + err.Error())
		}
	}
}

// newSkillIntegration connects and rebuilds this file's fixtures from scratch,
// so the tests are order-independent and rerunnable.
func newSkillIntegration(t *testing.T) (repository.SkillRepository, *pgxpool.Pool) {
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
	skRemoveFixtures(pool)
	t.Cleanup(func() { skRemoveFixtures(pool) })

	const audit = `NOW(), NOW(), 'fixture', 'fixture'`
	skExec(t, pool, `INSERT INTO skill_level_type (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, `+audit+`, 'sktest scale A'), ($2, `+audit+`, 'sktest scale B')`, skScaleA, skScaleB)
	for _, l := range []struct {
		id, name, typ string
		value         int
	}{{skLvlA20, "Beginner", skScaleA, 20}, {skLvlA50, "Intermediate", skScaleA, 50}, {skLvlA100, "Expert", skScaleA, 100}, {skLvlB20, "Basic", skScaleB, 20}} {
		skExec(t, pool, `INSERT INTO skill_level (id, created_on, updated_on, created_by, updated_by, name, value, level_type_id) VALUES ($1, `+audit+`, $2, $3, $4)`, l.id, l.name, l.value, l.typ)
	}
	skExec(t, pool, `INSERT INTO skill_category (id, created_on, updated_on, created_by, updated_by, name, is_lowest_category) VALUES ($1, `+audit+`, 'sktest top', false)`, skCatTop)
	skExec(t, pool, `INSERT INTO skill_category (id, created_on, updated_on, created_by, updated_by, name, is_lowest_category, parent_id) VALUES ($1, `+audit+`, 'sktest leaf', true, $2), ($3, `+audit+`, 'sktest other', true, NULL)`, skCatLeaf, skCatTop, skCatOther)
	for _, s := range []struct {
		id, name string
		active   bool
		typ      *string
	}{{skGo, "sktest Go", true, ptr(skScaleA)}, {skRust, "sktest Rust", true, ptr(skScaleB)}, {skOld, "sktest Old", false, ptr(skScaleA)}, {skNoScale, "sktest NoScale", true, nil}} {
		skExec(t, pool, `INSERT INTO skill (id, created_on, updated_on, created_by, updated_by, name, active, keywords, level_type_id) VALUES ($1, `+audit+`, $2, $3, 'golang systems', $4)`, s.id, s.name, s.active, s.typ)
	}
	skExec(t, pool, `INSERT INTO skill_category_link (created_on, updated_on, created_by, updated_by, skill_id, category_id) VALUES (`+audit+`, $1, $2)`, skGo, skCatLeaf)

	for _, u := range []struct {
		id, email, kind string
		active          bool
	}{
		{skJane, skJaneEmail, "INTERNAL", true}, {skLead, skLeadEmail, "INTERNAL", true},
		{skExt, "ext.skills@example.test", "EXTERNAL", true}, {skGone, "gone.skills@example.test", "INTERNAL", false},
		{skSreLead, "sre.lead.skills@example.test", "INTERNAL", true}, {skOddLead, "odd.lead.skills@example.test", "INTERNAL", true},
	} {
		skExec(t, pool, `INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, is_active, user_type) VALUES ($1, NOW(), NOW(), $2, $3, $2, $4, $5::user_type_enum)`,
			u.id, u.email, "Name "+u.email, u.active, u.kind)
	}
	// A user's type is derived from their roles (migration 0011), so being
	// internal staff means holding the internal role, which a database built
	// from migrations alone does not have: the data sync seeds it.
	skExec(t, pool, `
		INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name, description)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', 'internal', 'Internal staff')
		ON CONFLICT (name) DO NOTHING`)
	skExec(t, pool, `
		INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
		SELECT gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', u, r.id
		  FROM unnest($1::uuid[]) AS u, role r WHERE r.name = 'internal'`,
		[]string{skJane, skLead, skGone, skSreLead, skOddLead})
	for _, tm := range []struct{ id, typ string }{{skTeamCRE, "cre-abt"}, {skTeamSRE, "sre-abt"}, {skTeamOdd, "sales-abt"}} {
		skExec(t, pool, `INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type) VALUES ($1::uuid, `+audit+`, $1::text, $1::text, $2)`, tm.id, tm.typ)
	}
	skExec(t, pool, `INSERT INTO team_member (created_on, updated_on, created_by, updated_by, team_id, user_id, role) VALUES (`+audit+`, $1, $2, 'lead'), (`+audit+`, $3, $4, 'lead'), (`+audit+`, $5, $6, 'lead'), (`+audit+`, $1, $7, 'engineer')`,
		skTeamCRE, skLead, skTeamSRE, skSreLead, skTeamOdd, skOddLead, skJane)
	return repository.NewSkillRepository(pool), pool
}

func ptr[T any](v T) *T { return &v }

type histRow struct {
	from, to *string
	by       *string
}

func skHistory(t *testing.T, pool *pgxpool.Pool, user, skill string) []histRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT level_from_id::text, level_to_id::text, created_by FROM user_skill_history
		 WHERE user_id = $1 AND skill_id = $2 ORDER BY created_on, id`, user, skill)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	defer rows.Close()
	var out []histRow
	for rows.Next() {
		var h histRow
		if err := rows.Scan(&h.from, &h.to, &h.by); err != nil {
			t.Fatal(err)
		}
		out = append(out, h)
	}
	return out
}

func eq(a *string, b string) bool { return a != nil && *a == b }

func TestSkillIntegration_AddChangeRemoveWriteRowsAndHistory(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()

	res, err := repo.AddUserSkill(ctx, skJane, skGo, skLvlA20, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeDone || res.Row == nil || res.Row.Level == nil || res.Row.Level.ID != skLvlA20 {
		t.Fatalf("AddUserSkill = %+v, %v", res, err)
	}
	// Read back from the table, not from the return value.
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2 AND active AND NOT inherited AND skill_level_id = $3 AND created_by = $4`, skJane, skGo, skLvlA20, skActor); n != 1 {
		t.Fatalf("user_skill rows after add = %d, want 1", n)
	}

	res, err = repo.SetUserSkillLevel(ctx, skJane, skGo, skLvlA100, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeDone || res.Row.Level.ID != skLvlA100 {
		t.Fatalf("SetUserSkillLevel = %+v, %v", res, err)
	}
	// The same level again changes nothing and writes no history.
	res, err = repo.SetUserSkillLevel(ctx, skJane, skGo, skLvlA100, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeUnchanged {
		t.Fatalf("same level = %+v, %v", res, err)
	}
	res, err = repo.RemoveUserSkill(ctx, skJane, skGo, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeDone {
		t.Fatalf("RemoveUserSkill = %+v, %v", res, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2`, skJane, skGo); n != 0 {
		t.Fatalf("user_skill rows after remove = %d, want 0", n)
	}

	h := skHistory(t, pool, skJane, skGo)
	if len(h) != 3 {
		t.Fatalf("history rows = %d, want 3 (first set, change, removal)", len(h))
	}
	if h[0].from != nil || !eq(h[0].to, skLvlA20) {
		t.Fatalf("first set = %+v", h[0])
	}
	if !eq(h[1].from, skLvlA20) || !eq(h[1].to, skLvlA100) {
		t.Fatalf("change = %+v", h[1])
	}
	if !eq(h[2].from, skLvlA100) || h[2].to != nil {
		t.Fatalf("removal = %+v", h[2])
	}
	for _, r := range h {
		if !eq(r.by, skActor) {
			t.Fatalf("history actor = %v, want %s", r.by, skActor)
		}
	}

	// And through the API read.
	page, total, err := repo.ListUserSkillHistory(ctx, skJane, "", 10, 0)
	if err != nil || total != 3 || len(page) != 3 || page[0].LevelTo != nil || page[0].SkillName != "sktest Go" {
		t.Fatalf("ListUserSkillHistory = %+v, %d, %v (newest first: the removal)", page, total, err)
	}
	// Removing what is no longer held finds nothing, writes nothing.
	res, _ = repo.RemoveUserSkill(ctx, skJane, skGo, skActor)
	if res.Outcome != domain.SkillOutcomeNotHeld || len(skHistory(t, pool, skJane, skGo)) != 3 {
		t.Fatalf("remove again = %+v", res)
	}
}

func TestSkillIntegration_AddingTwiceNeverMakesASecondRow(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()
	if _, err := repo.AddUserSkill(ctx, skJane, skGo, skLvlA20, skActor); err != nil {
		t.Fatal(err)
	}
	res, err := repo.AddUserSkill(ctx, skJane, skGo, skLvlA50, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeAlreadyHeld {
		t.Fatalf("second add = %+v, %v", res, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2`, skJane, skGo); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
	if n := len(skHistory(t, pool, skJane, skGo)); n != 1 {
		t.Fatalf("history rows = %d, want 1: a refused add records nothing", n)
	}
}

// With no unique key on (user_id, skill_id), only the transaction's lock keeps
// concurrent adds from each inserting a row.
func TestSkillIntegration_ConcurrentAddsMakeOneRow(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	outcomes := make([]domain.SkillOutcome, 12)
	for i := range outcomes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := repo.AddUserSkill(ctx, skJane, skGo, skLvlA20, skActor)
			if err != nil {
				t.Errorf("add: %v", err)
			}
			outcomes[i] = res.Outcome
		}()
	}
	wg.Wait()
	done := 0
	for _, o := range outcomes {
		if o == domain.SkillOutcomeDone {
			done++
		}
	}
	if done != 1 {
		t.Fatalf("%d adds succeeded, want exactly 1 (%v)", done, outcomes)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2`, skJane, skGo); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
	if n := len(skHistory(t, pool, skJane, skGo)); n != 1 {
		t.Fatalf("history rows = %d, want 1", n)
	}
}

// swapScaleUnderWrite plays the admin's scale change racing a profile write,
// with the interleaving fixed rather than left to timing: a transaction takes
// the skill row's lock and moves the skill to scale B (as UpdateSkill does when
// nobody holds a level yet), then the profile write starts and must queue behind
// it, and only then does the admin commit. The write's own pre-check, done
// earlier against scale A, is not part of this: the repository has to catch the
// swap by itself.
func swapScaleUnderWrite(t *testing.T, pool *pgxpool.Pool, write func() (domain.SkillWriteResult, error)) domain.SkillWriteResult {
	t.Helper()
	return changeSkillUnderWrite(t, pool, `UPDATE skill SET level_type_id = $2 WHERE id = $1`, skScaleB, write)
}

// deactivateUnderWrite is the same with the admin deactivating the skill.
func deactivateUnderWrite(t *testing.T, pool *pgxpool.Pool, write func() (domain.SkillWriteResult, error)) domain.SkillWriteResult {
	t.Helper()
	return changeSkillUnderWrite(t, pool, `UPDATE skill SET active = false WHERE id = $1 AND $2::text IS NOT NULL`, skScaleB, write)
}

func changeSkillUnderWrite(t *testing.T, pool *pgxpool.Pool, change, arg string, write func() (domain.SkillWriteResult, error)) domain.SkillWriteResult {
	t.Helper()
	ctx := context.Background()
	admin, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = admin.Rollback(ctx) }()
	if _, err := admin.Exec(ctx, `SELECT 1 FROM skill WHERE id = $1 FOR UPDATE`, skGo); err != nil {
		t.Fatalf("lock skill: %v", err)
	}
	if _, err := admin.Exec(ctx, change, skGo, arg); err != nil {
		t.Fatalf("change skill: %v", err)
	}
	type result struct {
		res domain.SkillWriteResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		res, err := write()
		done <- result{res, err}
	}()
	// The write must be waiting on the skill row, not finished.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if skCount(t, pool, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query ILIKE '%FROM skill WHERE id%FOR SHARE%'`) > 0 {
			break
		}
		select {
		case r := <-done:
			t.Fatalf("the write finished (%+v, %v) without waiting for the skill row lock", r.res, r.err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the write never waited on the skill row lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := admin.Commit(ctx); err != nil {
		t.Fatalf("commit swap: %v", err)
	}
	r := <-done
	if r.err != nil {
		t.Fatalf("write: %v", r.err)
	}
	return r.res
}

func TestSkillIntegration_AddRacingAScaleChangeLeavesNoLevelOffTheScale(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	res := swapScaleUnderWrite(t, pool, func() (domain.SkillWriteResult, error) {
		return repo.AddUserSkill(context.Background(), skJane, skGo, skLvlA20, skActor)
	})
	if res.Outcome != domain.SkillOutcomeLevelNotInScale {
		t.Fatalf("outcome = %v, want LevelNotInScale", res.Outcome)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2`, skJane, skGo); n != 0 {
		t.Fatalf("user_skill rows = %d, want 0", n)
	}
	if n := len(skHistory(t, pool, skJane, skGo)); n != 0 {
		t.Fatalf("history rows = %d, want 0", n)
	}
	// No user_skill level may sit outside its skill's scale.
	if n := skCount(t, pool, `
		SELECT count(*) FROM user_skill us
		  JOIN skill s ON s.id = us.skill_id
		  JOIN skill_level sl ON sl.id = us.skill_level_id
		 WHERE us.skill_id = $1 AND sl.level_type_id <> s.level_type_id`, skGo); n != 0 {
		t.Fatalf("%d levels off the scale", n)
	}
}

func TestSkillIntegration_ReviveRacingAScaleChangeLeavesTheInactiveRowAlone(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	skExec(t, pool, `INSERT INTO user_skill (created_on, updated_on, user_id, skill_id, skill_level_id, active, inherited) VALUES (NOW(), NOW(), $1, $2, $3, false, false)`, skJane, skGo, skLvlA20)
	res := swapScaleUnderWrite(t, pool, func() (domain.SkillWriteResult, error) {
		return repo.AddUserSkill(context.Background(), skJane, skGo, skLvlA50, skActor)
	})
	if res.Outcome != domain.SkillOutcomeLevelNotInScale {
		t.Fatalf("outcome = %v, want LevelNotInScale", res.Outcome)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2 AND NOT active AND skill_level_id = $3`, skJane, skGo, skLvlA20); n != 1 {
		t.Fatalf("the inactive row was changed")
	}
}

func TestSkillIntegration_SetLevelRacingAScaleChangeKeepsTheHeldLevel(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	skExec(t, pool, `INSERT INTO user_skill (created_on, updated_on, user_id, skill_id, skill_level_id, active, inherited) VALUES (NOW(), NOW(), $1, $2, $3, true, false)`, skJane, skGo, skLvlA20)
	res := swapScaleUnderWrite(t, pool, func() (domain.SkillWriteResult, error) {
		return repo.SetUserSkillLevel(context.Background(), skJane, skGo, skLvlA50, skActor)
	})
	if res.Outcome != domain.SkillOutcomeLevelNotInScale {
		t.Fatalf("outcome = %v, want LevelNotInScale", res.Outcome)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2 AND skill_level_id = $3`, skJane, skGo, skLvlA20); n != 1 {
		t.Fatal("the held level was changed")
	}
	if n := len(skHistory(t, pool, skJane, skGo)); n != 0 {
		t.Fatalf("history rows = %d, want 0", n)
	}
}

func TestSkillIntegration_AddRacingADeactivationAddsNothing(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	res := deactivateUnderWrite(t, pool, func() (domain.SkillWriteResult, error) {
		return repo.AddUserSkill(context.Background(), skJane, skGo, skLvlA20, skActor)
	})
	if res.Outcome != domain.SkillOutcomeSkillInactive {
		t.Fatalf("outcome = %v, want SkillInactive", res.Outcome)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2`, skJane, skGo); n != 0 {
		t.Fatalf("user_skill rows = %d, want 0", n)
	}
	if n := len(skHistory(t, pool, skJane, skGo)); n != 0 {
		t.Fatalf("history rows = %d, want 0", n)
	}
}

func TestSkillIntegration_ReviveRacingADeactivationLeavesTheInactiveRowAlone(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	skExec(t, pool, `INSERT INTO user_skill (created_on, updated_on, user_id, skill_id, skill_level_id, active, inherited) VALUES (NOW(), NOW(), $1, $2, $3, false, false)`, skJane, skGo, skLvlA20)
	res := deactivateUnderWrite(t, pool, func() (domain.SkillWriteResult, error) {
		return repo.AddUserSkill(context.Background(), skJane, skGo, skLvlA20, skActor)
	})
	if res.Outcome != domain.SkillOutcomeSkillInactive {
		t.Fatalf("outcome = %v, want SkillInactive", res.Outcome)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2 AND NOT active`, skJane, skGo); n != 1 {
		t.Fatal("the inactive row was revived")
	}
	if n := len(skHistory(t, pool, skJane, skGo)); n != 0 {
		t.Fatalf("history rows = %d, want 0", n)
	}
}

func TestSkillIntegration_SetLevelStillWorksOnASkillDeactivatedAfterItWasHeld(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	skExec(t, pool, `INSERT INTO user_skill (created_on, updated_on, user_id, skill_id, skill_level_id, active, inherited) VALUES (NOW(), NOW(), $1, $2, $3, true, false)`, skJane, skGo, skLvlA20)
	skExec(t, pool, `UPDATE skill SET active = false WHERE id = $1`, skGo)
	res, err := repo.SetUserSkillLevel(context.Background(), skJane, skGo, skLvlA50, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeDone {
		t.Fatalf("set level on a deactivated held skill = %v, %v; want Done", res.Outcome, err)
	}
}

// The other order: a write in flight holds the skill row, so a scale change
// waits for it and then finds the level it wrote.
func TestSkillIntegration_ScaleChangeAfterAnAddIsRefusedAsInUse(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()
	if res, err := repo.AddUserSkill(ctx, skJane, skGo, skLvlA20, skActor); err != nil || res.Outcome != domain.SkillOutcomeDone {
		t.Fatalf("add = %+v, %v", res, err)
	}
	upd, err := repo.UpdateSkill(ctx, skGo, domain.UpdateSkillRequest{LevelTypeID: ptr(skScaleB)}, skActor)
	if err != nil || upd.Outcome != domain.SkillOutcomeLevelsInUse {
		t.Fatalf("UpdateSkill = %+v, %v", upd, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill WHERE id = $1 AND level_type_id = $2`, skGo, skScaleA); n != 1 {
		t.Fatal("the scale was changed")
	}
}

func TestSkillIntegration_AnInactiveOwnRowIsRevivedNotDuplicated(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	skExec(t, pool, `INSERT INTO user_skill (created_on, updated_on, user_id, skill_id, skill_level_id, active, inherited) VALUES (NOW(), NOW(), $1, $2, $3, false, false)`, skJane, skGo, skLvlA20)
	res, err := repo.AddUserSkill(context.Background(), skJane, skGo, skLvlA50, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeDone {
		t.Fatalf("add over an inactive row = %+v, %v", res, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2`, skJane, skGo); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2 AND active AND skill_level_id = $3`, skJane, skGo, skLvlA50); n != 1 {
		t.Fatalf("the row was not revived at the new level")
	}
}

func TestSkillIntegration_InheritedRowsAreNotTouched(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()
	skExec(t, pool, `INSERT INTO user_skill (created_on, updated_on, user_id, skill_id, skill_level_id, active, inherited, inherited_from_group_id) VALUES (NOW(), NOW(), $1, $2, $3, true, true, gen_random_uuid())`, skJane, skGo, skLvlA20)

	if res, _ := repo.AddUserSkill(ctx, skJane, skGo, skLvlA50, skActor); res.Outcome != domain.SkillOutcomeInherited {
		t.Fatalf("add over inherited = %v", res.Outcome)
	}
	if res, _ := repo.SetUserSkillLevel(ctx, skJane, skGo, skLvlA50, skActor); res.Outcome != domain.SkillOutcomeInherited {
		t.Fatalf("change inherited = %v", res.Outcome)
	}
	if res, _ := repo.RemoveUserSkill(ctx, skJane, skGo, skActor); res.Outcome != domain.SkillOutcomeInherited {
		t.Fatalf("remove inherited = %v", res.Outcome)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2 AND inherited AND skill_level_id = $3`, skJane, skGo, skLvlA20); n != 1 {
		t.Fatal("the inherited row changed")
	}
	if n := len(skHistory(t, pool, skJane, skGo)); n != 0 {
		t.Fatalf("history rows = %d, want 0", n)
	}
	rows, _ := repo.ListUserSkills(ctx, skJane)
	if len(rows) != 1 || !rows[0].Inherited || rows[0].InheritedFromGroupID == nil {
		t.Fatalf("ListUserSkills = %+v", rows)
	}
}

func TestSkillIntegration_ProfileIsOnePersonsActiveSkills(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()
	repo.AddUserSkill(ctx, skJane, skGo, skLvlA50, skActor)
	repo.AddUserSkill(ctx, skLead, skRust, skLvlB20, skActor)
	rows, err := repo.ListUserSkills(ctx, skJane)
	if err != nil || len(rows) != 1 || rows[0].SkillID != skGo || rows[0].Level.Name != "Intermediate" ||
		len(rows[0].Categories) != 1 || rows[0].Categories[0].ID != skCatLeaf {
		t.Fatalf("ListUserSkills = %+v, %v", rows, err)
	}
	_ = pool
}

func TestSkillIntegration_ManagerSearchPushesFiltersIntoSQL(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()
	repo.AddUserSkill(ctx, skJane, skGo, skLvlA50, skActor)
	repo.AddUserSkill(ctx, skLead, skGo, skLvlA100, skActor)
	repo.AddUserSkill(ctx, skLead, skRust, skLvlB20, skActor)
	// Rows the manager never sees: an external person, a person who left,
	// and a deactivated skill.
	skExec(t, pool, `INSERT INTO user_skill (created_on, updated_on, user_id, skill_id, skill_level_id, active) VALUES (NOW(), NOW(), $1, $3, $4, true), (NOW(), NOW(), $2, $3, $4, true), (NOW(), NOW(), $5, $6, $7, true)`,
		skExt, skGone, skGo, skLvlA20, skJane, skOld, skLvlA20)

	search := func(req domain.SearchUserSkillsRequest) ([]domain.UserSkillRow, int) {
		t.Helper()
		req.UserIDs = append([]string{}, req.UserIDs...)
		if req.Pagination.Limit == 0 {
			req.Pagination.Limit = 50
		}
		rows, total, err := repo.SearchUserSkills(ctx, req)
		if err != nil {
			t.Fatalf("SearchUserSkills: %v", err)
		}
		return rows, total
	}
	// Everybody the fixtures allow: Jane/Go, Lead/Go, Lead/Rust, and nothing of the other three.
	if rows, total := search(domain.SearchUserSkillsRequest{UserIDs: []string{skJane, skLead, skExt, skGone}}); total != 3 || len(rows) != 3 {
		t.Fatalf("unfiltered = %d rows, total %d, want 3", len(rows), total)
	}
	// By skill.
	if rows, total := search(domain.SearchUserSkillsRequest{SkillIDs: []string{skGo}, UserIDs: []string{skJane, skLead, skExt, skGone}}); total != 2 || len(rows) != 2 {
		t.Fatalf("by skill = %d / %d", len(rows), total)
	}
	// By level, and by a minimum level value.
	if rows, _ := search(domain.SearchUserSkillsRequest{LevelIDs: []string{skLvlA100}, UserIDs: []string{skJane, skLead}}); len(rows) != 1 || rows[0].UserID != skLead {
		t.Fatalf("by level = %+v", rows)
	}
	if rows, _ := search(domain.SearchUserSkillsRequest{MinLevelValue: ptr(50), UserIDs: []string{skJane, skLead}}); len(rows) != 2 {
		t.Fatalf("min level 50 = %d rows, want 2 (50 and 100, not Rust's 20)", len(rows))
	}
	// By team: Lead is on the CRE fixture team, Jane is on it as an engineer too.
	rows, _ := search(domain.SearchUserSkillsRequest{TeamKeys: []string{skTeamCRE}, UserIDs: []string{skJane, skLead}})
	if len(rows) != 3 || !slices.Contains(rows[0].TeamKeys, skTeamCRE) {
		t.Fatalf("by team = %+v", rows)
	}
	if rows, _ := search(domain.SearchUserSkillsRequest{TeamKeys: []string{skTeamSRE}, UserIDs: []string{skJane, skLead}}); len(rows) != 0 {
		t.Fatalf("by another team = %d rows, want 0", len(rows))
	}
	// By name, with LIKE wildcards taken literally.
	if rows, _ := search(domain.SearchUserSkillsRequest{SearchQuery: "lead.skills", UserIDs: []string{skJane, skLead}}); len(rows) != 2 {
		t.Fatalf("by name = %d rows, want 2", len(rows))
	}
	if rows, _ := search(domain.SearchUserSkillsRequest{SearchQuery: "%", UserIDs: []string{skJane, skLead}}); len(rows) != 0 {
		t.Fatalf("a bare %% matched %d rows", len(rows))
	}
	// Paging: total is the whole, the page is a slice of it.
	rows, total := search(domain.SearchUserSkillsRequest{UserIDs: []string{skJane, skLead}, Pagination: domain.Pagination{Limit: 2, Offset: 2}})
	if total != 3 || len(rows) != 1 {
		t.Fatalf("second page = %d rows, total %d, want 1 of 3", len(rows), total)
	}
}

func TestSkillIntegration_ManagerIsACRETeamLeadOnly(t *testing.T) {
	repo, _ := newSkillIntegration(t)
	ctx := context.Background()
	for email, want := range map[string]bool{
		skLeadEmail:                    true,  // lead of a CRE team
		"sre.lead.skills@example.test": true,  // lead of an SRE team
		"odd.lead.skills@example.test": false, // lead of a team outside both families
		skJaneEmail:                    false, // an engineer
		"nobody@example.test":          false,
	} {
		got, err := repo.IsCRESRELead(ctx, email)
		if err != nil || got != want {
			t.Fatalf("IsCRESRELead(%s) = %v, %v; want %v", email, got, err, want)
		}
	}
	// The caller is an active internal user found by email, case-insensitively.
	if id, ok, err := repo.ResolveUserID(ctx, "JANE.DOE.SKILLS@EXAMPLE.TEST"); err != nil || !ok || id != skJane {
		t.Fatalf("ResolveUserID = %q, %v, %v", id, ok, err)
	}
	for _, email := range []string{"ext.skills@example.test", "gone.skills@example.test"} {
		if _, ok, _ := repo.ResolveUserID(ctx, email); ok {
			t.Fatalf("%s resolved, but is not an active internal user", email)
		}
	}
}

func TestSkillIntegration_CatalogueSearchAndReads(t *testing.T) {
	repo, _ := newSkillIntegration(t)
	ctx := context.Background()
	search := func(req domain.SearchSkillsRequest) ([]domain.Skill, int) {
		t.Helper()
		req.Pagination.Limit = 50
		s, total, err := repo.SearchSkills(ctx, req)
		if err != nil {
			t.Fatalf("SearchSkills: %v", err)
		}
		return s, total
	}
	// Filed under the leaf; the parent category finds it through its child.
	if s, _ := search(domain.SearchSkillsRequest{CategoryIDs: []string{skCatTop}}); len(s) != 1 || s[0].ID != skGo || len(s[0].Categories) != 1 {
		t.Fatalf("by parent category = %+v", s)
	}
	if s, _ := search(domain.SearchSkillsRequest{CategoryIDs: []string{skCatOther}}); len(s) != 0 {
		t.Fatalf("by an empty category = %+v", s)
	}
	// Inactive skills are left out unless asked for.
	byName, _ := search(domain.SearchSkillsRequest{SearchQuery: "sktest"})
	if len(byName) != 3 {
		t.Fatalf("active sktest skills = %d, want 3", len(byName))
	}
	withInactive, total := search(domain.SearchSkillsRequest{SearchQuery: "sktest", IncludeInactive: true})
	if len(withInactive) != 4 || total != 4 {
		t.Fatalf("with inactive = %d / %d, want 4", len(withInactive), total)
	}
	// Keywords match too.
	if s, _ := search(domain.SearchSkillsRequest{SearchQuery: "golang systems"}); len(s) < 3 {
		t.Fatalf("by keywords = %d", len(s))
	}
	if g, _ := repo.GetSkill(ctx, skGo); g == nil || g.Name != "sktest Go" {
		t.Fatalf("GetSkill = %+v", g)
	}
	if g, _ := repo.GetSkill(ctx, "5e1e0000-0000-4000-8000-0000000000ff"); g != nil {
		t.Fatalf("GetSkill of nothing = %+v", g)
	}
	types, err := repo.ListLevelTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, lt := range types {
		if lt.ID == skScaleA {
			found = true
			if len(lt.Levels) != 3 || lt.Levels[0].Name != "Beginner" || lt.Levels[2].Name != "Expert" {
				t.Fatalf("scale A levels = %+v, want value order", lt.Levels)
			}
		}
	}
	if !found {
		t.Fatal("fixture scale missing from ListLevelTypes")
	}
	cats, err := repo.ListCategories(ctx)
	if err != nil || len(cats) < 3 {
		t.Fatalf("ListCategories = %d, %v", len(cats), err)
	}
	if typ, ok, _ := repo.LevelTypeOfLevel(ctx, skLvlB20); !ok || typ != skScaleB {
		t.Fatalf("LevelTypeOfLevel = %q, %v", typ, ok)
	}
}

func TestSkillIntegration_CatalogueEdits(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()

	created, err := repo.CreateSkill(ctx, domain.CreateSkillRequest{
		Name: "sktest Created", Description: ptr("about"), LevelTypeID: skScaleA, CategoryIDs: []string{skCatLeaf, skCatOther},
	}, skActor)
	if err != nil || created.Outcome != domain.SkillOutcomeDone || created.Skill == nil {
		t.Fatalf("CreateSkill = %+v, %v", created, err)
	}
	id := created.Skill.ID
	t.Cleanup(func() { skRemoveFixtures(pool) })
	if n := skCount(t, pool, `SELECT count(*) FROM skill WHERE id = $1 AND name = 'sktest Created' AND active AND level_type_id = $2 AND created_by = $3 AND description = 'about'`, id, skScaleA, skActor); n != 1 {
		t.Fatal("the skill row is not as created")
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill_category_link WHERE skill_id = $1`, id); n != 2 {
		t.Fatalf("category links = %d, want 2", n)
	}
	// Names are unique ignoring case.
	if dup, _ := repo.CreateSkill(ctx, domain.CreateSkillRequest{Name: "SKTEST created", LevelTypeID: skScaleA}, skActor); dup.Outcome != domain.SkillOutcomeNameTaken {
		t.Fatalf("a duplicate name = %v", dup.Outcome)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill WHERE lower(name) = 'sktest created'`); n != 1 {
		t.Fatalf("skills named sktest created = %d", n)
	}

	// Edit: rename, clear the description, deactivate, relink to one category.
	upd, err := repo.UpdateSkill(ctx, id, domain.UpdateSkillRequest{
		Name: ptr("sktest Renamed"), Description: ptr(""), Active: ptr(false), CategoryIDs: &[]string{skCatOther},
	}, skActor)
	if err != nil || upd.Outcome != domain.SkillOutcomeDone || upd.Skill.Active || upd.Skill.Description != nil {
		t.Fatalf("UpdateSkill = %+v, %v", upd, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill WHERE id = $1 AND name = 'sktest Renamed' AND NOT active AND description IS NULL AND updated_by = $2`, id, skActor); n != 1 {
		t.Fatal("the skill row is not as updated")
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill_category_link WHERE skill_id = $1 AND category_id = $2`, id, skCatOther); n != 1 {
		t.Fatal("links were not replaced")
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill_category_link WHERE skill_id = $1`, id); n != 1 {
		t.Fatalf("links = %d, want 1", n)
	}
	// Renaming onto another skill's name is refused.
	if r, _ := repo.UpdateSkill(ctx, id, domain.UpdateSkillRequest{Name: ptr("sktest go")}, skActor); r.Outcome != domain.SkillOutcomeNameTaken {
		t.Fatalf("rename onto a taken name = %v", r.Outcome)
	}
	// An unknown skill.
	if r, _ := repo.UpdateSkill(ctx, "5e1e0000-0000-4000-8000-0000000000ff", domain.UpdateSkillRequest{Active: ptr(true)}, skActor); r.Outcome != domain.SkillOutcomeNotFound {
		t.Fatalf("unknown skill = %v", r.Outcome)
	}

	// The scale cannot change under people who hold a level of it, but can
	// while nobody does (and a no-op "change" to the same scale is fine).
	repo.AddUserSkill(ctx, skJane, skGo, skLvlA20, skActor)
	if r, _ := repo.UpdateSkill(ctx, skGo, domain.UpdateSkillRequest{LevelTypeID: ptr(skScaleB)}, skActor); r.Outcome != domain.SkillOutcomeLevelsInUse {
		t.Fatalf("scale change while held = %v", r.Outcome)
	}
	if r, _ := repo.UpdateSkill(ctx, skGo, domain.UpdateSkillRequest{LevelTypeID: ptr(skScaleA)}, skActor); r.Outcome != domain.SkillOutcomeDone {
		t.Fatalf("same scale = %v", r.Outcome)
	}
	if r, _ := repo.UpdateSkill(ctx, id, domain.UpdateSkillRequest{LevelTypeID: ptr(skScaleB)}, skActor); r.Outcome != domain.SkillOutcomeDone || *r.Skill.LevelTypeID != skScaleB {
		t.Fatalf("scale change while unheld = %+v", r)
	}
	// Deactivating keeps what people hold.
	if r, _ := repo.UpdateSkill(ctx, skGo, domain.UpdateSkillRequest{Active: ptr(false)}, skActor); r.Outcome != domain.SkillOutcomeDone {
		t.Fatalf("deactivate = %v", r.Outcome)
	}
	if rows, _ := repo.ListUserSkills(ctx, skJane); len(rows) != 1 || rows[0].SkillActive {
		t.Fatalf("profile after deactivation = %+v, want the skill kept and marked inactive", rows)
	}
}

type openAccess struct{}

func (openAccess) ResolveScope(context.Context) (service.AccessScope, error) {
	return service.AccessScope{Unrestricted: true}, nil
}

// The whole path -- token email to platform user, scale check, write -- over a
// real database.
func TestSkillIntegration_ServiceEndToEnd(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	svc := service.NewSkillService(repo, openAccess{}, []string{"cs_admin"})
	jane := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: skJaneEmail, UserID: "idp-subject"})

	// A level of the other scale is refused before anything is written.
	if _, err := svc.AddMySkill(jane, domain.AddMySkillRequest{SkillID: skGo, LevelID: skLvlB20}); err == nil {
		t.Fatal("a level outside the scale was accepted")
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1`, skJane); n != 0 {
		t.Fatalf("rows after a refused add = %d", n)
	}
	// An inactive skill is refused too.
	if _, err := svc.AddMySkill(jane, domain.AddMySkillRequest{SkillID: skOld, LevelID: skLvlA20}); err == nil {
		t.Fatal("an inactive skill was added")
	}
	added, err := svc.AddMySkill(jane, domain.AddMySkillRequest{SkillID: skGo, LevelID: skLvlA20})
	if err != nil || added.Level == nil || added.Level.ID != skLvlA20 {
		t.Fatalf("AddMySkill = %+v, %v", added, err)
	}
	if _, err := svc.AddMySkill(jane, domain.AddMySkillRequest{SkillID: skGo, LevelID: skLvlA50}); err == nil {
		t.Fatal("a duplicate add was accepted")
	}
	if _, err := svc.SetMySkillLevel(jane, skGo, domain.SetMySkillLevelRequest{LevelID: skLvlA50}); err != nil {
		t.Fatalf("SetMySkillLevel: %v", err)
	}
	mine, err := svc.GetMySkills(jane)
	if err != nil || mine.UserID != skJane || len(mine.Skills) != 1 || mine.Skills[0].Level.ID != skLvlA50 {
		t.Fatalf("GetMySkills = %+v, %v", mine, err)
	}
	if err := svc.RemoveMySkill(jane, skGo); err != nil {
		t.Fatalf("RemoveMySkill: %v", err)
	}
	hist, err := svc.GetMySkillHistory(jane, domain.SkillHistoryRequest{SkillID: skGo})
	if err != nil || hist.Total != 3 {
		t.Fatalf("GetMySkillHistory = %+v, %v", hist, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill_history WHERE user_id = $1 AND created_by = $2`, skJane, skJaneEmail); n != 3 {
		t.Fatalf("history rows by the caller = %d, want 3", n)
	}

	// The manager read: an ordinary engineer is refused, a team lead is not.
	if _, err := svc.SearchUserSkills(jane, domain.SearchUserSkillsRequest{}); err == nil {
		t.Fatal("an engineer read everybody's skills")
	}
	lead := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: skLeadEmail})
	if _, err := svc.SearchUserSkills(lead, domain.SearchUserSkillsRequest{}); err != nil {
		t.Fatalf("a team lead was refused: %v", err)
	}
	// A lead does not edit the catalogue; an admin does.
	if _, err := svc.CreateSkill(lead, domain.CreateSkillRequest{Name: "sktest Nope", LevelTypeID: skScaleA}); err == nil {
		t.Fatal("a team lead created a skill")
	}
	admin := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: skLeadEmail, Roles: []string{"cs_admin"}})
	if _, err := svc.CreateSkill(admin, domain.CreateSkillRequest{Name: "sktest Admin Made", LevelTypeID: skScaleA}); err != nil {
		t.Fatalf("admin create: %v", err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill WHERE name = 'sktest Admin Made'`); n != 1 {
		t.Fatalf("admin-made skill rows = %d", n)
	}
}
