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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// recordingSLAEngineRepo is an in-memory repository.SLAEngineRepository
// fake that records every call, backed by the same fixed policy set
// fakePolicyLookupRepo uses (sla_policy_resolver_test.go), so
// SLAEngineService tests exercise the real resolver, not a stub.
type recordingSLAEngineRepo struct {
	fakePolicyLookupRepo
	registered []string // "workItemID|policyID"
	completed  []string // "workItemID|target"
	paused     []string // "workItemID|target|true" or "...|false"
	cancelled  []string // "workItemID"

	registerErr error
	registerOK  bool // if false, RegisterClock reports "already registered"
	cancelErr   error
	cancelCount int
	// failReviseClocksTimes controls how ReviseClocks fails, if at all:
	// 0 means never fail (ignores cancelErr); -1 means always fail with
	// cancelErr; a positive N means fail with cancelErr for the next N
	// calls (decrementing each time) then succeed -- used to test
	// ReviseCaseClocks' bounded retry actually recovers from a transient
	// failure, as opposed to the -1 "fails forever" case.
	failReviseClocksTimes int
	reviseClocksCalls     int
}

func newRecordingSLAEngineRepo() *recordingSLAEngineRepo {
	return &recordingSLAEngineRepo{fakePolicyLookupRepo: *newFakePolicyLookupRepo(), registerOK: true}
}

func (r *recordingSLAEngineRepo) RegisterClock(_ context.Context, workItemID string, policy repository.SLAPolicyRef) (bool, error) {
	if r.registerErr != nil {
		return false, r.registerErr
	}
	r.registered = append(r.registered, workItemID+"|"+policy.ID)
	return r.registerOK, nil
}

// ReviseClocks fakes the real repository's atomic cancel-then-register-many:
// on cancelErr, nothing is recorded at all (mirroring a rolled-back
// transaction -- neither the cancel nor any registration "took"), same as
// the real ReviseClocks' all-or-nothing guarantee.
func (r *recordingSLAEngineRepo) ReviseClocks(_ context.Context, workItemID string, policies []repository.SLAPolicyRef) (int, error) {
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

func (r *recordingSLAEngineRepo) CompleteClock(_ context.Context, workItemID, target string) (bool, error) {
	r.completed = append(r.completed, workItemID+"|"+target)
	return true, nil
}

func (r *recordingSLAEngineRepo) SetPaused(_ context.Context, workItemID, target string, paused bool) (bool, error) {
	r.paused = append(r.paused, fmt.Sprintf("%s|%s|%v", workItemID, target, paused))
	return true, nil
}

func TestSLAEngineService_RegisterCaseClocks_CatastrophicRegistersAllThree(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-1", &sev, "")

	want := []string{"case-1|s0-r", "case-1|s0-w", "case-1|s0-res"}
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
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityLow
	svc.RegisterCaseClocks(context.Background(), "case-2", &sev, "")

	if len(repo.registered) != 1 || repo.registered[0] != "case-2|s4-r" {
		t.Errorf("registered = %v, want exactly [case-2|s4-r] (Query/LOW: response only, matching the old sla_clocks design's LOW-severity behavior)", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_NilSeverityRegistersNothing(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	svc.RegisterCaseClocks(context.Background(), "case-3", nil, "")

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none for a case with no severity", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_MissingPolicySkipsThatClockTypeOnly(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	// Remove the workaround policy so CATASTROPHIC's middle clock type has
	// nothing to resolve -- response and resolution must still register.
	delete(repo.policies, "S0 - WORKAROUND (CSM)|WORKAROUND")
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-4", &sev, "")

	want := []string{"case-4|s0-r", "case-4|s0-res"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

func TestSLAEngineService_CompleteResponseClock(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	svc.CompleteResponseClock(context.Background(), "case-5")

	if len(repo.completed) != 1 || repo.completed[0] != "case-5|RESPONSE" {
		t.Errorf("completed = %v, want [case-5|RESPONSE]", repo.completed)
	}
}

func TestSLAEngineService_CompleteWorkaroundClock(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	svc.CompleteWorkaroundClock(context.Background(), "case-5")

	if len(repo.completed) != 1 || repo.completed[0] != "case-5|WORKAROUND" {
		t.Errorf("completed = %v, want [case-5|WORKAROUND]", repo.completed)
	}
}

// TestSLAEngineService_CompleteFixEtaSharedClocks verifies sharing a fix ETA
// with the customer completes BOTH the workaround and resolution clocks --
// unlike CompleteWorkaroundClock/CompleteResponseClock, which each complete
// exactly one target.
func TestSLAEngineService_CompleteFixEtaSharedClocks(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
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

func TestSLAEngineService_ApplyCaseStateEffects(t *testing.T) {
	tests := []struct {
		name          string
		state         domain.CaseState
		wantPaused    []string
		wantCompleted []string
	}{
		{
			"awaiting info pauses both",
			domain.CaseStateAwaitingInfo,
			[]string{"case-6|WORKAROUND|true", "case-6|RESOLUTION|true"},
			nil,
		},
		{
			"solution proposed pauses both",
			domain.CaseStateSolutionProposed,
			[]string{"case-6|WORKAROUND|true", "case-6|RESOLUTION|true"},
			nil,
		},
		{
			"closed resumes+completes all three clocks",
			domain.CaseStateClosed,
			[]string{"case-6|RESOLUTION|false", "case-6|WORKAROUND|false"},
			[]string{"case-6|RESOLUTION", "case-6|WORKAROUND", "case-6|RESPONSE"},
		},
		{
			"work in progress resumes both",
			domain.CaseStateWorkInProgress,
			[]string{"case-6|WORKAROUND|false", "case-6|RESOLUTION|false"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRecordingSLAEngineRepo()
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
		})
	}
}

// TestSLAEngineService_ReviseCaseClocks_CancelsThenRegistersFresh confirms
// a severity change on an existing case cancels every one of its existing
// clocks before registering entirely new ones for the new severity, in
// that order -- so the new registration is never blocked by RegisterClock's
// own NOT EXISTS guard seeing a still-active old-severity row, and the new
// clocks carry no relation to the old ones (fresh start_on, zero elapsed).
func TestSLAEngineService_ReviseCaseClocks_CancelsThenRegistersFresh(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic // P0
	svc.ReviseCaseClocks(context.Background(), "case-8", &sev, "")

	if len(repo.cancelled) != 1 || repo.cancelled[0] != "case-8" {
		t.Fatalf("cancelled = %v, want [case-8]", repo.cancelled)
	}
	want := []string{"case-8|s0-r", "case-8|s0-w", "case-8|s0-res"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
	for i, w := range want {
		if repo.registered[i] != w {
			t.Errorf("registered[%d] = %q, want %q", i, repo.registered[i], w)
		}
	}
}

// TestSLAEngineService_ReviseCaseClocks_DowngradeStillCancelsEveryClockType
// covers a severity DOWNGRADE: ReviseClocks cancels every active clock type
// on the case regardless of what the new severity resolves to, so a clock
// type no longer applicable after the downgrade (e.g. LOW dropping
// "workaround"/"resolution") is cancelled too, not left running. This is
// the one already-cancelled call recorded per case, independent of how
// many (fewer, for a downgrade) clock types get freshly registered.
func TestSLAEngineService_ReviseCaseClocks_DowngradeStillCancelsEveryClockType(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityLow // Query -- response only
	svc.ReviseCaseClocks(context.Background(), "case-9", &sev, "")

	if len(repo.cancelled) != 1 || repo.cancelled[0] != "case-9" {
		t.Fatalf("cancelled = %v, want [case-9]", repo.cancelled)
	}
	if len(repo.registered) != 1 || repo.registered[0] != "case-9|s4-r" {
		t.Errorf("registered = %v, want [case-9|s4-r]", repo.registered)
	}
}

// TestSLAEngineService_ReviseCaseClocks_NothingHappensIfRepoFails confirms
// ReviseClocks' atomicity guarantee at the service layer: when the
// repository call errors (simulating any failure inside its transaction --
// the cancel, a registration, or the commit itself), NEITHER the cancel NOR
// any registration is recorded, so the case's prior clocks are left exactly
// as they were rather than cancelled with no replacement.
func TestSLAEngineService_ReviseCaseClocks_NothingHappensIfRepoFails(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	repo.cancelErr = errors.New("db unavailable")
	repo.failReviseClocksTimes = -1 // fail every call, including both retry attempts
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

// TestSLAEngineService_ReviseCaseClocks_RetriesOnceAndRecovers confirms the
// bounded retry actually recovers from a transient failure: ReviseClocks
// fails on its first call and succeeds on the second, and the case ends up
// with its clocks cancelled and re-registered exactly as if the first call
// had never failed.
func TestSLAEngineService_ReviseCaseClocks_RetriesOnceAndRecovers(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	repo.cancelErr = errors.New("db unavailable")
	repo.failReviseClocksTimes = 1 // fail once, then succeed
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityCatastrophic
	svc.ReviseCaseClocks(context.Background(), "case-13", &sev, "")

	if repo.reviseClocksCalls != 2 {
		t.Fatalf("ReviseClocks calls = %d, want 2 (one failure, one successful retry)", repo.reviseClocksCalls)
	}
	if len(repo.cancelled) != 1 || repo.cancelled[0] != "case-13" {
		t.Errorf("cancelled = %v, want [case-13] -- the retry should have succeeded", repo.cancelled)
	}
	want := []string{"case-13|s0-r", "case-13|s0-w", "case-13|s0-res"}
	if len(repo.registered) != len(want) {
		t.Errorf("registered = %v, want %v -- the retry should have succeeded", repo.registered, want)
	}
}

// TestSLAEngineService_ReviseCaseClocks_NilSeverityStillCancels confirms a
// nil newSeverity (RegisterCaseClocks' own no-op case) still runs the
// cancel step -- a case moving to an unrecognised/nil severity must not
// keep its old clocks running just because the new one can't be resolved.
func TestSLAEngineService_ReviseCaseClocks_NilSeverityStillCancels(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	svc.ReviseCaseClocks(context.Background(), "case-10", nil, "")

	if len(repo.cancelled) != 1 || repo.cancelled[0] != "case-10" {
		t.Fatalf("cancelled = %v, want [case-10]", repo.cancelled)
	}
	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none (nil severity)", repo.registered)
	}
}

// TestSLAEngineService_ReviseCaseClocks_AbortsOnPolicyLookupFailure is the
// regression test for a real CodeRabbit finding: resolve() drops a clock
// type from the resolved list both when its policy is genuinely absent AND
// when the lookup itself fails (e.g. a database blip) -- resolveApplicablePolicies
// surfaces the latter case as lookupFailed. ReviseCaseClocks must NOT call
// ReviseClocks with an incomplete list in that case: doing so would cancel
// the case's existing clocks and commit no replacement for the clock type
// whose policy lookup failed. Confirms repo.ReviseClocks (and therefore
// CancelActiveClocks) is never even called.
func TestSLAEngineService_ReviseCaseClocks_AbortsOnPolicyLookupFailure(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	repo.errOnName = "S0 - WORKAROUND (CSM)" // one of S0's three clock types

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

// TestSLAEngineService_RegisterCaseClocks_MediumSeverityUsesOnlyFakedClockType
// confirms registration depends on severity alone now (no project/plan
// lookup involved at all): only S3 - Resolution is faked, so a Medium-
// severity case registers only that one clock type, with response/
// workaround for S3 simply missing from the fake set (same "no fallback
// duration" behavior as a genuinely unconfigured policy).
func TestSLAEngineService_RegisterCaseClocks_MediumSeverityUsesOnlyFakedClockType(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	sev := domain.CaseSeverityMedium // S3
	svc.RegisterCaseClocks(context.Background(), "case-7", &sev, "")

	if len(repo.registered) != 1 || repo.registered[0] != "case-7|s3-res" {
		t.Errorf("registered = %v, want [case-7|s3-res]", repo.registered)
	}
}
