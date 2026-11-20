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

package service

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// familyTestSubject is a test-only subject that names a policy family.
type familyTestSubject struct {
	caseSLASubject
	family string
}

func (f familyTestSubject) Family() string { return f.family }

// IsPolicy rejects everything: if the family path consulted it, nothing would match.
func (f familyTestSubject) IsPolicy(...string) bool { return false }

func (f familyTestSubject) Facts() slacondition.Facts {
	return caseConditionFacts{c: repository.CaseFacts{State: "OPEN", Severity: "S1", CaseType: "case"}}
}

// familyRecordingRepo wraps fakeCandidatePolicyRepo and records which list
// method the resolver used.
type familyRecordingRepo struct {
	*fakeCandidatePolicyRepo
	legacyCalls []string
	familyCalls []string
	family      []repository.SLAPolicyRef
}

func (r *familyRecordingRepo) ListCandidatePolicies(ctx context.Context, target string) ([]repository.SLAPolicyRef, error) {
	r.legacyCalls = append(r.legacyCalls, target)
	return r.fakeCandidatePolicyRepo.ListCandidatePolicies(ctx, target)
}

func (r *familyRecordingRepo) ListFamilyCandidatePolicies(_ context.Context, target, family string) ([]repository.SLAPolicyRef, error) {
	r.familyCalls = append(r.familyCalls, target+"/"+family)
	return r.family, nil
}

func TestSLAPolicyResolver_FamilySubject_ListsByFamilyAndIgnoresMarkers(t *testing.T) {
	repo := &familyRecordingRepo{
		fakeCandidatePolicyRepo: &fakeCandidatePolicyRepo{byTarget: map[string][]repository.SLAPolicyRef{
			"RESPONSE": {{ID: "legacy", Name: "legacy", Target: "RESPONSE", Duration: time.Hour, StartCondition: "state=OPEN^case_type=case"}},
		}},
		family: []repository.SLAPolicyRef{
			// No case or incident marker at all: only the family tag selects it.
			{ID: "fam", Name: "fam", Target: "RESPONSE", Duration: time.Hour, Source: "CSM", AppliesTo: "PRODUCT_ISSUE", StartCondition: "state=OPEN"},
		},
	}
	r := newSLAPolicyResolver(repo)

	got, err := r.resolveSubject(context.Background(), "RESPONSE", familyTestSubject{family: "PRODUCT_ISSUE"})
	if err != nil {
		t.Fatalf("resolveSubject: %v", err)
	}
	if len(got) != 1 || got[0].ID != "fam" {
		t.Fatalf("got %+v, want exactly the family row", got)
	}
	if len(repo.legacyCalls) != 0 || len(repo.familyCalls) != 1 || repo.familyCalls[0] != "RESPONSE/PRODUCT_ISSUE" {
		t.Errorf("legacy calls %v, family calls %v; want only RESPONSE/PRODUCT_ISSUE via the family list", repo.legacyCalls, repo.familyCalls)
	}
}

func TestSLAPolicyResolver_LegacySubject_NeverUsesFamilyList(t *testing.T) {
	repo := &familyRecordingRepo{fakeCandidatePolicyRepo: newFakeCandidatePolicyRepo()}
	r := newSLAPolicyResolver(repo)

	facts := repository.CaseFacts{State: "OPEN", Severity: "S0", CaseType: "case"}
	if _, err := r.resolve(context.Background(), "RESPONSE", facts); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(repo.familyCalls) != 0 || len(repo.legacyCalls) != 1 {
		t.Errorf("legacy calls %v, family calls %v; want one legacy call and no family call", repo.legacyCalls, repo.familyCalls)
	}
	if (caseSLASubject{}).Family() != "" || (incidentSLASubject{}).Family() != "" {
		t.Error("case and incident subjects must have an empty family")
	}
}
