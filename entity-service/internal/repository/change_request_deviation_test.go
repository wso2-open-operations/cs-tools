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

package repository

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestNormalizeImplementationDeviation(t *testing.T) {
	s := func(v string) *string { return &v }
	for _, blank := range []*string{nil, s(""), s("   "), s("\n\t")} {
		if got := normalizeImplementationDeviation(blank); got != nil {
			t.Errorf("normalizeImplementationDeviation(%q) = %q, want nil (NULL)", derefOr(blank), *got)
		}
	}
	v := s(" <p>kept as sent</p> ")
	if got := normalizeImplementationDeviation(v); got != v {
		t.Errorf("a non-blank deviation is not stored as sent: %v", got)
	}
}

func derefOr(v *string) string {
	if v == nil {
		return "<nil>"
	}
	return *v
}

func TestDeviationFinalMsg(t *testing.T) {
	for state, want := range map[string]string{
		crStateClosed:   "implementationDeviation can no longer be changed: the change request is closed, so its record is final",
		crStateCanceled: "implementationDeviation can no longer be changed: the change request is canceled, so its record is final",
		crStateRollback: "implementationDeviation can no longer be changed: the change request is rolled back, so its record is final",
	} {
		if got := deviationFinalMsg(state); got != want {
			t.Errorf("deviationFinalMsg(%s) = %q, want %q", state, got, want)
		}
	}
}

// A deviation-only PATCH takes the gate (the stored state is needed), but in a
// state that is not final it is never judged: no query runs.
func TestCheckDeviationEditable_OnlyFinalStatesAreJudged(t *testing.T) {
	s := func(v string) *string { return &v }
	if !changeRequestPatchNeedsGate(domain.PatchChangeRequestRequest{ImplementationDeviation: s("x")}) {
		t.Fatal("a deviation-only PATCH does not take the gate, so its state is never read")
	}
	for _, state := range []string{"", crStateNew, crStateAssess, crStateAuthorize, crStateCustomerApproval,
		crStateScheduled, crStateImplement, crStateReview, crStateCustomerReview} {
		// A nil querier would panic if it were used.
		if err := checkDeviationEditable(context.Background(), nil, "id", changeRequestGateSnapshot{state: state}, s("x")); err != nil {
			t.Errorf("state %q: %v, want the deviation accepted", state, err)
		}
	}
	if err := checkDeviationEditable(context.Background(), nil, "id", changeRequestGateSnapshot{state: crStateClosed}, nil); err != nil {
		t.Errorf("a PATCH without the deviation was judged in a final state: %v", err)
	}
}
