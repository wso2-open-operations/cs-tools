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
// excludeFields=["description"] on case search: the page query never names the
// description column, rows come back with a nil Description, and everything
// else about the page is unchanged. A search without it still reads the column.
// Runs the real repository on a real database and records the statements it
// sends. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ExcludeDescription

package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestSearchCasesExcludeDescriptionIntegration(t *testing.T) {
	pool, rec := tracedPool(t)
	cases := repository.NewCaseRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true, ViewerEmail: "exclude-description-test@example.com"}
	ctx := repository.WithCallerIdentity(context.Background(), scope)
	req := domain.SearchCasesRequest{
		SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
		Pagination: domain.Pagination{Limit: 5},
	}

	rec.reset()
	full, wantTotal, err := cases.SearchCases(ctx, req, scope)
	if err != nil {
		t.Fatalf("plain search: %v", err)
	}
	pageSQL := rec.nonCountStatements()
	if len(pageSQL) != 1 || !strings.Contains(pageSQL[0], "wi.description") {
		t.Errorf("a plain search must read wi.description, statements: %v", pageSQL)
	}

	rec.reset()
	req.ExcludeFields = []string{domain.SearchExcludeFieldDescription}
	slim, gotTotal, err := cases.SearchCases(ctx, req, scope)
	if err != nil {
		t.Fatalf("excluding search: %v", err)
	}
	pageSQL = rec.nonCountStatements()
	if len(pageSQL) != 1 || strings.Contains(pageSQL[0], "wi.description") {
		t.Errorf("an excluding search must not read wi.description, statements: %v", pageSQL)
	}
	if gotTotal != wantTotal || len(slim) != len(full) {
		t.Fatalf("excluding changed the page: total %d vs %d, rows %d vs %d", gotTotal, wantTotal, len(slim), len(full))
	}
	for i := range slim {
		if slim[i].Description != nil {
			t.Errorf("row %d: description must be nil when excluded", i)
		}
		if slim[i].ID != full[i].ID || slim[i].Number != full[i].Number {
			t.Errorf("row %d: identity differs between the two searches", i)
		}
	}
}
