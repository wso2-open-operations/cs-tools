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
// default (migration 0291) when it is created, and the backfill (migration
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
	req := func(project, deployment string, category *string) domain.CreateDeployedProductRequest {
		return domain.CreateDeployedProductRequest{ProjectID: project, DeploymentID: deployment, ProductID: depProductID, VersionID: dpdcVersionID, Category: category}
	}

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

	t.Run("servicenow-first create: a type with no default leaves it unset", func(t *testing.T) {
		id := "a5000000-0000-0000-0000-0000000000d4"
		snCreate(t, id, "DPDC-SN-4", req(depProjectTwo, depDeployTwo, nil))
		expect(t, id, nil)
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

		// The migration updates every uncategorised product in the database,
		// not only this test's, so it runs in a transaction that is rolled
		// back: nothing it writes outlives the test. Its own statements, the
		// clearing of app.is_internal included, run in that transaction, so the
		// flag read there right after it says whether the migration cleared it.
		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		expectIn := func(t *testing.T, id string, want *string) {
			t.Helper()
			var got *string
			if err := tx.QueryRow(ctx, `SELECT product_category::text FROM deployed_product WHERE id = $1`, id).Scan(&got); err != nil {
				t.Fatalf("read category: %v", err)
			}
			switch {
			case want == nil && got != nil:
				t.Errorf("category = %s, want none", *got)
			case want != nil && got == nil:
				t.Errorf("category = none, want %s", *want)
			case want != nil && *got != *want:
				t.Errorf("category = %s, want %s", *got, *want)
			}
		}
		for range 2 { // a second run must change nothing
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				t.Fatalf("apply 0292: %v", err)
			}
			var internalAfter string
			if err := tx.QueryRow(ctx, `SELECT current_setting('app.is_internal', true)`).Scan(&internalAfter); err != nil {
				t.Fatalf("read setting: %v", err)
			}
			if internalAfter == "true" {
				t.Fatalf("the migration left the session marked internal")
			}
			// deployed_product is row-level secured: read the results as an
			// internal session, for this transaction only.
			if _, err := tx.Exec(ctx, `SELECT set_config('app.is_internal', 'true', true)`); err != nil {
				t.Fatalf("mark internal: %v", err)
			}
			expectIn(t, depDPOneSet, str("MS"))
			expectIn(t, depDPOneLegacy, str("MS"))
			expectIn(t, depDPTwo, nil)
			expectIn(t, kept, str("CL"))
		}
		var by string
		if err := tx.QueryRow(ctx, `SELECT updated_by FROM deployed_product WHERE id = $1`, depDPOneLegacy).Scan(&by); err != nil {
			t.Fatalf("read updated_by: %v", err)
		}
		if by != "migration:0292_deployed_product_category_backfill" {
			t.Errorf("updated_by = %q", by)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatalf("rollback: %v", err)
		}
		// Rolled back: the fixture's products have no category again.
		expect(t, depDPOneSet, nil)
	})
}
