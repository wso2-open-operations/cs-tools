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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// product_version has one row per (product, version, deployment profile). A node
// reports only a product and version, so it resolves to one profile row
// (ingestVersionAM420 here, standing in for the control-plane profile) while the
// deployment's deployed product may reference another (the all-in-one profile
// below). Usage must reach the deployed product whatever the profile.
const (
	profileVersionAllInOne = "6c100000-0000-4000-8000-000000000001"
	profileVersionTraffic  = "6c100000-0000-4000-8000-000000000002"
	profileDPAllInOne      = "6f200000-0000-4000-8000-000000000001"
	profileDPTraffic       = "6f200000-0000-4000-8000-000000000002"
	profileDPControl       = "6f200000-0000-4000-8000-000000000003"
	profileNode            = "6f300000-0000-4000-8000-000000000001"
	profileDeployment      = "6e000000-0000-4000-8000-000000000002" // DEP-OLD of the project keyed CUPRSUBDEV
)

// seedProfileNode adds the extra profile rows, a deployed_product table holding
// the given (id, version_id, product_id) rows on the shared deployment, and one
// node on ingestVersionAM420 with 100 TRANSACTION_COUNT on 1 June.
func seedProfileNode(t *testing.T, ctx context.Context, exec func(context.Context, string, ...any) error, deployed [][3]string) {
	t.Helper()
	stmts := []string{
		// The shared fixture omits the display names the instance list selects.
		`ALTER TABLE project ADD COLUMN name VARCHAR(255)`,
		`ALTER TABLE deployment ADD COLUMN name VARCHAR(255)`,
		`INSERT INTO product_version (id, product_id, version) VALUES
			('` + profileVersionAllInOne + `', '` + ingestProductAM + `', '4.2.0'),
			('` + profileVersionTraffic + `', '` + ingestProductAM + `', '4.2.0')`,
		`CREATE TABLE deployed_product (id UUID PRIMARY KEY, name VARCHAR(255), number VARCHAR(50),
			deployment_id UUID, version_id UUID, product_id UUID)`,
		`INSERT INTO deployment_node (id, created_on, updated_on, created_by, updated_by, node_id, project_key, deployment_number, product_version_id)
			VALUES ('` + profileNode + `', NOW(), NOW(), 't', 't', 'profile-node', 'CUPRSUBDEV', 'DEP-OLD', '` + ingestVersionAM420 + `')`,
		`INSERT INTO daily_usage_summary (id, created_on, updated_on, created_by, updated_by, deployment_node_id, deployment_number, summary_date, count_type, value, data_source)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', '` + profileNode + `', 'DEP-OLD', '2026-06-01', 'TRANSACTION_COUNT', 100, 'API_CALL')`,
	}
	for _, d := range deployed {
		stmts = append(stmts, `INSERT INTO deployed_product (id, name, deployment_id, version_id, product_id) VALUES
			('`+d[0]+`', 'dp-`+d[0][:8]+d[0][len(d[0])-1:]+`', '`+profileDeployment+`', '`+d[1]+`', '`+d[2]+`')`)
	}
	for _, stmt := range stmts {
		if err := exec(ctx, stmt); err != nil {
			t.Fatalf("seed %.60q: %v", stmt, err)
		}
	}
}

func profileSetup(t *testing.T, deployed [][3]string) (context.Context, *repository.Scoped) {
	t.Helper()
	pool := instanceIngestionTestPool(t)
	ctx := internalCtx()
	seedProfileNode(t, ctx, func(c context.Context, q string, a ...any) error {
		_, err := pool.Exec(c, q, a...)
		return err
	}, deployed)
	return ctx, repository.NewScoped(pool)
}

func usageTotal(t *testing.T, ctx context.Context, db *repository.Scoped, deployedProductID string) int {
	t.Helper()
	got, err := repository.NewInstanceRepository(db).SearchInstanceUsageStats(ctx, domain.InstanceDateRangeFilters{
		StartDate: "2026-06-01", EndDate: "2026-06-30", DeployedProductIDs: []string{deployedProductID},
	}, nil)
	if err != nil {
		t.Fatalf("SearchInstanceUsageStats: %v", err)
	}
	return got.Stats["2026-06-01"]["TRANSACTION_COUNT"]
}

func metricsInstances(t *testing.T, ctx context.Context, db *repository.Scoped, deployedProductID string) (int, float64) {
	t.Helper()
	got, err := repository.NewDeployedProductRepository(db).
		SearchDeployedProductUsageCounts(ctx, deployedProductID, profileDeployment, "2026-06-01", "2026-06-30")
	if err != nil {
		t.Fatalf("SearchDeployedProductUsageCounts: %v", err)
	}
	for _, e := range got.ChartData {
		if e.Date == "2026-06-01" {
			if c, ok := e.Counts["TRANSACTION_COUNT"]; ok {
				return len(c.Instances), c.Value
			}
		}
	}
	return 0, 0
}

// A node on one profile row and a deployed product on another profile row of the
// same product and version are the same product: the usage reaches it.
func TestUsageJoin_NodeAndDeployedProductOnDifferentProfilesMatch(t *testing.T) {
	ctx, db := profileSetup(t, [][3]string{{profileDPAllInOne, profileVersionAllInOne, ingestProductAM}})

	if got := usageTotal(t, ctx, db, profileDPAllInOne); got != 100 {
		t.Errorf("SearchInstanceUsageStats for the deployed product = %d, want 100", got)
	}
	n, v := metricsInstances(t, ctx, db, profileDPAllInOne)
	if n != 1 || v != 100 {
		t.Errorf("SearchDeployedProductUsageCounts: %d instances, value %v, want 1 and 100", n, v)
	}

	list, total, err := repository.NewInstanceRepository(db).SearchInstances(ctx, domain.SearchInstancesRequest{
		Filters:    &domain.InstanceSearchFilters{DeployedProductIDs: []string{profileDPAllInOne}},
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("SearchInstances: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].DeployedProduct == nil || list[0].DeployedProduct.ID != profileDPAllInOne {
		t.Errorf("SearchInstances = %d total, %+v, want the node resolved to the deployed product", total, list)
	}
}

// Only the product and version matter: another version of the same product, or
// the same version text of another product, must not match.
func TestUsageJoin_DifferentVersionOrProductDoesNotMatch(t *testing.T) {
	ctx, db := profileSetup(t, [][3]string{
		{profileDPAllInOne, ingestVersionAM430, ingestProductAM},
		{profileDPTraffic, ingestVersionAxM420, ingestProductAxM},
	})

	for _, id := range []string{profileDPAllInOne, profileDPTraffic} {
		if got := usageTotal(t, ctx, db, id); got != 0 {
			t.Errorf("SearchInstanceUsageStats for %s = %d, want 0", id, got)
		}
		if n, _ := metricsInstances(t, ctx, db, id); n != 0 {
			t.Errorf("SearchDeployedProductUsageCounts for %s: %d instances, want 0", id, n)
		}
	}

	// The node is still listed, with its product but no deployed product.
	list, total, err := repository.NewInstanceRepository(db).SearchInstances(ctx, domain.SearchInstancesRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("SearchInstances: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].DeployedProduct != nil {
		t.Errorf("SearchInstances = %d total, %+v, want one node with no deployed product", total, list)
	}
}

// Two deployed products of the same product and version under different profiles
// must not repeat the node (and so double its usage): it belongs to exactly one,
// the lowest id, and the other sees none of it.
func TestUsageJoin_SeveralProfilesOfOneProductCountTheNodeOnce(t *testing.T) {
	ctx, db := profileSetup(t, [][3]string{
		{profileDPTraffic, profileVersionTraffic, ingestProductAM},
		{profileDPAllInOne, profileVersionAllInOne, ingestProductAM},
	})

	list, total, err := repository.NewInstanceRepository(db).SearchInstances(ctx, domain.SearchInstancesRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("SearchInstances: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("SearchInstances = %d total, %d rows, want the node once", total, len(list))
	}
	if list[0].DeployedProduct == nil || list[0].DeployedProduct.ID != profileDPAllInOne {
		t.Errorf("deployed product = %+v, want the lowest id %s", list[0].DeployedProduct, profileDPAllInOne)
	}

	if got := usageTotal(t, ctx, db, profileDPAllInOne); got != 100 {
		t.Errorf("owner usage = %d, want 100 (not doubled)", got)
	}
	if got := usageTotal(t, ctx, db, profileDPTraffic); got != 0 {
		t.Errorf("other profile's usage = %d, want 0", got)
	}
	if n, v := metricsInstances(t, ctx, db, profileDPAllInOne); n != 1 || v != 100 {
		t.Errorf("owner metrics: %d instances, value %v, want 1 and 100", n, v)
	}
	if n, _ := metricsInstances(t, ctx, db, profileDPTraffic); n != 0 {
		t.Errorf("other profile's metrics: %d instances, want 0", n)
	}
}

// A deployed product on the node's own product_version row wins over a lower id
// on another profile, so deployments that already matched keep their result.
func TestUsageJoin_ExactVersionRowWinsOverLowerIdOfAnotherProfile(t *testing.T) {
	ctx, db := profileSetup(t, [][3]string{
		{profileDPAllInOne, profileVersionAllInOne, ingestProductAM},
		{profileDPControl, ingestVersionAM420, ingestProductAM},
	})

	if got := usageTotal(t, ctx, db, profileDPControl); got != 100 {
		t.Errorf("exact-row deployed product usage = %d, want 100", got)
	}
	if got := usageTotal(t, ctx, db, profileDPAllInOne); got != 0 {
		t.Errorf("other profile usage = %d, want 0", got)
	}
}
