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

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type stubAccess struct {
	scope AccessScope
	err   error
}

func (s stubAccess) ResolveScope(context.Context) (AccessScope, error) { return s.scope, s.err }

type recordedSearch struct {
	scope  repository.SearchScope
	query  string
	sort   repository.SearchSortField
	desc   bool
	limit  int
	offset int
}

type fakeSearchRepo struct {
	projects, cases *recordedSearch
}

func (f *fakeSearchRepo) SearchProjects(_ context.Context, sc repository.SearchScope, q string, s repository.SearchSortField, d bool, p domain.Pagination) ([]domain.GlobalSearchProject, int, error) {
	f.projects = &recordedSearch{sc, q, s, d, p.Limit, p.Offset}
	return []domain.GlobalSearchProject{{ID: "p"}}, 7, nil
}

func (f *fakeSearchRepo) SearchCases(_ context.Context, sc repository.SearchScope, q string, s repository.SearchSortField, d bool, p domain.Pagination) ([]domain.GlobalSearchCase, int, error) {
	f.cases = &recordedSearch{sc, q, s, d, p.Limit, p.Offset}
	return []domain.GlobalSearchCase{{ID: "c"}}, 9, nil
}

// unusedReferenceDataRepo satisfies the reference-data dependency; global
// search never touches it.
type unusedReferenceDataRepo struct{}

func (unusedReferenceDataRepo) ListProjectTypes(context.Context) ([]repository.ProjectTypeRow, error) {
	return nil, nil
}
func (unusedReferenceDataRepo) GetProjectByID(context.Context, string) (bool, *repository.ProjectTypeRow, error) {
	return false, nil, nil
}
func (unusedReferenceDataRepo) EnumLabels(context.Context, []string) (map[string][]string, error) {
	return nil, nil
}

func newGlobal(acc AccessService, search *fakeSearchRepo) GlobalService {
	return NewGlobalService(unusedReferenceDataRepo{}, search, acc)
}

var customerScope = stubAccess{scope: AccessScope{ProjectIDs: []string{"p1", "p2"}}}

func TestGlobalSearch_AccessDecidesBeforeAnythingIsQueried(t *testing.T) {
	repo := &fakeSearchRepo{}
	denied := &apierror.ForbiddenError{Msg: "no access"}
	_, err := newGlobal(stubAccess{err: denied}, repo).GlobalSearch(context.Background(), domain.GlobalSearchRequest{})
	if !errors.Is(err, denied) {
		t.Fatalf("got %v, want the access error", err)
	}
	if repo.projects != nil || repo.cases != nil {
		t.Fatal("a search ran even though access was refused")
	}
}

func TestGlobalSearch_PassesScopeAndQueryToBothTables(t *testing.T) {
	repo := &fakeSearchRepo{}
	resp, err := newGlobal(customerScope, repo).GlobalSearch(context.Background(), domain.GlobalSearchRequest{
		Filters: &domain.GlobalSearchFilters{SearchQuery: "ssl"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]*recordedSearch{"projects": repo.projects, "cases": repo.cases} {
		if got == nil || got.query != "ssl" || got.scope.Unrestricted || strings.Join(got.scope.ProjectIDs, ",") != "p1,p2" {
			t.Errorf("%s search got %+v", name, got)
		}
		if got != nil && (got.limit != 20 || got.offset != 0) {
			t.Errorf("%s default pagination = %d/%d, want 20/0", name, got.limit, got.offset)
		}
	}
	if resp.Query != "ssl" || resp.ProjectsTotal != 7 || resp.CasesTotal != 9 || len(resp.Projects) != 1 || len(resp.Cases) != 1 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestGlobalSearch_UnrestrictedScopeIsForwarded(t *testing.T) {
	repo := &fakeSearchRepo{}
	_, err := newGlobal(stubAccess{scope: AccessScope{Unrestricted: true}}, repo).GlobalSearch(context.Background(), domain.GlobalSearchRequest{})
	if err != nil || repo.projects == nil || !repo.projects.scope.Unrestricted {
		t.Fatalf("scope not forwarded: %+v %v", repo.projects, err)
	}
}

func TestGlobalSearch_TablesFilter(t *testing.T) {
	repo := &fakeSearchRepo{}
	resp, err := newGlobal(customerScope, repo).GlobalSearch(context.Background(), domain.GlobalSearchRequest{
		Filters: &domain.GlobalSearchFilters{Tables: []string{"projects"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.cases != nil || repo.projects == nil {
		t.Fatalf("only projects should be searched: projects=%v cases=%v", repo.projects, repo.cases)
	}
	if resp.Cases == nil || len(resp.Cases) != 0 || resp.CasesTotal != 0 {
		t.Fatalf("unsearched table must serialize as an empty list, got %+v", resp.Cases)
	}
}

func TestGlobalSearch_Sorting(t *testing.T) {
	sortOf := func(s *domain.GlobalSearchSort) (proj, cs *recordedSearch) {
		repo := &fakeSearchRepo{}
		if _, err := newGlobal(customerScope, repo).GlobalSearch(context.Background(), domain.GlobalSearchRequest{SortBy: s}); err != nil {
			t.Fatal(err)
		}
		return repo.projects, repo.cases
	}
	p, c := sortOf(nil)
	if p.sort != repository.SearchSortName || p.desc || c.sort != repository.SearchSortUpdatedOn || !c.desc {
		t.Errorf("defaults: projects %+v cases %+v", p, c)
	}
	p, c = sortOf(&domain.GlobalSearchSort{Field: "name"})
	if p.sort != repository.SearchSortName || p.desc || c.sort != repository.SearchSortName || c.desc {
		t.Errorf("name defaults to asc for both: %+v %+v", p, c)
	}
	p, c = sortOf(&domain.GlobalSearchSort{Field: "createdOn"})
	if p.sort != repository.SearchSortCreatedOn || !p.desc || !c.desc {
		t.Errorf("date field defaults to desc: %+v %+v", p, c)
	}
	p, c = sortOf(&domain.GlobalSearchSort{Field: "updatedOn", Order: "asc"})
	if p.desc || c.desc || p.sort != repository.SearchSortUpdatedOn {
		t.Errorf("explicit order wins: %+v %+v", p, c)
	}
	p, c = sortOf(&domain.GlobalSearchSort{Order: "desc"})
	if !p.desc || !c.desc || p.sort != repository.SearchSortName || c.sort != repository.SearchSortUpdatedOn {
		t.Errorf("order alone keeps each table's default field: %+v %+v", p, c)
	}
	p, c = sortOf(&domain.GlobalSearchSort{Order: "asc"})
	if p.desc || c.desc {
		t.Errorf("order alone (asc) must reach both tables: %+v %+v", p, c)
	}
}

func TestGlobalSearch_Validation(t *testing.T) {
	bad := map[string]domain.GlobalSearchRequest{
		"invalid table":       {Filters: &domain.GlobalSearchFilters{Tables: []string{"users"}}},
		"query too long":      {Filters: &domain.GlobalSearchFilters{SearchQuery: strings.Repeat("x", 201)}},
		"invalid sort order":  {SortBy: &domain.GlobalSearchSort{Order: "sideways"}},
		"invalid sort field":  {SortBy: &domain.GlobalSearchSort{Field: "password"}},
		"projects limit > 50": {ProjectsPagination: &domain.Pagination{Limit: 51}},
		"cases limit > 50":    {CasesPagination: &domain.Pagination{Limit: 51}},
	}
	for name, req := range bad {
		repo := &fakeSearchRepo{}
		_, err := newGlobal(customerScope, repo).GlobalSearch(context.Background(), req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: got %v, want a ValidationError", name, err)
		}
		if repo.projects != nil || repo.cases != nil {
			t.Errorf("%s: a search ran despite invalid input", name)
		}
	}
}
