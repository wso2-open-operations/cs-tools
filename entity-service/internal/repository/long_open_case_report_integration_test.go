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

// Skipped unless ENTITY_TEST_DATABASE_URL is set. Runs against a migrated
// Postgres and seeds its own rows under the 6d2... id prefix, deleting
// them before and after so a half-finished run cannot poison the next.
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestLongOpenCaseReport ./internal/repository/

const (
	locrAssigneeID = "6d200000-0000-4000-8000-000000000001"
	locrGroupID    = "6d200000-0000-4000-8000-000000000010"
	locrAccountID  = "6d200000-0000-4000-8000-000000000020"
	locrTypeAID    = "6d200000-0000-4000-8000-000000000030"
	locrTypeBID    = "6d200000-0000-4000-8000-000000000031"
	locrTypeOther  = "6d200000-0000-4000-8000-000000000032"
	locrTagMigr    = "6d200000-0000-4000-8000-000000000040"
	locrTagAssist  = "6d200000-0000-4000-8000-000000000041"
	locrTagOther   = "6d200000-0000-4000-8000-000000000042"
)

func locrID(prefix string, n int) string {
	return fmt.Sprintf("6d2%05d-0000-4000-8000-%012d", prefix[0], n)
}

func locrCleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, s := range []string{
		`DELETE FROM work_item_tag WHERE work_item_id::text LIKE '6d2%'`,
		`DELETE FROM "case" WHERE id::text LIKE '6d2%'`,
		`DELETE FROM work_item WHERE id::text LIKE '6d2%'`,
		`DELETE FROM project WHERE id::text LIKE '6d2%'`,
		`DELETE FROM tag WHERE id::text LIKE '6d2%'`,
		`DELETE FROM project_type WHERE id::text LIKE '6d2%'`,
		`DELETE FROM account WHERE id::text LIKE '6d2%'`,
		`DELETE FROM "group" WHERE id::text LIKE '6d2%'`,
		`DELETE FROM "user" WHERE id::text LIKE '6d2%'`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("cleanup %q: %v", s, err)
		}
	}
}

func TestLongOpenCaseReportIntegration(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	locrCleanup(t, pool)
	t.Cleanup(func() { locrCleanup(t, pool) })
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -30)
	age := func(days int) time.Time { return now.AddDate(0, 0, -days) }

	exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, user_type, is_active)
		VALUES ($1, now(), now(), 'locr.assignee', 'Jane Doe', 'jane.doe@example.test', 'INTERNAL', true)`, locrAssigneeID)
	exec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, is_active)
		VALUES ($1, now(), now(), 't', 't', 'Team Alpha', true)`, locrGroupID)
	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, cre_team_id)
		VALUES ($1, now(), now(), 't', 't', 'Acme', 'ACC-LOCR-1', 'SF-LOCR-1', $2)`, locrAccountID, locrGroupID)
	exec(`INSERT INTO project_type (id, created_on, updated_on, created_by, updated_by, name) VALUES
		($1, now(), now(), 't', 't', 'LOCR Type A'),
		($2, now(), now(), 't', 't', 'LOCR Type B'),
		($3, now(), now(), 't', 't', 'LOCR Other Type')`, locrTypeAID, locrTypeBID, locrTypeOther)
	// Tags are seeded with odd casing: matching must ignore case.
	exec(`INSERT INTO tag (id, created_on, updated_on, name) VALUES
		($1, now(), now(), 'MIGRARTION'), ($2, now(), now(), 'Migration-Assistance'), ($3, now(), now(), 'locr-unrelated')`,
		locrTagMigr, locrTagAssist, locrTagOther)

	project := func(n int, typeID *string, onboarding *string) string {
		id := locrID("p", n)
		exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, project_type_id, onboarding_status)
			VALUES ($1, now(), now(), 't', 't', $2, $2, 'LOCR project', $3, $4, $5::onboarding_status_enum)`,
			id, fmt.Sprintf("LOCR-PRJ-%d", n), locrAccountID, typeID, onboarding)
		return id
	}
	str := func(s string) *string { return &s }
	pNull := project(1, str(locrTypeAID), nil)                // onboarding status NULL
	pDone := project(2, str(locrTypeBID), str("COMPLETED"))   // not onboarding
	pProg := project(3, str(locrTypeAID), str("IN_PROGRESS")) // onboarding
	pNew := project(4, str(locrTypeBID), str("NOT_STARTED"))  // onboarding
	pWrong := project(5, str(locrTypeOther), nil)             // type not configured
	pUntyped := project(6, nil, nil)                          // project without a type

	seq := 0
	// caseRow seeds a work item plus its case row. projectID may be nil.
	caseRow := func(createdOn time.Time, state, wiType string, projectID *string, severity *string) string {
		seq++
		id := locrID("c", seq)
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type,
				account_id, project_id, assigned_to_id)
			VALUES ($1, $2, $2, 't', 't', $3, $4, 'subject', $5::work_item_type_enum, $6, $7, $8)`,
			id, createdOn, fmt.Sprintf("CASELOCR%02d", seq), fmt.Sprintf("LOCR-WSO2-%02d", seq), wiType, locrAccountID, projectID, locrAssigneeID)
		exec(`INSERT INTO "case" (id, state, severity) VALUES ($1, $2::case_state_enum, $3::case_severity_enum)`, id, state, severity)
		return id
	}
	tagged := func(workItemID, tagID string) {
		exec(`INSERT INTO work_item_tag (id, created_on, updated_on, work_item_id, tag_id)
			VALUES (gen_random_uuid(), now(), now(), $1, $2)`, workItemID, tagID)
	}

	// Included. Created out of order so ordering is the query's work.
	incNew := caseRow(age(40), "REOPENED", "CASE", &pDone, str("S2"))      // cases, newest of the three
	caseRow(age(90), "OPEN", "CASE", &pNull, str("S1"))                    // cases, project onboarding NULL
	incTagged := caseRow(age(60), "WORK_IN_PROGRESS", "CASE", &pNull, nil) // cases, carries an unrelated tag
	tagged(incTagged, locrTagOther)
	caseRow(age(70), "WORK_IN_PROGRESS", "CASE", &pProg, str("S3")) // onboarding, newest
	caseRow(age(80), "WAITING_ON_WSO2", "CASE", &pNew, str("S4"))   // onboarding, oldest

	// Excluded, one per condition.
	caseRow(age(5), "OPEN", "CASE", &pNull, nil)               // too new
	caseRow(age(90), "AWAITING_INFO", "CASE", &pNull, nil)     // wrong state
	caseRow(age(90), "SOLUTION_PROPOSED", "CASE", &pNull, nil) // wrong state
	caseRow(age(90), "CLOSED", "CASE", &pNull, nil)            // wrong state
	caseRow(age(90), "OPEN", "CASE", &pWrong, nil)             // project type not configured
	caseRow(age(90), "OPEN", "CASE", &pUntyped, nil)           // project has no type
	caseRow(age(90), "OPEN", "CASE", nil, nil)                 // no project
	caseRow(age(90), "OPEN", "SERVICE_REQUEST", &pNull, nil)   // not a plain case
	tagMigr := caseRow(age(90), "OPEN", "CASE", &pNull, nil)   // excluded tag, other casing
	tagged(tagMigr, locrTagMigr)
	tagAssist := caseRow(age(90), "OPEN", "CASE", &pProg, nil) // excluded tag in the onboarding table
	tagged(tagAssist, locrTagAssist)
	etaShared := caseRow(age(90), "OPEN", "CASE", &pNull, nil) // ETA already shared
	exec(`UPDATE work_item SET eta_shared_on = now() WHERE id = $1`, etaShared)

	repo := repository.NewLongOpenCaseReportRepository(repository.NewScoped(pool))
	sysCtx := repository.WithSystemIdentity(ctx)
	types := []string{"LOCR Type A", "LOCR Type B"}
	excluded := []string{"migrartion", "MIGRATION-ASSISTANCE"}

	got, err := repo.ListLongOpenCases(sysCtx, cutoff, types, excluded, 5000)
	if err != nil {
		t.Fatalf("ListLongOpenCases: %v", err)
	}
	ids := func(rows []string) string { return fmt.Sprint(rows) }
	numbersOf := func(n ...string) []string { return n }
	var casesNums, onbNums []string
	for _, r := range got.Cases {
		casesNums = append(casesNums, r.Number)
	}
	for _, r := range got.OnboardingCases {
		onbNums = append(onbNums, r.Number)
	}
	// age(90) first, then age(60), then age(40).
	if want := numbersOf("CASELOCR02", "CASELOCR03", "CASELOCR01"); ids(casesNums) != ids(want) {
		t.Errorf("cases = %v, want %v (oldest first, only the included ones)", casesNums, want)
	}
	if want := numbersOf("CASELOCR05", "CASELOCR04"); ids(onbNums) != ids(want) {
		t.Errorf("onboardingCases = %v, want %v (oldest first, only the included ones)", onbNums, want)
	}
	if got.CasesTotal != 3 || got.OnboardingCasesTotal != 2 {
		t.Errorf("totals = %d/%d, want 3/2", got.CasesTotal, got.OnboardingCasesTotal)
	}

	first := got.Cases[0]
	if first.InternalID != "LOCR-WSO2-02" || first.State != "open" || first.Severity != "critical" ||
		first.AssignedEngineer != "Jane Doe" || first.CsTeam != "Team Alpha" {
		t.Errorf("first case row = %+v: identifiers, state, severity, assignee or CS team is wrong", first)
	}
	if wantID := locrID("c", 2); first.ID != wantID || first.ID == first.InternalID {
		t.Errorf("first case row id = %q, want the case UUID %q (and not the internal id)", first.ID, wantID)
	}
	if first.CreatedOn.IsZero() || first.UpdatedOn.IsZero() {
		t.Errorf("first case row dates not set: %+v", first)
	}
	if got.Cases[1].Severity != "" {
		t.Errorf("a case with no severity must report an empty severity, got %q", got.Cases[1].Severity)
	}
	if got.OnboardingCases[0].State != "waiting_on_wso2" || got.Cases[2].State != "reopened" {
		t.Errorf("states = %q/%q, want waiting_on_wso2/reopened", got.OnboardingCases[0].State, got.Cases[2].State)
	}

	// The cut-off instant itself is included; later than it is not.
	exec(`UPDATE work_item SET created_on = $1 WHERE id = $2`, cutoff, incNew)
	if got, err = repo.ListLongOpenCases(sysCtx, cutoff, types, excluded, 5000); err != nil || got.CasesTotal != 3 {
		t.Errorf("a case created exactly at the cut-off must be listed: total=%d err=%v", got.CasesTotal, err)
	}
	exec(`UPDATE work_item SET created_on = $1 WHERE id = $2`, cutoff.Add(time.Hour), incNew)
	if got, err = repo.ListLongOpenCases(sysCtx, cutoff, types, excluded, 5000); err != nil || got.CasesTotal != 2 {
		t.Errorf("a case created after the cut-off must not be listed: total=%d err=%v", got.CasesTotal, err)
	}
	exec(`UPDATE work_item SET created_on = $1 WHERE id = $2`, age(40), incNew)

	// The cap cuts each table to its oldest rows but the totals stay full.
	got, err = repo.ListLongOpenCases(sysCtx, cutoff, types, excluded, 1)
	if err != nil {
		t.Fatalf("ListLongOpenCases (capped): %v", err)
	}
	if len(got.Cases) != 1 || got.Cases[0].Number != "CASELOCR02" || got.CasesTotal != 3 ||
		len(got.OnboardingCases) != 1 || got.OnboardingCases[0].Number != "CASELOCR05" || got.OnboardingCasesTotal != 2 {
		t.Errorf("capped = %d rows (total %d) and %d rows (total %d), want the oldest 1 of 3 and 1 of 2",
			len(got.Cases), got.CasesTotal, len(got.OnboardingCases), got.OnboardingCasesTotal)
	}

	// With no excluded tags the two tagged cases come back.
	got, err = repo.ListLongOpenCases(sysCtx, cutoff, types, nil, 5000)
	if err != nil || got.CasesTotal != 4 || got.OnboardingCasesTotal != 3 {
		t.Errorf("without excluded tags totals = %d/%d (err %v), want 4/3", got.CasesTotal, got.OnboardingCasesTotal, err)
	}
}
