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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Skipped without CASE_STATS_TEST_DSN (a non-superuser role, so row-level
// security is enforced):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TimeCardReportVisibility

const (
	tcvAccountID        = "51111111-0000-0000-0000-000000000001"
	tcvAccountContactID = "51111111-0000-0000-0000-000000000002"
	tcvProjectID        = "51111111-1111-1111-1111-111111111111"
	tcvUserID           = "52222222-0000-0000-0000-000000000001"
	tcvViewerEmail      = "jane.doe@example.com"
	tcvSecurityEmail    = "john.roe@example.com"
)

// TestProjectStatsIntegration_TimeCardReportVisibility proves the report applies
// the same visibility rules as every other customer-reachable read. time_card's
// row-level security lets any project member read the project's cards, so a
// customer reaches this query and must not learn that a hidden change request
// or a hidden announcement has logged time.
//
// One project, approved cards on: a normal case, a change request the viewer
// may not see (still New), a change request the viewer may see (Closed, legacy
// rule), an announcement the viewer may not see (security announcement while
// the project has a security contact the viewer is not), and an announcement
// they may see (general).
func TestProjectStatsIntegration_TimeCardReportVisibility(t *testing.T) {
	pool := caseStatsPool(t)
	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM time_card WHERE created_by = 'tcv-test'`)
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE created_by = 'tcv-test'`)
		_, _ = pool.Exec(sys, `DELETE FROM project_contact WHERE created_by = 'tcv-test'`)
		_, _ = pool.Exec(sys, `DELETE FROM project_group WHERE created_by = 'tcv-test'`)
		_, _ = pool.Exec(sys, `DELETE FROM account_contact WHERE id = $1`, tcvAccountContactID)
		_, _ = pool.Exec(sys, `DELETE FROM "user" WHERE user_name = 'tcv-test@example.com'`)
		_, _ = pool.Exec(sys, `DELETE FROM project WHERE id = $1`, tcvProjectID)
		_, _ = pool.Exec(sys, `DELETE FROM account WHERE id = $1`, tcvAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(sys, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number)
	          VALUES ($1, now(), now(), 'tcv-test', 'tcv-test', 'Example Account', 'TCV-ACC')`, tcvAccountID)
	mustExec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
	          VALUES ($1, now(), now(), 'tcv-test', 'tcv-test', 'Jane Doe', $2)`, tcvAccountContactID, tcvAccountID)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
	          VALUES ($1, now(), now(), 'tcv-test', 'tcv-test', 'TCVTEST', 'sf-tcvtest', 'Example Project', $2)`, tcvProjectID, tcvAccountID)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
	          VALUES ($1, now(), now(), 'tcv-test@example.com', 'tcv-test@example.com', true)`, tcvUserID)

	// The viewer is a registered PORTAL_USER; a second registered contact is the
	// project's SECURITY_CONTACT, which is what hides a security announcement
	// from everyone who is not a security contact.
	contact := func(email, role, group string) {
		t.Helper()
		roleID := ensureProjectRole(t, pool, role)
		mustExec(`INSERT INTO project_group (id, created_on, updated_on, created_by, updated_by, "group")
		          VALUES ($1::uuid, now(), now(), 'tcv-test', 'tcv-test', $2)`, "53333333-0000-0000-0000-00000000000"+group[len(group)-1:], group)
		mustExec(`INSERT INTO project_group_role (id, created_on, updated_on, created_by, updated_by, project_group_id, project_role_id)
		          VALUES (gen_random_uuid(), now(), now(), 'tcv-test', 'tcv-test', $1::uuid, $2::uuid)`,
			"53333333-0000-0000-0000-00000000000"+group[len(group)-1:], roleID)
		mustExec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		          VALUES ($1::uuid, now(), now(), 'tcv-test', 'tcv-test', $2, $3, $4, 'REGISTERED')`,
			"54444444-0000-0000-0000-00000000000"+group[len(group)-1:], email, tcvAccountContactID, tcvProjectID)
		mustExec(`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
		          VALUES (gen_random_uuid(), now(), now(), 'tcv-test', 'tcv-test', $1::uuid, $2::uuid)`,
			"54444444-0000-0000-0000-00000000000"+group[len(group)-1:], "53333333-0000-0000-0000-00000000000"+group[len(group)-1:])
	}
	contact(tcvViewerEmail, "PORTAL_USER", "tcv-portal-1")
	contact(tcvSecurityEmail, "SECURITY_CONTACT", "tcv-security-2")

	const (
		wiCase   = "55555555-0000-0000-0000-000000000001"
		wiCRHid  = "55555555-0000-0000-0000-000000000002"
		wiCRVis  = "55555555-0000-0000-0000-000000000003"
		wiAnnHid = "55555555-0000-0000-0000-000000000004"
		wiAnnVis = "55555555-0000-0000-0000-000000000005"
	)
	workItem := func(id, number, typ string) {
		t.Helper()
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		          VALUES ($1::uuid, now(), now(), 'tcv-test', 'tcv-test', $2, $2, 'visibility test', $3::work_item_type_enum, $4)`,
			id, number, typ, tcvProjectID)
	}
	workItem(wiCase, "TCV-CASE", "CASE")
	workItem(wiCRHid, "TCV-CR-HIDDEN", "CHANGE_REQUEST")
	workItem(wiCRVis, "TCV-CR-VISIBLE", "CHANGE_REQUEST")
	workItem(wiAnnHid, "TCV-ANN-HIDDEN", "ANNOUNCEMENT")
	workItem(wiAnnVis, "TCV-ANN-VISIBLE", "ANNOUNCEMENT")
	mustExec(`INSERT INTO change_request (id, state) VALUES ($1::uuid, 'NEW'), ($2::uuid, 'CLOSED')`, wiCRHid, wiCRVis)
	mustExec(`INSERT INTO announcement (id, announcement_type, state) VALUES ($1::uuid, 'SECURITY', 'OPEN'), ($2::uuid, 'GENERAL', 'OPEN')`, wiAnnHid, wiAnnVis)

	for i, c := range []struct {
		wi      string
		minutes int
	}{{wiCase, 10}, {wiCRHid, 20}, {wiCRVis, 40}, {wiAnnHid, 80}, {wiAnnVis, 160}} {
		mustExec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id,
		              work_date, is_billable, state, analyzing_minutes)
		          VALUES ($1::uuid, now(), now(), 'tcv-test', 'tcv-test', $2::uuid, $3::uuid,
		              '2026-01-10', true, 'APPROVED', $4)`,
			"56666666-0000-0000-0000-00000000000"+string(rune('1'+i)), c.wi, tcvUserID, c.minutes)
	}

	repo := repository.NewProjectStatsRepository(scoped)
	numbers := func(caller context.Context) (nums []string, total int) {
		t.Helper()
		got, err := repo.TimeCardReport(caller, tcvProjectID, "2026-01-01", "2026-01-31")
		if err != nil {
			t.Fatalf("TimeCardReport: %v", err)
		}
		if got == nil {
			t.Fatal("TimeCardReport = nil for an existing project")
		}
		for _, c := range got.Cases {
			nums = append(nums, c.CaseNumber)
		}
		return nums, got.Totals.TotalMinutes
	}
	same := func(a, b []string) bool {
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

	// Internal caller: nothing is filtered (10+20+40+80+160).
	all := []string{"TCV-ANN-HIDDEN", "TCV-ANN-VISIBLE", "TCV-CASE", "TCV-CR-HIDDEN", "TCV-CR-VISIBLE"}
	if nums, total := numbers(sys); !same(nums, all) || total != 310 {
		t.Errorf("internal caller: cases %v total %d, want %v total 310", nums, total, all)
	}

	// Registered customer contact: the case, the visible change request and the
	// visible announcement only; the hidden rows are out of the list AND the
	// totals (10+40+160).
	viewer := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: tcvViewerEmail})
	want := []string{"TCV-ANN-VISIBLE", "TCV-CASE", "TCV-CR-VISIBLE"}
	if nums, total := numbers(viewer); !same(nums, want) || total != 210 {
		t.Errorf("customer viewer: cases %v total %d, want %v total 210", nums, total, want)
	}

	// A restricted caller with no identity email gets no change request rows.
	// (Row-level security already empties time_card for such a caller, so this
	// pins the outcome, not which layer produced it.)
	anon := repository.WithCallerIdentity(context.Background(), repository.SearchScope{})
	if nums, total := numbers(anon); len(nums) != 0 || total != 0 {
		t.Errorf("restricted caller without identity: cases %v total %d, want none", nums, total)
	}
}
