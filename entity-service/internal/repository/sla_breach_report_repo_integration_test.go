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

// This is an integration test: it exercises the real SQL in
// sla_breach_report_repo.go against a live PostgreSQL instance, because
// what it guards -- that a clock the CSM SLA engine breached is reported
// at all -- lives entirely in the interaction between the sla_live view,
// business_time_add (migration 0306) and the stored columns. No fake
// repository can reproduce it.
//
// The bug it regression-tests was live in production: the report keyed on
// original_breached_on, a column nothing in this platform writes, so a
// week in which the portal showed ~130 breached cases reported zero rows.
//
// Same DSN and skip-when-unset pattern as the other integration tests
// here (package repository_test, reuses caseStatsPool):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run SLABreachReportIntegration

package repository_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The reporting week these scenarios are built around: Monday
// 2026-11-16 00:00 to Monday 2026-11-23 00:00, Asia/Colombo, as
// SLABreachReportService resolves it.
var (
	slaBreachWeekFrom = time.Date(2026, 11, 15, 18, 30, 0, 0, time.UTC)
	slaBreachWeekTo   = time.Date(2026, 11, 22, 18, 30, 0, 0, time.UTC)
)

const (
	slaBreachAccountID  = "7a000000-0000-0000-0000-0000000000a1"
	slaBreachGroupID    = "7a000000-0000-0000-0000-0000000000a2"
	slaBreachProjectID  = "7a000000-0000-0000-0000-0000000000a3"
	slaBreachFlatPolicy = "7a000000-0000-0000-0000-0000000000b1"
	slaBreachBizPolicy  = "7a000000-0000-0000-0000-0000000000b2"
	// The product reached only through a deployed product, the way a real
	// case carries one.
	slaBreachProductID         = "7a000000-0000-0000-0000-0000000000d1"
	slaBreachDeployedProductID = "7a000000-0000-0000-0000-0000000000d2"
)

// seedSLABreachReport builds the six clocks the scenarios below assert on,
// and removes everything it created before and after the test so it stays
// re-runnable against a shared database.
//
// Every row belongs to one account whose CRE group and project are the
// test's own, so a report filtered to this fixture's policies can never
// pick up unrelated data from the database it runs against.
func seedSLABreachReport(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		stmts := []struct {
			sql  string
			args []any
		}{
			{`DELETE FROM sla WHERE sla_policy_id IN ($1::uuid, $2::uuid)`, []any{slaBreachFlatPolicy, slaBreachBizPolicy}},
			{`DELETE FROM "case" WHERE id IN (SELECT id FROM work_item WHERE account_id = $1::uuid)`, []any{slaBreachAccountID}},
			{`DELETE FROM work_item WHERE account_id = $1::uuid`, []any{slaBreachAccountID}},
			{`DELETE FROM deployed_product WHERE id = $1::uuid`, []any{slaBreachDeployedProductID}},
			{`DELETE FROM product WHERE id = $1::uuid`, []any{slaBreachProductID}},
			{`DELETE FROM sla_policy WHERE id IN ($1::uuid, $2::uuid)`, []any{slaBreachFlatPolicy, slaBreachBizPolicy}},
			{`DELETE FROM project WHERE id = $1::uuid`, []any{slaBreachProjectID}},
			{`DELETE FROM account WHERE id = $1::uuid`, []any{slaBreachAccountID}},
			{`DELETE FROM "group" WHERE id = $1::uuid`, []any{slaBreachGroupID}},
		}
		for _, st := range stmts {
			if _, err := scoped.Exec(ctx, st.sql, st.args...); err != nil {
				t.Logf("cleanup %q: %v", st.sql, err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}

	mustExec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name)
		VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', 'SLA Breach Test Team')`, slaBreachGroupID)
	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, cre_team_id)
		VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', 'SLA Breach Test Account', 'SLABRT-1', $2::uuid)`,
		slaBreachAccountID, slaBreachGroupID)
	// COMPLETED onboarding status: the CRE variant excludes IN_PROGRESS
	// and NOT_STARTED, so this project passes that filter.
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, onboarding_status)
		VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', 'SLABRT', 'COMPLETED')`, slaBreachProjectID)

	// One 24x7 policy and one business-day policy: two of the ten the
	// report covers are business-day ones, and they take different paths
	// through the derived breach instant.
	mustExec(`INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, is_active, target, duration, schedule)
		VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', 'SLABRT - Resolution (Test)', TRUE, 'RESOLUTION', INTERVAL '48 hours', '24 x 7'),
		       ($2::uuid, NOW(), NOW(), 'test', 'test', 'SLABRT - Response (Test)', TRUE, 'RESPONSE', INTERVAL '24 hours', 'WSO2 Business days')`,
		slaBreachFlatPolicy, slaBreachBizPolicy)

	for i := 1; i <= 8; i++ {
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id, wso2_id)
			VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', $2, 'sla breach fixture', 'CASE', $3::uuid, $4::uuid, $5)`,
			slaBreachWorkItemID(i), "SLABRT-NUM-"+strconv.Itoa(i), slaBreachAccountID, slaBreachProjectID, "SLABRT-"+strconv.Itoa(i))
		mustExec(`INSERT INTO "case" (id, issue_type, severity) VALUES ($1::uuid, 'ERROR', 'S1')`, slaBreachWorkItemID(i))
	}

	insertClock := func(n int, policy, stage string, startOn time.Time, hasBreached bool, recorded any) {
		mustExec(`INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id,
			schedule, is_active, stage, start_on, duration, business_elapsed_percentage,
			business_duration, remaining_business_duration, has_breached, original_breached_on)
			SELECT gen_random_uuid(), NOW(), NOW(), 'test', 'test', $1::uuid, $2::uuid,
			       sp.schedule, $3, $4::sla_stage_enum, $5::timestamptz, sp.duration, 0,
			       INTERVAL '0', sp.duration, $6, $7::timestamptz
			FROM sla_policy sp WHERE sp.id = $2::uuid`,
			slaBreachWorkItemID(n), policy, stage == "IN_PROGRESS", stage, startOn, hasBreached, recorded)
	}

	// 1: the previous system's own synced row -- a recorded instant,
	//    which must keep winning over any derivation. The instant is
	//    deliberately NOT start_on + the policy's 48 hours (which would
	//    be the 18th): an instant that matched the derivation would make
	//    RecordedInstantWins pass even with the precedence removed.
	insertClock(1, slaBreachFlatPolicy, "COMPLETED", time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC), true,
		time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC))
	// 2: engine clock, still running, 24x7, due inside the week. The
	//    case this whole change exists for: no recorded instant, stored
	//    has_breached still false.
	insertClock(2, slaBreachFlatPolicy, "IN_PROGRESS", time.Date(2026, 11, 16, 8, 0, 0, 0, time.UTC), false, nil)
	// 3: engine clock, still running, business-day policy. Started
	//    Friday 07:00 New York; 24 business hours later is Monday 07:00,
	//    which is inside the week -- flat arithmetic would have put it on
	//    the Saturday and missed the week entirely.
	insertClock(3, slaBreachBizPolicy, "IN_PROGRESS", time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC), false, nil)
	// 4: engine clock that STOPPED ON TIME. Its start + duration lands
	//    inside the week, so a derivation that ignored has_breached would
	//    report it as a breach. It must not appear.
	insertClock(4, slaBreachFlatPolicy, "ACHIEVED", time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC), false, nil)
	// 5: engine clock that stopped LATE -- has_breached written by the
	//    engine, instant still never recorded.
	insertClock(5, slaBreachFlatPolicy, "ACHIEVED", time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC), true, nil)
	// 6: cancelled clock. Breached, inside the week, but CANCELLED was
	//    not in the source report's stage filter and must stay out.
	insertClock(6, slaBreachFlatPolicy, "CANCELLED", time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC), true, nil)

	// 7 and 8 are engine clocks that were PAUSED, which sla_live never
	// recomputes: live_has_breached returns the stored flag, and pausing
	// never writes it. The frozen business_elapsed_percentage is the only
	// thing that separates them.
	insertPausedClock := func(n int, startOn time.Time, frozenPercentage float64) {
		mustExec(`INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id,
			schedule, is_active, stage, start_on, paused_on, duration, business_elapsed_percentage,
			business_duration, remaining_business_duration, has_breached, original_breached_on)
			SELECT gen_random_uuid(), NOW(), NOW(), 'test', 'test', $1::uuid, $2::uuid,
			       sp.schedule, FALSE, 'PAUSED'::sla_stage_enum, $3::timestamptz, NOW(), sp.duration, $4,
			       INTERVAL '0', sp.duration, FALSE, NULL
			FROM sla_policy sp WHERE sp.id = $2::uuid`,
			slaBreachWorkItemID(n), slaBreachFlatPolicy, startOn, frozenPercentage)
	}

	// 7: paused AFTER it had already run past its target -- 120% frozen
	//    at paused_on, has_breached still false. It must be reported:
	//    PAUSED is in the CRE variant's own stage list.
	//
	//    It also carries its product the way a real case does: through a
	//    deployed product, with work_item.product_id left NULL. Keying the
	//    report on product_id alone left this column empty.
	mustExec(`INSERT INTO product (id, created_on, updated_on, created_by, updated_by, name, category)
		VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', 'SLABRT Test Product', 'SOFTWARE')`, slaBreachProductID)
	mustExec(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, product_id)
		VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', 'SLABRT-DP-1', $2::uuid)`,
		slaBreachDeployedProductID, slaBreachProductID)
	mustExec(`UPDATE work_item SET deployed_product_id = $2::uuid WHERE id = $1::uuid`,
		slaBreachWorkItemID(7), slaBreachDeployedProductID)
	insertPausedClock(7, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC), 120)
	// 8: paused while still ON TIME -- 50% frozen. Its start + duration
	//    lands inside the week, so a predicate that admitted every PAUSED
	//    row rather than testing the frozen value would report it.
	insertPausedClock(8, time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC), 50)
}

func slaBreachWorkItemID(n int) string {
	return "7a000000-0000-0000-0000-00000000c00" + strconv.Itoa(n)
}

// creStages is the CRE variant's stage filter, as
// slabreachscan.CREVariant builds it.
var creStages = []string{"PAUSED", "COMPLETED", "ACHIEVED", "BREACHED"}

// TestSLABreachReportIntegrationReportsEngineWrittenBreaches is the
// regression test for the production bug: clocks the CSM SLA engine
// breached have no original_breached_on, and the report used to key on
// that column alone, so it reported nothing at all.
func TestSLABreachReportIntegrationReportsEngineWrittenBreaches(t *testing.T) {
	pool := caseStatsPool(t)
	seedSLABreachReport(t, pool)

	repo := repository.NewSLABreachReportRepository(repository.NewScoped(pool))
	rows, err := repo.ListBreaches(context.Background(), slaBreachWeekFrom, slaBreachWeekTo,
		[]string{"SLABRT - Resolution (Test)", "SLABRT - Response (Test)"}, creStages, nil,
		[]string{"IN_PROGRESS", "NOT_STARTED"})
	if err != nil {
		t.Fatalf("ListBreaches: %v", err)
	}

	got := map[string]bool{}
	for _, r := range rows {
		got[r.WSO2CaseID] = true
	}
	for _, want := range []string{"SLABRT-1", "SLABRT-2", "SLABRT-3", "SLABRT-5", "SLABRT-7"} {
		if !got[want] {
			t.Errorf("%s is missing from the report: %#v", want, got)
		}
	}
	// A clock that stopped on time, a cancelled one, and one paused while
	// still on time are not breaches of this week however their instant
	// derives.
	for _, unwanted := range []string{"SLABRT-4", "SLABRT-6", "SLABRT-8"} {
		if got[unwanted] {
			t.Errorf("%s must not be reported: %#v", unwanted, got)
		}
	}
}

// TestSLABreachReportIntegrationRecordedInstantWins: a row the previous
// system synced carries its own breach instant, and no derivation may
// move it -- otherwise this change would silently rewrite the history
// every earlier run of this report already mailed.
func TestSLABreachReportIntegrationRecordedInstantWins(t *testing.T) {
	pool := caseStatsPool(t)
	seedSLABreachReport(t, pool)

	repo := repository.NewSLABreachReportRepository(repository.NewScoped(pool))
	rows, err := repo.ListBreaches(context.Background(), slaBreachWeekFrom, slaBreachWeekTo,
		[]string{"SLABRT - Resolution (Test)"}, creStages, nil, nil)
	if err != nil {
		t.Fatalf("ListBreaches: %v", err)
	}
	for _, r := range rows {
		if r.WSO2CaseID != "SLABRT-1" {
			continue
		}
		// The derivation would put this clock on the 18th; the recorded
		// instant is the 19th, so the two cannot be confused.
		want := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
		if !r.BreachedOn.Equal(want) {
			t.Fatalf("recorded instant = %s, want %s", r.BreachedOn, want)
		}
		return
	}
	t.Fatal("the synced row is missing from the report")
}

// TestSLABreachReportIntegrationPausedClockThatHadAlreadyBreached: the
// CRE variant lists PAUSED among its stages, but sla_live only recomputes
// live_has_breached for raw stage IN_PROGRESS and pausing never writes
// the stored flag, so an engine clock that ran past its target and was
// then paused reads has_breached = FALSE forever. Its frozen
// business_elapsed_percentage is what says it breached; a clock paused
// while still on time keeps that value under 100 and stays out.
func TestSLABreachReportIntegrationPausedClockThatHadAlreadyBreached(t *testing.T) {
	pool := caseStatsPool(t)
	seedSLABreachReport(t, pool)

	repo := repository.NewSLABreachReportRepository(repository.NewScoped(pool))
	rows, err := repo.ListBreaches(context.Background(), slaBreachWeekFrom, slaBreachWeekTo,
		[]string{"SLABRT - Resolution (Test)"}, []string{"PAUSED"}, nil, nil)
	if err != nil {
		t.Fatalf("ListBreaches: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want only the clock that breached before it paused: %#v", len(rows), rows)
	}
	if rows[0].WSO2CaseID != "SLABRT-7" {
		t.Fatalf("reported %s, want SLABRT-7", rows[0].WSO2CaseID)
	}
	// Derived from start_on + the policy's 48 hours: the clock has no
	// recorded instant, and the open pause is not part of the budget.
	want := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	if !rows[0].BreachedOn.Equal(want) {
		t.Fatalf("breached on %s, want %s", rows[0].BreachedOn, want)
	}
	if rows[0].BusinessElapsedPercent < 100 {
		t.Fatalf("percentage = %v, want the frozen value the pause left behind", rows[0].BusinessElapsedPercent)
	}
	// This case carries its product only through a deployed product, with
	// work_item.product_id NULL -- the shape every row of the first
	// production run had, which reported an empty product column.
	if rows[0].Product != "SLABRT Test Product" {
		t.Fatalf("product = %q, want the name reached through the deployed product", rows[0].Product)
	}
}

// TestSLABreachReportIntegrationBusinessDayInstantSkipsTheWeekend pins the
// business-day derivation specifically: clock 3 starts Friday and is due
// 24 business hours later, which is Monday -- inside the reported week.
// Flat wall-clock arithmetic would put it on the Saturday, outside it, and
// the row would silently vanish from a week it belongs to.
func TestSLABreachReportIntegrationBusinessDayInstantSkipsTheWeekend(t *testing.T) {
	pool := caseStatsPool(t)
	seedSLABreachReport(t, pool)

	repo := repository.NewSLABreachReportRepository(repository.NewScoped(pool))
	rows, err := repo.ListBreaches(context.Background(), slaBreachWeekFrom, slaBreachWeekTo,
		[]string{"SLABRT - Response (Test)"}, creStages, nil, nil)
	if err != nil {
		t.Fatalf("ListBreaches: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want the one business-day clock: %#v", len(rows), rows)
	}
	want := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	if !rows[0].BreachedOn.Equal(want) {
		t.Fatalf("derived instant = %s, want %s (Monday, not the Saturday flat time gives)", rows[0].BreachedOn, want)
	}
}

// TestSLABreachReportIntegrationRunningClockReportsItsLivePercentage: a
// running clock's stored business_elapsed_percentage is whatever it was
// at registration (0), so a report that read it showed 0% for exactly the
// rows whose overrun it exists to report.
func TestSLABreachReportIntegrationRunningClockReportsItsLivePercentage(t *testing.T) {
	pool := caseStatsPool(t)
	seedSLABreachReport(t, pool)

	repo := repository.NewSLABreachReportRepository(repository.NewScoped(pool))
	rows, err := repo.ListBreaches(context.Background(), slaBreachWeekFrom, slaBreachWeekTo,
		[]string{"SLABRT - Resolution (Test)"}, creStages, nil, nil)
	if err != nil {
		t.Fatalf("ListBreaches: %v", err)
	}
	for _, r := range rows {
		if r.WSO2CaseID != "SLABRT-2" {
			continue
		}
		if r.BusinessElapsedPercent <= 100 {
			t.Fatalf("live percentage = %v, want a figure past 100 for a clock still running well past its target", r.BusinessElapsedPercent)
		}
		return
	}
	t.Fatal("the running engine clock is missing from the report")
}
