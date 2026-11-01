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
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeCandidatePolicyRepo answers ListCandidatePolicies from a fixed
// in-memory set keyed by target -- real policy shapes (condition text and
// all), not the old deterministic CSM-seeded names.
type fakeCandidatePolicyRepo struct {
	byTarget map[string][]repository.SLAPolicyRef
	calls    []string
	// errOnTarget, if set, makes ListCandidatePolicies fail for that target
	// -- simulates a transient lookup failure (as opposed to "no policy
	// configured").
	errOnTarget string
}

func (f *fakeCandidatePolicyRepo) ListCandidatePolicies(_ context.Context, target string) ([]repository.SLAPolicyRef, error) {
	f.calls = append(f.calls, target)
	if f.errOnTarget != "" && target == f.errOnTarget {
		return nil, errors.New("db unavailable")
	}
	return f.byTarget[target], nil
}

func (f *fakeCandidatePolicyRepo) FindPolicyByName(context.Context, string, string) (repository.SLAPolicyRef, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) GetCaseFacts(context.Context, string) (repository.CaseFacts, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) RegisterClock(context.Context, string, repository.SLAPolicyRef, time.Time) (bool, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) CompleteClock(context.Context, string, string) (bool, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) CompleteClockByPolicy(context.Context, string, string, string) (bool, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) SetPaused(context.Context, string, string, bool) (bool, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) SetPausedByPolicy(context.Context, string, string, string, bool) (bool, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) CancelClock(context.Context, string, string) (bool, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) CancelClockByPolicy(context.Context, string, string, string) (bool, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) ActiveClockPolicies(context.Context, string, string) ([]repository.SLAPolicyRef, error) {
	panic("not implemented")
}
func (f *fakeCandidatePolicyRepo) ReviseClocks(context.Context, string, []repository.SLAPolicyRef) (int, error) {
	panic("not implemented")
}

// newFakeCandidatePolicyRepo seeds a small, realistic slice of the real
// table's own shapes (same names/conditions as the live data, trimmed to
// what these tests need).
func newFakeCandidatePolicyRepo() *fakeCandidatePolicyRepo {
	return &fakeCandidatePolicyRepo{
		byTarget: map[string][]repository.SLAPolicyRef{
			"RESPONSE": {
				{ID: "p1-r-ms", Name: "P1 - Response (Managed Services)", Target: "RESPONSE", Duration: 15 * time.Minute,
					StartCondition: "state=OPEN^case_type=case^priority=S0^cs_engineer_response_comment=false"},
				{ID: "p1-r-os", Name: "P1 - Response (Open Source)", Target: "RESPONSE", Duration: time.Hour,
					StartCondition: "state=OPEN^priority=S1^case_type=case^cs_engineer_response_comment=false^NQstate=OPEN^case_type=security_report_analysis^cs_engineer_response_comment=false"},
				{ID: "svc-request", Name: "ServiceRequest SLA", Target: "RESPONSE", Duration: 24 * time.Hour, Schedule: "WSO2 Business days",
					StartCondition: "case_type=service_request^state=OPEN^case_typeISNOTEMPTY"},
				{ID: "incident-escalation", Name: "Escalated case response - 4 hours", Target: "RESPONSE", Duration: 4 * time.Hour,
					StartCondition: "active_escalation.escalation_severity=abc^active_escalation.state=101"},
			},
			"WORKAROUND": {
				{ID: "p1-w-os", Name: "P1 - Workaround (Open Source)", Target: "WORKAROUND", Duration: 24 * time.Hour,
					StartCondition: "project_type=Subscription^ORproject_type=Managed Cloud Subscription^case_type=case^priority=S1"},
			},
			"RESOLUTION": {
				{ID: "p0-res-os", Name: "P0 - Resolution (Open Source)", Target: "RESOLUTION", Duration: 48 * time.Hour,
					StartCondition: "priority=S0^case_type=case"},
			},
		},
	}
}

func TestSLAPolicyResolver_Resolve_MatchesRealCondition(t *testing.T) {
	repo := newFakeCandidatePolicyRepo()
	r := newSLAPolicyResolver(repo)

	facts := repository.CaseFacts{State: "OPEN", Severity: "S0", CaseType: "case", CSEngineerResponseComment: false}
	got, err := r.resolve(context.Background(), "RESPONSE", facts)
	if err != nil {
		t.Fatalf("resolve() error = %v, want nil", err)
	}
	if len(got) != 1 || got[0].ID != "p1-r-ms" {
		t.Errorf("resolve() = %+v, want exactly [P1 - Response (Managed Services)]", got)
	}
}

// TestSLAPolicyResolver_Resolve_NQGroupMatches confirms a case whose facts
// only satisfy the SECOND NQ-separated group of a policy's start_condition
// (P1 - Response (Open Source)'s own security_report_analysis branch)
// still resolves.
func TestSLAPolicyResolver_Resolve_NQGroupMatches(t *testing.T) {
	repo := newFakeCandidatePolicyRepo()
	r := newSLAPolicyResolver(repo)

	facts := repository.CaseFacts{State: "OPEN", CaseType: "security_report_analysis", CSEngineerResponseComment: false}
	got, err := r.resolve(context.Background(), "RESPONSE", facts)
	if err != nil {
		t.Fatalf("resolve() error = %v, want nil", err)
	}
	if len(got) != 1 || got[0].ID != "p1-r-os" {
		t.Fatalf("resolve() = %+v, want exactly [P1 - Response (Open Source)] via its second NQ group", got)
	}
}

// TestSLAPolicyResolver_Resolve_SkipsNonCasePolicy confirms a case's facts
// never accidentally match an incident/escalation-style candidate (which
// carries fields this engine never builds evaluation support for).
func TestSLAPolicyResolver_Resolve_SkipsNonCasePolicy(t *testing.T) {
	repo := newFakeCandidatePolicyRepo()
	r := newSLAPolicyResolver(repo)

	// No case in these tests' facts ever has state=101 (that's an
	// escalation-table-only code), so this also exercises "zero matches".
	facts := repository.CaseFacts{State: "CLOSED", CaseType: "case"}
	got, err := r.resolve(context.Background(), "RESPONSE", facts)
	if err != nil {
		t.Fatalf("resolve() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("resolve() = %+v, want none: nothing should match a closed case with no response comment yet", got)
	}
}

func TestSLAPolicyResolver_Resolve_NoMatch(t *testing.T) {
	repo := newFakeCandidatePolicyRepo()
	r := newSLAPolicyResolver(repo)

	facts := repository.CaseFacts{State: "OPEN", Severity: "S4", CaseType: "announcement"}
	got, err := r.resolve(context.Background(), "WORKAROUND", facts)
	if err != nil {
		t.Fatalf("resolve() error = %v, want nil -- a genuinely absent policy is not a lookup failure", err)
	}
	if len(got) != 0 {
		t.Fatalf("resolve() = %+v, want none: no WORKAROUND policy matches an announcement", got)
	}
}

// TestSLAPolicyResolver_Resolve_InfrastructureErrorPropagates confirms
// resolve() propagates a genuine lookup failure as an error rather than
// silently treating it the same as "no policy configured" -- callers like
// resolveApplicablePolicies must be able to tell the two apart (see that
// function's own doc comment).
func TestSLAPolicyResolver_Resolve_InfrastructureErrorPropagates(t *testing.T) {
	repo := newFakeCandidatePolicyRepo()
	repo.errOnTarget = "RESPONSE"
	r := newSLAPolicyResolver(repo)

	got, err := r.resolve(context.Background(), "RESPONSE", repository.CaseFacts{State: "OPEN"})
	if err == nil {
		t.Fatalf("resolve() error = nil, want the repository's error propagated")
	}
	if len(got) != 0 {
		t.Fatalf("resolve() = %+v, want none when the repository call itself fails", got)
	}
}

// TestSLAPolicyResolver_Resolve_BothMatchesReturned covers the real, live
// overlap between "Hosting Q/T - Response (Managed Service)" and
// "Query - Response (Open Source)" (both can match a hosting_query case) --
// the resolver must return BOTH, so each registers its own independent
// clock, and must do so deterministically (same order every call), not
// silently pick one and drop the other the way an earlier version did.
func TestSLAPolicyResolver_Resolve_BothMatchesReturned(t *testing.T) {
	repo := &fakeCandidatePolicyRepo{byTarget: map[string][]repository.SLAPolicyRef{
		"RESPONSE": {
			{ID: "a-hosting-qt", Name: "Hosting Q/T - Response (Managed Service)", Target: "RESPONSE", Duration: 24 * time.Hour,
				StartCondition: "case_type=hosting_query^ORcase_type=hosting_task"},
			{ID: "b-query-open-source", Name: "Query - Response (Open Source)", Target: "RESPONSE", Duration: 24 * time.Hour,
				StartCondition: "case_type!=case^case_type!=hosting^state=OPEN^case_type!=announcement^case_typeISNOTEMPTY^case_type!=engagement^case_type!=security_report_analysis^case_type!=service_request"},
		},
	}}
	r := newSLAPolicyResolver(repo)

	facts := repository.CaseFacts{State: "OPEN", CaseType: "hosting_query"}
	got1, err := r.resolve(context.Background(), "RESPONSE", facts)
	if err != nil || len(got1) != 2 {
		t.Fatalf("resolve() = %+v err=%v, want both policies matched", got1, err)
	}
	got2, err2 := r.resolve(context.Background(), "RESPONSE", facts)
	if err2 != nil || len(got2) != 2 {
		t.Fatalf("second resolve() = %+v err=%v, want both policies matched", got2, err2)
	}
	if got1[0].ID != got2[0].ID || got1[1].ID != got2[1].ID {
		t.Fatalf("resolve() returned a different order across calls (%q,%q then %q,%q) -- must be deterministic",
			got1[0].ID, got1[1].ID, got2[0].ID, got2[1].ID)
	}
	if got1[0].ID != "a-hosting-qt" || got1[1].ID != "b-query-open-source" {
		t.Errorf("resolve() = [%q, %q], want [a-hosting-qt, b-query-open-source] (ListCandidatePolicies' own id order)", got1[0].ID, got1[1].ID)
	}
}
