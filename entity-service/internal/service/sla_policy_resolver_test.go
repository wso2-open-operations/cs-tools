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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakePolicyLookupRepo answers FindPolicyByName from a fixed in-memory set
// of "name|target" -> policy, named the same deterministic way migration
// 0203 seeds them ("<severity> - <target> (CSM)").
type fakePolicyLookupRepo struct {
	policies map[string]repository.SLAPolicyRef
	calls    []string
	// errOnName, if set, makes FindPolicyByName fail with a real (non-
	// NotFoundError) error for that exact name -- used to simulate a
	// transient lookup failure (as opposed to a genuinely absent policy).
	errOnName string
}

func (f *fakePolicyLookupRepo) FindPolicyByName(_ context.Context, name, target string) (repository.SLAPolicyRef, error) {
	f.calls = append(f.calls, name+"|"+target)
	if f.errOnName != "" && name == f.errOnName {
		return repository.SLAPolicyRef{}, errors.New("db unavailable")
	}
	ref, ok := f.policies[name+"|"+target]
	if !ok {
		return repository.SLAPolicyRef{}, &apierror.NotFoundError{Msg: "no sla_policy found named " + name}
	}
	return ref, nil
}

func (f *fakePolicyLookupRepo) RegisterClock(context.Context, string, repository.SLAPolicyRef) (bool, error) {
	panic("not implemented")
}
func (f *fakePolicyLookupRepo) CompleteClock(context.Context, string, string) (bool, error) {
	panic("not implemented")
}
func (f *fakePolicyLookupRepo) SetPaused(context.Context, string, string, bool) (bool, error) {
	panic("not implemented")
}
func (f *fakePolicyLookupRepo) RecomputeActive(context.Context) (int, error) {
	panic("not implemented")
}
func (f *fakePolicyLookupRepo) ReviseClocks(context.Context, string, []repository.SLAPolicyRef) (int, error) {
	panic("not implemented")
}

func newFakePolicyLookupRepo() *fakePolicyLookupRepo {
	return &fakePolicyLookupRepo{
		policies: map[string]repository.SLAPolicyRef{
			"S0 - RESPONSE (CSM)|RESPONSE":     {ID: "s0-r", Target: "RESPONSE", Duration: 15 * time.Minute},
			"S0 - WORKAROUND (CSM)|WORKAROUND": {ID: "s0-w", Target: "WORKAROUND", Duration: 4 * time.Hour},
			"S0 - RESOLUTION (CSM)|RESOLUTION": {ID: "s0-res", Target: "RESOLUTION", Duration: 48 * time.Hour},
			"S1 - RESPONSE (CSM)|RESPONSE":     {ID: "s1-r", Target: "RESPONSE", Duration: time.Hour},
			"S2 - WORKAROUND (CSM)|WORKAROUND": {ID: "s2-w", Target: "WORKAROUND", Duration: 48 * time.Hour},
			"S3 - RESOLUTION (CSM)|RESOLUTION": {ID: "s3-res", Target: "RESOLUTION", Duration: 72 * time.Hour},
			"S4 - RESPONSE (CSM)|RESPONSE":     {ID: "s4-r", Target: "RESPONSE", Duration: 24 * time.Hour},
		},
	}
}

func TestSLAPolicyResolver_Resolve_MatchesDeterministicName(t *testing.T) {
	tests := []struct {
		name       string
		severity   domain.CaseSeverity
		clockType  string
		wantID     string
		wantTarget string
		wantName   string
	}{
		{"S1 response", domain.CaseSeverityCritical, slaClockTypeResponse, "s1-r", "RESPONSE", "S1 - RESPONSE (CSM)"},
		{"S2 workaround", domain.CaseSeverityHigh, slaClockTypeWorkaround, "s2-w", "WORKAROUND", "S2 - WORKAROUND (CSM)"},
		{"S3 resolution", domain.CaseSeverityMedium, slaClockTypeResolution, "s3-res", "RESOLUTION", "S3 - RESOLUTION (CSM)"},
		{"S4/LOW response -- the case that used to silently fail", domain.CaseSeverityLow, slaClockTypeResponse, "s4-r", "RESPONSE", "S4 - RESPONSE (CSM)"},
		{"S0/Catastrophic response", domain.CaseSeverityCatastrophic, slaClockTypeResponse, "s0-r", "RESPONSE", "S0 - RESPONSE (CSM)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakePolicyLookupRepo()
			r := newSLAPolicyResolver(repo)
			got, ok, err := r.resolve(context.Background(), tt.severity, tt.clockType)
			if err != nil {
				t.Fatalf("resolve() error = %v, want nil", err)
			}
			if !ok {
				t.Fatalf("resolve() ok = false, want true")
			}
			if got.ID != tt.wantID || got.Target != tt.wantTarget {
				t.Errorf("resolve() = %+v, want ID=%s Target=%s", got, tt.wantID, tt.wantTarget)
			}
			if len(repo.calls) != 1 || repo.calls[0] != tt.wantName+"|"+tt.wantTarget {
				t.Errorf("FindPolicyByName calls = %v, want exactly one lookup for %q -- no pattern/plan guessing", repo.calls, tt.wantName)
			}
		})
	}
}

func TestSLAPolicyResolver_Resolve_NoPolicySeeded(t *testing.T) {
	repo := newFakePolicyLookupRepo()
	r := newSLAPolicyResolver(repo)

	// S4/LOW has no workaround policy seeded at all (migration 0203 mirrors
	// sla_duration_policy, which has none for low/workaround either).
	_, ok, err := r.resolve(context.Background(), domain.CaseSeverityLow, slaClockTypeWorkaround)
	if err != nil {
		t.Fatalf("resolve() error = %v, want nil -- a genuinely absent policy is not a lookup failure", err)
	}
	if ok {
		t.Fatalf("resolve() ok = true, want false: no S4/WORKAROUND policy is seeded/faked at all")
	}
}

func TestSLAPolicyResolver_Resolve_UnknownClockType(t *testing.T) {
	repo := newFakePolicyLookupRepo()
	r := newSLAPolicyResolver(repo)

	_, ok, err := r.resolve(context.Background(), domain.CaseSeverityCritical, "bogus")
	if err != nil {
		t.Fatalf("resolve() error = %v, want nil", err)
	}
	if ok {
		t.Fatalf("resolve() ok = true, want false for an unrecognized clock type")
	}
	if len(repo.calls) != 0 {
		t.Errorf("FindPolicyByName calls = %v, want none -- an unknown clock type never reaches the repository", repo.calls)
	}
}

// TestSLAPolicyResolver_Resolve_InfrastructureErrorPropagates confirms
// resolve() propagates a genuine lookup failure as an error rather than
// silently treating it the same as "no policy configured" -- callers like
// resolveApplicablePolicies must be able to tell the two apart (see that
// function's own doc comment).
func TestSLAPolicyResolver_Resolve_InfrastructureErrorPropagates(t *testing.T) {
	repo := newFakePolicyLookupRepo()
	repo.errOnName = "S1 - RESPONSE (CSM)"
	r := newSLAPolicyResolver(repo)

	_, ok, err := r.resolve(context.Background(), domain.CaseSeverityCritical, slaClockTypeResponse)
	if err == nil {
		t.Fatalf("resolve() error = nil, want the repository's error propagated")
	}
	if ok {
		t.Fatalf("resolve() ok = true, want false when the repository call itself fails")
	}
}
