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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to the docker-composed Postgres, skipping the test
// (rather than failing) when it's unreachable.
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

// repeatedParam builds a query string with n repeated occurrences of
// name=v0, name=v1, ... — for exercising the FilterParamMaxValues cap.
func repeatedParam(name string, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("%s=v%d", name, i)
	}
	return strings.Join(parts, "&")
}

// TestListIssuesValidation400s verifies every malformed query parameter
// ListIssues accepts (bad repo/state/slaState/q/limit/bucket/order/priority)
// is rejected with 400 validation_failed rather than reaching the DB.
func TestListIssuesValidation400s(t *testing.T) {
	h := NewIssuesHandler(nil, handlerTestConfig, appconfig.Default().API)

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
		{"offset negative", "offset=-1"},
		{"offset not numeric", "offset=abc"},
		{"bad bucket", "bucket=nonexistent"},
		{"bad sort", "sort=nonexistent"},
		{"sort=age no longer accepted", "sort=age"},
		{"bad order", "order=sideways"},
		{"priority too long", "priority=" + strings.Repeat("x", 51)},
		{"abtTeam too long", "abtTeam=" + strings.Repeat("x", 101)},
		{"status exceeds filterParamMaxValues", repeatedParam("status", 51)},
		{"one of several status values invalid", "status=WOC&status=" + strings.Repeat("x", 51)},
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

// TestGetIssueValidation400ForNonIntegerID verifies a non-numeric path id
// is rejected with 400 rather than reaching the DB.
func TestGetIssueValidation400ForNonIntegerID(t *testing.T) {
	h := NewIssuesHandler(nil, handlerTestConfig, appconfig.Default().API)
	req := httptest.NewRequest(http.MethodGet, "/issues/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	h.GetIssue(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestGetIssueValidation400ForIDOutsideInt32Range guards against a
// platform-width strconv.Atoi accepting an id like 2147483648 that overflows
// issues.id's Postgres int4 column — pgx would reject it during Scan, which
// the query-error branch maps to 500, not the 400 this out-of-range id
// deserves.
func TestGetIssueValidation400ForIDOutsideInt32Range(t *testing.T) {
	h := NewIssuesHandler(nil, handlerTestConfig, appconfig.Default().API)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/issues/2147483648", nil)
	req.SetPathValue("id", "2147483648")
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
	createdAt     time.Time // zero value defaults to seedIssuesFixture's `base`
	updatedAt     time.Time
	title         *string
	abtTeam       *string
	openedBy      *string
}

// strp returns a pointer to s, for building literal *string fixture fields.
func strp(s string) *string { return &s }

// f64p returns a pointer to f, for building literal *float64 fixture fields.
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
		// createdAt deliberately doesn't track pctConsumed's or updatedAt's own
		// ranking among these five (101-104,107 are the base-scope set), so a
		// sort=created test can't pass by accident of sharing another field's
		// order: oldest -> newest is 103, 107, 104, 101, 102.
		{number: 101, priority: strp("Critical(P1)"), currentStatus: strp("In Progress"), state: "OPEN", slaState: "VIOLATED", budgetHours: f64p(24), pctConsumed: f64p(1.5), createdAt: base.Add(-1 * time.Hour), updatedAt: base.Add(5 * time.Hour), title: strp("Critical bug in auth"), abtTeam: strp("Atlas"), openedBy: strp("jane@wso2.com")},
		{number: 102, priority: strp("High(P2)"), currentStatus: strp("WOC"), state: "OPEN", slaState: "AT_RISK", budgetHours: f64p(24), pctConsumed: f64p(0.8), createdAt: base, updatedAt: base.Add(4 * time.Hour), abtTeam: strp("Atlas")},
		{number: 103, priority: strp("Medium(P3)"), currentStatus: strp("Open"), state: "OPEN", slaState: "OK", budgetHours: f64p(48), pctConsumed: f64p(0.2), createdAt: base.Add(-4 * time.Hour), updatedAt: base.Add(3 * time.Hour)},
		{number: 104, priority: nil, currentStatus: strp("In Progress"), state: "OPEN", slaState: "NO_SLA", createdAt: base.Add(-2 * time.Hour), updatedAt: base.Add(2 * time.Hour)},
		{number: 105, priority: strp("Critical(P1)"), currentStatus: strp("Resolved"), state: "OPEN", slaState: "TERMINAL", updatedAt: base.Add(1 * time.Hour)},
		{number: 106, priority: strp("Medium(P3)"), currentStatus: strp("Resolved"), state: "CLOSED", slaState: "TERMINAL", updatedAt: base},
		{number: 107, priority: strp("High(P2)"), currentStatus: strp("Pending Patch Queue"), state: "OPEN", slaState: "NO_SLA", createdAt: base.Add(-3 * time.Hour), updatedAt: base.Add(6 * time.Hour), abtTeam: strp("Nova")},
		{number: 108, priority: strp("Critical(P1)"), currentStatus: strp("In Progress"), state: "CLOSED", slaState: "OK", budgetHours: f64p(24), pctConsumed: f64p(0.1), updatedAt: base.Add(7 * time.Hour)},
	}

	for _, f := range fixtures {
		createdAt := base
		if !f.createdAt.IsZero() {
			createdAt = f.createdAt
		}
		var issueID int32
		if err := pool.QueryRow(ctx, `
			INSERT INTO issues (repository_id, github_number, state, html_url, priority, current_status, github_created_at, github_updated_at, title, abt_team, opened_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id
		`, repoID, f.number, f.state, "https://github.com/test-owner/test-issues/issues/"+strconv.Itoa(f.number), f.priority, f.currentStatus, createdAt, f.updatedAt, f.title, f.abtTeam, f.openedBy).Scan(&issueID); err != nil {
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

// seedSecondRepoIssue inserts a second project+repository ("test-owner/
// test-issues-2") with a single open, non-terminal issue (number 201), for
// tests that need to prove a `repo` filter ORs across repositories.
func seedSecondRepoIssue(t *testing.T, pool *pgxpool.Pool) (repoID int32) {
	t.Helper()
	ctx := context.Background()

	var projectID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_handler_test_2", "Handler Test 2").Scan(&projectID); err != nil {
		t.Fatalf("create second project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-issues-2", `label:"Origin/CS"`, projectID).Scan(&repoID); err != nil {
		t.Fatalf("create second repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repoID)
		pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var issueID int32
	if err := pool.QueryRow(ctx, `
		INSERT INTO issues (repository_id, github_number, state, html_url, priority, current_status, github_created_at, github_updated_at)
		VALUES ($1,201,'OPEN',$2,'Critical(P1)','Open',$3,$3) RETURNING id
	`, repoID, "https://github.com/test-owner/test-issues-2/issues/201", base).Scan(&issueID); err != nil {
		t.Fatalf("insert second repo issue: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO issue_sla (issue_id, priority, budget_hours, consumed_hours, remaining_hours, pct_consumed, sla_state, sla_running, computed_at, computed_through)
		VALUES ($1,'Critical(P1)',24,0,24,0.1,'OK',false,$2,$2)
	`, issueID, base); err != nil {
		t.Fatalf("insert second repo issue_sla: %v", err)
	}

	return repoID
}

// decodeIssueListEnvelope asserts rec is a 200 and decodes its body as a
// ListIssues response envelope ({issues, total, limit, offset, hasMore}).
func decodeIssueListEnvelope(t *testing.T, rec *httptest.ResponseRecorder) issueListWire {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var result issueListWire
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return result
}

// decodeIssueList is decodeIssueListEnvelope for callers that only care
// about the issues themselves, not total/limit/offset/hasMore.
func decodeIssueList(t *testing.T, rec *httptest.ResponseRecorder) []issueWire {
	t.Helper()
	return decodeIssueListEnvelope(t, rec).Issues
}

// numbersOf extracts each issue's GitHub number, in response order.
func numbersOf(issues []issueWire) []int {
	out := make([]int, len(issues))
	for i, iss := range issues {
		out[i] = iss.Number
	}
	return out
}

// TestListIssuesDefaultBucketExcludesTerminalAndClosed verifies the base
// scope (no bucket/state/slaState param) excludes CLOSED and TERMINAL
// issues.
func TestListIssuesDefaultBucketExcludesTerminalAndClosed(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	assertSameSet(t, got, []int{101, 102, 103, 104, 107})
}

// TestListIssuesBucketViolated verifies bucket=violated returns only
// VIOLATED issues.
func TestListIssuesBucketViolated(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=violated", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101})
}

// TestListIssuesBucketAtRisk verifies bucket=at_risk returns only AT_RISK
// issues.
func TestListIssuesBucketAtRisk(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=at_risk", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102})
}

// TestListIssuesBucketOnTrackExcludesCsSideStatuses verifies bucket=on_track
// returns only OK issues currently on a non-CS-side status.
func TestListIssuesBucketOnTrackExcludesCsSideStatuses(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=on_track", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// 103 is OK and on the product side (Open); no other issue is both OK and non-CS.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{103})
}

// TestListIssuesBucketCsIncludesNoSlaOnCsSide verifies bucket=cs returns
// every issue currently on a CS-side status, including NO_SLA ones.
func TestListIssuesBucketCsIncludesNoSlaOnCsSide(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=cs", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// 102 (AT_RISK, WOC) and 107 (NO_SLA, Pending Patch Queue) are both CS-side.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102, 107})
}

// TestListIssuesBucketCsNarrowedByStatusParam verifies bucket=cs combined
// with an explicit status param narrows to just that status.
func TestListIssuesBucketCsNarrowedByStatusParam(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=cs&status=WOC", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102})
}

// TestListIssuesBucketProductSideIncludesAllProductSideStatuses verifies
// bucket=product_side with no status param returns every open, non-terminal
// issue currently on a PRODUCT_SIDE status.
func TestListIssuesBucketProductSideIncludesAllProductSideStatuses(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=product_side", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// 101 (In Progress), 103 (Open), 104 (In Progress) are open/non-terminal
	// and PRODUCT_SIDE; 108 is In Progress but CLOSED, excluded by base scope.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 103, 104})
}

// TestListIssuesBucketProductSideNarrowedByStatusParam verifies
// bucket=product_side combined with an explicit status param narrows to
// just that status: the bucket's PRODUCT_SIDE status set and the explicit
// status list are separate AND-ed conditions, so the list narrows whatever
// the bucket already allows.
func TestListIssuesBucketProductSideNarrowedByStatusParam(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=product_side&status=Open", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{103})
}

// TestListIssuesBucketTracked verifies bucket=tracked returns issues with a
// non-nil priority, within the base OPEN/non-TERMINAL scope.
func TestListIssuesBucketTracked(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=tracked", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// Base scope (state=OPEN, not TERMINAL) still applies; only priority is overridden.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102, 103, 107})
}

// TestListIssuesBucketUntracked verifies bucket=untracked returns only
// issues with a nil priority.
func TestListIssuesBucketUntracked(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=untracked", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{104})
}

// TestListIssuesBucketAttention verifies bucket=attention returns the union
// of VIOLATED, AT_RISK, and currently-CS-side issues.
func TestListIssuesBucketAttention(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=attention", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// VIOLATED (101) ∪ AT_RISK (102) ∪ current CS statuses (102, 107) = {101, 102, 107}.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102, 107})
}

// TestListIssuesSlaStateParamOverridesBaseTerminalExclusion verifies an
// explicit slaState param (e.g. NO_SLA) replaces the base scope's implicit
// "not TERMINAL" filter rather than being ANDed with it.
func TestListIssuesSlaStateParamOverridesBaseTerminalExclusion(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&slaState=NO_SLA", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{104, 107})
}

// TestListIssuesStateParamOverridesBaseOpenFilter verifies an explicit
// state=CLOSED param replaces the base scope's implicit OPEN filter, while
// the base "not TERMINAL" sla filter still applies underneath it.
func TestListIssuesStateParamOverridesBaseOpenFilter(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&state=CLOSED", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// state=CLOSED overrides the base OPEN filter, but the base "not TERMINAL"
	// sla filter still applies (only slaState explicitly clears/replaces it):
	// 106 is CLOSED+TERMINAL (excluded), 108 is CLOSED+OK (included).
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{108})
}

// TestListIssuesPriorityFilter verifies the priority param narrows results
// to that exact priority label.
func TestListIssuesPriorityFilter(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&priority=High(P2)", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102, 107})
}

// TestListIssuesAbtTeamFilter verifies the abtTeam param narrows results to
// issues with that exact ABT team.
func TestListIssuesAbtTeamFilter(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&abtTeam=Atlas", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102})
}

// TestListIssuesAbtTeamFilterNarrowedByBucket verifies the abtTeam param
// combines with bucket rather than being overridden by it: bucket=cs alone
// returns both 102 and 107 (both CS-side), but adding abtTeam=Atlas narrows
// to just 102 since 107's ABT team is Nova.
func TestListIssuesAbtTeamFilterNarrowedByBucket(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&bucket=cs", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102, 107})

	req = httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&abtTeam=Atlas&bucket=cs", nil)
	rec = httptest.NewRecorder()
	h.ListIssues(rec, req)
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102})
}

// TestListIssuesRepoFilterOrsMultipleValues verifies repeated `repo` values
// are OR-ed together: issues from either named repository are returned.
func TestListIssuesRepoFilterOrsMultipleValues(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	seedSecondRepoIssue(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&repo=test-owner/test-issues-2", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102, 103, 104, 107, 201})
}

// TestListIssuesPriorityFilterOrsMultipleValues verifies repeated `priority`
// values are OR-ed together.
func TestListIssuesPriorityFilterOrsMultipleValues(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&priority=Critical(P1)&priority=High(P2)", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// 101 (Critical(P1)), 102 and 107 (High(P2)); 105 is Critical(P1) but TERMINAL.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102, 107})
}

// TestListIssuesAbtTeamFilterOrsMultipleValues verifies repeated `abtTeam`
// values are OR-ed together.
func TestListIssuesAbtTeamFilterOrsMultipleValues(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&abtTeam=Atlas&abtTeam=Nova", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102, 107})
}

// TestListIssuesStatusFilterOrsMultipleValues verifies repeated `status`
// values are OR-ed together.
func TestListIssuesStatusFilterOrsMultipleValues(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&status=WOC&status=Pending+Patch+Queue", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102, 107})
}

// TestListIssuesSlaStateFilterOrsMultipleValues verifies repeated `slaState`
// values are OR-ed together.
func TestListIssuesSlaStateFilterOrsMultipleValues(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&slaState=VIOLATED&slaState=AT_RISK", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{101, 102})
}

// TestListIssuesTwoMultiValueParamsAreAndedTogether verifies two different
// multi-value parameters are AND-ed: a status in {WOC, Pending Patch Queue}
// narrowed further to abtTeam=Atlas drops 107 (Nova) and keeps only 102.
func TestListIssuesTwoMultiValueParamsAreAndedTogether(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&status=WOC&status=Pending+Patch+Queue&abtTeam=Atlas", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102})
}

// TestListIssuesBucketAndedWithExplicitRepoFilter verifies a bucket combines
// with an explicit filter list rather than being replaced by it.
func TestListIssuesBucketAndedWithExplicitRepoFilter(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	seedSecondRepoIssue(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?bucket=on_track&repo=test-owner/test-issues", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	// bucket=on_track alone would also include 201 (test-issues-2, OK,
	// Open); the repo filter narrows it away.
	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{103})
}

// TestListIssuesPriorityNoneMatchesUnprioritizedIssues verifies
// priority=__none__ matches only issues with a nil priority.
func TestListIssuesPriorityNoneMatchesUnprioritizedIssues(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&priority=__none__", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{104})
}

// TestListIssuesPriorityNoneCombinedWithRealPriority verifies __none__ ORs
// with a real priority value in the same list, same as any other value.
func TestListIssuesPriorityNoneCombinedWithRealPriority(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&priority=__none__&priority=High(P2)", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{102, 104, 107})
}

// TestListIssuesSlaStateNoSlaMatchesIssueWithoutSlaRow verifies
// slaState=NO_SLA matches an issue that has no issue_sla row at all, via
// COALESCE(s.sla_state, 'NO_SLA').
func TestListIssuesSlaStateNoSlaMatchesIssueWithoutSlaRow(t *testing.T) {
	pool := testPool(t)
	repoID := seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO issues (repository_id, github_number, state, html_url, current_status, github_created_at, github_updated_at)
		VALUES ($1,999,'OPEN',$2,'Open',$3,$3)
	`, repoID, "https://github.com/test-owner/test-issues/issues/999", base); err != nil {
		t.Fatalf("insert issue without an issue_sla row: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&slaState=NO_SLA", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{104, 107, 999})
}

// TestListIssuesWireIncludesTitleAbtTeamOpenedBy verifies title/abtTeam/
// openedBy round-trip onto the wire: present when set on the row, null when
// not.
func TestListIssuesWireIncludesTitleAbtTeamOpenedBy(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	issues := decodeIssueList(t, rec)
	byNumber := make(map[int]issueWire, len(issues))
	for _, iss := range issues {
		byNumber[iss.Number] = iss
	}

	with, ok := byNumber[101]
	if !ok {
		t.Fatalf("expected issue 101 in response, got %v", numbersOf(issues))
	}
	if with.Title == nil || *with.Title != "Critical bug in auth" {
		t.Errorf("expected title=%q, got %v", "Critical bug in auth", with.Title)
	}
	if with.AbtTeam == nil || *with.AbtTeam != "Atlas" {
		t.Errorf("expected abtTeam=%q, got %v", "Atlas", with.AbtTeam)
	}
	if with.OpenedBy == nil || *with.OpenedBy != "jane@wso2.com" {
		t.Errorf("expected openedBy=%q, got %v", "jane@wso2.com", with.OpenedBy)
	}

	without, ok := byNumber[103]
	if !ok {
		t.Fatalf("expected issue 103 in response, got %v", numbersOf(issues))
	}
	if without.Title != nil {
		t.Errorf("expected title=nil, got %v", *without.Title)
	}
	if without.AbtTeam != nil {
		t.Errorf("expected abtTeam=nil, got %v", *without.AbtTeam)
	}
	if without.OpenedBy != nil {
		t.Errorf("expected openedBy=nil, got %v", *without.OpenedBy)
	}
}

// TestListIssuesQNumberFilter verifies q= filters to the issue whose GitHub
// number matches the query string.
func TestListIssuesQNumberFilter(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&q=103", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	assertSameSet(t, numbersOf(decodeIssueList(t, rec)), []int{103})
}

// TestListIssuesSortSLAConsumptionNullsLast verifies sort=sla_consumption
// sorts by pct_consumed descending, with null-budget issues sorted last.
func TestListIssuesSortSLAConsumptionNullsLast(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&sort=sla_consumption", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	// 101 (1.5) > 102 (0.8) > 103 (0.2) > {104, 107} (null, order unspecified between them)
	if len(got) != 5 || got[0] != 101 || got[1] != 102 || got[2] != 103 {
		t.Fatalf("expected [101 102 103 <104,107 in any order>], got %v", got)
	}
}

// TestListIssuesSortDefaultsToSLAConsumption verifies the default (no sort
// param) is the same ordering as an explicit sort=sla_consumption.
func TestListIssuesSortDefaultsToSLAConsumption(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	if len(got) != 5 || got[0] != 101 || got[1] != 102 || got[2] != 103 {
		t.Fatalf("expected [101 102 103 <104,107 in any order>], got %v", got)
	}
}

// TestListIssuesSortSLAConsumptionAscendingNullsLast verifies
// sort=sla_consumption&order=asc sorts lowest-consumed first, with null
// values still sorted last rather than first.
func TestListIssuesSortSLAConsumptionAscendingNullsLast(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&sort=sla_consumption&order=asc", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	// 103 (0.2) < 102 (0.8) < 101 (1.5) < {104, 107} (null, order unspecified between them)
	if len(got) != 5 || got[0] != 103 || got[1] != 102 || got[2] != 101 {
		t.Fatalf("expected [103 102 101 <104,107 in any order>], got %v", got)
	}
}

// TestListIssuesSortCreatedDescending verifies sort=created (default order,
// descending) orders newest githubCreatedAt first.
func TestListIssuesSortCreatedDescending(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&sort=created", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	want := []int{102, 101, 104, 107, 103} // newest -> oldest githubCreatedAt among the base-scope set
	if !sliceEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestListIssuesSortCreatedAscending verifies sort=created&order=asc orders
// oldest githubCreatedAt first, independent of pct_consumed's and
// githubUpdatedAt's own ranking among the same issues.
func TestListIssuesSortCreatedAscending(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&sort=created&order=asc", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	want := []int{103, 107, 104, 101, 102} // oldest -> newest githubCreatedAt among the base-scope set
	if !sliceEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestListIssuesSortUpdatedDescending verifies sort=updated (default order,
// descending) orders most-recently-updated first.
func TestListIssuesSortUpdatedDescending(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&sort=updated", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	want := []int{107, 101, 102, 103, 104} // most- -> least-recently-updated among the base-scope set
	if !sliceEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestListIssuesSortUpdatedAscending verifies sort=updated&order=asc orders
// least-recently-updated first.
func TestListIssuesSortUpdatedAscending(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&sort=updated&order=asc", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	got := numbersOf(decodeIssueList(t, rec))
	want := []int{104, 103, 102, 101, 107} // least- -> most-recently-updated among the base-scope set
	if !sliceEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestListIssuesSortStablePagingAcrossTiedValues verifies limit=1 paging
// through every offset reproduces the same order a single unpaged request
// returns, including across the tied (both-NULL) pct_consumed values of 104
// and 107 — proving the i.id ASC tie-breaker makes paging deterministic.
func TestListIssuesSortStablePagingAcrossTiedValues(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	fullReq := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&sort=sla_consumption&limit=10", nil)
	fullRec := httptest.NewRecorder()
	h.ListIssues(fullRec, fullReq)
	want := numbersOf(decodeIssueList(t, fullRec))

	got := make([]int, 0, len(want))
	for offset := 0; offset < len(want); offset++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/issues?repo=test-owner/test-issues&sort=sla_consumption&limit=1&offset=%d", offset), nil)
		rec := httptest.NewRecorder()
		h.ListIssues(rec, req)
		page := numbersOf(decodeIssueList(t, rec))
		if len(page) != 1 {
			t.Fatalf("offset %d: expected 1 issue, got %v", offset, page)
		}
		got = append(got, page[0])
	}
	if !sliceEqual(got, want) {
		t.Errorf("paging through tied sort values wasn't stable: want %v, got %v", want, got)
	}
}

// TestListIssuesOffsetWindowsThroughSortOrder verifies limit+offset page
// through the same sort=sla_consumption ordering already proven above,
// rather than returning an arbitrary/unstable subset.
func TestListIssuesOffsetWindowsThroughSortOrder(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&sort=sla_consumption&limit=2&offset=2", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)

	env := decodeIssueListEnvelope(t, rec)
	// Full sla_consumption order is [101 102 103 <104,107>]; offset=2,limit=2
	// lands on index 2 (103) and index 3 (104 or 107, whichever sorts there).
	if len(env.Issues) != 2 || env.Issues[0].Number != 103 {
		t.Fatalf("expected page [103 <104 or 107>], got %v", numbersOf(env.Issues))
	}
	if env.Total != 5 {
		t.Errorf("expected total=5, got %d", env.Total)
	}
	if env.Limit != 2 || env.Offset != 2 {
		t.Errorf("expected limit=2 offset=2 echoed back, got limit=%d offset=%d", env.Limit, env.Offset)
	}
	if !env.HasMore {
		t.Error("expected hasMore=true (2 more issues follow this page)")
	}
}

// TestListIssuesHasMoreFalseOnLastPage verifies hasMore is false once
// offset+len(issues) reaches total, including when offset lands past the
// end (empty page).
func TestListIssuesHasMoreFalseOnLastPage(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

	req := httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&limit=10&offset=0", nil)
	rec := httptest.NewRecorder()
	h.ListIssues(rec, req)
	env := decodeIssueListEnvelope(t, rec)
	if len(env.Issues) != 5 || env.Total != 5 || env.HasMore {
		t.Fatalf("expected all 5 issues on one page with hasMore=false, got %d issues total=%d hasMore=%v", len(env.Issues), env.Total, env.HasMore)
	}

	req = httptest.NewRequest(http.MethodGet, "/issues?repo=test-owner/test-issues&limit=10&offset=50", nil)
	rec = httptest.NewRecorder()
	h.ListIssues(rec, req)
	env = decodeIssueListEnvelope(t, rec)
	if len(env.Issues) != 0 || env.Total != 5 || env.HasMore {
		t.Fatalf("expected an empty page past the end with hasMore=false, got %d issues total=%d hasMore=%v", len(env.Issues), env.Total, env.HasMore)
	}
}

// TestGetIssueReturnsEventsAscendingWithNoActors verifies GetIssue's event
// timeline comes back ascending by occurredAt with no actor field on the
// wire.
func TestGetIssueReturnsEventsAscendingWithNoActors(t *testing.T) {
	pool := testPool(t)
	repoID := seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

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

// TestGetIssue404WhenAbsent verifies a non-existent issue id returns 404
// not_found.
func TestGetIssue404WhenAbsent(t *testing.T) {
	pool := testPool(t)
	seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

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

// TestGetIssue404WhenRepositoryDisabled verifies an issue whose repository
// was subsequently disabled returns 404, not its previously-visible data.
func TestGetIssue404WhenRepositoryDisabled(t *testing.T) {
	pool := testPool(t)
	repoID := seedIssuesFixture(t, pool)
	h := NewIssuesHandler(pool, handlerTestConfig, appconfig.Default().API)

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

// assertSameSet fails the test unless got and want contain the same issue
// numbers, ignoring order.
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

// sliceEqual reports whether a and b hold the same ints in the same order.
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
