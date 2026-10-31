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

package slacondition

import (
	"testing"
	"time"
)

// fakeFacts is a minimal, hand-built Facts implementation for these tests --
// no dependency on the service package's real CaseFacts, so this package
// can be tested in isolation.
type fakeFacts struct {
	fields  map[string]string
	present map[string]bool
	dates   map[string]time.Time
	noField map[string]bool // fields explicitly reported as not-ok by Field
}

func (f fakeFacts) Field(name string) (string, bool) {
	if f.noField[name] {
		return "", false
	}
	v, ok := f.fields[name]
	return v, ok
}

func (f fakeFacts) Present(name string) bool {
	return f.present[name]
}

func (f fakeFacts) Date(name string) (time.Time, bool) {
	d, ok := f.dates[name]
	return d, ok
}

func newFacts() fakeFacts {
	return fakeFacts{
		fields:  map[string]string{},
		present: map[string]bool{},
		dates:   map[string]time.Time{},
		noField: map[string]bool{},
	}
}

func TestEquality(t *testing.T) {
	f := newFacts()
	f.fields["state"] = "OPEN"

	if !Parse("state=OPEN").Evaluate(f) {
		t.Fatal("expected state=OPEN to match")
	}
	if Parse("state=CLOSED").Evaluate(f) {
		t.Fatal("expected state=CLOSED not to match")
	}
}

func TestInequality(t *testing.T) {
	f := newFacts()
	f.fields["priority"] = "S1"

	if !Parse("priority!=S0").Evaluate(f) {
		t.Fatal("expected priority!=S0 to match when priority is S1")
	}
	if Parse("priority!=S1").Evaluate(f) {
		t.Fatal("expected priority!=S1 not to match when priority is S1")
	}
}

func TestInequality_MissingFieldIsTreatedAsNotEqual(t *testing.T) {
	f := newFacts()
	f.noField["priority"] = true

	if !Parse("priority!=S1").Evaluate(f) {
		t.Fatal("expected a missing field to satisfy !=")
	}
}

func TestIn(t *testing.T) {
	f := newFacts()
	f.fields["state"] = "WORK_IN_PROGRESS"

	if !Parse("stateINWORK_IN_PROGRESS,AWAITING_INFO,SOLUTION_PROPOSED,CLOSED").Evaluate(f) {
		t.Fatal("expected IN list to match a contained value")
	}
	f.fields["state"] = "OPEN"
	if Parse("stateINWORK_IN_PROGRESS,AWAITING_INFO,SOLUTION_PROPOSED,CLOSED").Evaluate(f) {
		t.Fatal("expected IN list not to match a value outside the list")
	}
}

// TestNotInLiteralSpace covers the one operator whose ServiceNow spelling
// has a literal space ("NOT IN") unlike every other operator -- confirmed
// only ever seen on incident-type rows this engine skips, but the parser
// must still handle it rather than crash or silently misparse it as
// something else.
func TestNotInLiteralSpace(t *testing.T) {
	f := newFacts()
	f.fields["priority"] = "3"

	cond := Parse("priorityNOT IN1,2")
	if !cond.Evaluate(f) {
		t.Fatal("expected NOT IN to match a value outside the excluded list")
	}

	f.fields["priority"] = "1"
	if cond.Evaluate(f) {
		t.Fatal("expected NOT IN not to match an excluded value")
	}
}

func TestIsEmptyIsNotEmpty(t *testing.T) {
	f := newFacts()
	f.present["account"] = true

	if !Parse("accountISNOTEMPTY").Evaluate(f) {
		t.Fatal("expected ISNOTEMPTY to match a present field")
	}
	if Parse("accountISEMPTY").Evaluate(f) {
		t.Fatal("expected ISEMPTY not to match a present field")
	}

	f.present["account"] = false
	if Parse("accountISNOTEMPTY").Evaluate(f) {
		t.Fatal("expected ISNOTEMPTY not to match an absent field")
	}
	if !Parse("accountISEMPTY").Evaluate(f) {
		t.Fatal("expected ISEMPTY to match an absent field")
	}
}

// TestIsEmptyCheckedBeforeEquals makes sure a field name followed directly
// by ISNOTEMPTY/ISEMPTY is never misparsed by first trying to split on "=" --
// case_typeISNOTEMPTY has no "=" in it at all.
func TestIsEmptyCheckedBeforeEquals(t *testing.T) {
	f := newFacts()
	f.present["case_type"] = true
	f.fields["case_type"] = "service_request"

	clause := parseClauseToken("case_typeISNOTEMPTY")
	if clause.Op != OpIsNotEmpty || clause.Field != "case_type" {
		t.Fatalf("expected ISNOTEMPTY clause on case_type, got %+v", clause)
	}
}

func TestGTEDateField(t *testing.T) {
	f := newFacts()
	f.dates["opened_at"] = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	if !Parse("opened_at>=2022-11-29").Evaluate(f) {
		t.Fatal("expected a later created_on to satisfy >=")
	}

	f.dates["sys_created_on"] = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	if Parse("sys_created_on>=2022-08-26").Evaluate(f) {
		t.Fatal("expected an earlier created_on not to satisfy >=")
	}
}

// TestOrChain covers "^OR" joining a run of clauses into one OR'd term
// that's AND'd with the rest of the group -- the real shape every
// "[Cloud] P<n>" product condition uses.
func TestOrChain(t *testing.T) {
	f := newFacts()
	f.fields["product"] = "Asgardeo"
	f.fields["state"] = "OPEN"
	f.fields["priority"] = "S1"

	cond := Parse("product=WSO2 Developer Platform^ORproduct=Asgardeo^ORproduct=Devant^ORproduct=Bijira^state=OPEN^priority=S1")
	if !cond.Evaluate(f) {
		t.Fatal("expected the OR'd product list AND the rest of the group to match")
	}

	f.fields["product"] = "Something Else"
	if cond.Evaluate(f) {
		t.Fatal("expected no match when the product is outside the OR'd list")
	}
}

// TestNQStartsNewOrGroup covers "^NQ" starting an entirely new AND-group
// OR'd against the previous one -- the real shape "P1 - Response (Open
// Source)" uses to cover two unrelated case types under one policy.
func TestNQStartsNewOrGroup(t *testing.T) {
	cond := Parse("state=OPEN^priority=S1^case_type=case^cs_engineer_response_comment=false^NQstate=OPEN^case_type=security_report_analysis^cs_engineer_response_comment=false")

	f := newFacts()
	f.fields["state"] = "OPEN"
	f.fields["priority"] = "S1"
	f.fields["case_type"] = "case"
	f.fields["cs_engineer_response_comment"] = "false"
	if !cond.Evaluate(f) {
		t.Fatal("expected the first NQ group to match")
	}

	f2 := newFacts()
	f2.fields["state"] = "OPEN"
	f2.fields["priority"] = "S3" // irrelevant to the second group
	f2.fields["case_type"] = "security_report_analysis"
	f2.fields["cs_engineer_response_comment"] = "false"
	if !cond.Evaluate(f2) {
		t.Fatal("expected the second NQ group to match regardless of priority")
	}

	f3 := newFacts()
	f3.fields["state"] = "OPEN"
	f3.fields["priority"] = "S2"
	f3.fields["case_type"] = "hosting"
	f3.fields["cs_engineer_response_comment"] = "false"
	if cond.Evaluate(f3) {
		t.Fatal("expected neither group to match an unrelated case")
	}
}

// TestUnknownFieldNeverCrashesAndNeverMatches covers the incident/escalation
// fields (business_service, category, active_escalation.*,
// escalation_severity, source_table) that this engine never builds
// evaluation support for -- parsing must survive them, and they must never
// accidentally evaluate true.
func TestUnknownFieldNeverCrashesAndNeverMatches(t *testing.T) {
	f := newFacts()
	raw := "business_service=b9c999f81b86a01000ae86acdd4bcb61^ORbusiness_service=97ed1b8b1ba26c1000ae86acdd4bcbd3^priorityIN1,2^category=service_interruption"
	cond := Parse(raw)
	if cond.Evaluate(f) {
		t.Fatal("expected a condition built entirely from unknown/unsatisfied fields not to match")
	}

	// Also exercise the weird, value-less "active_escalation=" shape and the
	// dotted "active_escalation.escalation_severity" field -- neither known,
	// neither should panic.
	cond2 := Parse("active_escalation=^ORstateIN6,3,7")
	_ = cond2.Evaluate(f)
	cond3 := Parse("active_escalation.escalation_severity=3fd67373b3500300ff6e6e5f26a8dcd4^active_escalation.state=101")
	_ = cond3.Evaluate(f)
}

func TestEmptyConditionNeverMatches(t *testing.T) {
	f := newFacts()
	if Parse("").Evaluate(f) {
		t.Fatal("expected an empty condition never to match")
	}
	var nilCond Condition
	if nilCond.Evaluate(f) {
		t.Fatal("expected a nil condition never to match")
	}
}

func TestOpTrueLiteral(t *testing.T) {
	f := newFacts()

	cl := parseClauseToken("TRUE")
	if cl.Op != OpTrue {
		t.Fatalf("parseClauseToken(%q).Op = %v, want OpTrue", "TRUE", cl.Op)
	}
	if !cl.evaluate(f) {
		t.Fatal("OpTrue clause must always evaluate true")
	}

	// A plain AND: TRUE contributes nothing, state=OPEN still decides.
	if !Parse("TRUE^state=OPEN").Evaluate(newFactsWith(map[string]string{"state": "OPEN"})) {
		t.Fatal("TRUE^state=OPEN should match when state=OPEN")
	}
	if Parse("TRUE^state=OPEN").Evaluate(newFactsWith(map[string]string{"state": "CLOSED"})) {
		t.Fatal("TRUE^state=OPEN should not match when state=CLOSED")
	}

	// An OR run: a TRUE disjunct makes the whole OrTerm true regardless of
	// the other disjunct's own value -- this is the exact shape an adapter
	// uses to preserve a dropped-but-always-true clause's effect inside an
	// OR run without losing it (see OpTrue's own doc comment).
	if !Parse("state=CLOSED^ORTRUE").Evaluate(newFactsWith(map[string]string{"state": "OPEN"})) {
		t.Fatal("state=CLOSED^ORTRUE should match (the TRUE disjunct) even though state=CLOSED doesn't")
	}

	// A whole ^NQ group that's just TRUE: the overall condition matches
	// regardless of the other group's own outcome.
	if !Parse("state=CLOSED^NQTRUE").Evaluate(newFactsWith(map[string]string{"state": "OPEN"})) {
		t.Fatal("state=CLOSED^NQTRUE should match via the always-true second group")
	}

	if unknown := Parse("TRUE").UnknownClauses(); len(unknown) != 0 {
		t.Fatalf("UnknownClauses() = %v, want none -- OpTrue must never be reported as unknown", unknown)
	}
}

func newFactsWith(fields map[string]string) fakeFacts {
	f := newFacts()
	for k, v := range fields {
		f.fields[k] = v
	}
	return f
}

// TestRealPauseResumeConditions pins the parser against two real,
// currently-active rows (P1 - Workaround (Managed Services)) so a future
// change to the grammar can't silently break production policies.
func TestRealPauseResumeConditions(t *testing.T) {
	pause := Parse("stateINAWAITING_INFO,SOLUTION_PROPOSED^ORu_workaround_providedISNOTEMPTY")
	resume := Parse("stateINWORK_IN_PROGRESS,WAITING_ON_WSO2^u_workaround_providedISEMPTY")

	f := newFacts()
	f.fields["state"] = "AWAITING_INFO"
	f.present["u_workaround_provided"] = false
	if !pause.Evaluate(f) {
		t.Fatal("expected pause to match on AWAITING_INFO")
	}
	if resume.Evaluate(f) {
		t.Fatal("expected resume not to match on AWAITING_INFO")
	}

	f.fields["state"] = "WORK_IN_PROGRESS"
	f.present["u_workaround_provided"] = false
	if pause.Evaluate(f) {
		t.Fatal("expected pause not to match once workaround is empty and state is WORK_IN_PROGRESS")
	}
	if !resume.Evaluate(f) {
		t.Fatal("expected resume to match on WORK_IN_PROGRESS with no workaround yet")
	}

	f.fields["state"] = "WORK_IN_PROGRESS"
	f.present["u_workaround_provided"] = true
	if !pause.Evaluate(f) {
		t.Fatal("expected pause to match once a workaround has been provided, regardless of state")
	}
}
