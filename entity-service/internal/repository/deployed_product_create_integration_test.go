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

// DeployedProductRepository.CreateDeployedProduct, the plain PostgreSQL
// create, against a real database with all migrations applied: the number
// comes from the IBITM series and lands past every IBITM number already
// stored even when the sequence was never seeded, the deployment must belong
// to the project and the version to the product, and a customer can create
// only in their own project. Reuses the deployment RLS fixture. Skipped
// without CASE_STATS_TEST_DSN (a login that does not bypass row-level
// security).
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run DeployedProductCreate

package repository_test

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	dpcVersionID      = "a6000000-0000-0000-0000-000000000001" // a version of depProductID
	dpcOtherProductID = "a4000000-0000-0000-0000-000000000002"
	dpcOtherVersionID = "a6000000-0000-0000-0000-000000000002" // a version of dpcOtherProductID
	dpcSyncedDPID     = "a5000000-0000-0000-0000-0000000000c1"
	dpcSyncedNumber   = 9999990 // far above anything a real database holds
	dpcActor          = "dpc-actor@test.local"
)

var ibitmNumber = regexp.MustCompile(`^IBITM([0-9]{7,})$`)

func TestDeployedProductCreateIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	seedDeploymentRLSFixture(t, pool)
	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	repo := repository.NewDeployedProductRepository(scoped)
	now := time.Now().UTC()

	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM deployed_product WHERE created_by IN ($1, $2)`, depCreatedBy, dpcActor)
		_, _ = pool.Exec(sys, `DELETE FROM product_version WHERE id IN ($1, $2)`, dpcVersionID, dpcOtherVersionID)
		_, _ = pool.Exec(sys, `DELETE FROM product WHERE id = $1`, dpcOtherProductID)
	}
	// Registered before the fixture's own cleanup runs (LIFO), so the rows
	// referencing the fixture's product and deployments go first.
	t.Cleanup(cleanup)
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(sys, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name)
		VALUES ($1, $2, $2, 'test', 'test', 'WSO2', 'SOFTWARE', 'DPC Other Product')`, dpcOtherProductID, now)
	mustExec(`INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id)
		VALUES ($1, $2, $2, 'test', 'test', 'dpc-1.0', $3)`, dpcVersionID, now, depProductID)
	mustExec(`INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id)
		VALUES ($1, $2, $2, 'test', 'test', 'dpc-2.0', $3)`, dpcOtherVersionID, now, dpcOtherProductID)
	// A deployed product synced from ServiceNow, holding a high IBITM number.
	if _, err := scoped.Exec(sys, `INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id, deployment_id, product_id, version_id)
		VALUES ($1, $2, $2, $3, $3, $4, TRUE, $5, $6, $7, $8)`,
		dpcSyncedDPID, now, depCreatedBy, "IBITM"+strconv.Itoa(dpcSyncedNumber), depProjectOne, depDeployOne, depProductID, dpcVersionID); err != nil {
		t.Fatalf("seed synced deployed product: %v", err)
	}
	// The sequence as it is before the cutover seed: never advanced.
	mustExec(`SELECT setval('ibitm_number_seq', 1, false)`)

	member := depCtx(repository.SearchScope{ViewerEmail: depMemberOne})
	internal := depCtx(repository.SearchScope{Unrestricted: true})
	cores := 4
	category := "MS"
	req := func(project, deployment, product, version string) domain.CreateDeployedProductRequest {
		return domain.CreateDeployedProductRequest{ProjectID: project, DeploymentID: deployment, ProductID: product, VersionID: version, Cores: &cores, Category: &category}
	}
	numberOf := func(t *testing.T, id string) int {
		t.Helper()
		var number string
		if err := scoped.QueryRow(sys, `SELECT number FROM deployed_product WHERE id = $1`, id).Scan(&number); err != nil {
			t.Fatalf("read number: %v", err)
		}
		m := ibitmNumber.FindStringSubmatch(number)
		if m == nil {
			t.Fatalf("number %q is not IBITM + 7 digits", number)
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}

	t.Run("numbered past every stored IBITM number with an unseeded sequence", func(t *testing.T) {
		created, err := repo.CreateDeployedProduct(internal, req(depProjectOne, depDeployOne, depProductID, dpcVersionID), dpcActor)
		if err != nil {
			t.Fatalf("CreateDeployedProduct: %v", err)
		}
		if got := numberOf(t, created.ID); got != dpcSyncedNumber+1 {
			t.Errorf("number = IBITM%07d, want IBITM%07d", got, dpcSyncedNumber+1)
		}
		if created.CreatedBy != dpcActor {
			t.Errorf("createdBy = %q, want %q", created.CreatedBy, dpcActor)
		}
		var cores int
		var category, project string
		if err := scoped.QueryRow(sys, `SELECT core_count, product_category::text, project_id::text FROM deployed_product WHERE id = $1`, created.ID).Scan(&cores, &category, &project); err != nil {
			t.Fatalf("read row: %v", err)
		}
		if cores != 4 || category != "MS" || project != depProjectOne {
			t.Errorf("row = cores %d, category %s, project %s", cores, category, project)
		}
	})

	t.Run("a second create takes the next number", func(t *testing.T) {
		created, err := repo.CreateDeployedProduct(internal, req(depProjectOne, depDeployOne, depProductID, dpcVersionID), dpcActor)
		if err != nil {
			t.Fatalf("CreateDeployedProduct: %v", err)
		}
		if got := numberOf(t, created.ID); got != dpcSyncedNumber+2 {
			t.Errorf("number = IBITM%07d, want IBITM%07d", got, dpcSyncedNumber+2)
		}
	})

	t.Run("a customer creates in their own project", func(t *testing.T) {
		if _, err := repo.CreateDeployedProduct(member, req(depProjectOne, depDeployOne, depProductID, dpcVersionID), dpcActor); err != nil {
			t.Fatalf("CreateDeployedProduct as a member: %v", err)
		}
	})

	refused := map[string]struct {
		ctx context.Context
		req domain.CreateDeployedProductRequest
	}{
		"deployment of another project (internal caller)": {internal, req(depProjectOne, depDeployTwo, depProductID, dpcVersionID)},
		"version of another product":                      {internal, req(depProjectOne, depDeployOne, depProductID, dpcOtherVersionID)},
		"deployment that does not exist":                  {internal, req(depProjectOne, "a3000000-0000-0000-0000-0000000000ff", depProductID, dpcVersionID)},
		"customer, another project's deployment":          {member, req(depProjectTwo, depDeployTwo, depProductID, dpcVersionID)},
	}
	for name, c := range refused {
		t.Run("refused: "+name, func(t *testing.T) {
			var before int
			_ = scoped.QueryRow(sys, `SELECT count(*) FROM deployed_product WHERE created_by = $1`, dpcActor).Scan(&before)
			_, err := repo.CreateDeployedProduct(c.ctx, c.req, dpcActor)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want a ValidationError, got %v", err)
			}
			var after int
			_ = scoped.QueryRow(sys, `SELECT count(*) FROM deployed_product WHERE created_by = $1`, dpcActor).Scan(&after)
			if after != before {
				t.Errorf("a refused create wrote a row")
			}
		})
	}
}
