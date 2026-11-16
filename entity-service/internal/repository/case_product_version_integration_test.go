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

// productVersionID is the id of the fixture row with that suffix: cases,
// deployed products, product versions and products share the prefix, apart
// by suffix.
func productVersionID(suffix string) string {
	return "95000000-0000-0000-0000-0000000000" + suffix
}

// The fixture's products are "PV Alpha" and "PV Beta". PV Alpha has version
// 1.0.0 twice (two deployment profiles, as the table allows) and 2.0.0; PV
// Beta has 1.0.0 too. Its cases, all created in April 2019 (the tests'
// createdOn window keeps every other case out):
//
//	PV-1  PV Alpha 1.0.0 (default profile), open
//	PV-2  PV Alpha 1.0.0 (lite profile),    closed
//	PV-3  PV Alpha 2.0.0,                   open
//	PV-4  PV Alpha, no version,             open
//	PV-5  PV Beta 1.0.0,                    open
//	PV-6  PV Beta 1.0.0,                    open
var pvProducts = []struct{ suffix, name string }{
	{"a1", "PV Alpha"},
	{"b1", "PV Beta"},
}

var pvVersions = []struct{ suffix, product, version, profile string }{
	{"e1", "a1", "1.0.0", "default"},
	{"e2", "a1", "1.0.0", "lite"},
	{"e3", "a1", "2.0.0", "default"},
	{"e4", "b1", "1.0.0", "default"},
}

var pvCases = []struct{ suffix, product, version, state string }{
	{"01", "a1", "e1", "OPEN"},
	{"02", "a1", "e2", "CLOSED"},
	{"03", "a1", "e3", "OPEN"},
	{"04", "a1", "", "OPEN"},
	{"05", "b1", "e4", "OPEN"},
	{"06", "b1", "e4", "OPEN"},
}

// seedProductVersionFixture inserts the fixture's products, versions, one
// deployed product per case and the cases, removing any left over from an
// earlier run first and again when the test ends.
func seedProductVersionFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		for _, c := range pvCases {
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, productVersionID(c.suffix))
			_, _ = scoped.Exec(ctx, `DELETE FROM deployed_product WHERE id = $1`, productVersionID("d"+c.suffix[1:]))
		}
		for _, v := range pvVersions {
			_, _ = scoped.Exec(ctx, `DELETE FROM product_version WHERE id = $1`, productVersionID(v.suffix))
		}
		for _, p := range pvProducts {
			_, _ = scoped.Exec(ctx, `DELETE FROM product WHERE id = $1`, productVersionID(p.suffix))
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Date(2019, time.April, 10, 12, 0, 0, 0, time.UTC)
	for _, p := range pvProducts {
		if _, err := scoped.Exec(ctx, `INSERT INTO product (id, created_on, updated_on, created_by, updated_by, category, unit, name)
			VALUES ($1, $2, $2, 'test', 'test', 'SOFTWARE', 'APIM', $3)`, productVersionID(p.suffix), now, p.name); err != nil {
			t.Fatalf("seed product %s: %v", p.suffix, err)
		}
	}
	for _, v := range pvVersions {
		if _, err := scoped.Exec(ctx, `INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id, deployment_profile)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, $5)`, productVersionID(v.suffix), now, v.version, productVersionID(v.product), v.profile); err != nil {
			t.Fatalf("seed product version %s: %v", v.suffix, err)
		}
	}
	for _, c := range pvCases {
		var version any
		if c.version != "" {
			version = productVersionID(c.version)
		}
		dp := productVersionID("d" + c.suffix[1:])
		if _, err := scoped.Exec(ctx, `INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, product_id, version_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, $5)`, dp, now, "PV-DP-"+c.suffix, productVersionID(c.product), version); err != nil {
			t.Fatalf("seed deployed product %s: %v", c.suffix, err)
		}
		id := productVersionID(c.suffix)
		if _, err := scoped.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, deployed_product_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, 'product version integration test fixture', 'CASE', $5)`,
			id, now, "PV-"+c.suffix, "PV-WSO2-"+c.suffix, dp); err != nil {
			t.Fatalf("seed work_item %s: %v", c.suffix, err)
		}
		if _, err := scoped.Exec(ctx, `INSERT INTO "case" (id, state, severity) VALUES ($1, $2::case_state_enum, 'S3'::case_severity_enum)`,
			id, c.state); err != nil {
			t.Fatalf("seed case %s: %v", c.suffix, err)
		}
	}
}

// productVersionRequest is a case search in the fixture's createdOn window
// with the given parsed filters added, sorted as the service defaults it.
func productVersionRequest(p domain.ParsedCaseFilters) domain.SearchCasesRequest {
	from, to := time.Date(2019, time.April, 1, 0, 0, 0, 0, time.UTC), time.Date(2019, time.April, 30, 0, 0, 0, 0, time.UTC)
	p.StartCreatedDate, p.EndCreatedDate = &from, &to
	return domain.SearchCasesRequest{Parsed: p, Pagination: domain.Pagination{Limit: 50},
		SortBy: domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc}}
}

// TestProductVersionIntegration_FiltersByVersion: against a real database,
// the version filter keeps the cases whose deployed product has one of the
// versions, within the product filter's product: another product's case of
// the same version is out, both rows of a version shared by two deployment
// profiles are in, and a case without a version is never in.
func TestProductVersionIntegration_FiltersByVersion(t *testing.T) {
	pool := caseStatsPool(t)
	seedProductVersionFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	for _, tc := range []struct {
		name     string
		product  string
		versions []string
		want     []string
	}{
		{"one version, two profiles", "PV Alpha", []string{"1.0.0"}, []string{"PV-01", "PV-02"}},
		{"two versions", "PV Alpha", []string{"1.0.0", "2.0.0"}, []string{"PV-01", "PV-02", "PV-03"}},
		{"the same version of another product", "PV Beta", []string{"1.0.0"}, []string{"PV-05", "PV-06"}},
		{"a version the product does not have", "PV Beta", []string{"2.0.0"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			views, total, err := repo.SearchCases(ctx, productVersionRequest(domain.ParsedCaseFilters{
				ProductNames: []string{tc.product}, ProductVersions: tc.versions}), scope)
			if err != nil {
				t.Fatalf("SearchCases: %v", err)
			}
			got := map[string]bool{}
			for _, v := range views {
				got[v.Number] = true
			}
			want := map[string]bool{}
			for _, n := range tc.want {
				want[n] = true
			}
			if total != len(tc.want) || !reflect.DeepEqual(got, want) {
				t.Errorf("total %d, cases %v; want %v", total, got, tc.want)
			}
		})
	}
}

// TestProductVersionIntegration_GroupsByVersion: grouping by version within
// one product gives one bucket per version string (two deployment profiles of
// one version are one bucket), largest first; a case without a version is in
// no bucket; another product's cases of the same version are not counted; and
// each bucket's count is the version filter's total for its key.
func TestProductVersionIntegration_GroupsByVersion(t *testing.T) {
	pool := caseStatsPool(t)
	seedProductVersionFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	for _, tc := range []struct {
		name   string
		parsed domain.ParsedCaseFilters
		want   []domain.AggregateBucket
	}{
		{"every PV Alpha case", domain.ParsedCaseFilters{ProductNames: []string{"PV Alpha"}}, []domain.AggregateBucket{
			{Key: "1.0.0", Label: "1.0.0", Count: 2},
			{Key: "2.0.0", Label: "2.0.0", Count: 1},
		}},
		{"open PV Alpha cases", domain.ParsedCaseFilters{ProductNames: []string{"PV Alpha"}, States: []domain.CaseState{"open"}}, []domain.AggregateBucket{
			{Key: "1.0.0", Label: "1.0.0", Count: 1},
			{Key: "2.0.0", Label: "2.0.0", Count: 1},
		}},
		{"PV Beta", domain.ParsedCaseFilters{ProductNames: []string{"PV Beta"}}, []domain.AggregateBucket{
			{Key: "1.0.0", Label: "1.0.0", Count: 2},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.AggregateCases(ctx, productVersionRequest(tc.parsed), "productVersion", scope)
			if err != nil {
				t.Fatalf("AggregateCases: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buckets = %+v, want %+v", got, tc.want)
			}
			for _, b := range got {
				p := tc.parsed
				p.ProductVersions = []string{b.Key}
				_, total, err := repo.SearchCases(ctx, productVersionRequest(p), scope)
				if err != nil {
					t.Fatalf("SearchCases: %v", err)
				}
				if total != b.Count {
					t.Errorf("bucket %q counts %d, the version filter finds %d", b.Key, b.Count, total)
				}
			}
		})
	}
}
