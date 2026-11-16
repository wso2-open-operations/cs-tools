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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Integration test: the project onboarding owner on the Postgres data source,
// including projects with no owner, an owner with a NULL email and an owner
// with a blank name. Runs against a real Postgres, skipped without
// CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ProjectOnboardingOwnerIntegration

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	pooUserFull   = "91000000-0000-0000-0000-000000000001"
	pooUserNoMail = "91000000-0000-0000-0000-000000000002"
	pooUserBlank  = "91000000-0000-0000-0000-000000000003"
	pooProjOwner  = "91000000-0000-0000-0000-0000000000a1"
	pooProjNone   = "91000000-0000-0000-0000-0000000000a2"
	pooProjNoMail = "91000000-0000-0000-0000-0000000000a3"
	pooProjBlank  = "91000000-0000-0000-0000-0000000000a4"
)

func TestProjectOnboardingOwnerIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id IN ($1,$2,$3,$4)`, pooProjOwner, pooProjNone, pooProjNoMail, pooProjBlank)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id IN ($1,$2,$3)`, pooUserFull, pooUserNoMail, pooUserBlank)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email)
		VALUES ($1, now(), now(), 'poo-full', 'Jane', 'Doe', 'jane.doe@example.com')`, pooUserFull)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name)
		VALUES ($1, now(), now(), 'poo-nomail', 'Jane', 'Doe')`, pooUserNoMail)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email)
		VALUES ($1, now(), now(), 'poo-blank', 'blank.name@example.com')`, pooUserBlank)

	insert := func(id, key string, owner *string) {
		mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id, onboarding_owner_id)
			VALUES ($1, now(), now(), 'poo-test', 'poo-test', $2::text, $2::text, 'sf-'||$2::text, $3)`, id, key, owner)
	}
	full, noMail, blank := pooUserFull, pooUserNoMail, pooUserBlank
	insert(pooProjOwner, "POOWNER", &full)
	insert(pooProjNone, "POONONE", nil)
	insert(pooProjNoMail, "POONOMAIL", &noMail)
	insert(pooProjBlank, "POOBLANK", &blank)

	// suspension_process_overridden (migration 0281): FALSE by default, TRUE
	// round-trips on both project reads.
	mustExec(`UPDATE project SET suspension_process_overridden = TRUE WHERE id = $1`, pooProjOwner)

	repo := repository.NewProjectRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true}

	get := func(id string) domain.ProjectDetailsView {
		t.Helper()
		v, err := repo.GetProjectByID(ctx, id, scope)
		if err != nil {
			t.Fatalf("GetProjectByID(%s): %v", id, err)
		}
		return v
	}

	if o := get(pooProjOwner).OnboardingOwner; o == nil || o.ID != pooUserFull || o.Name != "Jane Doe" || o.Email == nil || *o.Email != "jane.doe@example.com" {
		t.Errorf("detail owner = %+v", o)
	}
	if o := get(pooProjNone).OnboardingOwner; o != nil {
		t.Errorf("detail owner for ownerless project = %+v, want nil", o)
	}
	if o := get(pooProjNoMail).OnboardingOwner; o == nil || o.Email != nil || o.Name != "Jane Doe" {
		t.Errorf("detail null-email owner = %+v", o)
	}
	if o := get(pooProjBlank).OnboardingOwner; o == nil || o.ID != pooUserBlank || o.Name != "" {
		t.Errorf("detail blank-name owner = %+v", o)
	}

	if !get(pooProjOwner).SuspensionProcessOverridden || get(pooProjNone).SuspensionProcessOverridden {
		t.Errorf("detail suspensionProcessOverridden: owner project must be true, ownerless project false")
	}

	projects, total, err := repo.SearchProjects(ctx, domain.SearchProjectsRequest{
		SearchQuery: "POO",
		Pagination:  domain.Pagination{Limit: 50},
	}, scope)
	if err != nil {
		t.Fatalf("SearchProjects: %v", err)
	}
	if total != 4 || len(projects) != 4 {
		t.Fatalf("total/rows = %d/%d, want 4/4 (owner join must not change row set or count)", total, len(projects))
	}
	byID := map[string]domain.Project{}
	for _, p := range projects {
		byID[p.ID] = p
	}
	if o := byID[pooProjOwner].OnboardingOwner; o == nil || o.ID != pooUserFull {
		t.Errorf("search owner = %+v", o)
	}
	if o := byID[pooProjNone].OnboardingOwner; o != nil {
		t.Errorf("search owner for ownerless project = %+v, want nil", o)
	}
	if o := byID[pooProjNoMail].OnboardingOwner; o == nil || o.Email != nil {
		t.Errorf("search null-email owner = %+v", o)
	}
	if o := byID[pooProjBlank].OnboardingOwner; o == nil || o.Name != "" {
		t.Errorf("search blank-name owner = %+v", o)
	}
	if !byID[pooProjOwner].SuspensionProcessOverridden || byID[pooProjNone].SuspensionProcessOverridden {
		t.Errorf("search suspensionProcessOverridden: owner project must be true, ownerless project false")
	}
}
