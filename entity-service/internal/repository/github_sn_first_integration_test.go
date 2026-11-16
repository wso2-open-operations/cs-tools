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
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A service request created ServiceNow-first from a GitHub issue, against a
// real database. Skipped unless ENTITY_TEST_DATABASE_URL is set.

const (
	snFirstID      = "0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9"
	snFirstOwner   = "sn-first-it"
	snFirstRepo    = "repo"
	snFirstGHOwner = "sn-first-it-gh"
)

func TestGithubSNFirstIntegration_TakesServiceNowIDs(t *testing.T) {
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
		mustExec(t, pool, `DELETE FROM work_item WHERE github_owner IN ($1, $2)`, snFirstOwner, snFirstGHOwner)
	}
	cleanup()
	t.Cleanup(cleanup)

	seq := func() (number, wso2 int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT (SELECT last_value FROM github_service_request_number_seq),
		                                      (SELECT last_value FROM github_service_request_wso2_id_seq)`).Scan(&number, &wso2); err != nil {
			t.Fatalf("read sequences: %v", err)
		}
		return
	}
	numberBefore, wso2Before := seq()

	repo := NewGithubMutationRepository(NewScoped(pool))
	id, number, err := repo.CreateServiceRequestFromIssue(ctx, NewServiceRequestFromIssue{
		Subject: "Rotate the gateway certificate", Description: "<span>ServiceNow's text</span>",
		IssueNumber: 42, Owner: snFirstOwner, Repository: snFirstRepo,
		Catalog: "General Requests", CreatedBy: "octocat",
		ServiceNowID: snFirstID, ServiceNowNumber: "CS0441400", ServiceNowWSO2ID: "WCSSUB-10",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var wso2ID string
	if err := pool.QueryRow(ctx, `SELECT wso2_id FROM work_item WHERE id = $1`, id).Scan(&wso2ID); err != nil {
		t.Fatal(err)
	}
	if id != snFirstID || number != "CS0441400" || wso2ID != "WCSSUB-10" {
		t.Errorf("id=%s number=%s wso2_id=%s, want ServiceNow's", id, number, wso2ID)
	}
	if n, w := seq(); n != numberBefore || w != wso2Before {
		t.Errorf("the GitHub series advanced for a ServiceNow-numbered record: %d->%d, %d->%d", numberBefore, n, wso2Before, w)
	}

	// The GitHub series still numbers a record with no project (see
	// github_native_numbering_integration_test.go for the project-bound case).
	ghID, ghNumber, err := repo.CreateServiceRequestFromIssue(ctx, NewServiceRequestFromIssue{
		Subject: "x", Description: "### Markdown", IssueNumber: 43, Owner: snFirstGHOwner, Repository: snFirstRepo,
		Catalog: "General Requests", CreatedBy: "octocat",
	})
	if err != nil {
		t.Fatalf("create GitHub-series record: %v", err)
	}
	if len(ghNumber) < 6 || ghNumber[:6] != "SR-GH-" {
		t.Errorf("GitHub-series number = %q", ghNumber)
	}

	// An edit keeps ServiceNow's text on its record and the issue's Markdown
	// on the GitHub-series one.
	edit := NewChangeRequestFromIssue{Subject: "edited", Description: "### Edited markdown",
		ServiceNowDescription: "<span>Edited ServiceNow text</span>", CreatedBy: "octocat"}
	for _, rid := range []string{id, ghID} {
		if err := repo.UpdateFromIssue(ctx, rid, edit); err != nil {
			t.Fatalf("update %s: %v", rid, err)
		}
	}
	for rid, want := range map[string]string{id: "<span>Edited ServiceNow text</span>", ghID: "### Edited markdown"} {
		var got string
		if err := pool.QueryRow(ctx, `SELECT description FROM work_item WHERE id = $1`, rid).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s description = %q, want %q", rid, got, want)
		}
	}
}
