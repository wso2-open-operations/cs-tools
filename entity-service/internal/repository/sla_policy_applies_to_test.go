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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// A SERVICENOW-sourced row tagged applies_to = 'PRODUCT_ISSUE' whose start
// condition would match a case must be invisible to ListCandidatePolicies (the
// case and incident listing) and visible to ListFamilyCandidatePolicies.
func TestSLAAppliesTo_FamilyRowIsolatedFromCaseListing(t *testing.T) {
	pool := slaCharPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	repo := repository.NewSLAEngineRepository(scoped, "")

	const untagged = "5c4a0000-0000-0000-0000-0000000000a1"
	const tagged = "5c4a0000-0000-0000-0000-0000000000a2"
	const csmTagged = "5c4a0000-0000-0000-0000-0000000000a3"
	ids := []string{untagged, tagged, csmTagged}
	cleanup := func() {
		for _, id := range ids {
			_, _ = scoped.Exec(ctx, `DELETE FROM sla_policy WHERE id = $1::uuid`, id)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	insert := func(id, name, source string, appliesTo any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, `
			INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, is_active, target, duration, source,
			                        start_condition, applies_to)
			VALUES ($1::uuid, NOW(), NOW(), $2, $2, $3, TRUE, 'RESPONSE', INTERVAL '1 hour', $4::sla_source_enum,
			        'state=OPEN^case_type=case', $5)`,
			id, slaCharActor, slaCharPolicyPrefix+name, source, appliesTo); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	insert(untagged, "applies_to untagged", "SERVICENOW", nil)
	insert(tagged, "applies_to tagged servicenow", "SERVICENOW", "PRODUCT_ISSUE")
	insert(csmTagged, "applies_to tagged csm", "CSM", "PRODUCT_ISSUE")

	has := func(refs []repository.SLAPolicyRef, id string) *repository.SLAPolicyRef {
		for i := range refs {
			if refs[i].ID == id {
				return &refs[i]
			}
		}
		return nil
	}

	legacy, err := repo.ListCandidatePolicies(ctx, "RESPONSE")
	if err != nil {
		t.Fatalf("ListCandidatePolicies: %v", err)
	}
	if has(legacy, untagged) == nil {
		t.Errorf("the untagged row must still be a case candidate")
	}
	if has(legacy, untagged).AppliesTo != "" {
		t.Errorf("a NULL applies_to must scan as the empty string")
	}
	if has(legacy, tagged) != nil || has(legacy, csmTagged) != nil {
		t.Errorf("ListCandidatePolicies returned a row tagged for another family")
	}

	fam, err := repo.ListFamilyCandidatePolicies(ctx, "RESPONSE", "PRODUCT_ISSUE")
	if err != nil {
		t.Fatalf("ListFamilyCandidatePolicies: %v", err)
	}
	if has(fam, tagged) == nil || has(fam, csmTagged) == nil {
		t.Errorf("ListFamilyCandidatePolicies must return tagged rows whatever their source, got %d rows", len(fam))
	}
	if r := has(fam, tagged); r != nil && r.AppliesTo != "PRODUCT_ISSUE" {
		t.Errorf("AppliesTo = %q, want PRODUCT_ISSUE", r.AppliesTo)
	}
	if has(fam, untagged) != nil {
		t.Errorf("ListFamilyCandidatePolicies returned an untagged row")
	}
	other, err := repo.ListFamilyCandidatePolicies(ctx, "WORKAROUND", "PRODUCT_ISSUE")
	if err != nil {
		t.Fatalf("ListFamilyCandidatePolicies(WORKAROUND): %v", err)
	}
	if has(other, tagged) != nil {
		t.Errorf("a RESPONSE row was returned for target WORKAROUND")
	}
}
