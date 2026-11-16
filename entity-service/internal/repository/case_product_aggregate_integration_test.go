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
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// productAggID is the id of the fixture row with that suffix: cases,
// deployed products and products share the prefix, apart by suffix.
func productAggID(suffix string) string {
	return "94000000-0000-0000-0000-0000000000" + suffix
}

// The fixture's products: "PX Alpha" twice (two catalog rows that share a
// name, apart by unit, as the table allows) and "PX Beta". Its cases, all
// created in March 2019 (the tests' createdOn window keeps every other case
// out):
//
//	PA-1  PX Alpha (first row),  open,   high
//	PA-2  PX Alpha (second row), closed, high
//	PA-3  PX Beta,               open,   low
//	PA-4  PX Beta,               open,   high
//	PA-5  PX Beta,               open,   low
//	PA-6  no deployed product,   open,   high
var productAggProducts = []struct{ suffix, name, unit string }{
	{"a1", "PX Alpha", "APIM"},
	{"a2", "PX Alpha", "IAM"},
	{"b1", "PX Beta", "APIM"},
}

var productAggCases = []struct{ suffix, product, state, severity string }{
	{"01", "a1", "OPEN", "S2"},
	{"02", "a2", "CLOSED", "S2"},
	{"03", "b1", "OPEN", "S4"},
	{"04", "b1", "OPEN", "S2"},
	{"05", "b1", "OPEN", "S4"},
	{"06", "", "OPEN", "S2"},
}

// seedProductAggFixture inserts the fixture's products, one deployed product
// per case that has one, and the cases, removing any left over from an
// earlier run first and again when the test ends.
func seedProductAggFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		for _, c := range productAggCases {
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, productAggID(c.suffix))
			_, _ = scoped.Exec(ctx, `DELETE FROM deployed_product WHERE id = $1`, productAggID("d"+c.suffix[1:]))
		}
		for _, p := range productAggProducts {
			_, _ = scoped.Exec(ctx, `DELETE FROM product WHERE id = $1`, productAggID(p.suffix))
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Date(2019, time.March, 10, 12, 0, 0, 0, time.UTC)
	for _, p := range productAggProducts {
		if _, err := scoped.Exec(ctx, `INSERT INTO product (id, created_on, updated_on, created_by, updated_by, category, unit, name)
			VALUES ($1, $2, $2, 'test', 'test', 'SOFTWARE', $3, $4)`, productAggID(p.suffix), now, p.unit, p.name); err != nil {
			t.Fatalf("seed product %s: %v", p.suffix, err)
		}
	}
	for _, c := range productAggCases {
		var deployed any
		if c.product != "" {
			dp := productAggID("d" + c.suffix[1:])
			if _, err := scoped.Exec(ctx, `INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, product_id)
				VALUES ($1, $2, $2, 'test', 'test', $3, $4)`, dp, now, "PA-DP-"+c.suffix, productAggID(c.product)); err != nil {
				t.Fatalf("seed deployed product %s: %v", c.suffix, err)
			}
			deployed = dp
		}
		id := productAggID(c.suffix)
		if _, err := scoped.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, deployed_product_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, 'product aggregate integration test fixture', 'CASE', $5)`,
			id, now, "PA-"+c.suffix, "PA-WSO2-"+c.suffix, deployed); err != nil {
			t.Fatalf("seed work_item %s: %v", c.suffix, err)
		}
		if _, err := scoped.Exec(ctx, `INSERT INTO "case" (id, state, severity) VALUES ($1, $2::case_state_enum, $3::case_severity_enum)`,
			id, c.state, c.severity); err != nil {
			t.Fatalf("seed case %s: %v", c.suffix, err)
		}
	}
}

// productAggRequest is a case search in the fixture's createdOn window with
// the given parsed filters added, sorted as the service defaults it.
func productAggRequest(p domain.ParsedCaseFilters) domain.SearchCasesRequest {
	from, to := time.Date(2019, time.March, 1, 0, 0, 0, 0, time.UTC), time.Date(2019, time.March, 31, 0, 0, 0, 0, time.UTC)
	p.StartCreatedDate, p.EndCreatedDate = &from, &to
	return domain.SearchCasesRequest{Parsed: p, Pagination: domain.Pagination{Limit: 50},
		SortBy: domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc}}
}

// TestProductAggregateIntegration_GroupsByProductName: against a real
// database, grouping by product gives one bucket per product name (two
// catalog rows of one name are one bucket), keyed and labelled with the name
// as the product filter takes it, largest first; a case without a deployed
// product is in no bucket; the search's other filters apply; and each
// bucket's count is the product filter's total for its key.
func TestProductAggregateIntegration_GroupsByProductName(t *testing.T) {
	pool := caseStatsPool(t)
	seedProductAggFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	for _, tc := range []struct {
		name   string
		parsed domain.ParsedCaseFilters
		want   []domain.AggregateBucket
	}{
		{"every case", domain.ParsedCaseFilters{}, []domain.AggregateBucket{
			{Key: "PX Beta", Label: "PX Beta", Count: 3},
			{Key: "PX Alpha", Label: "PX Alpha", Count: 2},
		}},
		{"open high-severity cases", domain.ParsedCaseFilters{States: []domain.CaseState{"open"}, Severities: []domain.CaseSeverity{"high"}}, []domain.AggregateBucket{
			{Key: "PX Alpha", Label: "PX Alpha", Count: 1},
			{Key: "PX Beta", Label: "PX Beta", Count: 1},
		}},
		{"one product's cases", domain.ParsedCaseFilters{ProductNames: []string{"PX Alpha"}}, []domain.AggregateBucket{
			{Key: "PX Alpha", Label: "PX Alpha", Count: 2},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.AggregateCases(ctx, productAggRequest(tc.parsed), "product", scope)
			if err != nil {
				t.Fatalf("AggregateCases: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buckets = %+v, want %+v", got, tc.want)
			}
			for _, b := range got {
				p := tc.parsed
				p.ProductNames = []string{b.Key}
				_, total, err := repo.SearchCases(ctx, productAggRequest(p), scope)
				if err != nil {
					t.Fatalf("SearchCases: %v", err)
				}
				if total != b.Count {
					t.Errorf("bucket %q counts %d, the product filter finds %d", b.Key, b.Count, total)
				}
			}
		})
	}
}

// TestProductAggregateIntegration_LeavesProductsOut: against a real
// database, product notIn leaves out every catalog row of each named product
// and keeps a case without a deployed product; beside a product in filter it
// narrows that; and a grouping by product under it has no bucket for a
// product left out.
func TestProductAggregateIntegration_LeavesProductsOut(t *testing.T) {
	pool := caseStatsPool(t)
	seedProductAggFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	for _, tc := range []struct {
		name   string
		parsed domain.ParsedCaseFilters
		want   int
	}{
		{"one product left out", domain.ParsedCaseFilters{ExcludeProductNames: []string{"PX Alpha"}}, 4},
		{"every product left out", domain.ParsedCaseFilters{ExcludeProductNames: []string{"PX Alpha", "PX Beta"}}, 1},
		{"beside a product kept", domain.ParsedCaseFilters{ProductNames: []string{"PX Alpha", "PX Beta"}, ExcludeProductNames: []string{"PX Alpha"}}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, total, err := repo.SearchCases(ctx, productAggRequest(tc.parsed), scope)
			if err != nil {
				t.Fatalf("SearchCases: %v", err)
			}
			if total != tc.want {
				t.Errorf("total = %d, want %d", total, tc.want)
			}
		})
	}

	got, err := repo.AggregateCases(ctx, productAggRequest(domain.ParsedCaseFilters{ExcludeProductNames: []string{"PX Alpha"}}), "product", scope)
	if err != nil {
		t.Fatalf("AggregateCases: %v", err)
	}
	if want := []domain.AggregateBucket{{Key: "PX Beta", Label: "PX Beta", Count: 3}}; !reflect.DeepEqual(got, want) {
		t.Errorf("buckets = %+v, want %+v", got, want)
	}
}
