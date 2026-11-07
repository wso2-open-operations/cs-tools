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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Skipped unless ENTITY_TEST_DATABASE_URL is set. Seeds its own rows under the
// 6d3... id prefix, deleting them before and after.
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestOpenBeforeYesterdayReport ./internal/repository/

func obyID(kind string, n int) string {
	return fmt.Sprintf("6d3%05d-0000-4000-8000-%012d", kind[0], n)
}

func obyCleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, s := range []string{
		`DELETE FROM work_item_tag WHERE work_item_id::text LIKE '6d3%'`,
		`DELETE FROM "case" WHERE id::text LIKE '6d3%'`,
		`DELETE FROM work_item WHERE id::text LIKE '6d3%'`,
		`DELETE FROM project WHERE id::text LIKE '6d3%'`,
		`DELETE FROM tag WHERE id::text LIKE '6d3%'`,
		`DELETE FROM product WHERE id::text LIKE '6d3%'`,
		`DELETE FROM project_type WHERE id::text LIKE '6d3%'`,
		`DELETE FROM account WHERE id::text LIKE '6d3%'`,
		`DELETE FROM "group" WHERE id::text LIKE '6d3%'`,
		`DELETE FROM "user" WHERE id::text LIKE '6d3%'`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("cleanup %q: %v", s, err)
		}
	}
}

func TestOpenBeforeYesterdayReportIntegration(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	obyCleanup(t, pool)
	t.Cleanup(func() { obyCleanup(t, pool) })
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -1)
	age := func(days int) time.Time { return now.AddDate(0, 0, -days) }

	const (
		assignee = "6d300000-0000-4000-8000-000000000001"
		group    = "6d300000-0000-4000-8000-000000000010"
		account  = "6d300000-0000-4000-8000-000000000020"
		typeSub  = "6d300000-0000-4000-8000-000000000030" // branch 1
		typeCS   = "6d300000-0000-4000-8000-000000000031" // branch 2: needs a product
		typeOth  = "6d300000-0000-4000-8000-000000000032" // not listed
		tagMigr  = "6d300000-0000-4000-8000-000000000040"
		product  = "6d300000-0000-4000-8000-000000000050"
	)
	exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, user_type, is_active)
		VALUES ($1, now(), now(), 'oby.assignee', 'Jane Doe', 'jane.doe@example.test', 'INTERNAL', true)`, assignee)
	exec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, is_active)
		VALUES ($1, now(), now(), 't', 't', 'Team Alpha', true)`, group)
	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, cre_team_id)
		VALUES ($1, now(), now(), 't', 't', 'Example Corp', 'ACC-OBY-1', 'SF-OBY-1', $2)`, account, group)
	exec(`INSERT INTO project_type (id, created_on, updated_on, created_by, updated_by, name) VALUES
		($1, now(), now(), 't', 't', 'OBY Subscription'),
		($2, now(), now(), 't', 't', 'OBY Cloud Support'),
		($3, now(), now(), 't', 't', 'OBY Other')`, typeSub, typeCS, typeOth)
	exec(`INSERT INTO tag (id, created_on, updated_on, name) VALUES ($1, now(), now(), 'migration')`, tagMigr)
	exec(`INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name, code)
		VALUES ($1, now(), now(), 't', 't', 'Example', 'SOFTWARE', 'OBY Product', 'obyprod')`, product)

	project := func(n int, typeID *string) string {
		id := obyID("p", n)
		exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, project_type_id)
			VALUES ($1, now(), now(), 't', 't', $2, $2, 'OBY project', $3, $4)`, id, fmt.Sprintf("OBY-PRJ-%d", n), account, typeID)
		return id
	}
	str := func(s string) *string { return &s }
	pSub := project(1, str(typeSub))
	pCS := project(2, str(typeCS))
	pOth := project(3, str(typeOth))
	pUntyped := project(4, nil)

	seq := 0
	caseRow := func(createdOn time.Time, state, wiType string, projectID *string, withProduct bool, severity *string) string {
		seq++
		id := obyID("c", seq)
		var prod *string
		if withProduct {
			prod = str(product)
		}
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type,
				account_id, project_id, assigned_to_id, product_id)
			VALUES ($1, $2, $2, 't', 't', $3, $4, 'subject', $5::work_item_type_enum, $6, $7, $8, $9)`,
			id, createdOn, fmt.Sprintf("CASEOBY%02d", seq), fmt.Sprintf("OBY-WSO2-%02d", seq), wiType, account, projectID, assignee, prod)
		exec(`INSERT INTO "case" (id, state, severity) VALUES ($1, $2::case_state_enum, $3::case_severity_enum)`, id, state, severity)
		return id
	}

	// Included, created out of order.
	incNew := caseRow(age(5), "OPEN", "CASE", &pSub, false, str("S2")) // branch 1, no product needed, newest
	caseRow(age(9), "OPEN", "CASE", &pCS, true, str("S1"))             // branch 2 with a product, oldest
	tagged := caseRow(age(7), "OPEN", "CASE", &pSub, false, nil)       // a migration tag does not exclude here
	exec(`INSERT INTO work_item_tag (id, created_on, updated_on, work_item_id, tag_id) VALUES (gen_random_uuid(), now(), now(), $1, $2)`, tagged, tagMigr)
	etaShared := caseRow(age(6), "OPEN", "CASE", &pSub, false, nil) // an ETA already shared does not exclude here
	exec(`UPDATE work_item SET eta_shared_on = now() WHERE id = $1`, etaShared)

	// Excluded, one per condition.
	caseRow(age(0), "OPEN", "CASE", &pSub, false, nil)             // created today
	caseRow(age(9), "WORK_IN_PROGRESS", "CASE", &pSub, false, nil) // not exactly open
	caseRow(age(9), "REOPENED", "CASE", &pSub, false, nil)         // not exactly open
	caseRow(age(9), "OPEN", "CASE", &pCS, false, nil)              // branch 2 without a product
	caseRow(age(9), "OPEN", "CASE", &pOth, true, nil)              // project type not configured
	caseRow(age(9), "OPEN", "CASE", &pUntyped, true, nil)          // project has no type
	caseRow(age(9), "OPEN", "CASE", nil, true, nil)                // no project
	caseRow(age(9), "OPEN", "SERVICE_REQUEST", &pSub, false, nil)  // not a plain case

	repo := repository.NewLongOpenCaseReportRepository(repository.NewScoped(pool))
	sysCtx := repository.WithSystemIdentity(ctx)
	types := []string{"OBY Subscription"}
	productTypes := []string{"OBY Cloud Support"}

	got, err := repo.ListOpenCasesBefore(sysCtx, cutoff, types, productTypes, 5000)
	if err != nil {
		t.Fatalf("ListOpenCasesBefore: %v", err)
	}
	var nums []string
	for _, r := range got.Cases {
		nums = append(nums, r.Number)
	}
	// age(9), age(7), age(6), age(5): oldest first.
	if want := []string{"CASEOBY02", "CASEOBY03", "CASEOBY04", "CASEOBY01"}; fmt.Sprint(nums) != fmt.Sprint(want) {
		t.Errorf("cases = %v, want %v", nums, want)
	}
	if got.CasesTotal != 4 || len(got.OnboardingCases) != 0 || got.OnboardingCasesTotal != 0 {
		t.Errorf("totals = %d/%d with %d onboarding rows, want 4/0/0", got.CasesTotal, got.OnboardingCasesTotal, len(got.OnboardingCases))
	}
	first := got.Cases[0]
	if first.InternalID != "OBY-WSO2-02" || first.State != "open" || first.Severity != "critical" ||
		first.AssignedEngineer != "Jane Doe" || first.CsTeam != "Team Alpha" || first.AccountName != "Example Corp" ||
		first.CreatedOn.IsZero() || first.UpdatedOn.IsZero() {
		t.Errorf("first row = %+v: identifiers, state, severity, assignee, team, account or dates are wrong", first)
	}

	// Strictly before the cut-off: a case created exactly at it is out.
	exec(`UPDATE work_item SET created_on = $1 WHERE id = $2`, cutoff, incNew)
	if got, err = repo.ListOpenCasesBefore(sysCtx, cutoff, types, productTypes, 5000); err != nil || got.CasesTotal != 3 {
		t.Errorf("a case created exactly at the cut-off must not be listed: total=%d err=%v", got.CasesTotal, err)
	}

	// The cap cuts to the oldest rows; the total stays full.
	got, err = repo.ListOpenCasesBefore(sysCtx, cutoff, types, productTypes, 1)
	if err != nil || len(got.Cases) != 1 || got.Cases[0].Number != "CASEOBY02" || got.CasesTotal != 3 {
		t.Errorf("capped = %d rows (total %d, err %v), want the oldest 1 of 3", len(got.Cases), got.CasesTotal, err)
	}

	// With the second set empty, branch 2 contributes nothing.
	if got, err = repo.ListOpenCasesBefore(sysCtx, cutoff, types, nil, 5000); err != nil || got.CasesTotal != 2 {
		t.Errorf("without branch-2 types total = %d (err %v), want 2", got.CasesTotal, err)
	}
}
