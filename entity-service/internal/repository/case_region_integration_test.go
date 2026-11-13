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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// regionID is the id of the fixture row with that suffix: accounts and cases
// share the prefix, apart by suffix.
func regionID(suffix string) string {
	return "97000000-0000-0000-0000-0000000000" + suffix
}

// The fixture's accounts and their regions: "APAC" twice (once written
// " apac ", as a hand-typed record may hold it), "EMEA " (with a trailing
// space), none, and a blank one. Its cases,
// all created in March 2019 (the tests' createdOn window keeps every other
// case out):
//
//	RG-1  account a1 (APAC)
//	RG-7  account a1 (APAC)
//	RG-2  account a2 (" apac ")
//	RG-3  account a3 ("EMEA ")
//	RG-4  account a4 (no region)
//	RG-5  no account
//	RG-6  account a5 (blank region)
var regionAccounts = []struct {
	suffix string
	region any
}{
	{"a1", "APAC"},
	{"a2", " apac "},
	{"a3", "EMEA "},
	{"a4", nil},
	{"a5", "  "},
}

var regionCases = []struct{ suffix, account string }{
	{"01", "a1"},
	{"02", "a2"},
	{"03", "a3"},
	{"04", "a4"},
	{"05", ""},
	{"06", "a5"},
	{"07", "a1"},
}

// seedRegionFixture inserts the fixture's accounts and cases, removing any
// left over from an earlier run first and again when the test ends.
func seedRegionFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		for _, c := range regionCases {
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, regionID(c.suffix))
		}
		for _, a := range regionAccounts {
			_, _ = scoped.Exec(ctx, `DELETE FROM account WHERE id = $1`, regionID(a.suffix))
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Date(2019, time.March, 10, 12, 0, 0, 0, time.UTC)
	for _, a := range regionAccounts {
		if _, err := scoped.Exec(ctx, `INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, life_cycle, region)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, $5, 'Customer', $6)`,
			regionID(a.suffix), now, "Region Test "+a.suffix, "RG-ACC-"+a.suffix, "sf-rg-"+a.suffix, a.region); err != nil {
			t.Fatalf("seed account %s: %v", a.suffix, err)
		}
	}
	for _, c := range regionCases {
		var account any
		if c.account != "" {
			account = regionID(c.account)
		}
		id := regionID(c.suffix)
		if _, err := scoped.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, 'region integration test fixture', 'CASE', $5)`,
			id, now, "RG-"+c.suffix, "RG-WSO2-"+c.suffix, account); err != nil {
			t.Fatalf("seed work_item %s: %v", c.suffix, err)
		}
		if _, err := scoped.Exec(ctx, `INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN'::case_state_enum, 'S2'::case_severity_enum)`, id); err != nil {
			t.Fatalf("seed case %s: %v", c.suffix, err)
		}
	}
}

// regionRequest is a case search in the fixture's createdOn window with the
// given parsed filters added.
func regionRequest(p domain.ParsedCaseFilters) domain.SearchCasesRequest {
	from, to := time.Date(2019, time.March, 1, 0, 0, 0, 0, time.UTC), time.Date(2019, time.March, 31, 0, 0, 0, 0, time.UTC)
	p.StartCreatedDate, p.EndCreatedDate = &from, &to
	return domain.SearchCasesRequest{Parsed: p, Pagination: domain.Pagination{Limit: 50},
		SortBy: domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc}}
}

// TestRegionIntegration_FiltersByAccountRegion: against a real database,
// region in matches the account's region ignoring case and surrounding
// spaces, on either side; notIn keeps a case whose account has no or a blank
// region and a case without an account; the two combine; and an aggregate under a
// region filter counts what the search finds.
func TestRegionIntegration_FiltersByAccountRegion(t *testing.T) {
	pool := caseStatsPool(t)
	seedRegionFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	for _, tc := range []struct {
		name   string
		parsed domain.ParsedCaseFilters
		want   int
	}{
		{"every case", domain.ParsedCaseFilters{}, 7},
		{"one region", domain.ParsedCaseFilters{Regions: []string{"APAC"}}, 3},
		{"one region as typed differently", domain.ParsedCaseFilters{Regions: []string{" Apac"}}, 3},
		{"two regions", domain.ParsedCaseFilters{Regions: []string{"apac", "EMEA"}}, 4},
		{"a region no account has", domain.ParsedCaseFilters{Regions: []string{"LATAM"}}, 0},
		{"one region left out", domain.ParsedCaseFilters{ExcludeRegions: []string{"apac"}}, 4},
		{"every region left out", domain.ParsedCaseFilters{ExcludeRegions: []string{"APAC", "EMEA"}}, 3},
		{"kept and left out together", domain.ParsedCaseFilters{Regions: []string{"APAC", "EMEA"}, ExcludeRegions: []string{"EMEA"}}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, total, err := repo.SearchCases(ctx, regionRequest(tc.parsed), scope)
			if err != nil {
				t.Fatalf("SearchCases: %v", err)
			}
			if total != tc.want {
				t.Errorf("total = %d, want %d", total, tc.want)
			}
		})
	}

	got, err := repo.AggregateCases(ctx, regionRequest(domain.ParsedCaseFilters{Regions: []string{"APAC"}}), "account", scope)
	if err != nil {
		t.Fatalf("AggregateCases: %v", err)
	}
	sum := 0
	for _, b := range got {
		sum += b.Count
	}
	if len(got) != 2 || sum != 3 {
		t.Errorf("by account under region APAC: %+v, want the two APAC accounts with three cases", got)
	}
}

// TestRegionIntegration_GroupsByAccountRegion: against a real database, a
// region grouping has one bucket per region as the filter compares it (APAC
// and " apac " are one), labelled as most of its cases' accounts write it; a case without an
// account, a region or with a blank one is in no bucket; and each bucket's
// count is the total of a region filter on its key.
func TestRegionIntegration_GroupsByAccountRegion(t *testing.T) {
	pool := caseStatsPool(t)
	seedRegionFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	got, err := repo.AggregateCases(ctx, regionRequest(domain.ParsedCaseFilters{}), "region", scope)
	if err != nil {
		t.Fatalf("AggregateCases: %v", err)
	}
	if len(got) != 2 || got[0].Key != "apac" || got[0].Count != 3 || got[1].Key != "emea" || got[1].Count != 1 {
		t.Fatalf("by region: %+v, want apac 3 and emea 1", got)
	}
	if got[0].Label != "APAC" {
		t.Errorf("apac label = %q, want APAC, as two of its three cases' account writes it", got[0].Label)
	}
	if got[1].Label != "EMEA" {
		t.Errorf("emea label = %q, want EMEA, without the space", got[1].Label)
	}
	for _, b := range got {
		_, total, err := repo.SearchCases(ctx, regionRequest(domain.ParsedCaseFilters{Regions: []string{b.Key}}), scope)
		if err != nil {
			t.Fatalf("SearchCases: %v", err)
		}
		if total != b.Count {
			t.Errorf("region %q: search total %d, bucket count %d", b.Key, total, b.Count)
		}
	}

	got, err = repo.AggregateCases(ctx, regionRequest(domain.ParsedCaseFilters{ExcludeRegions: []string{"EMEA"}}), "region", scope)
	if err != nil {
		t.Fatalf("AggregateCases: %v", err)
	}
	if len(got) != 1 || got[0].Key != "apac" || got[0].Count != 3 {
		t.Errorf("by region without EMEA: %+v, want apac 3 only", got)
	}
}
