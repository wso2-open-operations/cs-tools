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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A service request created from a GitHub issue takes the native numbers a
// portal-created one takes (CS number, <KEY>-<n> id) when its repository maps
// to a project, and keeps the GitHub series when it does not. Against a real
// database; skipped unless ENTITY_TEST_DATABASE_URL is set.

const (
	nativeProjectID = "0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8aa"
	nativeProjKey   = "NATV"
	nativeOwner     = "native-num-it"
)

func TestGithubNativeNumberingIntegration(t *testing.T) {
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
		mustExec(t, pool, `DELETE FROM work_item WHERE github_owner = $1`, nativeOwner)
		mustExec(t, pool, `DELETE FROM project WHERE id = $1::uuid`, nativeProjectID)
	}
	cleanup()
	t.Cleanup(cleanup)
	mustExec(t, pool, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key)
	                   VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', $2)`, nativeProjectID, nativeProjKey)

	repo := NewGithubMutationRepository(NewScoped(pool))
	create := func(n int, project string) (id, number, wso2 string) {
		t.Helper()
		id, number, err := repo.CreateServiceRequestFromIssue(ctx, NewServiceRequestFromIssue{
			Subject: "Rotate the gateway certificate", Description: "### Markdown",
			IssueNumber: n, Owner: nativeOwner, Repository: "repo",
			Catalog: "General Requests", CreatedBy: "octocat", ProjectID: project,
		})
		if err != nil {
			t.Fatalf("create issue %d: %v", n, err)
		}
		if err := pool.QueryRow(ctx, `SELECT wso2_id FROM work_item WHERE id = $1::uuid`, id).Scan(&wso2); err != nil {
			t.Fatal(err)
		}
		return
	}
	ghSeq := func() (n, w int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT (SELECT last_value FROM github_service_request_number_seq),
		                                      (SELECT last_value FROM github_service_request_wso2_id_seq)`).Scan(&n, &w); err != nil {
			t.Fatal(err)
		}
		return
	}

	// With a project: CS number and <KEY>-<n>, the GitHub series untouched.
	nBefore, wBefore := ghSeq()
	id1, number1, wso2a := create(1, nativeProjectID)
	_, number2, wso2b := create(2, nativeProjectID)
	if !regexp.MustCompile(`^CS\d+$`).MatchString(number1) || number1 == number2 {
		t.Errorf("numbers = %q, %q, want distinct CS-series numbers", number1, number2)
	}
	if wso2a != nativeProjKey+"-1" || wso2b != nativeProjKey+"-2" {
		t.Errorf("wso2 ids = %q, %q, want %s-1 and %s-2", wso2a, wso2b, nativeProjKey, nativeProjKey)
	}
	if n, w := ghSeq(); n != nBefore || w != wBefore {
		t.Errorf("GitHub series advanced for project-bound records: %d->%d, %d->%d", nBefore, n, wBefore, w)
	}

	// Without a project there is no key to build an id from: GitHub series.
	_, number3, wso2c := create(3, "")
	if !strings.HasPrefix(number3, "SR-GH-") || !strings.HasPrefix(wso2c, "WSO2-GH-") {
		t.Errorf("project-less record = %q / %q, want the GitHub series", number3, wso2c)
	}

	// An edit keeps the issue's Markdown on a natively numbered record, as it
	// does on a GitHub-series one: no ServiceNowDescription is supplied.
	if err := repo.UpdateFromIssue(ctx, id1, NewChangeRequestFromIssue{
		Subject: "edited", Description: "### Edited markdown", CreatedBy: "octocat",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	var got string
	if err := pool.QueryRow(ctx, `SELECT description FROM work_item WHERE id = $1::uuid`, id1).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "### Edited markdown" {
		t.Errorf("description after edit = %q", got)
	}
}
