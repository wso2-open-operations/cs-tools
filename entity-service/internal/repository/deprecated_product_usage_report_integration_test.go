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

// Runs the deprecated product usage query against a live, migrated PostgreSQL.
// Skipped unless ENTITY_TEST_DATABASE_URL is set:
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestDeprecatedProductUsageReport ./internal/repository/

func depID(n int) string {
	return "6e600000-0000-4000-8000-0000000000" + string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func depPool(t *testing.T) *pgxpool.Pool {
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

func depCleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, s := range []string{
		`DELETE FROM deployed_product WHERE id::text LIKE '6e600000-0000-4000-8000-%'`,
		`DELETE FROM deployment WHERE id::text LIKE '6e600000-0000-4000-8000-%'`,
		`DELETE FROM project WHERE id::text LIKE '6e600000-0000-4000-8000-%'`,
		`DELETE FROM account WHERE id::text LIKE '6e600000-0000-4000-8000-%'`,
		`DELETE FROM "group" WHERE id::text LIKE '6e600000-0000-4000-8000-%'`,
		`DELETE FROM "user" WHERE id::text LIKE '6e600000-0000-4000-8000-%'`,
		`DELETE FROM product_version WHERE id::text LIKE '6e600000-0000-4000-8000-%'`,
		`DELETE FROM product WHERE id::text LIKE '6e600000-0000-4000-8000-%'`,
	} {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatalf("cleanup %q: %v", s, err)
		}
	}
}

func TestDeprecatedProductUsageReportIntegration(t *testing.T) {
	pool := depPool(t)
	ctx := context.Background()
	depCleanup(t, pool)
	t.Cleanup(func() { depCleanup(t, pool) })

	const audit = `NOW(), 'it', NOW(), 'it'`
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`INSERT INTO "user" (id, user_name, email, first_name, last_name, created_on, updated_on) VALUES
		($1::uuid, 'dep-am', 'dep-am@example.com', 'Ann', 'Manager', NOW(), NOW()),
		($2::uuid, 'dep-to', 'dep-to@example.com', 'Tom', 'Owner', NOW(), NOW())`, depID(1), depID(2))
	exec(`INSERT INTO "group" (id, name, created_on, created_by, updated_on, updated_by) VALUES ($1::uuid, 'DEP IT Team', `+audit+`)`, depID(3))
	exec(`INSERT INTO account (id, number, name, region, cre_team_id, account_manager_id, technical_owner_id, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, 'DEPIT-A', 'DEP IT Account', 'APAC', $3::uuid, $4::uuid, $5::uuid, `+audit+`),
		($2::uuid, 'DEPIT-B', 'DEP IT No Region', NULL, NULL, NULL, NULL, `+audit+`)`, depID(10), depID(11), depID(3), depID(1), depID(2))
	exec(`INSERT INTO project (id, account_id, name, key, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, $3::uuid, 'DEP IT Project', 'DEPITPROJ', `+audit+`),
		($2::uuid, $4::uuid, 'DEP IT Other', 'DEPITOTHER', `+audit+`)`, depID(20), depID(21), depID(10), depID(11))
	exec(`INSERT INTO product (id, name, category, created_on, created_by, updated_on, updated_by) VALUES ($1::uuid, 'DEP IT Product', 'SOFTWARE', `+audit+`)`, depID(30))
	exec(`INSERT INTO product_version (id, product_id, version, support_eol_date, current_support_status, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, $4::uuid, '1.0.0', '2022-10-22', 'DEPRECATED', `+audit+`),
		($2::uuid, $4::uuid, '2.0.0', '2022-10-22', 'EXTENDED', `+audit+`),
		($3::uuid, $4::uuid, '0.9.0', NULL, 'DEPRECATED', `+audit+`),
		($5::uuid, $4::uuid, '0.8.0', '2030-01-01', 'DEPRECATED', `+audit+`)`, depID(31), depID(32), depID(33), depID(30), depID(34))
	// 40, 41: two active Primary Production deployments of one project; 42: inactive; 43: QA; 44: the no-region account's.
	exec(`INSERT INTO deployment (id, project_id, name, number, type, is_active, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, $6::uuid, 'pp1', 'DEPIT-D1', 'PRIMARY_PRODUCTION', TRUE, `+audit+`),
		($2::uuid, $6::uuid, 'pp2', 'DEPIT-D2', 'PRIMARY_PRODUCTION', TRUE, `+audit+`),
		($3::uuid, $6::uuid, 'old', 'DEPIT-D3', 'PRIMARY_PRODUCTION', FALSE, `+audit+`),
		($4::uuid, $6::uuid, 'qa',  'DEPIT-D4', 'QA', TRUE, `+audit+`),
		($5::uuid, $7::uuid, 'pp',  'DEPIT-D5', 'PRIMARY_PRODUCTION', TRUE, `+audit+`)`,
		depID(40), depID(41), depID(42), depID(43), depID(44), depID(20), depID(21))
	exec(`INSERT INTO deployed_product (id, number, deployment_id, version_id, active, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, 'DEPIT-P1', $8::uuid, $13::uuid, TRUE, `+audit+`),
		($2::uuid, 'DEPIT-P2', $9::uuid, $13::uuid, TRUE, `+audit+`),
		($3::uuid, 'DEPIT-P3', $8::uuid, $14::uuid, TRUE, `+audit+`),
		($4::uuid, 'DEPIT-P4', $10::uuid, $15::uuid, TRUE, `+audit+`),
		($5::uuid, 'DEPIT-P5', $11::uuid, $15::uuid, TRUE, `+audit+`),
		($6::uuid, 'DEPIT-P6', $8::uuid, $15::uuid, FALSE, `+audit+`),
		($7::uuid, 'DEPIT-P7', $12::uuid, $15::uuid, NULL, `+audit+`),
		($16::uuid, 'DEPIT-P8', $8::uuid, $17::uuid, TRUE, `+audit+`)`,
		depID(50), depID(51), depID(52), depID(53), depID(54), depID(55), depID(56),
		depID(40), depID(41), depID(42), depID(43), depID(44), depID(31), depID(32), depID(33), depID(57), depID(34))

	repo := repository.NewDeprecatedProductUsageReportRepository(repository.NewScoped(pool))
	rows, err := repo.ListDeprecatedUsage(repository.WithSystemIdentity(ctx))
	if err != nil {
		t.Fatalf("ListDeprecatedUsage: %v", err)
	}
	var mine []string
	for _, r := range rows {
		if r.AccountID == depID(10) || r.AccountID == depID(11) {
			mine = append(mine, r.Region+"|"+r.AccountName+"|"+r.ProjectKey+"|"+r.Version+"|"+r.EolDate+"|"+r.CsAbt+"|"+r.AccountManagerEmail+"|"+r.TechnicalOwnerName)
		}
	}
	// Deprecated versions whatever their EOL date: 1.0.0 (past, listed once
	// although two Primary Production deployments run it) before 0.8.0
	// (future). 2.0.0 is past its EOL date but not deprecated: the EOL report
	// lists it. 0.9.0 (no EOL date) only through active rows: the inactive
	// deployment, the QA deployment and the inactive deployed product are
	// left out, a NULL active flag counts as active. No region last.
	want := []string{
		"APAC|DEP IT Account|DEPITPROJ|1.0.0|2022-10-22|DEP IT Team|dep-am@example.com|Tom Owner",
		"APAC|DEP IT Account|DEPITPROJ|0.8.0|2030-01-01|DEP IT Team|dep-am@example.com|Tom Owner",
		"|DEP IT No Region|DEPITOTHER|0.9.0||||",
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

func TestDeprecatedProductUsageReportLeavesOutCustomersWhoLeft(t *testing.T) {
	pool := depPool(t)
	ctx := context.Background()
	depCleanup(t, pool)
	t.Cleanup(func() { depCleanup(t, pool) })

	const audit = `NOW(), 'it', NOW(), 'it'`
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	// 60: a current account; 61: an account deleted in Salesforce.
	exec(`INSERT INTO account (id, number, name, region, deleted_on, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, 'DEPIT-C', 'DEP IT Current', 'EMEA', NULL, `+audit+`),
		($2::uuid, 'DEPIT-D', 'DEP IT Deleted', 'EMEA', NOW(), `+audit+`)`, depID(60), depID(61))
	// Projects: 62 open, 63 inactive, 64 closed by WSO2, 65 suspended (still a
	// customer), 66 the deleted account's.
	exec(`INSERT INTO project (id, account_id, name, key, is_active, wso2_closure_state, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, $6::uuid, 'DEP IT Open', 'DEPITOPEN', TRUE, 'OPEN', `+audit+`),
		($2::uuid, $6::uuid, 'DEP IT Inactive', 'DEPITINACT', FALSE, NULL, `+audit+`),
		($3::uuid, $6::uuid, 'DEP IT Closed', 'DEPITCLOSED', TRUE, 'CLOSED', `+audit+`),
		($4::uuid, $6::uuid, 'DEP IT Suspended', 'DEPITSUSP', NULL, 'SUSPENDED', `+audit+`),
		($5::uuid, $7::uuid, 'DEP IT Gone', 'DEPITGONE', TRUE, NULL, `+audit+`)`,
		depID(62), depID(63), depID(64), depID(65), depID(66), depID(60), depID(61))
	exec(`INSERT INTO product (id, name, category, created_on, created_by, updated_on, updated_by) VALUES ($1::uuid, 'DEP IT Product', 'SOFTWARE', `+audit+`)`, depID(67))
	exec(`INSERT INTO product_version (id, product_id, version, support_eol_date, current_support_status, created_on, created_by, updated_on, updated_by) VALUES
		($1::uuid, $2::uuid, '1.0.0', '2027-10-22', 'DEPRECATED', `+audit+`)`, depID(68), depID(67))
	// One active Primary Production deployment per project, each running the deprecated version.
	for i := 0; i < 5; i++ {
		exec(`INSERT INTO deployment (id, project_id, name, number, type, is_active, created_on, created_by, updated_on, updated_by) VALUES
			($1::uuid, $2::uuid, 'pp', $3, 'PRIMARY_PRODUCTION', TRUE, `+audit+`)`, depID(70+i), depID(62+i), "DEPIT-L"+string(rune('0'+i)))
		exec(`INSERT INTO deployed_product (id, number, deployment_id, version_id, active, created_on, created_by, updated_on, updated_by) VALUES
			($1::uuid, $2, $3::uuid, $4::uuid, TRUE, `+audit+`)`, depID(80+i), "DEPIT-Q"+string(rune('0'+i)), depID(70+i), depID(68))
	}

	repo := repository.NewDeprecatedProductUsageReportRepository(repository.NewScoped(pool))
	rows, err := repo.ListDeprecatedUsage(repository.WithSystemIdentity(ctx))
	if err != nil {
		t.Fatalf("ListDeprecatedUsage: %v", err)
	}
	var keys []string
	for _, r := range rows {
		if r.AccountID == depID(60) || r.AccountID == depID(61) {
			keys = append(keys, r.ProjectKey)
		}
	}
	// The inactive project, the project WSO2 closed and the deleted account's
	// project are left out; the suspended project is still a customer.
	want := []string{"DEPITOPEN", "DEPITSUSP"}
	if len(keys) != len(want) || keys[0] != want[0] || keys[1] != want[1] {
		t.Fatalf("projects = %v, want %v", keys, want)
	}
}
