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

// Integration test: PATCH of a project's production-readiness completion date,
// its read-back (detail, search list item) and the project search's new
// onboarding date, closure-state and sort options, on the Postgres data source.
// Runs against a real Postgres, skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ProjectOnboardingReadinessIntegration

package repository_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	porProjectA = "92000000-0000-0000-0000-0000000000c1"
	porProjectB = "92000000-0000-0000-0000-0000000000c2"
)

func TestProjectOnboardingReadinessIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = ANY($1::uuid[])`, []string{porProjectA, porProjectB})
	}
	cleanup()
	t.Cleanup(cleanup)
	for id, key := range map[string]string{porProjectA: "PORPROJA", porProjectB: "PORPROJB"} {
		if _, err := pool.Exec(ctx, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id)
			VALUES ($1, now(), now(), 'por-test', 'por-test', $2::text, $2::text, 'sf-' || $2::text)`, id, key); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}

	repo := repository.NewProjectRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true}
	str := func(s string) *string { return &s }
	pat := func(id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResult, error) {
		return repo.UpdateProject(ctx, id, req, "jane.doe@example.com")
	}
	date := func(d interface{ Format(string) string }) string { return d.Format("2006-01-02") }

	// A: start 03-01, plan 05-01, live 05-15, expiry 2027-03-01, readiness 04-10, state SUSPENDED.
	res, err := pat(porProjectA, domain.ProjectUpdateRequest{
		OnboardingStartDate: str("2026-03-01"), OnboardingGoLivePlanDate: str("2026-05-01"),
		OnboardingGoLiveDate: str("2026-05-15"), OnboardingExpiryDate: str("2027-03-01"),
		OnboardingProductionReadinessCompletionDate: str("2026-04-10"),
	})
	if err != nil {
		t.Fatalf("patch A: %v", err)
	}
	if res.ProductionReadinessCompletionDate == nil || date(res.ProductionReadinessCompletionDate) != "2026-04-10" {
		t.Fatalf("patch result readiness = %v, want 2026-04-10", res.ProductionReadinessCompletionDate)
	}
	// B: start 06-01, readiness 07-01, state OPEN; no other dates.
	if _, err := pat(porProjectB, domain.ProjectUpdateRequest{
		OnboardingStartDate: str("2026-06-01"), OnboardingProductionReadinessCompletionDate: str("2026-07-01"),
	}); err != nil {
		t.Fatalf("patch B: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE project SET wso2_closure_state = 'SUSPENDED' WHERE id = $1`, porProjectA); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE project SET wso2_closure_state = 'OPEN' WHERE id = $1`, porProjectB); err != nil {
		t.Fatal(err)
	}

	// The stored column changed, and the detail read agrees.
	var stored string
	if err := pool.QueryRow(ctx, `SELECT onboarding_production_readiness_completion_date::text FROM project WHERE id = $1`, porProjectA).Scan(&stored); err != nil || stored != "2026-04-10" {
		t.Fatalf("stored column = %q, %v; want 2026-04-10", stored, err)
	}
	got, err := repo.GetProjectByID(ctx, porProjectA, scope)
	if err != nil || got.ProductionReadinessCompletionDate == nil || date(got.ProductionReadinessCompletionDate) != "2026-04-10" {
		t.Fatalf("GetProjectByID readiness = %v, %v", got.ProductionReadinessCompletionDate, err)
	}

	// Omitting the field leaves it; an earlier-than-start date is rejected and leaves it.
	if _, err := pat(porProjectA, domain.ProjectUpdateRequest{OnboardingExpiryDate: str("2027-04-01")}); err != nil {
		t.Fatalf("unrelated patch: %v", err)
	}
	var ve *apierror.ValidationError
	if _, err := pat(porProjectA, domain.ProjectUpdateRequest{OnboardingProductionReadinessCompletionDate: str("2026-02-01")}); !errors.As(err, &ve) {
		t.Fatalf("readiness before stored start: err = %v, want ValidationError", err)
	}
	if err := pool.QueryRow(ctx, `SELECT onboarding_production_readiness_completion_date::text FROM project WHERE id = $1`, porProjectA).Scan(&stored); err != nil || stored != "2026-04-10" {
		t.Fatalf("stored column after unrelated/rejected patches = %q, %v; want 2026-04-10", stored, err)
	}

	// Search: each filter on its own, scoped to the two fixtures by key.
	search := func(req domain.SearchProjectsRequest) []string {
		t.Helper()
		req.SearchQuery = "PORPROJ"
		req.Pagination = domain.Pagination{Limit: 50}
		if req.SortOrder == "" {
			req.SortOrder = "asc"
		}
		projects, _, err := repo.SearchProjects(ctx, req, scope)
		if err != nil {
			t.Fatalf("SearchProjects(%+v): %v", req, err)
		}
		keys := []string{}
		for _, p := range projects {
			keys = append(keys, p.Key)
		}
		return keys
	}
	for name, tc := range map[string]struct {
		req  domain.SearchProjectsRequest
		want []string
	}{
		"start from 04-01":          {domain.SearchProjectsRequest{OnboardingStartDateFrom: "2026-04-01"}, []string{"PORPROJB"}},
		"start to 03-01 inclusive":  {domain.SearchProjectsRequest{OnboardingStartDateTo: "2026-03-01"}, []string{"PORPROJA"}},
		"plan range inclusive":      {domain.SearchProjectsRequest{GoLivePlanDateFrom: "2026-05-01", GoLivePlanDateTo: "2026-05-01"}, []string{"PORPROJA"}},
		"plan range excludes null":  {domain.SearchProjectsRequest{GoLivePlanDateFrom: "2000-01-01"}, []string{"PORPROJA"}},
		"live range":                {domain.SearchProjectsRequest{GoLiveDateFrom: "2026-05-15", GoLiveDateTo: "2026-05-15"}, []string{"PORPROJA"}},
		"expiry to 2027-03-31":      {domain.SearchProjectsRequest{OnboardingExpiryDateTo: "2027-03-31"}, []string{}},
		"expiry from 2027-04-01":    {domain.SearchProjectsRequest{OnboardingExpiryDateFrom: "2027-04-01"}, []string{"PORPROJA"}},
		"readiness from 05-01":      {domain.SearchProjectsRequest{ProductionReadinessCompletionDateFrom: "2026-05-01"}, []string{"PORPROJB"}},
		"readiness to 04-10":        {domain.SearchProjectsRequest{ProductionReadinessCompletionDateTo: "2026-04-10"}, []string{"PORPROJA"}},
		"readiness window both":     {domain.SearchProjectsRequest{ProductionReadinessCompletionDateFrom: "2026-04-01", ProductionReadinessCompletionDateTo: "2026-12-31"}, []string{"PORPROJA", "PORPROJB"}},
		"closureStates Suspended":   {domain.SearchProjectsRequest{ClosureStates: []string{"Suspended"}}, []string{"PORPROJA"}},
		"closureStates any of":      {domain.SearchProjectsRequest{ClosureStates: []string{"Open", "Suspended"}, SortBy: "productionReadinessCompletionDate", SortOrder: "desc"}, []string{"PORPROJB", "PORPROJA"}},
		"closureStates Restricted":  {domain.SearchProjectsRequest{ClosureStates: []string{"Restricted"}}, []string{}},
		"sort plan asc, null last":  {domain.SearchProjectsRequest{SortBy: "goLivePlanDate"}, []string{"PORPROJA", "PORPROJB"}},
		"sort plan desc, null last": {domain.SearchProjectsRequest{SortBy: "goLivePlanDate", SortOrder: "desc"}, []string{"PORPROJA", "PORPROJB"}},
		"sort readiness asc":        {domain.SearchProjectsRequest{SortBy: "productionReadinessCompletionDate"}, []string{"PORPROJA", "PORPROJB"}},
	} {
		if g := search(tc.req); !reflect.DeepEqual(g, tc.want) {
			t.Errorf("%s: keys = %v, want %v", name, g, tc.want)
		}
	}

	// The list item carries the dates.
	req := domain.SearchProjectsRequest{SearchQuery: "PORPROJA", Pagination: domain.Pagination{Limit: 5}, SortOrder: "asc"}
	projects, _, err := repo.SearchProjects(ctx, req, scope)
	if err != nil || len(projects) != 1 {
		t.Fatalf("list A: %v, %d rows", err, len(projects))
	}
	p := projects[0]
	if p.OnboardingStartDate == nil || date(p.OnboardingStartDate) != "2026-03-01" ||
		p.GoLivePlanDate == nil || date(p.GoLivePlanDate) != "2026-05-01" ||
		p.GoLiveDate == nil || date(p.GoLiveDate) != "2026-05-15" ||
		p.OnboardingExpiryDate == nil || date(p.OnboardingExpiryDate) != "2027-04-01" ||
		p.ProductionReadinessCompletionDate == nil || date(p.ProductionReadinessCompletionDate) != "2026-04-10" {
		t.Errorf("list item dates = %+v", p)
	}
}
