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

// SearchCases must expose a case's resolved and closed times, and return
// null for each when the column is NULL. Runs against a real Postgres.
// Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run SearchCasesTimestampsIntegration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestSearchCasesTimestampsIntegration_ResolvedAndClosedOn(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	creator := "sct-fixture-creator-" + suffix + "@test.local"
	now := time.Now().UTC().Truncate(time.Second)
	resolved := now.Add(-2 * time.Hour)
	closed := now.Add(-time.Hour)

	seed := func(tag string, resolvedOn, closedOn *time.Time) string {
		var id string
		if err := scoped.QueryRow(ctx,
			`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
			 VALUES (gen_random_uuid(), $1, $1, $2, $2, $3, $4, 'search timestamps fixture', 'CASE')
			 RETURNING id`,
			now, creator, "SCT-"+tag+"-"+suffix, "SCT-WSO2-"+tag+"-"+suffix).Scan(&id); err != nil {
			t.Fatalf("seed work_item: %v", err)
		}
		t.Cleanup(func() { _, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, id) })
		if _, err := scoped.Exec(ctx,
			`INSERT INTO "case" (id, state, resolved_on, closed_on) VALUES ($1, 'OPEN', $2, $3)`,
			id, resolvedOn, closedOn); err != nil {
			t.Fatalf("seed case: %v", err)
		}
		return id
	}
	setID := seed("SET", &resolved, &closed)
	nullID := seed("NUL", nil, nil)

	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true, ViewerEmail: "sct-viewer@wso2.com"}
	req := domain.SearchCasesRequest{
		Parsed:     domain.ParsedCaseFilters{CreatedBy: []string{creator}},
		SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
		Pagination: domain.Pagination{Limit: 10},
	}
	results, _, err := repo.SearchCases(repository.WithCallerIdentity(context.Background(), scope), req, scope)
	if err != nil {
		t.Fatalf("SearchCases: %v", err)
	}
	byID := map[string]domain.SearchCaseView{}
	for _, r := range results {
		byID[r.ID] = r
	}
	got, ok := byID[setID]
	if !ok {
		t.Fatalf("case %s missing from results", setID)
	}
	if got.ResolvedOn == nil || *got.ResolvedOn != resolved.Format(time.RFC3339) {
		t.Errorf("ResolvedOn = %v, want %s", got.ResolvedOn, resolved.Format(time.RFC3339))
	}
	if got.ClosedOn == nil || *got.ClosedOn != closed.Format(time.RFC3339) {
		t.Errorf("ClosedOn = %v, want %s", got.ClosedOn, closed.Format(time.RFC3339))
	}
	empty, ok := byID[nullID]
	if !ok {
		t.Fatalf("case %s missing from results", nullID)
	}
	if empty.ResolvedOn != nil || empty.ClosedOn != nil {
		t.Errorf("NULL columns: ResolvedOn=%v ClosedOn=%v, want nil/nil", empty.ResolvedOn, empty.ClosedOn)
	}
}
