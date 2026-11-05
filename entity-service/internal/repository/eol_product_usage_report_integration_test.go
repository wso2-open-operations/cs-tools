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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Runs the EOL product usage query against a live, migrated PostgreSQL.
// Skipped unless ENTITY_TEST_DATABASE_URL is set:
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestEolProductUsageReport ./internal/repository/

func eolID(n int) string {
	return "6e500000-0000-4000-8000-0000000000" + string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func eolPool(t *testing.T) *pgxpool.Pool {
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

func eolCleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, s := range []string{
		`DELETE FROM deployed_product WHERE id::text LIKE '6e500000-0000-4000-8000-%'`,
		`DELETE FROM deployment WHERE id::text LIKE '6e500000-0000-4000-8000-%'`,
		`DELETE FROM project WHERE id::text LIKE '6e500000-0000-4000-8000-%'`,
		`DELETE FROM account WHERE id::text LIKE '6e500000-0000-4000-8000-%'`,
		`DELETE FROM "group" WHERE id::text LIKE '6e500000-0000-4000-8000-%'`,
		`DELETE FROM "user" WHERE id::text LIKE '6e500000-0000-4000-8000-%'`,
		`DELETE FROM product_version WHERE id::text LIKE '6e500000-0000-4000-8000-%'`,
		`DELETE FROM product WHERE id::text LIKE '6e500000-0000-4000-8000-%'`,
	} {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatalf("cleanup %q: %v", s, err)
		}
	}
}

func TestEolProductUsageReportIntegration(t *testing.T) {
	pool := eolPool(t)
	ctx := context.Background()
	eolCleanup(t, pool)
	t.Cleanup(func() { eolCleanup(t, pool) })

	const audit = `NOW(), 'it', NOW(), 'it'`
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`INSERT INTO "user" (id, user_name, email, first_name, last_name, created_on, updated_on) VALUES
		($1::uuid, 'eol-am', 'eol-am@example.com', 'Ann', 'Manager', NOW(), NOW()),
		($2::uuid, 'eol-to', 'eol-to@example.com', 'Tom', 'Owner', NOW(), NOW())`, eolID(1), eolID(2))
	exec(`INSERT INTO "group" (id, name, created_on, created_by, updated_on, updated_by) VALUES ($1::uuid, 'EOL IT Team', `+audit+`)`, eolID(3))
	exec(`INSERT INTO account (id, number, name, region, cre_team_id, account_manager_id, technical_owner_id, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, 'EOLIT-A', 'EOL IT Account', 'APAC', $3::uuid, $4::uuid, $5::uuid, `+audit+`),
		($2::uuid, 'EOLIT-B', 'EOL IT No Region', NULL, NULL, NULL, NULL, `+audit+`)`, eolID(10), eolID(11), eolID(3), eolID(1), eolID(2))
	exec(`INSERT INTO project (id, account_id, name, key, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, $3::uuid, 'EOL IT Project', 'EOLITPROJ', `+audit+`),
		($2::uuid, $4::uuid, 'EOL IT Other', 'EOLITOTHER', `+audit+`)`, eolID(20), eolID(21), eolID(10), eolID(11))
	exec(`INSERT INTO product (id, name, category, created_on, created_by, updated_on, updated_by) VALUES ($1::uuid, 'EOL IT Product', 'SOFTWARE', `+audit+`)`, eolID(30))
	exec(`INSERT INTO product_version (id, product_id, version, support_eol_date, current_support_status, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, $4::uuid, '1.0.0', '2022-10-22', 'EXTENDED', `+audit+`),
		($2::uuid, $4::uuid, '2.0.0', '2030-01-01', 'AVAILABLE', `+audit+`),
		($3::uuid, $4::uuid, '0.9.0', '2020-01-01', NULL, `+audit+`)`, eolID(31), eolID(32), eolID(33), eolID(30))
	// 40, 41: two active Primary Production deployments of one project; 42: inactive; 43: QA; 44: the no-region account's.
	exec(`INSERT INTO deployment (id, project_id, name, number, type, is_active, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, $6::uuid, 'pp1', 'EOLIT-D1', 'PRIMARY_PRODUCTION', TRUE, `+audit+`),
		($2::uuid, $6::uuid, 'pp2', 'EOLIT-D2', 'PRIMARY_PRODUCTION', TRUE, `+audit+`),
		($3::uuid, $6::uuid, 'old', 'EOLIT-D3', 'PRIMARY_PRODUCTION', FALSE, `+audit+`),
		($4::uuid, $6::uuid, 'qa',  'EOLIT-D4', 'QA', TRUE, `+audit+`),
		($5::uuid, $7::uuid, 'pp',  'EOLIT-D5', 'PRIMARY_PRODUCTION', TRUE, `+audit+`)`,
		eolID(40), eolID(41), eolID(42), eolID(43), eolID(44), eolID(20), eolID(21))
	exec(`INSERT INTO deployed_product (id, number, deployment_id, version_id, active, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, 'EOLIT-P1', $8::uuid, $13::uuid, TRUE, `+audit+`),
		($2::uuid, 'EOLIT-P2', $9::uuid, $13::uuid, TRUE, `+audit+`),
		($3::uuid, 'EOLIT-P3', $8::uuid, $14::uuid, TRUE, `+audit+`),
		($4::uuid, 'EOLIT-P4', $10::uuid, $15::uuid, TRUE, `+audit+`),
		($5::uuid, 'EOLIT-P5', $11::uuid, $15::uuid, TRUE, `+audit+`),
		($6::uuid, 'EOLIT-P6', $8::uuid, $15::uuid, FALSE, `+audit+`),
		($7::uuid, 'EOLIT-P7', $12::uuid, $15::uuid, NULL, `+audit+`)`,
		eolID(50), eolID(51), eolID(52), eolID(53), eolID(54), eolID(55), eolID(56),
		eolID(40), eolID(41), eolID(42), eolID(43), eolID(44), eolID(31), eolID(32), eolID(33))

	repo := repository.NewEolProductUsageReportRepository(repository.NewScoped(pool))
	rows, err := repo.ListEolUsage(repository.WithSystemIdentity(ctx), "2026-11-02")
	if err != nil {
		t.Fatalf("ListEolUsage: %v", err)
	}
	var mine []string
	for _, r := range rows {
		if r.AccountID == eolID(10) || r.AccountID == eolID(11) {
			mine = append(mine, r.Region+"|"+r.AccountName+"|"+r.ProjectKey+"|"+r.Version+"|"+r.CsAbt+"|"+r.AccountManagerEmail+"|"+r.TechnicalOwnerName)
		}
	}
	// 1.0.0 once although two Primary Production deployments run it; 2.0.0
	// is not past its EOL date; 0.9.0 only through active rows: the inactive
	// deployment, the QA deployment and the inactive deployed product are
	// left out, a NULL active flag counts as active; no-region last.
	want := []string{
		"APAC|EOL IT Account|EOLITPROJ|1.0.0|EOL IT Team|eol-am@example.com|Tom Owner",
		"|EOL IT No Region|EOLITOTHER|0.9.0|||",
	}
	if len(mine) != len(want) {
		t.Fatalf("rows = %v, want %v", mine, want)
	}
	for i := range want {
		if mine[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, mine[i], want[i])
		}
	}
}
