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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The project list's Action Required / Outstanding / Active Chats columns:
// GlobalSearchRepository.ProjectActivityCounts against a real schema. Skipped
// without CASE_STATS_TEST_DSN, like the other stats integration tests; run it as
// the non-superuser application role as well as a superuser, since only the
// former has row-level security applied (the isolation case below needs it).
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ProjectActivityCountsIntegration

const (
	plcProjectMain   = "7c100000-0000-0000-0000-000000000001" // the project under test
	plcProjectEmpty  = "7c100000-0000-0000-0000-000000000002" // nothing in it
	plcProjectOther  = "7c100000-0000-0000-0000-000000000003" // another customer's project
	plcAccount       = "7c100000-0000-0000-0000-0000000000a1"
	plcUserDesig     = "7c100000-0000-0000-0000-0000000000b1" // designated on two change requests
	plcUserUndesig   = "7c100000-0000-0000-0000-0000000000b2" // registered, never asked
	plcUserOther     = "7c100000-0000-0000-0000-0000000000b3" // registered on the other project only
	plcContactDesig  = "7c100000-0000-0000-0000-0000000000c1"
	plcContactUndes  = "7c100000-0000-0000-0000-0000000000c2"
	plcContactOther  = "7c100000-0000-0000-0000-0000000000c3"
	plcEmailDesig    = "plc-designated@example.com"
	plcEmailUndesig  = "plc-undesignated@example.com"
	plcEmailOther    = "plc-other@example.com"
	plcEmailPartner  = "plc-partner@example.com" // registered on the main project and plcExtraProjects more
	plcUserPartner   = "7c100000-0000-0000-0000-0000000000b4"
	plcContactPartn  = "7c100000-0000-0000-0000-0000000000c4"
	plcCreatedBy     = "plc-test"
	plcSeedItemFirst = 0x10
	// More than projectActivityChunkSize (10) projects in all, so the partner is served in chunks.
	plcExtraProjects = 11
)

func plcExtraProject(i int) string {
	return "7c100000-0000-0000-0000-" + plcHex(0x2000+i)
}

// The state vocabulary the service hands the repository (service/global_service.go
// takes it from the project stats constants; service tests pin that).
var plcStaffStates = repository.ProjectActivityStates{
	CaseClosed:         []string{"CLOSED"},
	CaseActionRequired: []string{"AWAITING_INFO", "SOLUTION_PROPOSED"},
	CROutstanding:      []string{"CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "ROLLBACK", "REVIEW", "CUSTOMER_REVIEW"},
	CRActionRequired:   []string{"CUSTOMER_APPROVAL", "CUSTOMER_REVIEW"},
	ChatActive:         []string{"OPEN", "ACTIVE"},
}

// A customer's Authorize is outstanding too (crOutstandingStatesFor).
func plcCustomerStates() repository.ProjectActivityStates {
	s := plcStaffStates
	s.CROutstanding = append([]string{"AUTHORIZE"}, plcStaffStates.CROutstanding...)
	return s
}

func plcID(n int) string {
	return "7c100000-0000-0000-0000-" + plcHex(0x1000+n)
}

func plcHex(n int) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 12)
	for i := 11; i >= 0; i-- {
		out[i] = digits[n&0xf]
		n >>= 4
	}
	return string(out)
}

// seedProjectActivity builds the fixture. In the main project:
//
//	cases:            OPEN, AWAITING_INFO, SOLUTION_PROPOSED, CLOSED, and two with no state of
//	                  their own type (a work item whose "case" extension row is missing, and one
//	                  typed CASE whose only extension row is an engagement's: real synced data has
//	                  both, no list filtered by state can show them, so neither is counted)
//	service request:  WORK_IN_PROGRESS
//	announcement:     OPEN                      (never counted)
//	change requests:  NEW, AUTHORIZE, CUSTOMER_APPROVAL, IMPLEMENT, CLOSED
//	conversations:    ACTIVE, OPEN, CLOSE
//
// plcUserDesig is asked (has an approver row on a Customer Approval stage) on
// the CUSTOMER_APPROVAL and the AUTHORIZE change requests; plcUserUndesig is a
// registered contact who was never asked. The other project has one
// AWAITING_INFO case and one ACTIVE conversation.
func seedProjectActivity(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE created_by = $1`, plcCreatedBy)
		_, _ = scoped.Exec(ctx, `DELETE FROM project_contact WHERE created_by = $1`, plcCreatedBy)
		_, _ = scoped.Exec(ctx, `DELETE FROM account_contact WHERE created_by = $1`, plcCreatedBy)
		_, _ = scoped.Exec(ctx, `DELETE FROM project WHERE created_by = $1`, plcCreatedBy)
		_, _ = scoped.Exec(ctx, `DELETE FROM account WHERE created_by = $1`, plcCreatedBy)
		_, _ = scoped.Exec(ctx, `DELETE FROM "user" WHERE user_name LIKE 'plc-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.70s): %v", sql, err)
		}
	}
	now := time.Now().UTC()

	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
	      VALUES ($1, $2, $2, $3, $3, 'PLC Account', 'PLC-ACC', 'PLC-SF-ACC')`, plcAccount, now, plcCreatedBy)
	for _, p := range []struct{ id, key string }{{plcProjectMain, "PLCMAIN"}, {plcProjectEmpty, "PLCEMPTY"}, {plcProjectOther, "PLCOTHER"}} {
		exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
		      VALUES ($1, $2, $2, $3, $3, $4, $5, $6, $7)`, p.id, now, plcCreatedBy, p.key, "SF-"+p.key, "PLC "+p.key, plcAccount)
	}

	// Three customers: who is registered where decides what row-level security
	// and the change request visibility rule let through.
	for _, u := range []struct{ id, email string }{{plcUserDesig, plcEmailDesig}, {plcUserUndesig, plcEmailUndesig}, {plcUserOther, plcEmailOther}} {
		exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
		      VALUES ($1, $2, $2, $3, $4, true)`, u.id, now, "plc-"+u.email, u.email)
	}
	exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
	      VALUES ($1, $2, $2, $3, $4, true)`, plcUserPartner, now, "plc-"+plcEmailPartner, plcEmailPartner)
	for i := 1; i <= plcExtraProjects; i++ {
		exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
		      VALUES ($1, $2, $2, $3, $3, $4, $5, $6, $7)`, plcExtraProject(i), now, plcCreatedBy,
			fmt.Sprintf("PLCEXTRA%d", i), fmt.Sprintf("SF-PLCEXTRA%d", i), fmt.Sprintf("PLC EXTRA %d", i), plcAccount)
	}
	partnerProjects := []string{plcProjectMain}
	for i := 1; i <= plcExtraProjects; i++ {
		partnerProjects = append(partnerProjects, plcExtraProject(i))
	}
	exec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
	      VALUES ($1, $2, $2, $3, $3, $4, $5)`, plcContactPartn, now, plcCreatedBy, "plc-contact-"+plcEmailPartner, plcAccount)
	for _, pr := range partnerProjects {
		exec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		      VALUES (gen_random_uuid(), $1, $1, $2, $2, $3, $4, $5, 'REGISTERED')`, now, plcCreatedBy, plcEmailPartner, plcContactPartn, pr)
	}
	for _, c := range []struct{ contact, email, project string }{
		{plcContactDesig, plcEmailDesig, plcProjectMain},
		{plcContactUndes, plcEmailUndesig, plcProjectMain},
		{plcContactOther, plcEmailOther, plcProjectOther},
	} {
		exec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		      VALUES ($1, $2, $2, $3, $3, $4, $5)`, c.contact, now, plcCreatedBy, "plc-contact-"+c.email, plcAccount)
		exec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		      VALUES (gen_random_uuid(), $1, $1, $2, $2, $3, $4, $5, 'REGISTERED')`, now, plcCreatedBy, c.email, c.contact, c.project)
	}

	next := plcSeedItemFirst
	item := func(typ, project string) string {
		next++
		id := plcID(next)
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		      VALUES ($1, $2, $2, $3, $3, $4, $5, 'plc fixture', $6, $7)`,
			id, now, plcCreatedBy, "PLC-"+id[24:], "PLC-W-"+id[24:], typ, project)
		return id
	}
	caseIn := func(project, state string) {
		id := item("CASE", project)
		exec(`INSERT INTO "case" (id, state) VALUES ($1, $2::text::case_state_enum)`, id, state)
	}
	convIn := func(project, state string) {
		id := item("CONVERSATION", project)
		exec(`INSERT INTO conversation (id, state) VALUES ($1, $2::text::conversation_state_enum)`, id, state)
	}
	crIn := func(project, state string) string {
		id := item("CHANGE_REQUEST", project)
		exec(`INSERT INTO change_request (id, state) VALUES ($1, $2::text::change_request_state_enum)`, id, state)
		return id
	}
	designate := func(crID, userID, approverState string) {
		stage := plcID(next + 0x400)
		next++
		exec(`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, checkpoint_label)
		      VALUES ($1, $2, $2, $3, $3, $4, 'Customer Approval')`, stage, now, plcCreatedBy, crID)
		exec(`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
		      VALUES (gen_random_uuid(), $1, $1, $2, $2, $3, $4, $5, $6::text::approval_stage_approver_state_enum)`,
			now, plcCreatedBy, stage, crID, userID, approverState)
	}

	// Main project.
	caseIn(plcProjectMain, "OPEN")
	caseIn(plcProjectMain, "AWAITING_INFO")
	caseIn(plcProjectMain, "SOLUTION_PROPOSED")
	caseIn(plcProjectMain, "CLOSED")
	item("CASE", plcProjectMain) // no "case" row: no state
	// typed CASE but its only extension row is an engagement's: a state, though not its own type's
	mismatchID := item("CASE", plcProjectMain)
	exec(`INSERT INTO engagement (id, state, type) VALUES ($1, 'OPEN', 'MIGRATION')`, mismatchID)
	srID := item("SERVICE_REQUEST", plcProjectMain)
	exec(`INSERT INTO service_request (id, state) VALUES ($1, 'WORK_IN_PROGRESS')`, srID)
	annID := item("ANNOUNCEMENT", plcProjectMain)
	exec(`INSERT INTO announcement (id, announcement_type, state) VALUES ($1, 'GENERAL', 'OPEN')`, annID)

	crIn(plcProjectMain, "NEW")
	crAuthorize := crIn(plcProjectMain, "AUTHORIZE")
	crCustomerApproval := crIn(plcProjectMain, "CUSTOMER_APPROVAL")
	crIn(plcProjectMain, "IMPLEMENT")
	crIn(plcProjectMain, "CLOSED")
	designate(crCustomerApproval, plcUserDesig, "REQUESTED")
	// A sibling whose row a colleague's answer cancelled is still designated.
	designate(crAuthorize, plcUserDesig, "CANCELLED")

	convIn(plcProjectMain, "ACTIVE")
	convIn(plcProjectMain, "OPEN")
	convIn(plcProjectMain, "CLOSE")

	// The other project.
	caseIn(plcProjectOther, "AWAITING_INFO")
	convIn(plcProjectOther, "ACTIVE")

	// The partner's extra projects: one open case and one active chat each.
	for i := 1; i <= plcExtraProjects; i++ {
		caseIn(plcExtraProject(i), "OPEN")
		convIn(plcExtraProject(i), "ACTIVE")
	}
}

func plcCustomer(email string, projects ...string) repository.SearchScope {
	return repository.SearchScope{ViewerEmail: email, ProjectIDs: projects}
}

func TestProjectActivityCountsIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	seedProjectActivity(t, pool)

	var bypassesRLS bool
	if err := pool.QueryRow(context.Background(),
		`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypassesRLS); err != nil {
		t.Fatalf("role check: %v", err)
	}

	strict := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	legacyRepo := repository.NewGlobalSearchRepository(repository.NewScoped(pool))
	strictRepo := repository.NewGlobalSearchRepository(repository.NewScoped(pool), repository.CRVisibility{StrictFrom: &strict})

	type want struct{ chats, action, outstanding int }
	check := func(t *testing.T, repo repository.GlobalSearchRepository, scope repository.SearchScope, states repository.ProjectActivityStates, projectID string, w want) {
		t.Helper()
		got, err := repo.ProjectActivityCounts(context.Background(), scope, []string{projectID}, states)
		if err != nil {
			t.Fatalf("ProjectActivityCounts: %v", err)
		}
		g := got[projectID]
		if g.ActiveChats != w.chats || g.ActionRequired != w.action || g.Outstanding != w.outstanding {
			t.Errorf("chats/action/outstanding = %d/%d/%d, want %d/%d/%d",
				g.ActiveChats, g.ActionRequired, g.Outstanding, w.chats, w.action, w.outstanding)
		}
	}

	t.Run("staff see what the dashboard counts", func(t *testing.T) {
		// Cases 3 (OPEN, AWAITING_INFO, SOLUTION_PROPOSED: CLOSED is left out, and so are the
		// two items with no state of their own type) + the service request 1 + change requests 2
		// (CUSTOMER_APPROVAL, IMPLEMENT: NEW, AUTHORIZE and CLOSED are not outstanding for staff)
		// = 6; the announcement is not counted. Action required: 2 cases + 1 change request.
		// Chats: ACTIVE, OPEN.
		check(t, legacyRepo, repository.SearchScope{Unrestricted: true}, plcStaffStates, plcProjectMain, want{2, 3, 6})
	})

	t.Run("a project with nothing in it is present at zero", func(t *testing.T) {
		got, err := legacyRepo.ProjectActivityCounts(context.Background(), repository.SearchScope{Unrestricted: true},
			[]string{plcProjectMain, plcProjectEmpty, plcProjectOther}, plcStaffStates)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d projects, want all 3 asked for: %+v", len(got), got)
		}
		if got[plcProjectEmpty] != (repository.ProjectActivityCounts{}) {
			t.Errorf("empty project = %+v, want zeros", got[plcProjectEmpty])
		}
		if o := got[plcProjectOther]; o.ActiveChats != 1 || o.ActionRequired != 1 || o.Outstanding != 1 {
			t.Errorf("other project = %+v, want 1/1/1 and none of the main project's items", o)
		}
	})

	t.Run("the ids global search returns are the keys the counts come back under", func(t *testing.T) {
		// globalService asks for counts by the ids SearchProjects returned, and reads the map by
		// the same ids, so a spelling difference between the two (case, hyphens) would silently
		// leave every row at zero. Use the real search output, not hand-written ids.
		scope := repository.SearchScope{Unrestricted: true}
		projects, _, err := legacyRepo.SearchProjects(context.Background(), scope, "PLC", repository.SearchSortName, false,
			domain.Pagination{Limit: 50})
		if err != nil {
			t.Fatalf("SearchProjects: %v", err)
		}
		if want := 3 + plcExtraProjects; len(projects) != want {
			t.Fatalf("search for the fixture's projects returned %d, want %d", len(projects), want)
		}
		ids := make([]string, len(projects))
		for i, p := range projects {
			ids[i] = p.ID
		}
		got, err := legacyRepo.ProjectActivityCounts(context.Background(), scope, ids, plcStaffStates)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range projects {
			c, ok := got[p.ID]
			if !ok {
				t.Fatalf("no counts under the id search returned for %s: %v", p.Key, got)
			}
			if p.Key == "PLCMAIN" && (c.ActiveChats != 2 || c.ActionRequired != 3 || c.Outstanding != 6) {
				t.Errorf("PLCMAIN counts = %+v, want chats 2 / action 3 / outstanding 6", c)
			}
		}
	})

	t.Run("no projects asked for is no query and no error", func(t *testing.T) {
		got, err := legacyRepo.ProjectActivityCounts(context.Background(), repository.SearchScope{Unrestricted: true}, nil, plcStaffStates)
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("a customer asked for a change request sees it, even in Authorize", func(t *testing.T) {
		// Designated on CUSTOMER_APPROVAL and AUTHORIZE; legacy visibility adds
		// IMPLEMENT (and CLOSED, which is not outstanding). NEW stays hidden.
		// Outstanding: 4 case-like + CUSTOMER_APPROVAL + AUTHORIZE + IMPLEMENT = 7.
		check(t, legacyRepo, plcCustomer(plcEmailDesig, plcProjectMain), plcCustomerStates(), plcProjectMain, want{2, 3, 7})
	})

	t.Run("strict visibility keeps only the designated change requests", func(t *testing.T) {
		// IMPLEMENT is no longer visible: only the two designated ones remain.
		check(t, strictRepo, plcCustomer(plcEmailDesig, plcProjectMain), plcCustomerStates(), plcProjectMain, want{2, 3, 6})
	})

	t.Run("a registered contact who was never asked sees no strict change request", func(t *testing.T) {
		check(t, strictRepo, plcCustomer(plcEmailUndesig, plcProjectMain), plcCustomerStates(), plcProjectMain, want{2, 2, 4})
	})

	t.Run("legacy visibility shows an undesignated contact what customers always saw", func(t *testing.T) {
		// CUSTOMER_APPROVAL and IMPLEMENT are past Authorize; AUTHORIZE and NEW are not.
		check(t, legacyRepo, plcCustomer(plcEmailUndesig, plcProjectMain), plcCustomerStates(), plcProjectMain, want{2, 3, 6})
	})

	// ---- a partner registered on more than 10 projects: served in chunks, each narrowed ----
	partnerProjects := []string{plcProjectMain}
	for i := 1; i <= plcExtraProjects; i++ {
		partnerProjects = append(partnerProjects, plcExtraProject(i))
	}
	// The same caller twice: scope.ProjectIDs only decides whether the work is chunked and narrowed
	// (more than 10 -> yes), the membership that is enforced is the database's either way.
	partnerNarrowed := plcCustomer(plcEmailPartner, partnerProjects...)
	partnerPlain := plcCustomer(plcEmailPartner)

	t.Run("a partner on many projects gets the same numbers narrowed as without narrowing", func(t *testing.T) {
		ids := append(append([]string{}, partnerProjects...), plcProjectOther) // one of them is not theirs
		narrowed, err := legacyRepo.ProjectActivityCounts(context.Background(), partnerNarrowed, ids, plcCustomerStates())
		if err != nil {
			t.Fatalf("narrowed: %v", err)
		}
		plain, err := legacyRepo.ProjectActivityCounts(context.Background(), partnerPlain, ids, plcCustomerStates())
		if err != nil {
			t.Fatalf("plain: %v", err)
		}
		if len(narrowed) != len(ids) || len(plain) != len(ids) {
			t.Fatalf("every asked id must be present: narrowed %d, plain %d, asked %d", len(narrowed), len(plain), len(ids))
		}
		for _, id := range ids {
			if narrowed[id] != plain[id] {
				t.Errorf("project %s: narrowed %+v, plain %+v", id, narrowed[id], plain[id])
			}
		}
		// The 11 extra projects, across the chunk boundary (10 + 2): one open case, one active chat each.
		for i := 1; i <= plcExtraProjects; i++ {
			if got := narrowed[plcExtraProject(i)]; got.Outstanding != 1 || got.ActiveChats != 1 || got.ActionRequired != 0 {
				t.Errorf("extra project %d = %+v, want outstanding 1 / chats 1 / action 0", i, got)
			}
		}
		// The partner is a registered contact of the main project too (never designated): what an
		// undesignated contact sees under legacy visibility.
		if got := narrowed[plcProjectMain]; got.ActiveChats != 2 || got.ActionRequired != 3 || got.Outstanding != 6 {
			t.Errorf("main project = %+v, want chats 2 / action 3 / outstanding 6", got)
		}
	})

	t.Run("narrowing never widens: a project the partner is not a member of stays at zero", func(t *testing.T) {
		if bypassesRLS {
			t.Skip("connected as a role that bypasses row-level security")
		}
		got, err := legacyRepo.ProjectActivityCounts(context.Background(), partnerNarrowed,
			[]string{plcProjectOther}, plcCustomerStates())
		if err != nil {
			t.Fatal(err)
		}
		if got[plcProjectOther] != (repository.ProjectActivityCounts{}) {
			t.Fatalf("a project outside the partner's membership = %+v, want zeros", got[plcProjectOther])
		}
		// Asking for it among member projects, in a chunk with members, must not let it in either.
		mixed := append([]string{plcProjectOther}, partnerProjects[:5]...)
		got, err = legacyRepo.ProjectActivityCounts(context.Background(), partnerNarrowed, mixed, plcCustomerStates())
		if err != nil {
			t.Fatal(err)
		}
		if got[plcProjectOther] != (repository.ProjectActivityCounts{}) {
			t.Fatalf("outside project in a chunk with members = %+v, want zeros", got[plcProjectOther])
		}
		if got[plcExtraProject(1)].Outstanding != 1 {
			t.Fatalf("a member project in the same chunk = %+v, want its own count", got[plcExtraProject(1)])
		}
	})

	t.Run("the narrowed project list does not outlive the call on a reused connection", func(t *testing.T) {
		if bypassesRLS {
			t.Skip("connected as a role that bypasses row-level security")
		}
		// One connection, so the statement after the narrowed call runs on the very connection
		// it used. If the narrowing leaked, the partner would see only the last chunk's projects.
		cfg, err := pgxpool.ParseConfig(os.Getenv("CASE_STATS_TEST_DSN"))
		if err != nil {
			t.Fatal(err)
		}
		cfg.MaxConns = 1
		one, err := pgxpool.NewWithConfig(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer one.Close()
		oneScoped := repository.NewScoped(one)
		oneRepo := repository.NewGlobalSearchRepository(oneScoped)

		if _, err := oneRepo.ProjectActivityCounts(context.Background(), partnerNarrowed, partnerProjects, plcCustomerStates()); err != nil {
			t.Fatal(err)
		}
		// The setting itself, read raw on that one connection: transaction-local, so it is back to
		// empty once the call is over. (Scoped sets it afresh for every statement, which would
		// hide a leak from the visibility check below, so this is the check that has teeth.)
		var left string
		if err := one.QueryRow(context.Background(), `SELECT COALESCE(current_setting('app.viewer_project_ids', true), '')`).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if left != "" {
			t.Fatalf("app.viewer_project_ids = %q after the call, want it reverted to empty: the narrowing outlived its transaction", left)
		}
		ctx := repository.WithCallerIdentity(context.Background(), partnerNarrowed)
		var visible int
		if err := oneScoped.QueryRow(ctx, `SELECT COUNT(DISTINCT project_id) FROM work_item WHERE project_id = ANY($1::text[]::uuid[])`,
			partnerProjects).Scan(&visible); err != nil {
			t.Fatal(err)
		}
		if visible != len(partnerProjects) {
			t.Fatalf("after a narrowed call the partner sees %d of their %d projects on the same connection: the narrowing leaked", visible, len(partnerProjects))
		}
	})

	t.Run("a customer of another project sees nothing of this one", func(t *testing.T) {
		if bypassesRLS {
			t.Skip("connected as a role that bypasses row-level security; the case and chat counts rely on it here")
		}
		check(t, strictRepo, plcCustomer(plcEmailOther, plcProjectOther), plcCustomerStates(), plcProjectMain, want{0, 0, 0})
		check(t, legacyRepo, plcCustomer(plcEmailOther, plcProjectOther), plcCustomerStates(), plcProjectMain, want{0, 0, 0})
	})
}
