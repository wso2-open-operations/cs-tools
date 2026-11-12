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

package repository

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A GitHub-raised service request on plain Postgres takes the ServiceNow-style
// series a portal-created one takes (migration 0180, next_wso2_id from 0290),
// against a real database. Skipped unless ENTITY_TEST_DATABASE_URL is set.

const (
	nativeAccountID = "4aaaaaaa-0000-0000-0000-0000000000a1"
	nativeProjectID = "4aaaaaaa-0000-0000-0000-0000000000b1"
	nativeKey       = "GHNATIVEIT"
	nativeOwner     = "gh-native-it"
)

func TestGithubNativeNumbersIntegration(t *testing.T) {
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
	cleanup := func() {
		mustExec(t, pool, `DELETE FROM work_item WHERE github_owner = $1 OR wso2_id LIKE $2`, nativeOwner, nativeKey+"-%")
		mustExec(t, pool, `DELETE FROM project WHERE id = $1`, nativeProjectID)
		mustExec(t, pool, `DELETE FROM account WHERE id = $1`, nativeAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec(t, pool, `INSERT INTO account (id, name) VALUES ($1, 'GitHub native numbering IT')`, nativeAccountID)
	mustExec(t, pool, `INSERT INTO project (id, key, account_id) VALUES ($1, $2, $3)`, nativeProjectID, nativeKey, nativeAccountID)
	// Two synced records already hold <key>-1 and <key>-5: next_wso2_id must
	// step past them rather than collide.
	for i, wso2 := range []string{nativeKey + "-1", nativeKey + "-5"} {
		mustExec(t, pool, `INSERT INTO work_item (id, number, wso2_id, type, project_id)
			VALUES (gen_random_uuid(), $1, $2, 'SERVICE_REQUEST', $3)`,
			"GHNATIVESYNCED"+string(rune('A'+i)), wso2, nativeProjectID)
	}

	seq := func() (number, wso2 int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT (SELECT last_value FROM github_service_request_number_seq),
		                                      (SELECT last_value FROM github_service_request_wso2_id_seq)`).Scan(&number, &wso2); err != nil {
			t.Fatalf("read GitHub sequences: %v", err)
		}
		return
	}
	ghNumberBefore, ghWSO2Before := seq()

	repo := NewGithubMutationRepository(NewScoped(pool))
	id, number, err := repo.CreateServiceRequestFromIssue(ctx, NewServiceRequestFromIssue{
		Subject: "Native numbering", Description: "<span>ServiceNow-format text</span>",
		IssueNumber: 101, AccountID: nativeAccountID, ProjectID: nativeProjectID,
		Owner: nativeOwner, Repository: "repo", Catalog: "General Requests", CreatedBy: "octocat",
		NativeNumbers: true,
	})
	if err != nil {
		t.Fatalf("create with native numbers: %v", err)
	}
	var wso2ID string
	if err := pool.QueryRow(ctx, `SELECT wso2_id FROM work_item WHERE id = $1::uuid`, id).Scan(&wso2ID); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^CS\d{7,}$`).MatchString(number) {
		t.Errorf("number = %q, want the CS series a portal-created SR takes", number)
	}
	if wso2ID != nativeKey+"-6" {
		t.Errorf("wso2_id = %q, want %s-6 (past the synced -1 and -5)", wso2ID, nativeKey)
	}
	if n, w := seq(); n != ghNumberBefore || w != ghWSO2Before {
		t.Errorf("the GitHub series advanced for a natively numbered record: %d->%d, %d->%d", ghNumberBefore, n, ghWSO2Before, w)
	}

	// Without a project there is no key to number from: the GitHub series.
	_, ghNumber, err := repo.CreateServiceRequestFromIssue(ctx, NewServiceRequestFromIssue{
		Subject: "No project", IssueNumber: 102, AccountID: nativeAccountID,
		Owner: nativeOwner, Repository: "repo", Catalog: "General Requests", CreatedBy: "octocat",
		NativeNumbers: true,
	})
	if err != nil {
		t.Fatalf("create without a project: %v", err)
	}
	if !regexp.MustCompile(`^SR-GH-\d+$`).MatchString(ghNumber) {
		t.Errorf("project-less number = %q, want the GitHub series", ghNumber)
	}

	// NativeNumbers off keeps the GitHub series even with a project (dual-write).
	_, offNumber, err := repo.CreateServiceRequestFromIssue(ctx, NewServiceRequestFromIssue{
		Subject: "Native off", IssueNumber: 103, AccountID: nativeAccountID, ProjectID: nativeProjectID,
		Owner: nativeOwner, Repository: "repo", Catalog: "General Requests", CreatedBy: "octocat",
	})
	if err != nil {
		t.Fatalf("create with native numbers off: %v", err)
	}
	if !regexp.MustCompile(`^SR-GH-\d+$`).MatchString(offNumber) {
		t.Errorf("number with native off = %q, want the GitHub series", offNumber)
	}

	// ServiceNow's ids, when it created the record first, win over both series.
	_, snNumber, err := repo.CreateServiceRequestFromIssue(ctx, NewServiceRequestFromIssue{
		Subject: "ServiceNow first", IssueNumber: 104, AccountID: nativeAccountID, ProjectID: nativeProjectID,
		Owner: nativeOwner, Repository: "repo", Catalog: "General Requests", CreatedBy: "octocat",
		ServiceNowID: "4aaaaaaa-0000-0000-0000-0000000000c1", ServiceNowNumber: "CS9990001",
		ServiceNowWSO2ID: nativeKey + "-900", NativeNumbers: true,
	})
	if err != nil {
		t.Fatalf("create with ServiceNow ids: %v", err)
	}
	if snNumber != "CS9990001" {
		t.Errorf("number with ServiceNow ids = %q, want ServiceNow's CS9990001", snNumber)
	}
}
