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
	"fmt"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeFullSLAEngineRepo is an in-memory repository.SLAEngineRepository fake
// exercising the real, condition-driven resolver (slaPolicyResolver) rather
// than a stub keyed by a deterministic name -- candidatesByTarget holds the
// real policy shapes (condition text and all) a test wants resolved,
// factsByCase the case's current CaseFacts, and activeByCaseTarget which
// policy a given (caseID, target) clock is currently bound to (for
// ApplyCaseStateEffects, which never re-resolves, only evaluates the
// already-bound policy's own conditions).
type fakeFullSLAEngineRepo struct {
	candidatesByTarget map[string][]repository.SLAPolicyRef
	factsByCase        map[string]repository.CaseFacts
	activeByCaseTarget map[string]repository.SLAPolicyRef

	registered     []string // "caseID|policyID"
	completed      []string // "caseID|target"
	paused         []string // "caseID|target|true" or "...|false"
	clockCancelled []string // "caseID|target" (CancelClock)
	cancelled      []string // "caseID" (ReviseClocks' bulk cancel)

	listErrOnTarget string
	getCaseFactsErr error
	registerErr     error
	registerOK      bool // if false, RegisterClock reports "already registered"
	cancelErr       error
	cancelCount     int
	// failReviseClocksTimes: 0 never fails; -1 always fails; N>0 fails the
	// next N calls then succeeds -- same semantics the old fixture used.
	failReviseClocksTimes int
	reviseClocksCalls     int
}

func newFakeFullSLAEngineRepo() *fakeFullSLAEngineRepo {
	return &fakeFullSLAEngineRepo{
		candidatesByTarget: map[string][]repository.SLAPolicyRef{},
		factsByCase:        map[string]repository.CaseFacts{},
		activeByCaseTarget: map[string]repository.SLAPolicyRef{},
		registerOK:         true,
	}
}

func (r *fakeFullSLAEngineRepo) FindPolicyByName(context.Context, string, string) (repository.SLAPolicyRef, error) {
	panic("not implemented")
}

func (r *fakeFullSLAEngineRepo) ListCandidatePolicies(_ context.Context, target string) ([]repository.SLAPolicyRef, error) {
	if r.listErrOnTarget != "" && target == r.listErrOnTarget {
		return nil, errors.New("db unavailable")
	}
	return r.candidatesByTarget[target], nil
}

func (r *fakeFullSLAEngineRepo) GetCaseFacts(_ context.Context, workItemID string) (repository.CaseFacts, error) {
	if r.getCaseFactsErr != nil {
		return repository.CaseFacts{}, r.getCaseFactsErr
	}
	return r.factsByCase[workItemID], nil
}

func (r *fakeFullSLAEngineRepo) RegisterClock(_ context.Context, workItemID string, policy repository.SLAPolicyRef, _ time.Time) (bool, error) {
	if r.registerErr != nil {
		return false, r.registerErr
	}
	r.registered = append(r.registered, workItemID+"|"+policy.ID)
	return r.registerOK, nil
}

func (r *fakeFullSLAEngineRepo) ReviseClocks(_ context.Context, workItemID string, policies []repository.SLAPolicyRef) (int, error) {
	r.reviseClocksCalls++
	if r.failReviseClocksTimes != 0 {
		if r.failReviseClocksTimes > 0 {
			r.failReviseClocksTimes--
		}
		return 0, r.cancelErr
	}
	r.cancelled = append(r.cancelled, workItemID)
	for _, policy := range policies {
		r.registered = append(r.registered, workItemID+"|"+policy.ID)
	}
	return r.cancelCount, nil
}

func (r *fakeFullSLAEngineRepo) CompleteClock(_ context.Context, workItemID, target string) (bool, error) {
	r.completed = append(r.completed, workItemID+"|"+target)
	return true, nil
}

func (r *fakeFullSLAEngineRepo) SetPaused(_ context.Context, workItemID, target string, paused bool) (bool, error) {
	r.paused = append(r.paused, fmt.Sprintf("%s|%s|%v", workItemID, target, paused))
	return true, nil
}

func (r *fakeFullSLAEngineRepo) CancelClock(_ context.Context, workItemID, target string) (bool, error) {
	r.clockCancelled = append(r.clockCancelled, workItemID+"|"+target)
	return true, nil
}

func (r *fakeFullSLAEngineRepo) ActiveClockPolicies(_ context.Context, workItemID, target string) ([]repository.SLAPolicyRef, error) {
	policy, ok := r.activeByCaseTarget[workItemID+"|"+target]
	if !ok {
		return nil, nil
	}
	return []repository.SLAPolicyRef{policy}, nil
}

func (r *fakeFullSLAEngineRepo) CompleteClockByPolicy(_ context.Context, workItemID, target, _ string) (bool, error) {
	r.completed = append(r.completed, workItemID+"|"+target)
	return true, nil
}

func (r *fakeFullSLAEngineRepo) CancelClockByPolicy(_ context.Context, workItemID, target, _ string) (bool, error) {
	r.clockCancelled = append(r.clockCancelled, workItemID+"|"+target)
	return true, nil
}

func (r *fakeFullSLAEngineRepo) SetPausedByPolicy(_ context.Context, workItemID, target, _ string, paused bool) (bool, error) {
	r.paused = append(r.paused, fmt.Sprintf("%s|%s|%v", workItemID, target, paused))
	return true, nil
}

// severityLadderCandidates seeds a clean, three-target severity ladder for
// case_type=case (S0 response/workaround/resolution, S4 response only) --
// enough to exercise RegisterCaseClocks/ReviseCaseClocks' own orchestration
// without re-testing the resolver's own ambiguity/classification logic
// (covered in sla_policy_resolver_test.go).
func severityLadderCandidates() map[string][]repository.SLAPolicyRef {
	return map[string][]repository.SLAPolicyRef{
		"RESPONSE": {
			{ID: "resp-s0", Name: "S0 Response", Target: "RESPONSE", StartCondition: "state=OPEN^case_type=case^priority=S0^cs_engineer_response_comment=false"},
			{ID: "resp-s4", Name: "S4 Response", Target: "RESPONSE", StartCondition: "state=OPEN^case_type=case^priority=S4^cs_engineer_response_comment=false"},
		},
		"WORKAROUND": {
			{ID: "work-s0", Name: "S0 Workaround", Target: "WORKAROUND", StartCondition: "case_type=case^priority=S0"},
		},
		"RESOLUTION": {
			{ID: "res-s0", Name: "S0 Resolution", Target: "RESOLUTION", StartCondition: "priority=S0^case_type=case"},
		},
	}
}

func TestSLAEngineService_RegisterCaseClocks_MatchesAllThreeTargets(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = severityLadderCandidates()
	repo.factsByCase["case-1"] = repository.CaseFacts{State: "OPEN", CaseType: "case"}
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-1", &sev, "")

	want := []string{"case-1|resp-s0", "case-1|work-s0", "case-1|res-s0"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
	for i, w := range want {
		if repo.registered[i] != w {
			t.Errorf("registered[%d] = %q, want %q", i, repo.registered[i], w)
		}
	}
}

func TestSLAEngineService_RegisterCaseClocks_LowSeverityRegistersResponseOnly(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = severityLadderCandidates()
	repo.factsByCase["case-2"] = repository.CaseFacts{State: "OPEN", CaseType: "case"}
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityLow
	svc.RegisterCaseClocks(context.Background(), "case-2", &sev, "")

	if len(repo.registered) != 1 || repo.registered[0] != "case-2|resp-s4" {
		t.Errorf("registered = %v, want exactly [case-2|resp-s4]: no WORKAROUND/RESOLUTION policy matches S4 in this fixture", repo.registered)
	}
}

// TestSLAEngineService_RegisterCaseClocks_SeverityIndependentPolicyStillRegisters
// is the real improvement a condition-driven resolver gives over the old
// severity-keyed map: a policy with no priority condition at all (e.g. the
// real "ServiceRequest SLA"/"Query - Resolution (Open Source)" rows) can
// still match and register even when the case has no severity at all.
func TestSLAEngineService_RegisterCaseClocks_SeverityIndependentPolicyStillRegisters(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = map[string][]repository.SLAPolicyRef{
		"RESPONSE": {
			{ID: "sr", Name: "ServiceRequest SLA", Target: "RESPONSE", StartCondition: "case_type=service_request^state=OPEN^case_typeISNOTEMPTY"},
		},
	}
	repo.factsByCase["case-sr"] = repository.CaseFacts{State: "OPEN", CaseType: "service_request"}
	svc := NewSLAEngineService(repo)

	svc.RegisterCaseClocks(context.Background(), "case-sr", nil, "")

	if len(repo.registered) != 1 || repo.registered[0] != "case-sr|sr" {
		t.Errorf("registered = %v, want [case-sr|sr] even with a nil severity", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_NoMatchRegistersNothing(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = severityLadderCandidates()
	repo.factsByCase["case-3"] = repository.CaseFacts{State: "CLOSED", CaseType: "announcement"}
	svc := NewSLAEngineService(repo)

	svc.RegisterCaseClocks(context.Background(), "case-3", nil, "")

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none for a case matching no policy at all", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_MissingPolicySkipsThatTargetOnly(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	candidates := severityLadderCandidates()
	delete(candidates, "WORKAROUND") // S0's middle target has nothing to resolve
	repo.candidatesByTarget = candidates
	repo.factsByCase["case-4"] = repository.CaseFacts{State: "OPEN", CaseType: "case"}
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-4", &sev, "")

	want := []string{"case-4|resp-s0", "case-4|res-s0"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

// TestSLAEngineService_RegisterCaseClocks_StopConditionAlreadyTrueSkipsRegistration
// is the regression test for a PR review finding: resolve() only ever
// evaluates start_condition, so a policy whose stop_condition ALSO already
// holds (upstream only ever attaches a definition when start holds AND stop
// doesn't) used to get registered anyway, immediately completed at ~0%
// elapsed by the very next state-effects pass -- a junk ACHIEVED row with
// no real tracking value. resolveApplicablePolicies must skip it before it
// is ever registered at all.
func TestSLAEngineService_RegisterCaseClocks_StopConditionAlreadyTrueSkipsRegistration(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = map[string][]repository.SLAPolicyRef{
		"RESOLUTION": {
			{
				ID:             "res-already-done",
				Name:           "Resolution (fix ETA already shared)",
				Target:         "RESOLUTION",
				StartCondition: "case_type=case^priority=S0",
				StopCondition:  "fix_eta_sharedISNOTEMPTY",
			},
		},
	}
	repo.factsByCase["case-fix-eta"] = repository.CaseFacts{State: "OPEN", CaseType: "case", FixEtaShared: true}
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-fix-eta", &sev, "")

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none -- the policy's own stop_condition already holds, so it must never be registered", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_FactsLookupFailureRegistersNothing(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.getCaseFactsErr = errors.New("db unavailable")
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-x", &sev, "")

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none when the case's own facts can't be read", repo.registered)
	}
}

func TestSLAEngineService_CompleteResponseClock(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	svc.CompleteResponseClock(context.Background(), "case-5")

	if len(repo.completed) != 1 || repo.completed[0] != "case-5|RESPONSE" {
		t.Errorf("completed = %v, want [case-5|RESPONSE]", repo.completed)
	}
}

func TestSLAEngineService_CompleteWorkaroundClock(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	svc.CompleteWorkaroundClock(context.Background(), "case-5")

	if len(repo.completed) != 1 || repo.completed[0] != "case-5|WORKAROUND" {
		t.Errorf("completed = %v, want [case-5|WORKAROUND]", repo.completed)
	}
}

// TestSLAEngineService_CompleteFixEtaSharedClocks verifies sharing a fix ETA
// with the customer completes BOTH the workaround and resolution clocks.
func TestSLAEngineService_CompleteFixEtaSharedClocks(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	svc.CompleteFixEtaSharedClocks(context.Background(), "case-5")

	want := []string{"case-5|WORKAROUND", "case-5|RESOLUTION"}
	if len(repo.completed) != len(want) {
		t.Fatalf("completed = %v, want %v", repo.completed, want)
	}
	for i, w := range want {
		if repo.completed[i] != w {
			t.Errorf("completed[%d] = %q, want %q", i, repo.completed[i], w)
		}
	}
}

// caseStateEffectsPolicies seeds the three targets' ACTIVE clock policies
// for ApplyCaseStateEffects, matching the real shape of
// "P1 - Response/Workaround/Resolution (Open Source)" closely enough to
// exercise stop/pause/resume generically -- RESPONSE has no
// pause_condition at all (matching every real RESPONSE policy), so it is
// never paused, exactly like the old hardcoded switch, but now as a
// consequence of the real data rather than special-cased Go logic.
func caseStateEffectsPolicies(caseID string) map[string]repository.SLAPolicyRef {
	return map[string]repository.SLAPolicyRef{
		caseID + "|RESPONSE": {
			ID: "resp", Target: "RESPONSE",
			StopCondition: "cs_engineer_response_comment=true^ORstateINCLOSED",
		},
		caseID + "|WORKAROUND": {
			ID: "work", Target: "WORKAROUND",
			StopCondition:   "stateINCLOSED^ORfix_eta_sharedISNOTEMPTY",
			PauseCondition:  "stateINAWAITING_INFO,SOLUTION_PROPOSED^ORu_workaround_providedISNOTEMPTY",
			ResumeCondition: "stateINWORK_IN_PROGRESS,WAITING_ON_WSO2^u_workaround_providedISEMPTY",
		},
		caseID + "|RESOLUTION": {
			ID: "res", Target: "RESOLUTION",
			StopCondition:   "stateINCLOSED",
			PauseCondition:  "stateINAWAITING_INFO,SOLUTION_PROPOSED",
			ResumeCondition: "stateINWORK_IN_PROGRESS,WAITING_ON_WSO2,REOPENED",
		},
	}
}

func TestSLAEngineService_ApplyCaseStateEffects(t *testing.T) {
	tests := []struct {
		name          string
		state         domain.CaseState
		wantPaused    []string
		wantCompleted []string
	}{
		{
			"awaiting info pauses workaround+resolution, leaves response alone",
			domain.CaseStateAwaitingInfo,
			[]string{"case-6|WORKAROUND|true", "case-6|RESOLUTION|true"},
			nil,
		},
		{
			"solution proposed pauses workaround+resolution",
			domain.CaseStateSolutionProposed,
			[]string{"case-6|WORKAROUND|true", "case-6|RESOLUTION|true"},
			nil,
		},
		{
			"closed completes all three clocks (via each policy's own stop_condition)",
			domain.CaseStateClosed,
			nil,
			[]string{"case-6|RESPONSE", "case-6|WORKAROUND", "case-6|RESOLUTION"},
		},
		{
			"work in progress resumes workaround+resolution",
			domain.CaseStateWorkInProgress,
			[]string{"case-6|WORKAROUND|false", "case-6|RESOLUTION|false"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeFullSLAEngineRepo()
			repo.factsByCase["case-6"] = repository.CaseFacts{CaseType: "case"}
			repo.activeByCaseTarget = caseStateEffectsPolicies("case-6")
			svc := NewSLAEngineService(repo)

			svc.ApplyCaseStateEffects(context.Background(), "case-6", tt.state)

			if len(repo.paused) != len(tt.wantPaused) {
				t.Fatalf("paused = %v, want %v", repo.paused, tt.wantPaused)
			}
			for i, w := range tt.wantPaused {
				if repo.paused[i] != w {
					t.Errorf("paused[%d] = %q, want %q", i, repo.paused[i], w)
				}
			}
			if len(repo.completed) != len(tt.wantCompleted) {
				t.Fatalf("completed = %v, want %v", repo.completed, tt.wantCompleted)
			}
			for i, w := range tt.wantCompleted {
				if repo.completed[i] != w {
					t.Errorf("completed[%d] = %q, want %q", i, repo.completed[i], w)
				}
			}
			if len(repo.clockCancelled) != 0 {
				t.Errorf("clockCancelled = %v, want none", repo.clockCancelled)
			}
		})
	}
}

// TestSLAEngineService_ApplyCaseStateEffects_CancelCondition confirms a
// clock whose bound policy's cancel_condition now matches (e.g. the case's
// type changed away from what the policy required) is cancelled rather
// than paused/resumed/completed.
func TestSLAEngineService_ApplyCaseStateEffects_CancelCondition(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.factsByCase["case-14"] = repository.CaseFacts{CaseType: "hosting"}
	repo.activeByCaseTarget = map[string]repository.SLAPolicyRef{
		"case-14|RESOLUTION": {
			ID: "res", Target: "RESOLUTION",
			StopCondition:   "stateINCLOSED",
			CancelCondition: "case_type!=case",
		},
	}
	svc := NewSLAEngineService(repo)

	svc.ApplyCaseStateEffects(context.Background(), "case-14", domain.CaseStateWorkInProgress)

	if len(repo.clockCancelled) != 1 || repo.clockCancelled[0] != "case-14|RESOLUTION" {
		t.Fatalf("clockCancelled = %v, want [case-14|RESOLUTION]", repo.clockCancelled)
	}
	if len(repo.completed) != 0 || len(repo.paused) != 0 {
		t.Errorf("completed=%v paused=%v, want neither when cancel_condition matches", repo.completed, repo.paused)
	}
}

// TestSLAEngineService_ApplyCaseStateEffects_ClosedSafetyNet_CompletesAClockWhoseOwnStopConditionNeverReachesClosed
// is the regression test for the code-review-found gap: the condition-driven
// engine only completes a clock when its own bound policy's stop_condition
// evaluates true -- so a policy whose stop_condition text simply never
// mentions CLOSED (a gap in the real data, or a future sync mistake) used to
// leave its clock running forever once the case actually closed, with
// nothing left to notice. The safety net force-completes it anyway.
func TestSLAEngineService_ApplyCaseStateEffects_ClosedSafetyNet_CompletesAClockWhoseOwnStopConditionNeverReachesClosed(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.factsByCase["case-16"] = repository.CaseFacts{CaseType: "case", State: "CLOSED"}
	repo.activeByCaseTarget = map[string]repository.SLAPolicyRef{
		// Deliberately never matches CLOSED, or anything else, for this
		// case's facts -- simulating a policy whose stop_condition has a gap.
		"case-16|RESOLUTION": {ID: "res", Target: "RESOLUTION", StopCondition: "priority=S4"},
	}
	svc := NewSLAEngineService(repo)

	svc.ApplyCaseStateEffects(context.Background(), "case-16", domain.CaseStateClosed)

	if len(repo.completed) != 1 || repo.completed[0] != "case-16|RESOLUTION" {
		t.Fatalf("completed = %v, want [case-16|RESOLUTION] via the CLOSED safety net", repo.completed)
	}
	if len(repo.paused) != 0 || len(repo.clockCancelled) != 0 {
		t.Errorf("paused=%v clockCancelled=%v, want neither", repo.paused, repo.clockCancelled)
	}
}

// TestSLAEngineService_ApplyCaseStateEffects_ClosedSafetyNet_DoesNotDoubleCompleteWhenStopConditionAlreadyMatched
// confirms the safety net doesn't fire a second, redundant CompleteClock
// when the policy's own stop_condition already completed the clock.
func TestSLAEngineService_ApplyCaseStateEffects_ClosedSafetyNet_DoesNotDoubleCompleteWhenStopConditionAlreadyMatched(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.factsByCase["case-17"] = repository.CaseFacts{CaseType: "case", State: "CLOSED"}
	repo.activeByCaseTarget = map[string]repository.SLAPolicyRef{
		"case-17|RESOLUTION": {ID: "res", Target: "RESOLUTION", StopCondition: "stateINCLOSED"},
	}
	svc := NewSLAEngineService(repo)

	svc.ApplyCaseStateEffects(context.Background(), "case-17", domain.CaseStateClosed)

	if len(repo.completed) != 1 || repo.completed[0] != "case-17|RESOLUTION" {
		t.Fatalf("completed = %v, want exactly one CompleteClock call, not a duplicate safety-net call", repo.completed)
	}
}

// TestSLAEngineService_ApplyCaseStateEffects_ClosedSafetyNet_DoesNotFireWhenCancelConditionMatched
// confirms a clock that CLOSED legitimately cancels (rather than completes)
// is left cancelled, not also force-completed by the safety net.
func TestSLAEngineService_ApplyCaseStateEffects_ClosedSafetyNet_DoesNotFireWhenCancelConditionMatched(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.factsByCase["case-18"] = repository.CaseFacts{CaseType: "hosting", State: "CLOSED"}
	repo.activeByCaseTarget = map[string]repository.SLAPolicyRef{
		"case-18|RESOLUTION": {ID: "res", Target: "RESOLUTION", StopCondition: "priority=S4", CancelCondition: "case_type!=case"},
	}
	svc := NewSLAEngineService(repo)

	svc.ApplyCaseStateEffects(context.Background(), "case-18", domain.CaseStateClosed)

	if len(repo.clockCancelled) != 1 || repo.clockCancelled[0] != "case-18|RESOLUTION" {
		t.Fatalf("clockCancelled = %v, want [case-18|RESOLUTION]", repo.clockCancelled)
	}
	if len(repo.completed) != 0 {
		t.Errorf("completed = %v, want none -- the clock was already settled by cancel_condition", repo.completed)
	}
}

// TestSLAEngineService_ApplyCaseStateEffects_NoActiveClockIsANoOp confirms a
// target with no currently open clock is simply skipped -- no panics, no
// spurious repo calls.
func TestSLAEngineService_ApplyCaseStateEffects_NoActiveClockIsANoOp(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.factsByCase["case-15"] = repository.CaseFacts{CaseType: "case"}
	svc := NewSLAEngineService(repo)

	svc.ApplyCaseStateEffects(context.Background(), "case-15", domain.CaseStateClosed)

	if len(repo.completed) != 0 || len(repo.paused) != 0 || len(repo.clockCancelled) != 0 {
		t.Errorf("expected no repo calls at all, got completed=%v paused=%v cancelled=%v", repo.completed, repo.paused, repo.clockCancelled)
	}
}

func TestSLAEngineService_ReviseCaseClocks_CancelsThenRegistersFresh(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = severityLadderCandidates()
	repo.factsByCase["case-8"] = repository.CaseFacts{State: "OPEN", CaseType: "case"}
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.ReviseCaseClocks(context.Background(), "case-8", &sev, "")

	if len(repo.cancelled) != 1 || repo.cancelled[0] != "case-8" {
		t.Fatalf("cancelled = %v, want [case-8]", repo.cancelled)
	}
	want := []string{"case-8|resp-s0", "case-8|work-s0", "case-8|res-s0"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

func TestSLAEngineService_ReviseCaseClocks_NothingHappensIfRepoFails(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = severityLadderCandidates()
	repo.factsByCase["case-11"] = repository.CaseFacts{State: "OPEN", CaseType: "case"}
	repo.cancelErr = errors.New("db unavailable")
	repo.failReviseClocksTimes = -1
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.ReviseCaseClocks(context.Background(), "case-11", &sev, "")

	if len(repo.cancelled) != 0 {
		t.Errorf("cancelled = %v, want none recorded (ReviseClocks errored)", repo.cancelled)
	}
	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none recorded (ReviseClocks errored -- atomic, all-or-nothing)", repo.registered)
	}
	if repo.reviseClocksCalls != 2 {
		t.Errorf("ReviseClocks calls = %d, want 2 (both retry attempts exhausted)", repo.reviseClocksCalls)
	}
}

func TestSLAEngineService_ReviseCaseClocks_RetriesOnceAndRecovers(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = severityLadderCandidates()
	repo.factsByCase["case-13"] = repository.CaseFacts{State: "OPEN", CaseType: "case"}
	repo.cancelErr = errors.New("db unavailable")
	repo.failReviseClocksTimes = 1
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.ReviseCaseClocks(context.Background(), "case-13", &sev, "")

	if repo.reviseClocksCalls != 2 {
		t.Fatalf("ReviseClocks calls = %d, want 2 (one failure, one successful retry)", repo.reviseClocksCalls)
	}
	if len(repo.cancelled) != 1 || repo.cancelled[0] != "case-13" {
		t.Errorf("cancelled = %v, want [case-13] -- the retry should have succeeded", repo.cancelled)
	}
}

func TestSLAEngineService_ReviseCaseClocks_NoMatchStillCancels(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.factsByCase["case-10"] = repository.CaseFacts{}
	svc := NewSLAEngineService(repo)

	svc.ReviseCaseClocks(context.Background(), "case-10", nil, "")

	if len(repo.cancelled) != 1 || repo.cancelled[0] != "case-10" {
		t.Fatalf("cancelled = %v, want [case-10]", repo.cancelled)
	}
	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none (no policy matches empty facts)", repo.registered)
	}
}

// TestSLAEngineService_ReviseCaseClocks_AbortsOnPolicyLookupFailure is the
// regression test for a real CodeRabbit finding on the old design: resolve()
// drops a target from the resolved list both when its policy is genuinely
// unmatched AND when the lookup itself fails -- resolveApplicablePolicies
// surfaces the latter as lookupFailed. ReviseCaseClocks must NOT call
// ReviseClocks with an incomplete list in that case.
func TestSLAEngineService_ReviseCaseClocks_AbortsOnPolicyLookupFailure(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = severityLadderCandidates()
	repo.factsByCase["case-12"] = repository.CaseFacts{State: "OPEN", CaseType: "case"}
	repo.listErrOnTarget = "WORKAROUND"

	svc := NewSLAEngineService(repo)
	sev := domain.CaseSeverityCatastrophic
	svc.ReviseCaseClocks(context.Background(), "case-12", &sev, "")

	if len(repo.cancelled) != 0 {
		t.Errorf("cancelled = %v, want none -- a policy lookup failure must abort before ReviseClocks is ever called", repo.cancelled)
	}
	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none -- a policy lookup failure must abort before ReviseClocks is ever called", repo.registered)
	}
}
