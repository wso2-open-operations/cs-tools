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

// UpdateDeployedProduct / UpdateDeployment through the service on plain
// PostgreSQL (no dual-write mirror wired): each update branch must change the
// row, read back from the table rather than taken from the response. Same DSN
// and skip-when-unset pattern as the other deployed_product_repo integration
// tests:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TestPlainPostgresUpdate

package repository_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const (
	ppuProjectID    = "39a11111-1111-1111-1111-111111111111"
	ppuDeploymentID = "39a22222-0000-0000-0000-000000000001"
	ppuOtherDepID   = "39a22222-0000-0000-0000-000000000002"
	ppuProductID    = "39a33333-0000-0000-0000-000000000001"
	ppuDeployedID   = "39a44444-0000-0000-0000-000000000001"
)

func ppuContext(t *testing.T, email string) context.Context {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"email": email})
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	req := httptest.NewRequest(http.MethodPatch, "/", nil)
	req.Header.Set("x-user-id-token", token)
	var captured context.Context
	middleware.UserIDToken(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), req)
	return repository.WithSystemIdentity(captured)
}

func seedPlainPostgresUpdateFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM deployed_product WHERE id = $1`, ppuDeployedID)
		_, _ = scoped.Exec(ctx, `DELETE FROM deployment WHERE id IN ($1, $2)`, ppuDeploymentID, ppuOtherDepID)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = $1`, ppuProductID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, ppuProjectID)
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
	          VALUES ($1, now(), now(), 'ppu-test', 'ppu-test', 'PPUTEST', 'sf-pputest', (now() + INTERVAL '30 days')::date)`, ppuProjectID)
	mustExec(`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, project_id)
	          VALUES ($1, now(), now(), 'ppu-test', 'ppu-test', 'PPUDEP01', 'ppu dep', true, $2),
	                 ($3, now(), now(), 'ppu-test', 'ppu-test', 'PPUDEP02', 'ppu other dep', true, $2)`,
		ppuDeploymentID, ppuProjectID, ppuOtherDepID)
	if _, err := pool.Exec(ctx, `INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name, code)
	          VALUES ($1, now(), now(), 'ppu-test', 'ppu-test', 'WSO2', 'SOFTWARE', 'PPU Test Product', 'pputest')`, ppuProductID); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	mustExec(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id, deployment_id, product_id, product_category, core_count, tps_count)
	          VALUES ($1, now(), now(), 'ppu-test', 'ppu-test', 'PPUDP01', true, $2, $3, $4, 'MS', 2, 1)`,
		ppuDeployedID, ppuProjectID, ppuDeploymentID, ppuProductID)
}

func TestPlainPostgresUpdateDeployedProduct(t *testing.T) {
	pool := caseStatsPool(t)
	seedPlainPostgresUpdateFixture(t, pool)
	ctx := ppuContext(t, "jane.doe@example.com")
	svc := service.NewDeployedProductService(repository.NewDeployedProductRepository(repository.NewScoped(pool)))

	cores, tps, category, inactive := 16, 42.5, "pdp", false
	steps := []struct {
		name  string
		req   domain.UpdateDeployedProductRequest
		check string
		want  string
	}{
		{"cores", domain.UpdateDeployedProductRequest{Cores: &cores}, `SELECT core_count::text FROM deployed_product WHERE id = $1`, "16"},
		{"tps", domain.UpdateDeployedProductRequest{TPS: &tps}, `SELECT tps_count::text FROM deployed_product WHERE id = $1`, "42.5"},
		{"description", domain.UpdateDeployedProductRequest{Description: json.RawMessage(`"edited"`)}, `SELECT description FROM deployed_product WHERE id = $1`, "edited"},
		{"updates", domain.UpdateDeployedProductRequest{Updates: []domain.ProductUpdateEntry{{UpdateLevel: 7, Date: "2026-01-15"}}}, `SELECT update_level_info->0->>'updateLevel' FROM deployed_product WHERE id = $1`, "7"},
		{"category", domain.UpdateDeployedProductRequest{Category: &category}, `SELECT product_category::text FROM deployed_product WHERE id = $1`, "PDP"},
		{"deactivate", domain.UpdateDeployedProductRequest{Active: &inactive}, `SELECT active::text FROM deployed_product WHERE id = $1`, "false"},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			req := s.req
			req.ID = ppuDeployedID
			resp, err := svc.UpdateDeployedProduct(ctx, req)
			if err != nil {
				t.Fatalf("UpdateDeployedProduct: %v", err)
			}
			if resp.DeployedProduct.UpdatedBy != "jane.doe@example.com" {
				t.Errorf("updatedBy = %q", resp.DeployedProduct.UpdatedBy)
			}
			var got string
			if err := pool.QueryRow(ctx, s.check, ppuDeployedID).Scan(&got); err != nil {
				t.Fatalf("read back: %v", err)
			}
			if got != s.want {
				t.Errorf("read back %q, want %q", got, s.want)
			}
		})
	}

	// deployment_id scoping is enforced in SQL: the wrong deployment finds nothing.
	other := ppuOtherDepID
	if _, err := svc.UpdateDeployedProduct(ctx, domain.UpdateDeployedProductRequest{ID: ppuDeployedID, DeploymentID: &other, Cores: &cores}); err == nil {
		t.Error("update scoped to a different deployment must fail")
	}
}

func TestPlainPostgresUpdateDeployment(t *testing.T) {
	pool := caseStatsPool(t)
	seedPlainPostgresUpdateFixture(t, pool)
	ctx := ppuContext(t, "jane.doe@example.com")
	svc := service.NewDeploymentService(repository.NewDeploymentRepository(repository.NewScoped(pool)))

	name := "Renamed deployment"
	if _, err := svc.UpdateDeployment(ctx, domain.UpdateDeploymentRequest{ID: ppuDeploymentID, Name: &name}); err != nil {
		t.Fatalf("UpdateDeployment (name): %v", err)
	}
	var gotName, gotBy string
	if err := pool.QueryRow(ctx, `SELECT name, updated_by FROM deployment WHERE id = $1`, ppuDeploymentID).Scan(&gotName, &gotBy); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if gotName != name || gotBy != "jane.doe@example.com" {
		t.Errorf("read back name=%q updated_by=%q", gotName, gotBy)
	}

	inactive := false
	if _, err := svc.UpdateDeployment(ctx, domain.UpdateDeploymentRequest{ID: ppuDeploymentID, Active: &inactive}); err != nil {
		t.Fatalf("UpdateDeployment (deactivate): %v", err)
	}
	var active bool
	if err := pool.QueryRow(ctx, `SELECT is_active FROM deployment WHERE id = $1`, ppuDeploymentID).Scan(&active); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if active {
		t.Error("deployment still active after deactivate")
	}
}
