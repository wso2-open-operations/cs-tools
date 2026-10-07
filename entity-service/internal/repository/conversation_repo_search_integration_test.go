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

// Regression tests for ConversationRepository.SearchConversations after its
// COUNT and page queries stopped running the project / linked-case / creator
// joins for every matching row (the page is now picked first and the display
// joins are applied to that page only).
//
// They prove, against a real Postgres with row-level security forced, that:
//
//   - the new queries return exactly what the previous ones did (same ids, same
//     order, same total, same project / state / creator per row) for an internal
//     caller, a registered project member and a stranger, across every filter
//     and both sorts, page by page;
//   - the result also matches a model computed in Go from the fixture alone, so
//     "same as before" cannot hide a shared bug;
//   - a project member sees only their own project's conversations and a
//     stranger sees none (the policies, not the SQL shape, decide visibility);
//   - two "user" rows sharing an email address no longer fan one conversation
//     out into two rows or inflate the total (the previous queries did both).
//
// The previous SQL is replayed here verbatim as the oracle. Skipped without
// CASE_STATS_TEST_DSN, and it fails loudly if the DSN's role bypasses RLS,
// since the member / stranger assertions would otherwise pass vacuously.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ConversationSearchIntegration

package repository_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	csAccountID = "7c000000-0000-4000-8000-000000000001"
	csContactID = "7c000000-0000-4000-8000-000000000002"
	csProjectA  = "7c000000-0000-4000-8000-000000000011"
	csProjectB  = "7c000000-0000-4000-8000-000000000012"
	csMember    = "cs-member@test.local"
	csStranger  = "cs-stranger@test.local"
	csCreator   = "cs-creator@test.local"
	csDupEmail  = "cs-dup@test.local"
	csNobody    = "cs-nobody@test.local"

	csUserCreator = "7c000000-0000-4000-8000-0000000000a1"
	csUserDupLow  = "7c000000-0000-4000-8000-0000000000a2"
	csUserDupHigh = "7c000000-0000-4000-8000-0000000000a3"
)

type csConv struct {
	id, number, subject, project string
	state                        domain.ConversationState
	createdBy                    string
	createdMin, updatedMin       int // minutes after csBase
}

var csBase = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// csConversations: 01 and 02 share created_on so the wi.id tie-break is
// exercised; 04's creator email matches two "user" rows when the duplicate
// users are seeded; 06's creator matches no user at all.
var csConversations = []csConv{
	{"7c000000-0000-4000-8000-000000000101", "CS-CHAT-1", "API gateway returns 502", csProjectA, domain.ConversationStateActive, csCreator, 10, 5},
	{"7c000000-0000-4000-8000-000000000102", "CS-CHAT-2", "Upgrade 100%_done question", csProjectA, domain.ConversationStateResolved, csCreator, 10, 9},
	{"7c000000-0000-4000-8000-000000000103", "CS-CHAT-3", "Database timeout in prod", csProjectB, domain.ConversationStateActive, csCreator, 20, 1},
	{"7c000000-0000-4000-8000-000000000104", "CS-CHAT-4", "Dup creator chat", csProjectA, domain.ConversationStateActive, csDupEmail, 30, 2},
	{"7c000000-0000-4000-8000-000000000105", "CS-CHAT-5", "Closed chat about API", csProjectA, domain.ConversationStateClosed, csCreator, 40, 3},
	{"7c000000-0000-4000-8000-000000000106", "CS-CHAT-6", "Orphan creator", csProjectA, domain.ConversationStateActive, csNobody, 50, 4},
	{"7c000000-0000-4000-8000-000000000107", "CS-CHAT-7", "Another B chat api", csProjectB, domain.ConversationStateAbandoned, csCreator, 60, 6},
}

func csIDs() []string {
	out := make([]string, len(csConversations))
	for i, c := range csConversations {
		out[i] = c.id
	}
	return out
}

// seedConversationSearchFixture builds two projects (the member is registered
// on A only), the creator users and the conversations above. withDupUsers adds a
// second "user" row sharing csDupEmail's address.
func seedConversationSearchFixture(t *testing.T, pool *pgxpool.Pool, withDupUsers bool) {
	t.Helper()
	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM conversation WHERE id = ANY($1::uuid[])`, csIDs())
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE id = ANY($1::uuid[])`, csIDs())
		_, _ = pool.Exec(sys, `DELETE FROM project_contact WHERE project_id IN ($1, $2)`, csProjectA, csProjectB)
		_, _ = pool.Exec(sys, `DELETE FROM project WHERE id IN ($1, $2)`, csProjectA, csProjectB)
		_, _ = pool.Exec(sys, `DELETE FROM account_contact WHERE id = $1`, csContactID)
		_, _ = pool.Exec(sys, `DELETE FROM account WHERE id = $1`, csAccountID)
		_, _ = pool.Exec(sys, `DELETE FROM "user" WHERE id IN ($1, $2, $3)`, csUserCreator, csUserDupLow, csUserDupHigh)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(sys, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	now := time.Now().UTC()

	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CS Search Account', 'CS-ACC-1', 'sf-cs-acc-1')`, csAccountID, now)
	mustExec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CS Member', $3)`, csContactID, now, csAccountID)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, account_id)
		VALUES ($1, $3, $3, 'test', 'test', 'CSSA', 'sf-cssa', $2), ($4, $3, $3, 'test', 'test', 'CSSB', 'sf-cssb', $2)`,
		csProjectA, csAccountID, now, csProjectB)
	mustExec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3, $4, 'REGISTERED')`, now, csMember, csContactID, csProjectA)

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, name, is_active)
		VALUES ($1, $3, $3, 'cs-creator', $2, 'CS Creator', TRUE)`, csUserCreator, csCreator, now)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, name, is_active)
		VALUES ($1, $3, $3, 'cs-dup-1', $2, 'CS Dup One', TRUE)`, csUserDupLow, csDupEmail, now)
	if withDupUsers {
		// Same address, different case: LOWER() matches both. Its id sorts after
		// csUserDupLow, which is the one the page query must pick.
		mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, name, is_active)
			VALUES ($1, $3, $3, 'cs-dup-2', $2, 'CS Dup Two', TRUE)`, csUserDupHigh, strings.ToUpper(csDupEmail), now)
	}

	repo := repository.NewConversationRepository(scoped)
	for _, c := range csConversations {
		if _, err := repo.CreateConversation(sys, repository.CreateConversationInput{
			ID: c.id, Number: c.number, ProjectID: c.project, Subject: c.subject,
			InitialMessage: c.subject, CreatedBy: c.createdBy, State: c.state,
		}); err != nil {
			t.Fatalf("seed conversation %s: %v", c.number, err)
		}
		if _, err := scoped.Exec(sys, `UPDATE work_item SET created_on = $2, updated_on = $3 WHERE id = $1`,
			c.id, csBase.Add(time.Duration(c.createdMin)*time.Minute), csBase.Add(time.Duration(c.updatedMin)*time.Minute)); err != nil {
			t.Fatalf("seed timestamps %s: %v", c.number, err)
		}
	}

	// The member/stranger assertions mean nothing if the role bypasses RLS.
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM work_item WHERE project_id = $1`, csProjectA).Scan(&n); err != nil {
		t.Fatalf("precondition query: %v", err)
	}
	if n != 0 {
		t.Fatalf("a raw pool with no identity sees %d work_item rows of the fixture's project: the DSN's role bypasses "+
			"row-level security (owner, superuser or BYPASSRLS), so this test would prove nothing", n)
	}
}

// csOldFrom is the FROM clause the search used before this change, kept here
// verbatim as the oracle.
const csOldFrom = `
	FROM work_item wi
	JOIN conversation c ON c.id = wi.id
	LEFT JOIN project p ON p.id = wi.project_id
	LEFT JOIN work_item case_wi ON case_wi.id = wi.parent_id
	LEFT JOIN "user" u ON LOWER(u.email) = LOWER(wi.created_by)`

type csRow struct {
	id      string
	project string
	state   string
	userID  string
}

// csOldSearch replays the pre-change COUNT and page queries.
func csOldSearch(t *testing.T, scoped *repository.Scoped, ctx context.Context, f domain.SearchConversationsFilters, sortCol, sortDir string, limit, offset int) ([]csRow, int) {
	t.Helper()
	where, args := repository.ConversationWhereClauseForTest(f, "")

	var total int
	if err := scoped.QueryRow(ctx, "SELECT COUNT(*) "+csOldFrom+" "+where, args...).Scan(&total); err != nil {
		t.Fatalf("old count: %v", err)
	}
	dataArgs := append(append([]any{}, args...), limit, offset)
	rows, err := scoped.Query(ctx, fmt.Sprintf(
		`SELECT wi.id, COALESCE(p.id::text, ''), COALESCE(c.state::text, ''), COALESCE(u.id::text, '')
		 %s %s ORDER BY %s %s, wi.id LIMIT $%d OFFSET $%d`,
		csOldFrom, where, sortCol, sortDir, len(args)+1, len(args)+2), dataArgs...)
	if err != nil {
		t.Fatalf("old page: %v", err)
	}
	defer rows.Close()
	var out []csRow
	for rows.Next() {
		var r csRow
		if err := rows.Scan(&r.id, &r.project, &r.state, &r.userID); err != nil {
			t.Fatalf("old scan: %v", err)
		}
		if r.state == "CLOSE" { // conversation_state_enum's label; the API says CLOSED
			r.state = string(domain.ConversationStateClosed)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("old rows: %v", err)
	}
	return out, total
}

func csViewRow(v domain.SearchConversationView) csRow {
	r := csRow{id: *v.ID}
	if v.Project != nil {
		r.project = v.Project.ID
	}
	if v.State != nil {
		r.state = *v.State
	}
	if v.CreatedBy != nil && v.CreatedBy.ID != nil {
		r.userID = *v.CreatedBy.ID
	}
	return r
}

// csModel computes the expected ids from the fixture alone (no SQL): the
// conversations the identity may see, matching the filters, in sort order.
func csModel(visible func(csConv) bool, f domain.SearchConversationsFilters, byUpdated, asc bool) []string {
	var matched []csConv
	for _, c := range csConversations {
		if !visible(c) {
			continue
		}
		if len(f.ProjectIDs) > 0 && !csContains(f.ProjectIDs, c.project) {
			continue
		}
		if len(f.States) > 0 && !csContainsState(f.States, c.state) {
			continue
		}
		if f.Number != nil && *f.Number != c.number {
			continue
		}
		if f.SearchQuery != "" {
			q := strings.ToLower(f.SearchQuery)
			if !strings.Contains(strings.ToLower(c.subject), q) && !strings.Contains(strings.ToLower(c.number), q) {
				continue
			}
		}
		if len(f.CreatedBy) > 0 {
			ok := false
			for _, e := range f.CreatedBy {
				ok = ok || strings.EqualFold(e, c.createdBy)
			}
			if !ok {
				continue
			}
		}
		matched = append(matched, c)
	}
	key := func(c csConv) int {
		if byUpdated {
			return c.updatedMin
		}
		return c.createdMin
	}
	sort.SliceStable(matched, func(i, j int) bool {
		ki, kj := key(matched[i]), key(matched[j])
		if ki != kj {
			if asc {
				return ki < kj
			}
			return ki > kj
		}
		return matched[i].id < matched[j].id
	})
	out := make([]string, len(matched))
	for i, c := range matched {
		out[i] = c.id
	}
	return out
}

func csContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func csContainsState(list []domain.ConversationState, s domain.ConversationState) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func csStrPtr(s string) *string { return &s }

type csIdentity struct {
	name    string
	scope   repository.SearchScope
	visible func(csConv) bool
}

func csIdentities() []csIdentity {
	return []csIdentity{
		{"internal", repository.SearchScope{Unrestricted: true}, func(csConv) bool { return true }},
		{"member of A", repository.SearchScope{ViewerEmail: csMember}, func(c csConv) bool { return c.project == csProjectA }},
		{"stranger", repository.SearchScope{ViewerEmail: csStranger}, func(csConv) bool { return false }},
	}
}

func csFilters() []struct {
	name string
	f    domain.SearchConversationsFilters
} {
	return []struct {
		name string
		f    domain.SearchConversationsFilters
	}{
		{"no filter", domain.SearchConversationsFilters{}},
		{"free text api", domain.SearchConversationsFilters{SearchQuery: "api"}},
		{"free text with LIKE metacharacters", domain.SearchConversationsFilters{SearchQuery: "100%_"}},
		{"free text matching a number", domain.SearchConversationsFilters{SearchQuery: "chat-3"}},
		{"free text matching nothing", domain.SearchConversationsFilters{SearchQuery: "zzzzqq"}},
		{"state active", domain.SearchConversationsFilters{States: []domain.ConversationState{domain.ConversationStateActive}}},
		{"state closed", domain.SearchConversationsFilters{States: []domain.ConversationState{domain.ConversationStateClosed}}},
		{"two states", domain.SearchConversationsFilters{States: []domain.ConversationState{domain.ConversationStateActive, domain.ConversationStateResolved}}},
		{"project A", domain.SearchConversationsFilters{ProjectIDs: []string{csProjectA}}},
		{"project B", domain.SearchConversationsFilters{ProjectIDs: []string{csProjectB}}},
		{"both projects", domain.SearchConversationsFilters{ProjectIDs: []string{csProjectA, csProjectB}}},
		{"exact number", domain.SearchConversationsFilters{Number: csStrPtr("CS-CHAT-4")}},
		{"created by", domain.SearchConversationsFilters{CreatedBy: []string{csCreator}}},
		{"created by, other case", domain.SearchConversationsFilters{CreatedBy: []string{strings.ToUpper(csCreator)}}},
		{"combined", domain.SearchConversationsFilters{SearchQuery: "chat", States: []domain.ConversationState{domain.ConversationStateActive}, ProjectIDs: []string{csProjectA}}},
	}
}

// csScoped confines a search to the fixture's two projects, so conversations
// already present in a shared test database do not enter the comparison. It
// leaves a filter that names its own projects alone (they are the fixture's).
func csScoped(f domain.SearchConversationsFilters) domain.SearchConversationsFilters {
	if len(f.ProjectIDs) == 0 {
		f.ProjectIDs = []string{csProjectA, csProjectB}
	}
	return f
}

// The new queries return what the previous ones did, and what the fixture says
// they should, for every identity, filter, sort and page.
func TestConversationSearchIntegration_MatchesPreviousQueriesAndModel(t *testing.T) {
	pool := caseStatsPool(t)
	seedConversationSearchFixture(t, pool, false)
	scoped := repository.NewScoped(pool)
	repo := repository.NewConversationRepository(scoped)

	sorts := []struct {
		name      string
		field     domain.ConversationSortField
		order     domain.ConversationSortOrder
		col, dir  string
		byUpdated bool
		asc       bool
	}{
		{"created desc", domain.ConversationSortFieldCreatedOn, domain.ConversationSortOrderDesc, "wi.created_on", "DESC", false, false},
		{"updated asc", domain.ConversationSortFieldUpdatedOn, domain.ConversationSortOrderAsc, "wi.updated_on", "ASC", true, true},
	}

	for _, id := range csIdentities() {
		ctx := repository.WithCallerIdentity(context.Background(), id.scope)
		for _, base := range csFilters() {
			// Every identity runs the fixture-scoped filter; a caller whose
			// visibility the policies decide (not "everything") also runs the
			// filter exactly as given, which no pre-existing row can reach.
			variants := []struct {
				name string
				f    domain.SearchConversationsFilters
			}{{base.name, csScoped(base.f)}}
			if !id.scope.Unrestricted {
				variants = append(variants, struct {
					name string
					f    domain.SearchConversationsFilters
				}{base.name + " (as given)", base.f})
			}
			for _, fc := range variants {
				for _, s := range sorts {
					model := csModel(id.visible, fc.f, s.byUpdated, s.asc)
					var gotAll []string
					for _, page := range []struct{ limit, offset int }{{2, 0}, {2, 2}, {2, 4}, {2, 6}, {20, 0}} {
						name := fmt.Sprintf("%s / %s / %s / limit %d offset %d", id.name, fc.name, s.name, page.limit, page.offset)

						views, total, err := repo.SearchConversations(ctx, domain.SearchConversationsRequest{
							Filters: fc.f, SortBy: domain.ConversationSort{Field: s.field, Order: s.order},
							Pagination: domain.Pagination{Limit: page.limit, Offset: page.offset},
						}, "")
						if err != nil {
							t.Fatalf("%s: SearchConversations: %v", name, err)
						}
						oldRows, oldTotal := csOldSearch(t, scoped, ctx, fc.f, s.col, s.dir, page.limit, page.offset)

						if total != oldTotal || total != len(model) {
							t.Errorf("%s: total = %d, previous query = %d, model = %d", name, total, oldTotal, len(model))
						}
						if len(views) != len(oldRows) {
							t.Fatalf("%s: %d rows, previous query returned %d", name, len(views), len(oldRows))
						}
						for i := range views {
							if got := csViewRow(views[i]); got != oldRows[i] {
								t.Errorf("%s: row %d = %+v, previous query = %+v", name, i, got, oldRows[i])
							}
						}
						if page.limit == 2 {
							for _, v := range views {
								gotAll = append(gotAll, *v.ID)
							}
						} else {
							// The wide page is the whole result: check it against the model.
							ids := make([]string, len(views))
							for i, v := range views {
								ids[i] = *v.ID
							}
							if strings.Join(ids, ",") != strings.Join(model, ",") {
								t.Errorf("%s: ids = %v, model = %v", name, ids, model)
							}
						}
					}
					// Walking the pages two at a time yields the model, once each, in order.
					if strings.Join(gotAll, ",") != strings.Join(model, ",") {
						t.Errorf("%s / %s / %s: paged ids = %v, model = %v", id.name, fc.name, s.name, gotAll, model)
					}
				}
			}
		}
	}
}

// A project member sees only their project's conversations and a stranger none:
// visibility comes from the row-level security policies on every statement.
func TestConversationSearchIntegration_VisibilityFollowsRLS(t *testing.T) {
	pool := caseStatsPool(t)
	seedConversationSearchFixture(t, pool, false)
	repo := repository.NewConversationRepository(repository.NewScoped(pool))

	count := func(scope repository.SearchScope, f domain.SearchConversationsFilters) (int, int) {
		ctx := repository.WithCallerIdentity(context.Background(), scope)
		views, total, err := repo.SearchConversations(ctx, domain.SearchConversationsRequest{Filters: f, Pagination: domain.Pagination{Limit: 20}}, "")
		if err != nil {
			t.Fatalf("SearchConversations: %v", err)
		}
		return len(views), total
	}
	// An internal caller sees every conversation (the fixture's two projects are
	// named so rows already in the database stay out of the count); the member
	// and the stranger run with no filter at all, so only the policies decide.
	if n, total := count(repository.SearchScope{Unrestricted: true}, csScoped(domain.SearchConversationsFilters{})); n != 7 || total != 7 {
		t.Errorf("internal: %d rows, total %d, want 7/7", n, total)
	}
	if n, total := count(repository.SearchScope{ViewerEmail: csMember}, domain.SearchConversationsFilters{}); n != 5 || total != 5 {
		t.Errorf("member of project A: %d rows, total %d, want 5/5", n, total)
	}
	if n, total := count(repository.SearchScope{ViewerEmail: csStranger}, domain.SearchConversationsFilters{}); n != 0 || total != 0 {
		t.Errorf("stranger: %d rows, total %d, want 0/0", n, total)
	}
	// Naming a project the caller is not in does not widen what they can see.
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: csMember})
	views, total, err := repo.SearchConversations(ctx, domain.SearchConversationsRequest{
		Filters:    domain.SearchConversationsFilters{ProjectIDs: []string{csProjectB}},
		Pagination: domain.Pagination{Limit: 20},
	}, "")
	if err != nil {
		t.Fatalf("SearchConversations: %v", err)
	}
	if len(views) != 0 || total != 0 {
		t.Errorf("member asking for project B: %d rows, total %d, want 0/0", len(views), total)
	}
}

// Two "user" rows sharing an address used to fan the conversation out into two
// rows and inflate the total; now it is one row, resolved to the lowest user id.
func TestConversationSearchIntegration_DuplicateCreatorEmailIsOneRow(t *testing.T) {
	pool := caseStatsPool(t)
	seedConversationSearchFixture(t, pool, true)
	scoped := repository.NewScoped(pool)
	repo := repository.NewConversationRepository(scoped)
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true})

	filters := domain.SearchConversationsFilters{Number: csStrPtr("CS-CHAT-4")}
	views, total, err := repo.SearchConversations(ctx, domain.SearchConversationsRequest{
		Filters: filters, Pagination: domain.Pagination{Limit: 20},
	}, "")
	if err != nil {
		t.Fatalf("SearchConversations: %v", err)
	}
	if len(views) != 1 || total != 1 {
		t.Fatalf("conversation with a duplicated creator address: %d rows, total %d, want 1/1", len(views), total)
	}
	if views[0].CreatedBy == nil || views[0].CreatedBy.ID == nil || *views[0].CreatedBy.ID != csUserDupLow {
		t.Errorf("creator = %+v, want the lowest user id %s", views[0].CreatedBy, csUserDupLow)
	}

	// The whole list: 7 conversations, once each, total 7.
	all, allTotal, err := repo.SearchConversations(ctx, domain.SearchConversationsRequest{
		Filters: csScoped(domain.SearchConversationsFilters{}), Pagination: domain.Pagination{Limit: 20},
	}, "")
	if err != nil {
		t.Fatalf("SearchConversations: %v", err)
	}
	seen := map[string]bool{}
	for _, v := range all {
		if seen[*v.ID] {
			t.Errorf("conversation %s listed twice", *v.ID)
		}
		seen[*v.ID] = true
	}
	if len(all) != 7 || allTotal != 7 {
		t.Errorf("full list: %d rows, total %d, want 7/7", len(all), allTotal)
	}

	// The previous queries really did double it, so this test would have caught it.
	oldRows, oldTotal := csOldSearch(t, scoped, ctx, filters, "wi.created_on", "DESC", 20, 0)
	if len(oldRows) != 2 || oldTotal != 2 {
		t.Errorf("previous queries for the same search: %d rows, total %d, expected the duplicated 2/2 this test guards against", len(oldRows), oldTotal)
	}
}
