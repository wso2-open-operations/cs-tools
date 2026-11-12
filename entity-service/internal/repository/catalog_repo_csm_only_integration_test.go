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

// Integration test for catalog_item.csm_only (migration 0291): an item marked
// csm_only is offered by SearchCatalogs, and its questions are returned by
// GetCatalogItemVariables, only when includeCSMOnly is true (WSO2 staff in
// the CSM portal). A category whose only item is csm_only disappears too.
// IsCatalogItemCSMOnly reads the flag. Same DSN and skip-when-unset pattern
// as the other repository integration tests:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TestCatalogRepository_CSMOnly

package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	csmOnlyProjectID    = "39111111-1111-1111-1111-111111111111"
	csmOnlyDeploymentID = "39222222-0000-0000-0000-000000000001"
	csmOnlyProductID    = "39333333-0000-0000-0000-000000000001"
	csmOnlyDeployedID   = "39444444-0000-0000-0000-000000000001"
	// csmOnlySharedCatID holds one ordinary item and one staff-only item.
	csmOnlySharedCatID = "39555555-0000-0000-0000-000000000001"
	// csmOnlyStaffCatID holds only the staff-only item.
	csmOnlyStaffCatID   = "39555555-0000-0000-0000-000000000002"
	csmOnlyPublicItemID = "39666666-0000-0000-0000-000000000001"
	csmOnlyStaffItemID  = "39666666-0000-0000-0000-000000000002"
)

func seedCSMOnlyCatalogFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM sr_category_routing_rule WHERE catalog_item_id IN ($1, $2)`, csmOnlyPublicItemID, csmOnlyStaffItemID)
		_, _ = pool.Exec(ctx, `DELETE FROM catalog_item WHERE id IN ($1, $2)`, csmOnlyPublicItemID, csmOnlyStaffItemID)
		_, _ = pool.Exec(ctx, `DELETE FROM sr_category WHERE id IN ($1, $2)`, csmOnlySharedCatID, csmOnlyStaffCatID)
		_, _ = scoped.Exec(ctx, `DELETE FROM deployed_product WHERE id = $1`, csmOnlyDeployedID)
		_, _ = scoped.Exec(ctx, `DELETE FROM deployment WHERE id = $1`, csmOnlyDeploymentID)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = $1`, csmOnlyProductID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, csmOnlyProjectID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, end_date)
	          VALUES ($1, now(), now(), 'csm-only-test', 'csm-only-test', 'CSMONLYTEST', 'sf-csmonlytest', (now() + INTERVAL '30 days')::date)`,
		csmOnlyProjectID)
	mustExec(`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, project_id)
	          VALUES ($1, now(), now(), 'csm-only-test', 'csm-only-test', 'CSMONLYDEP01', 'csm only dep', true, $2)`,
		csmOnlyDeploymentID, csmOnlyProjectID)
	mustExec(`INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name, code)
	          VALUES ($1, now(), now(), 'csm-only-test', 'csm-only-test', 'WSO2', 'SOFTWARE', 'CSM Only Test Product', 'csmonlytest')`,
		csmOnlyProductID)
	mustExec(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id, deployment_id, product_id, product_category)
	          VALUES ($1, now(), now(), 'csm-only-test', 'csm-only-test', 'CSMONLYDP01', true, $2, $3, $4, 'CL')`,
		csmOnlyDeployedID, csmOnlyProjectID, csmOnlyDeploymentID, csmOnlyProductID)

	mustExec(`INSERT INTO sr_category (id, created_on, updated_on, created_by, updated_by, name, is_active) VALUES
	          ($1, now(), now(), 'csm-only-test', 'csm-only-test', 'CSM Only Test Shared', true),
	          ($2, now(), now(), 'csm-only-test', 'csm-only-test', 'CSM Only Test Staff', true)`,
		csmOnlySharedCatID, csmOnlyStaffCatID)
	mustExec(`INSERT INTO catalog_item (id, created_on, updated_on, created_by, updated_by, name, csm_only) VALUES
	          ($1, now(), now(), 'csm-only-test', 'csm-only-test', 'CSM Only Test Public Item', false),
	          ($2, now(), now(), 'csm-only-test', 'csm-only-test', 'CSM Only Test Staff Item', true)`,
		csmOnlyPublicItemID, csmOnlyStaffItemID)
	mustExec(`INSERT INTO catalog_item_category (id, created_on, updated_on, created_by, updated_by, catalog_item_id, sr_category_id) VALUES
	          (uuidv7(), now(), now(), 'csm-only-test', 'csm-only-test', $1, $3),
	          (uuidv7(), now(), now(), 'csm-only-test', 'csm-only-test', $2, $3),
	          (uuidv7(), now(), now(), 'csm-only-test', 'csm-only-test', $2, $4)`,
		csmOnlyPublicItemID, csmOnlyStaffItemID, csmOnlySharedCatID, csmOnlyStaffCatID)
	mustExec(`INSERT INTO sr_category_routing_rule (id, created_on, updated_on, created_by, updated_by, "order", classification, catalog_item_id) VALUES
	          (uuidv7(), now(), now(), 'csm-only-test', 'csm-only-test', 1, 'CL', $1),
	          (uuidv7(), now(), now(), 'csm-only-test', 'csm-only-test', 1, 'CL', $2)`,
		csmOnlyPublicItemID, csmOnlyStaffItemID)
}

// catalogItemsByCategory flattens SearchCatalogs' result to category id ->
// item ids, limited to this fixture's categories.
func catalogItemsByCategory(catalogs []domain.Catalog) map[string][]string {
	out := map[string][]string{}
	for _, c := range catalogs {
		if c.ID != csmOnlySharedCatID && c.ID != csmOnlyStaffCatID {
			continue
		}
		ids := []string{}
		for _, item := range c.CatalogItems {
			ids = append(ids, item.ID)
		}
		out[c.ID] = ids
	}
	return out
}

func TestCatalogRepository_CSMOnly(t *testing.T) {
	pool := caseStatsPool(t)
	seedCSMOnlyCatalogFixture(t, pool)

	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewCatalogRepository(repository.NewScoped(pool))
	page := domain.Pagination{Limit: 100, Offset: 0}

	t.Run("staff see both items and both categories", func(t *testing.T) {
		catalogs, _, err := repo.SearchCatalogs(ctx, csmOnlyDeployedID, true, page)
		if err != nil {
			t.Fatalf("SearchCatalogs: %v", err)
		}
		got := catalogItemsByCategory(catalogs)
		if len(got[csmOnlySharedCatID]) != 2 || len(got[csmOnlyStaffCatID]) != 1 {
			t.Fatalf("categories = %v, want shared with 2 items and staff with 1", got)
		}
	})

	t.Run("others see only the ordinary item, and not the staff-only category", func(t *testing.T) {
		catalogs, _, err := repo.SearchCatalogs(ctx, csmOnlyDeployedID, false, page)
		if err != nil {
			t.Fatalf("SearchCatalogs: %v", err)
		}
		got := catalogItemsByCategory(catalogs)
		if items := got[csmOnlySharedCatID]; len(items) != 1 || items[0] != csmOnlyPublicItemID {
			t.Fatalf("shared category items = %v, want only %s", items, csmOnlyPublicItemID)
		}
		if _, ok := got[csmOnlyStaffCatID]; ok {
			t.Fatalf("staff-only category returned to a non-staff caller: %v", got)
		}
	})

	t.Run("questions of a staff-only item", func(t *testing.T) {
		if _, err := repo.GetCatalogItemVariables(ctx, csmOnlySharedCatID, csmOnlyStaffItemID, true); err != nil {
			t.Fatalf("staff: GetCatalogItemVariables: %v", err)
		}
		_, err := repo.GetCatalogItemVariables(ctx, csmOnlySharedCatID, csmOnlyStaffItemID, false)
		var notFound *apierror.NotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("non-staff: err = %v, want NotFoundError", err)
		}
		if _, err := repo.GetCatalogItemVariables(ctx, csmOnlySharedCatID, csmOnlyPublicItemID, false); err != nil {
			t.Fatalf("non-staff, ordinary item: GetCatalogItemVariables: %v", err)
		}
	})

	t.Run("IsCatalogItemCSMOnly", func(t *testing.T) {
		for id, want := range map[string]bool{
			csmOnlyStaffItemID:                     true,
			csmOnlyPublicItemID:                    false,
			"39999999-0000-0000-0000-000000000009": false,
		} {
			got, err := repo.IsCatalogItemCSMOnly(ctx, id)
			if err != nil {
				t.Fatalf("IsCatalogItemCSMOnly(%s): %v", id, err)
			}
			if got != want {
				t.Fatalf("IsCatalogItemCSMOnly(%s) = %v, want %v", id, got, want)
			}
		}
	})
}
