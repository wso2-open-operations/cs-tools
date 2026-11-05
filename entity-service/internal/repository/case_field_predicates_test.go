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

	// AccountIDs produces a wi.account_id predicate, the same shape as
	// ProjectIDs' own wi.project_id one -- a real, reported bug had this
	// field accepted by the parser but silently dropped here, so an
	// accountId+in filter matched every case regardless of account.
	accountPreds, accountArgs, _, err := caseFieldPredicates(caseFieldSet{AccountIDs: []string{"acc-1"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(accountPreds) != 1 || !strings.Contains(accountPreds[0], "wi.account_id = ANY($1::uuid[])") {
		t.Fatalf("accountPreds = %v, want a wi.account_id predicate", accountPreds)
	}
	if len(accountArgs) != 1 || accountArgs[0].([]string)[0] != "acc-1" {
		t.Fatalf("accountArgs = %v, want [[acc-1]]", accountArgs)
	}

	// An invalid escalation id is rejected before it can reach SQL.
	if _, _, _, err := caseFieldPredicates(caseFieldSet{EscalationLevels: []string{"9"}}, 1); err == nil {
		t.Error("escalation level 9 must be rejected")
	}
}

func TestCaseLikeStateLookupClause(t *testing.T) {
	// Scoped to the types actually requested: only that type's own branch,
	// matching the shape every dashboard widget's search sends (a {type}
	// filter alongside a {state} filter).
	got := caseLikeStateLookupClause([]string{"case"}, "$3::text[]", false)
	want := `wi.id = ANY(ARRAY(SELECT id FROM "case" WHERE state = ANY(ARRAY(SELECT s FROM unnest(enum_range(NULL::case_state_enum)) s WHERE s::TEXT = ANY($3::text[])))))`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Several requested types each contribute their own branch, UNION ALL'd.
	got = caseLikeStateLookupClause([]string{"case", "engagement"}, "$1::text[]", false)
	for _, want := range []string{
		`SELECT id FROM "case" WHERE state = ANY(ARRAY(SELECT s FROM unnest(enum_range(NULL::case_state_enum)) s WHERE s::TEXT = ANY($1::text[])))`,
		`SELECT id FROM engagement WHERE state = ANY(ARRAY(SELECT s FROM unnest(enum_range(NULL::engagement_state_enum)) s WHERE s::TEXT = ANY($1::text[])))`,
		" UNION ALL ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}

	// No types named: falls back to every case-like type (the DefaultTypes/
	// no-constraint-branch case), covering the same ground the COALESCE it
	// replaces always implicitly covered.
	got = caseLikeStateLookupClause(nil, "$1::text[]", false)
	for _, table := range caseLikeStateLookupAllTypes {
		if !strings.Contains(got, table) {
			t.Errorf("missing table %q in %s", table, got)
		}
	}

	// announcement gets the same CLOSE->CLOSED normalization caseLikeStateColumn applies.
	got = caseLikeStateLookupClause([]string{"announcement"}, "$1::text[]", false)
	if !strings.Contains(got, `CASE WHEN state::TEXT = 'CLOSE' THEN 'CLOSED' ELSE state::TEXT END`) {
		t.Errorf("announcement branch missing CLOSE normalization: %s", got)
	}

	// negate=true renders an anti-join, for the ExcludeStates (notIn) filter --
	// never "<> ALL(ARRAY(...))", which compares every row with every excluded
	// id and took tens of seconds for "notIn closed" on production-volume data.
	got = caseLikeStateLookupClause([]string{"case"}, "$1::text[]", true)
	wantNegated := `NOT EXISTS (SELECT 1 FROM (SELECT id FROM "case" WHERE state = ANY(ARRAY(SELECT s FROM unnest(enum_range(NULL::case_state_enum)) s WHERE s::TEXT = ANY($1::text[])))) excluded WHERE excluded.id = wi.id)`
	if got != wantNegated {
		t.Errorf("negate=true = %s\nwant %s", got, wantNegated)
	}
	if strings.Contains(got, "<> ALL(ARRAY(") {
		t.Errorf("negate=true must not render <> ALL(ARRAY(...)): %s", got)
	}
	got = caseLikeStateLookupClause(nil, "$1::text[]", true)
	if !strings.HasPrefix(got, "NOT EXISTS (SELECT 1 FROM (") || !strings.Contains(got, " UNION ALL ") || !strings.HasSuffix(got, ") excluded WHERE excluded.id = wi.id)") {
		t.Errorf("negate=true over every type should be one anti-join over the UNION ALL: %s", got)
	}

	// A type with no case-like state branch (reachable only from an anyOf
	// branch naming something like "incident") degrades to the same answer
	// the COALESCE already gives: never matches for "in", always satisfies
	// "not in" -- never invalid empty SQL.
	if got := caseLikeStateLookupClause([]string{"incident"}, "$1::text[]", false); got != "FALSE" {
		t.Errorf("non-case-like type (in) = %q, want FALSE", got)
	}
	if got := caseLikeStateLookupClause([]string{"incident"}, "$1::text[]", true); got != "TRUE" {
		t.Errorf("non-case-like type (not in) = %q, want TRUE", got)
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

// TestCaseLikeStateLookupUsesTheColumnNotACast pins that the enum-typed
// branches compare the state column itself (so its index can be used) and
// drop labels the enum does not know instead of casting the list.
func TestCaseLikeStateLookupUsesTheColumnNotACast(t *testing.T) {
	for typ, enum := range map[string]string{
		"case":                     "case_state_enum",
		"engagement":               "engagement_state_enum",
		"service_request":          "service_request_state_enum",
		"security_report_analysis": "security_report_analysis_state_enum",
	} {
		got := caseLikeStateLookupClause([]string{typ}, "$1::text[]", false)
		if strings.Contains(got, "state::TEXT = ANY($1") {
			t.Errorf("%s: compares state::TEXT, which cannot use the state index: %s", typ, got)
		}
		if !strings.Contains(got, "enum_range(NULL::"+enum+")") {
			t.Errorf("%s: list not filtered through %s: %s", typ, enum, got)
		}
		if strings.Contains(got, "::"+enum+"[]") {
			t.Errorf("%s: casts the list to %s[], which fails on an unknown label: %s", typ, enum, got)
		}
	}
	// Announcement keeps its CLOSE->CLOSED normalization.
	got := caseLikeStateLookupClause([]string{"announcement"}, "$1::text[]", false)
	if !strings.Contains(got, "WHEN state::TEXT = 'CLOSE' THEN 'CLOSED'") {
		t.Errorf("announcement lost its CLOSE->CLOSED normalization: %s", got)
	}
}

// TestCaseFieldPredicates_SeverityColumn: the severity filter matches
// "case".severity alone unless the caller may see a service request's
// severity, in which case it matches the case-or-service-request column.
func TestCaseFieldPredicates_SeverityColumn(t *testing.T) {
	set := caseFieldSet{Severities: []domain.CaseSeverity{domain.CaseSeverityHigh}}
	external, _, _, _ := caseFieldPredicates(set, 1)
	if len(external) != 1 || !strings.HasPrefix(external[0], "c.severity = ANY(") || strings.Contains(external[0], "sr.") {
		t.Errorf("default column predicate = %v, want c.severity only", external)
	}
	for _, tc := range []struct {
		scope        SearchScope
		wantInternal bool
	}{
		{SearchScope{Unrestricted: true}, true},
		{SearchScope{Unrestricted: true, ViaCustomerPortal: true}, false},
		{SearchScope{ProjectIDs: []string{"p"}}, false},
	} {
		scope, wantInternal := tc.scope, tc.wantInternal
		set.SeverityColumn = severityColumnFor(scope)
		got, _, _, _ := caseFieldPredicates(set, 1)
		has := strings.Contains(got[0], "sr.severity")
		if has != wantInternal {
			t.Errorf("scope %+v: predicate %q includes sr.severity = %v, want %v", scope, got[0], has, wantInternal)
		}
	}
}
