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

// Skipped unless ENTITY_TEST_DATABASE_URL is set. Runs against a migrated
// Postgres and seeds its own rows under the 6d2... id prefix, deleting
// them before and after so a half-finished run cannot poison the next.
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestProductRegressionReport ./internal/repository/

const (
	prrProductAID     = "6d200000-0000-4000-8000-000000000030"
	prrProductADupID  = "6d200000-0000-4000-8000-000000000031"
	prrProductBID     = "6d200000-0000-4000-8000-000000000032"
	prrDeployedID     = "6d200000-0000-4000-8000-000000000040"
	prrRegressionTag  = "6d200000-0000-4000-8000-000000000050"
	prrOtherTagID     = "6d200000-0000-4000-8000-000000000051"
	prrRegressionName = "S_RG_TEST"
)

// prrWorkItemID is the id of the n-th seeded work item.
func prrWorkItemID(n int) string {
	return "6d200000-0000-4000-8000-0000000001" + string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func prrPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func prrCleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, s := range []string{
		`DELETE FROM work_item_tag WHERE id::text LIKE '6d200000-0000-4000-8000-%' OR work_item_id::text LIKE '6d200000-0000-4000-8000-%'`,
		`DELETE FROM "case" WHERE id::text LIKE '6d200000-0000-4000-8000-%'`,
		`DELETE FROM work_item WHERE id::text LIKE '6d200000-0000-4000-8000-%'`,
		`DELETE FROM tag WHERE id::text LIKE '6d200000-0000-4000-8000-%'`,
		`DELETE FROM deployed_product WHERE id::text LIKE '6d200000-0000-4000-8000-%'`,
		`DELETE FROM product WHERE id::text LIKE '6d200000-0000-4000-8000-%'`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("cleanup %q: %v", s, err)
		}
	}
}

func TestProductRegressionReportIntegration(t *testing.T) {
	pool := prrPool(t)
	prrCleanup(t, pool)
	t.Cleanup(func() { prrCleanup(t, pool) })
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	loc, err := time.LoadLocation("Asia/Colombo")
	if err != nil {
		t.Fatalf("tz: %v", err)
	}
	periodStart := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	periodEnd := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)

	// Two product rows share a name, one with stray whitespace: the live
	// data has such a duplicate, and both must match and group together.
	exec(`INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name) VALUES
		($1, now(), now(), 't', 't', 'WSO2', 'SOFTWARE', 'PRR API Manager'),
		($2, now(), now(), 't', 't', 'WSO2', 'SOFTWARE', 'PRR API Manager '),
		($3, now(), now(), 't', 't', 'WSO2', 'SOFTWARE', 'PRR Identity Server')`,
		prrProductAID, prrProductADupID, prrProductBID)
	exec(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, name, product_id)
		VALUES ($1, now(), now(), 't', 't', 'PRR-DP-1', 'PRR API Manager 4.2.0 All In One', $2)`, prrDeployedID, prrProductAID)
	exec(`INSERT INTO tag (id, created_on, updated_on, name) VALUES
		($1, now(), now(), $3), ($2, now(), now(), 'prr_other')`, prrRegressionTag, prrOtherTagID, prrRegressionName)

	caseItem := func(n int, productID, deployedID *string, shared *time.Time, kind string) string {
		id := prrWorkItemID(n)
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type,
				product_id, deployed_product_id, eta_shared_on)
			VALUES ($1, now(), now(), 't', 't', $2, $3, $4, $5::work_item_type_enum, $6, $7, $8)`,
			id, "CSPRR"+id[len(id)-2:], "ACME-PRR-"+id[len(id)-2:], "regression "+id[len(id)-2:], kind, productID, deployedID, shared)
		if kind == "CASE" {
			exec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'WORK_IN_PROGRESS', 'S2')`, id)
		}
		return id
	}
	tagged := func(workItemID, tagID string) {
		exec(`INSERT INTO work_item_tag (id, created_on, updated_on, work_item_id, tag_id)
			VALUES (gen_random_uuid(), now(), now(), $1, $2)`, workItemID, tagID)
	}
	at := func(t time.Time) *time.Time { return &t }
	a, aDup, b, dp := prrProductAID, prrProductADupID, prrProductBID, prrDeployedID

	// Listed: product A, tagged, ETA shared mid-month, with a deployment.
	listed1 := caseItem(1, &a, &dp, at(time.Date(2026, 9, 10, 9, 0, 0, 0, loc)), "CASE")
	tagged(listed1, prrRegressionTag)
	// Listed: the duplicate product row, ETA shared in the last half hour
	// of September in Colombo (18:00 UTC on the 30th: inside the month in
	// either zone, but a UTC-anchored window would also admit the first
	// 5h30 of October, which case 3 guards against).
	listed2 := caseItem(2, &aDup, nil, at(time.Date(2026, 9, 30, 23, 30, 0, 0, loc)), "CASE")
	tagged(listed2, prrRegressionTag)
	// Not listed: ETA shared at the first instant of October (end is exclusive).
	tagged(caseItem(3, &a, nil, at(periodEnd), "CASE"), prrRegressionTag)
	// Not listed: ETA shared in the last minute of August.
	tagged(caseItem(4, &a, nil, at(periodStart.Add(-time.Minute)), "CASE"), prrRegressionTag)
	// Not listed: in the month, right product, but not tagged as a regression.
	tagged(caseItem(5, &a, nil, at(time.Date(2026, 9, 15, 0, 0, 0, 0, loc)), "CASE"), prrOtherTagID)
	// Listed only when product B is asked for.
	listedB := caseItem(6, &b, nil, at(time.Date(2026, 9, 5, 0, 0, 0, 0, loc)), "CASE")
	tagged(listedB, prrRegressionTag)
	// Not listed: no product at all.
	tagged(caseItem(7, nil, nil, at(time.Date(2026, 9, 5, 0, 0, 0, 0, loc)), "CASE"), prrRegressionTag)
	// Not listed: no ETA ever shared.
	tagged(caseItem(8, &a, nil, nil, "CASE"), prrRegressionTag)
	// Not listed: an engagement, not a case.
	tagged(caseItem(9, &a, nil, at(time.Date(2026, 9, 5, 0, 0, 0, 0, loc)), "ENGAGEMENT"), prrRegressionTag)

	repo := repository.NewProductRegressionReportRepository(repository.NewScoped(pool))
	sysCtx := repository.WithSystemIdentity(ctx)

	numbersOf := func(rows []domain.ProductRegressionReportRow) []string {
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.Number)
		}
		return out
	}

	// Product names and the tag are matched ignoring case and whitespace.
	rows, err := repo.ListRegressionCases(sysCtx, periodStart, periodEnd, "s_rg_test", []string{" prr api manager ", "PRR Microgateway"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got, want := numbersOf(rows), []string{"CSPRR01", "CSPRR02"}; !equalStringSlices(got, want) {
		t.Fatalf("product A rows = %v, want %v (ETA order; boundaries, tag, product, type and NULL ETA all respected)", got, want)
	}
	if rows[0].Product != "PRR API Manager" || rows[1].Product != "PRR API Manager" {
		t.Errorf("products = %q / %q, want both trimmed to the same name so they group together", rows[0].Product, rows[1].Product)
	}
	if rows[0].DeployedProduct != "PRR API Manager 4.2.0 All In One" || rows[1].DeployedProduct != "" {
		t.Errorf("deployed products = %q / %q", rows[0].DeployedProduct, rows[1].DeployedProduct)
	}
	if rows[0].IssueKey != "ACME-PRR-01" || rows[0].Subject != "regression 01" || rows[0].ID != listed1 {
		t.Errorf("row 1 = %+v", rows[0])
	}
	if !rows[1].EtaSharedOn.Equal(time.Date(2026, 9, 30, 23, 30, 0, 0, loc)) {
		t.Errorf("row 2 ETA shared = %s", rows[1].EtaSharedOn)
	}

	// Two products: product name order, B's case after A's.
	rows, err = repo.ListRegressionCases(sysCtx, periodStart, periodEnd, prrRegressionName, []string{"PRR Identity Server", "PRR API Manager"})
	if err != nil {
		t.Fatalf("list both: %v", err)
	}
	if got, want := numbersOf(rows), []string{"CSPRR01", "CSPRR02", "CSPRR06"}; !equalStringSlices(got, want) {
		t.Errorf("both products = %v, want %v in product name order", got, want)
	}

	// A product nobody has, or no product at all, lists nothing.
	rows, err = repo.ListRegressionCases(sysCtx, periodStart, periodEnd, prrRegressionName, []string{"PRR Microgateway"})
	if err != nil || len(rows) != 0 {
		t.Errorf("unknown product: rows = %v, err = %v", rows, err)
	}
	rows, err = repo.ListRegressionCases(sysCtx, periodStart, periodEnd, prrRegressionName, []string{" ", ""})
	if err != nil || rows == nil || len(rows) != 0 {
		t.Errorf("blank products: rows = %v, err = %v; want an empty, non-nil list without a query", rows, err)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
