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

// Integration test: PATCH of a project's suspensionProcessOverridden flag on
// the Postgres data source. Runs against a real Postgres, skipped without
// CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ProjectSuspensionOverriddenUpdateIntegration

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const psoProject = "93000000-0000-0000-0000-0000000000a1"

func TestProjectSuspensionOverriddenUpdateIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() { _, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, psoProject) }
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id, wso2_closure_state, suspension_process_state)
		VALUES ($1, now(), now(), 'pso-test', 'pso-test', 'PSOPROJ', 'PSOPROJ', 'sf-PSOPROJ', 'SUSPENDED', '{"based_on_due_invoices":{"event_type":"suspend"}}'::jsonb)`, psoProject)

	repo := repository.NewProjectRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true}
	boolp := func(b bool) *bool { return &b }
	patch := func(req domain.ProjectUpdateRequest) domain.ProjectUpdateResult {
		t.Helper()
		res, err := repo.UpdateProject(ctx, psoProject, req, "jane.doe@example.com")
		if err != nil {
			t.Fatalf("UpdateProject(%+v): %v", req, err)
		}
		return res
	}
	type snapshot struct{ closure, state, by string }
	snap := func() snapshot {
		t.Helper()
		var s snapshot
		if err := pool.QueryRow(ctx, `SELECT wso2_closure_state::text, suspension_process_state::text, updated_by FROM project WHERE id = $1`, psoProject).Scan(&s.closure, &s.state, &s.by); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		return s
	}
	flag := func() bool {
		t.Helper()
		got, err := repo.GetProjectByID(ctx, psoProject, scope)
		if err != nil {
			t.Fatalf("GetProjectByID: %v", err)
		}
		return got.SuspensionProcessOverridden
	}

	if flag() {
		t.Fatal("flag must default to false")
	}
	before := snap()

	// true: echoed in the response, stored, stamped, closure state untouched.
	res := patch(domain.ProjectUpdateRequest{SuspensionProcessOverridden: boolp(true)})
	if res.SuspensionProcessOverridden == nil || !*res.SuspensionProcessOverridden || !flag() {
		t.Errorf("after true: response %v, read %v", res.SuspensionProcessOverridden, flag())
	}
	after := snap()
	if after.closure != before.closure || after.state != before.state {
		t.Errorf("closure state/suspension state changed: before %+v after %+v", before, after)
	}
	if after.by != "jane.doe@example.com" {
		t.Errorf("updated_by = %q, want the caller", after.by)
	}

	// omitted (another field only) leaves it unchanged.
	patch(domain.ProjectUpdateRequest{VerificationEnabled: boolp(true)})
	if !flag() {
		t.Error("an unrelated patch changed the flag")
	}

	// combined with another field: both apply.
	res = patch(domain.ProjectUpdateRequest{SuspensionProcessOverridden: boolp(false), VerificationEnabled: boolp(false)})
	if res.VerificationEnabled == nil || *res.VerificationEnabled || flag() {
		t.Errorf("combined patch: verification %v, flag %v", res.VerificationEnabled, flag())
	}

	// nil is a no-op alongside another field.
	patch(domain.ProjectUpdateRequest{SuspensionProcessOverridden: boolp(true)})
	patch(domain.ProjectUpdateRequest{SuspensionProcessOverridden: nil, VerificationEnabled: boolp(true)})
	if !flag() {
		t.Error("nil flag changed the stored value")
	}
	if last := snap(); last.closure != before.closure || last.state != before.state {
		t.Errorf("closure state/suspension state changed: before %+v after %+v", before, last)
	}
}
