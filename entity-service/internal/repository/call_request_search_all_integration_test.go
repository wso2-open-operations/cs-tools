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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Runs SearchAllCallRequests against a real Postgres, because what matters here
// is the SQL the dashboards' "My Call Requests" and "Calls To Attend" widgets
// depend on (digiops-cs#3314): "assigned to" is the parent case's assignee, and a
// team is the parent case's account CRE team. Two callers are exercised: internal
// staff (what the dashboards are), and a customer scoped to one project, which
// shows the new predicates only ever narrow what row-level security already
// allows. Skipped without CALL_REQUEST_TEST_DSN so an ordinary `go test ./...`
// stays hermetic. Use a scratch database and a non-superuser, non-BYPASSRLS
// login: callRequestRLSPrecondition fails the test when the login can see the
// fixtures without any identity, since a customer test would then prove nothing.
//
//	CALL_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run CallRequestSearchAll
func callRequestTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CALL_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CALL_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

const (
	crOwnerA   = "ca000000-0000-0000-0000-000000000001" // owns caseX1, caseXClosed
	crOwnerB   = "ca000000-0000-0000-0000-000000000002" // owns caseY1
	crTeamX    = "ca000000-0000-0000-0000-000000000011"
	crTeamY    = "ca000000-0000-0000-0000-000000000012"
	crAccX     = "ca000000-0000-0000-0000-000000000021" // CRE team X
	crAccY     = "ca000000-0000-0000-0000-000000000022" // CRE team Y
	crAccNone  = "ca000000-0000-0000-0000-000000000023" // no CRE team
	crProjX    = "ca000000-0000-0000-0000-000000000031"
	crProjY    = "ca000000-0000-0000-0000-000000000032"
	crProjNone = "ca000000-0000-0000-0000-000000000033"

	// A customer registered on project X only (see crCustomerEmail).
	crCustomerContact = "ca000000-0000-0000-0000-000000000061"
	crCustomerEmail   = "cr-customer@test.local"

	crCaseX1       = "ca000000-0000-0000-0000-000000000041" // team X, owner A, open
	crCaseXClosed  = "ca000000-0000-0000-0000-000000000042" // team X, owner A, closed
	crCaseXNoOwner = "ca000000-0000-0000-0000-000000000043" // team X, no owner
	crCaseY1       = "ca000000-0000-0000-0000-000000000044" // team Y, owner B
	crCaseNoTeam   = "ca000000-0000-0000-0000-000000000045" // no team, owner A

	// A pending call with no assignee: the exact digiops-cs#3314 shape.
	crCallPendingX     = "ca000000-0000-0000-0000-000000000051"
	crCallPendingClosd = "ca000000-0000-0000-0000-000000000052"
	crCallPendingNoOwn = "ca000000-0000-0000-0000-000000000053"
	crCallPendingY     = "ca000000-0000-0000-0000-000000000054"
	// Scheduled on owner B's case but handed to owner A at call level.
	crCallHandedToA  = "ca000000-0000-0000-0000-000000000055"
	crCallPendingNoT = "ca000000-0000-0000-0000-000000000056"
	// Detached: no parent work item at all, but its own assignee is owner A. Only
	// internal staff can see such a row (migration 0143), and it can never belong to a team.
	crCallDetached = "ca000000-0000-0000-0000-000000000057"
)

var crCallIDs = []string{crCallPendingX, crCallPendingClosd, crCallPendingNoOwn, crCallPendingY, crCallHandedToA, crCallPendingNoT, crCallDetached}

func seedCallRequestSearchFixtures(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		// customer_call has no DELETE policy (migration 0143), so under row-level security a
		// row only goes away through its work item's ON DELETE CASCADE. The detached fixture
		// has no work item, so attach it to a fixture case first or it would outlive the test.
		_, _ = scoped.Exec(ctx, `UPDATE customer_call SET work_item_id = $2 WHERE id = $1`, crCallDetached, crCaseX1)
		_, _ = scoped.Exec(ctx, `DELETE FROM customer_call WHERE id = ANY($1::text[]::uuid[])`, crCallIDs)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE project_id = ANY($1::text[]::uuid[])`, []string{crProjX, crProjY, crProjNone})
		_, _ = scoped.Exec(ctx, `DELETE FROM project_contact WHERE project_id = ANY($1::text[]::uuid[])`, []string{crProjX, crProjY, crProjNone})
		_, _ = scoped.Exec(ctx, `DELETE FROM account_contact WHERE id = $1`, crCustomerContact)
		_, _ = scoped.Exec(ctx, `DELETE FROM project WHERE id = ANY($1::text[]::uuid[])`, []string{crProjX, crProjY, crProjNone})
		_, _ = scoped.Exec(ctx, `DELETE FROM account WHERE id = ANY($1::text[]::uuid[])`, []string{crAccX, crAccY, crAccNone})
		_, _ = scoped.Exec(ctx, `DELETE FROM "group" WHERE id = ANY($1::text[]::uuid[])`, []string{crTeamX, crTeamY})
		_, _ = scoped.Exec(ctx, `DELETE FROM "user" WHERE id = ANY($1::text[]::uuid[])`, []string{crOwnerA, crOwnerB})
	}
	cleanup()
	t.Cleanup(cleanup)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %s: %v", sql, err)
		}
	}
	now := time.Now().UTC()

	for id, name := range map[string]string{crOwnerA: "cr-owner-a", crOwnerB: "cr-owner-b"} {
		exec(`INSERT INTO "user" (id, created_on, updated_on, user_name) VALUES ($1, $2, $2, $3)`, id, now, name)
	}
	for id, name := range map[string]string{crTeamX: "CR Test Team X", crTeamY: "CR Test Team Y"} {
		exec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, $2, $2, 'test', 'test', $3)`, id, now, name)
	}
	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, cre_team_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CR Acc X', 'CR-ACC-X', 'CR-SF-ACC-X', $3)`, crAccX, now, crTeamX)
	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, cre_team_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CR Acc Y', 'CR-ACC-Y', 'CR-SF-ACC-Y', $3)`, crAccY, now, crTeamY)
	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CR Acc None', 'CR-ACC-N', 'CR-SF-ACC-N')`, crAccNone, now)
	for _, p := range []struct{ id, key, acc string }{{crProjX, "CRPROJX", crAccX}, {crProjY, "CRPROJY", crAccY}, {crProjNone, "CRPROJN", crAccNone}} {
		exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, $5, $6)`, p.id, now, p.key, "SF-"+p.key, "CR Test "+p.key, p.acc)
	}

	// The customer: a REGISTERED membership of project X is what the database turns into
	// app.viewer_project_ids for that email (setViewerProjectIDsSQL), and so into what
	// is_project_member lets through. Nothing registers them on projects Y or none.
	exec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'cr-customer', $3)`, crCustomerContact, now, crAccX)
	exec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3, $4, 'REGISTERED')`, now, crCustomerEmail, crCustomerContact, crProjX)

	// A case: work_item (project, account, assignee) plus its "case" extension row.
	mkCase := func(id, num, proj, acc, owner, state string) {
		var ownerArg any
		if owner != "" {
			ownerArg = owner
		}
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id, account_id, assigned_to_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, $5, 'CASE', $6, $7, $8)`,
			id, now, num, "WSO2-"+num, "call request fixture "+num, proj, acc, ownerArg)
		exec(`INSERT INTO "case" (id, state) VALUES ($1, $2::text::case_state_enum)`, id, state)
	}
	mkCase(crCaseX1, "CR-CASE-X1", crProjX, crAccX, crOwnerA, "OPEN")
	mkCase(crCaseXClosed, "CR-CASE-XC", crProjX, crAccX, crOwnerA, "CLOSED")
	mkCase(crCaseXNoOwner, "CR-CASE-XN", crProjX, crAccX, "", "OPEN")
	mkCase(crCaseY1, "CR-CASE-Y1", crProjY, crAccY, crOwnerB, "OPEN")
	mkCase(crCaseNoTeam, "CR-CASE-NT", crProjNone, crAccNone, crOwnerA, "OPEN")

	mkCall := func(id, caseID, state, callAssignee string) {
		var assigneeArg, caseArg any
		if callAssignee != "" {
			assigneeArg = callAssignee
		}
		if caseID != "" {
			caseArg = caseID
		}
		exec(`INSERT INTO customer_call (id, created_on, updated_on, created_by, updated_by, work_item_id, assigned_to_id, state, reason)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, $5::text::customer_call_state_enum, 'fixture')`,
			id, now, caseArg, assigneeArg, state)
	}
	mkCall(crCallPendingX, crCaseX1, "PENDING_ON_WSO2", "")
	mkCall(crCallPendingClosd, crCaseXClosed, "PENDING_ON_WSO2", "")
	mkCall(crCallPendingNoOwn, crCaseXNoOwner, "PENDING_ON_WSO2", "")
	mkCall(crCallPendingY, crCaseY1, "PENDING_ON_WSO2", "")
	mkCall(crCallHandedToA, crCaseY1, "SCHEDULED", crOwnerA)
	mkCall(crCallPendingNoT, crCaseNoTeam, "PENDING_ON_WSO2", "")
	mkCall(crCallDetached, "", "PENDING_ON_WSO2", crOwnerA)
}

// callRequestRLSPrecondition fails when the DSN's login can read the fixtures with
// no identity at all: that means it is a superuser, BYPASSRLS or the table owner,
// and any customer-scoped expectation below would be vacuously true.
func callRequestRLSPrecondition(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM customer_call WHERE id = ANY($1::text[]::uuid[])`, crCallIDs).Scan(&n); err != nil {
		t.Fatalf("precondition query: %v", err)
	}
	if n != 0 {
		t.Fatalf("a raw pool with no identity sees %d of the fixture call requests: the DSN's role bypasses "+
			"row-level security (owner, superuser or BYPASSRLS), so this test would prove nothing", n)
	}
}

// TestCallRequestSearchAllIntegration pins what each filter means. Every case is
// scoped to the fixtures' own unique users/teams, so the staging-like rows
// already in a copied database cannot leak into the expectations.
func TestCallRequestSearchAllIntegration(t *testing.T) {
	pool := callRequestTestPool(t)
	seedCallRequestSearchFixtures(t, pool)
	repo := repository.NewCallRequestRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	type F = domain.SearchAllCallRequestsFilters
	pending := []domain.CallRequestStateType{domain.CallRequestStatePendingOnWSO2, domain.CallRequestStateNotesPending}
	closed := []domain.CaseState{domain.CaseStateClosed}

	cases := []struct {
		name string
		f    F
		want []string
	}{
		{
			// The digiops-cs#3314 dashboard query for "My Call Requests": the widget sends the
			// signed-in engineer's id. Both pending calls on owner A's open case carry
			// no assignee of their own, which is why this used to return nothing.
			name: "my call requests: the engineer's open cases, pending states",
			f:    F{AssignedUserIDs: []string{crOwnerA}, ExcludeCaseStates: closed, States: pending},
			want: []string{crCallPendingX, crCallPendingNoT, crCallDetached},
		},
		{
			// Matches the case's owner, OR a call handed to A at call level.
			name: "assigned user matches the case owner or the call's own assignee",
			f:    F{AssignedUserIDs: []string{crOwnerA}},
			want: []string{crCallPendingX, crCallPendingClosd, crCallPendingNoT, crCallHandedToA, crCallDetached},
		},
		{
			name: "another engineer sees the calls on their case, scheduled included",
			f:    F{AssignedUserIDs: []string{crOwnerB}},
			want: []string{crCallPendingY, crCallHandedToA},
		},
		{
			// The digiops-cs#3314 dashboard query for "Calls To Attend": the team selector's CRE group.
			name: "calls to attend: team X, open cases, pending states",
			f:    F{AssignmentTeamIDs: []string{crTeamX}, ExcludeCaseStates: closed, States: pending},
			want: []string{crCallPendingX, crCallPendingNoOwn},
		},
		{
			name: "team Y",
			f:    F{AssignmentTeamIDs: []string{crTeamY}},
			want: []string{crCallPendingY, crCallHandedToA},
		},
		{
			name: "several teams are OR'd",
			f:    F{AssignmentTeamIDs: []string{crTeamX, crTeamY}, ExcludeCaseStates: closed},
			want: []string{crCallPendingX, crCallPendingNoOwn, crCallPendingY, crCallHandedToA},
		},
		{
			// Neither the no-team case's call nor the detached one (no case, so no account,
			// so no team) can match a team filter, even though both are owner A's.
			name: "calls with no CRE team, or no parent case, never match a team filter",
			f:    F{AssignmentTeamIDs: []string{crTeamX, crTeamY}, AssignedUserIDs: []string{crOwnerA}},
			want: []string{crCallPendingX, crCallPendingClosd, crCallHandedToA},
		},
		{
			name: "a team nobody belongs to matches nothing",
			f:    F{AssignmentTeamIDs: []string{"ca000000-0000-0000-0000-0000000000ff"}},
			want: nil,
		},
		{
			name: "team and assignee are ANDed",
			f:    F{AssignmentTeamIDs: []string{crTeamY}, AssignedUserIDs: []string{crOwnerA}},
			want: []string{crCallHandedToA},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := domain.Pagination{Limit: 50}
			rows, total, err := repo.SearchAllCallRequests(ctx, tc.f, domain.CallRequestSort{}, page)
			if err != nil {
				t.Fatalf("SearchAllCallRequests: %v", err)
			}
			got := make([]string, 0, len(rows))
			for _, r := range rows {
				got = append(got, r.ID)
			}
			want := append([]string(nil), tc.want...)
			sort.Strings(got)
			sort.Strings(want)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("rows = %v\nwant   %v", got, want)
			}
			// The count query shares the WHERE; a drift here would make a paged
			// widget show "N" with a different number of rows.
			if total != len(want) {
				t.Errorf("total = %d, want %d", total, len(want))
			}
		})
	}
}

// TestCallRequestSearchAllPaginationIntegration: the filter is applied to the
// count and the page alike, so paging through a filtered result agrees with it.
func TestCallRequestSearchAllPaginationIntegration(t *testing.T) {
	pool := callRequestTestPool(t)
	seedCallRequestSearchFixtures(t, pool)
	repo := repository.NewCallRequestRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	f := domain.SearchAllCallRequestsFilters{AssignmentTeamIDs: []string{crTeamX, crTeamY}}
	seen := map[string]bool{}
	var total int
	for offset := 0; offset < 10; offset += 2 {
		rows, n, err := repo.SearchAllCallRequests(ctx, f, domain.CallRequestSort{Field: domain.CallRequestSortFieldCreatedOn, Order: domain.CallRequestSortOrderAsc},
			domain.Pagination{Limit: 2, Offset: offset})
		if err != nil {
			t.Fatalf("offset %d: %v", offset, err)
		}
		total = n
		for _, r := range rows {
			if seen[r.ID] {
				t.Errorf("call %s returned on two pages", r.ID)
			}
			seen[r.ID] = true
		}
	}
	// team X: pending X, closed-case pending, no-owner pending; team Y: pending Y, handed-to-A.
	if total != 5 || len(seen) != 5 {
		t.Errorf("total = %d, distinct rows across pages = %d, want 5 and 5", total, len(seen))
	}
}

// TestCallRequestSearchAllCustomerScopeIntegration runs the same filters as a
// customer who is a member of one project only. Row-level security decides what
// that caller may see; the new assignee and team predicates are ANDed on top, so
// they must never reach a call on a project the customer is not a member of,
// whatever team or assignee the request names.
func TestCallRequestSearchAllCustomerScopeIntegration(t *testing.T) {
	pool := callRequestTestPool(t)
	seedCallRequestSearchFixtures(t, pool)
	callRequestRLSPrecondition(t, pool)
	repo := repository.NewCallRequestRepository(repository.NewScoped(pool))
	ctx := repository.WithCallerIdentity(context.Background(),
		repository.SearchScope{ViewerEmail: crCustomerEmail})

	type F = domain.SearchAllCallRequestsFilters
	closed := []domain.CaseState{domain.CaseStateClosed}
	cases := []struct {
		name string
		f    F
		want []string
	}{
		{
			// Both owners across both teams: still only the member project's calls.
			name: "both assignees across both teams: only the member project's calls",
			f:    F{AssignedUserIDs: []string{crOwnerA, crOwnerB}, AssignmentTeamIDs: []string{crTeamX, crTeamY}},
			want: []string{crCallPendingX, crCallPendingClosd},
		},
		{
			name: "team X: the member project's calls on that team",
			f:    F{AssignmentTeamIDs: []string{crTeamX}},
			want: []string{crCallPendingX, crCallPendingClosd, crCallPendingNoOwn},
		},
		{
			name: "team Y: another project's team is not a way in",
			f:    F{AssignmentTeamIDs: []string{crTeamY}},
			want: nil,
		},
		{
			// Owner A also owns a case on another project, was handed a call on a third,
			// and has a detached call: none of those are visible to this customer.
			name: "assignee A: the other projects' and the detached call stay hidden",
			f:    F{AssignedUserIDs: []string{crOwnerA}},
			want: []string{crCallPendingX, crCallPendingClosd},
		},
		{
			name: "assignee A, open cases only",
			f:    F{AssignedUserIDs: []string{crOwnerA}, ExcludeCaseStates: closed},
			want: []string{crCallPendingX},
		},
		{
			name: "assignee B (owns a case on a project the customer is not in)",
			f:    F{AssignedUserIDs: []string{crOwnerB}},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := repo.SearchAllCallRequests(ctx, tc.f, domain.CallRequestSort{}, domain.Pagination{Limit: 50})
			if err != nil {
				t.Fatalf("SearchAllCallRequests: %v", err)
			}
			got := make([]string, 0, len(rows))
			for _, r := range rows {
				got = append(got, r.ID)
			}
			want := append([]string(nil), tc.want...)
			sort.Strings(got)
			sort.Strings(want)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("rows = %v\nwant   %v", got, want)
			}
			if total != len(want) {
				t.Errorf("total = %d, want %d", total, len(want))
			}
		})
	}
}
