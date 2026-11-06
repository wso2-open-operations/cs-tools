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

// This is an integration test: it exercises the real SQL in
// deployed_product_repo.go's SearchDeployedProducts against a live
// PostgreSQL instance. It seeds rows, so point it at a scratch database,
// never a shared one. Same DSN and skip-when-unset pattern as
// deployed_product_repo_search_integration_test.go:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TestSearchDeployedProductsVersionProfile

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	dpProfileProjectID    = "38111111-1111-1111-1111-111111111111"
	dpProfileDeploymentID = "38222222-0000-0000-0000-000000000001"
	dpProfileProductID    = "38333333-0000-0000-0000-000000000001"
	// Three version rows that share product and version label and differ
	// only by deployment_profile (the last has none).
	dpProfileVersionCPID   = "38444444-0000-0000-0000-000000000001"
	dpProfileVersionTMID   = "38444444-0000-0000-0000-000000000002"
	dpProfileVersionNoneID = "38444444-0000-0000-0000-000000000003"
	dpProfileDPCPID        = "38555555-0000-0000-0000-000000000001"
	dpProfileDPTMID        = "38555555-0000-0000-0000-000000000002"
	dpProfileDPNoneID      = "38555555-0000-0000-0000-000000000003"
)

// TestSearchDeployedProductsVersionProfile guards the read-side gap where a
// deployed product's version reference carried only the version label: several
// product_version rows share one label and differ only by deployment_profile
// (migration 0105), so a caller could not tell which of them a deployed
// product points at.
func TestSearchDeployedProductsVersionProfile(t *testing.T) {
	pool := caseStatsPool(t)

	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM deployed_product WHERE id IN ($1, $2, $3)`, dpProfileDPCPID, dpProfileDPTMID, dpProfileDPNoneID)
		_, _ = scoped.Exec(ctx, `DELETE FROM deployment WHERE id = $1`, dpProfileDeploymentID)
		_, _ = pool.Exec(ctx, `DELETE FROM product_version WHERE id IN ($1, $2, $3)`, dpProfileVersionCPID, dpProfileVersionTMID, dpProfileVersionNoneID)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = $1`, dpProfileProductID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, dpProfileProjectID)
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
	          VALUES ($1, now(), now(), 'dp-profile-test', 'dp-profile-test', 'DPPROFILETEST', 'sf-dpprofiletest', (now() + INTERVAL '30 days')::date)`, dpProfileProjectID)

	mustExec(`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, project_id)
	          VALUES ($1, now(), now(), 'dp-profile-test', 'dp-profile-test', 'DPPROFILEDEP01', 'dp profile dep', true, $2)`,
		dpProfileDeploymentID, dpProfileProjectID)

	if _, err := pool.Exec(ctx, `INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name, code)
	          VALUES ($1, now(), now(), 'dp-profile-test', 'dp-profile-test', 'WSO2', 'SOFTWARE', 'DP Profile Test Product', 'dpprofiletest')`,
		dpProfileProductID); err != nil {
		t.Fatalf("seed product: %v", err)
	}

	for _, v := range []struct{ id, profile string }{
		{dpProfileVersionCPID, "API Control Plane"},
		{dpProfileVersionTMID, "Traffic Manager"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id, deployment_profile)
		          VALUES ($1, now(), now(), 'dp-profile-test', 'dp-profile-test', '4.6.0', $2, $3)`,
			v.id, dpProfileProductID, v.profile); err != nil {
			t.Fatalf("seed product_version %s: %v", v.profile, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id)
	          VALUES ($1, now(), now(), 'dp-profile-test', 'dp-profile-test', '4.6.0', $2)`,
		dpProfileVersionNoneID, dpProfileProductID); err != nil {
		t.Fatalf("seed product_version without a profile: %v", err)
	}

	for _, d := range []struct{ id, number, versionID string }{
		{dpProfileDPCPID, "DPPROFILECP01", dpProfileVersionCPID},
		{dpProfileDPTMID, "DPPROFILETM01", dpProfileVersionTMID},
		{dpProfileDPNoneID, "DPPROFILENO01", dpProfileVersionNoneID},
	} {
		mustExec(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id, deployment_id, product_id, version_id)
		          VALUES ($1, now(), now(), 'dp-profile-test', 'dp-profile-test', $2, true, $3, $4, $5, $6)`,
			d.id, d.number, dpProfileProjectID, dpProfileDeploymentID, dpProfileProductID, d.versionID)
	}

	repo := repository.NewDeployedProductRepository(repository.NewScoped(pool))
	views, _, err := repo.SearchDeployedProducts(ctx, domain.SearchDeployedProductsRequest{
		Pagination:    domain.Pagination{Limit: 50, Offset: 0},
		DeploymentIDs: []string{dpProfileDeploymentID},
	})
	if err != nil {
		t.Fatalf("SearchDeployedProducts: %v", err)
	}

	byID := make(map[string]domain.DeployedProductView, len(views))
	for _, v := range views {
		byID[v.ID] = v
	}

	cases := []struct {
		name string
		id   string
		want *string
	}{
		{"control plane", dpProfileDPCPID, strPtr("API Control Plane")},
		{"traffic manager", dpProfileDPTMID, strPtr("Traffic Manager")},
		{"no profile", dpProfileDPNoneID, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, ok := byID[c.id]
			if !ok {
				t.Fatalf("fixture row %s not found in search results", c.id)
			}
			if v.Version == nil {
				t.Fatalf("Version is nil, want the seeded 4.6.0 reference")
			}
			if v.Version.Name != "4.6.0" {
				t.Errorf("Version.Name = %q, want %q", v.Version.Name, "4.6.0")
			}
			switch {
			case c.want == nil && v.Version.DeploymentProfile != nil:
				t.Errorf("DeploymentProfile = %q, want nil", *v.Version.DeploymentProfile)
			case c.want != nil && v.Version.DeploymentProfile == nil:
				t.Errorf("DeploymentProfile = nil, want %q", *c.want)
			case c.want != nil && *v.Version.DeploymentProfile != *c.want:
				t.Errorf("DeploymentProfile = %q, want %q", *v.Version.DeploymentProfile, *c.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
