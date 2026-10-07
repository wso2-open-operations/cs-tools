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

// Regression test for a project member's create being refused by the
// extension-table INSERT policies (case_write 0147; engagement/
// service_request/security_report_analysis_write 0151): the create query
// inserts work_item and the extension row in one statement, so the policy's
// work_item lookup cannot see the new row and refused every non-internal
// caller with 404. Exercised through the real CaseRepository.CreateCase as a
// customer caller, against a real Postgres with those migrations applied and
// row-level security in force -- connect as a role that does NOT bypass RLS.
// Skipped without CASE_CREATE_RLS_TEST_DSN.
//
//	CASE_CREATE_RLS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseCreateMemberRLS

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	ccmAccountID  = "c7000000-0000-0000-0000-000000000001"
	ccmProjectID  = "c7000000-0000-0000-0000-000000000002"
	ccmOtherProj  = "c7000000-0000-0000-0000-000000000003"
	ccmDeployment = "c7000000-0000-0000-0000-000000000004"
	ccmDeployed   = "c7000000-0000-0000-0000-000000000005"
	ccmUserID     = "c7000000-0000-0000-0000-000000000006"
	ccmContactID  = "c7000000-0000-0000-0000-000000000007"
	ccmCustomer   = "ccm-customer@test.local"
)

func seedCaseCreateMemberFixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CASE_CREATE_RLS_TEST_DSN")
	if dsn == "" {
		t.Skip("CASE_CREATE_RLS_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE project_id IN ($1, $2)`, ccmProjectID, ccmOtherProj)
		_, _ = scoped.Exec(sys, `DELETE FROM deployed_product WHERE id = $1`, ccmDeployed)
		_, _ = scoped.Exec(sys, `DELETE FROM deployment WHERE id = $1`, ccmDeployment)
		_, _ = scoped.Exec(sys, `DELETE FROM project_contact WHERE email = $1`, ccmCustomer)
		_, _ = scoped.Exec(sys, `DELETE FROM project WHERE id IN ($1, $2)`, ccmProjectID, ccmOtherProj)
		_, _ = scoped.Exec(sys, `DELETE FROM account_contact WHERE id = $1`, ccmContactID)
		_, _ = scoped.Exec(sys, `DELETE FROM "user" WHERE id = $1`, ccmUserID)
		_, _ = scoped.Exec(sys, `DELETE FROM account WHERE id = $1`, ccmAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	for _, s := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, $2, $2, 't', 't', 'CCM Account', 'CCM-ACC')`, []any{ccmAccountID, now}},
		{`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, account_id) VALUES ($1, $2, $2, 't', 't', 'CCMPROJ', 'CCM', $3), ($4, $2, $2, 't', 't', 'CCMOTHER', 'CCM other', $3)`, []any{ccmProjectID, now, ccmAccountID, ccmOtherProj}},
		{`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, project_id) VALUES ($1, $2, $2, 't', 't', 'CCM-DEP', 'CCM prod', true, $3)`, []any{ccmDeployment, now, ccmProjectID}},
		{`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, deployment_id) VALUES ($1, $2, $2, 't', 't', 'CCM-DP', $3)`, []any{ccmDeployed, now, ccmDeployment}},
		{`INSERT INTO "user" (id, created_on, updated_on, user_name, email, user_type) VALUES ($1, $2, $2, 'ccm', $3, 'EXTERNAL')`, []any{ccmUserID, now, ccmCustomer}},
		{`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id) VALUES ($1, $2, $2, 't', 't', 'CCM Customer', $3)`, []any{ccmContactID, now, ccmAccountID}},
		{`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state) VALUES (gen_random_uuid(), $1, $1, 't', 't', $2, $3, $4, 'REGISTERED')`, []any{now, ccmCustomer, ccmContactID, ccmProjectID}},
	} {
		// Through Scoped as the system: deployment and friends carry RLS too.
		if _, err := scoped.Exec(sys, s.sql, s.args...); err != nil {
			t.Fatalf("seed (%.60s): %v", s.sql, err)
		}
	}
	return pool
}

func ccmRequest(caseType, projectID string) domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type: caseType, CreatedBy: ccmUserID, ProjectID: projectID,
		DeploymentID: ccmDeployment, DeployedProductID: ccmDeployed,
		Subject: "ccm " + caseType, Description: "d", Severity: domain.CaseSeverityLow, IssueType: domain.CaseIssueTypeQuestion,
	}
}

func TestCaseCreateMemberRLS(t *testing.T) {
	pool := seedCaseCreateMemberFixture(t)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	customer := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: ccmCustomer})

	for _, caseType := range []string{"case", "service_request"} {
		t.Run("member creates "+caseType, func(t *testing.T) {
			c, err := repo.CreateCase(customer, ccmRequest(caseType, ccmProjectID))
			if err != nil {
				t.Fatalf("CreateCase as a registered project member: %v", err)
			}
			if c.ID == "" || c.Number == "" {
				t.Errorf("created case = %+v", c)
			}
		})
	}

	t.Run("non-member is still refused", func(t *testing.T) {
		_, err := repo.CreateCase(customer, ccmRequest("service_request", ccmOtherProj))
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("CreateCase in a project the caller is not a member of: got %T %v, want NotFoundError", err, err)
		}
	})

	t.Run("internal caller unchanged", func(t *testing.T) {
		if _, err := repo.CreateCase(repository.WithSystemIdentity(context.Background()), ccmRequest("case", ccmOtherProj)); err != nil {
			t.Fatalf("CreateCase as an internal caller: %v", err)
		}
	})
}
