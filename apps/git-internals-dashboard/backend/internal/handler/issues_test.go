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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://gid:gid@localhost:5433/gid?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Skipf("skipping: postgres unreachable at %s: %v", url, err)
		return nil
	}
	t.Cleanup(pool.Close)
	return pool
}

var handlerTestConfig = &config.AppConfig{
	Taxonomy: config.Taxonomy{
		Statuses: []config.StatusEntry{
			{Name: "Open", Category: config.CategoryProductSide, AccruesSla: true, SortOrder: 10},
			{Name: "In Progress", Category: config.CategoryProductSide, AccruesSla: true, SortOrder: 20},
			{Name: "WOC", Category: config.CategoryCSSide, AccruesSla: false, SortOrder: 30},
			{Name: "Pending Patch Queue", Category: config.CategoryCSSide, AccruesSla: false, SortOrder: 40},
			{Name: "Resolved", Category: config.CategoryOther, AccruesSla: false, IsTerminal: true, SortOrder: 50},
		},
	},
}

// --- Validation tests (no DB) ---

func TestListIssuesValidation400s(t *testing.T) {
	h := NewIssuesHandler(nil, handlerTestConfig)

	cases := []struct {
		name  string
		query string
	}{
		{"bad repo format", "repo=not-a-repo"},
		{"bad state", "state=FOO"},
		{"bad slaState", "slaState=WEIRD"},
		{"bad q (not numeric)", "q=abc"},
		{"limit too low", "limit=0"},
		{"limit too high", "limit=501"},
		{"bad bucket", "bucket=nonexistent"},
		{"bad order", "order=nonexistent"},
		{"priority too long", "priority=" + strings.Repeat("x", 51)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/issues?"+tc.query, nil)
			rec := httptest.NewRecorder()
			h.ListIssues(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON envelope: %v", err)
			}
			if body.Error.Code != apierror.CodeValidationFailed {
				t.Errorf("expected code=%s, got %s", apierror.CodeValidationFailed, body.Error.Code)
			}
		})
	}
}

func TestGetIssueValidation400ForNonIntegerID(t *testing.T) {
	h := NewIssuesHandler(nil, handlerTestConfig)
	req := httptest.NewRequest(http.MethodGet, "/issues/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	h.GetIssue(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --- DB-backed fixture ---

type issueFixture struct {
	number        int
	priority      *string
	currentStatus *string
	state         string
	slaState      string
	budgetHours   *float64
	pctConsumed   *float64
	updatedAt     time.Time
}

func strp(s string) *string   { return &s }
func f64p(f float64) *float64 { return &f }

// seedIssuesFixture inserts a project+repository and a hand-picked set of
// issues (bypassing ingest entirely — these tests only care about the final
// issues/issue_sla row shape, not event history) covering every bucket case
// deterministically, independent of wall-clock time.
func seedIssuesFixture(t *testing.T, pool *pgxpool.Pool) (repoID int32) {
	t.Helper()
	ctx := context.Background()

	var projectID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_handler_test", "Handler Test").Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-issues", `label:"Origin/CS"`, projectID).Scan(&repoID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repoID)
		pool.Exec(bg, `DELETE FROM issue_status_events WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repoID)
		pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fixtures := []issueFixture{
		{number: 101, priority: strp("Critical(P1)"), currentStatus: strp("In Progress"), state: "OPEN", slaState: "VIOLATED", budgetHours: f64p(24), pctConsumed: f64p(1.5), updatedAt: base.Add(5 * time.Hour)},
		{number: 102, priority: strp("High(P2)"), currentStatus: strp("WOC"), state: "OPEN", slaState: "AT_RISK", budgetHours: f64p(24), pctConsumed: f64p(0.8), updatedAt: base.Add(4 * time.Hour)},
		{number: 103, priority: strp("Medium(P3)"), currentStatus: strp("Open"), state: "OPEN", slaState: "OK", budgetHours: f64p(48), pctConsumed: f64p(0.2), updatedAt: base.Add(3 * time.Hour)},
		{number: 104, priority: nil, currentStatus: strp("In Progress"), state: "OPEN", slaState: "NO_SLA", updatedAt: base.Add(2 * time.Hour)},
		{number: 105, priority: strp("Critical(P1)"), currentStatus: strp("Resolved"), state: "OPEN", slaState: "TERMINAL", updatedAt: base.Add(1 * time.Hour)},
		{number: 106, priority: strp("Medium(P3)"), currentStatus: strp("Resolved"), state: "CLOSED", slaState: "TERMINAL", updatedAt: base},
		{number: 107, priority: strp("High(P2)"), currentStatus: strp("Pending Patch Queue"), state: "OPEN", slaState: "NO_SLA", updatedAt: base.Add(6 * time.Hour)},
		{number: 108, priority: strp("Critical(P1)"), currentStatus: strp("In Progress"), state: "CLOSED", slaState: "OK", budgetHours: f64p(24), pctConsumed: f64p(0.1), updatedAt: base.Add(7 * time.Hour)},
	}

	for _, f := range fixtures {
		var issueID int32
		if err := pool.QueryRow(ctx, `
			INSERT INTO issues (repository_id, github_number, state, html_url, priority, current_status, github_created_at, github_updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id
		`, repoID, f.number, f.state, "https://github.com/test-owner/test-issues/issues/"+strconv.Itoa(f.number), f.priority, f.currentStatus, base, f.updatedAt).Scan(&issueID); err != nil {
			t.Fatalf("insert issue %d: %v", f.number, err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO issue_sla (issue_id, priority, budget_hours, consumed_hours, remaining_hours, pct_consumed, sla_state, sla_running, computed_at, computed_through)
			VALUES ($1,$2,$3,0,$3,$4,$5,false,$6,$6)
		`, issueID, f.priority, f.budgetHours, f.pctConsumed, f.slaState, base); err != nil {
			t.Fatalf("insert issue_sla %d: %v", f.number, err)
		}
	}

	return repoID
}

func decodeIssueList(t *testing.T, rec *httptest.ResponseRecorder) []issueWire {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var result []issueWire
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return result
}

func numbersOf(issues []issueWire) []int {
	out := make([]int, len(issues))
	for i, iss := range issues {
		out[i] = iss.Number
	}
	return out
}

func TestListIssuesDefaultBucketExcludesTerminalAndClosed(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	assertSameSet(t, got, []int{101, 102, 103, 104, 107})
}

func TestListIssuesBucketViolated(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=violated", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101})
}

func TestListIssuesBucketAtRisk(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=at_risk", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102})
}

func TestListIssuesBucketOnTrackExcludesCsSideStatuses(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=on_track", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// 103 is OK and on the product side (Open); no other issue is both OK and non-CS.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{103})
}

func TestListIssuesBucketCsIncludesNoSlaOnCsSide(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=cs", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// 102 (AT_RISK, WOC) and 107 (NO_SLA, Pending Patch Queue) are both CS-side.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102, 107})
}

func TestListIssuesBucketCsNarrowedByStatusParam(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=cs&status=WOC", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102})
}

func TestListIssuesBucketTracked(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=tracked", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// Base scope (state=OPEN, not TERMINAL) still applies; only priority is overridden.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102, 103, 107})
}

func TestListIssuesBucketUntracked(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=untracked", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{104})
}

func TestListIssuesBucketAttention(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=attention", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// VIOLATED (101) ∪ AT_RISK (102) ∪ current CS statuses (102, 107) = {101, 102, 107}.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102, 107})
}

func TestListIssuesSlaStateParamOverridesBaseTerminalExclusion(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&slaState=NO_SLA", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{104, 107})
}

func TestListIssuesStateParamOverridesBaseOpenFilter(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&state=CLOSED", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// state=CLOSED overrides the base OPEN filter, but the base "not TERMINAL"
	// sla filter still applies (only slaState explicitly clears/replaces it):
	// 106 is CLOSED+TERMINAL (excluded), 108 is CLOSED+OK (included).
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{108})
}

func TestListIssuesPriorityFilter(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&priority=High(P2)", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102, 107})
}

func TestListIssuesQNumberFilter(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&q=103", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{103})
}

func TestListIssuesOrderBudgetDescNullsLast(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&order=budget_desc", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	// 101 (1.5) > 102 (0.8) > 103 (0.2) > {104, 107} (null, order unspecified between them)
	if len(got) != 5 || got[0] != 101 || got[1] != 102 || got[2] != 103 {
		t.Fatalf("expected [101 102 103 <104,107 in any order>], got %v", got)
	}
}

func TestListIssuesOrderUpdatedDescIsDefault(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	want := []int{107, 101, 102, 103, 104} // descending githubUpdatedAt among the base-scope set
	if !sliceEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestGetIssueReturnsEventsAscendingWithNoActors(t *testing.T) {
	pool := testPool(t)
	repoID := seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	var issueID int32
	if err := pool.QueryRow(context.Background(), `SELECT id FROM issues WHERE repository_id = $1 AND github_number = 101`, repoID).Scan(&issueID); err != nil {
		t.Fatalf("lookup issue id: %v", err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO issue_status_events (issue_id, project_id, previous_status, status, occurred_at, source, dedupe_key)
		VALUES
			($1, (SELECT sla_project_id FROM repositories WHERE id = $2), NULL, 'Open', $3, 'synthetic', 'dk1'),
			($1, (SELECT sla_project_id FROM repositories WHERE id = $2), 'Open', 'In Progress', $4, 'synthetic', 'dk2')
	`, issueID, repoID, base, base.Add(time.Hour))
	if err != nil {
		t.Fatalf("insert events: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/issues/"+strconv.Itoa(int(issueID)), nil)
	req.SetPathValue("id", strconv.Itoa(int(issueID)))
	rec := httptest.NewRecorder()
	h.GetIssue(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var detail issueDetailWire
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(detail.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(detail.Events))
	}
	if !detail.Events[0].OccurredAt.Before(detail.Events[1].OccurredAt) {
		t.Errorf("expected ascending order, got %+v", detail.Events)
	}
	// PRIVACY: no actor field should exist on the wire at all — verified at
	// compile time by eventWire's field set (id/previousStatus/status/occurredAt only).
}

func TestGetIssue404WhenAbsent(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	req := httptest.NewRequest(http.MethodGet, "/issues/999999999", nil)
	req.SetPathValue("id", "999999999")
	rec := httptest.NewRecorder()
	h.GetIssue(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON envelope: %v", err)
	}
	if body.Error.Code != apierror.CodeNotFound {
		t.Errorf("expected code=%s, got %s", apierror.CodeNotFound, body.Error.Code)
	}
}

func TestGetIssue404WhenRepositoryDisabled(t *testing.T) {
	pool := testPool(t)
	repoID := seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig)

	if _, err := pool.Exec(context.Background(), `UPDATE repositories SET enabled = false WHERE id = $1`, repoID); err != nil {
		t.Fatalf("disable repository: %v", err)
	}

	var issueID int32
	if err := pool.QueryRow(context.Background(), `SELECT id FROM issues WHERE repository_id = $1 AND github_number = 101`, repoID).Scan(&issueID); err != nil {
		t.Fatalf("lookup issue id: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/issues/"+strconv.Itoa(int(issueID)), nil)
	req.SetPathValue("id", strconv.Itoa(int(issueID)))
	rec := httptest.NewRecorder()
	h.GetIssue(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when repository disabled, got %d", rec.Code)
	}
}

func assertSameSet(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d results %v, got %d %v", len(want), want, len(got), got)
	}
	seen := make(map[int]bool, len(want))
	for _, n := range want {
		seen[n] = true
	}
	for _, n := range got {
		if !seen[n] {
			t.Fatalf("unexpected issue number %d in result %v (want set %v)", n, got, want)
		}
		delete(seen, n)
	}
	if len(seen) != 0 {
		t.Fatalf("missing expected issue numbers %v in result %v", seen, got)
	}
}

func sliceEqual(a, b []int) bool {
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
