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

// Regression test for migration 0176: deployment and deployed_product now
// carry project-membership row-level security. Reads go through the real
// DeploymentRepository / DeployedProductRepository Go code (not just the SQL
// policy in isolation), writes through repository.Scoped so the exact same
// identity plumbing production uses is what the policies see. Before 0176 a
// customer on project A could list project B's deployments and deployed
// products. Runs against a real Postgres with 0176 applied. Skipped without
// CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run DeploymentRLS

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	depAccountID   = "a0000000-0000-0000-0000-000000000001"
	depContactID   = "a1000000-0000-0000-0000-000000000001"
	depProjectOne  = "a2000000-0000-0000-0000-000000000001"
	depProjectTwo  = "a2000000-0000-0000-0000-000000000002"
	depDeployOne   = "a3000000-0000-0000-0000-000000000001" // project one
	depDeployTwo   = "a3000000-0000-0000-0000-000000000002" // project two
	depDeployOrphn = "a3000000-0000-0000-0000-000000000003" // project_id NULL
	depProductID   = "a4000000-0000-0000-0000-000000000001"
	depDPOneSet    = "a5000000-0000-0000-0000-000000000001" // project one, project_id set
	depDPOneLegacy = "a5000000-0000-0000-0000-000000000002" // project one, project_id NULL (falls back to deployment)
	depDPTwo       = "a5000000-0000-0000-0000-000000000003" // project two
	depMemberOne   = "dep-member-one@test.local"
	depMemberTwo   = "dep-member-two@test.local"
	depStranger    = "dep-stranger@test.local"
	depCreatedBy   = "dep-rls-test"
)

// seedDeploymentRLSFixture builds two projects with one REGISTERED contact
// each, a deployment per project plus an orphan deployment whose project is
// NULL (what a deleted project leaves behind), and three deployed products:
// one with project_id set, one legacy row with project_id NULL that must
// resolve through its deployment, and one in the other project.
func seedDeploymentRLSFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM deployed_product WHERE created_by = $1`, depCreatedBy)
		_, _ = scoped.Exec(ctx, `DELETE FROM deployment WHERE created_by = $1`, depCreatedBy)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = $1`, depProductID)
		_, _ = pool.Exec(ctx, `DELETE FROM project_contact WHERE project_id IN ($1, $2)`, depProjectOne, depProjectTwo)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id IN ($1, $2)`, depProjectOne, depProjectTwo)
		_, _ = pool.Exec(ctx, `DELETE FROM account_contact WHERE id = $1`, depContactID)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, depAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	mustExecScoped := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed scoped (%.80s): %v", sql, err)
		}
	}
	now := time.Now().UTC()

	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'DEP Test Account', 'DEP-ACC-1', 'sf-dep-acc-1')`, depAccountID, now)
	mustExec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'DEP Test Contact', $3)`, depContactID, now, depAccountID)
	for i, p := range []string{depProjectOne, depProjectTwo} {
		key := []string{"DEPONE", "DEPTWO"}[i]
		mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id, account_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $3, $4, $5)`, p, now, key, "sf-"+key, depAccountID)
	}
	mustExec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3, $4, 'REGISTERED')`, now, depMemberOne, depContactID, depProjectOne)
	mustExec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3, $4, 'REGISTERED')`, now, depMemberTwo, depContactID, depProjectTwo)
	mustExec(`INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name)
		VALUES ($1, $2, $2, 'test', 'test', 'WSO2', 'SOFTWARE', 'DEP Test Product')`, depProductID, now)

	insertDeployment := func(id, number, name string, project any) {
		mustExecScoped(`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id)
			VALUES ($1, $2, $2, $3, $3, $4, $5, 'DEVELOPMENT', TRUE, $6)`, id, now, depCreatedBy, number, name, project)
	}
	insertDeployment(depDeployOne, "DEP-NUM-1", "dep one", depProjectOne)
	insertDeployment(depDeployTwo, "DEP-NUM-2", "dep two", depProjectTwo)
	insertDeployment(depDeployOrphn, "DEP-NUM-3", "dep orphan", nil)

	insertDP := func(id, number, deployment string, project any) {
		mustExecScoped(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id, deployment_id, product_id)
			VALUES ($1, $2, $2, $3, $3, $4, TRUE, $5, $6, $7)`, id, now, depCreatedBy, number, project, deployment, depProductID)
	}
	insertDP(depDPOneSet, "DEPP-1", depDeployOne, depProjectOne)
	insertDP(depDPOneLegacy, "DEPP-2", depDeployOne, nil)
	insertDP(depDPTwo, "DEPP-3", depDeployTwo, depProjectTwo)
}

func depCtx(scope repository.SearchScope) context.Context {
	return repository.WithCallerIdentity(context.Background(), scope)
}

func depIDs(views []domain.DeploymentView) map[string]bool {
	out := map[string]bool{}
	for _, v := range views {
		out[v.ID] = true
	}
	return out
}

func TestDeploymentRLSIntegration_SearchDeploymentsIsScopedToTheCallersProjects(t *testing.T) {
	pool := caseStatsPool(t)
	seedDeploymentRLSFixture(t, pool)
	repo := repository.NewDeploymentRepository(repository.NewScoped(pool))
	req := domain.SearchDeploymentsRequest{Pagination: domain.Pagination{Limit: 50}}

	cases := []struct {
		name  string
		scope repository.SearchScope
		want  []string
		deny  []string
	}{
		{"member of one", repository.SearchScope{ViewerEmail: depMemberOne}, []string{depDeployOne}, []string{depDeployTwo, depDeployOrphn}},
		{"member of two", repository.SearchScope{ViewerEmail: depMemberTwo}, []string{depDeployTwo}, []string{depDeployOne, depDeployOrphn}},
		{"stranger", repository.SearchScope{ViewerEmail: depStranger}, nil, []string{depDeployOne, depDeployTwo, depDeployOrphn}},
		{"internal sees both projects", repository.SearchScope{Unrestricted: true}, []string{depDeployOne, depDeployTwo}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			views, total, err := repo.SearchDeployments(depCtx(c.scope), req)
			if err != nil {
				t.Fatalf("SearchDeployments: %v", err)
			}
			got := depIDs(views)
			for _, id := range c.want {
				if !got[id] {
					t.Errorf("want deployment %s visible, got %v", id, got)
				}
			}
			for _, id := range c.deny {
				if got[id] {
					t.Errorf("deployment %s must NOT be visible to %s", id, c.name)
				}
			}
			// The count query runs on its own connection: it must be scoped
			// exactly like the data query, or total leaks other projects' rows.
			if !c.scope.Unrestricted && total != len(views) {
				t.Errorf("total = %d but %d rows returned: count query is not scoped like the data query", total, len(views))
			}
		})
	}
}

func TestDeploymentRLSIntegration_ProjectFilterCannotReachAForeignProject(t *testing.T) {
	pool := caseStatsPool(t)
	seedDeploymentRLSFixture(t, pool)
	repo := repository.NewDeploymentRepository(repository.NewScoped(pool))

	// The exact request the portal sends: an explicit projectIds filter naming
	// a project the caller does not belong to. This is the leak 0176 closes.
	req := domain.SearchDeploymentsRequest{Pagination: domain.Pagination{Limit: 50}, ProjectIDs: []string{depProjectTwo}}
	views, total, err := repo.SearchDeployments(depCtx(repository.SearchScope{ViewerEmail: depMemberOne}), req)
	if err != nil {
		t.Fatalf("SearchDeployments: %v", err)
	}
	if len(views) != 0 || total != 0 {
		t.Fatalf("member of project one asked for project two's deployments: got %d rows, total %d; want none", len(views), total)
	}
}

// The ids filter is the one way to resolve a single deployment with no
// project context at all (e.g. backend-v2's attachment authorization check
// for a deployment-referenced attachment) -- it must still only return rows
// RLS already lets this caller see, never bypass the scope.
func TestDeploymentRLSIntegration_IDsFilterStaysWithinScope(t *testing.T) {
	pool := caseStatsPool(t)
	seedDeploymentRLSFixture(t, pool)
	repo := repository.NewDeploymentRepository(repository.NewScoped(pool))
	req := domain.SearchDeploymentsRequest{Pagination: domain.Pagination{Limit: 50}, IDs: []string{depDeployOne, depDeployTwo}}

	views, total, err := repo.SearchDeployments(depCtx(repository.SearchScope{ViewerEmail: depMemberOne}), req)
	if err != nil {
		t.Fatalf("SearchDeployments: %v", err)
	}
	got := depIDs(views)
	if !got[depDeployOne] {
		t.Errorf("want own deployment %s visible, got %v", depDeployOne, got)
	}
	if got[depDeployTwo] {
		t.Errorf("deployment %s belongs to a project this caller is not a member of; must NOT be visible via ids alone", depDeployTwo)
	}
	if total != len(views) {
		t.Errorf("total = %d but %d rows returned", total, len(views))
	}
}

func TestDeploymentRLSIntegration_DeployedProductsFollowTheirDeploymentsProject(t *testing.T) {
	pool := caseStatsPool(t)
	seedDeploymentRLSFixture(t, pool)
	repo := repository.NewDeployedProductRepository(repository.NewScoped(pool))
	req := domain.SearchDeployedProductsRequest{
		Pagination:    domain.Pagination{Limit: 50},
		DeploymentIDs: []string{depDeployOne, depDeployTwo},
	}
	ids := func(vs []domain.DeployedProductView) map[string]bool {
		out := map[string]bool{}
		for _, v := range vs {
			out[v.ID] = true
		}
		return out
	}

	t.Run("member sees both of their rows, including the legacy row with NULL project_id", func(t *testing.T) {
		views, _, err := repo.SearchDeployedProducts(depCtx(repository.SearchScope{ViewerEmail: depMemberOne}), req)
		if err != nil {
			t.Fatalf("SearchDeployedProducts: %v", err)
		}
		got := ids(views)
		if !got[depDPOneSet] || !got[depDPOneLegacy] {
			t.Errorf("member must see project_id-set row and legacy NULL-project row via its deployment; got %v", got)
		}
		if got[depDPTwo] {
			t.Errorf("member must NOT see project two's deployed product; got %v", got)
		}
	})
	t.Run("stranger sees nothing", func(t *testing.T) {
		views, total, err := repo.SearchDeployedProducts(depCtx(repository.SearchScope{ViewerEmail: depStranger}), req)
		if err != nil {
			t.Fatalf("SearchDeployedProducts: %v", err)
		}
		if len(views) != 0 || total != 0 {
			t.Errorf("stranger got %d rows, total %d; want none", len(views), total)
		}
	})
	t.Run("internal sees every row", func(t *testing.T) {
		views, _, err := repo.SearchDeployedProducts(depCtx(repository.SearchScope{Unrestricted: true}), req)
		if err != nil {
			t.Fatalf("SearchDeployedProducts: %v", err)
		}
		got := ids(views)
		if !got[depDPOneSet] || !got[depDPOneLegacy] || !got[depDPTwo] {
			t.Errorf("internal caller must see all three; got %v", got)
		}
	})
}

func TestDeploymentRLSIntegration_OrphanDeploymentIsInternalOnly(t *testing.T) {
	pool := caseStatsPool(t)
	seedDeploymentRLSFixture(t, pool)
	scoped := repository.NewScoped(pool)

	count := func(scope repository.SearchScope) int {
		t.Helper()
		var n int
		if err := scoped.QueryRow(depCtx(scope), `SELECT COUNT(*) FROM deployment WHERE id = $1`, depDeployOrphn).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	if n := count(repository.SearchScope{ViewerEmail: depMemberOne}); n != 0 {
		t.Errorf("a customer saw a deployment with no project (%d rows)", n)
	}
	if n := count(repository.SearchScope{Unrestricted: true}); n != 1 {
		t.Errorf("internal caller must see the orphan deployment, got %d rows", n)
	}
}

func TestDeploymentRLSIntegration_WritePolicies(t *testing.T) {
	pool := caseStatsPool(t)
	seedDeploymentRLSFixture(t, pool)
	scoped := repository.NewScoped(pool)
	member := depCtx(repository.SearchScope{ViewerEmail: depMemberOne})
	internal := depCtx(repository.SearchScope{Unrestricted: true})
	now := time.Now().UTC()

	insert := func(ctx context.Context, id, number string, project string) error {
		_, err := scoped.Exec(ctx, `INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id)
			VALUES ($1, $2, $2, $3, $3, $4, 'dep write test', 'QA', TRUE, $5)`, id, now, depCreatedBy, number, project)
		return err
	}

	t.Run("member can create a deployment in their own project", func(t *testing.T) {
		if err := insert(member, "a3000000-0000-0000-0000-0000000000a1", "DEP-NUM-W1", depProjectOne); err != nil {
			t.Fatalf("insert into own project: %v", err)
		}
	})
	t.Run("member cannot create a deployment in another project", func(t *testing.T) {
		err := insert(member, "a3000000-0000-0000-0000-0000000000a2", "DEP-NUM-W2", depProjectTwo)
		if err == nil || !repository.IsRLSPolicyViolation(err) {
			t.Fatalf("want an RLS policy violation (42501), got %v", err)
		}
	})
	t.Run("member cannot move their deployment into another project", func(t *testing.T) {
		_, err := scoped.Exec(member, `UPDATE deployment SET project_id = $1 WHERE id = $2`, depProjectTwo, depDeployOne)
		if err == nil || !repository.IsRLSPolicyViolation(err) {
			t.Fatalf("want an RLS policy violation (42501) from WITH CHECK, got %v", err)
		}
	})
	t.Run("member cannot update another project's deployment (row is invisible)", func(t *testing.T) {
		tag, err := scoped.Exec(member, `UPDATE deployment SET name = 'hijacked' WHERE id = $1`, depDeployTwo)
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if tag.RowsAffected() != 0 {
			t.Fatalf("member updated %d rows of another project's deployment", tag.RowsAffected())
		}
	})
	t.Run("member cannot delete even their own deployment (internal-only)", func(t *testing.T) {
		tag, err := scoped.Exec(member, `DELETE FROM deployment WHERE id = $1`, depDeployOne)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		if tag.RowsAffected() != 0 {
			t.Fatalf("member deleted %d rows", tag.RowsAffected())
		}
	})
	t.Run("member cannot write a deployed product into another project", func(t *testing.T) {
		_, err := scoped.Exec(member, `INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id, deployment_id, product_id)
			VALUES ('a5000000-0000-0000-0000-0000000000b1', $1, $1, $2, $2, 'DEPP-W1', TRUE, $3, $4, $5)`,
			now, depCreatedBy, depProjectTwo, depDeployTwo, depProductID)
		if err == nil || !repository.IsRLSPolicyViolation(err) {
			t.Fatalf("want an RLS policy violation (42501), got %v", err)
		}
	})
	t.Run("member cannot pair their own project_id with another project's deployment (insert)", func(t *testing.T) {
		_, err := scoped.Exec(member, `INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id, deployment_id, product_id)
			VALUES ('a5000000-0000-0000-0000-0000000000b2', $1, $1, $2, $2, 'DEPP-W2', TRUE, $3, $4, $5)`,
			now, depCreatedBy, depProjectOne, depDeployTwo, depProductID)
		if err == nil || !repository.IsRLSPolicyViolation(err) {
			t.Fatalf("want an RLS policy violation (42501) for project_id/deployment_id from different projects, got %v", err)
		}
	})
	t.Run("member cannot repoint their deployed product at another project's deployment (update)", func(t *testing.T) {
		_, err := scoped.Exec(member, `UPDATE deployed_product SET deployment_id = $1 WHERE id = $2`, depDeployTwo, depDPOneSet)
		if err == nil || !repository.IsRLSPolicyViolation(err) {
			t.Fatalf("want an RLS policy violation (42501) from WITH CHECK, got %v", err)
		}
	})
	t.Run("member can write a consistent deployed product in their own project", func(t *testing.T) {
		if _, err := scoped.Exec(member, `INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id, deployment_id, product_id)
			VALUES ('a5000000-0000-0000-0000-0000000000b3', $1, $1, $2, $2, 'DEPP-W3', TRUE, $3, $4, $5)`,
			now, depCreatedBy, depProjectOne, depDeployOne, depProductID); err != nil {
			t.Fatalf("consistent insert into own project: %v", err)
		}
	})
	t.Run("internal caller can create a deployment in any project", func(t *testing.T) {
		if err := insert(internal, "a3000000-0000-0000-0000-0000000000a3", "DEP-NUM-W3", depProjectTwo); err != nil {
			t.Fatalf("internal insert: %v", err)
		}
	})
}
