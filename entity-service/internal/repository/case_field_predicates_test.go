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
	// Placeholders continue from argIdx; every predicate carries exactly one
	// arg except State, which carries two -- see its own branch's doc
	// comment: one array reused (via separate casts) across the four
	// case-like enum types that share its label set, and a second, smaller
	// array specifically for announcement_state_enum's own OPEN/CLOSE-only
	// vocabulary.
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
	if len(args) != 4 || next != 9 {
		t.Fatalf("args=%d next=%d, want 4 and 9 (default-types binds nothing; state binds two)", len(args), next)
	}
	joined := strings.Join(preds, " | ")
	for _, want := range []string{
		"c.state = ANY($5::case_state_enum[])",
		"eng.state = ANY($5::engagement_state_enum[])",
		"sr.state = ANY($5::service_request_state_enum[])",
		"sra.state = ANY($5::security_report_analysis_state_enum[])",
		"ann.state = ANY($6::announcement_state_enum[])",
		"$7::case_severity_enum[]",
		"$8::text[]::case_escalation_level_enum[]",
	} {
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

// TestCaseFieldPredicates_StateAnnouncementMapping is the regression guard
// for the part of the State branch most likely to silently break: the
// second, announcement-only array must map CLOSED -> CLOSE (the label
// announcement_state_enum actually has) and must DROP every state that
// enum doesn't define at all, rather than passing it through -- casting an
// unsupported label into announcement_state_enum would error the whole
// query, not just fail to match that one branch.
func TestCaseFieldPredicates_StateAnnouncementMapping(t *testing.T) {
	_, args, _, err := caseFieldPredicates(caseFieldSet{
		States: []domain.CaseState{
			domain.CaseStateOpen, domain.CaseStateClosed, domain.CaseStateWorkInProgress,
		},
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 {
		t.Fatalf("args = %d, want 2 (the shared array, then the announcement-mapped one)", len(args))
	}
	shared, ok := args[0].([]string)
	if !ok || strings.Join(shared, ",") != "OPEN,CLOSED,WORK_IN_PROGRESS" {
		t.Errorf("shared array = %v, want [OPEN CLOSED WORK_IN_PROGRESS] unchanged", args[0])
	}
	annStates, ok := args[1].([]string)
	if !ok || strings.Join(annStates, ",") != "OPEN,CLOSE" {
		t.Errorf("announcement array = %v, want [OPEN CLOSE] (CLOSED mapped, WORK_IN_PROGRESS dropped)", args[1])
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
