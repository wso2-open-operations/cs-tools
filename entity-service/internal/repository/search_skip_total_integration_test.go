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

// SkipTotal on the five searches global search uses (cases, incidents, change
// requests, problems, conversations): the COUNT is not run, the total is
// reported as TotalNotComputed, and the page is exactly the one the same search
// returns without it. Runs the real repositories on a real database and records
// the statements they send. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run SearchSkipTotal

package repository_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// statementRecorder is a pgx tracer that remembers every statement sent, except
// the identity-setting ones Scoped queues ahead of each query.
type statementRecorder struct {
	mu   sync.Mutex
	sqls []string
}

func (r *statementRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	r.add(d.SQL)
	return ctx
}
func (r *statementRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (r *statementRecorder) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return ctx
}
func (r *statementRecorder) TraceBatchQuery(_ context.Context, _ *pgx.Conn, d pgx.TraceBatchQueryData) {
	r.add(d.SQL)
}
func (r *statementRecorder) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

func (r *statementRecorder) add(sql string) {
	if strings.Contains(sql, "set_config") {
		return
	}
	r.mu.Lock()
	r.sqls = append(r.sqls, sql)
	r.mu.Unlock()
}

func (r *statementRecorder) reset() {
	r.mu.Lock()
	r.sqls = nil
	r.mu.Unlock()
}

// counts returns how many COUNT statements were sent.
func (r *statementRecorder) counts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.sqls {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "SELECT COUNT(*)") {
			n++
		}
	}
	return n
}

func tracedPool(t *testing.T) (*pgxpool.Pool, *statementRecorder) {
	t.Helper()
	dsn := os.Getenv("CASE_STATS_TEST_DSN")
	if dsn == "" {
		t.Skip("CASE_STATS_TEST_DSN not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	rec := &statementRecorder{}
	cfg.ConnConfig.Tracer = rec
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, rec
}

// skipTotalSearches runs each of the five searches under the given identity.
// The request is the same for the two modes; only SkipTotal differs.
func skipTotalSearches(scoped *repository.Scoped) []struct {
	name string
	run  func(scope repository.SearchScope, skip bool) (page any, total int, err error)
} {
	page := domain.Pagination{Limit: 5}
	cases := repository.NewCaseRepository(scoped)
	incidents := repository.NewIncidentRepository(scoped)
	changeRequests := repository.NewChangeRequestRepository(scoped)
	problems := repository.NewProblemRepository(scoped)
	conversations := repository.NewConversationRepository(scoped)
	as := func(scope repository.SearchScope) context.Context {
		return repository.WithCallerIdentity(context.Background(), scope)
	}

	return []struct {
		name string
		run  func(scope repository.SearchScope, skip bool) (any, int, error)
	}{
		{"cases", func(scope repository.SearchScope, skip bool) (any, int, error) {
			return cases.SearchCases(as(scope), domain.SearchCasesRequest{
				SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
				Pagination: page, SkipTotal: skip,
			}, scope)
		}},
		{"incidents", func(scope repository.SearchScope, skip bool) (any, int, error) {
			return incidents.SearchIncidents(as(scope), domain.SearchIncidentsRequest{
				SortBy:     domain.IncidentSort{Field: domain.IncidentSortFieldUpdatedOn, Order: domain.IncidentSortOrderDesc},
				Pagination: page, SkipTotal: skip,
			}, nil, nil, nil, nil, nil, nil, nil, nil)
		}},
		{"change requests", func(scope repository.SearchScope, skip bool) (any, int, error) {
			return changeRequests.SearchChangeRequests(as(scope), domain.SearchChangeRequestsRequest{
				SortBy:     domain.ChangeRequestSort{Field: domain.ChangeRequestSortFieldUpdatedOn, Order: domain.ChangeRequestSortOrderDesc},
				Pagination: page, SkipTotal: skip,
			}, nil, nil, nil, nil)
		}},
		{"problems", func(scope repository.SearchScope, skip bool) (any, int, error) {
			return problems.SearchProblems(as(scope), domain.SearchProblemsRequest{Pagination: page, SkipTotal: skip}, nil, nil, nil)
		}},
		{"conversations", func(scope repository.SearchScope, skip bool) (any, int, error) {
			return conversations.SearchConversations(as(scope), domain.SearchConversationsRequest{
				SortBy:     domain.ConversationSort{Field: domain.ConversationSortFieldCreatedOn, Order: domain.ConversationSortOrderDesc},
				Pagination: page, SkipTotal: skip,
			}, "")
		}},
	}
}

// The statements a search sends, for an internal caller and for an external
// one with no projects (the same row-level-security path a customer takes): a
// normal search sends exactly one COUNT, a SkipTotal search sends none and
// reports TotalNotComputed. True on any database, empty or not.
func TestSearchSkipTotalIntegration_StatementsSent(t *testing.T) {
	pool, rec := tracedPool(t)
	identities := []struct {
		name  string
		scope repository.SearchScope
	}{
		{"internal", repository.SearchScope{Unrestricted: true, ViewerEmail: "skip-total-test@wso2.com"}},
		{"external with no projects", repository.SearchScope{ViewerEmail: "skip-total-stranger@test.local"}},
	}

	for _, search := range skipTotalSearches(repository.NewScoped(pool)) {
		for _, id := range identities {
			t.Run(search.name+"/"+id.name, func(t *testing.T) {
				rec.reset()
				if _, total, err := search.run(id.scope, false); err != nil {
					t.Fatalf("search with the count: %v", err)
				} else if total < 0 {
					t.Errorf("a normal search reported total %d", total)
				}
				if n := rec.counts(); n != 1 {
					t.Errorf("a normal search sent %d COUNT statements, want 1", n)
				}

				rec.reset()
				_, total, err := search.run(id.scope, true)
				if err != nil {
					t.Fatalf("search without the count: %v", err)
				}
				if n := rec.counts(); n != 0 {
					t.Errorf("a SkipTotal search sent %d COUNT statements, want 0", n)
				}
				if total != domain.TotalNotComputed {
					t.Errorf("a SkipTotal search reported total %d, want %d", total, domain.TotalNotComputed)
				}
			})
		}
	}
}

// The page is the page: with the flag the rows are exactly those of the same
// search without it. Only meaningful where the database has rows to compare, so
// a resource with none is skipped (reported as skipped, not as proof) instead of
// passing on two empty lists.
func TestSearchSkipTotalIntegration_SamePage(t *testing.T) {
	pool, _ := tracedPool(t)
	internal := repository.SearchScope{Unrestricted: true, ViewerEmail: "skip-total-test@wso2.com"}

	for _, search := range skipTotalSearches(repository.NewScoped(pool)) {
		t.Run(search.name, func(t *testing.T) {
			withTotal, _, err := search.run(internal, false)
			if err != nil {
				t.Fatalf("search with the count: %v", err)
			}
			a, _ := json.Marshal(withTotal)
			if string(a) == "[]" || string(a) == "null" {
				t.Skipf("no %s in this database, so comparing two empty pages would prove nothing", search.name)
			}
			skipped, _, err := search.run(internal, true)
			if err != nil {
				t.Fatalf("search without the count: %v", err)
			}
			b, _ := json.Marshal(skipped)
			if string(a) != string(b) {
				t.Errorf("the page differs with SkipTotal:\n  with count:    %s\n  without count: %s", a, b)
			}
		})
	}
}
