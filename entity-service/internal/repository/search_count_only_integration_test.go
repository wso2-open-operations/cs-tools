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

// CountOnly on case search: the page query is never sent, Total matches what
// the same search would report without it, and Cases is an empty slice, not
// nil. Also proves the projectOnboardingStatus "in" filter's exclusion-list
// rewrite matches the original inclusion-join semantics exactly, against
// real data. Runs the real repository on a real database and records the
// statements it sends. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CountOnly
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ProjectOnboardingStatusInMatchesLegacyJoinForm

package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// nonCountStatements is every recorded statement that is not itself a
// "SELECT COUNT(*)" -- for a plain search this is the one page query; for a
// CountOnly search it must be none, since the page query is never built or
// sent at all.
func (r *statementRecorder) nonCountStatements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, s := range r.sqls {
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "SELECT COUNT(*)") {
			out = append(out, s)
		}
	}
	return out
}

func TestSearchCasesCountOnlyIntegration_StatementsSent(t *testing.T) {
	pool, rec := tracedPool(t)
	cases := repository.NewCaseRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true, ViewerEmail: "count-only-test@wso2.com"}
	ctx := repository.WithCallerIdentity(context.Background(), scope)
	req := domain.SearchCasesRequest{
		SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
		Pagination: domain.Pagination{Limit: 5},
	}

	rec.reset()
	if _, total, err := cases.SearchCases(ctx, req, scope); err != nil {
		t.Fatalf("plain search: %v", err)
	} else if total < 0 {
		t.Errorf("a plain search reported total %d", total)
	}
	if n := rec.counts(); n != 1 {
		t.Errorf("a plain search sent %d COUNT statements, want 1", n)
	}
	if n := len(rec.nonCountStatements()); n != 1 {
		t.Errorf("a plain search sent %d non-COUNT statements, want 1 (the page query)", n)
	}

	rec.reset()
	req.CountOnly = true
	cases2, total2, err := cases.SearchCases(ctx, req, scope)
	if err != nil {
		t.Fatalf("countOnly search: %v", err)
	}
	if n := rec.counts(); n != 1 {
		t.Errorf("a countOnly search sent %d COUNT statements, want 1", n)
	}
	if n := len(rec.nonCountStatements()); n != 0 {
		t.Errorf("a countOnly search sent %d non-COUNT statements, want 0 (no page query): %v", n, rec.nonCountStatements())
	}
	if cases2 == nil || len(cases2) != 0 {
		t.Errorf("expected an empty, non-nil Cases slice, got %#v", cases2)
	}
	if total2 < 0 {
		t.Errorf("a countOnly search reported total %d", total2)
	}
}

// Total must agree whether or not CountOnly is set -- it is still the same
// COUNT query either way, just optionally paired with a page query.
//
// buildCaseSearchWhere reads req.Parsed, not the raw Filters.Filters -- that
// translation normally happens in the service layer
// (service.ParseCaseFieldFilters), which this repository-level test bypasses
// entirely, so Parsed.Types is set directly here instead (same convention as
// case_repo_assignee_sort_integration_test.go's searchAssigneeSortNames).
func TestSearchCasesCountOnlyIntegration_TotalMatchesOrdinarySearch(t *testing.T) {
	pool, _ := tracedPool(t)
	cases := repository.NewCaseRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true, ViewerEmail: "count-only-test@wso2.com"}
	ctx := repository.WithCallerIdentity(context.Background(), scope)
	base := domain.SearchCasesRequest{
		SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
		Pagination: domain.Pagination{Limit: 5},
		Parsed:     domain.ParsedCaseFilters{Types: []string{"case"}},
	}

	_, wantTotal, err := cases.SearchCases(ctx, base, scope)
	if err != nil {
		t.Fatalf("plain search: %v", err)
	}

	base.CountOnly = true
	gotCases, gotTotal, err := cases.SearchCases(ctx, base, scope)
	if err != nil {
		t.Fatalf("countOnly search: %v", err)
	}
	if gotTotal != wantTotal {
		t.Errorf("countOnly total %d, plain search total %d", gotTotal, wantTotal)
	}
	if len(gotCases) != 0 {
		t.Errorf("expected no rows from a countOnly search, got %d", len(gotCases))
	}
}

// The projectOnboardingStatus "in" filter was rewritten from a join-based
// inclusion check (`p.onboarding_status = ANY(...)`) to an exclusion-list id
// lookup against "project", for a dashboard-widget-shaped filter that matches
// nearly every project (see case_repo.go's own doc comment on the filter).
// This proves the rewrite returns exactly the same total as the original
// join form, evaluated here as a raw, independent query against the live
// schema rather than against the (now-rewritten) repository code itself.
//
// Parsed is set directly (see TestSearchCasesCountOnlyIntegration_TotalMatchesOrdinarySearch's
// own doc comment for why), and the oracle query runs through the same
// Scoped/WithCallerIdentity path SearchCases itself uses -- a plain
// pool.QueryRow with no identity set would see a different row set than the
// scoped repository call under row-level security, which would make this
// "equivalence" check meaningless.
func TestSearchCasesIntegration_ProjectOnboardingStatusInMatchesLegacyJoinForm(t *testing.T) {
	pool, _ := tracedPool(t)
	scoped := repository.NewScoped(pool)
	cases := repository.NewCaseRepository(scoped)
	scope := repository.SearchScope{Unrestricted: true, ViewerEmail: "count-only-test@wso2.com"}
	ctx := repository.WithCallerIdentity(context.Background(), scope)

	// The six statuses every real dashboard widget sends: "every project
	// except the in-progress ones".
	statuses := []string{"Cancelled", "Completed", "Expired", "Not-Applicable", "Not-Started", "OnHold"}
	req := domain.SearchCasesRequest{
		SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
		Pagination: domain.Pagination{Limit: 1},
		CountOnly:  true,
		Parsed: domain.ParsedCaseFilters{
			Types:                     []string{"case"},
			States:                    []domain.CaseState{domain.CaseStateOpen},
			ProjectOnboardingStatuses: statuses,
		},
	}
	_, gotTotal, err := cases.SearchCases(ctx, req, scope)
	if err != nil {
		t.Fatalf("rewritten search: %v", err)
	}

	// The same predicate, evaluated the original way (an inclusion join
	// against the already-joined project row), run directly against the
	// live schema -- the independent oracle the rewrite is checked against.
	const legacyQuery = `
		SELECT COUNT(*) FROM work_item wi
		LEFT JOIN "case" c ON c.id = wi.id
		LEFT JOIN project p ON p.id = wi.project_id
		WHERE wi.type = 'CASE' AND c.state = 'OPEN'
		  AND p.onboarding_status = ANY($1::text[]::onboarding_status_enum[])`
	var wantTotal int
	if err := scoped.QueryRow(ctx, legacyQuery, []string{
		"CANCELLED", "COMPLETED", "EXPIRED", "NOT_APPLICABLE", "NOT_STARTED", "ON_HOLD",
	}).Scan(&wantTotal); err != nil {
		t.Fatalf("legacy oracle query: %v", err)
	}

	if gotTotal != wantTotal {
		t.Errorf("rewritten exclusion-list filter total %d, legacy inclusion-join oracle total %d", gotTotal, wantTotal)
	}
}
