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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// The query-hour port against a real Postgres, on the tables that
// already exist -- no migration accompanies this feature.
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" \
//	  go test -run TestQueryHourAgainstPostgres ./internal/service/
func TestQueryHourAgainstPostgres(t *testing.T) {
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

	seedQueryHourFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)

	svc := service.NewQueryHourService(
		repository.NewQueryHourRepository(repository.NewScoped(pool)), "@example.com")

	transitions, err := svc.Recompute(ctx)
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}

	// Two opportunities breach: one overdrawn, one at 92%. The third
	// is comfortably inside its allowance and must say nothing.
	if len(transitions) != 2 {
		t.Fatalf("transitions = %d, want 2:\n%s", len(transitions), mustJSON(t, transitions))
	}
	byProject := map[string]string{}
	for _, tr := range transitions {
		byProject[tr.ProjectKey] = string(tr.To)
	}
	if byProject["OVER"] != "CLOSURE_NOTICE_100" {
		t.Errorf("OVER = %q, want CLOSURE_NOTICE_100", byProject["OVER"])
	}
	if byProject["NEAR"] != "NOTIFIED_90" {
		t.Errorf("NEAR = %q, want NOTIFIED_90", byProject["NEAR"])
	}
	if _, listed := byProject["FINE"]; listed {
		t.Error("FINE is under 75% and must not notify")
	}

	// A second run changes nothing, so it must notify nobody. The
	// trigger is a state CHANGE; a recompute that re-mails every
	// breach on every tick would be unusable hourly.
	again, err := svc.Recompute(ctx)
	if err != nil {
		t.Fatalf("second Recompute: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("a repeat recompute produced %d notifications, want 0", len(again))
	}

	report, err := svc.WeeklyReport(ctx)
	if err != nil {
		t.Fatalf("WeeklyReport: %v", err)
	}
	if len(report.Exceeded) != 1 {
		t.Errorf("exceeded accounts = %d, want 1", len(report.Exceeded))
	}
	if len(report.CloseToExceed) != 1 {
		t.Errorf("close-to-exceed accounts = %d, want 1", len(report.CloseToExceed))
	}
	// Owners of accounts in BOTH tables, internal only.
	want := "am-over@example.com,am-near@example.com,to-over@example.com"
	got := map[string]bool{}
	for _, r := range report.Recipients {
		got[r] = true
	}
	for _, w := range []string{"am-over@example.com", "am-near@example.com", "to-over@example.com"} {
		if !got[w] {
			t.Errorf("recipients %v missing %s (want all of %s)", report.Recipients, w, want)
		}
	}
	if got["customer@elsewhere.com"] {
		t.Error("an external owner must never be a recipient")
	}

	// The figures land on the project, in the columns that already
	// hold ServiceNow's mirrored values.
	var total, consumed, remaining string
	if err := pool.QueryRow(ctx,
		`SELECT total_query_duration::text, consumed_duration::text, remaining_query_duration::text
		   FROM project WHERE key = 'OVER'`).Scan(&total, &consumed, &remaining); err != nil {
		t.Fatalf("read back project durations: %v", err)
	}
	if total == "" || consumed == "" || remaining == "" {
		t.Errorf("durations not written: total=%q consumed=%q remaining=%q", total, consumed, remaining)
	}
	t.Logf("OVER: total=%s consumed=%s remaining=%s", total, consumed, remaining)

	// Write the two payloads out so csm-scheduled-tasks can render and
	// send exactly what this produced, rather than a second fixture
	// that drifts from it.
	if out := os.Getenv("QUERY_HOUR_REPORT_JSON_OUT"); out != "" {
		writeJSON(t, out, report)
	}
	if out := os.Getenv("QUERY_HOUR_TRANSITIONS_JSON_OUT"); out != "" {
		writeJSON(t, out, map[string]any{"transitions": transitions})
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	blob, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// qhCase is one seeded account: an allowance and what has been drawn.
type qhCase struct {
	key          string
	accountName  string
	amEmail      string
	toEmail      string
	productName  string
	quantity     float64
	billableMins int
	// onboarding, when set, gives the project an onboarding grant and
	// records how much of it has been drawn.
	onboardingStatus   string
	onboardingTotal    int
	onboardingConsumed int
	// onboardingEnded makes the onboarding line end yesterday (and start 60
	// days ago) instead of running through the next 30 days.
	onboardingEnded bool
	// cardDaysAgo dates the billable card that many days back (0 = today).
	cardDaysAgo int
	// sharedBillableMins, when non-zero, adds a second project linked to the
	// same opportunity with that much approved billable time.
	sharedBillableMins int
	// lineEnded makes the query line end yesterday (and start 90 days ago)
	// instead of running through the next 30 days.
	lineEnded bool
}

// Three accounts, one per outcome. OVER is overdrawn, NEAR sits at 92%
// of its allowance, FINE has barely touched it.
var qhFixture = []qhCase{
	{key: "OVER", accountName: "Overdrawn Ltd", amEmail: "am-over@example.com",
		toEmail: "to-over@example.com", productName: "Development Support - 100 hours",
		quantity: 1, billableMins: 7000},
	{key: "NEAR", accountName: "Nearly Ltd", amEmail: "am-near@example.com",
		toEmail: "customer@elsewhere.com", productName: "Development Support - 100 hours",
		quantity: 1, billableMins: 5520},
	{key: "FINE", accountName: "Comfortable Ltd", amEmail: "am-fine@example.com",
		productName: "Development Support - 200 hours", quantity: 1, billableMins: 600},
}

func seedQueryHourFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}

	// Everything this fixture owns, not just the rows the current case
	// list would create. Cleaning up by the live list leaves behind
	// whatever a previous run with a different list created, and the
	// deterministic ids then collide on the next insert.
	//
	// Never a TRUNCATE: on a fully migrated database that cascades
	// into reference data the migrations seed once.
	exec(`DELETE FROM time_card WHERE created_by = 'qh-seed'`)
	exec(`DELETE FROM sf_opportunity_product WHERE created_by = 'qh-seed'`)
	exec(`DELETE FROM sf_opportunity_link WHERE created_by = 'qh-seed'`)
	exec(`DELETE FROM sf_opportunity WHERE created_by = 'qh-seed'`)
	exec(`DELETE FROM work_item WHERE created_by = 'qh-seed'`)
	exec(`DELETE FROM project WHERE created_by = 'qh-seed'`)
	exec(`DELETE FROM account WHERE created_by = 'qh-seed'`)
	// The people are identified by the id range the fixture allocates
	// from, since they carry no created_by of their own.
	var ids []string
	for i := 0; i < 50; i++ {
		ids = append(ids, uuidFor(4000+i), uuidFor(5000+i))
	}
	exec(`DELETE FROM team_member WHERE user_id = ANY($1::uuid[])`, ids)
	exec(`DELETE FROM "user" WHERE id = ANY($1::uuid[])`, ids)

	for i, c := range qhFixture {
		accID := uuidFor(1000 + i)
		projID := uuidFor(2000 + i)
		oppID := uuidFor(3000 + i)
		amID := uuidFor(4000 + i)
		toID := uuidFor(5000 + i)
		itemID := uuidFor(6000 + i)

		exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, is_active)
		      VALUES ($1, NOW(), NOW(), $2, $2, $2, TRUE)`, amID, c.amEmail)
		if c.toEmail != "" {
			exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, is_active)
			      VALUES ($1, NOW(), NOW(), $2, $2, $2, TRUE)`, toID, c.toEmail)
		}
		exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by,
		        name, number, sf_id, account_manager_id, technical_owner_id)
		      VALUES ($1, NOW(), NOW(), 'qh-seed', 'qh-seed', $2, $3, $4, $5, $6)`,
			accID, c.accountName, "ACC-"+c.key, "001"+c.key, amID, nullIfEmpty(c.toEmail, toID))
		status := c.onboardingStatus
		if status == "" {
			status = "NOT_APPLICABLE"
		}
		exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by,
		        name, key, sf_id, account_id, onboarding_status,
		        total_onboarding_duration, consumed_onboarding_duration)
		      VALUES ($1, NOW(), NOW(), 'qh-seed', 'qh-seed', $2, $3, $4, $5,
		              $6::onboarding_status_enum,
		              make_interval(mins => $7), make_interval(mins => $8))`,
			projID, c.accountName+" Project", c.key, "a0"+c.key, accID,
			status, c.onboardingTotal, c.onboardingConsumed)
		exec(`INSERT INTO sf_opportunity (id, created_on, updated_on, created_by, updated_by,
		        name, sf_id, account_id)
		      VALUES ($1, NOW(), NOW(), 'qh-seed', 'qh-seed', $2, $3, $4)`,
			oppID, c.key, "006"+c.key, accID)
		exec(`INSERT INTO sf_opportunity_link (id, created_on, updated_on, created_by, updated_by,
		        opportunity_id, project_id)
		      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed', $1, $2)`, oppID, projID)
		lineStart, lineEnd := 30, 30
		if c.lineEnded {
			lineStart, lineEnd = 90, -1
		}
		exec(`INSERT INTO sf_opportunity_product
		        (id, created_on, updated_on, created_by, updated_by,
		         opportunity_id, product_name, quantity,
		         service_start_date, service_end_date)
		      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed', $1, $2, $3,
		              CURRENT_DATE - $4::int, CURRENT_DATE + $5::int)`,
			oppID, c.productName, c.quantity, lineStart, lineEnd)
		if status != "NOT_APPLICABLE" {
			// The onboarding pool is derived from this line, not read from
			// the stored consumed figure.
			startOff, endOff := -30, 30
			if c.onboardingEnded {
				startOff, endOff = -60, -1
			}
			exec(`INSERT INTO sf_opportunity_product
			        (id, created_on, updated_on, created_by, updated_by,
			         opportunity_id, product_name, quantity,
			         service_start_date, service_end_date)
			      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed', $1,
			              'WSO2 Onboarding Services', 1, CURRENT_DATE + $2::int, CURRENT_DATE + $3::int)`, oppID, startOff, endOff)
		}

		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by,
		        number, wso2_id, subject, type)
		      VALUES ($1, NOW(), NOW(), 'qh-seed', 'qh-seed', $2, $2, $2, 'CASE')`,
			itemID, c.key+"-1")
		exec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
		        case_id, customer_project_id, user_id, work_date, state, is_billable, analyzing_minutes)
		      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed',
		              $1, $2, $3, CURRENT_DATE - $5::int, 'APPROVED', TRUE, $4)`,
			itemID, projID, amID, c.billableMins, c.cardDaysAgo)
		if c.sharedBillableMins > 0 {
			// A second project on the same opportunity, with its own time.
			proj2 := uuidFor(7000 + i)
			exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by,
			        name, key, sf_id, account_id, onboarding_status)
			      VALUES ($1, NOW(), NOW(), 'qh-seed', 'qh-seed', $2, $3, $4, $5,
			              $6::onboarding_status_enum)`,
				proj2, c.accountName+" Project 2", c.key+"2", "a1"+c.key, accID, status)
			exec(`INSERT INTO sf_opportunity_link (id, created_on, updated_on, created_by, updated_by,
			        opportunity_id, project_id)
			      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed', $1, $2)`, oppID, proj2)
			exec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
			        case_id, customer_project_id, user_id, work_date, state, is_billable, analyzing_minutes)
			      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed',
			              $1, $2, $3, CURRENT_DATE - $5::int, 'APPROVED', TRUE, $4)`,
				itemID, proj2, amID, c.sharedBillableMins, c.cardDaysAgo)
		}
		// Non-billable time that must draw nothing down.
		exec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
		        case_id, customer_project_id, user_id, work_date, state, is_billable, analyzing_minutes)
		      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed',
		              $1, $2, $3, CURRENT_DATE, 'APPROVED', FALSE, 9000)`,
			itemID, projID, amID)
	}
}

func uuidFor(n int) string {
	return "00000000-0000-0000-0000-" + pad12(n)
}

func pad12(n int) string {
	s := ""
	for i := 0; i < 12; i++ {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func nullIfEmpty(check, value string) any {
	if check == "" {
		return nil
	}
	return value
}

// *** THE ONBOARDING POOL IS DERIVED, AND FILLS FIRST. ***
//
// A project with an onboarding line is granted 200 onboarding hours, and
// billable work is charged against them before it reaches the query-support
// allowance. A project without one has no grant: its hours come straight off
// query support. What is consumed is always recomputed from approved
// billable time cards, never read back from a stored figure, so a stale
// stored value cannot move the result.
func TestQueryHourOnboardingPoolFillsFirst(t *testing.T) {
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

	// Each project logs the same billable minutes against the same
	// 100-hour allowance, and differs only in its onboarding state. The
	// stored onboarding figures are deliberately wrong: they must be ignored.
	saved := qhFixture
	qhFixture = []qhCase{
		{key: "INPROG", accountName: "In Progress Ltd", amEmail: "am-inprog@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			onboardingStatus: "IN_PROGRESS", onboardingTotal: 5, onboardingConsumed: 5},
		{key: "SPILL", accountName: "Spill Ltd", amEmail: "am-spill@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 13000,
			onboardingStatus: "IN_PROGRESS"},
		{key: "RENEW", accountName: "Renewal Ltd", amEmail: "am-renew@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			onboardingStatus: "NOT_APPLICABLE"},
		// A renewal carrying a stale onboarding figure must not have it
		// credited, and the figure is left as it was.
		// Two projects share one opportunity: the allowance is shared, so both
		// show the same total and the same (opportunity-wide) consumed.
		{key: "SHARE", accountName: "Shared Ltd", amEmail: "am-share@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			sharedBillableMins: 2000, onboardingStatus: "NOT_APPLICABLE"},
		// The onboarding line has ended: what it absorbed stays absorbed and
		// is still taken off query hours, run after run.
		{key: "EXPIRED", accountName: "Expired Ltd", amEmail: "am-expired@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			onboardingStatus: "IN_PROGRESS", onboardingEnded: true, cardDaysAgo: 30},
		{key: "STALE", accountName: "Stale Ltd", amEmail: "am-stale@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			onboardingStatus: "NOT_APPLICABLE", onboardingTotal: 200 * 60, onboardingConsumed: 1200},
	}
	defer func() { qhFixture = saved }()

	seedQueryHourFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)
	svc := service.NewQueryHourService(
		repository.NewQueryHourRepository(repository.NewScoped(pool)), "@example.com")
	if _, err := svc.Recompute(ctx); err != nil {
		t.Fatalf("Recompute: %v", err)
	}

	mins := func(key, col string) int {
		var m int
		if err := pool.QueryRow(ctx,
			`SELECT COALESCE(FLOOR(EXTRACT(EPOCH FROM `+col+`)/60),0)::int
			   FROM project WHERE key = $1`, key).Scan(&m); err != nil {
			t.Fatalf("read %s.%s: %v", key, col, err)
		}
		return m
	}
	check := func(key string, onbTotal, onbConsumed, onbRemaining, qConsumed, qRemaining int) {
		t.Helper()
		for col, want := range map[string]int{
			"total_onboarding_duration":     onbTotal,
			"consumed_onboarding_duration":  onbConsumed,
			"remaining_onboarding_duration": onbRemaining,
			"consumed_duration":             qConsumed,
			"remaining_query_duration":      qRemaining,
			"total_query_duration":          6000,
		} {
			if got := mins(key, col); got != want {
				t.Errorf("%s %s = %d, want %d", key, col, got, want)
			}
		}
	}
	// 3000 billable, all inside the 12000-minute onboarding pool.
	check("INPROG", 12000, 3000, 9000, 0, 6000)
	// 13000 billable: the pool takes 12000, 1000 overflows to query.
	check("SPILL", 12000, 12000, 0, 1000, 5000)
	// No onboarding: the whole 3000 hits query hours.
	check("RENEW", 0, 0, 0, 3000, 3000)
	// Not applicable: onboarding columns untouched, none credited.
	check("STALE", 12000, 1200, 0, 3000, 3000)
	// Shared opportunity: 3000 + 2000 billable across both projects.
	check("SHARE", 0, 0, 0, 5000, 1000)
	check("SHARE2", 0, 0, 0, 5000, 1000)
	// Ended onboarding line: the 3000 stays in the onboarding pool.
	check("EXPIRED", 12000, 3000, 9000, 0, 6000)

	// Running it again, or through the per-project entry point the time
	// card approval uses, changes nothing: it is derived, not accumulated.
	var projID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM project WHERE key = 'SPILL'`).Scan(&projID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := svc.RecomputeProjectHours(ctx, projID); err != nil {
			t.Fatalf("RecomputeProjectHours: %v", err)
		}
	}
	check("SPILL", 12000, 12000, 0, 1000, 5000)

	// The per-project function is durations only: an approval must not
	// move the opportunity state the hourly job compares against.
	var before, after *string
	if err := pool.QueryRow(ctx, `SELECT query_hour_state::text FROM sf_opportunity WHERE name = 'SPILL'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE time_card SET analyzing_minutes = analyzing_minutes + 9000
	                              WHERE customer_project_id = $1 AND is_billable`, projID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecomputeProjectHours(ctx, projID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT query_hour_state::text FROM sf_opportunity WHERE name = 'SPILL'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if (before == nil) != (after == nil) || (before != nil && *before != *after) {
		t.Errorf("opportunity state moved from %v to %v; only Recompute may write it", before, after)
	}
	if got := mins("SPILL", "consumed_duration"); got != 10000 {
		t.Errorf("SPILL consumed after more approved time = %d, want 10000", got)
	}
}

// A project whose only linked line has ended is written down to zero (the
// previous platform did the same), a project nothing is linked to keeps what
// it holds, and the siblings lookup finds projects sharing an opportunity.
func TestQueryHourLapsedAllowanceAndSiblingsAgainstPostgres(t *testing.T) {
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

	saved := qhFixture
	qhFixture = []qhCase{
		{key: "LAPSED", accountName: "Lapsed Ltd", amEmail: "am-lapsed@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 600,
			onboardingStatus: "NOT_APPLICABLE", lineEnded: true},
		{key: "SHARE", accountName: "Shared Ltd", amEmail: "am-share@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			sharedBillableMins: 2000, onboardingStatus: "NOT_APPLICABLE"},
	}
	defer func() { qhFixture = saved }()
	seedQueryHourFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%q: %v", sql, err)
		}
	}
	// Stale stored figures on the lapsed project, and a project with no links
	// at all that carries figures this calculation cannot vouch for.
	exec(`UPDATE project SET total_query_duration = make_interval(mins => 6000),
	        consumed_duration = make_interval(mins => 100), remaining_query_duration = make_interval(mins => 5900)
	      WHERE key = 'LAPSED'`)
	exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, name, key, sf_id, account_id,
	        total_query_duration)
	      SELECT $1, NOW(), NOW(), 'qh-seed', 'qh-seed', 'Unlinked', 'UNLINKED', 'a0UNLINKED', account_id,
	             make_interval(mins => 7777)
	        FROM project WHERE key = 'LAPSED'`, uuidFor(2900))

	repo := repository.NewQueryHourRepository(repository.NewScoped(pool))
	ids, err := repo.ProjectIDsWithLinkedLines(ctx)
	if err != nil {
		t.Fatal(err)
	}
	has := func(id string) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}
	lapsedID, shareID, share2ID := uuidFor(2000), uuidFor(2001), uuidFor(7001)
	if !has(lapsedID) || !has(shareID) || has(uuidFor(2900)) {
		t.Errorf("projects with linked lines = %v: want lapsed and shared, not the unlinked one", ids)
	}

	svc := service.NewQueryHourService(repo, "@example.com")
	if _, err := svc.Recompute(ctx); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	mins := func(key, col string) int {
		var m int
		if err := pool.QueryRow(ctx, `SELECT COALESCE(FLOOR(EXTRACT(EPOCH FROM `+col+`)/60),0)::int FROM project WHERE key = $1`, key).Scan(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, col := range []string{"total_query_duration", "consumed_duration", "remaining_query_duration"} {
		if got := mins("LAPSED", col); got != 0 {
			t.Errorf("LAPSED %s = %d, want 0", col, got)
		}
	}
	if got := mins("UNLINKED", "total_query_duration"); got != 7777 {
		t.Errorf("UNLINKED total = %d, want 7777 left untouched", got)
	}

	sib, err := repo.SiblingProjectIDs(ctx, shareID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sib) != 1 || sib[0] != share2ID {
		t.Errorf("siblings of the shared project = %v, want [%s]", sib, share2ID)
	}
	none, err := repo.SiblingProjectIDs(ctx, lapsedID)
	if err != nil || len(none) != 0 {
		t.Errorf("siblings of a lone project = %v, %v", none, err)
	}
}
