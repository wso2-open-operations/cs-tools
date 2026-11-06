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

// Regression test: SearchCases on Postgres returned "account": null for every
// row, even though caseSearchJoins already joined account and its CRE/SRE
// groups, because the page query never selected those columns. A dashboard
// widget grouping search rows by account therefore lumped every case into one
// "No account" bucket. Runs against a real Postgres. Skipped without
// CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run SearchCasesAccountIntegration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestSearchCasesAccountIntegration_AccountPopulatedAndNilWhenAbsent(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	createdBy := "sca-fixture-creator-" + suffix + "@test.local"
	now := time.Now().UTC()

	seedGroup := func(name string) string {
		t.Helper()
		var id string
		if err := scoped.QueryRow(ctx,
			`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name)
			 VALUES (gen_random_uuid(), $1, $1, $2, $2, $3) RETURNING id`,
			now, createdBy, name).Scan(&id); err != nil {
			t.Fatalf("seed group %q: %v", name, err)
		}
		t.Cleanup(func() { _, _ = scoped.Exec(ctx, `DELETE FROM "group" WHERE id = $1`, id) })
		return id
	}
	creName := "SCA CRE Team " + suffix
	sreName := "SCA SRE Team " + suffix
	creID := seedGroup(creName)
	sreID := seedGroup(sreName)

	accountName := "SCA Account " + suffix
	var accountID string
	if err := scoped.QueryRow(ctx,
		`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id,
		                      support_tier, cre_team_id, sre_team_id)
		 VALUES (gen_random_uuid(), $1, $1, $2, $2, $3, $4, $5, 'ENTERPRISE', $6, $7) RETURNING id`,
		now, createdBy, accountName, "SCA-"+suffix, "sf-sca-"+suffix, creID, sreID).Scan(&accountID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	t.Cleanup(func() { _, _ = scoped.Exec(ctx, `DELETE FROM account WHERE id = $1`, accountID) })

	seedCase := func(tag string, account any) string {
		t.Helper()
		var id string
		if err := scoped.QueryRow(ctx,
			`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id)
			 VALUES (gen_random_uuid(), $1, $1, $2, $2, $3, $4, 'search account integration test fixture', 'CASE', $5)
			 RETURNING id`,
			now, createdBy, "SCA-TEST-"+tag+"-"+suffix, "SCA-TEST-WSO2-"+tag+"-"+suffix, account).Scan(&id); err != nil {
			t.Fatalf("seed work_item %s: %v", tag, err)
		}
		t.Cleanup(func() { _, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, id) })
		if _, err := scoped.Exec(ctx, `INSERT INTO "case" (id, state) VALUES ($1, 'OPEN')`, id); err != nil {
			t.Fatalf("seed case %s: %v", tag, err)
		}
		return id
	}
	withAccountID := seedCase("ACC", accountID)
	noAccountID := seedCase("NOACC", nil)

	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true, ViewerEmail: "sca-viewer@example.com"}
	searchCtx := repository.WithCallerIdentity(context.Background(), scope)

	req := domain.SearchCasesRequest{
		Parsed:     domain.ParsedCaseFilters{CreatedBy: []string{createdBy}},
		SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
		Pagination: domain.Pagination{Limit: 5},
	}
	results, total, err := repo.SearchCases(searchCtx, req, scope)
	if err != nil {
		t.Fatalf("SearchCases: %v", err)
	}
	if total != 2 || len(results) != 2 {
		t.Fatalf("SearchCases returned %d result(s) (total=%d), want exactly 2", len(results), total)
	}

	byID := map[string]domain.SearchCaseView{}
	for _, r := range results {
		byID[r.ID] = r
	}

	got, ok := byID[withAccountID]
	if !ok {
		t.Fatalf("case with account %q missing from results", withAccountID)
	}
	if got.AccountDetails == nil {
		t.Fatalf("AccountDetails = nil, want account %q", accountID)
	}
	a := got.AccountDetails
	if a.ID != accountID || a.Name != accountName || a.Type != "ENTERPRISE" {
		t.Errorf("AccountDetails = {ID:%q Name:%q Type:%q}, want {ID:%q Name:%q Type:%q}",
			a.ID, a.Name, a.Type, accountID, accountName, "ENTERPRISE")
	}
	if a.CreTeam == nil || a.CreTeam.ID != creID || a.CreTeam.Name != creName {
		t.Errorf("AccountDetails.CreTeam = %+v, want {ID:%q Name:%q}", a.CreTeam, creID, creName)
	}
	if a.SreTeam == nil || a.SreTeam.ID != sreID || a.SreTeam.Name != sreName {
		t.Errorf("AccountDetails.SreTeam = %+v, want {ID:%q Name:%q}", a.SreTeam, sreID, sreName)
	}

	none, ok := byID[noAccountID]
	if !ok {
		t.Fatalf("case without account %q missing from results", noAccountID)
	}
	if none.AccountDetails != nil {
		t.Errorf("AccountDetails = %+v, want nil for a case with no account", *none.AccountDetails)
	}
}
