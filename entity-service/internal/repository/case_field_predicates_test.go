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
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestEscalationEnumLabels(t *testing.T) {
	got, err := escalationEnumLabels([]string{"0", "3", " 5 "})
	if err != nil || strings.Join(got, ",") != "EL0,EL3,EL5" {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, bad := range []string{"6", "-1", "10", "x", "", "EL1"} {
		_, err := escalationEnumLabels([]string{"1", bad})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%q: want ValidationError, got %v", bad, err)
		}
	}
}

func TestCaseFieldPredicates(t *testing.T) {
	// Placeholders continue from argIdx and every predicate carries exactly one arg.
	preds, args, next, err := caseFieldPredicates(caseFieldSet{
		Severities:       []domain.CaseSeverity{domain.CaseSeverityCritical},
		EscalationLevels: []string{"3", "4"},
		States:           []domain.CaseState{"open"},
		DefaultTypes:     true,
	}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 4 { // default types + state + severity + escalation
		t.Fatalf("preds = %d %v", len(preds), preds)
	}
	if len(args) != 3 || next != 8 {
		t.Fatalf("args=%d next=%d, want 3 and 8 (the default-types predicate binds nothing)", len(args), next)
	}
	joined := strings.Join(preds, " | ")
	for _, want := range []string{"$5::text[]", "$6::case_severity_enum[]", "$7::text[]::case_escalation_level_enum[]"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %s", want, joined)
		}
	}

	// An anyOf branch with no types adds no type constraint of its own.
	branch, _, _, _ := caseFieldPredicates(caseFieldSet{Severities: []domain.CaseSeverity{domain.CaseSeverityHigh}}, 1)
	if len(branch) != 1 || strings.Contains(branch[0], "wi.type") {
		t.Errorf("branch preds = %v, want only the severity predicate", branch)
	}

	// Nothing set: no predicates, index untouched.
	none, noArgs, idx, _ := caseFieldPredicates(caseFieldSet{}, 3)
	if len(none) != 0 || len(noArgs) != 0 || idx != 3 {
		t.Errorf("empty set produced %v %v %d", none, noArgs, idx)
	}

	// An invalid escalation id is rejected before it can reach SQL.
	if _, _, _, err := caseFieldPredicates(caseFieldSet{EscalationLevels: []string{"9"}}, 1); err == nil {
		t.Error("escalation level 9 must be rejected")
	}
}

func TestCaseFieldSetFromGroup(t *testing.T) {
	f := caseFieldSetFromGroup(domain.CaseFilterGroup{
		Types: []string{"case"}, EscalationLevels: []string{"1"}, Tags: []string{"patch"}, ExcludeTags: []string{"s_dip"},
	})
	if f.DefaultTypes {
		t.Error("a branch must not default its types")
	}
	if len(f.Types) != 1 || len(f.EscalationLevels) != 1 || len(f.Tags) != 1 || len(f.ExcludeTags) != 1 {
		t.Errorf("fields not carried over: %+v", f)
	}
}
