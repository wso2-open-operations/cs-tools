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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// Proves POST /announcements/registry/rows (grouping, ordering and paging in
// SQL) returns exactly what the registry used to build in Go from every
// announcement case and every published request. Runs against a real Postgres
// (see announcementVisibilityPool):
//
//	ANNOUNCEMENT_VISIBILITY_TEST_DSN=postgres://... go test ./internal/repository/ -run AnnouncementRegistryRows
const (
	rrAccountID = "7e000000-0000-4000-8000-0000000000a1"
	rrProjectA  = "7e000000-0000-4000-8000-000000000001"
	rrProjectB  = "7e000000-0000-4000-8000-000000000002"
	rrProjectC  = "7e000000-0000-4000-8000-000000000003"
	rrProjectV  = "7e000000-0000-4000-8000-000000000004"
	rrMarker    = "registry-rows-test"
)

// rrMixProjects scopes assertions to the mix seed's own cases, so they hold on a
// database that already holds other announcements.
var rrMixProjects = []string{rrProjectA, rrProjectB, rrProjectC}

func rrCaseID(i int) string { return fmt.Sprintf("7e1%05d-0000-4000-8000-%012d", i, i) }
func rrReqID(i int) string  { return fmt.Sprintf("7e2%05d-0000-4000-8000-%012d", i, i) }

// rrCleanup removes everything these tests create, in dependency order.
func rrCleanup(ctx context.Context, pool *pgxpool.Pool) {
	scoped := repository.NewScoped(pool)
	_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE project_id IN ($1, $2, $3, $4)`, rrProjectA, rrProjectB, rrProjectC, rrProjectV)
	_, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE created_by = $1`, rrMarker)
	_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id IN ($1, $2, $3, $4)`, rrProjectA, rrProjectB, rrProjectC, rrProjectV)
	_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, rrAccountID)
}

func rrSeedBase(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	rrCleanup(ctx, pool)
	t.Cleanup(func() { rrCleanup(ctx, pool) })
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'RR Test Account', 'RR-ACC-1', 'RR-SF-ACC-1')`, rrAccountID, now); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	for i, id := range []string{rrProjectA, rrProjectB, rrProjectC, rrProjectV} {
		if _, err := pool.Exec(ctx, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, $5, $6)`,
			id, now, fmt.Sprintf("RRPROJ%d", i), fmt.Sprintf("RR-SF-PROJ-%d", i), fmt.Sprintf("RR Project %c", 'A'+i), rrAccountID); err != nil {
			t.Fatalf("seed project %d: %v", i, err)
		}
	}
}

// rrRequest describes one announcement_requests row to seed.
type rrRequest struct {
	id            string
	state         string
	subject       string
	createdByMail *string
	createdAt     time.Time
	updatedAt     time.Time
	projectCount  *int
	security      bool
	members       []string // nil stores SQL NULL, empty stores []
}

func rrInsertRequest(t *testing.T, pool *pgxpool.Pool, r rrRequest) {
	t.Helper()
	var ids any
	if r.members != nil {
		b, _ := json.Marshal(r.members)
		ids = string(b)
	}
	typ := "GENERAL"
	if r.security {
		typ = "SECURITY"
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO announcement_requests
		(id, kind, state, subject, announcement_type, created_by, created_by_email, created_on, updated_on, resolved_project_count, published_case_ids)
		VALUES ($1, 'customer', $2, $3, $4::announcement_type_enum, $5, $6, $7, $8, $9, $10::jsonb)`,
		r.id, r.state, r.subject, typ, rrMarker, r.createdByMail, r.createdAt, r.updatedAt, r.projectCount, ids); err != nil {
		t.Fatalf("seed announcement request %s: %v", r.id, err)
	}
}

// rrSeedMix seeds 60 announcement cases across three projects (pairs share an
// updated_on so the id tie-break matters; every 5th is closed) and the request
// shapes the grouping has to get right. Returns the cases' base time.
func rrSeedMix(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	base := time.Now().UTC().Truncate(time.Second).Add(-24 * time.Hour)
	projects := []string{rrProjectA, rrProjectB, rrProjectC}
	for i := 0; i < 60; i++ {
		updated := base.Add(-time.Duration(i/2) * time.Minute)
		if _, err := scoped.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
			VALUES ($1, $2, $3, $4, 'test', $5, $6, $7, 'ANNOUNCEMENT', $8)`,
			rrCaseID(i), updated.Add(-time.Hour), updated, fmt.Sprintf("jane.doe%d@example.com", i%4),
			fmt.Sprintf("RR-%03d", i), fmt.Sprintf("RR-WSO2-%03d", i), fmt.Sprintf("regtest announcement %02d", i), projects[i%3]); err != nil {
			t.Fatalf("seed work_item %d: %v", i, err)
		}
		state := "OPEN"
		if i%5 == 0 {
			state = "CLOSE"
		}
		if _, err := scoped.Exec(ctx, `INSERT INTO announcement (id, announcement_type, state) VALUES ($1, 'GENERAL', $2)`, rrCaseID(i), state); err != nil {
			t.Fatalf("seed announcement %d: %v", i, err)
		}
	}
	ids := func(is ...int) []string {
		out := make([]string, 0, len(is))
		for _, i := range is {
			out = append(out, rrCaseID(i))
		}
		return out
	}
	mail := "jane.doe@example.com"
	five := 7
	reqBase := base.Add(-48 * time.Hour)
	for _, r := range []rrRequest{
		// Several members; case 0 is closed, so a closed-only filter keeps one
		// of five members matching. Resolved count differs from member count.
		{id: rrReqID(1), state: "published", subject: "regtest batch one", createdByMail: &mail, createdAt: reqBase.Add(2 * time.Hour), updatedAt: reqBase.Add(3 * time.Hour), projectCount: &five, security: true, members: ids(0, 1, 2, 3, 4)},
		// No creator email, no resolved count: falls back to created_by and the member count.
		{id: rrReqID(2), state: "published", subject: "regtest batch two", createdAt: reqBase.Add(4 * time.Hour), updatedAt: reqBase.Add(4 * time.Hour), members: ids(10, 11, 12)},
		// Not published, though it lists cases: those cases stay bare rows.
		{id: rrReqID(3), state: "approved", subject: "regtest approved", createdAt: reqBase.Add(5 * time.Hour), updatedAt: reqBase.Add(5 * time.Hour), members: ids(20, 21)},
		{id: rrReqID(4), state: "draft", subject: "regtest draft", createdAt: reqBase.Add(5 * time.Hour), updatedAt: reqBase.Add(5 * time.Hour), members: ids(22, 23)},
		// A missing case id and a malformed id between real members.
		{id: rrReqID(5), state: "published", subject: "regtest batch with gaps", createdAt: reqBase.Add(6 * time.Hour), updatedAt: reqBase.Add(7 * time.Hour),
			members: []string{rrCaseID(30), "7effffff-0000-4000-8000-ffffffffffff", "not-a-uuid", rrCaseID(31), rrCaseID(32)}},
		// Lists case 4 too, and is OLDER than request 1: it owns case 4.
		{id: rrReqID(6), state: "published", subject: "regtest older batch sharing case 4", createdAt: reqBase.Add(1 * time.Hour), updatedAt: reqBase.Add(1 * time.Hour), members: ids(40, 41, 4)},
		// Empty and NULL lists own nothing.
		{id: rrReqID(7), state: "published", subject: "regtest empty list", createdAt: reqBase.Add(8 * time.Hour), updatedAt: reqBase.Add(8 * time.Hour), members: []string{}},
		{id: rrReqID(8), state: "published", subject: "regtest null list", createdAt: reqBase.Add(9 * time.Hour), updatedAt: reqBase.Add(9 * time.Hour)},
		// Same created_on as request 6 would be a tie on the ownership rule; exercised below.
	} {
		rrInsertRequest(t, pool, r)
	}
}

// rrOracle is the grouping the registry did in Go before this route existed
// (the BFF's SearchAnnouncementRegistry plus its case and request fetches),
// ported here as the oracle the SQL must equal. It is deliberately a literal
// port, not a reimplementation of the new rules: cases newest-updated first
// from the one-shot read, published requests newest-created first, a
// caseId -> request map where the later request in that walk overwrites an
// earlier one, then one row per request or bare case in case order.
func rrOracle(t *testing.T, pool *pgxpool.Pool, req domain.SearchCasesRequest, offset, limit int) domain.SearchAnnouncementRegistryRowsResponse {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	regRepo := repository.NewAnnouncementRegistryRepository(scoped)
	scope := repository.SearchScope{Unrestricted: true}

	cases, err := regRepo.SearchAnnouncementCases(ctx, req, scope, 100000)
	if err != nil {
		t.Fatalf("oracle cases: %v", err)
	}
	allReq := domain.SearchCasesRequest{Parsed: domain.ParsedCaseFilters{Types: []string{"announcement"}}}
	all, err := regRepo.SearchAnnouncementCases(ctx, allReq, scope, 100000)
	if err != nil {
		t.Fatalf("oracle unfiltered cases: %v", err)
	}
	lookup := map[string]domain.SearchCaseView{}
	for _, c := range all {
		lookup[c.ID] = c
	}

	type reqView struct {
		id, subject, createdBy string
		email                  *string
		createdOn, updatedOn   time.Time
		count                  *int
		security               bool
		ids                    []string
	}
	rs, err := pool.Query(ctx, `SELECT id::text, subject, created_by, created_by_email, created_on, updated_on, resolved_project_count,
		announcement_type::text = 'SECURITY', COALESCE(published_case_ids, '[]'::jsonb)
		FROM announcement_requests WHERE state = 'published' ORDER BY created_on DESC, id`)
	if err != nil {
		t.Fatalf("oracle requests: %v", err)
	}
	var requests []reqView
	for rs.Next() {
		var v reqView
		var raw []byte
		if err := rs.Scan(&v.id, &v.subject, &v.createdBy, &v.email, &v.createdOn, &v.updatedOn, &v.count, &v.security, &raw); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &v.ids); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, v)
	}
	rs.Close()

	caseToRequest := map[string]*reqView{}
	for i := range requests {
		for _, id := range requests[i].ids {
			caseToRequest[id] = &requests[i]
		}
	}
	var rows []domain.AnnouncementRegistryRow
	memberIDs := map[int][]string{}
	added := map[string]bool{}
	for _, c := range cases {
		if rv, ok := caseToRequest[c.ID]; ok {
			if added[rv.id] {
				continue
			}
			added[rv.id] = true
			count := len(rv.ids)
			if rv.count != nil {
				count = *rv.count
			}
			by := rv.createdBy
			if rv.email != nil && *rv.email != "" {
				by = *rv.email
			}
			memberIDs[len(rows)] = rv.ids
			rows = append(rows, domain.AnnouncementRegistryRow{
				Kind: "batch", Subject: rv.subject, CreatedBy: by,
				CreatedOn: rv.createdOn.UTC().Format(time.RFC3339), UpdatedOn: rv.updatedOn.UTC().Format(time.RFC3339),
				AnnouncementRequestID: rv.id, ProjectCount: count, IsSecurityAnnouncement: rv.security,
			})
			continue
		}
		row := domain.AnnouncementRegistryRow{
			Kind: "case", CreatedOn: c.CreatedOn, UpdatedOn: c.UpdatedOn, CaseID: c.ID, CaseNumber: c.Number, WSO2CaseID: c.InternalID,
		}
		if c.Subject != nil {
			row.Subject = *c.Subject
		}
		if c.State != nil {
			row.State = *c.State
		}
		if c.CreatedBy != nil {
			row.CreatedBy = c.CreatedBy.Name
			if row.CreatedBy == "" {
				row.CreatedBy = c.CreatedBy.Email
			}
		}
		if c.Project != nil {
			row.ProjectName = c.Project.Name
		}
		rows = append(rows, row)
	}
	total := len(rows)
	start := min(offset, total)
	end := min(start+limit, total)
	for i := start; i < end; i++ {
		ids := memberIDs[i]
		if len(ids) == 0 {
			continue
		}
		members := make([]domain.AnnouncementRegistryCaseMember, 0, len(ids))
		for _, id := range ids {
			cv, ok := lookup[id]
			if !ok {
				continue
			}
			m := domain.AnnouncementRegistryCaseMember{CaseID: cv.ID, CaseNumber: cv.Number, WSO2CaseID: cv.InternalID}
			if cv.Project != nil {
				m.ProjectName = cv.Project.Name
			}
			members = append(members, m)
		}
		rows[i].Cases = members
	}
	page := rows[start:end]
	if page == nil {
		page = []domain.AnnouncementRegistryRow{}
	}
	return domain.SearchAnnouncementRegistryRowsResponse{Rows: page, Total: total, Limit: limit, Offset: offset, HasMore: end < total}
}

func rrJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func rrRequestFor(search string, states []domain.CaseState, projects []string, offset, limit int) domain.SearchCasesRequest {
	return domain.SearchCasesRequest{
		Filters:    domain.SearchCasesFilters{SearchQuery: search},
		Parsed:     domain.ParsedCaseFilters{Types: []string{"announcement"}, States: states, ProjectIDs: projects},
		Pagination: domain.Pagination{Limit: limit, Offset: offset},
	}
}

func TestAnnouncementRegistryRowsParityWithGoGroupingIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	rrSeedBase(t, pool)
	rrSeedMix(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewAnnouncementRegistryRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true}

	filters := map[string]struct {
		search   string
		states   []domain.CaseState
		projects []string
	}{
		"no filter":              {},
		"closed only":            {states: []domain.CaseState{domain.CaseStateClosed}},
		"open only":              {states: []domain.CaseState{domain.CaseStateOpen}},
		"one project":            {projects: []string{rrProjectA}},
		"two projects":           {projects: []string{rrProjectB, rrProjectC}},
		"free text":              {search: "announcement 1"},
		"free text, no matches":  {search: "no-such-registry-text"},
		"closed in one project":  {states: []domain.CaseState{domain.CaseStateClosed}, projects: []string{rrProjectC}},
		"free text and projects": {search: "regtest", projects: []string{rrProjectA}},
	}
	pages := [][2]int{{0, 20}, {5, 5}, {10, 7}, {1000, 20}}
	for name, f := range filters {
		for _, pg := range pages {
			t.Run(fmt.Sprintf("%s/offset %d limit %d", name, pg[0], pg[1]), func(t *testing.T) {
				req := rrRequestFor(f.search, f.states, f.projects, pg[0], pg[1])
				got, err := repo.SearchAnnouncementRegistryRows(ctx, req, scope)
				if err != nil {
					t.Fatalf("SearchAnnouncementRegistryRows: %v", err)
				}
				want := rrOracle(t, pool, req, pg[0], pg[1])
				if g, w := rrJSON(t, got), rrJSON(t, want); g != w {
					t.Fatalf("new route differs from the Go grouping\n--- got\n%s\n--- want\n%s", g, w)
				}
			})
		}
	}

	// Guard against a vacuous oracle: the unfiltered result really contains
	// both kinds of row and the request shapes seeded above.
	t.Run("the seed exercises both row kinds", func(t *testing.T) {
		kinds := map[string]int{}
		total := 0
		for offset := 0; offset < 60; offset += 30 {
			got, err := repo.SearchAnnouncementRegistryRows(ctx, rrRequestFor("", nil, rrMixProjects, offset, 30), scope)
			if err != nil {
				t.Fatal(err)
			}
			total = got.Total
			for _, r := range got.Rows {
				kinds[r.Kind]++
			}
		}
		// 60 cases: request 1 (cases 0-3), 2 (10-12), 5 (30-32) and 6 (40, 41
		// and the shared case 4) collapse 13 cases into four batches, leaving
		// 47 bare cases.
		if kinds["batch"] != 4 || kinds["case"] != 47 || total != 51 {
			t.Fatalf("kinds=%v total=%d, want 4 batches, 47 cases, total 51", kinds, total)
		}
	})
}

// A filter that matches only some members of a batch picks which rows appear;
// it must never shrink the members listed for a row that does.
func TestAnnouncementRegistryRowsFilterKeepsAllMembersIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	rrSeedBase(t, pool)
	rrSeedMix(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewAnnouncementRegistryRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true}

	// Only case 0 of request 1 (cases 0-4) is closed.
	got, err := repo.SearchAnnouncementRegistryRows(ctx,
		rrRequestFor("", []domain.CaseState{domain.CaseStateClosed}, []string{rrProjectA}, 0, 50), scope)
	if err != nil {
		t.Fatal(err)
	}
	var batch *domain.AnnouncementRegistryRow
	for i := range got.Rows {
		if got.Rows[i].AnnouncementRequestID == rrReqID(1) {
			batch = &got.Rows[i]
		}
	}
	if batch == nil {
		t.Fatalf("request 1 missing from a closed-in-project-A search: %s", rrJSON(t, got))
	}
	if len(batch.Cases) != 5 {
		t.Fatalf("batch lists %d members, want all 5 despite the filter: %+v", len(batch.Cases), batch.Cases)
	}
	for i, m := range batch.Cases {
		if m.CaseID != rrCaseID(i) {
			t.Fatalf("member %d = %s, want %s (published_case_ids order)", i, m.CaseID, rrCaseID(i))
		}
	}
	if batch.ProjectCount != 7 || !batch.IsSecurityAnnouncement || batch.CreatedBy != "jane.doe@example.com" {
		t.Fatalf("batch fields wrong: %+v", *batch)
	}

	// The gaps request: missing and malformed ids are skipped, order kept.
	all, err := repo.SearchAnnouncementRegistryRows(ctx, rrRequestFor("", nil, rrMixProjects, 0, 50), scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range all.Rows {
		if r.AnnouncementRequestID != rrReqID(5) {
			continue
		}
		if len(r.Cases) != 3 || r.Cases[0].CaseID != rrCaseID(30) || r.Cases[1].CaseID != rrCaseID(31) || r.Cases[2].CaseID != rrCaseID(32) {
			t.Fatalf("gaps batch members = %+v, want cases 30, 31, 32 only", r.Cases)
		}
		if r.ProjectCount != 5 {
			t.Fatalf("project count = %d, want 5 (every listed id, as before)", r.ProjectCount)
		}
		return
	}
	t.Fatal("gaps batch not found")
}

// A case listed by two published requests must not appear twice and must not
// flip between runs: the OLDEST request (created_on, then the greater id)
// owns it. That is the owner the previous Go grouping ended with.
func TestAnnouncementRegistryRowsCaseInTwoPublishedRequestsIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	rrSeedBase(t, pool)
	rrSeedMix(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewAnnouncementRegistryRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true}

	// Case 4 is listed by request 1 (newer) and request 6 (older). A search
	// matching ONLY case 4 returns exactly one row, owned by request 6.
	req := rrRequestFor("announcement 04", nil, rrMixProjects, 0, 50)
	for run := 0; run < 3; run++ {
		got, err := repo.SearchAnnouncementRegistryRows(ctx, req, scope)
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != 1 || len(got.Rows) != 1 || got.Rows[0].AnnouncementRequestID != rrReqID(6) {
			t.Fatalf("run %d: want one row owned by the older request 6, got %s", run, rrJSON(t, got))
		}
	}

	// Equal created_on: the greater id owns it, matching the Go walk.
	tie := time.Now().UTC().Truncate(time.Second).Add(-100 * time.Hour)
	for _, id := range []string{rrReqID(11), rrReqID(12)} {
		rrInsertRequest(t, pool, rrRequest{id: id, state: "published", subject: "regtest tie " + id, createdAt: tie, updatedAt: tie, members: []string{rrCaseID(50)}})
	}
	got, err := repo.SearchAnnouncementRegistryRows(ctx, rrRequestFor("announcement 50", nil, rrMixProjects, 0, 50), scope)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || got.Rows[0].AnnouncementRequestID != rrReqID(12) {
		t.Fatalf("created_on tie: want request %s (greater id) to own the case, got %s", rrReqID(12), rrJSON(t, got))
	}
	want := rrOracle(t, pool, rrRequestFor("announcement 50", nil, rrMixProjects, 0, 50), 0, 50)
	if g, w := rrJSON(t, got), rrJSON(t, want); g != w {
		t.Fatalf("tie rule differs from the Go grouping\n--- got\n%s\n--- want\n%s", g, w)
	}
}

// Volume: more announcements than the old 10,000 row cap, with no error, the
// right total, and correct first and deep pages (checked against the oracle).
func TestAnnouncementRegistryRowsVolumeIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	rrSeedBase(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	repo := repository.NewAnnouncementRegistryRepository(scoped)
	scope := repository.SearchScope{Unrestricted: true}

	const cases, requests, perRequest = 12500, 500, 20 // 10,000 batched cases in 500 batches, 2,500 bare
	vid := func(expr string) string {
		return fmt.Sprintf(`('7d' || lpad(to_hex(%[1]s), 6, '0') || '-0000-4000-8000-' || lpad(to_hex(%[1]s), 12, '0'))::uuid`, expr)
	}
	if _, err := scoped.Exec(ctx, fmt.Sprintf(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		SELECT %s, now() - interval '30 days', now() - ((i * 7919) %% 100000) * interval '1 second', 'jane.doe@example.com', 'test',
		       'RRV-' || i, 'RRV-WSO2-' || i, 'regtest volume announcement ' || i, 'ANNOUNCEMENT', $1
		FROM generate_series(0, $2 - 1) AS i`, vid("i")), rrProjectV, cases); err != nil {
		t.Fatalf("seed volume work_item: %v", err)
	}
	if _, err := scoped.Exec(ctx, fmt.Sprintf(`INSERT INTO announcement (id, announcement_type, state)
		SELECT %s, 'GENERAL', CASE WHEN i %% 9 = 0 THEN 'CLOSE'::announcement_state_enum ELSE 'OPEN'::announcement_state_enum END
		FROM generate_series(0, $1 - 1) AS i`, vid("i")), cases); err != nil {
		t.Fatalf("seed volume announcement: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO announcement_requests (id, kind, state, subject, created_by, created_on, updated_on, published_case_ids)
		SELECT ('7d3' || lpad(to_hex(r), 5, '0') || '-0000-4000-8000-' || lpad(to_hex(r), 12, '0'))::uuid, 'customer', 'published',
		       'regtest volume batch ' || r, $1, now() - r * interval '1 minute', now() - r * interval '1 minute',
		       (SELECT jsonb_agg(%s ORDER BY k) FROM generate_series(0, $3 - 1) AS k)
		FROM generate_series(0, $2 - 1) AS r`, vid("(r * $3 + k)")), rrMarker, requests, perRequest); err != nil {
		t.Fatalf("seed volume requests: %v", err)
	}

	for _, pg := range [][2]int{{0, 50}, {0, 20}, {1500, 50}, {2900, 50}, {2990, 50}, {50000, 20}} {
		req := rrRequestFor("", nil, []string{rrProjectV}, pg[0], pg[1])
		got, err := repo.SearchAnnouncementRegistryRows(ctx, req, scope)
		if err != nil {
			t.Fatalf("offset %d: %v", pg[0], err)
		}
		if got.Total != requests+cases-requests*perRequest {
			t.Fatalf("offset %d: total = %d, want %d grouped rows", pg[0], got.Total, requests+cases-requests*perRequest)
		}
		want := rrOracle(t, pool, req, pg[0], pg[1])
		if g, w := rrJSON(t, got), rrJSON(t, want); g != w {
			t.Fatalf("offset %d limit %d differs from the Go grouping\n--- got\n%.2000s\n--- want\n%.2000s", pg[0], pg[1], g, w)
		}
	}
	// The same data cannot be served by the capped read: that is the failure
	// this route exists to avoid.
	if _, err := repository.NewAnnouncementRegistryRepository(scoped).SearchAnnouncementCases(ctx, rrRequestFor("", nil, []string{rrProjectV}, 0, 50), scope, 10000); err == nil {
		t.Fatal("expected the capped read to refuse 12,500 announcements; the volume seed is too small to prove anything")
	}
}

type rrRestrictedAccess struct{}

func (rrRestrictedAccess) ResolveScope(context.Context) (service.AccessScope, error) {
	return service.AccessScope{ProjectIDs: []string{rrProjectA}, ViewerEmail: "jane.doe@example.com"}, nil
}

// A caller who is not internal gets 403 and nothing is read.
func TestAnnouncementRegistryRowsRefusesNonInternalCallerIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	rrSeedBase(t, pool)
	rrSeedMix(t, pool)
	repo := repository.NewAnnouncementRegistryRepository(repository.NewScoped(pool))
	h := handler.NewAnnouncementRegistryHandler(service.NewAnnouncementRegistryService(repo, rrRestrictedAccess{}))

	rec := httptest.NewRecorder()
	h.SearchRegistryRows(rec, httptest.NewRequest(http.MethodPost, "/announcements/registry/rows", strings.NewReader(`{}`)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

// The handler's wire shape: JSON names, omitempty per kind, RFC 3339 UTC.
func TestAnnouncementRegistryRowsWireShapeIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	rrSeedBase(t, pool)
	rrSeedMix(t, pool)
	repo := repository.NewAnnouncementRegistryRepository(repository.NewScoped(pool))
	h := handler.NewAnnouncementRegistryHandler(service.NewAnnouncementRegistryService(repo, rrUnrestrictedAccess{}))

	body := `{"filters":{"searchQuery":"announcement 0","filters":[{"field":"projectId","op":"in","values":["` + strings.Join(rrMixProjects, `","`) + `"]}]},"pagination":{"offset":0,"limit":3}}`
	rec := httptest.NewRecorder()
	h.SearchRegistryRows(rec, httptest.NewRequest(http.MethodPost, "/announcements/registry/rows", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var raw struct {
		Rows    []map[string]any `json:"rows"`
		Total   int              `json:"total"`
		Limit   int              `json:"limit"`
		Offset  int              `json:"offset"`
		HasMore bool             `json:"hasMore"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Rows) != 3 || raw.Limit != 3 || raw.Offset != 0 || !raw.HasMore || raw.Total <= 3 {
		t.Fatalf("envelope wrong: %s", rec.Body.String())
	}
	for _, r := range raw.Rows {
		switch r["kind"] {
		case "case":
			for _, k := range []string{"caseId", "caseNumber", "wso2CaseId", "state", "projectName", "createdOn", "updatedOn"} {
				if _, ok := r[k]; !ok {
					t.Fatalf("case row lacks %s: %v", k, r)
				}
			}
			for _, k := range []string{"announcementRequestId", "projectCount", "cases", "isSecurityAnnouncement"} {
				if _, ok := r[k]; ok {
					t.Fatalf("case row carries batch field %s: %v", k, r)
				}
			}
			if _, err := time.Parse(time.RFC3339, r["updatedOn"].(string)); err != nil || !strings.HasSuffix(r["updatedOn"].(string), "Z") {
				t.Fatalf("updatedOn %v is not RFC 3339 UTC", r["updatedOn"])
			}
		case "batch":
			for _, k := range []string{"announcementRequestId", "projectCount", "cases", "createdBy"} {
				if _, ok := r[k]; !ok {
					t.Fatalf("batch row lacks %s: %v", k, r)
				}
			}
			for _, k := range []string{"caseId", "caseNumber", "state", "projectName"} {
				if _, ok := r[k]; ok {
					t.Fatalf("batch row carries case field %s: %v", k, r)
				}
			}
		default:
			t.Fatalf("unknown kind in %v", r)
		}
	}
}

type rrUnrestrictedAccess struct{}

func (rrUnrestrictedAccess) ResolveScope(context.Context) (service.AccessScope, error) {
	return service.AccessScope{Unrestricted: true}, nil
}

// An empty page (offset past the end, or a search with no hits) still carries
// the total from the same statement, and hasMore is never true with no rows.
func TestAnnouncementRegistryRowsEmptyPageCarriesTotalIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	rrSeedBase(t, pool)
	rrSeedMix(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewAnnouncementRegistryRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true}

	for _, offset := range []int{51, 52, 500, 100000} {
		got, err := repo.SearchAnnouncementRegistryRows(ctx, rrRequestFor("", nil, rrMixProjects, offset, 20), scope)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Rows) != 0 || got.Total != 51 || got.HasMore || got.Offset != offset || got.Limit != 20 {
			t.Fatalf("offset %d: want no rows, total 51, hasMore false; got %s", offset, rrJSON(t, got))
		}
	}
	got, err := repo.SearchAnnouncementRegistryRows(ctx, rrRequestFor("no-such-registry-text", nil, rrMixProjects, 0, 20), scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 0 || got.Total != 0 || got.HasMore {
		t.Fatalf("zero-hit search: %s", rrJSON(t, got))
	}
	// Last page: exactly at the boundary, hasMore flips to false.
	got, err = repo.SearchAnnouncementRegistryRows(ctx, rrRequestFor("", nil, rrMixProjects, 50, 20), scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Total != 51 || got.HasMore {
		t.Fatalf("last page: %s", rrJSON(t, got))
	}
}
