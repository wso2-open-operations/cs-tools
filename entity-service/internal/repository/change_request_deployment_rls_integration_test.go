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

// change_request_deployment is the junction csm-sync-service owns (its own
// 0136 migration), and the sync writes it the way it writes work_item_watcher:
// INSERT ... ON CONFLICT (change_request_id, deployment_id) DO UPDATE. Migration
// 0191 turned FORCE ROW LEVEL SECURITY on for it without an UPDATE policy, which
// refuses that conflict branch for every caller; migration 0205 adds the
// internal-only UPDATE policy. This is the behaviour the schema guard
// (TestRLSSchemaIntegration_EveryProtectedTableHasAPolicyForEveryCommand) only
// asserts structurally: an internal session (the sync connects with
// app.is_internal = true) can re-write existing rows, a customer session still
// cannot touch them.
//
// Row-level security is only observable from a role that does not bypass it.
// The migrate script's csm_app is one; a superuser DSN (which the other change
// request integration tests are happy with) skips this test.
func TestChangeRequestDeploymentRLS_InternalSessionCanUpsertWhatTheSyncWrites(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage}
	})

	var bypass bool
	if err := f.pool.QueryRow(context.Background(),
		`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypass); err != nil {
		t.Fatalf("read the connected role: %v", err)
	}
	if bypass {
		t.Skip("the connected role bypasses row-level security (superuser or BYPASSRLS): connect as the non-superuser csm_app to exercise the policy")
	}

	// The sync's upsert shape: every pair already exists, so every row takes
	// the conflict branch.
	tag, err := f.scoped.Exec(f.sys, `
		INSERT INTO change_request_deployment (id, change_request_id, deployment_id)
		SELECT gen_random_uuid(), change_request_id, deployment_id
		  FROM change_request_deployment WHERE change_request_id = $1
		ON CONFLICT (change_request_id, deployment_id) DO UPDATE SET id = EXCLUDED.id`, id)
	if err != nil {
		t.Fatalf("an internal session's upsert over existing rows was refused: %v", err)
	}
	if tag.RowsAffected() != 2 {
		t.Fatalf("internal upsert touched %d rows, want 2", tag.RowsAffected())
	}

	// A customer session (not internal, a member of no project) still cannot
	// UPDATE them: no policy grants it, so it affects nothing.
	outsider := repository.WithCallerIdentity(context.Background(),
		repository.SearchScope{Unrestricted: false, ViewerEmail: "nobody-rls-outsider@example.test"})
	tag, err = f.scoped.Exec(outsider, `UPDATE change_request_deployment SET id = gen_random_uuid() WHERE change_request_id = $1`, id)
	if err != nil {
		t.Fatalf("outsider UPDATE: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Fatalf("a customer session updated %d change_request_deployment rows, want 0", tag.RowsAffected())
	}
}
