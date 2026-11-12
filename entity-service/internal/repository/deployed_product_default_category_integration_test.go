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

// A deployed product created without a category gets its project type's
// default (migration 0291), on both creates, and the backfill (migration
// 0292) gives one to the products that have none. Reuses the deployment RLS
// fixture. Skipped without CASE_STATS_TEST_DSN (a login that does not bypass
// row-level security, all migrations applied).
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run DeployedProductDefaultCategory

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	dpdcTypeWithDefault = "a7000000-0000-0000-0000-000000000001" // default MS
	dpdcTypeNoDefault   = "a7000000-0000-0000-0000-000000000002" // no default
	dpdcVersionID       = "a6000000-0000-0000-0000-0000000000d1" // a version of depProductID
	dpdcActor           = "dpdc-actor@test.local"
)

func TestDeployedProductDefaultCategoryIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	seedDeploymentRLSFixture(t, pool)
	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	repo := repository.NewDeployedProductRepository(scoped)
	now := time.Now().UTC()

	var hasColumn bool
	if err := pool.QueryRow(sys, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'project_type' AND column_name = 'default_deployed_product_category')`).Scan(&hasColumn); err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if !hasColumn {
		t.Fatalf("project_type.default_deployed_product_category is missing: apply migration 0291 to the test database")
	}

	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM deployed_product WHERE created_by = $1`, dpdcActor)
		_, _ = pool.Exec(sys, `UPDATE project SET project_type_id = NULL WHERE id IN ($1, $2)`, depProjectOne, depProjectTwo)
		_, _ = pool.Exec(sys, `DELETE FROM project_type WHERE id IN ($1, $2)`, dpdcTypeWithDefault, dpdcTypeNoDefault)
		_, _ = pool.Exec(sys, `DELETE FROM product_version WHERE id = $1`, dpdcVersionID)
	}
	cleanup()
	// Registered after the fixture's own cleanup, so it runs first (LIFO).
	t.Cleanup(cleanup)
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(sys, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO project_type (id, created_on, updated_on, created_by, updated_by, name, default_deployed_product_category)
		VALUES ($1, $2, $2, 'test', 'test', 'DPDC type with a default', 'MS'),
		       ($3, $2, $2, 'test', 'test', 'DPDC type with no default', NULL)`, dpdcTypeWithDefault, now, dpdcTypeNoDefault)
	mustExec(`UPDATE project SET project_type_id = $1 WHERE id = $2`, dpdcTypeWithDefault, depProjectOne)
	mustExec(`UPDATE project SET project_type_id = $1 WHERE id = $2`, dpdcTypeNoDefault, depProjectTwo)
	mustExec(`INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id)
		VALUES ($1, $2, $2, 'test', 'test', 'dpdc-1.0', $3)`, dpdcVersionID, now, depProductID)

	categoryOf := func(t *testing.T, id string) *string {
		t.Helper()
		var category *string
		if err := scoped.QueryRow(sys, `SELECT product_category::text FROM deployed_product WHERE id = $1`, id).Scan(&category); err != nil {
			t.Fatalf("read category: %v", err)
		}
		return category
	}
	expect := func(t *testing.T, id string, want *string) {
		t.Helper()
		got := categoryOf(t, id)
		switch {
		case want == nil && got != nil:
			t.Errorf("category = %s, want none", *got)
		case want != nil && got == nil:
			t.Errorf("category = none, want %s", *want)
		case want != nil && *got != *want:
			t.Errorf("category = %s, want %s", *got, *want)
		}
	}
	str := func(s string) *string { return &s }
	internal := depCtx(repository.SearchScope{Unrestricted: true})
	req := func(project, deployment string, category *string) domain.CreateDeployedProductRequest {
		return domain.CreateDeployedProductRequest{ProjectID: project, DeploymentID: deployment, ProductID: depProductID, VersionID: dpdcVersionID, Category: category}
	}

	t.Run("postgres create: no category takes the project type's default", func(t *testing.T) {
		created, err := repo.CreateDeployedProduct(internal, req(depProjectOne, depDeployOne, nil), dpdcActor)
		if err != nil {
			t.Fatalf("CreateDeployedProduct: %v", err)
		}
		expect(t, created.ID, str("MS"))
	})

	t.Run("postgres create: a category in the request wins", func(t *testing.T) {
		created, err := repo.CreateDeployedProduct(internal, req(depProjectOne, depDeployOne, str("pc")), dpdcActor)
		if err != nil {
			t.Fatalf("CreateDeployedProduct: %v", err)
		}
		expect(t, created.ID, str("PC"))
	})

	t.Run("postgres create: a type with no default leaves it unset", func(t *testing.T) {
		created, err := repo.CreateDeployedProduct(internal, req(depProjectTwo, depDeployTwo, nil), dpdcActor)
		if err != nil {
			t.Fatalf("CreateDeployedProduct: %v", err)
		}
		expect(t, created.ID, nil)
	})

	snCreate := func(t *testing.T, id, number string, r domain.CreateDeployedProductRequest) {
		t.Helper()
		if _, err := repo.CreateDeployedProductFromServiceNow(sys, r, id, number, dpdcActor, now); err != nil {
			t.Fatalf("CreateDeployedProductFromServiceNow: %v", err)
		}
	}

	t.Run("servicenow-first create: no category takes the project type's default", func(t *testing.T) {
		id := "a5000000-0000-0000-0000-0000000000d1"
		snCreate(t, id, "DPDC-SN-1", req(depProjectOne, depDeployOne, nil))
		expect(t, id, str("MS"))
	})

	t.Run("servicenow-first create: a category in the request wins", func(t *testing.T) {
		id := "a5000000-0000-0000-0000-0000000000d2"
		snCreate(t, id, "DPDC-SN-2", req(depProjectOne, depDeployOne, str("pdp")))
		expect(t, id, str("PDP"))
	})

	t.Run("backfill fills only empty categories, finding the project through the deployment", func(t *testing.T) {
		sql, err := os.ReadFile("../../migrations/0292_deployed_product_category_backfill.sql")
		if err != nil {
			t.Fatalf("read migration: %v", err)
		}
		// The fixture's deployed products have no category: one with its own
		// project (one), one whose project is only its deployment's (one), one
		// in project two, whose type has no default. Give a fourth one a
		// category to show it is left alone.
		kept := "a5000000-0000-0000-0000-0000000000d3"
		snCreate(t, kept, "DPDC-SN-3", req(depProjectOne, depDeployOne, str("cl")))

		for range 2 { // a second run must change nothing
			if _, err := pool.Exec(context.Background(), string(sql)); err != nil {
				t.Fatalf("apply 0292: %v", err)
			}
			expect(t, depDPOneSet, str("MS"))
			expect(t, depDPOneLegacy, str("MS"))
			expect(t, depDPTwo, nil)
			expect(t, kept, str("CL"))
		}
		var by string
		if err := scoped.QueryRow(sys, `SELECT updated_by FROM deployed_product WHERE id = $1`, depDPOneLegacy).Scan(&by); err != nil {
			t.Fatalf("read updated_by: %v", err)
		}
		if by != "migration:0292_deployed_product_category_backfill" {
			t.Errorf("updated_by = %q", by)
		}
		var internalAfter string
		if err := pool.QueryRow(context.Background(), `SELECT current_setting('app.is_internal', true)`).Scan(&internalAfter); err != nil {
			t.Fatalf("read setting: %v", err)
		}
		if internalAfter == "true" {
			t.Errorf("the migration left the session marked internal")
		}
	})
}
