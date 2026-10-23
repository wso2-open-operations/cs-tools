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
		exec(`INSERT INTO sf_opportunity_product
		        (id, created_on, updated_on, created_by, updated_by,
		         opportunity_id, product_name, quantity,
		         service_start_date, service_end_date)
		      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed', $1, $2, $3,
		              CURRENT_DATE - 30, CURRENT_DATE + 30)`,
			oppID, c.productName, c.quantity)

		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by,
		        number, wso2_id, subject, type)
		      VALUES ($1, NOW(), NOW(), 'qh-seed', 'qh-seed', $2, $2, $2, 'CASE')`,
			itemID, c.key+"-1")
		exec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
		        case_id, customer_project_id, user_id, work_date, state, is_billable, analyzing_minutes)
		      VALUES (gen_random_uuid(), NOW(), NOW(), 'qh-seed', 'qh-seed',
		              $1, $2, $3, CURRENT_DATE, 'APPROVED', TRUE, $4)`,
			itemID, projID, amID, c.billableMins)
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

// *** THE ONBOARDING GRANT IS ONE-TIME, AND ONLY NEW PROJECTS GET ONE. ***
//
// A new project is granted onboarding hours -- 200 on this deployment --
// and work is charged against those before it reaches the query-support
// allowance. A renewal has no grant: its hours come straight off query
// support.
//
// Once onboarding is COMPLETED the grant stops absorbing, so later work
// lands on query hours. What it already absorbed stays absorbed, which
// is what makes the grant one-time rather than a discount that is
// clawed back the moment onboarding closes.
func TestQueryHourOnboardingGrantAppliesToNewProjectsOnly(t *testing.T) {
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

	// Each project logs the same 3000 billable minutes against the same
	// 100-hour allowance, and differs only in its onboarding state.
	saved := qhFixture
	qhFixture = []qhCase{
		{key: "INPROG", accountName: "In Progress Ltd", amEmail: "am-inprog@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			onboardingStatus: "IN_PROGRESS", onboardingTotal: 200 * 60, onboardingConsumed: 1200},
		{key: "DONE", accountName: "Completed Ltd", amEmail: "am-done@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			onboardingStatus: "COMPLETED", onboardingTotal: 200 * 60, onboardingConsumed: 1200},
		{key: "RENEW", accountName: "Renewal Ltd", amEmail: "am-renew@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			onboardingStatus: "NOT_APPLICABLE"},
		// A renewal carrying a stale onboarding figure must still not
		// have it credited -- the status decides, not the number.
		{key: "STALE", accountName: "Stale Ltd", amEmail: "am-stale@example.com",
			productName: "Development Support - 100 hours", quantity: 1, billableMins: 3000,
			onboardingStatus: "NOT_APPLICABLE", onboardingTotal: 200 * 60, onboardingConsumed: 1200},
	}
	defer func() { qhFixture = saved }()

	seedQueryHourFixture(ctx, t, pool)
	ctx = repository.WithSystemIdentity(ctx)
	if _, err := service.NewQueryHourService(
		repository.NewQueryHourRepository(repository.NewScoped(pool)), "@example.com").
		Recompute(ctx); err != nil {
		t.Fatalf("Recompute: %v", err)
	}

	consumed := func(key string) int {
		var mins int
		if err := pool.QueryRow(ctx,
			`SELECT COALESCE(FLOOR(EXTRACT(EPOCH FROM consumed_duration)/60),0)::int
			   FROM project WHERE key = $1`, key).Scan(&mins); err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		return mins
	}

	// Onboarding live: 1200 of the 3000 came out of the grant.
	if got := consumed("INPROG"); got != 1800 {
		t.Errorf("IN_PROGRESS consumed = %d, want 1800 (3000 less the 1200 onboarding absorbed)", got)
	}
	// Onboarding finished: what it absorbed stays absorbed.
	if got := consumed("DONE"); got != 1800 {
		t.Errorf("COMPLETED consumed = %d, want 1800 -- a one-time grant is not clawed back", got)
	}
	// Renewal: the whole 3000 hits query hours.
	if got := consumed("RENEW"); got != 3000 {
		t.Errorf("renewal consumed = %d, want 3000 -- renewals have no grant", got)
	}
	if got := consumed("STALE"); got != 3000 {
		t.Errorf("renewal with a stale onboarding figure consumed = %d, want 3000", got)
	}
}
